package errors

import (
	"fmt"
	"runtime"
	"strings"
)

// Error codes
const (
	// Connection errors
	ErrCodeConnectionFailed = "ERR_CONNECTION_FAILED"
	ErrCodeConnectionClosed = "ERR_CONNECTION_CLOSED"
	ErrCodeConnectionLimit  = "ERR_CONNECTION_LIMIT"
	ErrCodeAccountLimit     = "ERR_ACCOUNT_LIMIT"
	ErrCodeAuthFailed       = "ERR_AUTH_FAILED"

	// Stream errors
	ErrCodeStreamNotFound = "ERR_STREAM_NOT_FOUND"
	ErrCodeStreamExists   = "ERR_STREAM_EXISTS"
	ErrCodeStreamLimit    = "ERR_STREAM_LIMIT"
	ErrCodeStreamClosed   = "ERR_STREAM_CLOSED"

	// Protocol errors
	ErrCodeFrameTooLarge  = "ERR_FRAME_TOO_LARGE"
	ErrCodeBadFrame       = "ERR_BAD_FRAME"
	ErrCodeInvalidVersion = "ERR_INVALID_VERSION"
	ErrCodeEncodeFailed   = "ERR_ENCODE_FAILED"
	ErrCodeDecodeFailed   = "ERR_DECODE_FAILED"

	// Request errors
	ErrCodeRequestTooLarge   = "ERR_REQUEST_TOO_LARGE"
	ErrCodeRequestTimeout    = "ERR_REQUEST_TIMEOUT"
	ErrCodeRateLimitExceeded = "ERR_RATE_LIMIT_EXCEEDED"
	ErrCodeTunnelNotFound    = "ERR_TUNNEL_NOT_FOUND"

	// Internal errors
	ErrCodeInternal      = "ERR_INTERNAL"
	ErrCodeConfigInvalid = "ERR_CONFIG_INVALID"
)

// TunnelError represents a structured error with code, context, and stack trace
type TunnelError struct {
	Code    string
	Message string
	Cause   error
	Context map[string]interface{}
	Stack   string
}

// Error implements the error interface
func (e *TunnelError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause error
func (e *TunnelError) Unwrap() error {
	return e.Cause
}

// New creates a new TunnelError with the given code and message
func New(code, message string) *TunnelError {
	return &TunnelError{
		Code:    code,
		Message: message,
		Context: make(map[string]interface{}),
		Stack:   captureStack(2),
	}
}

// Wrap wraps an existing error with a TunnelError
func Wrap(err error, code, message string) *TunnelError {
	if err == nil {
		return nil
	}
	return &TunnelError{
		Code:    code,
		Message: message,
		Cause:   err,
		Context: make(map[string]interface{}),
		Stack:   captureStack(2),
	}
}

// WithContext adds context to the error
func (e *TunnelError) WithContext(key string, value interface{}) *TunnelError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithContextMap adds multiple context values
func (e *TunnelError) WithContextMap(ctx map[string]interface{}) *TunnelError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	for k, v := range ctx {
		e.Context[k] = v
	}
	return e
}

// captureStack captures the current stack trace
func captureStack(skip int) string {
	const maxDepth = 10
	var pcs [maxDepth]uintptr
	n := runtime.Callers(skip+1, pcs[:])

	frames := runtime.CallersFrames(pcs[:n])
	var builder strings.Builder

	for {
		frame, more := frames.Next()
		if !strings.Contains(frame.File, "runtime/") {
			fmt.Fprintf(&builder, "%s:%d %s\n", frame.File, frame.Line, frame.Function)
		}
		if !more {
			break
		}
	}

	return builder.String()
}

// IsCode checks if an error has a specific error code
func IsCode(err error, code string) bool {
	if te, ok := err.(*TunnelError); ok {
		return te.Code == code
	}
	return false
}

// GetCode returns the error code from an error, or empty string if not a TunnelError
func GetCode(err error) string {
	if te, ok := err.(*TunnelError); ok {
		return te.Code
	}
	return ""
}
