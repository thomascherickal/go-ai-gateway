package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"
)

func TestD1StoreExtraOperations(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	nowStr := now.Format(time.RFC3339)
	teamCols := []string{
		"team_id", "team_name", "api_key_hash", "monthly_limit_micros",
		"spent_micros", "period_ym", "max_tier", "pii_mode", "enabled",
		"created_at", "updated_at",
	}
	teamRow := []driver.Value{
		"platform", "Platform", "abc", int64(5000), int64(300),
		"2026-11", "reasoning", "redact", int64(1), nowStr, nowStr,
	}

	conn := &stubConnector{
		steps: []stubStep{
			{match: "SET spent_micros = MAX(0", rowsAffected: 1},
			{match: "FROM team_budgets WHERE team_id", columns: teamCols, rows: [][]driver.Value{teamRow}},
			{match: "SET spent_micros = 0, period_ym", rowsAffected: 1},
			{match: "FROM team_budgets WHERE team_id", columns: teamCols, rows: [][]driver.Value{teamRow}},
			{match: "FROM team_budgets WHERE team_id", columns: teamCols, rows: [][]driver.Value{teamRow}},
			{match: "FROM team_budgets WHERE api_key_hash", columns: teamCols, rows: [][]driver.Value{teamRow}},
			{match: "FROM team_budgets ORDER BY", columns: teamCols, rows: [][]driver.Value{teamRow}},
			{match: "COUNT(*)", columns: []string{"total", "ok", "bb", "bp", "ue", "pii", "tok", "cost"}, rows: [][]driver.Value{
				{int64(2), int64(1), int64(0), int64(1), int64(0), int64(3), int64(100), int64(50)},
			}},
			{match: "GROUP BY tb.team_id", columns: []string{"team_id", "team_name", "limit", "spent", "reqs", "blocked", "pii"}, rows: [][]driver.Value{
				{"platform", "Platform", int64(5000), int64(300), int64(2), int64(1), int64(3)},
			}},
		},
	}
	st := NewD1Store(func() (*sql.DB, error) { return sql.OpenDB(conn), nil })
	if st.BackendName() == "" {
		t.Fatal("expected non-empty BackendName")
	}
	if _, err := st.ReconcileBudget(context.Background(), "platform", -100, now); err != nil {
		t.Fatalf("ReconcileBudget: %v", err)
	}
	if _, err := st.ResetTeamBudget(context.Background(), "platform", now); err != nil {
		t.Fatalf("ResetTeamBudget: %v", err)
	}
	if team, err := st.GetTeam(context.Background(), "platform"); err != nil || team.TeamID != "platform" {
		t.Fatalf("GetTeam: %+v err=%v", team, err)
	}
	if team, err := st.LookupTeamByKey(context.Background(), "raw-key"); err != nil || team.TeamID != "platform" {
		t.Fatalf("LookupTeamByKey: %+v err=%v", team, err)
	}
	if teams, err := st.ListTeams(context.Background()); err != nil || len(teams) != 1 {
		t.Fatalf("ListTeams: len=%d err=%v", len(teams), err)
	}
	if sum, err := st.GetSummary(context.Background()); err != nil || sum.TotalRequests != 2 {
		t.Fatalf("GetSummary: %+v err=%v", sum, err)
	}
}
