package connection

import (
	"context"
	"sync"
	"time"

	v1 "github.com/hydragon2m/tunnel-protocol/go/v1"
)

// Connection đại diện cho 1 persistent connection từ agent
type Connection struct {
	ID            string
	Conn          Conn // net.Conn wrapper với timeout support
	AgentID       string
	Metadata      map[string]string
	CreatedAt     time.Time
	LastHeartbeat time.Time

	// Stream management
	streams      map[uint32]*Stream
	streamsMu    sync.RWMutex
	nextStreamID uint32

	// State
	ctx      context.Context
	cancel   context.CancelFunc
	closed   bool
	closedMu sync.RWMutex
}

// Conn là interface cho network connection với timeout support
type Conn interface {
	Read(b []byte) (n int, err error)
	Write(b []byte) (n int, err error)
	Close() error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	RemoteAddr() string
}

// Stream đại diện cho 1 stream trên connection
type Stream struct {
	ID        uint32
	State     StreamState
	CreatedAt time.Time
	Metadata  map[string]string

	// Data channels
	dataIn  chan []byte
	dataOut chan []byte
	closeCh chan struct{}

	mu sync.RWMutex
}

// StreamState là state của stream
type StreamState int

const (
	StreamStateInit StreamState = iota
	StreamStateOpen
	StreamStateData
	StreamStateClosed
	StreamStateError
)

// Manager quản lý tất cả connections từ agents
type Manager struct {
	connections map[string]*Connection // agentID -> Connection
	connsMu     sync.RWMutex

	// Config
	maxConnections   int
	heartbeatTimeout time.Duration

	// Callbacks
	onConnectionClosed func(connID string)
	onStreamCreated    func(connID string, streamID uint32)
	onStreamClosed     func(connID string, streamID uint32)
}

// NewManager tạo Connection Manager mới
func NewManager(maxConnections int, heartbeatTimeout time.Duration) *Manager {
	return &Manager{
		connections:      make(map[string]*Connection),
		maxConnections:   maxConnections,
		heartbeatTimeout: heartbeatTimeout,
	}
}

