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

func TestGatewayProviderMonthlyCapsOverflow(t *testing.T) {
	st := store.NewMemoryStore(true)
	mock := &provider.MockProvider{
		Handler: func(_ context.Context, model router.ModelSpec, _ types.ChatCompletionRequest) (provider.Result, error) {
			return provider.Result{
				Content: "from:" + model.Provider, FinishReason: "stop",
				Usage: types.Usage{PromptTokens: 1000, CompletionTokens: 1000, TotalTokens: 2000},
			}, nil
		},
	}
	prov := &provider.MultiProvider{
		Providers: map[string]provider.Provider{
			router.ProviderAnthropic: mock,
			router.ProviderWorkersAI: mock,
		},
	}
	gw := New(Config{
		Store: st, Router: router.New(router.Options{IsProviderAvailable: prov.HasProvider}),
		Provider: prov, AdminTokenFunc: func() string { return "test_admin_secret" },
		ProviderLimitsFunc: func() string { return "anthropic=0.005" },
	})

	rec1 := doChatReq(t, gw, "gw_platform_demo_key", types.ChatCompletionRequest{
		Model: "claude-sonnet-4-5", Messages: []types.ChatMessage{{Role: "user", Content: "first"}},
	})
	if rec1.Code != http.StatusOK || rec1.Header().Get("X-Gateway-Provider") != router.ProviderAnthropic {
		t.Fatalf("first call expected anthropic, got code=%d prov=%s", rec1.Code, rec1.Header().Get("X-Gateway-Provider"))
	}

	rec2 := doChatReq(t, gw, "gw_platform_demo_key", types.ChatCompletionRequest{
		Model: "claude-sonnet-4-5", Messages: []types.ChatMessage{{Role: "user", Content: "second"}},
	})
	if rec2.Code != http.StatusOK || rec2.Header().Get("X-Gateway-Provider") != router.ProviderWorkersAI {
		t.Fatalf("second call expected workers-ai overflow, got code=%d prov=%s", rec2.Code, rec2.Header().Get("X-Gateway-Provider"))
	}

	reqProv := httptest.NewRequest(http.MethodGet, "/admin/providers", nil)
	reqProv.Header.Set("Authorization", "Bearer test_admin_secret")
	recProv := httptest.NewRecorder()
	gw.ServeHTTP(recProv, reqProv)
	if recProv.Code != http.StatusOK || !strings.Contains(recProv.Body.String(), `"anthropic":5000`) {
		t.Fatalf("unexpected /admin/providers output: %d %s", recProv.Code, recProv.Body.String())
	}
}
