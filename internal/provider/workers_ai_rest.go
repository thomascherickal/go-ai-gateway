package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

// WorkersAIRESTProvider calls Cloudflare Workers AI REST API for standalone non-Worker deployments.
type WorkersAIRESTProvider struct {
	AccountID  string
	APIToken   string
	HTTPClient *http.Client
}

func (p *WorkersAIRESTProvider) Name() string { return "workers-ai-rest" }

func (p *WorkersAIRESTProvider) Chat(ctx context.Context, model router.ModelSpec, req types.ChatCompletionRequest) (Result, error) {
	if p.AccountID == "" || p.APIToken == "" {
		return Result{}, errors.New("CF_ACCOUNT_ID and CF_API_TOKEN are required for Workers AI REST mode")
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = model.DefaultMaxTokens
	}
	payload := map[string]any{
		"messages":   req.Messages,
		"max_tokens": maxTokens,
	}
	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}

	endpoint := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/run/%s", p.AccountID, model.ID)
	headers := map[string]string{"Authorization": "Bearer " + p.APIToken}
	respBytes, err := doJSONPost(ctx, p.HTTPClient, endpoint, headers, payload, "workers-ai REST")
	if err != nil {
		return Result{}, err
	}

	var envelope struct {
		Success bool `json:"success"`
		Result  struct {
			Response string       `json:"response"`
			Usage    *types.Usage `json:"usage"`
		} `json:"result"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		return Result{}, err
	}
	if !envelope.Success {
		msg := "workers-ai REST error"
		if len(envelope.Errors) > 0 {
			msg = envelope.Errors[0].Message
		}
		return Result{}, errors.New(msg)
	}

	var u types.Usage
	if envelope.Result.Usage != nil {
		u = *envelope.Result.Usage
	}
	return Result{
		Content:      envelope.Result.Response,
		FinishReason: "stop",
		Usage:        ensureUsage(u, req.Messages, envelope.Result.Response),
	}, nil
}

