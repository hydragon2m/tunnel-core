package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hydragon2m/tunnel-core/connection"
	"github.com/hydragon2m/tunnel-core/internal/account"
	"github.com/hydragon2m/tunnel-core/internal/dashboard"
	"github.com/hydragon2m/tunnel-core/internal/handshake"
	"github.com/hydragon2m/tunnel-core/internal/listener"
	"github.com/hydragon2m/tunnel-core/internal/quota"
	"github.com/hydragon2m/tunnel-core/internal/registry"
	"github.com/hydragon2m/tunnel-core/internal/router"
	"github.com/hydragon2m/tunnel-core/pkg/health"
	"github.com/hydragon2m/tunnel-core/pkg/metrics"
	v1 "github.com/hydragon2m/tunnel-protocol/go/v1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Agent listener config
	agentAddr     = flag.String("agent-addr", ":8443", "Address to listen for agent connections")
	agentTLS      = flag.Bool("agent-tls", true, "Enable TLS for agent connections")
	agentCertFile = flag.String("agent-cert", "", "TLS certificate file for agent connections")
	agentKeyFile  = flag.String("agent-key", "", "TLS key file for agent connections")

	// Public listener config
	publicAddr     = flag.String("public-addr", ":8080", "Address to listen for public HTTP requests")
	publicTLS      = flag.Bool("public-tls", false, "Enable TLS for public connections")
	publicCertFile = flag.String("public-cert", "", "TLS certificate file for public connections")
	publicKeyFile  = flag.String("public-key", "", "TLS key file for public connections")

	// Base domain
	baseDomain    = flag.String("base-domain", "localhost", "Base domain for tunnels")
	dashboardAddr = flag.String("dashboard-addr", ":9000", "Address to listen for dashboard")
	metricsAddr   = flag.String("metrics-addr", ":9090", "Address to listen for Prometheus metrics")

	// Config
	maxConnections   = flag.Int("max-connections", 1000, "Maximum number of agent connections")
	heartbeatTimeout = flag.Duration("heartbeat-timeout", 30*time.Second, "Heartbeat timeout")
	authTimeout      = flag.Duration("auth-timeout", 10*time.Second, "Authentication timeout")

	// Auth config
	jwtSecret = flag.String("jwt-secret", "", "JWT secret key (if empty, uses simple token validation)")
)

