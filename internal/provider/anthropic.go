package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

const defaultAnthropicVersion = "2023-06-01"

// AnthropicProvider calls Anthropic's native /v1/messages endpoint.
type AnthropicProvider struct {
	BaseURL    string // defaults to https://api.anthropic.com
	APIKey     string
	Version    string
	HTTPClient *http.Client
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

type antBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type antMsg struct {
	Role    string     `json:"role"`
	Content []antBlock `json:"content"`
}

func (p *AnthropicProvider) Chat(ctx context.Context, model router.ModelSpec, req types.ChatCompletionRequest) (Result, error) {
	if p.APIKey == "" {
		return Result{}, errors.New("ANTHROPIC_API_KEY is not configured")
	}
	base := strings.TrimRight(p.BaseURL, "/")
	if base == "" {
		base = "https://api.anthropic.com"
	}
	version := p.Version
	if version == "" {
		version = defaultAnthropicVersion
	}

	sysParts, msgs := buildAnthropicMessages(req.Messages)

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = model.DefaultMaxTokens
	}
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	payload := map[string]any{
		"model":      model.ID,
		"max_tokens": maxTokens,
		"messages":   msgs,
	}
	if len(sysParts) > 0 {
		payload["system"] = strings.Join(sysParts, "\n\n")
	}
	if len(req.Tools) > 0 {
		antTools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			schema := t.Function.Parameters
			if len(schema) == 0 {
				schema = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			antTools = append(antTools, map[string]any{
				"name":         t.Function.Name,
				"description":  t.Function.Description,
				"input_schema": schema,
			})
		}
		payload["tools"] = antTools
	}
	if len(req.ToolChoice) > 0 {
		payload["tool_choice"] = mapOpenAIToolChoiceToAnthropic(req.ToolChoice)
	}
	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		payload["top_p"] = *req.TopP
	}

	headers := map[string]string{
		"x-api-key":         p.APIKey,
		"anthropic-version": version,
	}
	respBytes, err := doJSONPost(ctx, p.HTTPClient, base+"/v1/messages", headers, payload, "anthropic")
	if err != nil {
		return Result{}, err
	}

	var parsed struct {
		Content    []antBlock `json:"content"`
		StopReason string     `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return Result{}, fmt.Errorf("invalid anthropic JSON: %w", err)
	}

	var outText strings.Builder
	var toolCalls []types.ToolCall
	for _, blk := range parsed.Content {
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
		default:
			outText.WriteString(blk.Text)
		}
	}
	content := outText.String()
	if content == "" && len(toolCalls) == 0 {
		return Result{}, errors.New("anthropic returned empty content")
	}

	finish := "stop"
	switch parsed.StopReason {
	case "max_tokens":
		finish = "length"
	case "tool_use":
		finish = "tool_calls"
	}

	u := types.Usage{
		PromptTokens:     parsed.Usage.InputTokens,
		CompletionTokens: parsed.Usage.OutputTokens,
	}
	return Result{
		Content:      content,
		ToolCalls:    toolCalls,
		FinishReason: finish,
		Usage:        ensureUsage(u, req.Messages, content),
	}, nil
}

func buildAnthropicMessages(in []types.ChatMessage) ([]string, []antMsg) {
	var sysParts []string
	var msgs []antMsg

	appendMsg := func(role string, blocks []antBlock) {
		if len(blocks) == 0 {
			return
		}
		if len(msgs) > 0 && msgs[len(msgs)-1].Role == role {
			msgs[len(msgs)-1].Content = append(msgs[len(msgs)-1].Content, blocks...)
			return
		}
		msgs = append(msgs, antMsg{Role: role, Content: blocks})
	}

	for _, m := range in {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		switch role {
		case "system":
			if m.Content != "" {
				sysParts = append(sysParts, m.Content)
			}
		case "tool":
			appendMsg("user", []antBlock{{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
			}})
		case "assistant":
			var blocks []antBlock
			if m.Content != "" {
				blocks = append(blocks, antBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				rawArgs := json.RawMessage(strings.TrimSpace(tc.Function.Arguments))
				if len(rawArgs) == 0 || !json.Valid(rawArgs) {
					rawArgs = json.RawMessage(`{}`)
				}
				blocks = append(blocks, antBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: rawArgs,
				})
			}
			appendMsg("assistant", blocks)
		default:
			appendMsg("user", []antBlock{{Type: "text", Text: m.Content}})
		}
	}
	if len(msgs) == 0 {
		msgs = append(msgs, antMsg{Role: "user", Content: []antBlock{{Type: "text", Text: ""}}})
	}
	return sysParts, msgs
}

func mapOpenAIToolChoiceToAnthropic(raw json.RawMessage) any {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch s {
		case "required":
			return map[string]string{"type": "any"}
		case "none":
			return map[string]string{"type": "none"}
		case "auto":
			return map[string]string{"type": "auto"}
		}
	}
	return json.RawMessage(raw)
}
