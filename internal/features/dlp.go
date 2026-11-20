package features

import (
	"regexp"
	"strings"
	"sync"
)

// TeamDLPRule holds custom regex patterns and case-insensitive keywords for a team.
type TeamDLPRule struct {
	Keywords []string
	Patterns []*regexp.Regexp
}

// CustomDLP manages per-team custom regex and keyword DLP rules.
type CustomDLP struct {
	mu    sync.RWMutex
	rules map[string]TeamDLPRule
}

// NewCustomDLP initializes an empty custom DLP registry.
func NewCustomDLP() *CustomDLP {
	return &CustomDLP{rules: make(map[string]TeamDLPRule)}
}

// SetRules compiles and registers custom keywords and regex patterns for a team.
func (d *CustomDLP) SetRules(teamID string, keywords, patterns []string) error {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return err
		}
		compiled = append(compiled, re)
	}
	d.mu.Lock()
	d.rules[teamID] = TeamDLPRule{Keywords: keywords, Patterns: compiled}
	d.mu.Unlock()
	return nil
}

// Apply redacts custom team keywords and regex matches in input, returning redacted text and match count.
func (d *CustomDLP) Apply(teamID, input string) (string, int) {
	d.mu.RLock()
	rule, ok := d.rules[teamID]
	d.mu.RUnlock()
	if !ok || input == "" {
		return input, 0
	}
	out := input
	hits := 0
	for _, kw := range rule.Keywords {
		if kw == "" {
			continue
		}
		if strings.Contains(strings.ToLower(out), strings.ToLower(kw)) {
			re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(kw))
			out = re.ReplaceAllStringFunc(out, func(string) string {
				hits++
				return "[REDACTED_CUSTOM_KEYWORD]"
			})
		}
	}
	for _, re := range rule.Patterns {
		out = re.ReplaceAllStringFunc(out, func(string) string {
			hits++
			return "[REDACTED_CUSTOM_REGEX]"
		})
	}
	return out, hits
}
