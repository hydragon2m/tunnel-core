package testutil

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	v1 "github.com/hydragon2m/tunnel-protocol/go/v1"
)

// MockConnection implements a mock network connection for testing
type MockConnection struct {
	readData  []byte
	writeData []byte
	readPos   int
	closed    bool
	mu        sync.Mutex
	readErr   error
	writeErr  error
}

// NewMockConnection creates a new mock connection
func NewMockConnection() *MockConnection {
	return &MockConnection{
		readData:  make([]byte, 0),
		writeData: make([]byte, 0),
	}
}

// SetReadData sets the data to be returned by Read
func (m *MockConnection) SetReadData(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readData = data
	m.readPos = 0
}

// SetReadError sets an error to be returned by Read
func (m *MockConnection) SetReadError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readErr = err
}

// SetWriteError sets an error to be returned by Write
func (m *MockConnection) SetWriteError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeErr = err
}

// GetWrittenData returns all data written to the connection
func (m *MockConnection) GetWrittenData() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeData
}

// Read implements io.Reader
func (m *MockConnection) Read(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.readErr != nil {
		return 0, m.readErr
	}

	if m.readPos >= len(m.readData) {
		return 0, nil
	}

	n = copy(b, m.readData[m.readPos:])
	m.readPos += n
	return n, nil
}

// Write implements io.Writer
func (m *MockConnection) Write(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.writeErr != nil {
		return 0, m.writeErr
	}

	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

// Close implements io.Closer
func (m *MockConnection) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// SetReadDeadline implements net.Conn
func (m *MockConnection) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline implements net.Conn
func (m *MockConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

// SetDeadline implements net.Conn
func (m *MockConnection) SetDeadline(t time.Time) error {
	return nil
}

// LocalAddr implements net.Conn
func (m *MockConnection) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
}

// RemoteAddr implements net.Conn
func (m *MockConnection) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 54321}
}

// IsClosed returns whether the connection is closed
func (m *MockConnection) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// Frame Builders

// BuildAuthFrame creates an authentication frame
func BuildAuthFrame(agentID, accountID, token string) *v1.Frame {
	payload := []byte(token)
	return &v1.Frame{
		Version:  v1.Version,
		Type:     v1.FrameAuth,
		Flags:    v1.FlagNone,
		StreamID: 0,
		Payload:  payload,
	}
}

// BuildHeartbeatFrame creates a heartbeat frame
func BuildHeartbeatFrame() *v1.Frame {
	return &v1.Frame{
		Version:  v1.Version,
		Type:     v1.FrameHeartbeat,
		Flags:    v1.FlagNone,
		StreamID: 0,
		Payload:  nil,
	}
}

// BuildOpenStreamFrame creates an open stream frame
func BuildOpenStreamFrame(streamID uint32, payload []byte) *v1.Frame {
	return &v1.Frame{
		Version:  v1.Version,
		Type:     v1.FrameOpenStream,
		Flags:    v1.FlagNone,
		StreamID: streamID,
		Payload:  payload,
	}
}

// BuildDataFrame creates a data frame
func BuildDataFrame(streamID uint32, data []byte, flags uint8) *v1.Frame {
	return &v1.Frame{
		Version:  v1.Version,
		Type:     v1.FrameData,
		Flags:    flags,
		StreamID: streamID,
		Payload:  data,
	}
}

// BuildCloseFrame creates a close frame
func BuildCloseFrame(streamID uint32) *v1.Frame {
	return &v1.Frame{
		Version:  v1.Version,
		Type:     v1.FrameClose,
		Flags:    v1.FlagNone,
		StreamID: streamID,
		Payload:  nil,
	}
}

// Test Assertions

// AssertFrameType asserts that a frame has the expected type
func AssertFrameType(t *testing.T, frame *v1.Frame, expected uint8) {
	t.Helper()
	if frame.Type != expected {
		t.Errorf("Expected frame type %v, got %v", expected, frame.Type)
	}
}

// AssertFrameStreamID asserts that a frame has the expected stream ID
func AssertFrameStreamID(t *testing.T, frame *v1.Frame, expected uint32) {
	t.Helper()
	if frame.StreamID != expected {
		t.Errorf("Expected stream ID %d, got %d", expected, frame.StreamID)
	}
}

// AssertFrameFlags asserts that a frame has the expected flags
func AssertFrameFlags(t *testing.T, frame *v1.Frame, expected uint8) {
	t.Helper()
	if frame.Flags != expected {
		t.Errorf("Expected flags %d, got %d", expected, frame.Flags)
	}
}

// AssertNoError asserts that an error is nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// AssertError asserts that an error is not nil
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
}

// AssertEqual asserts that two values are equal
func AssertEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

// AssertTrue asserts that a condition is true
func AssertTrue(t *testing.T, condition bool, message string) {
	t.Helper()
	if !condition {
		t.Errorf("Assertion failed: %s", message)
	}
}

// AssertFalse asserts that a condition is false
func AssertFalse(t *testing.T, condition bool, message string) {
	t.Helper()
	if condition {
		t.Errorf("Assertion failed: %s", message)
	}
}

// Test Helpers

// WaitForCondition waits for a condition to become true or times out
func WaitForCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("Timeout waiting for condition: %s", message)
		case <-ticker.C:
			if condition() {
				return
			}
		}
	}
}

// RunConcurrent runs a function n times concurrently
func RunConcurrent(n int, fn func(i int)) {
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			fn(idx)
		}(i)
	}
	wg.Wait()
}
