package store

import (
	"context"
	"sync"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

// MemoryStore is a thread-safe in-memory implementation of Store for tests and local dev.
type MemoryStore struct {
	mu      sync.RWMutex
	teams   map[string]types.TeamBudget
	keyIdx  map[string]string // api_key_hash -> team_id
	audits  []types.AuditEntry
	nowFunc func() time.Time
}

// NewMemoryStore initializes an in-memory store, optionally seeded with demo teams.
func NewMemoryStore(seed bool) *MemoryStore {
	m := &MemoryStore{
		teams:   make(map[string]types.TeamBudget),
		keyIdx:  make(map[string]string),
		nowFunc: func() time.Time { return time.Now().UTC() },
	}
	if seed {
		for _, t := range DemoTeams(m.nowFunc()) {
			_, _ = m.UpsertTeam(context.Background(), t)
		}
	}
	return m
}

func (m *MemoryStore) BackendName() string { return "memory" }
