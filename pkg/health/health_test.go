package health

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewChecker(t *testing.T) {
	checker := NewChecker("1.0.0")
	if checker == nil {
		t.Fatal("Checker should not be nil")
	}
	if checker.version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", checker.version)
	}
}

func TestRegisterCheck(t *testing.T) {
	checker := NewChecker("1.0.0")

	checker.RegisterCheck("test", func() Check {
		return Check{
			Status:  StatusHealthy,
			Message: "Test check",
		}
	}, true)

	if len(checker.checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(checker.checks))
	}
}

func TestCheck_Healthy(t *testing.T) {
	checker := NewChecker("1.0.0")

	checker.RegisterCheck("test1", func() Check {
		return Check{
			Status:  StatusHealthy,
			Message: "All good",
		}
	}, true)

	result := checker.Check()

	if result.Status != StatusHealthy {
		t.Errorf("Expected healthy status, got %s", result.Status)
	}
	if result.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", result.Version)
	}
	if len(result.Checks) != 1 {
		t.Errorf("Expected 1 check result, got %d", len(result.Checks))
	}
}

func TestCheck_Unhealthy(t *testing.T) {
	checker := NewChecker("1.0.0")

	checker.RegisterCheck("critical", func() Check {
		return Check{
			Status:  StatusUnhealthy,
			Message: "Service down",
		}
	}, true)

	result := checker.Check()

	if result.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy status, got %s", result.Status)
	}
}

func TestCheck_Degraded(t *testing.T) {
	checker := NewChecker("1.0.0")

	checker.RegisterCheck("degraded", func() Check {
		return Check{
			Status:  StatusDegraded,
			Message: "Slow response",
		}
	}, false)

	result := checker.Check()

	if result.Status != StatusDegraded {
		t.Errorf("Expected degraded status, got %s", result.Status)
	}
}

func TestCheck_Mixed(t *testing.T) {
	checker := NewChecker("1.0.0")

	checker.RegisterCheck("healthy", func() Check {
		return Check{Status: StatusHealthy}
	}, false)

	checker.RegisterCheck("degraded", func() Check {
		return Check{Status: StatusDegraded}
	}, false)

	result := checker.Check()

	// Should be degraded (not unhealthy)
	if result.Status != StatusDegraded {
		t.Errorf("Expected degraded status, got %s", result.Status)
	}
}

func TestCheck_Caching(t *testing.T) {
	checker := NewChecker("1.0.0")
	callCount := 0

	checker.RegisterCheck("test", func() Check {
		callCount++
		return Check{Status: StatusHealthy}
	}, false)

	// First call
	checker.Check()
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	// Second call (should use cache)
	checker.Check()
	if callCount != 1 {
		t.Errorf("Expected 1 call (cached), got %d", callCount)
	}

	// Wait for cache to expire
	time.Sleep(1100 * time.Millisecond)

	// Third call (cache expired)
	checker.Check()
	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
}

func TestLivenessHandler(t *testing.T) {
	checker := NewChecker("1.0.0")
	handler := checker.LivenessHandler()

	req := httptest.NewRequest("GET", "/health/live", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "alive" {
		t.Errorf("Expected status 'alive', got %v", response["status"])
	}
}

func TestReadinessHandler_Healthy(t *testing.T) {
	checker := NewChecker("1.0.0")
	checker.RegisterCheck("test", func() Check {
		return Check{Status: StatusHealthy}
	}, true)

	handler := checker.ReadinessHandler()

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestReadinessHandler_Unhealthy(t *testing.T) {
	checker := NewChecker("1.0.0")
	checker.RegisterCheck("test", func() Check {
		return Check{Status: StatusUnhealthy}
	}, true)

	handler := checker.ReadinessHandler()

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != 503 {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestDetailedHandler(t *testing.T) {
	checker := NewChecker("1.0.0")
	checker.RegisterCheck("test", func() Check {
		return Check{
			Status:  StatusHealthy,
			Message: "All systems operational",
			Details: map[string]interface{}{
				"count": 42,
			},
		}
	}, true)

	handler := checker.DetailedHandler()

	req := httptest.NewRequest("GET", "/health/detailed", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != StatusHealthy {
		t.Errorf("Expected healthy status, got %s", response.Status)
	}
	if response.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", response.Version)
	}
	if len(response.Checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(response.Checks))
	}
}

func TestSimpleHandler_Healthy(t *testing.T) {
	checker := NewChecker("1.0.0")
	checker.RegisterCheck("test", func() Check {
		return Check{Status: StatusHealthy}
	}, true)

	handler := checker.SimpleHandler()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "OK" {
		t.Errorf("Expected 'OK', got '%s'", w.Body.String())
	}
}

func TestSimpleHandler_Unhealthy(t *testing.T) {
	checker := NewChecker("1.0.0")
	checker.RegisterCheck("test", func() Check {
		return Check{Status: StatusUnhealthy}
	}, true)

	handler := checker.SimpleHandler()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != 503 {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
	if w.Body.String() != "ERROR" {
		t.Errorf("Expected 'ERROR', got '%s'", w.Body.String())
	}
}

func TestSystemCheck(t *testing.T) {
	check := SystemCheck()

	if check.Status == "" {
		t.Error("Status should not be empty")
	}
	if check.Details == nil {
		t.Error("Details should not be nil")
	}

	// Verify details contain expected fields
	if _, ok := check.Details["memory_mb"]; !ok {
		t.Error("Details should contain memory_mb")
	}
	if _, ok := check.Details["goroutines"]; !ok {
		t.Error("Details should contain goroutines")
	}
	if _, ok := check.Details["num_cpu"]; !ok {
		t.Error("Details should contain num_cpu")
	}
}

func BenchmarkCheck(b *testing.B) {
	checker := NewChecker("1.0.0")
	checker.RegisterCheck("test", func() Check {
		return Check{Status: StatusHealthy}
	}, true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.Check()
	}
}

func BenchmarkCheckCached(b *testing.B) {
	checker := NewChecker("1.0.0")
	checker.RegisterCheck("test", func() Check {
		time.Sleep(10 * time.Millisecond) // Simulate slow check
		return Check{Status: StatusHealthy}
	}, true)

	// Prime cache
	checker.Check()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.Check() // Should use cache
	}
}
