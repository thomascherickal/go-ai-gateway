package gateway

import (
	"net/http"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/features"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func (g *Gateway) handleAdminAuditExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed", "")
		return
	}
	entries, err := g.store.ListAudit(r.Context(), types.AuditFilter{Limit: 200})
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "server_error", err.Error(), "")
		return
	}
	if strings.EqualFold(r.URL.Query().Get("format"), "csv") {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="audit-logs.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(features.ExportAuditCSV(entries))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(features.ExportAuditJSON(entries))
}

func (g *Gateway) handleAdminForecast(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed", "")
		return
	}
	teams, err := g.store.ListTeams(r.Context())
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "server_error", err.Error(), "")
		return
	}
	now := g.now()
	out := make([]map[string]any, 0, len(teams))
	for _, t := range teams {
		out = append(out, map[string]any{
			"team_id":                 t.TeamID,
			"spent_micros":            t.SpentMicros,
			"monthly_limit_micros":    t.MonthlyLimitMicros,
			"forecast_monthly_micros": features.ForecastMonthlyMicros(t.SpentMicros, now),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"teams": out})
}
