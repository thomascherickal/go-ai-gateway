package gateway

import (
	"encoding/json"
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
