package gateway

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/thomas/go-ai-gateway/internal/features"
	"github.com/thomas/go-ai-gateway/internal/provider"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
)

func TestAdminExportForecastRBACAndFeatures(t *testing.T) {
	gw := New(Config{
		Store: store.NewMemoryStore(true), Router: router.New(router.Options{}),
		Provider: &provider.MockProvider{}, AdminTokenFunc: func() string { return "adm" }, AuditorTokenFunc: func() string { return "aud" },
	})
	for _, tc := range []struct {
		m, p, tok string
		want      int
	}{
		{"GET", "/admin/audit/export?format=csv", "aud", 200}, {"GET", "/admin/audit/export?format=json", "aud", 200},
		{"POST", "/admin/audit/export", "adm", 405}, {"GET", "/admin/forecast", "aud", 200},
		{"POST", "/admin/forecast", "adm", 405}, {"POST", "/admin/teams", "aud", 403},
	} {
		req := httptest.NewRequest(tc.m, tc.p, nil)
		req.Header.Set("Authorization", "Bearer "+tc.tok)
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("%s %s: got %d want %d", tc.m, tc.p, rec.Code, tc.want)
		}
	}
	gwFail := New(Config{Store: failingStore{err: errors.New("db")}, Router: router.New(router.Options{}), Provider: &provider.MockProvider{}, AdminTokenFunc: func() string { return "adm" }})
	for _, p := range []string{"/admin/audit/export", "/admin/forecast"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		req.Header.Set("Authorization", "Bearer adm")
		rec := httptest.NewRecorder()
		gwFail.ServeHTTP(rec, req)
		if rec.Code != 500 {
			t.Fatalf("fail %s: got %d", p, rec.Code)
		}
	}

	// GatewayFeatures.AllowRequest branches
	var nilF *GatewayFeatures
	reqNoZT := httptest.NewRequest(http.MethodGet, "/", nil)
	reqZT := httptest.NewRequest(http.MethodGet, "/", nil)
	reqZT.Header.Set("Cf-Access-Jwt-Assertion", "jwt")
	rl := features.NewRateLimiter()
	rl.SetLimit("t", features.RateLimitConfig{RPM: 1})
	mp := features.NewModelPolicy()
	mp.SetPolicy("t", nil, []string{"banned"})
	gf := &GatewayFeatures{RateLimiter: rl, ModelPolicy: mp, RequireZT: true}
	if c, _ := nilF.AllowRequest(reqNoZT, "t", "m", 1, time.Now()); c != 200 {
		t.Fatalf("nil features: %d", c)
	}
	if c, _ := gf.AllowRequest(reqNoZT, "t", "ok", 1, time.Now()); c != 403 {
		t.Fatalf("ZT deny: %d", c)
	}
	if c, _ := gf.AllowRequest(reqZT, "t", "banned", 1, time.Now()); c != 403 {
		t.Fatalf("model deny: %d", c)
	}
	if c, _ := gf.AllowRequest(reqZT, "t", "ok", 1, time.Now()); c != 429 {
		t.Fatalf("rate limit: %d", c)
	}
	if c, _ := (&GatewayFeatures{}).AllowRequest(reqZT, "t", "ok", 1, time.Now()); c != 200 {
		t.Fatalf("open features: %d", c)
	}
}

func TestGatewayP95LatencyUnderLoad(t *testing.T) {
	gw := New(Config{Store: store.NewMemoryStore(true), Router: router.New(router.Options{}), Provider: &provider.MockProvider{}})
	const n = 20
	latencies := make([]time.Duration, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			start := time.Now()
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"fast","messages":[{"role":"user","content":"hi"}]}`))
			req.Header.Set("Authorization", "Bearer gw_platform_demo_key")
			gw.ServeHTTP(httptest.NewRecorder(), req)
			latencies[idx] = time.Since(start)
		}(i)
	}
	wg.Wait()
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	if p95 := latencies[int(float64(n)*0.95)-1]; p95 > 500*time.Millisecond {
		t.Fatalf("p95 latency too high: %v", p95)
	}
}
