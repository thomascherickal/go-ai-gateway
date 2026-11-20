package gateway

import (
	"encoding/json"
	"net/http"

	"github.com/thomas/go-ai-gateway/internal/router"
)

type anthropicMessageIn struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type anthropicToolIn struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type anthropicRequestIn struct {
	Model       string               `json:"model"`
	System      json.RawMessage      `json:"system"`
	Messages    []anthropicMessageIn `json:"messages"`
	Tools       []anthropicToolIn    `json:"tools"`
	ToolChoice  json.RawMessage      `json:"tool_choice"`
	MaxTokens   int                  `json:"max_tokens"`
	Temperature *float64             `json:"temperature"`
	TopP        *float64             `json:"top_p"`
	Stream      bool                 `json:"stream"`
	Metadata    struct {
		UserID string `json:"user_id"`
	} `json:"metadata"`
}

func (g *Gateway) handleAnthropicMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed", "")
		return
	}
	apiKey := extractFirstKey(r, "x-api-key", "X-Team-Key")
	bodyBytes, ok := readLimitedBody(w, r)
	if !ok {
		return
	}
	var in anthropicRequestIn
	if err := json.Unmarshal(bodyBytes, &in); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "invalid Anthropic JSON payload", "")
		return
	}

	canonical := anthropicToCanonical(in)
	resp, ok := g.executeChatPipeline(w, r, "anthropic", apiKey, canonical)
	if !ok {
		return
	}
	if in.Stream {
		writeAnthropicSSE(w, resp)
		return
	}
	writeJSON(w, http.StatusOK, buildAnthropicResponse(resp))
}

func (g *Gateway) handleAnthropicCountTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed", "")
		return
	}
	apiKey := extractFirstKey(r, "x-api-key", "X-Team-Key")
	if _, err := g.store.LookupTeamByKey(r.Context(), apiKey); err != nil {
		writeOpenAIError(w, http.StatusUnauthorized, "authentication_error", "invalid or missing team API key", "invalid_api_key")
		return
	}
	bodyBytes, ok := readLimitedBody(w, r)
	if !ok {
		return
	}
	var in anthropicRequestIn
	if err := json.Unmarshal(bodyBytes, &in); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "invalid Anthropic JSON payload", "")
		return
	}
	canonical := anthropicToCanonical(in)
	tokens := router.EstimateMessagesTokens(canonical.Messages)
	if len(in.Tools) > 0 {
		b, _ := json.Marshal(in.Tools)
		tokens += router.EstimateTextTokens(string(b))
	}
	writeJSON(w, http.StatusOK, map[string]int{"input_tokens": tokens})
}
