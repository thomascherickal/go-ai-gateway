//go:build !js

package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestBuildGatewayPersonalModeAndEnterpriseMode(t *testing.T) {
	t.Setenv("PERSONAL_MODE", "true")
	t.Setenv("PERSONAL_API_KEY", "gw_personal_test_key")
	t.Setenv("ADMIN_TOKEN", "gw_admin_test_token")
	t.Setenv("GATEWAY_DATA_FILE", filepath.Join(t.TempDir(), "state.json"))
	t.Setenv("PROVIDER_LIMITS_USD", "anthropic=5")

	gw := buildGateway()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz: got %d, want 200", rec.Code)
	}

	// Enterprise mode with memory store fallback & default dev admin token
	t.Setenv("PERSONAL_MODE", "false")
	t.Setenv("ADMIN_TOKEN", "")
	t.Setenv("GATEWAY_DATA_FILE", "")
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
