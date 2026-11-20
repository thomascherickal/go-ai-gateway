package redact

import "strings"

var injectionPatterns = []string{
	"ignore previous instructions",
	"ignore all previous instructions",
	"disregard your system prompt",
	"reveal your system prompt",
	"you are now dan",
	"bypass safety filters",
	"override developer instructions",
}

// DetectPromptInjection scans text for common prompt-injection and jailbreak heuristics.
func DetectPromptInjection(text string) []string {
	lower := strings.ToLower(text)
	var hits []string
	for _, pat := range injectionPatterns {
		if strings.Contains(lower, pat) {
			hits = append(hits, pat)
		}
	}
	return hits
}
