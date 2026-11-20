package gateway

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/provider"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
)

func TestGatewayAdaptersRoutesAndAdminFull(t *testing.T) {
	st := store.NewMemoryStore(true)
	// Gateway with nil Router (triggers default router.New) and empty AdminToken (triggers requireAdmin 503)
	gwNoAdmin := New(Config{Store: st, Provider: &provider.MockProvider{}})
	rec := httptest.NewRecorder()
	gwNoAdmin.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/summary", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unconfigured admin token, got %d", rec.Code)
	}

	gw := New(Config{
		Store:          st,
		Router:         router.New(router.Options{}),
		Provider:       &provider.MockProvider{},
		AdminTokenFunc: func() string { return "adm" },
	})

	// Method-not-allowed & bad body paths on routes, adapters, and admin
	for _, tc := range []struct {
		m, p, key string
		body      any
		want      int
	}{
		{"GET", "/does-not-exist", "", nil, 404},
		{"POST", "/v1/models", "", nil, 405},
		{"POST", "/v1beta/models", "", nil, 405},
		{"POST", "/v1/chat/completions", "gw_platform_demo_key", badBodyReader{}, 400},
		{"POST", "/v1/messages", "gw_platform_demo_key", badBodyReader{}, 400},
		{"POST", "/v1/messages", "bad_key", `{"model":"fast","messages":[{"role":"user","content":"hi"}]}`, 401},
		{"POST", "/v1/messages/count_tokens", "bad_key", `{}`, 401},
		{"POST", "/v1/messages/count_tokens", "gw_platform_demo_key", badBodyReader{}, 400},
		{"POST", "/v1/messages/count_tokens", "gw_platform_demo_key", `{"model":"fast","tools":[{"name":"f"}],"messages":[{"role":"user","content":"hi"}]}`, 200},
		{"POST", "/v1beta/models/gemini-2.5-flash:generateContent?key=gw_platform_demo_key", "", badBodyReader{}, 400},
		{"POST", "/v1beta/models/gemini-2.5-flash:generateContent?key=bad", "", `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`, 401},
		{"POST", "/v1beta/models/gemini-2.5-flash:streamGenerateContent?key=gw_platform_demo_key", "", `{"systemInstruction":{"parts":[{"text":"s1"},{"text":"s2"}]},"tools":[{"functionDeclarations":[{"name":"fn","description":"d","parameters":{"type":"object"}}]}],"generationConfig":{"temperature":0.5,"topP":0.9},"contents":[{"role":"model","parts":[{"text":"m"}]},{"role":"","parts":[{"text":"u"}]}]}`, 200},
		{"POST", "/admin/teams", "adm", badBodyReader{}, 400},
		{"POST", "/admin/teams", "adm", `{"team_id":"x","max_tier":"bad","pii_mode":"redact"}`, 400},
		{"POST", "/admin/teams", "adm", `{"team_id":"x","max_tier":"fast","pii_mode":"bad"}`, 400},
		{"DELETE", "/admin/teams/platform", "adm", "", 404},
	} {
		var req *http.Request
		if br, ok := tc.body.(badBodyReader); ok {
			req = httptest.NewRequest(tc.m, tc.p, br)
		} else if s, ok := tc.body.(string); ok {
			req = httptest.NewRequest(tc.m, tc.p, bytes.NewBufferString(s))
		} else {
			req = httptest.NewRequest(tc.m, tc.p, nil)
		}
		if tc.key != "" {
			req.Header.Set("Authorization", "Bearer "+tc.key)
		}
		r := httptest.NewRecorder()
		gw.ServeHTTP(r, req)
		if r.Code != tc.want {
			t.Fatalf("%s %s: got %d want %d body=%s", tc.m, tc.p, r.Code, tc.want, r.Body.String())
		}
	}
}
