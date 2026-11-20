package types

import "time"

// ModelTier classifies model capability and cost tiers.
type ModelTier string

const (
	TierFast      ModelTier = "fast"
	TierBalanced  ModelTier = "balanced"
	TierReasoning ModelTier = "reasoning"
)

// TierRank returns an ordinal for comparing tier capabilities.
func TierRank(t ModelTier) int {
	switch t {
	case TierFast:
		return 1
	case TierBalanced:
		return 2
	case TierReasoning:
		return 3
	default:
		return 0
	}
}

// PIIMode controls how detected personal data or secrets are handled.
type PIIMode string

const (
	PIIModeRedact    PIIMode = "redact"
	PIIModeBlock     PIIMode = "block"
	PIIModeAuditOnly PIIMode = "audit_only"
)

// AuditStatus records the terminal outcome of a gateway request.
type AuditStatus string

const (
	StatusOK            AuditStatus = "ok"
	StatusBlockedBudget AuditStatus = "blocked_budget"
	StatusBlockedPII    AuditStatus = "blocked_pii"
	StatusBlockedPolicy AuditStatus = "blocked_policy"
	StatusUpstreamError AuditStatus = "upstream_error"
	StatusUnauthorized  AuditStatus = "unauthorized"
)

// TeamBudget defines a team's identity, spend cap, and data-protection policy.
type TeamBudget struct {
	TeamID             string    `json:"team_id"`
	TeamName           string    `json:"team_name"`
	APIKeyHash         string    `json:"-"`
	MonthlyLimitMicros int64     `json:"monthly_limit_micros"`
	SpentMicros        int64     `json:"spent_micros"`
	RemainingMicros    int64     `json:"remaining_micros"`
	PeriodYM           string    `json:"period_ym"`
	MaxTier            ModelTier `json:"max_tier"`
	PIIMode            PIIMode   `json:"pii_mode"`
	Enabled            bool      `json:"enabled"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
