package redact

import (
	"fmt"
	"strings"
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
	out = reJWT.ReplaceAllStringFunc(out, func(m string) string { return s.placeholder(TypeJWT, m) })
	out = reAPIKey.ReplaceAllStringFunc(out, func(m string) string { return s.placeholder(TypeAPIKey, m) })
	out = reBearer.ReplaceAllStringFunc(out, func(m string) string {
		parts := reBearer.FindStringSubmatch(m)
		return "Bearer " + s.placeholder(TypeAPIKey, parts[1])
	})
	out = reEmail.ReplaceAllStringFunc(out, func(m string) string { return s.placeholder(TypeEmail, strings.ToLower(m)) })
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
		return s.placeholder(TypePhone, extractDigits(m))
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
