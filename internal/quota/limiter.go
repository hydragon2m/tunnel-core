package quota

import (
	"sync"
	"time"
)

// Limiter quản lý rate limiting và resource quotas
type Limiter struct {
	// Per-agent limits
	agentLimits map[string]*AgentLimit
	agentMu     sync.RWMutex

	// Per-domain limits
	domainLimits map[string]*DomainLimit
	domainMu     sync.RWMutex

	// Global limits
	maxConnections int
	maxStreams     int
}

// AgentLimit là limit cho 1 agent
type AgentLimit struct {
	AgentID        string
	MaxStreams     int          // Max concurrent streams
	MaxBandwidth   int64        // Max bandwidth (bytes/second)
	RateLimit      int          // Max requests per second
	TokenBucket    *TokenBucket // Token bucket cho rate limiting
	CurrentStreams int          // Current active streams
	LastReset      time.Time    // Last time limits were reset
	mu             sync.RWMutex
}

// DomainLimit là limit cho 1 domain
type DomainLimit struct {
	Domain         string
	MaxStreams     int          // Max concurrent streams
	RateLimit      int          // Max requests per second
	TokenBucket    *TokenBucket // Token bucket cho rate limiting
	CurrentStreams int          // Current active streams
	LastReset      time.Time    // Last time limits were reset
	mu             sync.RWMutex
}

// TokenBucket implements token bucket algorithm for rate limiting
type TokenBucket struct {
	capacity   int       // Max tokens
	tokens     float64   // Current tokens
	refillRate float64   // Tokens per second
	lastRefill time.Time // Last refill time
	mu         sync.Mutex
}

// NewLimiter tạo Limiter mới
func NewLimiter(maxConnections, maxStreams int) *Limiter {
	return &Limiter{
		agentLimits:    make(map[string]*AgentLimit),
		domainLimits:   make(map[string]*DomainLimit),
		maxConnections: maxConnections,
		maxStreams:     maxStreams,
	}
}

// SetAgentLimit set limit cho agent
func (l *Limiter) SetAgentLimit(agentID string, maxStreams int, maxBandwidth int64, rateLimit int) {
	l.agentMu.Lock()
	defer l.agentMu.Unlock()

	limit := &AgentLimit{
		AgentID:      agentID,
		MaxStreams:   maxStreams,
		MaxBandwidth: maxBandwidth,
		RateLimit:    rateLimit,
		TokenBucket:  NewTokenBucket(rateLimit, rateLimit),
		LastReset:    time.Now(),
	}

	l.agentLimits[agentID] = limit
}

// SetDomainLimit set limit cho domain
func (l *Limiter) SetDomainLimit(domain string, maxStreams int, rateLimit int) {
	l.domainMu.Lock()
	defer l.domainMu.Unlock()

	limit := &DomainLimit{
		Domain:      domain,
		MaxStreams:  maxStreams,
		RateLimit:   rateLimit,
		TokenBucket: NewTokenBucket(rateLimit, rateLimit),
		LastReset:   time.Now(),
	}

	l.domainLimits[domain] = limit
}

// CheckAgentStreamLimit kiểm tra xem agent có thể tạo stream mới không
func (l *Limiter) CheckAgentStreamLimit(agentID string) error {
	l.agentMu.RLock()
	limit, exists := l.agentLimits[agentID]
	l.agentMu.RUnlock()

	if !exists {
		// No limit set, allow
		return nil
	}

	limit.mu.Lock()
	defer limit.mu.Unlock()

	if limit.CurrentStreams >= limit.MaxStreams {
		return ErrAgentStreamLimitExceeded
	}

	return nil
}

// CheckDomainStreamLimit kiểm tra xem domain có thể tạo stream mới không
func (l *Limiter) CheckDomainStreamLimit(domain string) error {
	l.domainMu.RLock()
	limit, exists := l.domainLimits[domain]
	l.domainMu.RUnlock()

	if !exists {
		// No limit set, allow
		return nil
	}

	limit.mu.Lock()
	defer limit.mu.Unlock()

	if limit.CurrentStreams >= limit.MaxStreams {
		return ErrDomainStreamLimitExceeded
	}

	return nil
}

// CheckAgentRateLimit kiểm tra rate limit cho agent
func (l *Limiter) CheckAgentRateLimit(agentID string) error {
	l.agentMu.RLock()
	limit, exists := l.agentLimits[agentID]
	l.agentMu.RUnlock()

	if !exists {
		// No limit set, allow
		return nil
	}

	if !limit.TokenBucket.Allow() {
		return ErrAgentRateLimitExceeded
	}

	return nil
}

// CheckDomainRateLimit kiểm tra rate limit cho domain
func (l *Limiter) CheckDomainRateLimit(domain string) error {
	l.domainMu.RLock()
	limit, exists := l.domainLimits[domain]
	l.domainMu.RUnlock()

	if !exists {
		// No limit set, allow
		return nil
	}

	if !limit.TokenBucket.Allow() {
		return ErrDomainRateLimitExceeded
	}

	return nil
}

