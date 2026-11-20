package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/provider"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestGatewayAnthropicAndGeminiInboundProtocols(t *testing.T) {
	st := store.NewMemoryStore(true)
	mock := &provider.MockProvider{
		Handler: func(_ context.Context, model router.ModelSpec, req types.ChatCompletionRequest) (provider.Result, error) {
			return provider.Result{
				Content:      "routed:" + model.Provider + ":" + model.ID + ":" + req.Messages[len(req.Messages)-1].Content,
				FinishReason: "stop",
				Usage:        types.Usage{PromptTokens: 15, CompletionTokens: 10, TotalTokens: 25},
			}, nil
		},
	}
	prov := &provider.MultiProvider{
		Providers: map[string]provider.Provider{
			router.ProviderWorkersAI: mock, router.ProviderAnthropic: mock, router.ProviderGemini: mock,
			router.ProviderGrok: mock, router.ProviderDeepSeek: mock, router.ProviderQwen: mock,
		},
	}
	gw := New(Config{
		Store: st, Router: router.New(router.Options{IsProviderAvailable: prov.HasProvider}),
		Provider: prov, AdminTokenFunc: func() string { return "test_admin_secret" },
	})

	t.Run("Anthropic /v1/messages inbound with PII redaction", func(t *testing.T) {
		payload := `{"model":"claude-sonnet-4-5","max_tokens":256,"system":"Be brief.","messages":[{"role":"user","content":[{"type":"text","text":"Email alice@corp.io"}]}]}`
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(payload))
		req.Header.Set("x-api-key", "gw_platform_demo_key")
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var out struct {
			Type, StopReason string
			Content          []struct{ Text string }
			Gateway          types.GatewayMeta
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out.Type != "message" || !strings.Contains(out.Content[0].Text, "[REDACTED_EMAIL_1]") || out.Gateway.Provider != router.ProviderAnthropic {
			t.Fatalf("unexpected anthropic response: %+v", out)
		}
	})

	t.Run("Gemini /v1beta/models/...:generateContent inbound", func(t *testing.T) {
		payload := `{"systemInstruction":{"parts":[{"text":"Follow policy"}]},"contents":[{"role":"user","parts":[{"text":"Hello"}]}],"generationConfig":{"maxOutputTokens":128}}`
		req := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:generateContent", strings.NewReader(payload))
		req.Header.Set("x-goog-api-key", "gw_platform_demo_key")
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"finishReason":"STOP"`) {
			t.Fatalf("unexpected gemini response: %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("OpenAI inbound dispatches to Grok, DeepSeek, Qwen, and Meta Llama 4", func(t *testing.T) {
		for _, tc := range []struct{ model, wantProv string }{
			{"grok-3", router.ProviderGrok}, {"deepseek-chat", router.ProviderDeepSeek},
			{"qwen-plus", router.ProviderQwen}, {"llama-4-scout", router.ProviderWorkersAI},
		} {
			rec := doChatReq(t, gw, "gw_platform_demo_key", types.ChatCompletionRequest{
				Model: tc.model, Messages: []types.ChatMessage{{Role: "user", Content: "ping"}},
			})
			if rec.Code != http.StatusOK || rec.Header().Get("X-Gateway-Provider") != tc.wantProv {
				t.Fatalf("%s got (%d, %q), want (200, %q)", tc.model, rec.Code, rec.Header().Get("X-Gateway-Provider"), tc.wantProv)
			}
		}
	})
}
