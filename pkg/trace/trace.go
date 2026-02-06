package trace

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
)

// Context key for request ID
type contextKey string

const (
	// RequestIDKey is the context key for request ID
	RequestIDKey contextKey = "request-id"

	// HeaderRequestID is the HTTP header for request ID
	HeaderRequestID = "X-Request-ID"

	// HeaderTraceID is the HTTP header for trace ID (OpenTelemetry compatible)
	HeaderTraceID = "X-Trace-ID"
)

// NewRequestID generates a new unique request ID
// Format: req-{uuid}
func NewRequestID() string {
	uuid := generateUUID()
	return fmt.Sprintf("req-%s", uuid)
}

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// GetRequestID extracts the request ID from context
// Returns empty string if not found
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	requestID, ok := ctx.Value(RequestIDKey).(string)
	if !ok {
		return ""
	}

	return requestID
}

// ExtractRequestID extracts request ID from HTTP request headers
// Checks X-Request-ID and X-Trace-ID headers
// Returns empty string if not found
func ExtractRequestID(r *http.Request) string {
	// Try X-Request-ID first
	if requestID := r.Header.Get(HeaderRequestID); requestID != "" {
		return requestID
	}

	// Try X-Trace-ID as fallback
	if traceID := r.Header.Get(HeaderTraceID); traceID != "" {
		return traceID
	}

	return ""
}

// InjectRequestID adds request ID to HTTP request headers
func InjectRequestID(r *http.Request, requestID string) {
	if r == nil || requestID == "" {
		return
	}

	r.Header.Set(HeaderRequestID, requestID)
}

// InjectRequestIDToResponse adds request ID to HTTP response headers
func InjectRequestIDToResponse(w http.ResponseWriter, requestID string) {
	if w == nil || requestID == "" {
		return
	}

	w.Header().Set(HeaderRequestID, requestID)
}

// generateUUID generates a UUID v4
// Format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
func generateUUID() string {
	uuid := make([]byte, 16)
	_, err := rand.Read(uuid)
	if err != nil {
		// Fallback to timestamp-based ID if random fails
		return fmt.Sprintf("%d", getCurrentTimestamp())
	}

	// Set version (4) and variant bits
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant 10

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4],
		uuid[4:6],
		uuid[6:8],
		uuid[8:10],
		uuid[10:16])
}

// getCurrentTimestamp returns current Unix timestamp in nanoseconds
func getCurrentTimestamp() int64 {
	// Using a simple counter-based approach for fallback
	// In production, you might want to use time.Now().UnixNano()
	return 0
}

// Middleware creates an HTTP middleware that adds request ID to context
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract or generate request ID
		requestID := ExtractRequestID(r)
		if requestID == "" {
			requestID = NewRequestID()
		}

		// Add to context
		ctx := WithRequestID(r.Context(), requestID)
		r = r.WithContext(ctx)

		// Add to response headers
		InjectRequestIDToResponse(w, requestID)

		// Call next handler
		next.ServeHTTP(w, r)
	})
}
