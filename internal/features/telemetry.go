package features

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
)

// SpanRecord represents a lightweight OTLP-compatible upstream call span.
type SpanRecord struct {
	TraceID   string `json:"trace_id"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	LatencyMs int64  `json:"latency_ms"`
	Status    string `json:"status"`
}

// TelemetryRecorder tracks per-provider latency histograms and exports OTLP spans.
type TelemetryRecorder struct {
	mu       sync.Mutex
	Endpoint string
	Client   *http.Client
	buckets  map[string]map[string]int64
}

// NewTelemetryRecorder creates a recorder with optional OTLP HTTP endpoint.
func NewTelemetryRecorder(endpoint string, client *http.Client) *TelemetryRecorder {
	if client == nil {
		client = http.DefaultClient
	}
	return &TelemetryRecorder{
		Endpoint: endpoint,
		Client:   client,
		buckets:  make(map[string]map[string]int64),
	}
}

func latencyBucket(ms int64) string {
	switch {
	case ms <= 100:
		return "le_100ms"
	case ms <= 500:
		return "le_500ms"
	default:
		return "gt_500ms"
	}
}

// Record updates the provider latency histogram and exports the span if Endpoint is set.
func (t *TelemetryRecorder) Record(ctx context.Context, span SpanRecord) error {
	t.mu.Lock()
	if t.buckets[span.Provider] == nil {
		t.buckets[span.Provider] = make(map[string]int64)
	}
	t.buckets[span.Provider][latencyBucket(span.LatencyMs)]++
	t.mu.Unlock()

	if t.Endpoint == "" {
		return nil
	}
	b, _ := json.Marshal(map[string]any{"resourceSpans": []SpanRecord{span}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.Endpoint, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.Client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

// Histogram returns a snapshot of recorded latency buckets per provider.
func (t *TelemetryRecorder) Histogram() map[string]map[string]int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make(map[string]map[string]int64, len(t.buckets))
	for p, m := range t.buckets {
		cp := make(map[string]int64, len(m))
		for k, v := range m {
			cp[k] = v
		}
		out[p] = cp
	}
	return out
}
