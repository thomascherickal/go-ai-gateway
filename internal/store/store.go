package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

var (
	ErrTeamNotFound   = errors.New("team not found")
	ErrTeamDisabled   = errors.New("team is disabled")
	ErrBudgetExceeded = errors.New("monthly team budget exceeded")
)

// UpsertTeamInput defines parameters for creating or updating a team policy.
type UpsertTeamInput struct {
	TeamID             string
	TeamName           string
	RawAPIKey          string // optional on update; required on create
	MonthlyLimitMicros int64
	MaxTier            types.ModelTier
	PIIMode            types.PIIMode
	Enabled            *bool
}

// Store abstracts persistence for team budgets and audit logs (Cloudflare D1 or in-memory).
type Store interface {
	BackendName() string
	LookupTeamByKey(ctx context.Context, rawKey string) (types.TeamBudget, error)
	GetTeam(ctx context.Context, teamID string) (types.TeamBudget, error)
	ListTeams(ctx context.Context) ([]types.TeamBudget, error)
	UpsertTeam(ctx context.Context, in UpsertTeamInput) (types.TeamBudget, error)
	ReserveBudget(ctx context.Context, teamID string, amountMicros int64, now time.Time) (types.TeamBudget, error)
	ReconcileBudget(ctx context.Context, teamID string, deltaMicros int64, now time.Time) (types.TeamBudget, error)
	ResetTeamBudget(ctx context.Context, teamID string, now time.Time) (types.TeamBudget, error)
	RecordAudit(ctx context.Context, entry types.AuditEntry) error
	ListAudit(ctx context.Context, filter types.AuditFilter) ([]types.AuditEntry, error)
	GetSummary(ctx context.Context) (types.AuditSummary, error)
}

// HashKey returns the SHA-256 hex digest of an API key.
func HashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// PeriodYM formats a timestamp as YYYY-MM in UTC for monthly budget windows.
func PeriodYM(t time.Time) string {
	return t.UTC().Format("2006-01")
}

func normalizeLimit(limit int) int {
	if limit <= 0 || limit > 200 {
		return 50
	}
	return limit
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
