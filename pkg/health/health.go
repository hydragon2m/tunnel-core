package health

import (
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// Status represents health status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// Check represents a health check
type Check struct {
	Name      string                 `json:"name"`
	Status    Status                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// HealthResponse represents the overall health response
type HealthResponse struct {
	Status    Status           `json:"status"`
	Timestamp time.Time        `json:"timestamp"`
	Version   string           `json:"version,omitempty"`
	Uptime    string           `json:"uptime,omitempty"`
	Checks    map[string]Check `json:"checks,omitempty"`
}

// Checker performs health checks
type Checker struct {
	mu         sync.RWMutex
	checks     map[string]*CheckFunc
	startTime  time.Time
	version    string
	cacheTTL   time.Duration
	lastResult *HealthResponse
	lastCheck  time.Time
}

// CheckFunc is a function that performs a health check
type CheckFunc struct {
	Name     string
	CheckFn  func() Check
	Critical bool // If true, failure makes overall status unhealthy
}

// NewChecker creates a new health checker
func NewChecker(version string) *Checker {
	return &Checker{
		checks:    make(map[string]*CheckFunc),
		startTime: time.Now(),
		version:   version,
		cacheTTL:  1 * time.Second, // Cache results for 1 second
	}
}

// RegisterCheck registers a health check
func (c *Checker) RegisterCheck(name string, checkFn func() Check, critical bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.checks[name] = &CheckFunc{
		Name:     name,
		CheckFn:  checkFn,
		Critical: critical,
	}
}

// Check performs all health checks
func (c *Checker) Check() HealthResponse {
	c.mu.RLock()
	// Return cached result if still valid
	if c.lastResult != nil && time.Since(c.lastCheck) < c.cacheTTL {
		result := *c.lastResult
		c.mu.RUnlock()
		return result
	}
	c.mu.RUnlock()

	// Perform checks
	c.mu.Lock()
	defer c.mu.Unlock()

	checks := make(map[string]Check)
	overallStatus := StatusHealthy

	for name, checkFunc := range c.checks {
		check := checkFunc.CheckFn()
		check.Name = name
		check.Timestamp = time.Now()
		checks[name] = check

		// Update overall status
		if check.Status == StatusUnhealthy && checkFunc.Critical {
			overallStatus = StatusUnhealthy
		} else if check.Status == StatusDegraded && overallStatus == StatusHealthy {
			overallStatus = StatusDegraded
		}
	}

	response := HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Version:   c.version,
		Uptime:    time.Since(c.startTime).String(),
		Checks:    checks,
	}

	// Cache result
	c.lastResult = &response
	c.lastCheck = time.Now()

	return response
}

// LivenessHandler returns HTTP handler for liveness probe
func (c *Checker) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Liveness just checks if the process is running
		// Always return 200 OK if we can respond
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "alive",
			"timestamp": time.Now(),
		})
	}
}

// ReadinessHandler returns HTTP handler for readiness probe
func (c *Checker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result := c.Check()

		w.Header().Set("Content-Type", "application/json")

		// Return 503 if unhealthy, 200 otherwise
		if result.Status == StatusUnhealthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		// Simple response for readiness
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    result.Status,
			"timestamp": result.Timestamp,
		})
	}
}

// DetailedHandler returns HTTP handler for detailed health check
func (c *Checker) DetailedHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result := c.Check()

		w.Header().Set("Content-Type", "application/json")

		// Return 503 if unhealthy, 200 otherwise
		if result.Status == StatusUnhealthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		json.NewEncoder(w).Encode(result)
	}
}

// SimpleHandler returns simple OK/ERROR handler
func (c *Checker) SimpleHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result := c.Check()

		w.Header().Set("Content-Type", "text/plain")

		if result.Status == StatusUnhealthy {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("ERROR"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}
	}
}

// SystemCheck returns system-level health check
func SystemCheck() Check {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	status := StatusHealthy
	message := "System healthy"

	// Check memory usage (warning if > 1GB)
	memoryMB := m.Alloc / 1024 / 1024
	if memoryMB > 1024 {
		status = StatusDegraded
		message = "High memory usage"
	}

	// Check goroutines (warning if > 10000)
	goroutines := runtime.NumGoroutine()
	if goroutines > 10000 {
		status = StatusDegraded
		message = "High goroutine count"
	}

	return Check{
		Status:  status,
		Message: message,
		Details: map[string]interface{}{
			"memory_mb":  memoryMB,
			"goroutines": goroutines,
			"num_cpu":    runtime.NumCPU(),
		},
	}
}
