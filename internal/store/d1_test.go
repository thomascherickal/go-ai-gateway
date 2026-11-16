package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

type stubStep struct {
	match        string
	rowsAffected int64
	columns      []string
	rows         [][]driver.Value
}

type stubConnector struct {
	steps []stubStep
	idx   int
	calls []string
}

func (c *stubConnector) Connect(context.Context) (driver.Conn, error) { return c, nil }
func (c *stubConnector) Driver() driver.Driver                        { return nil }
func (c *stubConnector) Prepare(q string) (driver.Stmt, error)        { return nil, errors.New("unused") }
func (c *stubConnector) Close() error                                 { return nil }
func (c *stubConnector) Begin() (driver.Tx, error)                    { return nil, errors.New("no tx") }

func (c *stubConnector) nextStep(query string) (stubStep, error) {
	c.calls = append(c.calls, query)
	if c.idx >= len(c.steps) {
		return stubStep{}, errors.New("unexpected SQL call: " + query)
	}
	st := c.steps[c.idx]
	c.idx++
	if st.match != "" && !strings.Contains(query, st.match) {
		return stubStep{}, errors.New("SQL mismatch: got " + query + " want substring " + st.match)
	}
	return st, nil
}

func (c *stubConnector) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	st, err := c.nextStep(query)
	if err != nil {
		return nil, err
	}
	return driver.RowsAffected(st.rowsAffected), nil
}

func (c *stubConnector) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	st, err := c.nextStep(query)
	if err != nil {
		return nil, err
	}
	return &stubRows{cols: st.columns, data: st.rows}, nil
}

type stubRows struct {
	cols []string
	data [][]driver.Value
	pos  int
}

func (r *stubRows) Columns() []string { return r.cols }
func (r *stubRows) Close() error      { return nil }
func (r *stubRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.pos])
	r.pos++
	return nil
}

func TestD1StoreReserveBudgetAndAudit(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	nowStr := now.Format(time.RFC3339)

	t.Run("atomic reserve succeeds and returns updated team", func(t *testing.T) {
		conn := &stubConnector{
			steps: []stubStep{
				{match: "SET spent_micros = 0, period_ym", rowsAffected: 0},
				{match: "SET spent_micros = spent_micros + ?", rowsAffected: 1},
				{
					match:   "FROM team_budgets",
					columns: []string{"team_id", "team_name", "api_key_hash", "monthly_limit_micros", "spent_micros", "period_ym", "max_tier", "pii_mode", "enabled", "created_at", "updated_at"},
					rows: [][]driver.Value{
						{"platform", "Platform", "abc", int64(5000), int64(300), "2026-11", "reasoning", "redact", int64(1), nowStr, nowStr},
					},
				},
			},
		}
		st := NewD1Store(func() (*sql.DB, error) { return sql.OpenDB(conn), nil })
		team, err := st.ReserveBudget(context.Background(), "platform", 300, now)
		if err != nil {
			t.Fatalf("ReserveBudget failed: %v", err)
		}
		if team.SpentMicros != 300 || team.RemainingMicros != 4700 {
			t.Fatalf("unexpected team: %+v", team)
		}
	})

	t.Run("atomic reserve returns ErrBudgetExceeded when 0 rows updated and team enabled", func(t *testing.T) {
		conn := &stubConnector{
			steps: []stubStep{
				{match: "SET spent_micros = 0, period_ym", rowsAffected: 0},
				{match: "SET spent_micros = spent_micros + ?", rowsAffected: 0},
				{
					match:   "FROM team_budgets",
					columns: []string{"team_id", "team_name", "api_key_hash", "monthly_limit_micros", "spent_micros", "period_ym", "max_tier", "pii_mode", "enabled", "created_at", "updated_at"},
					rows: [][]driver.Value{
						{"sandbox", "Sandbox", "xyz", int64(1000), int64(950), "2026-11", "fast", "redact", int64(1), nowStr, nowStr},
					},
				},
			},
		}
		st := NewD1Store(func() (*sql.DB, error) { return sql.OpenDB(conn), nil })
		_, err := st.ReserveBudget(context.Background(), "sandbox", 100, now)
		if !errors.Is(err, ErrBudgetExceeded) {
			t.Fatalf("got %v, want ErrBudgetExceeded", err)
		}
	})

	t.Run("RecordAudit and ListAudit round-trip", func(t *testing.T) {
		conn := &stubConnector{
			steps: []stubStep{
				{match: "INSERT INTO audit_logs", rowsAffected: 1},
				{
					match:   "FROM audit_logs",
					columns: []string{"id", "created_at", "team_id", "tool_name", "inbound_protocol", "requested_model", "routed_model", "provider", "route_reason", "fallback_used", "status", "pii_detected", "pii_types_json", "pii_action", "prompt_tokens", "completion_tokens", "total_tokens", "cost_micros", "latency_ms", "prompt_sha256", "error_message"},
					rows: [][]driver.Value{
						{"gw_1", nowStr, "platform", "cli", "openai", "auto", "@cf/meta/llama-3.1-8b-instruct-fast", "workers-ai", "auto_fast_default", int64(0), "ok", int64(1), `{"email":1}`, "redacted", int64(10), int64(20), int64(30), int64(8), int64(42), "deadbeef", ""},
					},
				},
			},
		}
		st := NewD1Store(func() (*sql.DB, error) { return sql.OpenDB(conn), nil })
		if err := st.RecordAudit(context.Background(), types.AuditEntry{
			ID:             "gw_1",
			CreatedAt:      now,
			TeamID:         "platform",
			RequestedModel: "auto",
			Status:         types.StatusOK,
			PIIDetected:    1,
			PIIFindings:    map[string]int{"email": 1},
			PIIAction:      "redacted",
		}); err != nil {
			t.Fatal(err)
		}
		entries, err := st.ListAudit(context.Background(), types.AuditFilter{TeamID: "platform", PIIOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].PIIFindings["email"] != 1 {
			t.Fatalf("unexpected entries: %+v", entries)
		}
	})
}

