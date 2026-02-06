package quota

import (
	"sync"
	"testing"

	"github.com/hydragon2m/tunnel-core/testutil"
)

// TestLimiter_AgentStreamLimit tests agent stream limit enforcement
func TestLimiter_AgentStreamLimit(t *testing.T) {
	limiter := NewLimiter(100, 10) // maxConnections=100, maxStreams=10

	agentID := "test-agent"
	domain := "test.localhost"

	// Set agent limit to 3 streams
	limiter.SetAgentLimit(agentID, 3, 1000000, 100)

	// Acquire 3 streams - should succeed
	for i := 0; i < 3; i++ {
		err := limiter.AcquireStream(agentID, domain)
		testutil.AssertNoError(t, err)
	}

	// 4th stream should fail
	err := limiter.AcquireStream(agentID, domain)
	testutil.AssertError(t, err)

	// Release one stream
	limiter.ReleaseStream(agentID, domain)

	// Now should be able to acquire again
	err = limiter.AcquireStream(agentID, domain)
	testutil.AssertNoError(t, err)
}

// TestLimiter_DomainStreamLimit tests domain stream limit enforcement
func TestLimiter_DomainStreamLimit(t *testing.T) {
	limiter := NewLimiter(100, 10)

	agentID := "test-agent"
	domain := "test.localhost"

	// Set domain limit to 2 streams
	limiter.SetDomainLimit(domain, 2, 100)

	// Acquire 2 streams - should succeed
	for i := 0; i < 2; i++ {
		err := limiter.AcquireStream(agentID, domain)
		testutil.AssertNoError(t, err)
	}

	// 3rd stream should fail
	err := limiter.AcquireStream(agentID, domain)
	testutil.AssertError(t, err)

	// Release one stream
	limiter.ReleaseStream(agentID, domain)

	// Now should be able to acquire again
	err = limiter.AcquireStream(agentID, domain)
	testutil.AssertNoError(t, err)
}

// TestLimiter_AgentRateLimit tests agent rate limiting
func TestLimiter_AgentRateLimit(t *testing.T) {
	limiter := NewLimiter(100, 50)

	agentID := "test-agent"

	// Set rate limit to 5 requests/sec
	limiter.SetAgentLimit(agentID, 10, 1000000, 5)

	// First 5 requests should succeed
	for i := 0; i < 5; i++ {
		err := limiter.CheckAgentRateLimit(agentID)
		testutil.AssertNoError(t, err)
	}

	// 6th request should fail (rate limited)
	err := limiter.CheckAgentRateLimit(agentID)
	testutil.AssertError(t, err)
}

// TestLimiter_DomainRateLimit tests domain rate limiting
func TestLimiter_DomainRateLimit(t *testing.T) {
	limiter := NewLimiter(100, 50)

	domain := "test.localhost"

	// Set rate limit to 3 requests/sec
	limiter.SetDomainLimit(domain, 10, 3)

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		err := limiter.CheckDomainRateLimit(domain)
		testutil.AssertNoError(t, err)
	}

	// 4th request should fail (rate limited)
	err := limiter.CheckDomainRateLimit(domain)
	testutil.AssertError(t, err)
}

// TestLimiter_CheckRequest tests combined request checking
func TestLimiter_CheckRequest(t *testing.T) {
	limiter := NewLimiter(100, 50)

	agentID := "test-agent"
	domain := "test.localhost"

	// Set limits
	limiter.SetAgentLimit(agentID, 10, 1000000, 100)
	limiter.SetDomainLimit(domain, 10, 100)

	// Should succeed with reasonable limits
	err := limiter.CheckRequest(agentID, domain)
	testutil.AssertNoError(t, err)
}

// TestLimiter_ConcurrentAccess tests concurrent quota operations
func TestLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewLimiter(100, 50)

	agentID := "test-agent"
	domain := "test.localhost"

	// Set stream limit to 50
	limiter.SetAgentLimit(agentID, 50, 1000000, 1000)

	successCount := 0
	var mu sync.Mutex

	// Try to acquire 100 streams concurrently
	testutil.RunConcurrent(100, func(i int) {
		err := limiter.AcquireStream(agentID, domain)
		if err == nil {
			mu.Lock()
			successCount++
			mu.Unlock()
		}
	})

	// Should have exactly 50 successful acquisitions
	testutil.AssertEqual(t, 50, successCount)
}

// TestLimiter_GetAgentLimit tests retrieving agent limits
func TestLimiter_GetAgentLimit(t *testing.T) {
	limiter := NewLimiter(100, 50)

	agentID := "test-agent"

	// Set limits
	limiter.SetAgentLimit(agentID, 10, 1000000, 100)

	// Get limits
	limit, ok := limiter.GetAgentLimit(agentID)
	testutil.AssertTrue(t, ok, "agent limit should exist")
	testutil.AssertEqual(t, 10, limit.MaxStreams)
	testutil.AssertEqual(t, int64(1000000), limit.MaxBandwidth)
	testutil.AssertEqual(t, 100, limit.RateLimit)
}

// TestLimiter_GetDomainLimit tests retrieving domain limits
func TestLimiter_GetDomainLimit(t *testing.T) {
	limiter := NewLimiter(100, 50)

	domain := "test.localhost"

	// Set limits
	limiter.SetDomainLimit(domain, 5, 50)

	// Get limits
	limit, ok := limiter.GetDomainLimit(domain)
	testutil.AssertTrue(t, ok, "domain limit should exist")
	testutil.AssertEqual(t, 5, limit.MaxStreams)
	testutil.AssertEqual(t, 50, limit.RateLimit)
}

// TestLimiter_ResetAgentLimits tests resetting agent limits
func TestLimiter_ResetAgentLimits(t *testing.T) {
	limiter := NewLimiter(100, 50)

	agentID := "test-agent"
	domain := "test.localhost"

	// Set limits and acquire streams
	limiter.SetAgentLimit(agentID, 2, 1000000, 100)
	limiter.AcquireStream(agentID, domain)
	limiter.AcquireStream(agentID, domain)

	// Should be at limit
	err := limiter.AcquireStream(agentID, domain)
	testutil.AssertError(t, err)

	// Reset limits
	limiter.ResetAgentLimits(agentID)

	// Should work again
	err = limiter.AcquireStream(agentID, domain)
	testutil.AssertNoError(t, err)
}

// TestLimiter_ResetDomainLimits tests resetting domain limits
func TestLimiter_ResetDomainLimits(t *testing.T) {
	limiter := NewLimiter(100, 50)

	agentID := "test-agent"
	domain := "test.localhost"

	// Set limits and acquire streams
	limiter.SetDomainLimit(domain, 2, 100)
	limiter.AcquireStream(agentID, domain)
	limiter.AcquireStream(agentID, domain)

	// Should be at limit
	err := limiter.AcquireStream(agentID, domain)
	testutil.AssertError(t, err)

	// Reset limits
	limiter.ResetDomainLimits(domain)

	// Should work again
	err = limiter.AcquireStream(agentID, domain)
	testutil.AssertNoError(t, err)
}

// Add missing import
