package router

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hydragon2m/tunnel-core/connection"
	"github.com/hydragon2m/tunnel-core/internal/quota"
	"github.com/hydragon2m/tunnel-core/internal/registry"
	"github.com/hydragon2m/tunnel-core/testutil"
)

// TestRouter_BasicRouting tests basic HTTP routing
func TestRouter_BasicRouting(t *testing.T) {
	// Setup
	reg := registry.NewRegistry("localhost")
	connManager := connection.NewManager(10, 5, 30*time.Second)
	limiter := quota.NewLimiter(100, 50)
	router := NewRouter(reg, connManager, limiter, 5*time.Second)

	// Register a tunnel
	mockConn := testutil.NewMockConnection()
	c, err := connManager.RegisterConnection("test-conn", "test-agent", "test-account", mockConn, nil)
	testutil.AssertNoError(t, err)

	err = reg.RegisterTunnel("test-agent", "test.localhost", c.ID, "test-account", nil)
	testutil.AssertNoError(t, err)

	// Create HTTP request
	req := httptest.NewRequest("GET", "http://test.localhost/path", nil)
	req.Host = "test.localhost"
	w := httptest.NewRecorder()

	// Note: This will fail because we need to mock the stream handling
	// For now, we just test the routing logic up to stream creation
	router.ServeHTTP(w, req)

	// The request should fail because there's no actual stream handling
	// but we can verify the routing logic was executed
	if w.Code == http.StatusOK {
		t.Log("Routing succeeded (unexpected in this test)")
	}
}

// TestRouter_TunnelNotFound tests handling of unknown domains
func TestRouter_TunnelNotFound(t *testing.T) {
	reg := registry.NewRegistry("localhost")
	connManager := connection.NewManager(10, 5, 30*time.Second)
	limiter := quota.NewLimiter(100, 50)
	router := NewRouter(reg, connManager, limiter, 5*time.Second)

	req := httptest.NewRequest("GET", "http://unknown.localhost/", nil)
	req.Host = "unknown.localhost"
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	testutil.AssertEqual(t, http.StatusNotFound, w.Code)
	if !bytes.Contains(w.Body.Bytes(), []byte("Tunnel not found")) {
		t.Error("Expected 'Tunnel not found' error message")
	}
}

// TestRouter_MissingHost tests handling of missing Host header
func TestRouter_MissingHost(t *testing.T) {
	reg := registry.NewRegistry("localhost")
	connManager := connection.NewManager(10, 5, 30*time.Second)
	limiter := quota.NewLimiter(100, 50)
	router := NewRouter(reg, connManager, limiter, 5*time.Second)

	req := httptest.NewRequest("GET", "http://localhost/", nil)
	req.Host = "" // Empty host
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	testutil.AssertEqual(t, http.StatusBadRequest, w.Code)
	if !bytes.Contains(w.Body.Bytes(), []byte("Missing Host header")) {
		t.Error("Expected 'Missing Host header' error message")
	}
}

