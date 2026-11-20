package features

import (
	"sync"
	"time"
)

// RateLimitConfig defines per-team requests-per-minute and tokens-per-minute limits.
type RateLimitConfig struct {
	RPM int
	TPM int
}

type bucket struct {
	windowStart time.Time
	reqs        int
	tokens      int
}

// RateLimiter enforces per-team RPM and TPM limits using a sliding 1-minute window.
type RateLimiter struct {
	mu      sync.Mutex
	limits  map[string]RateLimitConfig
	buckets map[string]*bucket
}

// NewRateLimiter creates a per-team rate limiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limits:  make(map[string]RateLimitConfig),
		buckets: make(map[string]*bucket),
	}
}

// SetLimit configures RPM and TPM caps for a team.
func (r *RateLimiter) SetLimit(teamID string, cfg RateLimitConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limits[teamID] = cfg
}

// Allow checks and consumes 1 request and estTokens for teamID at now.
func (r *RateLimiter) Allow(teamID string, estTokens int, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	cfg, ok := r.limits[teamID]
	if !ok || (cfg.RPM <= 0 && cfg.TPM <= 0) {
		return true
	}
	b, exists := r.buckets[teamID]
	if !exists || now.Sub(b.windowStart) >= time.Minute {
		b = &bucket{windowStart: now}
		r.buckets[teamID] = b
	}
	if cfg.RPM > 0 && b.reqs+1 > cfg.RPM {
		return false
	}
	if cfg.TPM > 0 && b.tokens+estTokens > cfg.TPM {
		return false
	}
	b.reqs++
	b.tokens += estTokens
	return true
}
