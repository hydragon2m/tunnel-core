package connection

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	v1 "github.com/hydragon2m/tunnel-protocol/go/v1"
)

// Connection đại diện cho 1 persistent connection từ agent
type Connection struct {
	ID            string
	Conn          Conn // net.Conn wrapper với timeout support
	AgentID       string
	AccountID     string // Tenant/Account ID
	Metadata      map[string]string
	CreatedAt     time.Time
	LastHeartbeat time.Time

	// Stream management
	streams      map[uint32]*Stream
	streamsMu    sync.RWMutex
	nextStreamID uint32

	// State
	ctx     context.Context
	cancel  context.CancelFunc
	state   ConnectionState
	stateMu sync.RWMutex

	// Callbacks
	onStateChange func(oldState, newState ConnectionState)

	manager *Manager // Reference to parent manager for callbacks
}

// Conn là interface cho network connection với timeout support
type Conn interface {
	Read(b []byte) (n int, err error)
	Write(b []byte) (n int, err error)
	Close() error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	RemoteAddr() net.Addr
}

// Stream đại diện cho 1 stream trên connection
type Stream struct {
	ID        uint32
	State     StreamState
	CreatedAt time.Time
	Metadata  map[string]string

	// Data channels
	dataIn  chan []byte
	closeCh chan struct{}

	conn *Connection // Reference to parent connection for writing
	mu   sync.RWMutex

	// Internal read buffer for Read interface
	readBuf []byte
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
	connections  map[string]*Connection // connectionID -> Connection
	accountConns map[string]int         // accountID -> count
	connsMu      sync.RWMutex

	// Config
	maxConnections           int
	maxConnectionsPerAccount int
	heartbeatTimeout         time.Duration

	// Callbacks
	onConnectionClosed      func(connID string)
	onConnectionStateChange func(connID string, oldState, newState ConnectionState)
	onStreamCreated         func(connID string, streamID uint32)
	onStreamClosed          func(connID string, streamID uint32)

	// Metrics & Observability
	onTraffic func(accountID string, bytesIn, bytesOut int64)
}

// NewManager tạo Connection Manager mới
func NewManager(maxConnections, maxConnectionsPerAccount int, heartbeatTimeout time.Duration) *Manager {
	if maxConnectionsPerAccount <= 0 {
		maxConnectionsPerAccount = maxConnections // Default to global limit if not set
	}
	return &Manager{
		connections:              make(map[string]*Connection),
		accountConns:             make(map[string]int),
		maxConnections:           maxConnections,
		maxConnectionsPerAccount: maxConnectionsPerAccount,
		heartbeatTimeout:         heartbeatTimeout,
	}
}

