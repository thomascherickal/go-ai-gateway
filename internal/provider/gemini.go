package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

// GeminiProvider calls Google Gemini's native generateContent REST API.
type GeminiProvider struct {
	BaseURL    string // defaults to https://generativelanguage.googleapis.com
	APIKey     string
	HTTPClient *http.Client
}

func (p *GeminiProvider) Name() string { return "gemini" }

func (p *GeminiProvider) Chat(ctx context.Context, model router.ModelSpec, req types.ChatCompletionRequest) (Result, error) {
	if p.APIKey == "" {
		return Result{}, errors.New("GEMINI_API_KEY is not configured")
	}
	base := strings.TrimRight(p.BaseURL, "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com"
	}

	sysParts, contents := buildGeminiContents(req.Messages)
	genCfg := map[string]any{}
	if req.MaxTokens > 0 {
		genCfg["maxOutputTokens"] = req.MaxTokens
	} else if model.DefaultMaxTokens > 0 {
		genCfg["maxOutputTokens"] = model.DefaultMaxTokens
	}
	if req.Temperature != nil {
		genCfg["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		genCfg["topP"] = *req.TopP
	}

	payload := map[string]any{
		"contents":         contents,
		"generationConfig": genCfg,
	}
	if len(sysParts) > 0 {
		payload["systemInstruction"] = map[string]any{"parts": sysParts}
	}
	if len(req.Tools) > 0 {
		payload["tools"] = buildGeminiTools(req.Tools)
	}

	modelID := strings.TrimPrefix(model.ID, "models/")
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent", base, modelID)
	headers := map[string]string{"x-goog-api-key": p.APIKey}
	respBytes, err := doJSONPost(ctx, p.HTTPClient, endpoint, headers, payload, "gemini")
	if err != nil {
		return Result{}, err
	}
	return parseGeminiResponse(respBytes, req.Messages)
}
