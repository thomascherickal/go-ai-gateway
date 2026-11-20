package provider

import (
	"context"
	"errors"
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
		payload["tools"] = buildAnthropicTools(req.Tools)
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
	return parseAnthropicResponse(respBytes, req.Messages)
}
