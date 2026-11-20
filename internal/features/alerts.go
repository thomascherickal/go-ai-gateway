package features

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// BudgetAlerter sends webhook notifications when teams cross 60%, 80%, or 100% of monthly budget.
type BudgetAlerter struct {
	mu         sync.Mutex
	WebhookURL string
	Client     *http.Client
	sent       map[string]bool
}

// NewBudgetAlerter creates a budget threshold webhook alerter.
func NewBudgetAlerter(webhookURL string, client *http.Client) *BudgetAlerter {
	if client == nil {
		client = http.DefaultClient
	}
	return &BudgetAlerter{
		WebhookURL: webhookURL,
		Client:     client,
		sent:       make(map[string]bool),
	}
}

// CheckAndNotify evaluates spend/limit and fires a webhook for newly crossed thresholds (60/80/100%).
func (a *BudgetAlerter) CheckAndNotify(ctx context.Context, teamID, periodYM string, spent, limit int64) ([]int, error) {
	if limit <= 0 || spent <= 0 {
		return nil, nil
	}
	pct := int((spent * 100) / limit)
	var triggered []int
	a.mu.Lock()
	for _, th := range []int{60, 80, 100} {
		key := fmt.Sprintf("%s:%s:%d", teamID, periodYM, th)
		if pct >= th && !a.sent[key] {
			a.sent[key] = true
			triggered = append(triggered, th)
		}
	}
	a.mu.Unlock()

	if len(triggered) == 0 || a.WebhookURL == "" {
		return triggered, nil
	}
	payload, _ := json.Marshal(map[string]any{
		"team_id":    teamID,
		"period_ym":  periodYM,
		"spent":      spent,
		"limit":      limit,
		"thresholds": triggered,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.WebhookURL, bytes.NewReader(payload))
	if err != nil {
		return triggered, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.Client.Do(req)
	if err != nil {
		return triggered, err
	}
	_ = resp.Body.Close()
	return triggered, nil
}
