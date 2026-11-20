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
			FROM team_budgets WHERE api_key_hash = ?`, hash)
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
		FROM team_budgets WHERE team_id = ?`, teamID)
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
		UPDATE team_budgets SET spent_micros = 0, period_ym = ?, updated_at = ?
		WHERE team_id = ? AND period_ym <> ?`, ym, nowStr, t.TeamID, ym); err != nil {
		return types.TeamBudget{}, err
	}
	t.SpentMicros, t.PeriodYM, t.RemainingMicros, t.UpdatedAt = 0, ym, maxInt64(0, t.MonthlyLimitMicros), now
	return t, nil
}

func (s *D1Store) ListTeams(ctx context.Context) ([]types.TeamBudget, error) {
	out := make([]types.TeamBudget, 0)
	err := s.withDB(func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx, `SELECT team_id, team_name, api_key_hash, monthly_limit_micros, spent_micros, period_ym, max_tier, pii_mode, enabled, created_at, updated_at FROM team_budgets ORDER BY team_id ASC`)
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
			if t, err = s.ensurePeriodRollover(ctx, db, t, now); err != nil {
				return err
			}
			out = append(out, t)
		}
		return rows.Err()
	})
	return out, err
}
