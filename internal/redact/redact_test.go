package redact

import (
	"strings"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestRedactText(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantContain []string
		wantAbsent  []string
		wantType    string
		wantCount   int
	}{
		{
			name:        "email deduplication across occurrences",
			input:       "Email Alice at Alice@acme.com and cc alice@acme.com or bob@acme.org",
			wantContain: []string{"[REDACTED_EMAIL_1]", "[REDACTED_EMAIL_2]"},
			wantAbsent:  []string{"Alice@acme.com", "alice@acme.com", "bob@acme.org"},
			wantType:    "email",
			wantCount:   3,
		},
		{
			name:        "valid SSN redacted and invalid SSN ignored",
			input:       "Valid SSN 123-45-6789 vs invalid 000-12-3456 and 999-12-3456",
			wantContain: []string{"[REDACTED_SSN_1]", "000-12-3456", "999-12-3456"},
			wantAbsent:  []string{"123-45-6789"},
			wantType:    "ssn",
			wantCount:   1,
		},
		{
			name:        "valid Luhn credit card vs non-Luhn order ID",
			input:       "Card 4532-0151-1283-0366 and order 1234-5678-9012-3456",
			wantContain: []string{"[REDACTED_CREDIT_CARD_1]", "1234-5678-9012-3456"},
			wantAbsent:  []string{"4532-0151-1283-0366"},
			wantType:    "credit_card",
			wantCount:   1,
		},
		{
			name:        "phone number redacted but ISO date preserved",
			input:       "Call (415) 555-0199 or +1-650-555-0144 on 2026-11-16",
			wantContain: []string{"[REDACTED_PHONE_1]", "[REDACTED_PHONE_2]", "2026-11-16"},
			wantAbsent:  []string{"415", "650-555-0144"},
			wantType:    "phone",
			wantCount:   2,
		},
		{
			name:        "API keys and JWT redacted",
			input:       "Use sk-proj-abcdefghijklmnopqrstuvwxyz1234 and ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ123456 and eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			wantContain: []string{"[REDACTED_API_KEY_1]", "[REDACTED_API_KEY_2]", "[REDACTED_JWT_1]"},
			wantAbsent:  []string{"sk-proj-", "ghp_", "eyJhbGci"},
			wantType:    "api_key",
			wantCount:   2,
		},
		{
			name:        "IPv4 redacted and invalid octets ignored",
			input:       "Host 10.24.8.15 failed; version 999.1.2.3 ignored",
			wantContain: []string{"[REDACTED_IPV4_1]", "999.1.2.3"},
			wantAbsent:  []string{"10.24.8.15"},
			wantType:    "ipv4",
			wantCount:   1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := Text(tc.input)
			for _, want := range tc.wantContain {
				if !strings.Contains(res.Text, want) {
					t.Errorf("expected redacted text %q to contain %q", res.Text, want)
				}
			}
			for _, absent := range tc.wantAbsent {
				if strings.Contains(res.Text, absent) {
					t.Errorf("expected redacted text %q NOT to contain %q", res.Text, absent)
				}
			}
			if got := res.Findings[tc.wantType]; got != tc.wantCount {
				t.Errorf("findings[%q] = %d, want %d (all: %v)", tc.wantType, got, tc.wantCount, res.Findings)
			}
		})
	}
}

func TestMessagesConsistentSession(t *testing.T) {
	msgs := []types.ChatMessage{
		{Role: "system", Content: "Never expose alice@corp.example"},
		{Role: "user", Content: "Forward invoice to alice@corp.example and 192.168.10.5"},
	}

	redacted := Messages(msgs, true)
	if redacted.Total != 3 {
		t.Fatalf("total findings = %d, want 3", redacted.Total)
	}
	if !strings.Contains(redacted.Messages[0].Content, "[REDACTED_EMAIL_1]") ||
		!strings.Contains(redacted.Messages[1].Content, "[REDACTED_EMAIL_1]") {
		t.Fatalf("expected consistent placeholder across messages: %+v", redacted.Messages)
	}
	// Original slice must not be mutated.
	if !strings.Contains(msgs[0].Content, "alice@corp.example") {
		t.Fatalf("input slice was unexpectedly mutated")
	}

	auditOnly := Messages(msgs, false)
	if auditOnly.Total != 3 {
		t.Fatalf("auditOnly total = %d, want 3", auditOnly.Total)
	}
	if auditOnly.Messages[1].Content != msgs[1].Content {
		t.Fatalf("auditOnly mode mutated content: %q", auditOnly.Messages[1].Content)
	}
}

