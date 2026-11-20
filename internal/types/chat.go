package types

import "encoding/json"

// ChatMessage represents a single message in a canonical chat completion request.
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ChatCompletionRequest represents an OpenAI-compatible /v1/chat/completions payload.
type ChatCompletionRequest struct {
	Model        string           `json:"model"`
	Messages     []ChatMessage    `json:"messages"`
	Tools        []ToolDefinition `json:"tools,omitempty"`
	ToolChoice   json.RawMessage  `json:"tool_choice,omitempty"`
	MaxTokens    int              `json:"max_tokens,omitempty"`
	Temperature  *float64         `json:"temperature,omitempty"`
	TopP         *float64         `json:"top_p,omitempty"`
	Stream       bool             `json:"stream,omitempty"`
	User         string           `json:"user,omitempty"`
	RouteHint    string           `json:"route_hint,omitempty"`
	ProviderHint string           `json:"provider_hint,omitempty"`
}

// Usage tracks prompt and completion token counts.
type Usage struct {
	PromptTokens     int  `json:"prompt_tokens"`
	CompletionTokens int  `json:"completion_tokens"`
	TotalTokens      int  `json:"total_tokens"`
	Estimated        bool `json:"estimated,omitempty"`
}

// ChatChoice is a single completion candidate returned to the caller.
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// GatewayMeta exposes control-plane telemetry in responses without breaking SDK clients.
type GatewayMeta struct {
	RequestID       string         `json:"request_id"`
	TeamID          string         `json:"team_id"`
	ToolName        string         `json:"tool_name,omitempty"`
	InboundProtocol string         `json:"inbound_protocol,omitempty"`
	RequestedModel  string         `json:"requested_model"`
	RoutedModel     string         `json:"routed_model"`
	Provider        string         `json:"provider"`
	RouteReason     string         `json:"route_reason"`
	FallbackUsed    bool           `json:"fallback_used"`
	PIIAction       string         `json:"pii_action"`
	PIIDetected     int            `json:"pii_detected"`
	PIIFindings     map[string]int `json:"pii_findings,omitempty"`
	CostMicros      int64          `json:"cost_micros"`
	RemainingMicros int64          `json:"remaining_micros"`
	LatencyMs       int64          `json:"latency_ms"`
}

// ChatCompletionResponse mirrors the OpenAI chat completion response format.
type ChatCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   Usage        `json:"usage"`
	Gateway *GatewayMeta `json:"gateway,omitempty"`
}

// ErrorResponse follows OpenAI's error JSON envelope.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail provides structured error details.
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Param   string `json:"param,omitempty"`
}
