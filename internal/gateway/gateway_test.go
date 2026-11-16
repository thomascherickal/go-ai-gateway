package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/provider"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func newTestGateway(t *testing.T, p provider.Provider) (*Gateway, *store.MemoryStore) {
	t.Helper()
	st := store.NewMemoryStore(true)
	rt := router.New(router.Options{})
	if p == nil {
		p = &provider.MockProvider{}
	}
	gw := New(Config{
		Store:          st,
		Router:         rt,
		Provider:       p,
		AdminTokenFunc: func() string { return "test_admin_secret" },
	})
	return gw, st
}

func doChatReq(t *testing.T, gw http.Handler, apiKey string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("X-Tool-Name", "ci-bot")
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)
	return rec
}

func TestGatewayUnauthorizedAndAudit(t *testing.T) {
	gw, st := newTestGateway(t, nil)
	rec := doChatReq(t, gw, "bad_key", types.ChatCompletionRequest{
		Model:    "auto",
		Messages: []types.ChatMessage{{Role: "user", Content: "hi"}},
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	logs, _ := st.ListAudit(context.Background(), types.AuditFilter{Status: types.StatusUnauthorized})
	if len(logs) != 1 {
		t.Fatalf("expected 1 unauthorized audit log, got %d", len(logs))
	}
}

func TestGatewayPIIRedactionAndBlock(t *testing.T) {
	var seenByModel string
	mock := &provider.MockProvider{
		Handler: func(_ context.Context, model router.ModelSpec, req types.ChatCompletionRequest) (provider.Result, error) {
			seenByModel = req.Messages[0].Content
			return provider.Result{
				Content:      "Acknowledged.",
				FinishReason: "stop",
				Usage:        types.Usage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30},
			}, nil
		},
	}
	gw, st := newTestGateway(t, mock)

	// 1. Platform team uses pii_mode=redact: email and SSN must never reach provider.
	rec := doChatReq(t, gw, "gw_platform_demo_key", types.ChatCompletionRequest{
		Model: "auto",
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Contact alice@corp.io with SSN 123-45-6789"},
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(seenByModel, "alice@corp.io") || strings.Contains(seenByModel, "123-45-6789") {
		t.Fatalf("PII leaked to model: %q", seenByModel)
	}
	if !strings.Contains(seenByModel, "[REDACTED_EMAIL_1]") || !strings.Contains(seenByModel, "[REDACTED_SSN_1]") {
		t.Fatalf("expected redaction placeholders, got: %q", seenByModel)
	}

	// 2. Support team uses pii_mode=block: request rejected with 400 and audited.
	recBlock := doChatReq(t, gw, "gw_support_demo_key", types.ChatCompletionRequest{
		Model: "balanced",
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Customer email bob@example.com"},
		},
	})
	if recBlock.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recBlock.Code)
	}
	logs, _ := st.ListAudit(context.Background(), types.AuditFilter{TeamID: "support", Status: types.StatusBlockedPII})
	if len(logs) != 1 || logs[0].PIIDetected != 1 {
		t.Fatalf("expected 1 blocked_pii audit entry, got %+v", logs)
	}
}

