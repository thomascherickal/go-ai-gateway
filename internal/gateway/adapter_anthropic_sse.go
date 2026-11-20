package gateway

import (
	"net/http"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func writeAnthropicSSE(w http.ResponseWriter, resp types.ChatCompletionResponse) {
	startSSE(w)

	var choice types.ChatChoice
	if len(resp.Choices) > 0 {
		choice = resp.Choices[0]
	}
	stopReason := anthropicStopReason(choice.FinishReason, len(choice.Message.ToolCalls) > 0)

	writeSSEData(w, "message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id": resp.ID, "type": "message", "role": "assistant", "model": resp.Model,
			"content": []any{}, "stop_reason": nil, "stop_sequence": nil,
			"usage": map[string]int{"input_tokens": resp.Usage.PromptTokens, "output_tokens": 0},
		},
	})

	blockIdx := 0
	if choice.Message.Content != "" || len(choice.Message.ToolCalls) == 0 {
		writeSSEData(w, "content_block_start", map[string]any{
			"type": "content_block_start", "index": blockIdx,
			"content_block": map[string]string{"type": "text", "text": ""},
		})
		for _, chunk := range splitSSEChunks(choice.Message.Content, 32) {
			writeSSEData(w, "content_block_delta", map[string]any{
				"type": "content_block_delta", "index": blockIdx,
				"delta": map[string]string{"type": "text_delta", "text": chunk},
			})
		}
		writeSSEData(w, "content_block_stop", map[string]any{"type": "content_block_stop", "index": blockIdx})
		blockIdx++
	}

	for _, tc := range choice.Message.ToolCalls {
		writeSSEData(w, "content_block_start", map[string]any{
			"type": "content_block_start", "index": blockIdx,
			"content_block": map[string]any{"type": "tool_use", "id": tc.ID, "name": tc.Function.Name, "input": map[string]any{}},
		})
		args := tc.Function.Arguments
		if args == "" {
			args = "{}"
		}
		writeSSEData(w, "content_block_delta", map[string]any{
			"type": "content_block_delta", "index": blockIdx,
			"delta": map[string]string{"type": "input_json_delta", "partial_json": args},
		})
		writeSSEData(w, "content_block_stop", map[string]any{"type": "content_block_stop", "index": blockIdx})
		blockIdx++
	}

	writeSSEData(w, "message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": stopReason, "stop_sequence": nil},
		"usage": map[string]int{"output_tokens": resp.Usage.CompletionTokens},
	})
	writeSSEData(w, "message_stop", map[string]string{"type": "message_stop"})
}
