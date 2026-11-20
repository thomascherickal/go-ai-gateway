package store

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func (m *MemoryStore) rolloverLocked(t *types.TeamBudget, now time.Time) {
	ym := PeriodYM(now)
	if t.PeriodYM != ym {
		t.SpentMicros = 0
		t.PeriodYM = ym
		t.UpdatedAt = now
	}
	t.RemainingMicros = maxInt64(0, t.MonthlyLimitMicros-t.SpentMicros)
}

func (m *MemoryStore) LookupTeamByKey(_ context.Context, rawKey string) (types.TeamBudget, error) {
	if rawKey == "" {
		return types.TeamBudget{}, ErrTeamNotFound
	}
	hash := HashKey(rawKey)
	m.mu.Lock()
	defer m.mu.Unlock()

	teamID, ok := m.keyIdx[hash]
	if !ok {
		return types.TeamBudget{}, ErrTeamNotFound
	}
	t, ok := m.teams[teamID]
	if !ok {
		return types.TeamBudget{}, ErrTeamNotFound
	}
	m.rolloverLocked(&t, m.nowFunc())
	m.teams[teamID] = t
	return t, nil
}

func (m *MemoryStore) GetTeam(_ context.Context, teamID string) (types.TeamBudget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.teams[teamID]
	if !ok {
		return types.TeamBudget{}, ErrTeamNotFound
	}
	m.rolloverLocked(&t, m.nowFunc())
	m.teams[teamID] = t
	return t, nil
}

func (m *MemoryStore) ListTeams(_ context.Context) ([]types.TeamBudget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.nowFunc()
	out := make([]types.TeamBudget, 0, len(m.teams))
	for id, t := range m.teams {
		m.rolloverLocked(&t, now)
		m.teams[id] = t
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TeamID < out[j].TeamID })
	return out, nil
}

func (m *MemoryStore) UpsertTeam(_ context.Context, in UpsertTeamInput) (types.TeamBudget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.nowFunc()
	var existingPtr *types.TeamBudget
	if existing, ok := m.teams[in.TeamID]; ok {
		existingPtr = &existing
	}
	t, err := mergeTeamInput(existingPtr, in, now)
	if err != nil {
		return types.TeamBudget{}, err
	}
	if strings.TrimSpace(in.RawAPIKey) != "" {
		if existingPtr != nil && existingPtr.APIKeyHash != "" {
			delete(m.keyIdx, existingPtr.APIKeyHash)
		}
		m.keyIdx[t.APIKeyHash] = t.TeamID
	}
	m.rolloverLocked(&t, now)
	m.teams[t.TeamID] = t
	return t, nil
}
