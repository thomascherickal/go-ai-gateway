package features

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strconv"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

// ForecastMonthlyMicros projects end-of-month spend based on current UTC day-of-month.
func ForecastMonthlyMicros(spentMicros int64, now time.Time) int64 {
	if spentMicros <= 0 {
		return 0
	}
	day := now.UTC().Day()
	nextMonth := time.Date(now.UTC().Year(), now.UTC().Month()+1, 1, 0, 0, 0, 0, time.UTC)
	daysInMonth := nextMonth.Add(-24 * time.Hour).Day()
	return (spentMicros * int64(daysInMonth)) / int64(day)
}

// ExportAuditCSV serializes audit entries as RFC-4180 CSV.
func ExportAuditCSV(entries []types.AuditEntry) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"id", "created_at", "team_id", "provider", "routed_model", "status", "pii_detected", "total_tokens", "cost_micros"})
	for _, e := range entries {
		_ = w.Write([]string{
			e.ID, e.CreatedAt.UTC().Format(time.RFC3339), e.TeamID, e.Provider, e.RoutedModel,
			string(e.Status), strconv.Itoa(e.PIIDetected), strconv.Itoa(e.TotalTokens), strconv.FormatInt(e.CostMicros, 10),
		})
	}
	w.Flush()
	return buf.Bytes()
}

// ExportAuditJSON serializes audit entries as indented JSON.
func ExportAuditJSON(entries []types.AuditEntry) []byte {
	b, _ := json.MarshalIndent(entries, "", "  ")
	return b
}
