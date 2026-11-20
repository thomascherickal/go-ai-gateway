package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func doJSONPost(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	headers map[string]string,
	payload any,
	providerLabel string,
) ([]byte, error) {
	if client == nil {
		client = http.DefaultClient
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		if v != "" {
			req.Header.Set(k, v)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s status %d: %s", providerLabel, resp.StatusCode, truncateString(string(respBytes), 240))
	}
	return respBytes, nil
}

func ensureUsage(u types.Usage, msgs []types.ChatMessage, content string) types.Usage {
	if u.TotalTokens == 0 && (u.PromptTokens > 0 || u.CompletionTokens > 0) {
		u.TotalTokens = u.PromptTokens + u.CompletionTokens
		return u
	}
	if u.TotalTokens > 0 {
		return u
	}
	u.PromptTokens = router.EstimateMessagesTokens(msgs)
	u.CompletionTokens = router.EstimateTextTokens(content)
	u.TotalTokens = u.PromptTokens + u.CompletionTokens
	u.Estimated = true
	return u
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