func main() {
	flag.Parse()

	// Override with environment variables if set
	if envAddr := os.Getenv("AGENT_ADDR"); envAddr != "" {
		*agentAddr = envAddr
	}
	if envTLS := os.Getenv("AGENT_TLS"); envTLS != "" {
		*agentTLS = (envTLS == "true")
	}
	if envCert := os.Getenv("AGENT_CERT"); envCert != "" {
		*agentCertFile = envCert
	}
	if envKey := os.Getenv("AGENT_KEY"); envKey != "" {
		*agentKeyFile = envKey
	}
	if envPublicAddr := os.Getenv("PUBLIC_ADDR"); envPublicAddr != "" {
		*publicAddr = envPublicAddr
	}
	if envBaseDomain := os.Getenv("BASE_DOMAIN"); envBaseDomain != "" {
		*baseDomain = envBaseDomain
	}
	if envMaxConn := os.Getenv("MAX_CONNECTIONS"); envMaxConn != "" {
		if maxConn, err := parseInt(envMaxConn); err == nil {
			*maxConnections = maxConn
		}
	}
	if envHeartbeat := os.Getenv("HEARTBEAT_TIMEOUT"); envHeartbeat != "" {
		if timeout, err := time.ParseDuration(envHeartbeat); err == nil {
			*heartbeatTimeout = timeout
		}
	}

	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Tunnel Core Server...")

	// Initialize Prometheus metrics
	metrics.Init()
	log.Println("Prometheus metrics initialized")

	// Initialize health checker
	healthChecker := health.NewChecker("1.0.0")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize components
	accountStore, err := account.NewJSONStore("accounts.json")
	if err != nil {
		log.Fatalf("Failed to initialize account store: %v", err)
	}

	// Create default account if none exists
	if accs, _ := accountStore.List(); len(accs) == 0 {
		log.Println("Creating default account...")
		accountStore.Save(&account.Account{
			ID:         "default",
			Token:      "test-token",
			AdminToken: "admin-token",
			Subdomains: []string{"test", "demo"},
			MaxConns:   10,
		})
	}

	connManager := connection.NewManager(*maxConnections, *maxConnections, *heartbeatTimeout)
	reg := registry.NewRegistry(*baseDomain)
	limiter := quota.NewLimiter(*maxConnections, 10000) // Max 10000 concurrent streams globally

	var validator handshake.TokenValidator
	if *jwtSecret != "" {
		log.Println("Using JWT authentication")
		validator = handshake.NewJWTValidator([]byte(*jwtSecret))
	} else {
		log.Println("Using account-based token authentication")
		validator = &handshake.SimpleValidator{
			ValidateFn: func(token string) (agentID string, err error) {
				acc, err := accountStore.GetByToken(token)
				if err != nil {
					return "", handshake.ErrInvalidToken
				}
				return acc.ID, nil
			},
		}
	}

	authenticator := handshake.NewAuthenticator(validator, *authTimeout)

	// Setup connection callbacks
	connManager.SetOnConnectionClosed(func(connID string) {
		log.Printf("Connection closed: %s", connID)
		// Cleanup tunnels for this connection
		reg.UnregisterConnectionTunnels(connID)
	})

	// Start agent listener
	agentListener, err := startAgentListener(*agentAddr, *agentTLS, *agentCertFile, *agentKeyFile)
	if err != nil {
		log.Fatalf("Failed to start agent listener: %v", err)
	}
	defer agentListener.Close()

	log.Printf("Agent listener started on %s (TLS: %v)", *agentAddr, *agentTLS)

	// Create router with limiter
	httpRouter := router.NewRouter(reg, connManager, limiter, 30*time.Second)

	// Register health checks
	healthChecker.RegisterCheck("connections", func() health.Check {
		// Get all connections and count active ones
		conns := connManager.GetAllConnections()
		activeCount := len(conns)
		maxConn := *maxConnections

		status := health.StatusHealthy
		message := fmt.Sprintf("%d/%d connections", activeCount, maxConn)

		if activeCount >= maxConn {
			status = health.StatusUnhealthy
			message = "Connection limit reached"
		} else if float64(activeCount)/float64(maxConn) > 0.9 {
			status = health.StatusDegraded
			message = "Connection limit nearly reached"
		}

		return health.Check{
			Status:  status,
			Message: message,
			Details: map[string]interface{}{
				"active": activeCount,
				"max":    maxConn,
			},
		}
	}, true)

	healthChecker.RegisterCheck("system", func() health.Check {
		return health.SystemCheck()
	}, false)

	// Start public listener
	publicListener, err := listener.NewHTTPListener(*publicAddr, *publicTLS, *publicCertFile, *publicKeyFile, httpRouter)
	if err != nil {
		log.Fatalf("Failed to start public listener: %v", err)
	}
	defer publicListener.Close()

	log.Printf("Public listener started on %s (TLS: %v)", *publicAddr, *publicTLS)

	// Handle agent connections
	go handleAgentConnections(ctx, agentListener, connManager, reg, authenticator)

	// Handle public HTTP requests
	go func() {
		if err := publicListener.StartWithContext(ctx); err != nil {
			log.Printf("Public listener error: %v", err)
		}
	}()

	// Start health check server (on dashboard port with /health endpoints)
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", healthChecker.SimpleHandler())
	healthMux.HandleFunc("/health/live", healthChecker.LivenessHandler())
	healthMux.HandleFunc("/health/ready", healthChecker.ReadinessHandler())
	healthMux.HandleFunc("/health/detailed", healthChecker.DetailedHandler())

	// Combine dashboard, health and metrics endpoints
	db := dashboard.NewDashboard(connManager, reg, accountStore)
	combinedMux := http.NewServeMux()
	combinedMux.Handle("/health", healthMux)
	combinedMux.Handle("/health/", healthMux)
	combinedMux.Handle("/metrics", promhttp.Handler())
	combinedMux.Handle("/", db)

	dashboardServer := &http.Server{
		Addr:    *dashboardAddr,
		Handler: combinedMux,
	}
	go func() {
		log.Printf("Dashboard, health checks, and metrics started on %s", *dashboardAddr)
		if err := dashboardServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Dashboard server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	log.Println("Server started. Press Ctrl+C to stop.")
	<-sigCh

	log.Println("Shutting down...")
	cancel()

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Close listeners
	agentListener.Close()
	publicListener.Close()

	// Shutdown dashboard server
	if err := dashboardServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Dashboard server shutdown error: %v", err)
	}

	// Gracefully close all connections
	log.Println("Closing agent connections...")
	timedOut := connManager.GracefulShutdown(shutdownCtx)
	if timedOut > 0 {
		log.Printf("Warning: %d connections were forcefully closed due to timeout", timedOut)
	} else {
		log.Println("All connections closed gracefully")
	}

	select {
	case <-shutdownCtx.Done():
		log.Println("Shutdown timeout")
	case <-time.After(100 * time.Millisecond):
		log.Println("Shutdown complete")
	}
}

