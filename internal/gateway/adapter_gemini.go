package gateway

import (
	"encoding/json"
	"net/http"
	"strings"
)

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