func TestGatewayBudgetEnforcementAndRefund(t *testing.T) {
	failPrimary := true
	mock := &provider.MockProvider{
		Handler: func(_ context.Context, model router.ModelSpec, _ types.ChatCompletionRequest) (provider.Result, error) {
			if failPrimary && model.ID == "@cf/meta/llama-3.1-8b-instruct-fast" {
				return provider.Result{}, errors.New("primary overloaded")
			}
			if model.ID == "always-fail" {
				return provider.Result{}, errors.New("boom")
			}
			return provider.Result{
				Content:      "fallback ok",
				FinishReason: "stop",
				Usage:        types.Usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
			}, nil
		},
	}
	gw, st := newTestGateway(t, mock)
	ctx := context.Background()

	// Create tight-budget team: 500 micros limit.
	enabled := true
	_, err := st.UpsertTeam(ctx, store.UpsertTeamInput{
		TeamID:             "tight",
		RawAPIKey:          "gw_tight",
		MonthlyLimitMicros: 500,
		MaxTier:            types.TierFast,
		PIIMode:            types.PIIModeRedact,
		Enabled:            &enabled,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Request triggers fallback (primary fails -> mistral-7b succeeds).
	rec := doChatReq(t, gw, "gw_tight", types.ChatCompletionRequest{
		Model:     "fast",
		MaxTokens: 100,
		Messages:  []types.ChatMessage{{Role: "user", Content: "hello"}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp types.ChatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Gateway.FallbackUsed || resp.Model != "@cf/mistral/mistral-7b-instruct-v0.2" {
		t.Fatalf("expected fallback to mistral-7b, got %+v", resp.Gateway)
	}

	// Next request with huge max_tokens exceeds remaining 500-micro budget -> 429.
	recQuota := doChatReq(t, gw, "gw_tight", types.ChatCompletionRequest{
		Model:     "fast",
		MaxTokens: 5000,
		Messages:  []types.ChatMessage{{Role: "user", Content: "hello"}},
	})
	if recQuota.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (body: %s)", recQuota.Code, recQuota.Body.String())
	}

	// Verify budget is intact after failed upstream call (no phantom charges).
	teamBefore, _ := st.GetTeam(ctx, "tight")
	failBoth := &provider.MockProvider{
		Handler: func(context.Context, router.ModelSpec, types.ChatCompletionRequest) (provider.Result, error) {
			return provider.Result{}, errors.New("upstream down")
		},
	}
	gwFail := New(Config{
		Store:          st,
		Router:         router.New(router.Options{}),
		Provider:       failBoth,
		AdminTokenFunc: func() string { return "test_admin_secret" },
	})
	recErr := doChatReq(t, gwFail, "gw_tight", types.ChatCompletionRequest{
		Model:     "fast",
		MaxTokens: 50,
		Messages:  []types.ChatMessage{{Role: "user", Content: "hi"}},
	})
	if recErr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", recErr.Code)
	}
	teamAfter, _ := st.GetTeam(ctx, "tight")
	if teamAfter.SpentMicros != teamBefore.SpentMicros {
		t.Fatalf("spent changed on failed call: before=%d after=%d", teamBefore.SpentMicros, teamAfter.SpentMicros)
	}
}

func TestGatewayAdminAPI(t *testing.T) {
	gw, _ := newTestGateway(t, nil)

	// Unauthenticated admin call rejected.
	req := httptest.NewRequest(http.MethodGet, "/admin/teams", nil)
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}

	// Authenticated team upsert.
	payload := `{"team_id":"ml","team_name":"ML Research","api_key":"gw_ml_123","monthly_limit_micros":25000000,"max_tier":"reasoning","pii_mode":"redact"}`
	reqUpsert := httptest.NewRequest(http.MethodPost, "/admin/teams", strings.NewReader(payload))
	reqUpsert.Header.Set("Authorization", "Bearer test_admin_secret")
	recUpsert := httptest.NewRecorder()
	gw.ServeHTTP(recUpsert, reqUpsert)
	if recUpsert.Code != http.StatusOK {
		t.Fatalf("upsert status = %d, body = %s", recUpsert.Code, recUpsert.Body.String())
	}

	// Verify new team can call gateway immediately.
	recChat := doChatReq(t, gw, "gw_ml_123", types.ChatCompletionRequest{
		Model:    "auto",
		Messages: []types.ChatMessage{{Role: "user", Content: "hello"}},
	})
	if recChat.Code != http.StatusOK {
		t.Fatalf("chat status = %d, body = %s", recChat.Code, recChat.Body.String())
	}

	// Check summary endpoint.
	reqSum := httptest.NewRequest(http.MethodGet, "/admin/summary", nil)
	reqSum.Header.Set("X-Admin-Token", "test_admin_secret")
	recSum := httptest.NewRecorder()
	gw.ServeHTTP(recSum, reqSum)
	if recSum.Code != http.StatusOK {
		t.Fatalf("summary status = %d", recSum.Code)
	}
	var sum types.AuditSummary
	if err := json.Unmarshal(recSum.Body.Bytes(), &sum); err != nil {
		t.Fatal(err)
	}
	if sum.TotalRequests != 1 || sum.SuccessfulCount != 1 {
		t.Fatalf("unexpected summary: %+v", sum)
	}
}

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
			router.ProviderWorkersAI: mock,
			router.ProviderAnthropic: mock,
			router.ProviderGemini:    mock,
			router.ProviderGrok:      mock,
			router.ProviderDeepSeek:  mock,
			router.ProviderQwen:      mock,
		},
	}
	gw := New(Config{
		Store:          st,
		Router:         router.New(router.Options{IsProviderAvailable: prov.HasProvider}),
		Provider:       prov,
		AdminTokenFunc: func() string { return "test_admin_secret" },
	})

	t.Run("Anthropic /v1/messages inbound with PII redaction", func(t *testing.T) {
		payload := `{
			"model": "claude-sonnet-4-5",
			"max_tokens": 256,
			"system": "Be brief.",
			"messages": [{"role": "user", "content": [{"type": "text", "text": "Email alice@corp.io"}]}]
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(payload))
		req.Header.Set("x-api-key", "gw_platform_demo_key")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var out struct {
			Type       string `json:"type"`
			StopReason string `json:"stop_reason"`
			Content    []struct {
				Text string `json:"text"`
			} `json:"content"`
			Gateway types.GatewayMeta `json:"gateway"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out.Type != "message" || out.StopReason != "end_turn" {
			t.Fatalf("unexpected anthropic response: %+v", out)
		}
		if !strings.Contains(out.Content[0].Text, "[REDACTED_EMAIL_1]") || strings.Contains(out.Content[0].Text, "alice@corp.io") {
			t.Fatalf("expected PII redaction in anthropic flow, got %q", out.Content[0].Text)
		}
		if out.Gateway.InboundProtocol != "anthropic" || out.Gateway.Provider != router.ProviderAnthropic {
			t.Fatalf("unexpected gateway meta: %+v", out.Gateway)
		}
	})

	t.Run("Gemini /v1beta/models/...:generateContent inbound", func(t *testing.T) {
		payload := `{
			"systemInstruction": {"parts": [{"text": "Follow policy"}]},
			"contents": [{"role": "user", "parts": [{"text": "Hello from Gemini SDK"}]}],
			"generationConfig": {"maxOutputTokens": 128}
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:generateContent", strings.NewReader(payload))
		req.Header.Set("x-goog-api-key", "gw_platform_demo_key")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var out struct {
			Candidates []struct {
				FinishReason string `json:"finishReason"`
				Content      struct {
					Role  string `json:"role"`
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
			UsageMetadata struct {
				TotalTokenCount int `json:"totalTokenCount"`
			} `json:"usageMetadata"`
			Gateway types.GatewayMeta `json:"gateway"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if len(out.Candidates) != 1 || out.Candidates[0].FinishReason != "STOP" || out.UsageMetadata.TotalTokenCount != 25 {
			t.Fatalf("unexpected gemini response: %+v", out)
		}
		if out.Gateway.InboundProtocol != "gemini" || out.Gateway.Provider != router.ProviderGemini {
			t.Fatalf("unexpected gateway meta: %+v", out.Gateway)
		}
	})

	t.Run("OpenAI inbound dispatches to Grok, DeepSeek, Qwen, and Meta Llama 4", func(t *testing.T) {
		for _, tc := range []struct {
			model    string
			wantProv string
		}{
			{"grok-3", router.ProviderGrok},
			{"deepseek-chat", router.ProviderDeepSeek},
			{"qwen-plus", router.ProviderQwen},
			{"llama-4-scout", router.ProviderWorkersAI},
		} {
			rec := doChatReq(t, gw, "gw_platform_demo_key", types.ChatCompletionRequest{
				Model:    tc.model,
				Messages: []types.ChatMessage{{Role: "user", Content: "ping"}},
			})
			if rec.Code != http.StatusOK {
				t.Fatalf("%s status = %d, body = %s", tc.model, rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("X-Gateway-Provider"); got != tc.wantProv {
				t.Fatalf("%s routed provider = %q, want %q", tc.model, got, tc.wantProv)
			}
		}
	})
}

func TestGatewayProviderMonthlyCapsOverflow(t *testing.T) {
	st := store.NewMemoryStore(true)
	mock := &provider.MockProvider{
		Handler: func(_ context.Context, model router.ModelSpec, _ types.ChatCompletionRequest) (provider.Result, error) {
			return provider.Result{
				Content:      "from:" + model.Provider,
				FinishReason: "stop",
				Usage:        types.Usage{PromptTokens: 1000, CompletionTokens: 1000, TotalTokens: 2000},
			}, nil
		},
	}
	prov := &provider.MultiProvider{
		Providers: map[string]provider.Provider{
			router.ProviderAnthropic: mock,
			router.ProviderWorkersAI: mock,
		},
	}
	// Cap Anthropic at $0.005 (5,000 micros). One call to claude-sonnet-4-5 (1k in + 1k out = 18,000 micros) exceeds cap,
	// so the second call to claude-sonnet-4-5 automatically overflows to Workers AI.
	gw := New(Config{
		Store:              st,
		Router:             router.New(router.Options{IsProviderAvailable: prov.HasProvider}),
		Provider:           prov,
		AdminTokenFunc:     func() string { return "test_admin_secret" },
		ProviderLimitsFunc: func() string { return "anthropic=0.005" },
	})

	rec1 := doChatReq(t, gw, "gw_platform_demo_key", types.ChatCompletionRequest{
		Model:    "claude-sonnet-4-5",
		Messages: []types.ChatMessage{{Role: "user", Content: "first"}},
	})
	if rec1.Code != http.StatusOK || rec1.Header().Get("X-Gateway-Provider") != router.ProviderAnthropic {
		t.Fatalf("first call expected anthropic, got code=%d prov=%s body=%s", rec1.Code, rec1.Header().Get("X-Gateway-Provider"), rec1.Body.String())
	}

	rec2 := doChatReq(t, gw, "gw_platform_demo_key", types.ChatCompletionRequest{
		Model:    "claude-sonnet-4-5",
		Messages: []types.ChatMessage{{Role: "user", Content: "second"}},
	})
	if rec2.Code != http.StatusOK || rec2.Header().Get("X-Gateway-Provider") != router.ProviderWorkersAI {
		t.Fatalf("second call expected workers-ai overflow, got code=%d prov=%s body=%s", rec2.Code, rec2.Header().Get("X-Gateway-Provider"), rec2.Body.String())
	}

	reqProv := httptest.NewRequest(http.MethodGet, "/admin/providers", nil)
	reqProv.Header.Set("Authorization", "Bearer test_admin_secret")
	recProv := httptest.NewRecorder()
	gw.ServeHTTP(recProv, reqProv)
	if recProv.Code != http.StatusOK || !strings.Contains(recProv.Body.String(), `"anthropic":5000`) {
		t.Fatalf("unexpected /admin/providers output: %d %s", recProv.Code, recProv.Body.String())
	}
}

func TestGatewayStreamingToolsAndCountTokens(t *testing.T) {
	st := store.NewMemoryStore(true)
	mock := &provider.MockProvider{
		Handler: func(_ context.Context, model router.ModelSpec, req types.ChatCompletionRequest) (provider.Result, error) {
			if len(req.Tools) > 0 && req.Messages[len(req.Messages)-1].Role != "tool" {
				return provider.Result{
					Content: "Checking file now.",
					ToolCalls: []types.ToolCall{{
						ID:   "toolu_1",
						Type: "function",
						Function: types.ToolCallFunction{
							Name:      req.Tools[0].Function.Name,
							Arguments: `{"path":"readme.md"}`,
						},
					}},
					FinishReason: "tool_calls",
					Usage:        types.Usage{PromptTokens: 20, CompletionTokens: 15, TotalTokens: 35},
				}, nil
			}
			return provider.Result{
				Content:      "Done streaming.",
				FinishReason: "stop",
				Usage:        types.Usage{PromptTokens: 10, CompletionTokens: 8, TotalTokens: 18},
			}, nil
		},
	}
	prov := &provider.MultiProvider{
		Providers: map[string]provider.Provider{
			router.ProviderAnthropic: mock,
			router.ProviderGemini:    mock,
			router.ProviderWorkersAI: mock,
		},
	}
	gw := New(Config{
		Store:          st,
		Router:         router.New(router.Options{IsProviderAvailable: prov.HasProvider}),
		Provider:       prov,
		AdminTokenFunc: func() string { return "test_admin_secret" },
	})

	t.Run("Anthropic SSE streaming with tools (Claude Code flow)", func(t *testing.T) {
		payload := `{
			"model": "claude-sonnet-4-5",
			"stream": true,
			"max_tokens": 512,
			"tools": [{"name": "read_file", "description": "Read a file", "input_schema": {"type": "object"}}],
			"messages": [{"role": "user", "content": "Read readme.md"}]
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(payload))
		req.Header.Set("x-api-key", "gw_platform_demo_key")
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
			t.Fatalf("expected text/event-stream, got %q", ct)
		}
		body := rec.Body.String()
		for _, want := range []string{"event: message_start", "event: content_block_start", "event: content_block_delta", `"type":"tool_use"`, `"name":"read_file"`, `"stop_reason":"tool_use"`, "event: message_stop"} {
			if !strings.Contains(body, want) {
				t.Fatalf("SSE body missing %q:\n%s", want, body)
			}
		}
	})

	t.Run("Anthropic count_tokens endpoint", func(t *testing.T) {
		payload := `{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"Hello world"}]}`
		req := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", strings.NewReader(payload))
		req.Header.Set("x-api-key", "gw_platform_demo_key")
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"input_tokens":`) {
			t.Fatalf("unexpected count_tokens response: %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("OpenAI SSE streaming", func(t *testing.T) {
		rec := doChatReq(t, gw, "gw_platform_demo_key", types.ChatCompletionRequest{
			Model:    "auto",
			Stream:   true,
			Messages: []types.ChatMessage{{Role: "user", Content: "hi"}},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, `"object":"chat.completion.chunk"`) || !strings.Contains(body, "data: [DONE]") {
			t.Fatalf("unexpected OpenAI SSE body:\n%s", body)
		}
	})

	t.Run("Gemini streamGenerateContent SSE and model discovery", func(t *testing.T) {
		reqList := httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
		recList := httptest.NewRecorder()
		gw.ServeHTTP(recList, reqList)
		if recList.Code != http.StatusOK || !strings.Contains(recList.Body.String(), `"models/gemini-2.5-flash"`) {
			t.Fatalf("unexpected /v1beta/models response: %d %s", recList.Code, recList.Body.String())
		}

		reqStream := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse",
			strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
		reqStream.Header.Set("x-goog-api-key", "gw_platform_demo_key")
		recStream := httptest.NewRecorder()
		gw.ServeHTTP(recStream, reqStream)
		if recStream.Code != http.StatusOK || !strings.Contains(recStream.Header().Get("Content-Type"), "text/event-stream") {
			t.Fatalf("unexpected gemini SSE status/header: %d %s", recStream.Code, recStream.Header().Get("Content-Type"))
		}
		if !strings.Contains(recStream.Body.String(), "Done streaming.") {
			t.Fatalf("unexpected gemini SSE body: %s", recStream.Body.String())
		}
	})

	t.Run("CORS preflight for Cloudflare Pages demo", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/v1/chat/completions", nil)
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("OPTIONS status = %d, want 204", rec.Code)
		}
		if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("missing CORS allow-origin header")
		}
	})
}




