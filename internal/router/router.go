package router

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hydragon2m/tunnel-protocol/go/v1"
	"github.com/hydragon2m/tunnel-core/internal/connection"
	"github.com/hydragon2m/tunnel-core/internal/quota"
	"github.com/hydragon2m/tunnel-core/internal/registry"
)

// Router route HTTP requests đến agent connections
type Router struct {
	registry    *registry.Registry
	connManager *connection.Manager
	limiter     *quota.Limiter
	timeout     time.Duration
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

// ServeHTTP implements http.Handler
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
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
	if err := r.handleRequest(ctx, conn, streamID, w, req); err != nil {
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
	// Build request payload (simplified - can be enhanced with full HTTP serialization)
	requestData := r.buildRequestPayload(req)

	// Send FrameOpenStream
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

	// Get stream
	stream, ok := conn.GetStream(streamID)
	if !ok {
		return fmt.Errorf("stream not found after creation")
	}

	// Forward request body if present
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("failed to read request body: %w", err)
		}

		if len(body) > 0 {
			dataFrame := &v1.Frame{
				Version:  v1.Version,
				Type:     v1.FrameData,
				Flags:    v1.FlagNone,
				StreamID: streamID,
				Payload:  body,
			}

			if err := conn.SendFrame(dataFrame); err != nil {
				return fmt.Errorf("failed to send request body: %w", err)
			}
		}
	}

	// Send EndStream flag to indicate request complete
	endFrame := &v1.Frame{
		Version:  v1.Version,
		Type:     v1.FrameData,
		Flags:    v1.FlagEndStream,
		StreamID: streamID,
		Payload:  nil,
	}

	if err := conn.SendFrame(endFrame); err != nil {
		return fmt.Errorf("failed to send end stream frame: %w", err)
	}

	// Wait for response from stream
	return r.waitForResponse(ctx, stream, streamID, w)
}

// buildRequestPayload builds request payload from HTTP request
func (r *Router) buildRequestPayload(req *http.Request) []byte {
	// Simplified payload - can be enhanced with full HTTP/1.1 serialization
	// Format: "METHOD PATH HTTP/1.1\r\nHeaders\r\n\r\n"
	var buf bytes.Buffer

	// Request line
	buf.WriteString(fmt.Sprintf("%s %s %s\r\n", req.Method, req.URL.Path, req.Proto))

	// Headers
	for key, values := range req.Header {
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
	streamID uint32,
	w http.ResponseWriter,
) error {
	// Read response data from stream
	responseData := make([]byte, 0)
	streamClosed := false

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case data, ok := <-stream.DataIn():
			if !ok {
				streamClosed = true
				break
			}
			responseData = append(responseData, data...)

		case <-stream.CloseCh():
			streamClosed = true
			break
		}

		if streamClosed {
			break
		}
	}

	// Parse and write response (simplified - assumes response is already HTTP formatted)
	// In production, should parse HTTP response from agent
	if len(responseData) > 0 {
		// For now, just write raw response
		// TODO: Parse HTTP response headers and status
		w.WriteHeader(http.StatusOK)
		w.Write(responseData)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}

	return nil
}

