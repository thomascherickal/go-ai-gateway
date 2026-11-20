package features

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestArchiverDLPRBACBreakerAndExport(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

	// 6. Multi-region D1 replica selector & R2 NDJSON archiver
	for in, want := range map[string]string{
		"WNAM": "nam", "WEUR": "eur", "APAC": "apac", "OTHER": "primary",
	} {
		if got := SelectReplicaRegion(in); got != want {
			t.Fatalf("SelectReplicaRegion(%q)=%q want %q", in, got, want)
		}
	}
	var ndjson bytes.Buffer
	entries := []types.AuditEntry{{ID: "a1", CreatedAt: now, TeamID: "platform", Provider: "openai", RoutedModel: "gpt-4o", Status: types.StatusOK, TotalTokens: 10, CostMicros: 5}}
	if n, err := ArchiveAuditNDJSON(&ndjson, entries); err != nil || n == 0 {
		t.Fatalf("ArchiveAuditNDJSON: n=%d err=%v", n, err)
	}

	// 7. CustomDLP
	dlp := NewCustomDLP()
	if out, hits := dlp.Apply("none", "hello"); out != "hello" || hits != 0 {
		t.Fatal("expected passthrough for unconfigured team")
	}
	if err := dlp.SetRules("t1", nil, []string{"[invalid"}); err == nil {
		t.Fatal("expected regex compile error")
	}
	if err := dlp.SetRules("t1", []string{"", "ProjectX"}, []string{`ACCT-\d{4}`}); err != nil {
		t.Fatal(err)
	}
	redacted, hits := dlp.Apply("t1", "Secret projectx for ACCT-1234")
	if hits != 2 || !strings.Contains(redacted, "[REDACTED_CUSTOM_KEYWORD]") || !strings.Contains(redacted, "[REDACTED_CUSTOM_REGEX]") {
		t.Fatalf("unexpected DLP result: %q hits=%d", redacted, hits)
	}

	// 8. RBACAuthorizer
	auth := RBACAuthorizer{AdminToken: "adm", AuditorToken: "aud"}
	if auth.ResolveRole("") != RoleNone || auth.ResolveRole("bad") != RoleNone {
		t.Fatal("expected RoleNone")
	}
	if auth.ResolveRole("adm") != RoleAdmin || auth.ResolveRole("aud") != RoleAuditor {
		t.Fatal("expected admin/auditor roles")
	}
	if !auth.Allowed(RoleAdmin, http.MethodPost) || !auth.Allowed(RoleAuditor, http.MethodGet) || auth.Allowed(RoleAuditor, http.MethodPost) || auth.Allowed(RoleNone, http.MethodGet) {
		t.Fatal("unexpected RBAC Allowed result")
	}

	// 9. CircuitBreaker
	cb := NewCircuitBreaker(0, 0)
	if !cb.Allow("openai", now) {
		t.Fatal("expected closed circuit")
	}
	if cb.RecordFailure("openai", now) || cb.RecordFailure("openai", now) || !cb.RecordFailure("openai", now) {
		t.Fatal("expected trip on 3rd failure")
	}
	if cb.Allow("openai", now.Add(10*time.Second)) {
		t.Fatal("expected open circuit during cooldown")
	}
	if !cb.Allow("openai", now.Add(31*time.Second)) {
		t.Fatal("expected half-open after cooldown")
	}
	cb.RecordSuccess("openai")
	if !cb.Allow("openai", now) {
		t.Fatal("expected closed after RecordSuccess")
	}

	// 10. ForecastMonthlyMicros & Audit CSV/JSON export
	if ForecastMonthlyMicros(0, now) != 0 {
		t.Fatal("expected 0 forecast for 0 spend")
	}
	if got := ForecastMonthlyMicros(1500, now); got != 3000 {
		t.Fatalf("expected 3000 forecast on day 15 of 30, got %d", got)
	}
	if csvOut := string(ExportAuditCSV(entries)); !strings.Contains(csvOut, "platform") {
		t.Fatalf("unexpected CSV: %q", csvOut)
	}
	if jsonOut := string(ExportAuditJSON(entries)); !strings.Contains(jsonOut, `"a1"`) {
		t.Fatalf("unexpected JSON: %q", jsonOut)
	}
}
