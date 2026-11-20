package gateway

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thomas/go-ai-gateway/internal/provider"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
	"github.com/thomas/go-ai-gateway/internal/types"
)

type reserveErrStore struct {
	*store.MemoryStore
	resErr error
}

func (r *reserveErrStore) ReserveBudget(context.Context, string, int64, time.Time) (types.TeamBudget, error) {
	return types.TeamBudget{}, r.resErr
}

func TestGatewayPipelineAndCapsFull(t *testing.T) {
	st := store.NewMemoryStore(true)
	disabled := false
	_, _ = st.UpsertTeam(context.Background(), store.UpsertTeamInput{TeamID: "dis", TeamName: "Dis", RawAPIKey: "gw_dis", MonthlyLimitMicros: 1000, Enabled: &disabled})
	_, _ = st.UpsertTeam(context.Background(), store.UpsertTeamInput{TeamID: "aud", TeamName: "Aud", RawAPIKey: "gw_aud", MonthlyLimitMicros: 100000, MaxTier: types.TierFast, PIIMode: types.PIIModeAuditOnly})
	_ = st.RecordAudit(context.Background(), types.AuditEntry{ID: "a1", CreatedAt: time.Now().UTC(), Provider: router.ProviderWorkersAI, Status: types.StatusOK, CostMicros: 500})

	gwCapped := New(Config{
		Store:              st,
		Router:             router.New(router.Options{}),
		Provider:           &provider.MockProvider{},
		ProviderLimitsFunc: func() string { return "workers-ai=0" },
	})
	for _, tc := range []struct {
		key, body string
		want      int
	}{
		{"gw_dis", `{"model":"fast","messages":[{"role":"user","content":"hi"}]}`, 403},
		{"gw_platform_demo_key", `{"model":"fast","messages":[]}`, 400},
		{"gw_aud", `{"model":"","messages":[{"role":"user","content":"email alice@acme.com"}]}`, 429},
		{"gw_aud", `{"model":"deepseek-r1","messages":[{"role":"user","content":"hi"}]}`, 403},
	} {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(tc.body))
		req.Header.Set("Authorization", "Bearer "+tc.key)
		rec := httptest.NewRecorder()
		gwCapped.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("key=%s: got %d want %d body=%s", tc.key, rec.Code, tc.want, rec.Body.String())
		}
	}

	for _, tc := range []struct {
		err  error
		want int
	}{{store.ErrTeamDisabled, 403}, {errors.New("db err"), 500}} {
		gwErr := New(Config{Store: &reserveErrStore{MemoryStore: st, resErr: tc.err}, Router: router.New(router.Options{}), Provider: &provider.MockProvider{}})
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"fast","messages":[{"role":"user","content":"hi"}]}`))
		req.Header.Set("Authorization", "Bearer gw_platform_demo_key")
		rec := httptest.NewRecorder()
		gwErr.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("reserve err %v: got %d want %d", tc.err, rec.Code, tc.want)
		}
	}

	tr := newProviderCapTracker(nil)
	tr.RecordSpend(context.Background(), failingStore{err: errors.New("x")}, "", 0, time.Now())
	tr.Snapshot(context.Background(), failingStore{err: errors.New("x")}, time.Now())
	gwNoCap := New(Config{Store: st, Router: router.New(router.Options{}), Provider: &provider.MockProvider{}})
	if cands, _ := gwNoCap.filterProviderCaps(context.Background(), nil, time.Now()); len(cands) != 0 {
		t.Fatal("expected nil candidates")
	}
}
