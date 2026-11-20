package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func (s *D1Store) UpsertTeam(ctx context.Context, in UpsertTeamInput) (types.TeamBudget, error) {
	var out types.TeamBudget
	err := s.withDB(func(db *sql.DB) error {
		now := time.Now().UTC()
		existing, err := s.getTeamWithDB(ctx, db, in.TeamID)
		if err != nil && !errors.Is(err, ErrTeamNotFound) {
			return err
		}
		var existingPtr *types.TeamBudget
		if err == nil {
			existingPtr = &existing
		}
		merged, err := mergeTeamInput(existingPtr, in, now)
		if err != nil {
			return err
		}

		enabledInt := 0
		if merged.Enabled {
			enabledInt = 1
		}
		nowStr := now.Format(time.RFC3339)

		if existingPtr == nil {
			_, err = db.ExecContext(ctx, `
				INSERT INTO team_budgets (
					team_id, team_name, api_key_hash, monthly_limit_micros, spent_micros,
					period_ym, max_tier, pii_mode, enabled, created_at, updated_at
				) VALUES (?, ?, ?, ?, 0, ?, ?, ?, ?, ?, ?)`,
				merged.TeamID, merged.TeamName, merged.APIKeyHash, merged.MonthlyLimitMicros,
				merged.PeriodYM, string(merged.MaxTier), string(merged.PIIMode), enabledInt, nowStr, nowStr,
			)
		} else {
			_, err = db.ExecContext(ctx, `
				UPDATE team_budgets
				SET team_name = ?, api_key_hash = ?, monthly_limit_micros = ?,
				    max_tier = ?, pii_mode = ?, enabled = ?, updated_at = ?
				WHERE team_id = ?`,
				merged.TeamName, merged.APIKeyHash, merged.MonthlyLimitMicros,
				string(merged.MaxTier), string(merged.PIIMode), enabledInt, nowStr, merged.TeamID,
			)
		}
		if err != nil {
			return err
		}
		out, err = s.getTeamWithDB(ctx, db, merged.TeamID)
		return err
	})
	return out, err
}

func scanTeam(r rowScanner) (types.TeamBudget, error) {
	var t types.TeamBudget
	var maxTier, piiMode, createdStr, updatedStr string
	var enabledInt int
	if err := r.Scan(
		&t.TeamID, &t.TeamName, &t.APIKeyHash, &t.MonthlyLimitMicros, &t.SpentMicros,
		&t.PeriodYM, &maxTier, &piiMode, &enabledInt, &createdStr, &updatedStr,
	); err != nil {
		return types.TeamBudget{}, err
	}
	t.MaxTier = types.ModelTier(maxTier)
	t.PIIMode = types.PIIMode(piiMode)
	t.Enabled = enabledInt != 0
	t.RemainingMicros = maxInt64(0, t.MonthlyLimitMicros-t.SpentMicros)
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	return t, nil
}
