package connection

import (
	"net"
	"sync"
	"testing"
	"time"

	v1 "github.com/hydragon2m/tunnel-protocol/go/v1"
)

func TestConnectionManager_RegisterConnection(t *testing.T) {
	cm := NewManager(100, 30*time.Second)

	// Create mock connection
	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()

	// Create Conn wrapper
	connWrapper := &mockConn{conn: conn1}

	conn, err := cm.RegisterConnection("conn-1", "agent-1", connWrapper, map[string]string{"version": "1.0.0"})
	if err != nil {
		t.Fatalf("RegisterConnection failed: %v", err)
	}

	if conn == nil {
		t.Fatal("Expected connection, got nil")
	}

	if conn.AgentID != "agent-1" {
		t.Errorf("Expected agent ID 'agent-1', got '%s'", conn.AgentID)
	}
}

// mockConn implements Conn interface
type mockConn struct {
	conn net.Conn
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	return m.conn.Read(b)
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	return m.conn.Write(b)
}

func (m *mockConn) Close() error {
	return m.conn.Close()
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	return m.conn.SetReadDeadline(t)
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return m.conn.SetWriteDeadline(t)
}

func (m *mockConn) RemoteAddr() string {
	return m.conn.RemoteAddr().String()
}

func TestConnectionManager_GetConnection(t *testing.T) {
	cm := NewManager(100, 30*time.Second)

	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()

	connWrapper := &mockConn{conn: conn1}

	_, err := cm.RegisterConnection("conn-1", "agent-1", connWrapper, nil)
	if err != nil {
		t.Fatalf("RegisterConnection failed: %v", err)
	}

	conn, ok := cm.GetConnection("conn-1")
	if !ok {
		t.Fatal("Expected connection to exist")
	}

	if conn.AgentID != "agent-1" {
		t.Errorf("Expected agent ID 'agent-1', got '%s'", conn.AgentID)
	}

	// Test non-existent connection
	_, ok = cm.GetConnection("conn-2")
	if ok {
		t.Error("Expected connection to not exist")
	}
}

func TestConnectionManager_CloseConnection(t *testing.T) {
	cm := NewManager(100, 30*time.Second)

	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()

	connWrapper := &mockConn{conn: conn1}

	_, err := cm.RegisterConnection("conn-1", "agent-1", connWrapper, nil)
	if err != nil {
		t.Fatalf("RegisterConnection failed: %v", err)
	}

	err = cm.CloseConnection("conn-1")
	if err != nil {
		t.Fatalf("CloseConnection failed: %v", err)
	}

	_, ok := cm.GetConnection("conn-1")
	if ok {
		t.Error("Expected connection to be closed")
	}
}

func TestConnection_CreateStream(t *testing.T) {
	cm := NewManager(100, 30*time.Second)

	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()

	connWrapper := &mockConn{conn: conn1}

	conn, err := cm.RegisterConnection("conn-1", "agent-1", connWrapper, nil)
	if err != nil {
		t.Fatalf("RegisterConnection failed: %v", err)
	}

	// Allocate stream ID
	streamID := conn.AllocateStreamID()

	// Create stream manually
	stream := conn.createStream(streamID)

	if stream == nil {
		t.Fatal("Expected stream, got nil")
	}

	if stream.ID != streamID {
		t.Errorf("Expected stream ID %d, got %d", streamID, stream.ID)
	}
}

func TestConnection_GetStream(t *testing.T) {
	cm := NewManager(100, 30*time.Second)

	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()

	connWrapper := &mockConn{conn: conn1}

	conn, err := cm.RegisterConnection("conn-1", "agent-1", connWrapper, nil)
	if err != nil {
		t.Fatalf("RegisterConnection failed: %v", err)
	}

	// Allocate stream ID
	streamID := conn.AllocateStreamID()

	// Create stream manually (simulate FrameOpenStream)
	stream := conn.createStream(streamID)

	// Verify stream exists
	gotStream, ok := conn.GetStream(streamID)
	if !ok {
		t.Fatal("Expected stream to exist")
	}

	if gotStream.ID != streamID {
		t.Errorf("Expected stream ID %d, got %d", streamID, gotStream.ID)
	}

	if gotStream != stream {
		t.Error("Expected same stream instance")
	}

	// Test non-existent stream
	_, ok = conn.GetStream(9999)
	if ok {
		t.Error("Expected stream to not exist")
	}
}

