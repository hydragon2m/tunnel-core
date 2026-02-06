package router

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hydragon2m/tunnel-core/connection"
	"github.com/hydragon2m/tunnel-core/internal/quota"
	"github.com/hydragon2m/tunnel-core/internal/registry"
	v1 "github.com/hydragon2m/tunnel-protocol/go/v1"
)

// Router route HTTP requests đến agent connections
type Router struct {
	registry    *registry.Registry
	connManager *connection.Manager
	limiter     *quota.Limiter
	timeout     time.Duration
	onRequest   func(accountID string, duration time.Duration, success bool)
}

// NewRouter tạo Router mới
func NewRouter(reg *registry.Registry, connManager *connection.Manager, limiter *quota.Limiter, timeout time.Duration) *Router {
	return &Router{
		registry:    reg,
		connManager: connManager,
		limiter:     limiter,
		timeout:     timeout,
	}
}

// SetOnRequest đặt callback khi xử lý xong request
func (r *Router) SetOnRequest(callback func(accountID string, duration time.Duration, success bool)) {
	r.onRequest = callback
}

// ServeHTTP implements http.Handler
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Health check endpoint
	if req.URL.Path == "/health" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}

	// Extract domain from Host header
	host := req.Host
	if host == "" {
		http.Error(w, "Missing Host header", http.StatusBadRequest)
		return
	}

	// Lookup tunnel
	tunnel, ok := r.registry.GetTunnel(host)
	if !ok {
		http.Error(w, fmt.Sprintf("Tunnel not found for domain: %s", host), http.StatusNotFound)
		return
	}

	// Check quota/rate limits
	if r.limiter != nil {
		if err := r.limiter.CheckRequest(tunnel.AgentID, host); err != nil {
			http.Error(w, fmt.Sprintf("Rate limit exceeded: %v", err), http.StatusTooManyRequests)
			return
		}
	}

	// Get connection
	conn, ok := r.connManager.GetConnection(tunnel.ConnectionID)
	if !ok {
		http.Error(w, "Connection not found", http.StatusServiceUnavailable)
		return
	}

	// Acquire stream quota
	if r.limiter != nil {
		if err := r.limiter.AcquireStream(tunnel.AgentID, host); err != nil {
			http.Error(w, fmt.Sprintf("Stream limit exceeded: %v", err), http.StatusTooManyRequests)
			return
		}
		// Release stream quota when done
		defer r.limiter.ReleaseStream(tunnel.AgentID, host)
	}

	// Create new stream
	streamID := conn.AllocateStreamID()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	// Handle request
	start := time.Now()
	err := r.handleRequest(ctx, conn, streamID, w, req)
	duration := time.Since(start)

	success := (err == nil)
	if r.onRequest != nil {
		r.onRequest(conn.AccountID, duration, success)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// handleRequest handles a single HTTP request
func (r *Router) handleRequest(
	ctx context.Context,
	conn *connection.Connection,
	streamID uint32,
	w http.ResponseWriter,
	req *http.Request,
) error {
	// 1. Build request payload (Headers)
	requestData := r.buildRequestPayload(req)

	// 2. Open Stream with Headers
	openFrame := &v1.Frame{
		Version:  v1.Version,
		Type:     v1.FrameOpenStream,
		Flags:    v1.FlagNone,
		StreamID: streamID,
		Payload:  requestData,
	}

	if err := conn.SendFrame(openFrame); err != nil {
		return fmt.Errorf("failed to send open stream frame: %w", err)
	}

	// 3. Get stream
	stream, ok := conn.GetStream(streamID)
	if !ok {
		return fmt.Errorf("stream not found after creation")
	}
	defer stream.Close()

	// 4. Stream request body if present (Async)
	if req.Body != nil {
		go func() {
			// io.Copy will call Stream.Write which sends FrameData
			if _, copyErr := io.Copy(stream, req.Body); copyErr != nil {
				// Log error but don't fail the request - response might still be valid
				fmt.Printf("Warning: failed to copy request body: %v\n", copyErr)
			}
			if closeErr := req.Body.Close(); closeErr != nil {
				fmt.Printf("Warning: failed to close request body: %v\n", closeErr)
			}
			// EndStream flag is sent by stream.Close() via defer above
		}()
	}

	// 5. Wait for response and stream back to client
	return r.waitForResponse(ctx, stream, w)
}

// buildRequestPayload builds request payload from HTTP request
func (r *Router) buildRequestPayload(req *http.Request) []byte {
	var buf bytes.Buffer
	// Request line
	buf.WriteString(fmt.Sprintf("%s %s %s\r\n", req.Method, req.URL.RequestURI(), req.Proto))
	// Host header (stored separately in http.Request)
	buf.WriteString(fmt.Sprintf("Host: %s\r\n", req.Host))
	// Headers
	for key, values := range req.Header {
		if key == "Host" {
			continue
		}
		for _, value := range values {
			buf.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
		}
	}
	buf.WriteString("\r\n")
	return buf.Bytes()
}

// waitForResponse waits for response from stream and writes to HTTP response
func (r *Router) waitForResponse(
	ctx context.Context,
	stream *connection.Stream,
	w http.ResponseWriter,
) error {
	// 1. Create a buffered reader to parse the response
	reader := bufio.NewReader(stream)

	// 2. Parse HTTP response from stream
	// Note: The agent sends a full HTTP response (Response line + Headers + Body)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		return fmt.Errorf("failed to read response from agent: %w", err)
	}
	defer resp.Body.Close()

	// 3. Copy headers from agent response to public response
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	// 4. Set status code
	w.WriteHeader(resp.StatusCode)

	// 5. Stream response body back to the end-user
	_, err = io.Copy(w, resp.Body)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to stream response body: %w", err)
	}

	return nil
}
