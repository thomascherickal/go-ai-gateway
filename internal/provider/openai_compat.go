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

// OpenAICompatProvider calls any OpenAI-compatible /v1/chat/completions HTTP endpoint.
type OpenAICompatProvider struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Label      string
}

func (p *OpenAICompatProvider) Name() string {
	if p.Label != "" {
		return p.Label
	}
	return "openai-compat"
}

func (p *OpenAICompatProvider) Chat(ctx context.Context, model router.ModelSpec, req types.ChatCompletionRequest) (Result, error) {
	if p.BaseURL == "" {
		return Result{}, errors.New("upstream base URL is empty")
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = model.DefaultMaxTokens
	}
	payload := struct {
		Model       string                 `json:"model"`
		Messages    []types.ChatMessage    `json:"messages"`
		Tools       []types.ToolDefinition `json:"tools,omitempty"`
		ToolChoice  json.RawMessage        `json:"tool_choice,omitempty"`
		MaxTokens   int                    `json:"max_tokens,omitempty"`
		Temperature *float64               `json:"temperature,omitempty"`
		TopP        *float64               `json:"top_p,omitempty"`
	}{
		Model:       model.ID,
		Messages:    req.Messages,
		Tools:       req.Tools,
		ToolChoice:  req.ToolChoice,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}

	headers := map[string]string{}
	if p.APIKey != "" {
		headers["Authorization"] = "Bearer " + p.APIKey
	}

	endpoint := strings.TrimRight(p.BaseURL, "/") + "/chat/completions"
	respBytes, err := doJSONPost(ctx, p.HTTPClient, endpoint, headers, payload, p.Name())
	if err != nil {
		return Result{}, err
	}

	var parsed types.ChatCompletionResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return Result{}, fmt.Errorf("invalid upstream JSON: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return Result{}, errors.New("upstream returned no choices")
	}

	msg := parsed.Choices[0].Message
	finish := parsed.Choices[0].FinishReason
	if finish == "" {
		if len(msg.ToolCalls) > 0 {
			finish = "tool_calls"
		} else {
			finish = "stop"
		}
	}
	return Result{
		Content:      msg.Content,
		ToolCalls:    msg.ToolCalls,
		FinishReason: finish,
		Usage:        ensureUsage(parsed.Usage, req.Messages, msg.Content),
	}, nil
}
