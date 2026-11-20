package gateway

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thomas/go-ai-gateway/internal/provider"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
	"github.com/thomas/go-ai-gateway/internal/types"
)

type failingStore struct{ err error }

func (f failingStore) BackendName() string { return "failing" }
func (f failingStore) LookupTeamByKey(context.Context, string) (types.TeamBudget, error) {
	return types.TeamBudget{}, f.err
}
func (f failingStore) GetTeam(context.Context, string) (types.TeamBudget, error) {
	return types.TeamBudget{}, f.err
}
func (f failingStore) ListTeams(context.Context) ([]types.TeamBudget, error) { return nil, f.err }
func (f failingStore) UpsertTeam(context.Context, store.UpsertTeamInput) (types.TeamBudget, error) {
	return types.TeamBudget{}, f.err
}
func (f failingStore) ReserveBudget(context.Context, string, int64, time.Time) (types.TeamBudget, error) {
	return types.TeamBudget{}, f.err
}
func (f failingStore) ReconcileBudget(context.Context, string, int64, time.Time) (types.TeamBudget, error) {
	return types.TeamBudget{}, f.err
}
func (f failingStore) ResetTeamBudget(context.Context, string, time.Time) (types.TeamBudget, error) {
	return types.TeamBudget{}, f.err
}
func (f failingStore) RecordAudit(context.Context, types.AuditEntry) error { return f.err }
func (f failingStore) ListAudit(context.Context, types.AuditFilter) ([]types.AuditEntry, error) {
	return nil, f.err
}
func (f failingStore) GetSummary(context.Context) (types.AuditSummary, error) {
	return types.AuditSummary{}, f.err
}

func TestGatewayStoreErrorPaths(t *testing.T) {
	gw := New(Config{
		Store:          failingStore{err: errors.New("boom")},
		Router:         router.New(router.Options{}),
		Provider:       &provider.MockProvider{},
		AdminTokenFunc: func() string { return "adm" },
	})
	for _, tc := range []struct {
		m, p, key, body string
		want            int
	}{
		{"POST", "/v1/chat/completions", "k", `{"model":"fast","messages":[{"role":"user","content":"hi"}]}`, 500},
		{"GET", "/admin/teams", "adm", "", 500},
		{"POST", "/admin/teams", "adm", `{"team_id":"a","team_name":"A","api_key":"k","monthly_limit_micros":1000}`, 400},
		{"GET", "/admin/teams/a", "adm", "", 500},
		{"POST", "/admin/teams/a/reset-budget", "adm", "", 500},
		{"GET", "/admin/audit", "adm", "", 500},
		{"GET", "/admin/summary", "adm", "", 500},
	} {
		req := httptest.NewRequest(tc.m, tc.p, bytes.NewBufferString(tc.body))
		req.Header.Set("Authorization", "Bearer "+tc.key)
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("%s %s: got %d want %d", tc.m, tc.p, rec.Code, tc.want)
		}
	}
}
