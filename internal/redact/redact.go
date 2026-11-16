package redact

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/types"
)

// FindingType names a category of personal data or secret detected in prompts.
type FindingType string

const (
	TypeAPIKey     FindingType = "api_key"
	TypeJWT        FindingType = "jwt"
	TypeSSN        FindingType = "ssn"
	TypeCreditCard FindingType = "credit_card"
	TypeEmail      FindingType = "email"
	TypePhone      FindingType = "phone"
	TypeIPv4       FindingType = "ipv4"
)

// Result captures the redacted text and non-sensitive finding telemetry.
type Result struct {
	Text     string
	Total    int
	Findings map[string]int
}

// MessagesResult captures the outcome of scanning/redacting a slice of chat messages.
type MessagesResult struct {
	Messages []types.ChatMessage
	Total    int
	Findings map[string]int
}

var (
	reJWT    = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`)
	reAPIKey = regexp.MustCompile(`\b(?:sk-(?:proj-|ant-)?[A-Za-z0-9_-]{16,}|gh[pous]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|AKIA[0-9A-Z]{16})\b`)
	reBearer = regexp.MustCompile(`(?i)\bBearer\s+([A-Za-z0-9._~+/=-]{16,})`)
	reEmail  = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)
	reSSN    = regexp.MustCompile(`\b(\d{3})-(\d{2})-(\d{4})\b`)
	reCC     = regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`)
	rePhone  = regexp.MustCompile(`(?:\+\d{1,3}[\s.-]?)?(?:\(\d{3}\)|\d{3})[\s.-]\d{3}[\s.-]\d{4}\b`)
	reIPv4   = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
)

// Session keeps placeholder numbering consistent across messages in a single request.
type Session struct {
	placeholders map[string]string
	counters     map[FindingType]int
	findings     map[string]int
	total        int
}

// NewSession initializes a per-request redaction session.
func NewSession() *Session {
	return &Session{
		placeholders: make(map[string]string),
		counters:     make(map[FindingType]int),
		findings:     make(map[string]int),
	}
}

func (s *Session) placeholder(kind FindingType, raw string) string {
	key := string(kind) + ":" + raw
	if p, ok := s.placeholders[key]; ok {
		s.findings[string(kind)]++
		s.total++
		return p
	}
	s.counters[kind]++
	p := fmt.Sprintf("[REDACTED_%s_%d]", strings.ToUpper(string(kind)), s.counters[kind])
	s.placeholders[key] = p
	s.findings[string(kind)]++
	s.total++
	return p
}

// RedactText replaces detected secrets and personal data with stable placeholders.
func (s *Session) RedactText(input string) string {
	if input == "" {
		return ""
	}
	out := input

	// Order matters: redact structured credentials before general patterns (e.g., phone/CC).
	out = reJWT.ReplaceAllStringFunc(out, func(m string) string {
		return s.placeholder(TypeJWT, m)
	})
	out = reAPIKey.ReplaceAllStringFunc(out, func(m string) string {
		return s.placeholder(TypeAPIKey, m)
	})
	out = reBearer.ReplaceAllStringFunc(out, func(m string) string {
		parts := reBearer.FindStringSubmatch(m)
		if len(parts) < 2 {
			return m
		}
		return "Bearer " + s.placeholder(TypeAPIKey, parts[1])
	})
	out = reEmail.ReplaceAllStringFunc(out, func(m string) string {
		return s.placeholder(TypeEmail, strings.ToLower(m))
	})
	out = reSSN.ReplaceAllStringFunc(out, func(m string) string {
		if !isValidSSN(m) {
			return m
		}
		return s.placeholder(TypeSSN, m)
	})
	out = reCC.ReplaceAllStringFunc(out, func(m string) string {
		digits := extractDigits(m)
		if len(digits) < 13 || len(digits) > 19 || !luhnValid(digits) {
			return m
		}
		return s.placeholder(TypeCreditCard, digits)
	})
	out = rePhone.ReplaceAllStringFunc(out, func(m string) string {
		digits := extractDigits(m)
		if len(digits) < 10 || len(digits) > 15 {
			return m
		}
		return s.placeholder(TypePhone, digits)
	})
	out = reIPv4.ReplaceAllStringFunc(out, func(m string) string {
		if !isValidIPv4(m) {
			return m
		}
		return s.placeholder(TypeIPv4, m)
	})

	return out
}

// Summary returns the aggregate count and per-category map of findings seen by this session.
func (s *Session) Summary() (int, map[string]int) {
	if len(s.findings) == 0 {
		return 0, nil
	}
	cp := make(map[string]int, len(s.findings))
	for k, v := range s.findings {
		cp[k] = v
	}
	return s.total, cp
}

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
