package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestGeminiProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v1beta/models/gemini-2.5-flash:generateContent") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "gem-key" {
			t.Fatalf("missing x-goog-api-key header")
		}
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), `"systemInstruction"`) || !strings.Contains(string(raw), `"role":"model"`) {
			t.Fatalf("expected systemInstruction and model role mapping, got %s", string(raw))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{
					"content":      map[string]any{"parts": []map[string]string{{"text": "Hello from Gemini"}}},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     14,
				"candidatesTokenCount": 6,
				"totalTokenCount":      20,
			},
		})
	}))
	defer srv.Close()

	p := &GeminiProvider{BaseURL: srv.URL, APIKey: "gem-key", HTTPClient: srv.Client()}
	res, err := p.Chat(context.Background(), router.ModelSpec{ID: "gemini-2.5-flash"}, types.ChatCompletionRequest{
		Messages: []types.ChatMessage{
			{Role: "system", Content: "Follow policy."},
			{Role: "user", Content: "Hi"},
			{Role: "assistant", Content: "Hello!"},
			{Role: "user", Content: "Status?"},
		},
	})
	if err != nil || res.Content != "Hello from Gemini" || res.Usage.TotalTokens != 20 {
		t.Fatalf("unexpected result: %+v, err=%v", res, err)
	}
}
