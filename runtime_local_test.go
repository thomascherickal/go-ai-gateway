//go:build !js

package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestMainAndLocalRuntimeFullCoverage(t *testing.T) {
	t.Setenv("PERSONAL_MODE", "true")
	t.Setenv("PERSONAL_API_KEY", "")
	t.Setenv("PERSONAL_MONTHLY_BUDGET_USD", "12.5")
	t.Setenv("PERSONAL_PII_MODE", "")
	t.Setenv("PERSONAL_MAX_TIER", "")
	t.Setenv("ADMIN_TOKEN", "gw_admin_test_token")
	t.Setenv("GATEWAY_DATA_FILE", filepath.Join(t.TempDir(), "state.json"))
	t.Setenv("PROVIDER_LIMITS_USD", "anthropic=5")
	t.Setenv("CF_ACCOUNT_ID", "acc")
	t.Setenv("CF_API_TOKEN", "tok")

	orig := serveHandler
	serveHandler = func(h http.Handler) {
		req := httptest.NewRequest(http.MethodGet, "/admin/providers", nil)
		req.Header.Set("Authorization", "Bearer gw_admin_test_token")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /admin/providers: got %d", rec.Code)
		}
	}
	t.Cleanup(func() { serveHandler = orig })
	main()

	// Personal mode with default home path & home-error fallback
	t.Setenv("GATEWAY_DATA_FILE", "")
	origHome := userHomeDir
	t.Cleanup(func() { userHomeDir = origHome })
	tmpHome := t.TempDir()
	userHomeDir = func() (string, error) { return tmpHome, nil }
	_ = initLocalStore(true)
	userHomeDir = func() (string, error) { return "", errors.New("no home") }
	t.Cleanup(func() { _ = os.Remove("gateway-state.json") })
	_ = initLocalStore(true)

	// Unwritable data file (directory) falls back to memory store
	t.Setenv("GATEWAY_DATA_FILE", t.TempDir())
	_ = initLocalStore(false)

	// No vendor keys -> mock-all-vendors; then single vendor -> workers-ai-mock
	t.Setenv("PERSONAL_MODE", "false")
	t.Setenv("ADMIN_TOKEN", "")
	t.Setenv("GATEWAY_DATA_FILE", "")
	t.Setenv("CF_ACCOUNT_ID", "")
	t.Setenv("CF_API_TOKEN", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	_ = buildGateway()

	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	_ = buildGateway()
	if modeLabel(false) != "enterprise" || modeLabel(true) != "personal" {
		t.Fatal("unexpected modeLabel")
	}
}
