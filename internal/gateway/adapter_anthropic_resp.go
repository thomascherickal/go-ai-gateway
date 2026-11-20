package gateway

import (
	"encoding/json"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func anthropicStopReason(finishReason string, hasTools bool) string {
	if hasTools || finishReason == "tool_calls" {
		return "tool_use"
	}
	if finishReason == "length" {
		return "max_tokens"
	}
	return "end_turn"
}

func buildAnthropicResponse(resp types.ChatCompletionResponse) map[string]any {
	var choice types.ChatChoice
	if len(resp.Choices) > 0 {
		choice = resp.Choices[0]
	}
	content := make([]map[string]any, 0, 1+len(choice.Message.ToolCalls))
	if choice.Message.Content != "" || len(choice.Message.ToolCalls) == 0 {
		content = append(content, map[string]any{
			"type": "text",
			"text": choice.Message.Content,
		})
	}
	for _, tc := range choice.Message.ToolCalls {
		var inputObj any = map[string]any{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputObj); err != nil {
			inputObj = map[string]any{}
		}
		content = append(content, map[string]any{
			"type":  "tool_use",
			"id":    tc.ID,
			"name":  tc.Function.Name,
			"input": inputObj,
		})
	}
	return map[string]any{
		"id":          resp.ID,
		"type":        "message",
		"role":        "assistant",
		"model":       resp.Model,
		"stop_reason": anthropicStopReason(choice.FinishReason, len(choice.Message.ToolCalls) > 0),
		"content":     content,
		"usage": map[string]int{
			"input_tokens":  resp.Usage.PromptTokens,
			"output_tokens": resp.Usage.CompletionTokens,
		},
		"gateway": resp.Gateway,
	}
}
