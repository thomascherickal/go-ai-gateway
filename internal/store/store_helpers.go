package store

import (
	"errors"
	"strings"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

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
