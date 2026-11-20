package gateway

import (
	"math"
	"strconv"
	"strings"
)

// ParseProviderLimitsUSD parses specs like "anthropic=5,openai=5,grok=2.50" into micro-dollars.
func ParseProviderLimitsUSD(raw string) map[string]int64 {
	out := make(map[string]int64)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		prov := strings.ToLower(strings.TrimSpace(k))
		usd, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if prov == "" || err != nil || usd < 0 {
			continue
		}
		out[prov] = int64(math.Round(usd * 1_000_000))
	}
	return out
}
