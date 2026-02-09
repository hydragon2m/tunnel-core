package connection

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

// TestGracefulShutdown tests the graceful shutdown functionality
func TestGracefulShutdown(t *testing.T) {
	tests := []struct {
		name          string
		numConns      int
		timeout       time.Duration
		expectTimeout bool
	}{
		{
			name:          "shutdown with no connections",
			numConns:      0,
			timeout:       5 * time.Second,
			expectTimeout: false,
		},
		{
			name:          "shutdown with few connections",
			numConns:      3,
			timeout:       5 * time.Second,
			expectTimeout: false,
		},
		{
			name:          "shutdown with many connections",
			numConns:      10,
			timeout:       5 * time.Second,
			expectTimeout: false,
		},
		{
			name:          "shutdown with timeout",
			numConns:      5,
			timeout:       1 * time.Millisecond, // Very short timeout
			expectTimeout: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(100, 10, 30*time.Second)

			// Register connections
			conns := make([]*mockConnSimple, tt.numConns)
			for i := 0; i < tt.numConns; i++ {
				mockConn := newMockConnSimple()
				conns[i] = mockConn

				connID := "conn-" + string(rune(i+'0'))
				agentID := "agent-" + string(rune(i+'0'))

				_, err := manager.RegisterConnection(connID, agentID, "test-account", mockConn, nil)
				if err != nil {
					t.Fatalf("Failed to register connection: %v", err)
				}
			}

			// Verify connections registered
			if len(manager.GetAllConnections()) != tt.numConns {
				t.Errorf("Expected %d connections, got %d", tt.numConns, len(manager.GetAllConnections()))
			}

			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			// Graceful shutdown
			timedOut := manager.GracefulShutdown(ctx)

			// Verify results
			if tt.expectTimeout {
				if timedOut == 0 {
					t.Error("Expected timeout, but all connections closed gracefully")
				}
			} else {
				if timedOut > 0 {
					t.Errorf("Expected graceful shutdown, but %d connections timed out", timedOut)
				}
			}

			// Verify all connections are closed
			remaining := len(manager.GetAllConnections())
			if remaining > 0 {
				t.Errorf("Expected 0 remaining connections, got %d", remaining)
			}

			// Verify mock connections were closed
			for i, conn := range conns {
				if !conn.closed {
					t.Errorf("Connection %d was not closed", i)
				}
			}
		})
	}
}

// TestGracefulShutdownWithContext tests graceful shutdown with context cancellation
func TestGracefulShutdownWithContext(t *testing.T) {
	manager := NewManager(100, 10, 30*time.Second)

	// Register a few connections
	for i := 0; i < 3; i++ {
		mockConn := newMockConnSimple()
		connID := "conn-" + string(rune(i+'0'))
		agentID := "agent-" + string(rune(i+'0'))

		_, err := manager.RegisterConnection(connID, agentID, "test-account", mockConn, nil)
		if err != nil {
			t.Fatalf("Failed to register connection: %v", err)
		}
	}

	// Create context and cancel it immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Attempt graceful shutdown with cancelled context
	timedOut := manager.GracefulShutdown(ctx)

	// Should timeout since context is already cancelled
	if timedOut == 0 {
		t.Error("Expected timeout with cancelled context")
	}
}

// mockConnSimple implements the Conn interface for graceful shutdown testing
type mockConnSimple struct {
	closed  bool
	readCh  chan byte
	writeCh chan byte
}

func newMockConnSimple() *mockConnSimple {
	return &mockConnSimple{
		readCh:  make(chan byte),
		writeCh: make(chan byte),
	}
}

func (m *mockConnSimple) Read(b []byte) (n int, err error) {
	// Block until closed
	<-m.readCh
	return 0, io.EOF
}

func (m *mockConnSimple) Write(b []byte) (n int, err error) {
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return len(b), nil
}

func (m *mockConnSimple) Close() error {
	if !m.closed {
		m.closed = true
		close(m.readCh)
	}
	return nil
}

func (m *mockConnSimple) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConnSimple) SetWriteDeadline(t time.Time) error {
	return nil
}

func (m *mockConnSimple) RemoteAddr() net.Addr {
	return &mockAddrSimple{}
}

type mockAddrSimple struct{}

func (mockAddrSimple) Network() string { return "tcp" }
func (mockAddrSimple) String() string  { return "127.0.0.1:12345" }
