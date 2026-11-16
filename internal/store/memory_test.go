package store

import (
	"context"
	"errors"
	"sync"
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
	if err != nil {
		t.Fatalf("LookupTeamByKey failed: %v", err)
	}
	if team.TeamID != "eng" || team.RemainingMicros != 1000 {
		t.Fatalf("unexpected team: %+v", team)
	}

	// Reserve 600 -> succeeds, 400 remaining.
	team, err = m.ReserveBudget(ctx, "eng", 600, now)
	if err != nil {
		t.Fatalf("first ReserveBudget failed: %v", err)
	}
	if team.RemainingMicros != 400 {
		t.Fatalf("remaining = %d, want 400", team.RemainingMicros)
	}

	// Reserve 500 -> exceeds remaining 400.
	_, err = m.ReserveBudget(ctx, "eng", 500, now)
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("got err %v, want ErrBudgetExceeded", err)
	}

	// Reconcile actual cost was 250 (delta = -350) -> 750 remaining.
	team, err = m.ReconcileBudget(ctx, "eng", -350, now)
	if err != nil {
		t.Fatalf("ReconcileBudget failed: %v", err)
	}
	if team.SpentMicros != 250 || team.RemainingMicros != 750 {
		t.Fatalf("unexpected post-reconcile state: %+v", team)
	}

	// Advance clock to next month -> automatic rollover resets spent to 0.
	nextMonth := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	m.nowFunc = func() time.Time { return nextMonth }
	team, err = m.LookupTeamByKey(ctx, "gw_eng_secret")
	if err != nil {
		t.Fatalf("next month lookup failed: %v", err)
	}
	if team.SpentMicros != 0 || team.PeriodYM != "2026-12" {
		t.Fatalf("expected monthly rollover, got %+v", team)
	}
}

func TestMemoryStoreConcurrentReservations(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore(false)
	now := time.Now().UTC()
	enabled := true
	_, err := m.UpsertTeam(ctx, UpsertTeamInput{
		TeamID:             "conc",
		RawAPIKey:          "gw_conc",
		MonthlyLimitMicros: 500,
		Enabled:            &enabled,
	})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	succeeded := 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.ReserveBudget(ctx, "conc", 100, now)
			if err == nil {
				mu.Lock()
				succeeded++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if succeeded != 5 {
		t.Fatalf("succeeded reservations = %d, want 5", succeeded)
	}
}

func TestMemoryStoreAuditAndSummary(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore(true)

	_ = m.RecordAudit(ctx, types.AuditEntry{
		ID:             "req_1",
		TeamID:         "platform",
		RequestedModel: "auto",
		RoutedModel:    "@cf/meta/llama-3.1-8b-instruct-fast",
		Status:         types.StatusOK,
		PIIDetected:    2,
		PIIFindings:    map[string]int{"email": 2},
		TotalTokens:    120,
		CostMicros:     25,
	})
	_ = m.RecordAudit(ctx, types.AuditEntry{
		ID:             "req_2",
		TeamID:         "support",
		RequestedModel: "balanced",
		Status:         types.StatusBlockedPII,
		PIIDetected:    1,
		PIIFindings:    map[string]int{"ssn": 1},
	})

	filtered, err := m.ListAudit(ctx, types.AuditFilter{TeamID: "support", PIIOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].ID != "req_2" {
		t.Fatalf("unexpected filtered audit: %+v", filtered)
	}

	sum, err := m.GetSummary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sum.TotalRequests != 2 || sum.SuccessfulCount != 1 || sum.BlockedPII != 1 || sum.TotalPIIDetected != 3 {
		t.Fatalf("unexpected summary: %+v", sum)
	}
}

