package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hydragon2m/tunnel-protocol/go/v1"
	"github.com/hydragon2m/tunnel-core/internal/connection"
	"github.com/hydragon2m/tunnel-core/internal/handshake"
	"github.com/hydragon2m/tunnel-core/internal/listener"
	"github.com/hydragon2m/tunnel-core/internal/quota"
	"github.com/hydragon2m/tunnel-core/internal/registry"
	"github.com/hydragon2m/tunnel-core/internal/router"
)

var (
	// Agent listener config
	agentAddr     = flag.String("agent-addr", ":8443", "Address to listen for agent connections")
	agentTLS      = flag.Bool("agent-tls", true, "Enable TLS for agent connections")
	agentCertFile  = flag.String("agent-cert", "", "TLS certificate file for agent connections")
	agentKeyFile   = flag.String("agent-key", "", "TLS key file for agent connections")

	// Public listener config
	publicAddr    = flag.String("public-addr", ":8080", "Address to listen for public HTTP requests")
	publicTLS     = flag.Bool("public-tls", false, "Enable TLS for public connections")
	publicCertFile = flag.String("public-cert", "", "TLS certificate file for public connections")
	publicKeyFile  = flag.String("public-key", "", "TLS key file for public connections")

	// Base domain
	baseDomain = flag.String("base-domain", "localhost", "Base domain for tunnels")

	// Config
	maxConnections    = flag.Int("max-connections", 1000, "Maximum number of agent connections")
	heartbeatTimeout  = flag.Duration("heartbeat-timeout", 30*time.Second, "Heartbeat timeout")
	authTimeout       = flag.Duration("auth-timeout", 10*time.Second, "Authentication timeout")
)

func main() {
	flag.Parse()

	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Tunnel Core Server...")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize components
	connManager := connection.NewManager(*maxConnections, *heartbeatTimeout)
	reg := registry.NewRegistry(*baseDomain)
	limiter := quota.NewLimiter(*maxConnections, 10000) // Max 10000 concurrent streams globally

	// Simple token validator (replace with your auth logic)
	validateToken := func(token string) (agentID string, err error) {
		// TODO: Implement actual token validation
		// For now, accept any non-empty token
		if token == "" {
			return "", handshake.ErrInvalidToken
		}
		// Extract agent ID from token or use token as agent ID
		return token, nil
	}

	authenticator := handshake.NewAuthenticator(validateToken, *authTimeout)

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

	// Close all connections
	// TODO: Implement graceful connection close

	select {
	case <-shutdownCtx.Done():
		log.Println("Shutdown timeout")
	case <-time.After(1 * time.Second):
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
	registeredConn, err := connManager.RegisterConnection(connID, agentID, conn, metadata)
	if err != nil {
		log.Printf("Failed to register connection: %v", err)
		return
	}

	log.Printf("Connection registered: %s (agent: %s)", connID, agentID)

	// Wait for connection to close
	<-registeredConn.Context().Done()
	log.Printf("Connection closed: %s", connID)
}

// netConnWrapper wraps net.Conn to implement connection.Conn interface
type netConnWrapper struct {
	net.Conn
}

func (w *netConnWrapper) RemoteAddr() string {
	return w.Conn.RemoteAddr().String()
}
