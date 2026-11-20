package gateway

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func convertGeminiContent(cIdx int, role string, parts []geminiPartIn) []types.ChatMessage {
	var textBuf strings.Builder
	var toolCalls []types.ToolCall
	var toolResults []types.ChatMessage

	for pIdx, p := range parts {
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
				ID:       fmt.Sprintf("call_gem_%d_%d", cIdx, pIdx),
				Type:     "function",
				Function: types.ToolCallFunction{Name: p.FunctionCall.Name, Arguments: string(argsBytes)},
			})
		}
		if p.FunctionResponse != nil {
			toolResults = append(toolResults, types.ChatMessage{
				Role: "tool", Name: p.FunctionResponse.Name, Content: string(p.FunctionResponse.Response),
			})
		}
	}

	var out []types.ChatMessage
	if textBuf.Len() > 0 || len(toolCalls) > 0 || len(toolResults) == 0 {
		out = append(out, types.ChatMessage{Role: role, Content: textBuf.String(), ToolCalls: toolCalls})
	}
	return append(out, toolResults...)
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
			"functionCall": map[string]any{"name": tc.Function.Name, "args": args},
		})
	}

	return map[string]any{
		"candidates": []map[string]any{
			{"index": 0, "finishReason": finish, "content": map[string]any{"role": "model", "parts": parts}},
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
