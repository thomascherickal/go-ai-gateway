//go:build !js

package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestMainAndLocalRuntimeBranches(t *testing.T) {
	t.Setenv("PERSONAL_MODE", "true")
	t.Setenv("PERSONAL_API_KEY", "")
	t.Setenv("PERSONAL_MONTHLY_BUDGET_USD", "12.5")
	t.Setenv("ADMIN_TOKEN", "gw_admin_test_token")
	t.Setenv("GATEWAY_DATA_FILE", filepath.Join(t.TempDir(), "state.json"))
	t.Setenv("PROVIDER_LIMITS_USD", "anthropic=5")
	t.Setenv("CF_ACCOUNT_ID", "acc")
	t.Setenv("CF_API_TOKEN", "tok")

	called := false
	orig := serveHandler
	serveHandler = func(h http.Handler) {
		called = true
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /healthz: got %d", rec.Code)
		}
	}
	t.Cleanup(func() { serveHandler = orig })
	main()
	if !called {
		t.Fatal("expected serveHandler to be called")
	}

	// Enterprise mode + external vendor without CF creds -> workers-ai-mock fallback
	t.Setenv("PERSONAL_MODE", "false")
	t.Setenv("ADMIN_TOKEN", "")
	t.Setenv("GATEWAY_DATA_FILE", "")
	t.Setenv("CF_ACCOUNT_ID", "")
	t.Setenv("CF_API_TOKEN", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	gwEnt := buildGateway()
	rec2 := httptest.NewRecorder()
	gwEnt.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("enterprise /healthz: got %d", rec2.Code)
	}
	if modeLabel(false) != "enterprise" || modeLabel(true) != "personal" {
		t.Fatal("unexpected modeLabel")
	}
}
