package provider

import (
	"encoding/json"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/types"
)

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

func buildGeminiTools(tools []types.ToolDefinition) []map[string]any {
	decls := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		decl := map[string]any{"name": t.Function.Name, "description": t.Function.Description}
		if len(t.Function.Parameters) > 0 {
			decl["parameters"] = t.Function.Parameters
		}
		decls = append(decls, decl)
	}
	return []map[string]any{{"functionDeclarations": decls}}
}

func buildGeminiContents(in []types.ChatMessage) ([]gemPart, []gemContent) {
	var sysParts []gemPart
	var contents []gemContent
	toolNamesByID := make(map[string]string)

	for _, m := range in {
		switch strings.ToLower(strings.TrimSpace(m.Role)) {
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
				Role:  "user",
				Parts: []gemPart{{FunctionResponse: &gemFunctionResponse{Name: name, Response: map[string]any{"result": m.Content}}}},
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
				parts = append(parts, gemPart{FunctionCall: &gemFunctionCall{Name: tc.Function.Name, Args: argsMap}})
			}
			if len(parts) > 0 {
				contents = append(contents, gemContent{Role: "model", Parts: parts})
			}
		default:
			contents = append(contents, gemContent{Role: "user", Parts: []gemPart{{Text: m.Content}}})
		}
	}
	if len(contents) == 0 {
		contents = append(contents, gemContent{Role: "user", Parts: []gemPart{{Text: ""}}})
	}
	return sysParts, contents
}
