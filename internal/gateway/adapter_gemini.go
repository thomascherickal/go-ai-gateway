package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/types"
)

type geminiFunctionCallIn struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type geminiFunctionResponseIn struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

type geminiPartIn struct {
	Text             string                    `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCallIn     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponseIn `json:"functionResponse,omitempty"`
}

type geminiContentIn struct {
	Role  string         `json:"role"`
	Parts []geminiPartIn `json:"parts"`
}

type geminiToolDeclIn struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type geminiRequestIn struct {
	SystemInstruction *geminiContentIn  `json:"systemInstruction"`
	Contents          []geminiContentIn `json:"contents"`
	Tools             []struct {
		FunctionDeclarations []geminiToolDeclIn `json:"functionDeclarations"`
	} `json:"tools"`
	GenerationConfig struct {
		MaxOutputTokens int      `json:"maxOutputTokens"`
		Temperature     *float64 `json:"temperature"`
		TopP            *float64 `json:"topP"`
	} `json:"generationConfig"`
}

func (g *Gateway) handleGeminiModelList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed", "")
		return
	}
	specs := g.router.Catalog()
	models := make([]map[string]any, 0, len(specs))
	for _, s := range specs {
		models = append(models, map[string]any{
			"name":                       "models/" + s.ID,
			"displayName":                s.ID,
			"description":                s.Description,
			"supportedGenerationMethods": []string{"generateContent", "streamGenerateContent"},
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func (g *Gateway) handleGeminiModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed", "")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1beta/models/")
	path = strings.TrimPrefix(path, "/v1/models/")

	var modelName string
	streaming := false
	switch {
	case strings.HasSuffix(path, ":generateContent"):
		modelName = strings.TrimSuffix(path, ":generateContent")
	case strings.HasSuffix(path, ":streamGenerateContent"):
		modelName = strings.TrimSuffix(path, ":streamGenerateContent")
		streaming = true
	default:
		writeOpenAIError(w, http.StatusNotFound, "not_found", "unsupported Gemini endpoint; expected :generateContent or :streamGenerateContent", "")
		return
	}
	modelName = strings.Trim(modelName, "/")

	apiKey := extractFirstKey(r, "x-goog-api-key", "X-Team-Key")
	if apiKey == "" {
		apiKey = strings.TrimSpace(r.URL.Query().Get("key"))
	}

	bodyBytes, ok := readLimitedBody(w, r)
	if !ok {
		return
	}
	var in geminiRequestIn
	if err := json.Unmarshal(bodyBytes, &in); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "invalid Gemini JSON payload", "")
		return
	}

	canonical := geminiToCanonical(modelName, in, streaming)
	resp, ok := g.executeChatPipeline(w, r, "gemini", apiKey, canonical)
	if !ok {
		return
	}

	out := buildGeminiResponse(resp)
	if !streaming {
		writeJSON(w, http.StatusOK, out)
		return
	}
	if r.URL.Query().Get("alt") == "sse" || strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		startSSE(w)
		writeSSEData(w, "", out)
		return
	}
	writeJSON(w, http.StatusOK, []map[string]any{out})
}

func geminiToCanonical(modelName string, in geminiRequestIn, streaming bool) types.ChatCompletionRequest {
	var msgs []types.ChatMessage
	if in.SystemInstruction != nil {
		if txt := joinGeminiTextParts(in.SystemInstruction.Parts); txt != "" {
			msgs = append(msgs, types.ChatMessage{Role: "system", Content: txt})
		}
	}
	for cIdx, c := range in.Contents {
		role := strings.ToLower(strings.TrimSpace(c.Role))
		if role == "model" {
			role = "assistant"
		} else if role == "" {
			role = "user"
		}
		var textBuf strings.Builder
		var toolCalls []types.ToolCall
		var toolResults []types.ChatMessage

		for pIdx, p := range c.Parts {
			if p.Text != "" {
				if textBuf.Len() > 0 {
					textBuf.WriteByte('\n')
				}
				textBuf.WriteString(p.Text)
			}
			if p.FunctionCall != nil {
				argsBytes, _ := json.Marshal(p.FunctionCall.Args)
				if len(argsBytes) == 0 || string(argsBytes) == "null" {
					argsBytes = []byte("{}")
				}
				toolCalls = append(toolCalls, types.ToolCall{
					ID:   fmt.Sprintf("call_gem_%d_%d", cIdx, pIdx),
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      p.FunctionCall.Name,
						Arguments: string(argsBytes),
					},
				})
			}
			if p.FunctionResponse != nil {
				toolResults = append(toolResults, types.ChatMessage{
					Role:    "tool",
					Name:    p.FunctionResponse.Name,
					Content: string(p.FunctionResponse.Response),
				})
			}
		}
		if textBuf.Len() > 0 || len(toolCalls) > 0 || len(toolResults) == 0 {
			msgs = append(msgs, types.ChatMessage{Role: role, Content: textBuf.String(), ToolCalls: toolCalls})
		}
		msgs = append(msgs, toolResults...)
	}

	var tools []types.ToolDefinition
	for _, tg := range in.Tools {
		for _, d := range tg.FunctionDeclarations {
			tools = append(tools, types.ToolDefinition{
				Type: "function",
				Function: types.ToolFunction{
					Name:        d.Name,
					Description: d.Description,
					Parameters:  d.Parameters,
				},
			})
		}
	}

	return types.ChatCompletionRequest{
		Model:       modelName,
		Messages:    msgs,
		Tools:       tools,
		MaxTokens:   in.GenerationConfig.MaxOutputTokens,
		Temperature: in.GenerationConfig.Temperature,
		TopP:        in.GenerationConfig.TopP,
		Stream:      streaming,
	}
}

func buildGeminiResponse(resp types.ChatCompletionResponse) map[string]any {
	var choice types.ChatChoice
	if len(resp.Choices) > 0 {
		choice = resp.Choices[0]
	}
	finish := "STOP"
	if choice.FinishReason == "length" {
		finish = "MAX_TOKENS"
	}

	var parts []map[string]any
	if choice.Message.Content != "" || len(choice.Message.ToolCalls) == 0 {
		parts = append(parts, map[string]any{"text": choice.Message.Content})
	}
	for _, tc := range choice.Message.ToolCalls {
		var args map[string]any
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		parts = append(parts, map[string]any{
			"functionCall": map[string]any{
				"name": tc.Function.Name,
				"args": args,
			},
		})
	}

	return map[string]any{
		"candidates": []map[string]any{
			{
				"index":        0,
				"finishReason": finish,
				"content":      map[string]any{"role": "model", "parts": parts},
			},
		},
		"usageMetadata": map[string]int{
			"promptTokenCount":     resp.Usage.PromptTokens,
			"candidatesTokenCount": resp.Usage.CompletionTokens,
			"totalTokenCount":      resp.Usage.TotalTokens,
		},
		"modelVersion": resp.Model,
		"gateway":      resp.Gateway,
	}
}

func joinGeminiTextParts(parts []geminiPartIn) string {
	var b strings.Builder
	for _, p := range parts {
		if p.Text != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(p.Text)
		}
	}
	return b.String()
}
