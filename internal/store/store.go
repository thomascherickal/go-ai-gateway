package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
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

// DemoTeams returns pre-configured teams useful for local development and tests.
func DemoTeams(_ time.Time) []UpsertTeamInput {
	enabled := true
	return []UpsertTeamInput{
		{
			TeamID:             "platform",
			TeamName:           "Platform Engineering",
			RawAPIKey:          "gw_platform_demo_key",
			MonthlyLimitMicros: 50_000_000,
			MaxTier:            types.TierReasoning,
			PIIMode:            types.PIIModeRedact,
			Enabled:            &enabled,
		},
		{
			TeamID:             "support",
			TeamName:           "Customer Support",
			RawAPIKey:          "gw_support_demo_key",
			MonthlyLimitMicros: 15_000_000,
			MaxTier:            types.TierBalanced,
			PIIMode:            types.PIIModeBlock,
			Enabled:            &enabled,
		},
		{
			TeamID:             "sandbox",
			TeamName:           "Internal Sandbox",
			RawAPIKey:          "gw_sandbox_demo_key",
			MonthlyLimitMicros: 1_000_000,
			MaxTier:            types.TierFast,
			PIIMode:            types.PIIModeRedact,
			Enabled:            &enabled,
		},
	}
}

// mergeTeamInput validates and applies UpsertTeamInput onto an optional existing TeamBudget.
func mergeTeamInput(existing *types.TeamBudget, in UpsertTeamInput, now time.Time) (types.TeamBudget, error) {
	if strings.TrimSpace(in.TeamID) == "" {
		return types.TeamBudget{}, errors.New("team_id is required")
	}
	if existing == nil && strings.TrimSpace(in.RawAPIKey) == "" {
		return types.TeamBudget{}, errors.New("api_key is required when creating a new team")
	}

	var t types.TeamBudget
	if existing != nil {
		t = *existing
	} else {
		t = types.TeamBudget{
			TeamID:    in.TeamID,
			PeriodYM:  PeriodYM(now),
			MaxTier:   types.TierReasoning,
			PIIMode:   types.PIIModeRedact,
			Enabled:   true,
			CreatedAt: now,
		}
	}

	if in.TeamName != "" {
		t.TeamName = in.TeamName
	} else if t.TeamName == "" {
		t.TeamName = in.TeamID
	}
	if in.MonthlyLimitMicros > 0 {
		t.MonthlyLimitMicros = in.MonthlyLimitMicros
	}
	if in.MaxTier != "" {
		t.MaxTier = in.MaxTier
	}
	if in.PIIMode != "" {
		t.PIIMode = in.PIIMode
	}
	if in.Enabled != nil {
		t.Enabled = *in.Enabled
	}
	if strings.TrimSpace(in.RawAPIKey) != "" {
		t.APIKeyHash = HashKey(in.RawAPIKey)
	}
	t.UpdatedAt = now
	t.RemainingMicros = maxInt64(0, t.MonthlyLimitMicros-t.SpentMicros)
	return t, nil
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
