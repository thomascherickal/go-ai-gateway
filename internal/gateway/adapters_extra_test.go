package gateway

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/provider"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
)

func TestAdapterAndAdminEdgeCases(t *testing.T) {
	st := store.NewMemoryStore(true)
	gw := New(Config{
		Store:          st,
		Router:         router.New(router.Options{}),
		Provider:       &provider.MockProvider{},
		AdminTokenFunc: func() string { return "admin-secret" },
	})

	// Anthropic system array + count_tokens + invalid JSON
	if got := extractAnthropicSystemText(json.RawMessage(`[{"type":"text","text":"sys1"},{"type":"text","text":"sys2"}]`)); got != "sys1\nsys2" {
		t.Fatalf("unexpected system text: %q", got)
	}
	for _, tc := range []struct {
		m, p, key, body string
		want            int
	}{
		{"GET", "/v1/messages", "gw_platform_demo_key", "", 405},
		{"POST", "/v1/messages", "gw_platform_demo_key", "{bad", 400},
		{"POST", "/v1/messages/count_tokens", "gw_platform_demo_key", `{"model":"claude-sonnet-4-5","system":[{"type":"text","text":"hi"}],"messages":[{"role":"user","content":"hello"}]}`, 200},
		{"GET", "/v1/messages/count_tokens", "gw_platform_demo_key", "", 405},
		{"POST", "/v1/messages/count_tokens", "gw_platform_demo_key", "{bad", 400},
		{"GET", "/v1beta/models/gemini-2.5-flash:generateContent", "gw_platform_demo_key", "", 405},
		{"POST", "/v1beta/models/gemini-2.5-flash:unknownAction", "gw_platform_demo_key", "{}", 404},
		{"POST", "/v1beta/models/gemini-2.5-flash:generateContent", "gw_platform_demo_key", "{bad", 400},
		{"DELETE", "/admin/teams", "admin-secret", "", 405},
		{"POST", "/admin/teams", "admin-secret", "{bad", 400},
		{"POST", "/admin/summary", "admin-secret", "", 405},
		{"POST", "/admin/audit", "admin-secret", "", 405},
		{"POST", "/admin/providers", "admin-secret", "", 405},
	} {
		req := httptest.NewRequest(tc.m, tc.p, bytes.NewBufferString(tc.body))
		if tc.key != "" {
			req.Header.Set("Authorization", "Bearer "+tc.key)
		}
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("%s %s: got %d want %d body=%s", tc.m, tc.p, rec.Code, tc.want, rec.Body.String())
		}
	}
}
