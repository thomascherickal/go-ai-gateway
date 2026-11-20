package gateway

import (
	"net/http"
	"time"

	"github.com/thomas/go-ai-gateway/internal/features"
)

// GatewayFeatures bundles optional enterprise control-plane extensions for a Gateway.
type GatewayFeatures struct {
	RateLimiter *features.RateLimiter
	Cache       *features.ResponseCache
	Breaker     *features.CircuitBreaker
	DLP         *features.CustomDLP
	Alerter     *features.BudgetAlerter
	Telemetry   *features.TelemetryRecorder
	ModelPolicy *features.ModelPolicy
	RequireZT   bool
	ZTSuffix    string
}

// AllowRequest evaluates Zero Trust, rate limits, and model allowlists/denylists.
func (f *GatewayFeatures) AllowRequest(r *http.Request, teamID, modelID string, estTokens int, now time.Time) (int, string) {
	if f == nil {
		return http.StatusOK, ""
	}
	if !features.VerifyZeroTrust(r, f.RequireZT, f.ZTSuffix) {
		return http.StatusForbidden, "zero_trust_denied"
	}
	if f.RateLimiter != nil && !f.RateLimiter.Allow(teamID, estTokens, now) {
		return http.StatusTooManyRequests, "rate_limit_exceeded"
	}
	if f.ModelPolicy != nil && !f.ModelPolicy.Allowed(teamID, modelID) {
		return http.StatusForbidden, "model_not_allowed"
	}
	return http.StatusOK, ""
}
