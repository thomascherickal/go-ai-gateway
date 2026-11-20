package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestMemoryStoreBudgetAndRollover(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore(false)

	now := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	m.nowFunc = func() time.Time { return now }

	enabled := true
	_, err := m.UpsertTeam(ctx, UpsertTeamInput{
		TeamID:             "eng",
		TeamName:           "Engineering",
		RawAPIKey:          "gw_eng_secret",
		MonthlyLimitMicros: 1000,
		MaxTier:            types.TierBalanced,
		PIIMode:            types.PIIModeRedact,
		Enabled:            &enabled,
	})
	if err != nil {
		t.Fatalf("UpsertTeam failed: %v", err)
	}

	team, err := m.LookupTeamByKey(ctx, "gw_eng_secret")
	if err != nil || team.TeamID != "eng" || team.RemainingMicros != 1000 {
		t.Fatalf("unexpected team: %+v, err=%v", team, err)
	}

	team, err = m.ReserveBudget(ctx, "eng", 600, now)
	if err != nil || team.RemainingMicros != 400 {
		t.Fatalf("first ReserveBudget failed: %+v, err=%v", team, err)
	}

	if _, err = m.ReserveBudget(ctx, "eng", 500, now); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("got err %v, want ErrBudgetExceeded", err)
	}

	team, err = m.ReconcileBudget(ctx, "eng", -350, now)
	if err != nil || team.SpentMicros != 250 || team.RemainingMicros != 750 {
		t.Fatalf("unexpected post-reconcile state: %+v, err=%v", team, err)
	}

	nextMonth := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	m.nowFunc = func() time.Time { return nextMonth }
	team, err = m.LookupTeamByKey(ctx, "gw_eng_secret")
	if err != nil || team.SpentMicros != 0 || team.PeriodYM != "2026-12" {
		t.Fatalf("expected monthly rollover, got %+v, err=%v", team, err)
	}
}
