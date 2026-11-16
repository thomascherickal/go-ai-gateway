package gateway

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/store"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func (g *Gateway) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expected := strings.TrimSpace(g.adminToken())
		if expected == "" {
			writeOpenAIError(w, http.StatusForbidden, "admin_auth_error", "ADMIN_TOKEN is not configured", "admin_disabled")
			return
		}
		provided := extractBearerOrHeader(r, "X-Admin-Token")
		if len(provided) == 0 || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			writeOpenAIError(w, http.StatusUnauthorized, "admin_auth_error", "invalid admin token", "unauthorized")
			return
		}
		next(w, r)
	}
}

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

func (g *Gateway) handleAdminAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed", "")
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	piiOnly := q.Get("pii_only") == "1" || q.Get("pii_only") == "true"

	entries, err := g.store.ListAudit(r.Context(), types.AuditFilter{
		TeamID:  strings.TrimSpace(q.Get("team_id")),
		Status:  types.AuditStatus(strings.TrimSpace(q.Get("status"))),
		Model:   strings.TrimSpace(q.Get("model")),
		PIIOnly: piiOnly,
		Limit:   limit,
	})
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "server_error", err.Error(), "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(entries), "entries": entries})
}

func (g *Gateway) handleAdminSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed", "")
		return
	}
	summary, err := g.store.GetSummary(r.Context())
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "server_error", err.Error(), "")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (g *Gateway) handleAdminProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed", "")
		return
	}
	spent, caps := g.providerCaps.Snapshot(r.Context(), g.store, g.now())
	writeJSON(w, http.StatusOK, map[string]any{
		"period_ym":           store.PeriodYM(g.now()),
		"provider_spend_micros": spent,
		"provider_caps_micros":  caps,
	})
}

func writeStoreLookupError(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	if errors.Is(err, store.ErrTeamNotFound) {
		code = http.StatusNotFound
	}
	writeOpenAIError(w, code, "invalid_request_error", err.Error(), "")
}