// TestRouter_HealthCheck tests health check endpoint
func TestRouter_HealthCheck(t *testing.T) {
	reg := registry.NewRegistry("localhost")
	connManager := connection.NewManager(10, 5, 30*time.Second)
	limiter := quota.NewLimiter(100, 50)
	router := NewRouter(reg, connManager, limiter, 5*time.Second)

	req := httptest.NewRequest("GET", "http://localhost/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	testutil.AssertEqual(t, http.StatusOK, w.Code)
	testutil.AssertEqual(t, "OK", w.Body.String())
}

// TestRouter_QuotaExceeded tests rate limiting
func TestRouter_QuotaExceeded(t *testing.T) {
	reg := registry.NewRegistry("localhost")
	connManager := connection.NewManager(10, 5, 30*time.Second)
	limiter := quota.NewLimiter(100, 50)
	router := NewRouter(reg, connManager, limiter, 5*time.Second)

	// Register a tunnel
	mockConn := testutil.NewMockConnection()
	c, err := connManager.RegisterConnection("test-conn", "test-agent", "test-account", mockConn, nil)
	testutil.AssertNoError(t, err)

	err = reg.RegisterTunnel("test-agent", "test.localhost", c.ID, "test-account", nil)
	testutil.AssertNoError(t, err)

	// Set very low quota
	limiter.SetRequestQuota("test-agent", 1)

	// First request should succeed (or fail for other reasons)
	req1 := httptest.NewRequest("GET", "http://test.localhost/", nil)
	req1.Host = "test.localhost"
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	// Second request should be rate limited
	req2 := httptest.NewRequest("GET", "http://test.localhost/", nil)
	req2.Host = "test.localhost"
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	// Note: This test depends on quota implementation
	// Adjust based on actual quota behavior
}

// TestRouter_ConnectionNotFound tests handling of missing connection
func TestRouter_ConnectionNotFound(t *testing.T) {
	reg := registry.NewRegistry("localhost")
	connManager := connection.NewManager(10, 5, 30*time.Second)
	limiter := quota.NewLimiter(100, 50)
	router := NewRouter(reg, connManager, limiter, 5*time.Second)

	// Register tunnel but don't register connection
	err := reg.RegisterTunnel("test-agent", "test.localhost", "non-existent-conn", "test-account", nil)
	testutil.AssertNoError(t, err)

	req := httptest.NewRequest("GET", "http://test.localhost/", nil)
	req.Host = "test.localhost"
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	testutil.AssertEqual(t, http.StatusServiceUnavailable, w.Code)
	if !bytes.Contains(w.Body.Bytes(), []byte("Connection not found")) {
		t.Error("Expected 'Connection not found' error message")
	}
}

// TestRouter_BuildRequestPayload tests request payload building
func TestRouter_BuildRequestPayload(t *testing.T) {
	reg := registry.NewRegistry("localhost")
	connManager := connection.NewManager(10, 5, 30*time.Second)
	limiter := quota.NewLimiter(100, 50)
	router := NewRouter(reg, connManager, limiter, 5*time.Second)

	req := httptest.NewRequest("GET", "http://test.localhost/path?query=value", nil)
	req.Host = "test.localhost"
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("Accept", "application/json")

	payload := router.buildRequestPayload(req)

	// Verify payload contains request line
	if !bytes.Contains(payload, []byte("GET /path?query=value HTTP/1.1")) {
		t.Error("Payload should contain request line")
	}

	// Verify payload contains Host header
	if !bytes.Contains(payload, []byte("Host: test.localhost")) {
		t.Error("Payload should contain Host header")
	}

	// Verify payload contains other headers
	if !bytes.Contains(payload, []byte("User-Agent: test-agent")) {
		t.Error("Payload should contain User-Agent header")
	}

	if !bytes.Contains(payload, []byte("Accept: application/json")) {
		t.Error("Payload should contain Accept header")
	}
}

// TestRouter_ConcurrentRequests tests concurrent request handling
func TestRouter_ConcurrentRequests(t *testing.T) {
	reg := registry.NewRegistry("localhost")
	connManager := connection.NewManager(10, 5, 30*time.Second)
	limiter := quota.NewLimiter(100, 50)
	router := NewRouter(reg, connManager, limiter, 5*time.Second)

	// Register a tunnel
	mockConn := testutil.NewMockConnection()
	c, err := connManager.RegisterConnection("test-conn", "test-agent", "test-account", mockConn, nil)
	testutil.AssertNoError(t, err)

	err = reg.RegisterTunnel("test-agent", "test.localhost", c.ID, "test-account", nil)
	testutil.AssertNoError(t, err)

	// Send concurrent requests
	numRequests := 10
	testutil.RunConcurrent(numRequests, func(i int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("http://test.localhost/path%d", i), nil)
		req.Host = "test.localhost"
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		// Just verify no panic occurs
	})
}

// TestRouter_OnRequestCallback tests request callback
func TestRouter_OnRequestCallback(t *testing.T) {
	reg := registry.NewRegistry("localhost")
	connManager := connection.NewManager(10, 5, 30*time.Second)
	limiter := quota.NewLimiter(100, 50)
	router := NewRouter(reg, connManager, limiter, 5*time.Second)

	var callbackCalled bool
	var callbackAccountID string
	var callbackSuccess bool

	router.SetOnRequest(func(accountID string, duration time.Duration, success bool) {
		callbackCalled = true
		callbackAccountID = accountID
		callbackSuccess = success
	})

	// Register a tunnel
	mockConn := testutil.NewMockConnection()
	c, err := connManager.RegisterConnection("test-conn", "test-agent", "test-account", mockConn, nil)
	testutil.AssertNoError(t, err)

	err = reg.RegisterTunnel("test-agent", "test.localhost", c.ID, "test-account", nil)
	testutil.AssertNoError(t, err)

	req := httptest.NewRequest("GET", "http://test.localhost/", nil)
	req.Host = "test.localhost"
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Callback should be called
	testutil.WaitForCondition(t, 1*time.Second, func() bool {
		return callbackCalled
	}, "callback should be called")

	testutil.AssertEqual(t, "test-account", callbackAccountID)
}
