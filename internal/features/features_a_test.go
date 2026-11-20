package features

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type errResponseWriter struct{ hdr http.Header }

func (e errResponseWriter) Header() http.Header       { return e.hdr }
func (e errResponseWriter) Write([]byte) (int, error) { return 0, errors.New("write fail") }
func (e errResponseWriter) WriteHeader(int)           {}

func TestRateLimitCacheTelemetryAlertsStream(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

	// 1. RateLimiter
	rl := NewRateLimiter()
	if !rl.Allow("unconfigured", 100, now) {
		t.Fatal("expected unconfigured team allowed")
	}
	rl.SetLimit("t1", RateLimitConfig{RPM: 2, TPM: 100})
	if !rl.Allow("t1", 60, now) || rl.Allow("t1", 50, now) || !rl.Allow("t1", 30, now) || rl.Allow("t1", 1, now) {
		t.Fatal("unexpected RPM/TPM enforcement")
	}
	if !rl.Allow("t1", 50, now.Add(time.Minute)) {
		t.Fatal("expected window reset after 1m")
	}

	// 2. ResponseCache
	c := NewResponseCache(0)
	key := CacheKey("m1", "sha")
	if _, ok := c.Get(key, now); ok {
		t.Fatal("expected cache miss")
	}
	c.Set(key, types.ChatCompletionResponse{ID: "r1"}, now)
	if got, ok := c.Get(key, now.Add(time.Minute)); !ok || got.ID != "r1" {
		t.Fatalf("expected cache hit: %+v %v", got, ok)
	}
	if _, ok := c.Get(key, now.Add(10*time.Minute)); ok {
		t.Fatal("expected expired miss")
	}

	// 3. TelemetryRecorder
	okHTTP := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString("{}"))}, nil
	})}
	failHTTP := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("net") })}
	trNoEP := NewTelemetryRecorder("", nil)
	_ = trNoEP.Record(context.Background(), SpanRecord{Provider: "openai", LatencyMs: 50})
	_ = trNoEP.Record(context.Background(), SpanRecord{Provider: "openai", LatencyMs: 250})
	_ = trNoEP.Record(context.Background(), SpanRecord{Provider: "openai", LatencyMs: 900})
	if hist := trNoEP.Histogram(); hist["openai"]["le_100ms"] != 1 || hist["openai"]["le_500ms"] != 1 || hist["openai"]["gt_500ms"] != 1 {
		t.Fatalf("unexpected histogram: %v", hist)
	}
	if err := NewTelemetryRecorder("http://otlp.local", okHTTP).Record(context.Background(), SpanRecord{Provider: "a"}); err != nil {
		t.Fatalf("expected OTLP export ok: %v", err)
	}
	if err := NewTelemetryRecorder("://bad", okHTTP).Record(context.Background(), SpanRecord{Provider: "a"}); err == nil {
		t.Fatal("expected bad URL err")
	}
	if err := NewTelemetryRecorder("http://otlp.local", failHTTP).Record(context.Background(), SpanRecord{Provider: "a"}); err == nil {
		t.Fatal("expected transport err")
	}

	// 4. BudgetAlerter & 5. StreamPassThrough
	al := NewBudgetAlerter("", nil)
	if th, _ := al.CheckAndNotify(context.Background(), "t1", "2026-11", 0, 100); len(th) != 0 {
		t.Fatal("expected no alert on 0 spend")
	}
	if th, _ := al.CheckAndNotify(context.Background(), "t1", "2026-11", 85, 100); len(th) != 2 {
		t.Fatalf("expected 60 and 80 alerts, got %v", th)
	}
	if th, _ := NewBudgetAlerter("http://hook.local", okHTTP).CheckAndNotify(context.Background(), "t1", "2026-11", 100, 100); len(th) != 3 {
		t.Fatalf("expected 60,80,100 alerts, got %v", th)
	}
	_, _ = NewBudgetAlerter("://bad", okHTTP).CheckAndNotify(context.Background(), "t1", "2026-11", 100, 100)
	_, _ = NewBudgetAlerter("http://hook.local", failHTTP).CheckAndNotify(context.Background(), "t1", "2026-11", 100, 100)
	rec := httptest.NewRecorder()
	if err := StreamPassThrough(rec, strings.NewReader("data: hello\n\n")); err != nil || !strings.Contains(rec.Body.String(), "data: hello") {
		t.Fatalf("StreamPassThrough: %v", err)
	}
	if err := StreamPassThrough(errResponseWriter{hdr: make(http.Header)}, strings.NewReader("data: x\n")); err == nil {
		t.Fatal("expected write error")
	}
}
