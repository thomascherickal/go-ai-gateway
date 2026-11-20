package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestOpenAICompatProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("unexpected request: %s %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(types.ChatCompletionResponse{
			ID:      "cmpl_1",
			Model:   "grok-3",
			Choices: []types.ChatChoice{{Index: 0, Message: types.ChatMessage{Role: "assistant", Content: "ok"}, FinishReason: "stop"}},
			Usage:   types.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		})
	}))
	defer srv.Close()

	p := &OpenAICompatProvider{BaseURL: srv.URL + "/v1", APIKey: "secret", HTTPClient: srv.Client()}
	res, err := p.Chat(context.Background(), router.ModelSpec{ID: "grok-3"}, types.ChatCompletionRequest{
		Messages: []types.ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil || res.Content != "ok" || res.Usage.TotalTokens != 15 {
		t.Fatalf("unexpected result: %+v, err=%v", res, err)
	}
}

func TestAnthropicProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "ant-key" || r.Header.Get("anthropic-version") == "" {
			t.Fatalf("invalid anthropic request: %s", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		if body["system"] != "Be concise." {
			t.Fatalf("expected system prompt extraction, got %v", body["system"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":          "msg_1",
			"stop_reason": "end_turn",
			"content":     []map[string]string{{"type": "text", "text": "Hello from Claude"}},
			"usage":       map[string]int{"input_tokens": 12, "output_tokens": 8},
		})
	}))
	defer srv.Close()

	p := &AnthropicProvider{BaseURL: srv.URL, APIKey: "ant-key", HTTPClient: srv.Client()}
	res, err := p.Chat(context.Background(), router.ModelSpec{ID: "claude-sonnet-4-5"}, types.ChatCompletionRequest{
		Messages: []types.ChatMessage{
			{Role: "system", Content: "Be concise."},
			{Role: "user", Content: "Hi"},
		},
	})
	if err != nil || res.Content != "Hello from Claude" || res.Usage.PromptTokens != 12 || res.Usage.CompletionTokens != 8 {
		t.Fatalf("unexpected result: %+v, err=%v", res, err)
	}
}
