package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestMemoryStoreConcurrentReservations(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore(false)
	now := time.Now().UTC()
	enabled := true
	if _, err := m.UpsertTeam(ctx, UpsertTeamInput{
		TeamID: "conc", RawAPIKey: "gw_conc", MonthlyLimitMicros: 500, Enabled: &enabled,
	}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	succeeded := 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := m.ReserveBudget(ctx, "conc", 100, now); err == nil {
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
		ID: "req_1", TeamID: "platform", RequestedModel: "auto",
		RoutedModel: "@cf/meta/llama-3.1-8b-instruct-fast", Status: types.StatusOK,
		PIIDetected: 2, PIIFindings: map[string]int{"email": 2}, TotalTokens: 120, CostMicros: 25,
	})
	_ = m.RecordAudit(ctx, types.AuditEntry{
		ID: "req_2", TeamID: "support", RequestedModel: "balanced",
		Status: types.StatusBlockedPII, PIIDetected: 1, PIIFindings: map[string]int{"ssn": 1},
	})

	filtered, err := m.ListAudit(ctx, types.AuditFilter{TeamID: "support", PIIOnly: true})
	if err != nil || len(filtered) != 1 || filtered[0].ID != "req_2" {
		t.Fatalf("unexpected filtered audit: %+v, err=%v", filtered, err)
	}

	sum, err := m.GetSummary(ctx)
	if err != nil || sum.TotalRequests != 2 || sum.SuccessfulCount != 1 || sum.BlockedPII != 1 || sum.TotalPIIDetected != 3 {
		t.Fatalf("unexpected summary: %+v, err=%v", sum, err)
	}
}
