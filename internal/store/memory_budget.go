package store

import (
	"context"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func (m *MemoryStore) ReserveBudget(_ context.Context, teamID string, amountMicros int64, now time.Time) (types.TeamBudget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.teams[teamID]
	if !ok {
		return types.TeamBudget{}, ErrTeamNotFound
	}
	m.rolloverLocked(&t, now)
	if !t.Enabled {
		m.teams[teamID] = t
		return t, ErrTeamDisabled
	}
	if amountMicros > 0 && t.SpentMicros+amountMicros > t.MonthlyLimitMicros {
		m.teams[teamID] = t
		return t, ErrBudgetExceeded
	}
	if amountMicros > 0 {
		t.SpentMicros += amountMicros
	}
	t.UpdatedAt = now
	t.RemainingMicros = maxInt64(0, t.MonthlyLimitMicros-t.SpentMicros)
	m.teams[teamID] = t
	return t, nil
}

func (m *MemoryStore) ReconcileBudget(_ context.Context, teamID string, deltaMicros int64, now time.Time) (types.TeamBudget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.teams[teamID]
	if !ok {
		return types.TeamBudget{}, ErrTeamNotFound
	}
	t.SpentMicros = maxInt64(0, t.SpentMicros+deltaMicros)
	t.RemainingMicros = maxInt64(0, t.MonthlyLimitMicros-t.SpentMicros)
	t.UpdatedAt = now
	m.teams[teamID] = t
	return t, nil
}

func (m *MemoryStore) ResetTeamBudget(_ context.Context, teamID string, now time.Time) (types.TeamBudget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.teams[teamID]
	if !ok {
		return types.TeamBudget{}, ErrTeamNotFound
	}
	t.SpentMicros = 0
	t.PeriodYM = PeriodYM(now)
	t.RemainingMicros = t.MonthlyLimitMicros
	t.UpdatedAt = now
	m.teams[teamID] = t
	return t, nil
}
