package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/provider"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
)

func TestRoutesAndAdminEndpoints(t *testing.T) {
	st := store.NewMemoryStore(true)
	gw := New(Config{Store: st, Router: router.New(router.Options{}), Provider: &provider.MockProvider{}, AdminTokenFunc: func() string { return "admin-secret" }})

	// Root HTML + JSON, healthz, models
	for _, tc := range []struct {
		method, path, accept string
		want                 int
	}{
		{"GET", "/", "text/html", http.StatusOK},
		{"GET", "/", "application/json", http.StatusOK},
		{"GET", "/healthz", "", http.StatusOK},
		{"GET", "/v1/models", "", http.StatusOK},
		{"GET", "/v1beta/models", "", http.StatusOK},
		{"GET", "/admin/summary", "", http.StatusUnauthorized},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if tc.accept != "" {
			req.Header.Set("Accept", tc.accept)
		}
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("%s %s: got %d want %d", tc.method, tc.path, rec.Code, tc.want)
		}
	}

	adminReq := func(method, path string, body any) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, path, &buf)
		req.Header.Set("Authorization", "Bearer admin-secret")
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)
		return rec
	}

	if rec := adminReq("POST", "/admin/teams", map[string]any{
		"team_id": "qa", "team_name": "QA", "api_key": "gw_qa_key",
		"monthly_limit_micros": 5000000, "max_tier": "balanced", "pii_mode": "redact",
	}); rec.Code != http.StatusOK {
		t.Fatalf("POST /admin/teams: got %d", rec.Code)
	}
	for _, tc := range []struct {
		m, p string
		want int
	}{
		{"GET", "/admin/teams", 200},
		{"GET", "/admin/teams/qa", 200},
		{"GET", "/admin/teams/missing", 404},
		{"POST", "/admin/teams/qa/reset-budget", 200},
		{"GET", "/admin/audit?team_id=qa&limit=10&pii_only=1", 200},
		{"GET", "/admin/summary", 200},
		{"GET", "/admin/providers", 200},
	} {
		if rec := adminReq(tc.m, tc.p, nil); rec.Code != tc.want {
			t.Fatalf("%s %s: got %d want %d", tc.m, tc.p, rec.Code, tc.want)
		}
	}

	// Error & policy paths
	for _, tc := range []struct {
		m, p, body, strict string
		want               int
	}{
		{"GET", "/v1/chat/completions", "", "", 405},
		{"POST", "/v1/chat/completions", "{bad", "", 400},
		{"POST", "/v1/chat/completions", `{"model":"nonexistent-xyz","messages":[{"role":"user","content":"hi"}]}`, "", 400},
		{"POST", "/v1/chat/completions", `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`, "true", 503},
	} {
		req := httptest.NewRequest(tc.m, tc.p, bytes.NewBufferString(tc.body))
		req.Header.Set("Authorization", "Bearer gw_platform_demo_key")
		if tc.strict != "" {
			req.Header.Set("X-Gateway-Strict-Provider", tc.strict)
		}
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("%s %s: got %d want %d", tc.m, tc.p, rec.Code, tc.want)
		}
	}
}
