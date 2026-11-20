package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/provider"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestGatewayStreamingToolsAndCountTokens(t *testing.T) {
	st := store.NewMemoryStore(true)
	mock := &provider.MockProvider{
		Handler: func(_ context.Context, _ router.ModelSpec, req types.ChatCompletionRequest) (provider.Result, error) {
			if len(req.Tools) > 0 && req.Messages[len(req.Messages)-1].Role != "tool" {
				return provider.Result{
					Content: "Checking file now.",
					ToolCalls: []types.ToolCall{{
						ID: "toolu_1", Type: "function",
						Function: types.ToolCallFunction{Name: req.Tools[0].Function.Name, Arguments: `{"path":"readme.md"}`},
					}},
					FinishReason: "tool_calls",
					Usage:        types.Usage{PromptTokens: 20, CompletionTokens: 15, TotalTokens: 35},
				}, nil
			}
			return provider.Result{
				Content: "Done streaming.", FinishReason: "stop",
				Usage: types.Usage{PromptTokens: 10, CompletionTokens: 8, TotalTokens: 18},
			}, nil
		},
	}
	prov := &provider.MultiProvider{
		Providers: map[string]provider.Provider{
			router.ProviderAnthropic: mock, router.ProviderGemini: mock, router.ProviderWorkersAI: mock,
		},
	}
	gw := New(Config{
		Store: st, Router: router.New(router.Options{IsProviderAvailable: prov.HasProvider}),
		Provider: prov, AdminTokenFunc: func() string { return "test_admin_secret" },
	})

	t.Run("Anthropic SSE streaming with tools", func(t *testing.T) {
		payload := `{"model":"claude-sonnet-4-5","stream":true,"max_tokens":512,"tools":[{"name":"read_file","input_schema":{"type":"object"}}],"messages":[{"role":"user","content":"Read readme.md"}]}`
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(payload))
		req.Header.Set("x-api-key", "gw_platform_demo_key")
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)
		body := rec.Body.String()
		if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "text/event-stream") ||
			!strings.Contains(body, `"type":"tool_use"`) || !strings.Contains(body, "event: message_stop") {
			t.Fatalf("unexpected Anthropic SSE: %d %s", rec.Code, body)
		}
	})

	t.Run("Anthropic count_tokens endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", strings.NewReader(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"Hello"}]}`))
		req.Header.Set("x-api-key", "gw_platform_demo_key")
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"input_tokens":`) {
			t.Fatalf("unexpected count_tokens: %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("OpenAI SSE streaming", func(t *testing.T) {
		rec := doChatReq(t, gw, "gw_platform_demo_key", types.ChatCompletionRequest{
			Model: "auto", Stream: true, Messages: []types.ChatMessage{{Role: "user", Content: "hi"}},
		})
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"object":"chat.completion.chunk"`) || !strings.Contains(rec.Body.String(), "data: [DONE]") {
			t.Fatalf("unexpected OpenAI SSE: %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Gemini streamGenerateContent SSE, discovery, and CORS", func(t *testing.T) {
		recList := httptest.NewRecorder()
		gw.ServeHTTP(recList, httptest.NewRequest(http.MethodGet, "/v1beta/models", nil))
		if recList.Code != http.StatusOK || !strings.Contains(recList.Body.String(), `"models/gemini-2.5-flash"`) {
			t.Fatalf("unexpected /v1beta/models: %d", recList.Code)
		}
		reqStream := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse", strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
		reqStream.Header.Set("x-goog-api-key", "gw_platform_demo_key")
		recStream := httptest.NewRecorder()
		gw.ServeHTTP(recStream, reqStream)
		if recStream.Code != http.StatusOK || !strings.Contains(recStream.Body.String(), "Done streaming.") {
			t.Fatalf("unexpected gemini SSE: %d %s", recStream.Code, recStream.Body.String())
		}
		recCORS := httptest.NewRecorder()
		gw.ServeHTTP(recCORS, httptest.NewRequest(http.MethodOptions, "/v1/chat/completions", nil))
		if recCORS.Code != http.StatusNoContent || recCORS.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("unexpected CORS preflight: %d", recCORS.Code)
		}
	})
}
