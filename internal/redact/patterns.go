package redact

import (
	"regexp"

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
