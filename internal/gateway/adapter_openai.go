package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func (g *Gateway) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed", "")
		return
	}
	apiKey := extractBearerOrHeader(r, "X-Team-Key")
	bodyBytes, ok := readLimitedBody(w, r)
	if !ok {
		return
	}
	var req types.ChatCompletionRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON payload", "")
		return
	}
	resp, ok := g.executeChatPipeline(w, r, "openai", apiKey, req)
	if !ok {
		return
	}
	if !req.Stream {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	writeOpenAISSE(w, resp)
}

func writeOpenAISSE(w http.ResponseWriter, resp types.ChatCompletionResponse) {
	startSSE(w)

	var choice types.ChatChoice
	if len(resp.Choices) > 0 {
		choice = resp.Choices[0]
	}

	writeSSEData(w, "", map[string]any{
		"id":      resp.ID,
		"object":  "chat.completion.chunk",
		"created": resp.Created,
		"model":   resp.Model,
		"choices": []map[string]any{
			{"index": 0, "delta": map[string]any{"role": "assistant"}, "finish_reason": nil},
		},
	})

	for _, chunk := range splitSSEChunks(choice.Message.Content, 32) {
		writeSSEData(w, "", map[string]any{
			"id":      resp.ID,
			"object":  "chat.completion.chunk",
			"created": resp.Created,
			"model":   resp.Model,
			"choices": []map[string]any{
				{"index": 0, "delta": map[string]any{"content": chunk}, "finish_reason": nil},
			},
		})
	}

	if len(choice.Message.ToolCalls) > 0 {
		indexed := make([]map[string]any, 0, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			indexed = append(indexed, map[string]any{
				"index": i,
				"id":    tc.ID,
				"type":  tc.Type,
				"function": map[string]string{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			})
		}
		writeSSEData(w, "", map[string]any{
			"id":      resp.ID,
			"object":  "chat.completion.chunk",
			"created": resp.Created,
			"model":   resp.Model,
			"choices": []map[string]any{
				{"index": 0, "delta": map[string]any{"tool_calls": indexed}, "finish_reason": nil},
			},
		})
	}

	finish := choice.FinishReason
	if finish == "" {
		finish = "stop"
	}
	writeSSEData(w, "", map[string]any{
		"id":      resp.ID,
		"object":  "chat.completion.chunk",
		"created": resp.Created,
		"model":   resp.Model,
		"choices": []map[string]any{
			{"index": 0, "delta": map[string]any{}, "finish_reason": finish},
		},
		"usage": resp.Usage,
	})
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
