package gateway

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
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

func anthropicToCanonical(in anthropicRequestIn) types.ChatCompletionRequest {
	var msgs []types.ChatMessage
	if sys := extractAnthropicSystemText(in.System); sys != "" {
		msgs = append(msgs, types.ChatMessage{Role: "system", Content: sys})
	}
	for _, m := range in.Messages {
		msgs = append(msgs, convertAnthropicMessage(m)...)
	}

	var tools []types.ToolDefinition
	for _, t := range in.Tools {
		tools = append(tools, types.ToolDefinition{
			Type: "function",
			Function: types.ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	return types.ChatCompletionRequest{
		Model:       in.Model,
		Messages:    msgs,
		Tools:       tools,
		ToolChoice:  in.ToolChoice,
		MaxTokens:   in.MaxTokens,
		Temperature: in.Temperature,
		TopP:        in.TopP,
		Stream:      in.Stream,
		User:        in.Metadata.UserID,
	}
}

func extractAnthropicSystemText(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var b strings.Builder
		for _, blk := range blocks {
			if (blk.Type == "text" || blk.Type == "") && blk.Text != "" {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(blk.Text)
			}
		}
		return b.String()
	}
	return ""
}

func convertAnthropicMessage(m anthropicMessageIn) []types.ChatMessage {
	trimmed := strings.TrimSpace(string(m.Content))
	if trimmed == "" || trimmed == "null" {
		return []types.ChatMessage{{Role: m.Role, Content: ""}}
	}
	var asString string
	if err := json.Unmarshal(m.Content, &asString); err == nil {
		return []types.ChatMessage{{Role: m.Role, Content: asString}}
	}

	var blocks []struct {
		Type      string          `json:"type"`
		Text      string          `json:"text"`
		ID        string          `json:"id"`
		Name      string          `json:"name"`
		Input     json.RawMessage `json:"input"`
		ToolUseID string          `json:"tool_use_id"`
		Content   json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(m.Content, &blocks); err != nil {
		return []types.ChatMessage{{Role: m.Role, Content: ""}}
	}

	var textBuf strings.Builder
	var toolCalls []types.ToolCall
	var toolResults []types.ChatMessage

	for _, blk := range blocks {
		switch blk.Type {
		case "tool_use":
			args := strings.TrimSpace(string(blk.Input))
			if args == "" {
				args = "{}"
			}
			toolCalls = append(toolCalls, types.ToolCall{
				ID:   blk.ID,
				Type: "function",
				Function: types.ToolCallFunction{
					Name:      blk.Name,
					Arguments: args,
				},
			})
		case "tool_result":
			resText := extractAnthropicSystemText(blk.Content)
			if resText == "" && len(blk.Content) > 0 {
				resText = string(blk.Content)
			}
			toolResults = append(toolResults, types.ChatMessage{
				Role:       "tool",
				ToolCallID: blk.ToolUseID,
				Content:    resText,
			})
		default:
			if blk.Text != "" {
				if textBuf.Len() > 0 {
					textBuf.WriteByte('\n')
				}
				textBuf.WriteString(blk.Text)
			}
		}
	}

	var out []types.ChatMessage
	if textBuf.Len() > 0 || len(toolCalls) > 0 || len(toolResults) == 0 {
		out = append(out, types.ChatMessage{
			Role:      m.Role,
			Content:   textBuf.String(),
			ToolCalls: toolCalls,
		})
	}
	out = append(out, toolResults...)
	return out
}

func anthropicStopReason(finishReason string, hasTools bool) string {
	if hasTools || finishReason == "tool_calls" {
		return "tool_use"
	}
	if finishReason == "length" {
		return "max_tokens"
	}
	return "end_turn"
}

func buildAnthropicResponse(resp types.ChatCompletionResponse) map[string]any {
	var choice types.ChatChoice
	if len(resp.Choices) > 0 {
		choice = resp.Choices[0]
	}
	content := make([]map[string]any, 0, 1+len(choice.Message.ToolCalls))
	if choice.Message.Content != "" || len(choice.Message.ToolCalls) == 0 {
		content = append(content, map[string]any{
			"type": "text",
			"text": choice.Message.Content,
		})
	}
	for _, tc := range choice.Message.ToolCalls {
		var inputObj any = map[string]any{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputObj); err != nil {
			inputObj = map[string]any{}
		}
		content = append(content, map[string]any{
			"type":  "tool_use",
			"id":    tc.ID,
			"name":  tc.Function.Name,
			"input": inputObj,
		})
	}
	return map[string]any{
		"id":          resp.ID,
		"type":        "message",
		"role":        "assistant",
		"model":       resp.Model,
		"stop_reason": anthropicStopReason(choice.FinishReason, len(choice.Message.ToolCalls) > 0),
		"content":     content,
		"usage": map[string]int{
			"input_tokens":  resp.Usage.PromptTokens,
			"output_tokens": resp.Usage.CompletionTokens,
		},
		"gateway": resp.Gateway,
	}
}

func writeAnthropicSSE(w http.ResponseWriter, resp types.ChatCompletionResponse) {
	startSSE(w)

	var choice types.ChatChoice
	if len(resp.Choices) > 0 {
		choice = resp.Choices[0]
	}
	stopReason := anthropicStopReason(choice.FinishReason, len(choice.Message.ToolCalls) > 0)

	writeSSEData(w, "message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            resp.ID,
			"type":          "message",
			"role":          "assistant",
			"model":         resp.Model,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]int{
				"input_tokens":  resp.Usage.PromptTokens,
				"output_tokens": 0,
			},
		},
	})

	blockIdx := 0
	if choice.Message.Content != "" || len(choice.Message.ToolCalls) == 0 {
		writeSSEData(w, "content_block_start", map[string]any{
			"type":          "content_block_start",
			"index":         blockIdx,
			"content_block": map[string]string{"type": "text", "text": ""},
		})
		for _, chunk := range splitSSEChunks(choice.Message.Content, 32) {
			writeSSEData(w, "content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": blockIdx,
				"delta": map[string]string{"type": "text_delta", "text": chunk},
			})
		}
		writeSSEData(w, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": blockIdx,
		})
		blockIdx++
	}

	for _, tc := range choice.Message.ToolCalls {
		writeSSEData(w, "content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": blockIdx,
			"content_block": map[string]any{
				"type":  "tool_use",
				"id":    tc.ID,
				"name":  tc.Function.Name,
				"input": map[string]any{},
			},
		})
		args := tc.Function.Arguments
		if args == "" {
			args = "{}"
		}
		writeSSEData(w, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": blockIdx,
			"delta": map[string]string{"type": "input_json_delta", "partial_json": args},
		})
		writeSSEData(w, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": blockIdx,
		})
		blockIdx++
	}

	writeSSEData(w, "message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   stopReason,
			"stop_sequence": nil,
		},
		"usage": map[string]int{"output_tokens": resp.Usage.CompletionTokens},
	})
	writeSSEData(w, "message_stop", map[string]string{"type": "message_stop"})
}
