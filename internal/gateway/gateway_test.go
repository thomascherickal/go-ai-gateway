package gateway

import (
	"bytes"
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

func newTestGateway(t *testing.T, p provider.Provider) (*Gateway, *store.MemoryStore) {
	t.Helper()
	st := store.NewMemoryStore(true)
	rt := router.New(router.Options{})
	if p == nil {
		p = &provider.MockProvider{}
	}
	return New(Config{
		Store: st, Router: rt, Provider: p,
		AdminTokenFunc: func() string { return "test_admin_secret" },
	}), st
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
		Model: "auto", Messages: []types.ChatMessage{{Role: "user", Content: "hi"}},
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
		Handler: func(_ context.Context, _ router.ModelSpec, req types.ChatCompletionRequest) (provider.Result, error) {
			seenByModel = req.Messages[0].Content
			return provider.Result{
				Content: "Acknowledged.", FinishReason: "stop",
				Usage: types.Usage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30},
			}, nil
		},
	}
	gw, st := newTestGateway(t, mock)

	rec := doChatReq(t, gw, "gw_platform_demo_key", types.ChatCompletionRequest{
		Model:    "auto",
		Messages: []types.ChatMessage{{Role: "user", Content: "Contact alice@corp.io with SSN 123-45-6789"}},
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

	recBlock := doChatReq(t, gw, "gw_support_demo_key", types.ChatCompletionRequest{
		Model:    "balanced",
		Messages: []types.ChatMessage{{Role: "user", Content: "Customer email bob@example.com"}},
	})
	if recBlock.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recBlock.Code)
	}
	logs, _ := st.ListAudit(context.Background(), types.AuditFilter{TeamID: "support", Status: types.StatusBlockedPII})
	if len(logs) != 1 || logs[0].PIIDetected != 1 {
		t.Fatalf("expected 1 blocked_pii audit entry, got %+v", logs)
	}
}
