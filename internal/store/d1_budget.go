package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

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
