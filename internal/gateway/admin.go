package gateway

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/features"
	"github.com/thomas/go-ai-gateway/internal/store"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func (g *Gateway) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expected := strings.TrimSpace(g.adminToken())
		auditor := strings.TrimSpace(g.auditorToken())
		if expected == "" && auditor == "" {
			writeOpenAIError(w, http.StatusForbidden, "admin_auth_error", "ADMIN_TOKEN is not configured", "admin_disabled")
			return
		}
		auth := features.RBACAuthorizer{AdminToken: expected, AuditorToken: auditor}
		role := auth.ResolveRole(extractBearerOrHeader(r, "X-Admin-Token"))
		if role == features.RoleNone {
			writeOpenAIError(w, http.StatusUnauthorized, "admin_auth_error", "invalid admin token", "unauthorized")
			return
		}
		if !auth.Allowed(role, r.Method) {
			writeOpenAIError(w, http.StatusForbidden, "admin_auth_error", "read-only auditor token cannot perform write actions", "forbidden")
			return
		}
		next(w, r)
	}
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
		"period_ym":             store.PeriodYM(g.now()),
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
