package gateway

import (
	"fmt"
	"net/http"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func writeOpenAISSE(w http.ResponseWriter, resp types.ChatCompletionResponse) {
	startSSE(w)

	var choice types.ChatChoice
	if len(resp.Choices) > 0 {
		choice = resp.Choices[0]
	}

	writeSSEData(w, "", map[string]any{
		"id": resp.ID, "object": "chat.completion.chunk", "created": resp.Created, "model": resp.Model,
		"choices": []map[string]any{{"index": 0, "delta": map[string]any{"role": "assistant"}, "finish_reason": nil}},
	})

	for _, chunk := range splitSSEChunks(choice.Message.Content, 32) {
		writeSSEData(w, "", map[string]any{
			"id": resp.ID, "object": "chat.completion.chunk", "created": resp.Created, "model": resp.Model,
			"choices": []map[string]any{{"index": 0, "delta": map[string]any{"content": chunk}, "finish_reason": nil}},
		})
	}

	if len(choice.Message.ToolCalls) > 0 {
		indexed := make([]map[string]any, 0, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			indexed = append(indexed, map[string]any{
				"index": i, "id": tc.ID, "type": tc.Type,
				"function": map[string]string{"name": tc.Function.Name, "arguments": tc.Function.Arguments},
			})
		}
		writeSSEData(w, "", map[string]any{
			"id": resp.ID, "object": "chat.completion.chunk", "created": resp.Created, "model": resp.Model,
			"choices": []map[string]any{{"index": 0, "delta": map[string]any{"tool_calls": indexed}, "finish_reason": nil}},
		})
	}

	finish := choice.FinishReason
	if finish == "" {
		finish = "stop"
	}
	writeSSEData(w, "", map[string]any{
		"id": resp.ID, "object": "chat.completion.chunk", "created": resp.Created, "model": resp.Model,
		"choices": []map[string]any{{"index": 0, "delta": map[string]any{}, "finish_reason": finish}},
		"usage":   resp.Usage,
	})
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
