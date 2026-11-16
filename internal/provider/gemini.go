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

// GeminiProvider calls Google Gemini's native generateContent REST API.
type GeminiProvider struct {
	BaseURL    string // defaults to https://generativelanguage.googleapis.com
	APIKey     string
	HTTPClient *http.Client
}

func (p *GeminiProvider) Name() string { return "gemini" }

type gemFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type gemFunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type gemPart struct {
	Text             string               `json:"text,omitempty"`
	FunctionCall     *gemFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *gemFunctionResponse `json:"functionResponse,omitempty"`
}

type gemContent struct {
	Role  string    `json:"role,omitempty"`
	Parts []gemPart `json:"parts"`
}

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
		decls := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			decl := map[string]any{
				"name":        t.Function.Name,
				"description": t.Function.Description,
			}
			if len(t.Function.Parameters) > 0 {
				decl["parameters"] = t.Function.Parameters
			}
			decls = append(decls, decl)
		}
		payload["tools"] = []map[string]any{{"functionDeclarations": decls}}
	}

	modelID := strings.TrimPrefix(model.ID, "models/")
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent", base, modelID)
	headers := map[string]string{"x-goog-api-key": p.APIKey}
	respBytes, err := doJSONPost(ctx, p.HTTPClient, endpoint, headers, payload, "gemini")
	if err != nil {
		return Result{}, err
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []gemPart `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return Result{}, fmt.Errorf("invalid gemini JSON: %w", err)
	}
	if len(parsed.Candidates) == 0 {
		return Result{}, errors.New("gemini returned no candidates")
	}

	var outText strings.Builder
	var toolCalls []types.ToolCall
	for idx, pt := range parsed.Candidates[0].Content.Parts {
		if pt.Text != "" {
			outText.WriteString(pt.Text)
		}
		if pt.FunctionCall != nil {
			argsBytes, _ := json.Marshal(pt.FunctionCall.Args)
			if len(argsBytes) == 0 || string(argsBytes) == "null" {
				argsBytes = []byte("{}")
			}
			toolCalls = append(toolCalls, types.ToolCall{
				ID:   fmt.Sprintf("call_gem_%d", idx+1),
				Type: "function",
				Function: types.ToolCallFunction{
					Name:      pt.FunctionCall.Name,
					Arguments: string(argsBytes),
				},
			})
		}
	}
	content := outText.String()
	if content == "" && len(toolCalls) == 0 {
		return Result{}, errors.New("gemini returned empty candidate text")
	}

	finish := "stop"
	if len(toolCalls) > 0 {
		finish = "tool_calls"
	} else if strings.EqualFold(parsed.Candidates[0].FinishReason, "MAX_TOKENS") {
		finish = "length"
	}

	u := types.Usage{
		PromptTokens:     parsed.UsageMetadata.PromptTokenCount,
		CompletionTokens: parsed.UsageMetadata.CandidatesTokenCount,
		TotalTokens:      parsed.UsageMetadata.TotalTokenCount,
	}
	return Result{
		Content:      content,
		ToolCalls:    toolCalls,
		FinishReason: finish,
		Usage:        ensureUsage(u, req.Messages, content),
	}, nil
}

func buildGeminiContents(in []types.ChatMessage) ([]gemPart, []gemContent) {
	var sysParts []gemPart
	var contents []gemContent
	toolNamesByID := make(map[string]string)

	for _, m := range in {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		switch role {
		case "system":
			if m.Content != "" {
				sysParts = append(sysParts, gemPart{Text: m.Content})
			}
		case "tool":
			name := m.Name
			if name == "" {
				name = toolNamesByID[m.ToolCallID]
			}
			if name == "" {
				name = "tool"
			}
			contents = append(contents, gemContent{
				Role: "user",
				Parts: []gemPart{{
					FunctionResponse: &gemFunctionResponse{
						Name:     name,
						Response: map[string]any{"result": m.Content},
					},
				}},
			})
		case "assistant", "model":
			var parts []gemPart
			if m.Content != "" {
				parts = append(parts, gemPart{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				toolNamesByID[tc.ID] = tc.Function.Name
				var argsMap map[string]any
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &argsMap)
				parts = append(parts, gemPart{
					FunctionCall: &gemFunctionCall{
						Name: tc.Function.Name,
						Args: argsMap,
					},
				})
			}
			if len(parts) > 0 {
				contents = append(contents, gemContent{Role: "model", Parts: parts})
			}
		default:
			contents = append(contents, gemContent{
				Role:  "user",
				Parts: []gemPart{{Text: m.Content}},
			})
		}
	}
	if len(contents) == 0 {
		contents = append(contents, gemContent{Role: "user", Parts: []gemPart{{Text: ""}}})
	}
	return sysParts, contents
}