// RegisterConnection đăng ký connection mới từ agent
func (m *Manager) RegisterConnection(connID, agentID, accountID string, conn Conn, metadata map[string]string) (*Connection, error) {
	m.connsMu.Lock()
	defer m.connsMu.Unlock()

	// Check max connections
	if len(m.connections) >= m.maxConnections {
		return nil, ErrMaxConnections
	}

	// Check account limit
	if m.accountConns[accountID] >= m.maxConnectionsPerAccount {
		return nil, fmt.Errorf("max connections limit reached for account %s", accountID)
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
		AccountID:     accountID,
		Metadata:      metadata,
		CreatedAt:     time.Now(),
		LastHeartbeat: time.Now(),
		streams:       make(map[uint32]*Stream),
		nextStreamID:  1, // Start from 1, 0 is for control
		ctx:           ctx,
		cancel:        cancel,
		state:         StateInit,
		manager:       m,
	}

	// Setup state change callback
	c.onStateChange = func(oldState, newState ConnectionState) {
		m.connsMu.RLock()
		callback := m.onConnectionStateChange
		m.connsMu.RUnlock()
		if callback != nil {
			callback(connID, oldState, newState)
		}
	}

	c.state = StateConnected // Set initial state to Connected

	m.connections[connID] = c
	m.accountConns[accountID]++

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

// GetAllConnections returns all active connections
func (m *Manager) GetAllConnections() []*Connection {
	m.connsMu.RLock()
	defer m.connsMu.RUnlock()

	conns := make([]*Connection, 0, len(m.connections))
	for _, conn := range m.connections {
		conns = append(conns, conn)
	}
	return conns
}

// SetOnConnectionClosed set callback khi connection đóng
func (m *Manager) SetOnConnectionClosed(callback func(connID string)) {
	m.connsMu.Lock()
	defer m.connsMu.Unlock()
	m.onConnectionClosed = callback
}

// SetOnConnectionStateChange set callback khi connection state thay đổi
func (m *Manager) SetOnConnectionStateChange(callback func(connID string, oldState, newState ConnectionState)) {
	m.connsMu.Lock()
	defer m.connsMu.Unlock()
	m.onConnectionStateChange = callback
}

// SetOnTraffic set callback khi có traffic
func (m *Manager) SetOnTraffic(callback func(accountID string, bytesIn, bytesOut int64)) {
	m.connsMu.Lock()
	defer m.connsMu.Unlock()
	m.onTraffic = callback
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
		m.accountConns[conn.AccountID]--
		if m.accountConns[conn.AccountID] <= 0 {
			delete(m.accountConns, conn.AccountID)
		}
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

// DisconnectAccount closes all connections belonging to a specific account
func (m *Manager) DisconnectAccount(accountID string) int {
	m.connsMu.RLock()
	var toClose []string
	for connID, conn := range m.connections {
		if conn.AccountID == accountID {
			toClose = append(toClose, connID)
		}
	}
	m.connsMu.RUnlock()

	closedCount := 0
	for _, connID := range toClose {
		if err := m.CloseConnection(connID); err == nil {
			closedCount++
		}
	}
	return closedCount
}

// GracefulShutdown gracefully closes all connections with timeout
// Returns the number of connections that were forcefully closed due to timeout
func (m *Manager) GracefulShutdown(ctx context.Context) int {
	// Get all connections
	m.connsMu.RLock()
	connIDs := make([]string, 0, len(m.connections))
	for connID := range m.connections {
		connIDs = append(connIDs, connID)
	}
	m.connsMu.RUnlock()

	if len(connIDs) == 0 {
		return 0
	}

	// Create done channel for each connection
	doneCh := make(chan string, len(connIDs))

	// Close each connection gracefully
	for _, connID := range connIDs {
		go func(id string) {
			m.CloseConnection(id)
			doneCh <- id
		}(connID)
	}

	// Wait for all connections to close or timeout
	closed := 0
	timeout := time.NewTimer(0)
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.NewTimer(time.Until(deadline))
	} else {
		timeout = time.NewTimer(10 * time.Second) // Default 10s timeout
	}
	defer timeout.Stop()

	for closed < len(connIDs) {
		select {
		case <-doneCh:
			closed++
		case <-timeout.C:
			// Timeout - force close remaining connections
			remaining := len(connIDs) - closed
			return remaining
		case <-ctx.Done():
			// Context cancelled - force close remaining
			remaining := len(connIDs) - closed
			return remaining
		}
	}

	return 0 // All connections closed gracefully
}

// handleConnection xử lý frames từ connection
func (m *Manager) handleConnection(c *Connection) {
	// Ensure connection is cleaned up when this function exits
	defer m.CloseConnection(c.ID)

	// Set initial state
	if err := c.SetState(StateConnected); err != nil {
		c.Close()
		return
	}
	defer c.Close()

	// Heartbeat checker
	ticker := time.NewTicker(m.heartbeatTimeout / 2)
	defer ticker.Stop()

	// Frame reading goroutine
	frameCh := make(chan *v1.Frame, 10)
	go func() {
		defer c.Close()
		for {
			// Set read deadline
			c.Conn.SetReadDeadline(time.Now().Add(m.heartbeatTimeout))

			// 1. Read Length (4 bytes)
			length, err := v1.ReadFrameLength(c.Conn)
			if err != nil {
				return
			}

			// 2. Get Buffer from Pool
			buf := v1.GetBuffer(int(length))

			// 3. Read Frame Body
			if _, err := io.ReadFull(c.Conn, buf[:length]); err != nil {
				v1.PutBuffer(buf)
				return
			}

			// 4. Parse Frame
			frame, err := v1.ParseFrame(buf[:length])
			if err != nil {
				v1.PutBuffer(buf)
				return
			}

			// 5. Optimization: Handle heartbeat immediately to avoid false timeouts
			if frame.Type == v1.FrameHeartbeat {
				c.updateHeartbeat()
				// Send Heartbeat ACK back immediately if it's the Core
				// ACK: Version=1, Type=Heartbeat, Flags=Ack, StreamID=0
				ack := &v1.Frame{
					Version:  v1.Version,
					Type:     v1.FrameHeartbeat,
					Flags:    v1.FlagAck,
					StreamID: v1.StreamIDControl,
				}
				if err := v1.Encode(c.Conn, ack); err != nil {
					fmt.Printf("Warning: failed to send heartbeat ACK: %v\n", err)
				}
				v1.PutBuffer(buf)
				continue
			}

			// 6. Copy payload if needed for async processing and return buffer to pool
			if len(frame.Payload) > 0 {
				newPayload := make([]byte, len(frame.Payload))
				copy(newPayload, frame.Payload)
				frame.Payload = newPayload
			}
			v1.PutBuffer(buf)

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
			c.stateMu.RLock()
			lastHB := c.LastHeartbeat
			c.stateMu.RUnlock()
			if time.Since(lastHB) > m.heartbeatTimeout {
				return // Connection timeout
			}

		case frame := <-frameCh:
			// Handle frame
			if err := m.handleFrame(c, frame); err != nil {
				return // Protocol error
			}
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
		// Forward data to stream (Non-blocking to avoid HOL blocking)
		// We use a small timeout to ensure we don't block the entire connection
		// if one stream is slow, but also don't immediately drop packets.
		select {
		case stream.dataIn <- frame.Payload:
		case <-time.After(100 * time.Millisecond):
			// If stream channel is full for too long, we might need to drop or close
			// For now, log and continue to avoid blocking others.
			// Ideally, we'd implement flow control.
			fmt.Printf("Warning: stream %d data channel full, dropping frame to avoid HOL blocking\n", frame.StreamID)
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
		dataIn:    make(chan []byte, 100),
		closeCh:   make(chan struct{}),
		conn:      c,
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

// GetAllStreams returns all active streams on this connection
func (c *Connection) GetAllStreams() []*Stream {
	c.streamsMu.RLock()
	defer c.streamsMu.RUnlock()

	streams := make([]*Stream, 0, len(c.streams))
	for _, s := range c.streams {
		streams = append(streams, s)
	}
	return streams
}

// AllocateStreamID cấp phát ID mới cho stream
func (c *Connection) AllocateStreamID() uint32 {
	c.streamsMu.Lock()
	defer c.streamsMu.Unlock()

	streamID := c.nextStreamID
	c.nextStreamID++
	return streamID
}

// SendFrame gửi frame đến agent
func (c *Connection) SendFrame(frame *v1.Frame) error {
	c.stateMu.RLock()
	// Allow sending if Connected or Authenticated
	if c.state != StateConnected && c.state != StateAuthenticated {
		c.stateMu.RUnlock()
		return ErrConnectionClosed
	}
	if frame.Type == v1.FrameOpenStream {
		c.createStream(frame.StreamID)
	}
	c.stateMu.RUnlock()

	return v1.Encode(c.Conn, frame)
}

// Close đóng connection
func (c *Connection) Close() error {
	c.stateMu.Lock()
	if c.state == StateClosed || c.state == StateClosing {
		c.stateMu.Unlock()
		return nil
	}
	// Transition to Closing? Or directly Closed?
	// For simplicity, goes to Closed, but if we have async cleanup, Closing is better.
	// Since Close() does synchronous cleanup here, we can set Closed immediately or Closing then Closed.
	c.state = StateClosed
	c.stateMu.Unlock()

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
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
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
func (s *Stream) DataIn() chan<- []byte {
	return s.dataIn
}

// Read implements io.Reader
func (s *Stream) Read(p []byte) (n int, err error) {
	if len(s.readBuf) > 0 {
		n = copy(p, s.readBuf)
		s.readBuf = s.readBuf[n:]
		return n, nil
	}

	select {
	case data, ok := <-s.dataIn:
		if !ok {
			return 0, io.EOF
		}
		n = copy(p, data)
		if n < len(data) {
			s.readBuf = data[n:]
		}

		// Report traffic (dataIn is ingress from agent -> server -> end-user, so it's BytesIn from agent perspective, but BytesOut from server perspective?)
		// Let's clarify: BytesIn = end-user -> server -> agent. BytesOut = agent -> server -> end-user.
		// Stream.Read is called by Router to read from agent and write to end-user. So this is data from agent (BytesOut).
		if s.conn.manager != nil && s.conn.manager.onTraffic != nil {
			s.conn.manager.onTraffic(s.conn.AccountID, 0, int64(n))
		}

		return n, nil
	case <-s.closeCh:
		return 0, io.EOF
	case <-s.conn.ctx.Done():
		return 0, s.conn.ctx.Err()
	}
}

// Write implements io.Writer
func (s *Stream) Write(p []byte) (n int, err error) {
	// Send FrameData
	frame := &v1.Frame{
		Version:  v1.Version,
		Type:     v1.FrameData,
		Flags:    v1.FlagNone,
		StreamID: s.ID,
		Payload:  p,
	}

	if err := s.conn.SendFrame(frame); err != nil {
		return 0, err
	}

	// Report traffic: Stream.Write is called by Router to write data from end-user to agent (BytesIn).
	if s.conn.manager != nil && s.conn.manager.onTraffic != nil {
		s.conn.manager.onTraffic(s.conn.AccountID, int64(len(p)), 0)
	}

	return len(p), nil
}

// Close implements io.Closer
func (s *Stream) Close() error {
	// Send EndStream frame
	frame := &v1.Frame{
		Version:  v1.Version,
		Type:     v1.FrameData,
		Flags:    v1.FlagEndStream,
		StreamID: s.ID,
		Payload:  nil,
	}
	return s.conn.SendFrame(frame)
}

// CloseCh returns the close channel
func (s *Stream) CloseCh() <-chan struct{} {
	return s.closeCh
}

// SetState transitions the connection to a new state
func (c *Connection) SetState(newState ConnectionState) error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	if c.state == newState {
		return nil
	}

	if !IsValidTransition(c.state, newState) {
		return fmt.Errorf("invalid state transition from %s to %s", c.state, newState)
	}

	c.state = newState

	// Trigger callback explicitly outside the lock to avoid deadlocks?
	// Ideally yes, but here we just hold stateMu. callback calls Manager which holds connsMu.
	// If Manager callbacks call back into Connection methods that need stateMu, we deadlock.
	// Safe practice: define callback to NOT call back into Connection synchronosly or be careful.
	// For now, simple invocation.
	if c.onStateChange != nil {
		c.onStateChange(c.state, newState)
	}

	return nil
}

// GetState returns the current connection state
func (c *Connection) GetState() ConnectionState {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.state
}

// Context returns context for connection (for cancellation)
func (c *Connection) Context() context.Context {
	return c.ctx
}
