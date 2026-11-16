package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func (s *D1Store) LookupTeamByKey(ctx context.Context, rawKey string) (types.TeamBudget, error) {
	if rawKey == "" {
		return types.TeamBudget{}, ErrTeamNotFound
	}
	hash := HashKey(rawKey)
	var out types.TeamBudget
	err := s.withDB(func(db *sql.DB) error {
		row := db.QueryRowContext(ctx, `
			SELECT team_id, team_name, api_key_hash, monthly_limit_micros, spent_micros,
			       period_ym, max_tier, pii_mode, enabled, created_at, updated_at
			FROM team_budgets
			WHERE api_key_hash = ?`, hash)
		t, err := scanTeam(row)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrTeamNotFound
			}
			return err
		}
		out, err = s.ensurePeriodRollover(ctx, db, t, time.Now().UTC())
		return err
	})
	return out, err
}

func (s *D1Store) GetTeam(ctx context.Context, teamID string) (types.TeamBudget, error) {
	var out types.TeamBudget
	err := s.withDB(func(db *sql.DB) error {
		t, err := s.getTeamWithDB(ctx, db, teamID)
		if err != nil {
			return err
		}
		out, err = s.ensurePeriodRollover(ctx, db, t, time.Now().UTC())
		return err
	})
	return out, err
}

func (s *D1Store) getTeamWithDB(ctx context.Context, db *sql.DB, teamID string) (types.TeamBudget, error) {
	row := db.QueryRowContext(ctx, `
		SELECT team_id, team_name, api_key_hash, monthly_limit_micros, spent_micros,
		       period_ym, max_tier, pii_mode, enabled, created_at, updated_at
		FROM team_budgets
		WHERE team_id = ?`, teamID)
	t, err := scanTeam(row)
	if errors.Is(err, sql.ErrNoRows) {
		return types.TeamBudget{}, ErrTeamNotFound
	}
	return t, err
}

func (s *D1Store) ensurePeriodRollover(ctx context.Context, db *sql.DB, t types.TeamBudget, now time.Time) (types.TeamBudget, error) {
	ym := PeriodYM(now)
	if t.PeriodYM == ym {
		return t, nil
	}
	nowStr := now.UTC().Format(time.RFC3339)
	if _, err := db.ExecContext(ctx, `
		UPDATE team_budgets
		SET spent_micros = 0, period_ym = ?, updated_at = ?
		WHERE team_id = ? AND period_ym <> ?`, ym, nowStr, t.TeamID, ym); err != nil {
		return types.TeamBudget{}, err
	}
	t.SpentMicros = 0
	t.PeriodYM = ym
	t.RemainingMicros = maxInt64(0, t.MonthlyLimitMicros)
	t.UpdatedAt = now
	return t, nil
}

func (s *D1Store) ListTeams(ctx context.Context) ([]types.TeamBudget, error) {
	out := make([]types.TeamBudget, 0)
	err := s.withDB(func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx, `
			SELECT team_id, team_name, api_key_hash, monthly_limit_micros, spent_micros,
			       period_ym, max_tier, pii_mode, enabled, created_at, updated_at
			FROM team_budgets
			ORDER BY team_id ASC`)
		if err != nil {
			return err
		}
		defer rows.Close()
		now := time.Now().UTC()
		for rows.Next() {
			t, err := scanTeam(rows)
			if err != nil {
				return err
			}
			t, err = s.ensurePeriodRollover(ctx, db, t, now)
			if err != nil {
				return err
			}
			out = append(out, t)
		}
		return rows.Err()
	})
	return out, err
}

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

func (s *D1Store) ReserveBudget(ctx context.Context, teamID string, amountMicros int64, now time.Time) (types.TeamBudget, error) {
	var out types.TeamBudget
	err := s.withDB(func(db *sql.DB) error {
		ym := PeriodYM(now)
		nowStr := now.UTC().Format(time.RFC3339)

		if _, err := db.ExecContext(ctx, `
			UPDATE team_budgets
			SET spent_micros = 0, period_ym = ?, updated_at = ?
			WHERE team_id = ? AND period_ym <> ?`, ym, nowStr, teamID, ym); err != nil {
			return err
		}

		res, err := db.ExecContext(ctx, `
			UPDATE team_budgets
			SET spent_micros = spent_micros + ?, updated_at = ?
			WHERE team_id = ? AND enabled = 1 AND (spent_micros + ?) <= monthly_limit_micros`,
			amountMicros, nowStr, teamID, amountMicros)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}

		t, getErr := s.getTeamWithDB(ctx, db, teamID)
		if getErr != nil {
			return getErr
		}
		out = t
		if affected == 0 {
			if !t.Enabled {
				return ErrTeamDisabled
			}
			return ErrBudgetExceeded
		}
		return nil
	})
	return out, err
}

func (s *D1Store) ReconcileBudget(ctx context.Context, teamID string, deltaMicros int64, now time.Time) (types.TeamBudget, error) {
	var out types.TeamBudget
	err := s.withDB(func(db *sql.DB) error {
		nowStr := now.UTC().Format(time.RFC3339)
		if _, err := db.ExecContext(ctx, `
			UPDATE team_budgets
			SET spent_micros = MAX(0, spent_micros + ?), updated_at = ?
			WHERE team_id = ?`, deltaMicros, nowStr, teamID); err != nil {
			return err
		}
		var err error
		out, err = s.getTeamWithDB(ctx, db, teamID)
		return err
	})
	return out, err
}

func (s *D1Store) ResetTeamBudget(ctx context.Context, teamID string, now time.Time) (types.TeamBudget, error) {
	var out types.TeamBudget
	err := s.withDB(func(db *sql.DB) error {
		nowStr := now.UTC().Format(time.RFC3339)
		res, err := db.ExecContext(ctx, `
			UPDATE team_budgets
			SET spent_micros = 0, period_ym = ?, updated_at = ?
			WHERE team_id = ?`, PeriodYM(now), nowStr, teamID)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err == nil && affected == 0 {
			return ErrTeamNotFound
		}
		out, err = s.getTeamWithDB(ctx, db, teamID)
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

