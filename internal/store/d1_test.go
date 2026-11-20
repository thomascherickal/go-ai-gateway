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

func TestD1StoreReserveBudgetAndAudit(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	nowStr := now.Format(time.RFC3339)
	teamCols := []string{"team_id", "team_name", "api_key_hash", "monthly_limit_micros", "spent_micros", "period_ym", "max_tier", "pii_mode", "enabled", "created_at", "updated_at"}

	t.Run("atomic reserve succeeds and returns updated team", func(t *testing.T) {
		conn := &stubConnector{
			steps: []stubStep{
				{match: "SET spent_micros = 0, period_ym", rowsAffected: 0},
				{match: "SET spent_micros = spent_micros + ?", rowsAffected: 1},
				{match: "FROM team_budgets", columns: teamCols, rows: [][]driver.Value{
					{"platform", "Platform", "abc", int64(5000), int64(300), "2026-11", "reasoning", "redact", int64(1), nowStr, nowStr},
				}},
			},
		}
		st := NewD1Store(func() (*sql.DB, error) { return sql.OpenDB(conn), nil })
		team, err := st.ReserveBudget(context.Background(), "platform", 300, now)
		if err != nil || team.SpentMicros != 300 || team.RemainingMicros != 4700 {
			t.Fatalf("unexpected result: %+v, err=%v", team, err)
		}
	})

	t.Run("atomic reserve returns ErrBudgetExceeded when 0 rows updated and team enabled", func(t *testing.T) {
		conn := &stubConnector{
			steps: []stubStep{
				{match: "SET spent_micros = 0, period_ym", rowsAffected: 0},
				{match: "SET spent_micros = spent_micros + ?", rowsAffected: 0},
				{match: "FROM team_budgets", columns: teamCols, rows: [][]driver.Value{
					{"sandbox", "Sandbox", "xyz", int64(1000), int64(950), "2026-11", "fast", "redact", int64(1), nowStr, nowStr},
				}},
			},
		}
		st := NewD1Store(func() (*sql.DB, error) { return sql.OpenDB(conn), nil })
		if _, err := st.ReserveBudget(context.Background(), "sandbox", 100, now); !errors.Is(err, ErrBudgetExceeded) {
			t.Fatalf("got %v, want ErrBudgetExceeded", err)
		}
	})

	t.Run("RecordAudit and ListAudit round-trip", func(t *testing.T) {
		auditCols := []string{"id", "created_at", "team_id", "tool_name", "inbound_protocol", "requested_model", "routed_model", "provider", "route_reason", "fallback_used", "status", "pii_detected", "pii_types_json", "pii_action", "prompt_tokens", "completion_tokens", "total_tokens", "cost_micros", "latency_ms", "prompt_sha256", "error_message"}
		conn := &stubConnector{
			steps: []stubStep{
				{match: "INSERT INTO audit_logs", rowsAffected: 1},
				{match: "FROM audit_logs", columns: auditCols, rows: [][]driver.Value{
					{"gw_1", nowStr, "platform", "cli", "openai", "auto", "@cf/meta/llama-3.1-8b-instruct-fast", "workers-ai", "auto_fast_default", int64(0), "ok", int64(1), `{"email":1}`, "redacted", int64(10), int64(20), int64(30), int64(8), int64(42), "deadbeef", ""},
				}},
			},
		}
		st := NewD1Store(func() (*sql.DB, error) { return sql.OpenDB(conn), nil })
		if err := st.RecordAudit(context.Background(), types.AuditEntry{
			ID: "gw_1", CreatedAt: now, TeamID: "platform", RequestedModel: "auto",
			Status: types.StatusOK, PIIDetected: 1, PIIFindings: map[string]int{"email": 1}, PIIAction: "redacted",
		}); err != nil {
			t.Fatal(err)
		}
		entries, err := st.ListAudit(context.Background(), types.AuditFilter{TeamID: "platform", PIIOnly: true})
		if err != nil || len(entries) != 1 || entries[0].PIIFindings["email"] != 1 {
			t.Fatalf("unexpected entries: %+v, err=%v", entries, err)
		}
	})
}
