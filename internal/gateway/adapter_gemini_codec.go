package gateway

import (
	"encoding/json"
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
		msgs = append(msgs, convertGeminiContent(cIdx, role, c.Parts)...)
	}

	var tools []types.ToolDefinition
	for _, tg := range in.Tools {
		for _, d := range tg.FunctionDeclarations {
			tools = append(tools, types.ToolDefinition{
				Type:     "function",
				Function: types.ToolFunction{Name: d.Name, Description: d.Description, Parameters: d.Parameters},
			})
		}
	}
	return types.ChatCompletionRequest{
		Model: modelName, Messages: msgs, Tools: tools,
		MaxTokens: in.GenerationConfig.MaxOutputTokens, Temperature: in.GenerationConfig.Temperature,
		TopP: in.GenerationConfig.TopP, Stream: streaming,
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
