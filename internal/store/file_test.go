//go:build !js

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestFileStorePersistenceAcrossReload(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.json")

	fs1, err := NewFileStore(path, false)
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	_, err = fs1.UpsertTeam(ctx, UpsertTeamInput{
		TeamID:             "personal",
		TeamName:           "Personal",
		RawAPIKey:          "gw_personal_123",
		MonthlyLimitMicros: 10_000_000,
		MaxTier:            types.TierReasoning,
		PIIMode:            types.PIIModeRedact,
		Enabled:            &enabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = fs1.ReserveBudget(ctx, "personal", 2500, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	_ = fs1.RecordAudit(ctx, types.AuditEntry{
		ID:             "gw_p1",
		TeamID:         "personal",
		RequestedModel: "claude-sonnet-4-5",
		Status:         types.StatusOK,
		CostMicros:     2500,
	})

	// Re-open from disk in a fresh FileStore instance.
	fs2, err := NewFileStore(path, false)
	if err != nil {
		t.Fatal(err)
	}
	team, err := fs2.LookupTeamByKey(ctx, "gw_personal_123")
	if err != nil {
		t.Fatalf("LookupTeamByKey after reload failed: %v", err)
	}
	if team.SpentMicros != 2500 || team.RemainingMicros != 9_997_500 {
		t.Fatalf("unexpected reloaded budget: %+v", team)
	}
	audits, err := fs2.ListAudit(ctx, types.AuditFilter{TeamID: "personal"})
	if err != nil || len(audits) != 1 || audits[0].ID != "gw_p1" {
		t.Fatalf("unexpected reloaded audits: %+v, err=%v", audits, err)
	}
}
