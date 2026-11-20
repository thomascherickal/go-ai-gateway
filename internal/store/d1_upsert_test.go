package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestD1StoreUpsertAndRollover(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	nowStr := now.Format(time.RFC3339)
	teamCols := []string{
		"team_id", "team_name", "api_key_hash", "monthly_limit_micros",
		"spent_micros", "period_ym", "max_tier", "pii_mode", "enabled",
		"created_at", "updated_at",
	}
	oldRow := []driver.Value{
		"eng", "Eng", "hash1", int64(5000), int64(300),
		"2026-10", "reasoning", "redact", int64(1), nowStr, nowStr,
	}
	newRow := []driver.Value{
		"eng", "Eng", "hash1", int64(5000), int64(0),
		"2026-11", "reasoning", "redact", int64(1), nowStr, nowStr,
	}

	conn := &stubConnector{
		steps: []stubStep{
			// UpsertTeam: check existing (empty), then INSERT, then SELECT
			{match: "FROM team_budgets WHERE team_id", columns: teamCols, rows: nil},
			{match: "INSERT INTO team_budgets", rowsAffected: 1},
			{match: "FROM team_budgets WHERE team_id", columns: teamCols, rows: [][]driver.Value{newRow}},
			// GetTeam triggers ensurePeriodRollover when period_ym != current month
			{match: "FROM team_budgets WHERE team_id", columns: teamCols, rows: [][]driver.Value{oldRow}},
			{match: "SET spent_micros = 0, period_ym", rowsAffected: 1},
		},
	}
	st := NewD1Store(func() (*sql.DB, error) { return sql.OpenDB(conn), nil })

	enabled := true
	team, err := st.UpsertTeam(context.Background(), UpsertTeamInput{
		TeamID: "eng", TeamName: "Eng", RawAPIKey: "gw_eng_key",
		MonthlyLimitMicros: 5000, MaxTier: types.TierReasoning, PIIMode: types.PIIModeRedact,
		Enabled: &enabled,
	})
	if err != nil || team.TeamID != "eng" {
		t.Fatalf("UpsertTeam: %+v err=%v", team, err)
	}

	rolled, err := st.GetTeam(context.Background(), "eng")
	if err != nil || rolled.SpentMicros != 0 {
		t.Fatalf("GetTeam rollover: %+v err=%v", rolled, err)
	}
}
