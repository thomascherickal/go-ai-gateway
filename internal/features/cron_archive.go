package features

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

// RunDailyAuditArchive uploads an NDJSON audit snapshot to an R2-compatible HTTP endpoint.
func RunDailyAuditArchive(ctx context.Context, entries []types.AuditEntry, endpoint string, client *http.Client, now time.Time) (int, error) {
	if endpoint == "" || len(entries) == 0 {
		return 0, nil
	}
	if client == nil {
		client = http.DefaultClient
	}
	var buf bytes.Buffer
	n, _ := ArchiveAuditNDJSON(&buf, entries)
	url := fmt.Sprintf("%s/audit-%s.ndjson", endpoint, now.UTC().Format("2006-01-02"))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, &buf)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("r2 archive status %d", resp.StatusCode)
	}
	return n, nil
}
