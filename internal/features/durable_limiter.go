package features

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

// DurableRateLimiter delegates global RPM/TPM checks to a Cloudflare Durable Object endpoint.
type DurableRateLimiter struct {
	Endpoint string
	Client   *http.Client
}

// NewDurableRateLimiter creates a Durable Object rate-limiter adapter.
func NewDurableRateLimiter(endpoint string, client *http.Client) *DurableRateLimiter {
	if client == nil {
		client = http.DefaultClient
	}
	return &DurableRateLimiter{Endpoint: endpoint, Client: client}
}

// Allow queries the Durable Object rate-limiter stub for teamID and estTokens.
func (d *DurableRateLimiter) Allow(ctx context.Context, teamID string, estTokens int) (bool, error) {
	if d.Endpoint == "" {
		return true, nil
	}
	body, _ := json.Marshal(map[string]any{"team_id": teamID, "tokens": estTokens})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.Endpoint, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.Client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return false, nil
	}
	return resp.StatusCode >= 200 && resp.StatusCode < 300, nil
}
