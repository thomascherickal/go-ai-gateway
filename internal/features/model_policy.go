package features

import (
	"strings"
	"sync"
)

type teamModelLists struct {
	allow map[string]bool
	deny  map[string]bool
}

// ModelPolicy enforces per-team model allowlists and denylists.
type ModelPolicy struct {
	mu    sync.RWMutex
	teams map[string]teamModelLists
}

// NewModelPolicy creates an empty per-team model policy registry.
func NewModelPolicy() *ModelPolicy {
	return &ModelPolicy{teams: make(map[string]teamModelLists)}
}

// SetPolicy configures allowed and denied model IDs for a team.
func (m *ModelPolicy) SetPolicy(teamID string, allow, deny []string) {
	aMap := make(map[string]bool, len(allow))
	for _, id := range allow {
		aMap[strings.ToLower(strings.TrimSpace(id))] = true
	}
	dMap := make(map[string]bool, len(deny))
	for _, id := range deny {
		dMap[strings.ToLower(strings.TrimSpace(id))] = true
	}
	m.mu.Lock()
	m.teams[teamID] = teamModelLists{allow: aMap, deny: dMap}
	m.mu.Unlock()
}

// Allowed reports whether teamID is permitted to invoke modelID.
func (m *ModelPolicy) Allowed(teamID, modelID string) bool {
	m.mu.RLock()
	lists, ok := m.teams[teamID]
	m.mu.RUnlock()
	if !ok {
		return true
	}
	key := strings.ToLower(strings.TrimSpace(modelID))
	if lists.deny[key] {
		return false
	}
	if len(lists.allow) > 0 {
		return lists.allow[key]
	}
	return true
}
