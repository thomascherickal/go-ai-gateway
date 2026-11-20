package redact

import (
	"strings"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/types"
)

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
