package gateway

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/store"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func (g *Gateway) handleAdminTeams(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	switch r.Method {
	case http.MethodGet:
		teams, err := g.store.ListTeams(ctx)
		if err != nil {
			writeOpenAIError(w, http.StatusInternalServerError, "server_error", err.Error(), "")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"teams": teams})

	case http.MethodPost:
		body, ok := readLimitedBody(w, r)
		if !ok {
			return
		}
		var req struct {
			TeamID             string          `json:"team_id"`
			TeamName           string          `json:"team_name"`
			APIKey             string          `json:"api_key"`
			MonthlyLimitMicros int64           `json:"monthly_limit_micros"`
			MaxTier            types.ModelTier `json:"max_tier"`
			PIIMode            types.PIIMode   `json:"pii_mode"`
			Enabled            *bool           `json:"enabled"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON", "")
			return
		}
		if req.MaxTier != "" && types.TierRank(req.MaxTier) == 0 {
			writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "max_tier must be fast, balanced, or reasoning", "")
			return
		}
		if req.PIIMode != "" && req.PIIMode != types.PIIModeRedact && req.PIIMode != types.PIIModeBlock && req.PIIMode != types.PIIModeAuditOnly {
			writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "pii_mode must be redact, block, or audit_only", "")
			return
		}
		team, err := g.store.UpsertTeam(ctx, store.UpsertTeamInput{
			TeamID:             strings.TrimSpace(req.TeamID),
			TeamName:           strings.TrimSpace(req.TeamName),
			RawAPIKey:          strings.TrimSpace(req.APIKey),
			MonthlyLimitMicros: req.MonthlyLimitMicros,
			MaxTier:            req.MaxTier,
			PIIMode:            req.PIIMode,
			Enabled:            req.Enabled,
		})
		if err != nil {
			writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error(), "")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"team": team})

	default:
		writeOpenAIError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed", "")
	}
}

func (g *Gateway) handleAdminTeamAction(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin/teams/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 2 && parts[1] == "reset-budget" && r.Method == http.MethodPost {
		team, err := g.store.ResetTeamBudget(r.Context(), parts[0], g.now())
		if err != nil {
			writeStoreLookupError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"team": team})
		return
	}
	if len(parts) == 1 && parts[0] != "" && r.Method == http.MethodGet {
		team, err := g.store.GetTeam(r.Context(), parts[0])
		if err != nil {
			writeStoreLookupError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"team": team})
		return
	}
	writeOpenAIError(w, http.StatusNotFound, "not_found", "unknown admin team route", "")
}
