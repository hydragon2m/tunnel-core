package listener

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"
)

// HTTPListener là HTTP/HTTPS server nhận requests từ public
type HTTPListener struct {
	server   *http.Server
	listener net.Listener
	handler  http.Handler
}

// NewHTTPListener tạo HTTP listener mới
func NewHTTPListener(addr string, useTLS bool, certFile, keyFile string, handler http.Handler) (*HTTPListener, error) {
	// Create HTTP server
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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

	return &HTTPListener{
		server:   server,
		listener: listener,
		handler:  handler,
	}, nil
}

// Start starts the HTTP server
func (l *HTTPListener) Start() error {
	return l.server.Serve(l.listener)
}

// StartWithContext starts the HTTP server with context for graceful shutdown
func (l *HTTPListener) StartWithContext(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		if err := l.server.Serve(l.listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		// Graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return l.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// Close closes the listener
func (l *HTTPListener) Close() error {
	if l.listener != nil {
		return l.listener.Close()
	}
	return nil
}

// Addr returns the listener address
func (l *HTTPListener) Addr() net.Addr {
	if l.listener != nil {
		return l.listener.Addr()
	}
	return nil
}

