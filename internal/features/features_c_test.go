package features

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestDurableLimiterModelPolicyCronAndZeroTrust(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	statusClient := func(code int) *http.Client {
		return &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString("{}"))}, nil
		})}
	}
	failHTTP := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("net") })}

	// DurableRateLimiter
	if ok, err := NewDurableRateLimiter("", nil).Allow(ctx, "t1", 10); !ok || err != nil {
		t.Fatalf("empty endpoint: %v %v", ok, err)
	}
	if ok, err := NewDurableRateLimiter("http://do.local", statusClient(200)).Allow(ctx, "t1", 10); !ok || err != nil {
		t.Fatalf("200 allow: %v %v", ok, err)
	}
	if ok, err := NewDurableRateLimiter("http://do.local", statusClient(429)).Allow(ctx, "t1", 10); ok || err != nil {
		t.Fatalf("429 deny: %v %v", ok, err)
	}
	if _, err := NewDurableRateLimiter("://bad", statusClient(200)).Allow(ctx, "t1", 10); err == nil {
		t.Fatal("expected bad URL err")
	}
	if _, err := NewDurableRateLimiter("http://do.local", failHTTP).Allow(ctx, "t1", 10); err == nil {
		t.Fatal("expected net err")
	}

	// ModelPolicy
	mp := NewModelPolicy()
	if !mp.Allowed("none", "gpt-4o") {
		t.Fatal("expected unconfigured team allowed")
	}
	mp.SetPolicy("t1", []string{"gpt-4o", "claude-sonnet-4-5"}, []string{"gpt-4o"})
	if mp.Allowed("t1", "gpt-4o") || !mp.Allowed("t1", "claude-sonnet-4-5") || mp.Allowed("t1", "other") {
		t.Fatal("unexpected allow/deny policy")
	}
	mp.SetPolicy("t2", nil, []string{"blocked"})
	if !mp.Allowed("t2", "any") || mp.Allowed("t2", "blocked") {
		t.Fatal("unexpected deny-only policy")
	}

	// RunDailyAuditArchive
	entries := []types.AuditEntry{{ID: "1", CreatedAt: now}}
	if n, err := RunDailyAuditArchive(ctx, nil, "", nil, now); n != 0 || err != nil {
		t.Fatalf("empty archive: %d %v", n, err)
	}
	if n, err := RunDailyAuditArchive(ctx, entries, "http://r2.local", statusClient(200), now); n == 0 || err != nil {
		t.Fatalf("ok archive: %d %v", n, err)
	}
	if _, err := RunDailyAuditArchive(ctx, entries, "://bad", nil, now); err == nil {
		t.Fatal("expected bad URL err")
	}
	if _, err := RunDailyAuditArchive(ctx, entries, "http://r2.local", failHTTP, now); err == nil {
		t.Fatal("expected net err")
	}
	if _, err := RunDailyAuditArchive(ctx, entries, "http://r2.local", statusClient(500), now); err == nil {
		t.Fatal("expected 500 err")
	}

	// VerifyZeroTrust
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if !VerifyZeroTrust(req, false, "") || VerifyZeroTrust(req, true, "") {
		t.Fatal("unexpected JWT check")
	}
	req.Header.Set("Cf-Access-Jwt-Assertion", "eyJ.tok.sig")
	req.Header.Set("Cf-Access-Authenticated-User-Email", "alice@acme.com")
	if !VerifyZeroTrust(req, true, "@acme.com") || VerifyZeroTrust(req, true, "@other.com") {
		t.Fatal("unexpected email suffix check")
	}
}
