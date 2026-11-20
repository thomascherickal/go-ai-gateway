package store

import (
	"context"
	"testing"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestMemoryAndFileStoreFullCoverage(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	m := NewMemoryStore(false)

	// Lookup/Get/Reserve/Reconcile/Reset error paths
	if _, err := m.LookupTeamByKey(ctx, ""); err != ErrTeamNotFound {
		t.Fatal("expected ErrTeamNotFound for empty key")
	}
	if _, err := m.LookupTeamByKey(ctx, "unknown"); err != ErrTeamNotFound {
		t.Fatal("expected ErrTeamNotFound for unknown key")
	}
	m.keyIdx[HashKey("orphan")] = "missing"
	if _, err := m.LookupTeamByKey(ctx, "orphan"); err != ErrTeamNotFound {
		t.Fatal("expected ErrTeamNotFound for orphan idx")
	}
	if _, err := m.GetTeam(ctx, "missing"); err != ErrTeamNotFound {
		t.Fatal("expected ErrTeamNotFound")
	}
	if _, err := m.ReserveBudget(ctx, "missing", 10, now); err != ErrTeamNotFound {
		t.Fatal("expected ErrTeamNotFound")
	}
	if _, err := m.ReconcileBudget(ctx, "missing", 10, now); err != ErrTeamNotFound {
		t.Fatal("expected ErrTeamNotFound")
	}
	if _, err := m.ResetTeamBudget(ctx, "missing", now); err != ErrTeamNotFound {
		t.Fatal("expected ErrTeamNotFound")
	}

	// UpsertTeam error + update rotating key + disabled ReserveBudget
	if _, err := m.UpsertTeam(ctx, UpsertTeamInput{}); err == nil {
		t.Fatal("expected validation error")
	}
	en := false
	if _, err := m.UpsertTeam(ctx, UpsertTeamInput{TeamID: "t1", TeamName: "T1", RawAPIKey: "k1", MonthlyLimitMicros: 100, Enabled: &en}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.UpsertTeam(ctx, UpsertTeamInput{TeamID: "t1", RawAPIKey: "k2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ReserveBudget(ctx, "t1", 10, now); err != ErrTeamDisabled {
		t.Fatalf("expected ErrTeamDisabled, got %v", err)
	}

	// Audit filters + summary statuses + unknown team in audit
	for _, e := range []types.AuditEntry{
		{ID: "1", TeamID: "ghost", Status: types.StatusBlockedBudget, RequestedModel: "m1"},
		{ID: "2", TeamID: "t1", Status: types.StatusUpstreamError, RequestedModel: "m2"},
		{ID: "3", TeamID: "t1", Status: types.StatusOK, RequestedModel: "m2", PIIDetected: 1},
	} {
		_ = m.RecordAudit(ctx, e)
	}
	_, _ = m.ListAudit(ctx, types.AuditFilter{Status: types.StatusOK, Model: "m2", PIIOnly: true, Limit: 1})
	_, _ = m.ListAudit(ctx, types.AuditFilter{Status: types.StatusBlockedPII})
	_, _ = m.ListAudit(ctx, types.AuditFilter{Model: "nonexistent"})
	_, _ = m.ListAudit(ctx, types.AuditFilter{Model: "m1", PIIOnly: true})
	_, _ = m.ListAudit(ctx, types.AuditFilter{Limit: 1})
	if sum, err := m.GetSummary(ctx); err != nil || sum.BlockedBudget != 1 || sum.UpstreamErrors != 1 {
		t.Fatalf("GetSummary: %+v %v", sum, err)
	}

	// Helpers: maxInt64, normalizeLimit, mergeTeamInput edge cases
	if maxInt64(1, 5) != 5 || maxInt64(5, 1) != 5 || normalizeLimit(999) <= 0 {
		t.Fatal("helper mismatch")
	}
	if _, err := mergeTeamInput(nil, UpsertTeamInput{TeamID: "x"}, now); err == nil {
		t.Fatal("expected missing key error")
	}
	_, _ = mergeTeamInput(nil, UpsertTeamInput{TeamID: "x", RawAPIKey: "k", MaxTier: "bad", PIIMode: "bad"}, now)
}