// RegisterConnection đăng ký connection mới từ agent
func (m *Manager) RegisterConnection(connID, agentID string, conn Conn, metadata map[string]string) (*Connection, error) {
	m.connsMu.Lock()
	defer m.connsMu.Unlock()

	// Check max connections
	if len(m.connections) >= m.maxConnections {
		return nil, ErrMaxConnections
	}

	// Check duplicate
	if _, exists := m.connections[connID]; exists {
		return nil, ErrConnectionExists
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Connection{
		ID:            connID,
		Conn:          conn,
		AgentID:       agentID,
		Metadata:      metadata,
		CreatedAt:     time.Now(),
		LastHeartbeat: time.Now(),
		streams:       make(map[uint32]*Stream),
		nextStreamID:  1, // Start from 1, 0 is for control
		ctx:           ctx,
		cancel:        cancel,
	}

	m.connections[connID] = c

	// Start connection handler
	go m.handleConnection(c)

	return c, nil
}

// GetConnection lấy connection theo ID
func (m *Manager) GetConnection(connID string) (*Connection, bool) {
	m.connsMu.RLock()
	defer m.connsMu.RUnlock()

	conn, ok := m.connections[connID]
	return conn, ok
}

// GetConnectionByAgentID lấy connection theo agent ID
func (m *Manager) GetConnectionByAgentID(agentID string) (*Connection, bool) {
	m.connsMu.RLock()
	defer m.connsMu.RUnlock()

	for _, conn := range m.connections {
		if conn.AgentID == agentID {
			return conn, true
		}
	}
	return nil, false
}

// SetOnConnectionClosed set callback khi connection đóng
func (m *Manager) SetOnConnectionClosed(callback func(connID string)) {
	m.connsMu.Lock()
	defer m.connsMu.Unlock()
	m.onConnectionClosed = callback
}

// SetOnStreamCreated set callback khi stream được tạo
func (m *Manager) SetOnStreamCreated(callback func(connID string, streamID uint32)) {
	m.connsMu.Lock()
	defer m.connsMu.Unlock()
	m.onStreamCreated = callback
}

// SetOnStreamClosed set callback khi stream đóng
func (m *Manager) SetOnStreamClosed(callback func(connID string, streamID uint32)) {
	m.connsMu.Lock()
	defer m.connsMu.Unlock()
	m.onStreamClosed = callback
}

// CloseConnection đóng connection và cleanup
func (m *Manager) CloseConnection(connID string) error {
	m.connsMu.Lock()
	conn, exists := m.connections[connID]
	if exists {
		delete(m.connections, connID)
	}
	m.connsMu.Unlock()

	if !exists {
		return ErrConnectionNotFound
	}

	conn.Close()

	if m.onConnectionClosed != nil {
		m.onConnectionClosed(connID)
	}

	return nil
}

// handleConnection xử lý frames từ connection
func (m *Manager) handleConnection(c *Connection) {
	defer c.Close()

	// Heartbeat checker
	ticker := time.NewTicker(m.heartbeatTimeout / 2)
	defer ticker.Stop()

	// Frame reading goroutine
	frameCh := make(chan *v1.Frame, 10)
	errCh := make(chan error, 1)

	go func() {
		for {
			// Set read deadline để tránh block vô hạn
			c.Conn.SetReadDeadline(time.Now().Add(m.heartbeatTimeout))

			// Decode frame
			frame, err := v1.Decode(c.Conn)
			if err != nil {
				errCh <- err
				return
			}

			select {
			case frameCh <- frame:
			case <-c.ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-c.ctx.Done():
			return

		case <-ticker.C:
			// Check heartbeat timeout
			if time.Since(c.LastHeartbeat) > m.heartbeatTimeout {
				return // Connection timeout
			}

		case frame := <-frameCh:
			// Handle frame
			if err := m.handleFrame(c, frame); err != nil {
				return // Protocol error
			}

		case err := <-errCh:
			// Connection error
			_ = err
			return
		}
	}
}

// handleFrame xử lý frame từ connection
func (m *Manager) handleFrame(c *Connection, frame *v1.Frame) error {
	// Control frames (StreamID = 0)
	if frame.IsControlFrame() {
		return m.handleControlFrame(c, frame)
	}

	// Data stream frames (StreamID > 0)
	return m.handleStreamFrame(c, frame)
}

// handleControlFrame xử lý control frames
func (m *Manager) handleControlFrame(c *Connection, frame *v1.Frame) error {
	switch frame.Type {
	case v1.FrameAuth:
		// Auth đã được xử lý ở handshake, chỉ update heartbeat
		c.updateHeartbeat()
		return nil

	case v1.FrameHeartbeat:
		c.updateHeartbeat()
		return nil

	case v1.FrameClose:
		// Agent muốn close connection
		return ErrConnectionClosedByAgent

	default:
		return ErrInvalidControlFrame
	}
}

// handleStreamFrame xử lý stream frames
func (m *Manager) handleStreamFrame(c *Connection, frame *v1.Frame) error {
	c.streamsMu.Lock()
	stream, exists := c.streams[frame.StreamID]
	c.streamsMu.Unlock()

	switch frame.Type {
	case v1.FrameOpenStream:
		if exists {
			return ErrStreamExists
		}
		// Create new stream
		stream = c.createStream(frame.StreamID)
		if m.onStreamCreated != nil {
			m.onStreamCreated(c.ID, frame.StreamID)
		}

	case v1.FrameData:
		if !exists {
			return ErrStreamNotFound
		}
		// Forward data to stream
		select {
		case stream.dataIn <- frame.Payload:
		case <-stream.closeCh:
			return ErrStreamClosed
		case <-c.ctx.Done():
			return c.ctx.Err()
		}

		// Check EndStream flag
		if frame.IsEndStream() {
			stream.setState(StreamStateClosed)
			c.closeStream(frame.StreamID)
			if m.onStreamClosed != nil {
				m.onStreamClosed(c.ID, frame.StreamID)
			}
		}

	case v1.FrameClose:
		if !exists {
			return nil // Already closed
		}
		stream.setState(StreamStateClosed)
		c.closeStream(frame.StreamID)
		if m.onStreamClosed != nil {
			m.onStreamClosed(c.ID, frame.StreamID)
		}

	default:
		return ErrInvalidStreamFrame
	}

	return nil
}

// createStream tạo stream mới trên connection
func (c *Connection) createStream(streamID uint32) *Stream {
	c.streamsMu.Lock()
	defer c.streamsMu.Unlock()

	stream := &Stream{
		ID:        streamID,
		State:     StreamStateInit,
		CreatedAt: time.Now(),
		Metadata:  make(map[string]string),
		dataIn:    make(chan []byte, 10),
		dataOut:   make(chan []byte, 10),
		closeCh:   make(chan struct{}),
	}

	c.streams[streamID] = stream
	return stream
}

// closeStream đóng stream và cleanup
func (c *Connection) closeStream(streamID uint32) {
	c.streamsMu.Lock()
	defer c.streamsMu.Unlock()

	stream, exists := c.streams[streamID]
	if !exists {
		return
	}

	close(stream.closeCh)
	delete(c.streams, streamID)
}

// GetStream lấy stream theo ID
func (c *Connection) GetStream(streamID uint32) (*Stream, bool) {
	c.streamsMu.RLock()
	defer c.streamsMu.RUnlock()

	stream, ok := c.streams[streamID]
	return stream, ok
}

// AllocateStreamID cấp phát stream ID mới
func (c *Connection) AllocateStreamID() uint32 {
	c.streamsMu.Lock()
	defer c.streamsMu.Unlock()

	streamID := c.nextStreamID
	c.nextStreamID++
	return streamID
}

// SendFrame gửi frame đến agent
func (c *Connection) SendFrame(frame *v1.Frame) error {
	c.closedMu.RLock()
	if c.closed {
		c.closedMu.RUnlock()
		return ErrConnectionClosed
	}
	c.closedMu.RUnlock()

	return v1.Encode(c.Conn, frame)
}

// Close đóng connection
func (c *Connection) Close() error {
	c.closedMu.Lock()
	if c.closed {
		c.closedMu.Unlock()
		return nil
	}
	c.closed = true
	c.closedMu.Unlock()

	c.cancel()

	// Close all streams
	c.streamsMu.Lock()
	for streamID := range c.streams {
		c.closeStream(streamID)
	}
	c.streamsMu.Unlock()

	return c.Conn.Close()
}

// updateHeartbeat cập nhật heartbeat timestamp
func (c *Connection) updateHeartbeat() {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()
	c.LastHeartbeat = time.Now()
}

// setState set state của stream
func (s *Stream) setState(state StreamState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = state
}

// GetState lấy state của stream
func (s *Stream) GetState() StreamState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

// DataIn returns the data input channel
func (s *Stream) DataIn() <-chan []byte {
	return s.dataIn
}

// CloseCh returns the close channel
func (s *Stream) CloseCh() <-chan struct{} {
	return s.closeCh
}

// Context returns context for connection (for cancellation)
func (c *Connection) Context() context.Context {
	return c.ctx
}
