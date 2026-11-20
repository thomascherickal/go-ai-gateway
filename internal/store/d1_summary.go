package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func (s *D1Store) GetSummary(ctx context.Context) (types.AuditSummary, error) {
	summary := types.AuditSummary{Teams: make([]types.TeamSpendSummary, 0)}
	err := s.withDB(func(db *sql.DB) error {
		row := db.QueryRowContext(ctx, `
			SELECT
				COUNT(*),
				COALESCE(SUM(CASE WHEN status = 'ok' THEN 1 ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN status = 'blocked_budget' THEN 1 ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN status = 'blocked_pii' THEN 1 ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN status = 'upstream_error' THEN 1 ELSE 0 END), 0),
				COALESCE(SUM(pii_detected), 0),
				COALESCE(SUM(total_tokens), 0),
				COALESCE(SUM(cost_micros), 0)
			FROM audit_logs`)
		if err := row.Scan(
			&summary.TotalRequests, &summary.SuccessfulCount, &summary.BlockedBudget,
			&summary.BlockedPII, &summary.UpstreamErrors, &summary.TotalPIIDetected,
			&summary.TotalTokens, &summary.TotalCostMicros,
		); err != nil {
			return err
		}

		rows, err := db.QueryContext(ctx, `
			SELECT tb.team_id, tb.team_name, tb.monthly_limit_micros, tb.spent_micros,
				COUNT(al.id) AS request_count,
				COALESCE(SUM(CASE WHEN al.status IN ('blocked_budget','blocked_pii','blocked_policy') THEN 1 ELSE 0 END), 0),
				COALESCE(SUM(al.pii_detected), 0)
			FROM team_budgets tb
			LEFT JOIN audit_logs al ON al.team_id = tb.team_id
			GROUP BY tb.team_id, tb.team_name, tb.monthly_limit_micros, tb.spent_micros
			ORDER BY tb.team_id ASC`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var ts types.TeamSpendSummary
			if err := rows.Scan(
				&ts.TeamID, &ts.TeamName, &ts.MonthlyLimitMicros, &ts.SpentMicros,
				&ts.RequestCount, &ts.BlockedCount, &ts.PIIDetections,
			); err != nil {
				return err
			}
			summary.Teams = append(summary.Teams, ts)
		}
		return rows.Err()
	})
	return summary, err
}

func scanAudit(r rowScanner) (types.AuditEntry, error) {
	var a types.AuditEntry
	var createdStr, statusStr, findingsJSON string
	var fallbackInt int
	if err := r.Scan(
		&a.ID, &createdStr, &a.TeamID, &a.ToolName, &a.InboundProtocol, &a.RequestedModel, &a.RoutedModel,
		&a.Provider, &a.RouteReason, &fallbackInt, &statusStr, &a.PIIDetected,
		&findingsJSON, &a.PIIAction, &a.PromptTokens, &a.CompletionTokens,
		&a.TotalTokens, &a.CostMicros, &a.LatencyMs, &a.PromptSHA256, &a.ErrorMessage,
	); err != nil {
		return types.AuditEntry{}, err
	}
	a.FallbackUsed = fallbackInt != 0
	a.Status = types.AuditStatus(statusStr)
	if parsed, err := time.Parse(time.RFC3339Nano, createdStr); err == nil {
		a.CreatedAt = parsed
	}
	if findingsJSON != "" && findingsJSON != "{}" {
		_ = json.Unmarshal([]byte(findingsJSON), &a.PIIFindings)
	}
	return a, nil
}
