package trace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRequestID(t *testing.T) {
	id := NewRequestID()

	if id == "" {
		t.Error("Request ID should not be empty")
	}

	if !strings.HasPrefix(id, "req-") {
		t.Errorf("Request ID should start with 'req-', got: %s", id)
	}

	// Should be unique
	id2 := NewRequestID()
	if id == id2 {
		t.Error("Request IDs should be unique")
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "test-request-123"

	ctx = WithRequestID(ctx, requestID)

	extracted := GetRequestID(ctx)
	if extracted != requestID {
		t.Errorf("Expected %s, got %s", requestID, extracted)
	}
}

func TestGetRequestID_Empty(t *testing.T) {
	ctx := context.Background()

	requestID := GetRequestID(ctx)
	if requestID != "" {
		t.Errorf("Expected empty string, got %s", requestID)
	}
}

func TestGetRequestID_Nil(t *testing.T) {
	requestID := GetRequestID(nil)
	if requestID != "" {
		t.Errorf("Expected empty string for nil context, got %s", requestID)
	}
}

func TestExtractRequestID_FromHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(HeaderRequestID, "req-from-header")

	requestID := ExtractRequestID(req)
	if requestID != "req-from-header" {
		t.Errorf("Expected 'req-from-header', got %s", requestID)
	}
}

func TestExtractRequestID_FromTraceHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(HeaderTraceID, "trace-123")

	requestID := ExtractRequestID(req)
	if requestID != "trace-123" {
		t.Errorf("Expected 'trace-123', got %s", requestID)
	}
}

func TestExtractRequestID_Priority(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(HeaderRequestID, "req-id")
	req.Header.Set(HeaderTraceID, "trace-id")

	// X-Request-ID should take priority
	requestID := ExtractRequestID(req)
	if requestID != "req-id" {
		t.Errorf("Expected 'req-id', got %s", requestID)
	}
}

func TestExtractRequestID_Empty(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	requestID := ExtractRequestID(req)
	if requestID != "" {
		t.Errorf("Expected empty string, got %s", requestID)
	}
}

func TestInjectRequestID(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	InjectRequestID(req, "req-injected")

	extracted := req.Header.Get(HeaderRequestID)
	if extracted != "req-injected" {
		t.Errorf("Expected 'req-injected', got %s", extracted)
	}
}

func TestInjectRequestID_Nil(t *testing.T) {
	// Should not panic
	InjectRequestID(nil, "req-123")
}

func TestInjectRequestID_Empty(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	// Should not add header if ID is empty
	InjectRequestID(req, "")

	extracted := req.Header.Get(HeaderRequestID)
	if extracted != "" {
		t.Errorf("Expected empty header, got %s", extracted)
	}
}

func TestInjectRequestIDToResponse(t *testing.T) {
	w := httptest.NewRecorder()

	InjectRequestIDToResponse(w, "req-response")

	extracted := w.Header().Get(HeaderRequestID)
	if extracted != "req-response" {
		t.Errorf("Expected 'req-response', got %s", extracted)
	}
}

func TestInjectRequestIDToResponse_Nil(t *testing.T) {
	// Should not panic
	InjectRequestIDToResponse(nil, "req-123")
}

func TestMiddleware_GeneratesID(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		if requestID == "" {
			t.Error("Request ID should be in context")
		}

		if !strings.HasPrefix(requestID, "req-") {
			t.Errorf("Request ID should start with 'req-', got: %s", requestID)
		}

		w.WriteHeader(http.StatusOK)
	})

	middleware := Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	// Check response header
	responseID := w.Header().Get(HeaderRequestID)
	if responseID == "" {
		t.Error("Response should have X-Request-ID header")
	}
}

func TestMiddleware_PreservesExistingID(t *testing.T) {
	existingID := "req-existing-123"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		if requestID != existingID {
			t.Errorf("Expected %s, got %s", existingID, requestID)
		}

		w.WriteHeader(http.StatusOK)
	})

	middleware := Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(HeaderRequestID, existingID)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	// Check response header
	responseID := w.Header().Get(HeaderRequestID)
	if responseID != existingID {
		t.Errorf("Expected %s in response, got %s", existingID, responseID)
	}
}

func TestGenerateUUID(t *testing.T) {
	uuid := generateUUID()

	if uuid == "" {
		t.Error("UUID should not be empty")
	}

	// Check format (should have 4 hyphens)
	parts := strings.Split(uuid, "-")
	if len(parts) != 5 {
		t.Errorf("UUID should have 5 parts, got %d: %s", len(parts), uuid)
	}

	// Should be unique
	uuid2 := generateUUID()
	if uuid == uuid2 {
		t.Error("UUIDs should be unique")
	}
}

func BenchmarkNewRequestID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewRequestID()
	}
}

func BenchmarkWithRequestID(b *testing.B) {
	ctx := context.Background()
	requestID := "req-benchmark"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WithRequestID(ctx, requestID)
	}
}

func BenchmarkGetRequestID(b *testing.B) {
	ctx := WithRequestID(context.Background(), "req-benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetRequestID(ctx)
	}
}

func BenchmarkMiddleware(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	middleware := Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
	}
}
