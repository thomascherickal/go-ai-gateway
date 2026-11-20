package types

import "time"

// AuditEntry is an immutable record written for every gateway request.
type AuditEntry struct {
	ID               string         `json:"id"`
	CreatedAt        time.Time      `json:"created_at"`
	TeamID           string         `json:"team_id"`
	ToolName         string         `json:"tool_name,omitempty"`
	InboundProtocol  string         `json:"inbound_protocol,omitempty"`
	RequestedModel   string         `json:"requested_model"`
	RoutedModel      string         `json:"routed_model,omitempty"`
	Provider         string         `json:"provider,omitempty"`
	RouteReason      string         `json:"route_reason,omitempty"`
	FallbackUsed     bool           `json:"fallback_used"`
	Status           AuditStatus    `json:"status"`
	PIIDetected      int            `json:"pii_detected"`
	PIIFindings      map[string]int `json:"pii_findings,omitempty"`
	PIIAction        string         `json:"pii_action"`
	PromptTokens     int            `json:"prompt_tokens"`
	CompletionTokens int            `json:"completion_tokens"`
	TotalTokens      int            `json:"total_tokens"`
	CostMicros       int64          `json:"cost_micros"`
	LatencyMs        int64          `json:"latency_ms"`
	PromptSHA256     string         `json:"prompt_sha256,omitempty"`
	ErrorMessage     string         `json:"error_message,omitempty"`
}

// AuditFilter narrows audit log queries.
type AuditFilter struct {
	TeamID  string
	Status  AuditStatus
	Model   string
	PIIOnly bool
	Limit   int
}

// TeamSpendSummary aggregates usage for a single team.
type TeamSpendSummary struct {
	TeamID             string `json:"team_id"`
	TeamName           string `json:"team_name"`
	MonthlyLimitMicros int64  `json:"monthly_limit_micros"`
	SpentMicros        int64  `json:"spent_micros"`
	RequestCount       int64  `json:"request_count"`
	BlockedCount       int64  `json:"blocked_count"`
	PIIDetections      int64  `json:"pii_detections"`
}

// AuditSummary provides control-plane wide visibility across teams.
type AuditSummary struct {
	TotalRequests    int64              `json:"total_requests"`
	SuccessfulCount  int64              `json:"successful_count"`
	BlockedBudget    int64              `json:"blocked_budget"`
	BlockedPII       int64              `json:"blocked_pii"`
	UpstreamErrors   int64              `json:"upstream_errors"`
	TotalPIIDetected int64              `json:"total_pii_detected"`
	TotalTokens      int64              `json:"total_tokens"`
	TotalCostMicros  int64              `json:"total_cost_micros"`
	Teams            []TeamSpendSummary `json:"teams"`
}
