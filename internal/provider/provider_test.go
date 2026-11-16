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

func TestOpenAICompatProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		_ = json.NewEncoder(w).Encode(types.ChatCompletionResponse{
			ID:    "cmpl_1",
			Model: "grok-3",
			Choices: []types.ChatChoice{
				{Index: 0, Message: types.ChatMessage{Role: "assistant", Content: "ok"}, FinishReason: "stop"},
			},
			Usage: types.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		})
	}))
	defer srv.Close()

	p := &OpenAICompatProvider{
		BaseURL:    srv.URL + "/v1",
		APIKey:     "secret",
		HTTPClient: srv.Client(),
	}
	res, err := p.Chat(context.Background(), router.ModelSpec{ID: "grok-3"}, types.ChatCompletionRequest{
		Messages: []types.ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if res.Content != "ok" || res.Usage.TotalTokens != 15 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestAnthropicProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "ant-key" || r.Header.Get("anthropic-version") == "" {
			t.Fatalf("missing anthropic auth/version headers")
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

	p := &AnthropicProvider{
		BaseURL:    srv.URL,
		APIKey:     "ant-key",
		HTTPClient: srv.Client(),
	}
	res, err := p.Chat(context.Background(), router.ModelSpec{ID: "claude-sonnet-4-5"}, types.ChatCompletionRequest{
		Messages: []types.ChatMessage{
			{Role: "system", Content: "Be concise."},
			{Role: "user", Content: "Hi"},
		},
	})
	if err != nil {
		t.Fatalf("Anthropic Chat failed: %v", err)
	}
	if res.Content != "Hello from Claude" || res.Usage.PromptTokens != 12 || res.Usage.CompletionTokens != 8 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

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

	p := &GeminiProvider{
		BaseURL:    srv.URL,
		APIKey:     "gem-key",
		HTTPClient: srv.Client(),
	}
	res, err := p.Chat(context.Background(), router.ModelSpec{ID: "gemini-2.5-flash"}, types.ChatCompletionRequest{
		Messages: []types.ChatMessage{
			{Role: "system", Content: "Follow policy."},
			{Role: "user", Content: "Hi"},
			{Role: "assistant", Content: "Hello!"},
			{Role: "user", Content: "Status?"},
		},
	})
	if err != nil {
		t.Fatalf("Gemini Chat failed: %v", err)
	}
	if res.Content != "Hello from Gemini" || res.Usage.TotalTokens != 20 {
		t.Fatalf("unexpected result: %+v", res)
	}
}
