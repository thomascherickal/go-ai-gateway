package redact

import (
	"strings"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestSecurityBearerAndToolArgRedaction(t *testing.T) {
	if res := Text(""); res.Text != "" || res.Total != 0 {
		t.Fatalf("empty input unexpected: %+v", res)
	}
	bearer := Text("Authorization: Bearer super_secret_token_value_123456")
	if strings.Contains(bearer.Text, "super_secret_token_value_123456") || bearer.Findings["api_key"] != 1 {
		t.Fatalf("bearer not redacted: %+v", bearer)
	}
	msgs := []types.ChatMessage{{
		Role: "assistant",
		ToolCalls: []types.ToolCall{{
			ID: "c1", Type: "function",
			Function: types.ToolCallFunction{Name: "send", Arguments: `{"to":"alice@corp.io"}`},
		}},
	}}
	res := Messages(msgs, true)
	if strings.Contains(res.Messages[0].ToolCalls[0].Function.Arguments, "alice@corp.io") {
		t.Fatalf("tool call argument leaked PII: %+v", res.Messages[0].ToolCalls)
	}
}

func BenchmarkRedactText(b *testing.B) {
	sample := "Contact alice@acme.com SSN 123-45-6789 card 4532-0151-1283-0366 host 10.0.1.5 key sk-proj-abcdefghijklmnopqrstuvwxyz1234"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Text(sample)
	}
}
