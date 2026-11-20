package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestD1StoreOpenAndQueryErrors(t *testing.T) {
	openErr := errors.New("db open failed")
	st := NewD1Store(func() (*sql.DB, error) { return nil, openErr })
	ctx := context.Background()
	now := time.Now().UTC()

	if _, err := st.LookupTeamByKey(ctx, "k"); !errors.Is(err, openErr) {
		t.Fatalf("LookupTeamByKey: got %v", err)
	}
	if _, err := st.GetTeam(ctx, "t"); !errors.Is(err, openErr) {
		t.Fatalf("GetTeam: got %v", err)
	}
	if _, err := st.ListTeams(ctx); !errors.Is(err, openErr) {
		t.Fatalf("ListTeams: got %v", err)
	}
	if _, err := st.UpsertTeam(ctx, UpsertTeamInput{TeamID: "t"}); !errors.Is(err, openErr) {
		t.Fatalf("UpsertTeam: got %v", err)
	}
	if _, err := st.ReserveBudget(ctx, "t", 10, now); !errors.Is(err, openErr) {
		t.Fatalf("ReserveBudget: got %v", err)
	}
	if _, err := st.ReconcileBudget(ctx, "t", -5, now); !errors.Is(err, openErr) {
		t.Fatalf("ReconcileBudget: got %v", err)
	}
	if _, err := st.ResetTeamBudget(ctx, "t", now); !errors.Is(err, openErr) {
		t.Fatalf("ResetTeamBudget: got %v", err)
	}
	if err := st.RecordAudit(ctx, types.AuditEntry{ID: "1"}); !errors.Is(err, openErr) {
		t.Fatalf("RecordAudit: got %v", err)
	}
	if _, err := st.ListAudit(ctx, types.AuditFilter{}); !errors.Is(err, openErr) {
		t.Fatalf("ListAudit: got %v", err)
	}
	if _, err := st.GetSummary(ctx); !errors.Is(err, openErr) {
		t.Fatalf("GetSummary: got %v", err)
	}
}
