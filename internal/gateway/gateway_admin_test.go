package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestGatewayAdminAPI(t *testing.T) {
	gw, _ := newTestGateway(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/teams", nil)
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}

	payload := `{"team_id":"ml","team_name":"ML Research","api_key":"gw_ml_123","monthly_limit_micros":25000000,"max_tier":"reasoning","pii_mode":"redact"}`
	reqUpsert := httptest.NewRequest(http.MethodPost, "/admin/teams", strings.NewReader(payload))
	reqUpsert.Header.Set("Authorization", "Bearer test_admin_secret")
	recUpsert := httptest.NewRecorder()
	gw.ServeHTTP(recUpsert, reqUpsert)
	if recUpsert.Code != http.StatusOK {
		t.Fatalf("upsert status = %d, body = %s", recUpsert.Code, recUpsert.Body.String())
	}

	recChat := doChatReq(t, gw, "gw_ml_123", types.ChatCompletionRequest{
		Model: "auto", Messages: []types.ChatMessage{{Role: "user", Content: "hello"}},
	})
	if recChat.Code != http.StatusOK {
		t.Fatalf("chat status = %d, body = %s", recChat.Code, recChat.Body.String())
	}

	reqSum := httptest.NewRequest(http.MethodGet, "/admin/summary", nil)
	reqSum.Header.Set("X-Admin-Token", "test_admin_secret")
	recSum := httptest.NewRecorder()
	gw.ServeHTTP(recSum, reqSum)
	if recSum.Code != http.StatusOK {
		t.Fatalf("summary status = %d", recSum.Code)
	}
	var sum types.AuditSummary
	if err := json.Unmarshal(recSum.Body.Bytes(), &sum); err != nil {
		t.Fatal(err)
	}
	if sum.TotalRequests != 1 || sum.SuccessfulCount != 1 {
		t.Fatalf("unexpected summary: %+v", sum)
	}
}
