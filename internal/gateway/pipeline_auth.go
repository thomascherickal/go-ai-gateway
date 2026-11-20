package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/thomas/go-ai-gateway/internal/redact"
	"github.com/thomas/go-ai-gateway/internal/store"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func (g *Gateway) authenticateTeam(
	w http.ResponseWriter, ctx context.Context, reqID string, start time.Time,
	inboundProtocol, toolName, apiKey string,
) (types.TeamBudget, bool) {
	team, err := g.store.LookupTeamByKey(ctx, apiKey)
	if err != nil {
		code := http.StatusUnauthorized
		msg := "invalid or missing team API key"
		if !errors.Is(err, store.ErrTeamNotFound) {
			code = http.StatusInternalServerError
			msg = "failed to verify team credentials"
		}
		g.recordAuditBestEffort(ctx, types.AuditEntry{
			ID: reqID, CreatedAt: start, TeamID: "unknown", ToolName: toolName,
			InboundProtocol: inboundProtocol, Status: types.StatusUnauthorized,
			PIIAction: "none", LatencyMs: elapsedMs(g.now(), start), ErrorMessage: err.Error(),
		})
		writeOpenAIError(w, code, "authentication_error", msg, "invalid_api_key")
		return types.TeamBudget{}, false
	}
	w.Header().Set("X-Gateway-Team", team.TeamID)

	if !team.Enabled {
		g.recordAuditBestEffort(ctx, types.AuditEntry{
			ID: reqID, CreatedAt: start, TeamID: team.TeamID, ToolName: toolName,
			InboundProtocol: inboundProtocol, Status: types.StatusBlockedPolicy,
			PIIAction: "none", LatencyMs: elapsedMs(g.now(), start), ErrorMessage: "team is disabled",
		})
		writeOpenAIError(w, http.StatusForbidden, "policy_error", "team access is currently disabled", "team_disabled")
		return types.TeamBudget{}, false
	}
	return team, true
}

func (g *Gateway) inspectPII(
	w http.ResponseWriter, ctx context.Context, reqID string, start time.Time,
	inboundProtocol, toolName, requestedModel string, team types.TeamBudget, msgs []types.ChatMessage,
) (redact.MessagesResult, string, string, bool) {
	shouldMutate := team.PIIMode == "" || team.PIIMode == types.PIIModeRedact
	piiRes := redact.Messages(msgs, shouldMutate)
	promptHash := hashMessages(piiRes.Messages)

	piiAction := "none"
	if piiRes.Total > 0 {
		switch team.PIIMode {
		case types.PIIModeBlock:
			piiAction = "blocked"
		case types.PIIModeAuditOnly:
			piiAction = "audited"
		default:
			piiAction = "redacted"
		}
	}
	w.Header().Set("X-Gateway-PII-Action", piiAction)
	w.Header().Set("X-Gateway-PII-Detected", strconv.Itoa(piiRes.Total))

	if team.PIIMode == types.PIIModeBlock && piiRes.Total > 0 {
		g.recordAuditBestEffort(ctx, types.AuditEntry{
			ID: reqID, CreatedAt: start, TeamID: team.TeamID, ToolName: toolName,
			InboundProtocol: inboundProtocol, RequestedModel: requestedModel,
			Status: types.StatusBlockedPII, PIIDetected: piiRes.Total,
			PIIFindings: piiRes.Findings, PIIAction: piiAction,
			PromptSHA256: promptHash, LatencyMs: elapsedMs(g.now(), start),
			ErrorMessage: "personal data or secret detected under block policy",
		})
		writeOpenAIError(w, http.StatusBadRequest, "pii_policy_violation",
			fmt.Sprintf("request blocked: detected %d personal-data/secret item(s)", piiRes.Total), "pii_detected")
		return piiRes, piiAction, promptHash, false
	}
	return piiRes, piiAction, promptHash, true
}
