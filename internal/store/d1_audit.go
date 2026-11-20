package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func (s *D1Store) RecordAudit(ctx context.Context, entry types.AuditEntry) error {
	return s.withDB(func(db *sql.DB) error {
		findingsJSON := "{}"
		if len(entry.PIIFindings) > 0 {
			if b, err := json.Marshal(entry.PIIFindings); err == nil {
				findingsJSON = string(b)
			}
		}
		fallbackInt := 0
		if entry.FallbackUsed {
			fallbackInt = 1
		}
		created := entry.CreatedAt
		if created.IsZero() {
			created = time.Now().UTC()
		}
		_, err := db.ExecContext(ctx, `
			INSERT INTO audit_logs (
				id, created_at, team_id, tool_name, inbound_protocol, requested_model, routed_model,
				provider, route_reason, fallback_used, status, pii_detected,
				pii_types_json, pii_action, prompt_tokens, completion_tokens,
				total_tokens, cost_micros, latency_ms, prompt_sha256, error_message
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			entry.ID, created.UTC().Format(time.RFC3339Nano), entry.TeamID, entry.ToolName, entry.InboundProtocol,
			entry.RequestedModel, entry.RoutedModel, entry.Provider, entry.RouteReason,
			fallbackInt, string(entry.Status), entry.PIIDetected, findingsJSON, entry.PIIAction,
			entry.PromptTokens, entry.CompletionTokens, entry.TotalTokens,
			entry.CostMicros, entry.LatencyMs, entry.PromptSHA256, entry.ErrorMessage,
		)
		return err
	})
}

func (s *D1Store) ListAudit(ctx context.Context, filter types.AuditFilter) ([]types.AuditEntry, error) {
	out := make([]types.AuditEntry, 0)
	err := s.withDB(func(db *sql.DB) error {
		var where []string
		var args []any
		if filter.TeamID != "" {
			where = append(where, "team_id = ?")
			args = append(args, filter.TeamID)
		}
		if filter.Status != "" {
			where = append(where, "status = ?")
			args = append(args, string(filter.Status))
		}
		if filter.Model != "" {
			where = append(where, "(routed_model = ? OR requested_model = ?)")
			args = append(args, filter.Model, filter.Model)
		}
		if filter.PIIOnly {
			where = append(where, "pii_detected > 0")
		}

		query := `
			SELECT id, created_at, team_id, tool_name, inbound_protocol, requested_model, routed_model,
			       provider, route_reason, fallback_used, status, pii_detected,
			       pii_types_json, pii_action, prompt_tokens, completion_tokens,
			       total_tokens, cost_micros, latency_ms, prompt_sha256, error_message
			FROM audit_logs`
		if len(where) > 0 {
			query += " WHERE " + strings.Join(where, " AND ")
		}
		query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT %d", normalizeLimit(filter.Limit))

		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			a, err := scanAudit(rows)
			if err != nil {
				return err
			}
			out = append(out, a)
		}
		return rows.Err()
	})
	return out, err
}
