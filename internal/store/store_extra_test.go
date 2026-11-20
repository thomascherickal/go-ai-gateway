package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreExtraOperations(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	m := NewMemoryStore(true)
	if m.BackendName() != "memory" {
		t.Fatalf("unexpected backend: %s", m.BackendName())
	}
	teams, err := m.ListTeams(ctx)
	if err != nil || len(teams) != 3 {
		t.Fatalf("ListTeams = %d, err=%v", len(teams), err)
	}
	team, err := m.GetTeam(ctx, "platform")
	if err != nil || team.TeamID != "platform" {
		t.Fatalf("GetTeam failed: %+v, err=%v", team, err)
	}
	if _, err := m.ReserveBudget(ctx, "platform", 100, now); err != nil {
		t.Fatal(err)
	}
	reset, err := m.ResetTeamBudget(ctx, "platform", now)
	if err != nil || reset.SpentMicros != 0 {
		t.Fatalf("ResetTeamBudget failed: %+v, err=%v", reset, err)
	}

	fs, err := NewFileStore(filepath.Join(t.TempDir(), "s.json"), true)
	if err != nil || fs.BackendName() != "file:s.json" {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	if _, err := fs.ReconcileBudget(ctx, "platform", 50, now); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.ResetTeamBudget(ctx, "platform", now); err != nil {
		t.Fatal(err)
	}
}