// AcquireStream tăng stream count cho agent và domain
func (l *Limiter) AcquireStream(agentID, domain string) error {
	// Check agent limit
	if err := l.CheckAgentStreamLimit(agentID); err != nil {
		return err
	}

	// Check domain limit
	if err := l.CheckDomainStreamLimit(domain); err != nil {
		return err
	}

	// Acquire
	l.agentMu.Lock()
	if limit, exists := l.agentLimits[agentID]; exists {
		limit.mu.Lock()
		limit.CurrentStreams++
		limit.mu.Unlock()
	}
	l.agentMu.Unlock()

	l.domainMu.Lock()
	if limit, exists := l.domainLimits[domain]; exists {
		limit.mu.Lock()
		limit.CurrentStreams++
		limit.mu.Unlock()
	}
	l.domainMu.Unlock()

	return nil
}

// ReleaseStream giảm stream count cho agent và domain
func (l *Limiter) ReleaseStream(agentID, domain string) {
	l.agentMu.Lock()
	if limit, exists := l.agentLimits[agentID]; exists {
		limit.mu.Lock()
		if limit.CurrentStreams > 0 {
			limit.CurrentStreams--
		}
		limit.mu.Unlock()
	}
	l.agentMu.Unlock()

	l.domainMu.Lock()
	if limit, exists := l.domainLimits[domain]; exists {
		limit.mu.Lock()
		if limit.CurrentStreams > 0 {
			limit.CurrentStreams--
		}
		limit.mu.Unlock()
	}
	l.domainMu.Unlock()
}

// CheckRequest kiểm tra tất cả limits cho 1 request
func (l *Limiter) CheckRequest(agentID, domain string) error {
	// Check rate limits
	if err := l.CheckAgentRateLimit(agentID); err != nil {
		return err
	}

	if err := l.CheckDomainRateLimit(domain); err != nil {
		return err
	}

	// Check stream limits
	if err := l.CheckAgentStreamLimit(agentID); err != nil {
		return err
	}

	if err := l.CheckDomainStreamLimit(domain); err != nil {
		return err
	}

	return nil
}

// NewTokenBucket tạo token bucket mới
func NewTokenBucket(capacity, refillRate int) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     float64(capacity),
		refillRate: float64(refillRate),
		lastRefill: time.Now(),
	}
}

// Allow kiểm tra xem có token không và consume nếu có
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens = min(float64(tb.capacity), tb.tokens+elapsed*tb.refillRate)
	tb.lastRefill = now

	// Check if we have tokens
	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}

	return false
}

// AllowN kiểm tra xem có đủ N tokens không và consume nếu có
func (tb *TokenBucket) AllowN(n int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens = min(float64(tb.capacity), tb.tokens+elapsed*tb.refillRate)
	tb.lastRefill = now

	// Check if we have enough tokens
	if tb.tokens >= float64(n) {
		tb.tokens -= float64(n)
		return true
	}

	return false
}

// GetStats lấy statistics của token bucket
func (tb *TokenBucket) GetStats() (tokens float64, capacity int) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens = min(float64(tb.capacity), tb.tokens+elapsed*tb.refillRate)
	tb.lastRefill = now

	return tb.tokens, tb.capacity
}

// min returns minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// GetAgentLimit lấy limit của agent
func (l *Limiter) GetAgentLimit(agentID string) (*AgentLimit, bool) {
	l.agentMu.RLock()
	defer l.agentMu.RUnlock()

	limit, ok := l.agentLimits[agentID]
	return limit, ok
}

// GetDomainLimit lấy limit của domain
func (l *Limiter) GetDomainLimit(domain string) (*DomainLimit, bool) {
	l.domainMu.RLock()
	defer l.domainMu.Unlock()

	limit, ok := l.domainLimits[domain]
	return limit, ok
}

// ResetAgentLimits reset limits cho agent (for testing/admin)
func (l *Limiter) ResetAgentLimits(agentID string) {
	l.agentMu.Lock()
	defer l.agentMu.Unlock()

	if limit, exists := l.agentLimits[agentID]; exists {
		limit.mu.Lock()
		limit.CurrentStreams = 0
		limit.TokenBucket = NewTokenBucket(limit.RateLimit, limit.RateLimit)
		limit.LastReset = time.Now()
		limit.mu.Unlock()
	}
}

// ResetDomainLimits reset limits cho domain (for testing/admin)
func (l *Limiter) ResetDomainLimits(domain string) {
	l.domainMu.Lock()
	defer l.domainMu.Unlock()

	if limit, exists := l.domainLimits[domain]; exists {
		limit.mu.Lock()
		limit.CurrentStreams = 0
		limit.TokenBucket = NewTokenBucket(limit.RateLimit, limit.RateLimit)
		limit.LastReset = time.Now()
		limit.mu.Unlock()
	}
}
