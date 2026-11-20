package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func d1With(steps ...stubStep) *D1Store {
	conn := &stubConnector{steps: steps}
	return NewD1Store(func() (*sql.DB, error) { return sql.OpenDB(conn), nil })
}

func TestD1StoreFullBranchCoverage(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	boom := errors.New("boom")
	tCols := []string{"team_id", "team_name", "api_key_hash", "monthly_limit_micros", "spent_micros", "period_ym", "max_tier", "pii_mode", "enabled", "created_at", "updated_at"}
	disRow := []driver.Value{"t1", "T1", "h", int64(100), int64(0), PeriodYM(now), "fast", "redact", int64(0), now.Format(time.RFC3339), now.Format(time.RFC3339)}
	oldRow := []driver.Value{"t1", "T1", "h", int64(100), int64(50), "2020-01", "fast", "redact", int64(1), now.Format(time.RFC3339), now.Format(time.RFC3339)}

	// RecordAudit with FallbackUsed=true + zero CreatedAt; ListAudit filters & errors
	_ = d1With(stubStep{rowsAffected: 1}).RecordAudit(ctx, types.AuditEntry{ID: "1", FallbackUsed: true})
	_, _ = d1With(stubStep{err: boom}).ListAudit(ctx, types.AuditFilter{Status: types.StatusOK, Model: "m"})
	_, _ = d1With(stubStep{columns: []string{"bad"}, rows: [][]driver.Value{{"x"}}}).ListAudit(ctx, types.AuditFilter{})

	// ReserveBudget: rollover err, update err, RowsAffected err, getTeam err, disabled team
	_, _ = d1With(stubStep{err: boom}).ReserveBudget(ctx, "t1", 10, now)
	_, _ = d1With(stubStep{}, stubStep{err: boom}).ReserveBudget(ctx, "t1", 10, now)
	_, _ = d1With(stubStep{}, stubStep{rowsErr: boom}).ReserveBudget(ctx, "t1", 10, now)
	_, _ = d1With(stubStep{}, stubStep{rowsAffected: 1}, stubStep{err: boom}).ReserveBudget(ctx, "t1", 10, now)
	if _, err := d1With(stubStep{}, stubStep{rowsAffected: 0}, stubStep{columns: tCols, rows: [][]driver.Value{disRow}}).ReserveBudget(ctx, "t1", 10, now); !errors.Is(err, ErrTeamDisabled) {
		t.Fatalf("expected ErrTeamDisabled, got %v", err)
	}

	// ReconcileBudget err; ResetTeamBudget err + 0 rows affected
	_, _ = d1With(stubStep{err: boom}).ReconcileBudget(ctx, "t1", 10, now)
	_, _ = d1With(stubStep{err: boom}).ResetTeamBudget(ctx, "t1", now)
	if _, err := d1With(stubStep{rowsAffected: 0}).ResetTeamBudget(ctx, "t1", now); !errors.Is(err, ErrTeamNotFound) {
		t.Fatalf("expected ErrTeamNotFound, got %v", err)
	}

	// GetSummary: first scan err, second query err, second scan err
	_, _ = d1With(stubStep{err: boom}).GetSummary(ctx)
	_, _ = d1With(stubStep{columns: []string{"a", "b", "c", "d", "e", "f", "g", "h"}, rows: [][]driver.Value{{int64(0), int64(0), int64(0), int64(0), int64(0), int64(0), int64(0), int64(0)}}}, stubStep{err: boom}).GetSummary(ctx)
	_, _ = d1With(stubStep{columns: []string{"a", "b", "c", "d", "e", "f", "g", "h"}, rows: [][]driver.Value{{int64(0), int64(0), int64(0), int64(0), int64(0), int64(0), int64(0), int64(0)}}}, stubStep{columns: []string{"bad"}, rows: [][]driver.Value{{"x"}}}).GetSummary(ctx)

	// LookupTeamByKey empty, ErrNoRows, other err; GetTeam err; ensurePeriodRollover err; ListTeams query/scan/rollover err
	_, _ = d1With().LookupTeamByKey(ctx, "")
	_, _ = d1With(stubStep{columns: tCols, rows: nil}).LookupTeamByKey(ctx, "k")
	_, _ = d1With(stubStep{err: boom}).LookupTeamByKey(ctx, "k")
	_, _ = d1With(stubStep{err: boom}).GetTeam(ctx, "t1")
	_, _ = d1With(stubStep{columns: tCols, rows: [][]driver.Value{oldRow}}, stubStep{err: boom}).GetTeam(ctx, "t1")
	_, _ = d1With(stubStep{err: boom}).ListTeams(ctx)
	_, _ = d1With(stubStep{columns: []string{"bad"}, rows: [][]driver.Value{{"x"}}}).ListTeams(ctx)
	_, _ = d1With(stubStep{columns: tCols, rows: [][]driver.Value{oldRow}}, stubStep{err: boom}).ListTeams(ctx)

	// UpsertTeam: getTeam err, validation err, UPDATE existing path, exec err
	_, _ = d1With(stubStep{err: boom}).UpsertTeam(ctx, UpsertTeamInput{TeamID: "t1"})
	_, _ = d1With(stubStep{columns: tCols, rows: nil}).UpsertTeam(ctx, UpsertTeamInput{})
	_, _ = d1With(stubStep{columns: tCols, rows: [][]driver.Value{disRow}}, stubStep{rowsAffected: 1}, stubStep{columns: tCols, rows: [][]driver.Value{disRow}}).UpsertTeam(ctx, UpsertTeamInput{TeamID: "t1", TeamName: "T1"})
	_, _ = d1With(stubStep{columns: tCols, rows: nil}, stubStep{err: boom}).UpsertTeam(ctx, UpsertTeamInput{TeamID: "t1", RawAPIKey: "k"})
}
