//go:build !js

package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalGatewaySmokeE2E(t *testing.T) {
	t.Setenv("PERSONAL_MODE", "true")
	t.Setenv("PERSONAL_API_KEY", "gw_e2e_personal")
	t.Setenv("PERSONAL_MONTHLY_BUDGET_USD", "10")
	t.Setenv("ADMIN_TOKEN", "e2e_admin")
	t.Setenv("GATEWAY_DATA_FILE", filepath.Join(t.TempDir(), "state.json"))

	gw := buildGateway()

	recHealth := httptest.NewRecorder()
	gw.ServeHTTP(recHealth, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recHealth.Code != http.StatusOK || !strings.Contains(recHealth.Body.String(), `"status":"ok"`) {
		t.Fatalf("healthz failed: %d %s", recHealth.Code, recHealth.Body.String())
	}

	recRoot := httptest.NewRecorder()
	gw.ServeHTTP(recRoot, httptest.NewRequest(http.MethodGet, "/", nil))
	if recRoot.Code != http.StatusOK || !strings.Contains(recRoot.Body.String(), "AI Traffic Control Plane") {
		t.Fatalf("root dashboard failed: %d", recRoot.Code)
	}

	recModels := httptest.NewRecorder()
	gw.ServeHTTP(recModels, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if recModels.Code != http.StatusOK || !strings.Contains(recModels.Body.String(), "claude-sonnet-4-5") {
		t.Fatalf("models failed: %d", recModels.Code)
	}

	reqChat := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		bytes.NewBufferString(`{"model":"auto","messages":[{"role":"user","content":"Hello"}]}`))
	reqChat.Header.Set("Authorization", "Bearer gw_e2e_personal")
	recChat := httptest.NewRecorder()
	gw.ServeHTTP(recChat, reqChat)
	if recChat.Code != http.StatusOK {
		t.Fatalf("chat failed: %d %s", recChat.Code, recChat.Body.String())
	}
}