// startAgentListener starts TCP/TLS listener for agent connections
func startAgentListener(addr string, useTLS bool, certFile, keyFile string) (net.Listener, error) {
	var listener net.Listener
	var err error

	if useTLS {
		if certFile == "" || keyFile == "" {
			return nil, fmt.Errorf("TLS certificate and key files required when TLS is enabled")
		}

		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
		}

		config := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		listener, err = tls.Listen("tcp", addr, config)
	} else {
		listener, err = net.Listen("tcp", addr)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	return listener, nil
}

// handleAgentConnections handles incoming agent connections
func handleAgentConnections(
	ctx context.Context,
	listener net.Listener,
	connManager *connection.Manager,
	reg *registry.Registry,
	authenticator *handshake.Authenticator,
) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Accept connection
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					log.Printf("Failed to accept connection: %v", err)
					continue
				}
			}

			// Handle connection in goroutine
			go handleAgentConnection(ctx, conn, connManager, reg, authenticator)
		}
	}
}

// handleAgentConnection handles a single agent connection
func handleAgentConnection(
	ctx context.Context,
	rawConn net.Conn,
	connManager *connection.Manager,
	reg *registry.Registry,
	authenticator *handshake.Authenticator,
) {
	defer rawConn.Close()

	remoteAddr := rawConn.RemoteAddr().String()
	log.Printf("New agent connection from %s", remoteAddr)

	// Wrap connection
	conn := &netConnWrapper{Conn: rawConn}

	// Set read deadline for auth (10 seconds default)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// Read and decode first frame (should be FrameAuth)
	frame, err := v1.Decode(conn)
	if err != nil {
		log.Printf("Failed to decode auth frame from %s: %v", remoteAddr, err)
		return
	}

	// Handle authentication
	agentID, metadata, err := authenticator.HandleAuth(frame)
	if err != nil {
		log.Printf("Authentication failed for %s: %v", remoteAddr, err)
		// Send error response
		errorFrame, _ := authenticator.CreateAuthErrorResponse(err.Error())
		_ = v1.Encode(conn, errorFrame)
		return
	}

	// Send success response
	successFrame, err := authenticator.CreateAuthSuccessResponse(agentID, nil)
	if err != nil {
		log.Printf("Failed to create auth response: %v", err)
		return
	}

	if err := v1.Encode(conn, successFrame); err != nil {
		log.Printf("Failed to send auth response: %v", err)
		return
	}

	log.Printf("Agent authenticated: %s from %s", agentID, remoteAddr)

	// Generate connection ID
	connID := fmt.Sprintf("%s-%d", agentID, time.Now().UnixNano())

	// Register connection
	registeredConn, err := connManager.RegisterConnection(connID, agentID, "legacy-account", conn, metadata)
	if err != nil {
		log.Printf("Failed to register connection: %v", err)
		return
	}

	log.Printf("Connection registered: %s (agent: %s)", connID, agentID)

	// Register tunnels based on metadata
	registeredTunnels := 0
	if subdomains, ok := metadata["subdomains"]; ok && subdomains != "" {
		for _, sub := range strings.Split(subdomains, ",") {
			sub = strings.TrimSpace(sub)
			if sub == "" {
				continue
			}
			tunnel, err := reg.RegisterTunnel("", sub, connID, agentID, metadata)
			if err != nil {
				log.Printf("Failed to register tunnel for subdomain %s: %v", sub, err)
			} else {
				log.Printf("Tunnel registered: %s -> %s", tunnel.FullDomain, connID)
				registeredTunnels++
			}
		}
	} else if subdomain, ok := metadata["subdomain"]; ok && subdomain != "" {
		tunnel, err := reg.RegisterTunnel("", subdomain, connID, agentID, metadata)
		if err != nil {
			log.Printf("Failed to register tunnel for subdomain %s: %v", subdomain, err)
		} else {
			log.Printf("Tunnel registered: %s -> %s", tunnel.FullDomain, connID)
			registeredTunnels++
		}
	}

	// Fallback to use agentID as subdomain
	if agentID != "" {
		tunnel, err := reg.RegisterTunnel("", agentID, connID, agentID, metadata)
		if err != nil {
			log.Printf("Failed to register fallback tunnel for agent %s: %v", agentID, err)
		} else {
			log.Printf("Fallback tunnel registered: %s -> %s", tunnel.FullDomain, connID)
		}
	}

	// Wait for connection to close
	<-registeredConn.Context().Done()
	log.Printf("Connection closed: %s", connID)
	// Tunnels are cleaned up by the OnConnectionClosed callback registered in main()
}

// netConnWrapper wraps net.Conn to implement connection.Conn interface
type netConnWrapper struct {
	net.Conn
}

// parseInt parses string to int
func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}
