package redact

import "github.com/thomas/go-ai-gateway/internal/types"

// Text redacts a single string with a fresh session.
func Text(input string) Result {
	sess := NewSession()
	redacted := sess.RedactText(input)
	total, findings := sess.Summary()
	return Result{
		Text:     redacted,
		Total:    total,
		Findings: findings,
	}
}

// Messages scans and redacts all message contents, names, and tool call arguments.
// When mutate is false, it still scans and returns findings while leaving message contents untouched.
func Messages(msgs []types.ChatMessage, mutate bool) MessagesResult {
	sess := NewSession()
	out := make([]types.ChatMessage, len(msgs))
	for i, m := range msgs {
		redactedContent := sess.RedactText(m.Content)
		redactedName := sess.RedactText(m.Name)
		var redactedCalls []types.ToolCall
		if len(m.ToolCalls) > 0 {
			redactedCalls = make([]types.ToolCall, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				tcCopy := tc
				tcCopy.Function.Arguments = sess.RedactText(tc.Function.Arguments)
				redactedCalls[j] = tcCopy
			}
		}
		if mutate {
			out[i] = types.ChatMessage{
				Role:       m.Role,
				Content:    redactedContent,
				Name:       redactedName,
				ToolCalls:  redactedCalls,
				ToolCallID: m.ToolCallID,
			}
		} else {
			out[i] = m
		}
	}
	total, findings := sess.Summary()
	return MessagesResult{
		Messages: out,
		Total:    total,
		Findings: findings,
	}
}