func TestConnection_CloseStream(t *testing.T) {
	cm := NewManager(100, 30*time.Second)

	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()

	connWrapper := &mockConn{conn: conn1}

	conn, err := cm.RegisterConnection("conn-1", "agent-1", connWrapper, nil)
	if err != nil {
		t.Fatalf("RegisterConnection failed: %v", err)
	}

	// Allocate stream ID
	streamID := conn.AllocateStreamID()

	// Create stream manually
	conn.createStream(streamID)

	// Close stream
	conn.closeStream(streamID)

	_, ok := conn.GetStream(streamID)
	if ok {
		t.Error("Expected stream to be closed")
	}
}

func TestConnection_Heartbeat(t *testing.T) {
	cm := NewManager(100, 30*time.Second)

	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()

	connWrapper := &mockConn{conn: conn1}

	conn, err := cm.RegisterConnection("conn-1", "agent-1", connWrapper, nil)
	if err != nil {
		t.Fatalf("RegisterConnection failed: %v", err)
	}

	// Check initial heartbeat
	initialHeartbeat := conn.LastHeartbeat
	if initialHeartbeat.IsZero() {
		t.Error("Expected initial heartbeat to be set")
	}

	// Wait a bit and update again (using private method via reflection or direct field access)
	time.Sleep(10 * time.Millisecond)
	conn.updateHeartbeat()
	
	newHeartbeat := conn.LastHeartbeat
	if !newHeartbeat.After(initialHeartbeat) {
		t.Error("Expected new heartbeat to be after old heartbeat")
	}
}

func TestConnectionManager_ConcurrentAccess(t *testing.T) {
	cm := NewManager(100, 30*time.Second)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent RegisterConnection
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			conn1, conn2 := net.Pipe()
			defer conn1.Close()
			defer conn2.Close()

			connWrapper := &mockConn{conn: conn1}
			connID := "conn-" + string(rune(id))
			agentID := "agent-" + string(rune(id))
			_, err := cm.RegisterConnection(connID, agentID, connWrapper, nil)
			if err != nil {
				t.Errorf("RegisterConnection failed: %v", err)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent GetConnection
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			connID := "conn-" + string(rune(id))
			_, _ = cm.GetConnection(connID)
		}(i)
	}
	wg.Wait()
}

func TestConnection_ConcurrentStreams(t *testing.T) {
	cm := NewManager(100, 30*time.Second)

	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()

	connWrapper := &mockConn{conn: conn1}

	conn, err := cm.RegisterConnection("conn-1", "agent-1", connWrapper, nil)
	if err != nil {
		t.Fatalf("RegisterConnection failed: %v", err)
	}

	var wg sync.WaitGroup
	numStreams := 10
	streamIDs := make([]uint32, numStreams)

	// Concurrent CreateStream
	wg.Add(numStreams)
	for i := 0; i < numStreams; i++ {
		go func(idx int) {
			defer wg.Done()
			streamID := conn.AllocateStreamID()
			conn.createStream(streamID)
			streamIDs[idx] = streamID
		}(i)
	}
	wg.Wait()

	// Verify all streams exist
	for _, streamID := range streamIDs {
		if streamID == 0 {
			continue // Skip failed streams
		}
		_, ok := conn.GetStream(streamID)
		if !ok {
			t.Errorf("Expected stream %d to exist", streamID)
		}
	}
}

func TestConnection_Context(t *testing.T) {
	cm := NewManager(100, 30*time.Second)

	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()

	connWrapper := &mockConn{conn: conn1}

	conn, err := cm.RegisterConnection("conn-1", "agent-1", connWrapper, nil)
	if err != nil {
		t.Fatalf("RegisterConnection failed: %v", err)
	}

	ctx := conn.Context()
	if ctx == nil {
		t.Fatal("Expected context, got nil")
	}

	// Context should not be done
	select {
	case <-ctx.Done():
		t.Error("Expected context to not be done")
	default:
		// OK
	}
}

func TestConnection_SendFrame(t *testing.T) {
	cm := NewManager(100, 30*time.Second)

	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()

	connWrapper := &mockConn{conn: conn1}

	conn, err := cm.RegisterConnection("conn-1", "agent-1", connWrapper, nil)
	if err != nil {
		t.Fatalf("RegisterConnection failed: %v", err)
	}

	// Create frame
	frame := &v1.Frame{
		Version:  v1.Version,
		Type:     v1.FrameHeartbeat,
		Flags:    v1.FlagNone,
		StreamID: v1.StreamIDControl,
		Payload:  nil,
	}

	// Send frame in goroutine (non-blocking)
	go func() {
		err := conn.SendFrame(frame)
		if err != nil {
			t.Errorf("SendFrame failed: %v", err)
		}
	}()

	// Read frame from other end
	receivedFrame, err := v1.Decode(conn2)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if receivedFrame.Type != v1.FrameHeartbeat {
		t.Errorf("Expected FrameHeartbeat, got %d", receivedFrame.Type)
	}
}

