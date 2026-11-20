package features

import (
	"sync"
	"time"
)

type breakerState struct {
	failures  int
	openUntil time.Time
}

// CircuitBreaker trips open when an upstream provider exceeds consecutive failure thresholds.
type CircuitBreaker struct {
	mu        sync.Mutex
	threshold int
	cooldown  time.Duration
	states    map[string]*breakerState
}

// NewCircuitBreaker creates a provider circuit breaker.
func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 3
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	return &CircuitBreaker{
		threshold: threshold,
		cooldown:  cooldown,
		states:    make(map[string]*breakerState),
	}
}

// Allow reports whether calls to provider are permitted at now.
func (c *CircuitBreaker) Allow(provider string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	st, ok := c.states[provider]
	if !ok {
		return true
	}
	return !now.Before(st.openUntil)
}

// RecordSuccess resets the consecutive failure counter for provider.
func (c *CircuitBreaker) RecordSuccess(provider string) {
	c.mu.Lock()
	delete(c.states, provider)
	c.mu.Unlock()
}

// RecordFailure increments consecutive failures and opens the circuit once threshold is reached.
func (c *CircuitBreaker) RecordFailure(provider string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	st, ok := c.states[provider]
	if !ok {
		st = &breakerState{}
		c.states[provider] = st
	}
	st.failures++
	if st.failures >= c.threshold {
		st.openUntil = now.Add(c.cooldown)
		return true
	}
	return false
}
