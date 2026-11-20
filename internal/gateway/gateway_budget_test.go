package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/provider"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestGatewayBudgetEnforcementAndRefund(t *testing.T) {
	mock := &provider.MockProvider{
		Handler: func(_ context.Context, model router.ModelSpec, _ types.ChatCompletionRequest) (provider.Result, error) {
			if model.ID == "@cf/meta/llama-3.1-8b-instruct-fast" {
				return provider.Result{}, errors.New("primary overloaded")
			}
			return provider.Result{
				Content: "fallback ok", FinishReason: "stop",
				Usage: types.Usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
			}, nil
		},
	}
	gw, st := newTestGateway(t, mock)
	ctx := context.Background()

	enabled := true
	if _, err := st.UpsertTeam(ctx, store.UpsertTeamInput{
		TeamID: "tight", RawAPIKey: "gw_tight", MonthlyLimitMicros: 500,
		MaxTier: types.TierFast, PIIMode: types.PIIModeRedact, Enabled: &enabled,
	}); err != nil {
		t.Fatal(err)
	}

	rec := doChatReq(t, gw, "gw_tight", types.ChatCompletionRequest{
		Model: "fast", MaxTokens: 100, Messages: []types.ChatMessage{{Role: "user", Content: "hello"}},
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

	recQuota := doChatReq(t, gw, "gw_tight", types.ChatCompletionRequest{
		Model: "fast", MaxTokens: 5000, Messages: []types.ChatMessage{{Role: "user", Content: "hello"}},
	})
	if recQuota.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (body: %s)", recQuota.Code, recQuota.Body.String())
	}

	teamBefore, _ := st.GetTeam(ctx, "tight")
	failBoth := &provider.MockProvider{
		Handler: func(context.Context, router.ModelSpec, types.ChatCompletionRequest) (provider.Result, error) {
			return provider.Result{}, errors.New("upstream down")
		},
	}
	gwFail := New(Config{
		Store: st, Router: router.New(router.Options{}), Provider: failBoth,
		AdminTokenFunc: func() string { return "test_admin_secret" },
	})
	recErr := doChatReq(t, gwFail, "gw_tight", types.ChatCompletionRequest{
		Model: "fast", MaxTokens: 50, Messages: []types.ChatMessage{{Role: "user", Content: "hi"}},
	})
	if recErr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", recErr.Code)
	}
	teamAfter, _ := st.GetTeam(ctx, "tight")
	if teamAfter.SpentMicros != teamBefore.SpentMicros {
		t.Fatalf("spent changed on failed call: before=%d after=%d", teamBefore.SpentMicros, teamAfter.SpentMicros)
	}
}
