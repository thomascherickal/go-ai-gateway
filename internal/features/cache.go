package features

import (
	"sync"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

type cacheEntry struct {
	resp      types.ChatCompletionResponse
	expiresAt time.Time
}

// ResponseCache caches identical redacted prompts keyed by model + promptSHA256 with TTL.
type ResponseCache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]cacheEntry
}

// NewResponseCache creates a response cache with the given TTL.
func NewResponseCache(ttl time.Duration) *ResponseCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &ResponseCache{ttl: ttl, entries: make(map[string]cacheEntry)}
}

// CacheKey builds a deterministic key from model ID and redacted prompt SHA-256.
func CacheKey(modelID, promptSHA256 string) string { return modelID + ":" + promptSHA256 }

// Get returns a cached response if present and not expired.
func (c *ResponseCache) Get(key string, now time.Time) (types.ChatCompletionResponse, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || now.After(e.expiresAt) {
		return types.ChatCompletionResponse{}, false
	}
	return e.resp, true
}

// Set stores a response in the cache until now + ttl.
func (c *ResponseCache) Set(key string, resp types.ChatCompletionResponse, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{resp: resp, expiresAt: now.Add(c.ttl)}
}
