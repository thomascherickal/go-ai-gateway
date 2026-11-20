package gateway

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/thomas/go-ai-gateway/internal/redact"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func (g *Gateway) resolveRoute(
	w http.ResponseWriter, ctx context.Context, reqID string, start time.Time,
	inboundProtocol, toolName, requestedModel string, team types.TeamBudget,
	req types.ChatCompletionRequest, strictProvider bool,
	piiRes redact.MessagesResult, piiAction, promptHash string,
) (router.Decision, bool) {
	decision, err := g.router.ResolveWithOptions(req, team, router.ResolveOptions{StrictProvider: strictProvider})
	if err == nil {
		return decision, true
	}
	statusCode := http.StatusBadRequest
	errCode := "invalid_model"
	if errors.Is(err, router.ErrTierNotAllowed) {
		statusCode = http.StatusForbidden
		errCode = "tier_not_allowed"
	} else if errors.Is(err, router.ErrProviderUnavailable) {
		statusCode = http.StatusServiceUnavailable
		errCode = "provider_unavailable"
	}
	g.recordAuditBestEffort(ctx, types.AuditEntry{
		ID: reqID, CreatedAt: start, TeamID: team.TeamID, ToolName: toolName,
		InboundProtocol: inboundProtocol, RequestedModel: requestedModel,
		Status: types.StatusBlockedPolicy, PIIDetected: piiRes.Total,
		PIIFindings: piiRes.Findings, PIIAction: piiAction,
		PromptSHA256: promptHash, LatencyMs: elapsedMs(g.now(), start), ErrorMessage: err.Error(),
	})
	writeOpenAIError(w, statusCode, "policy_error", err.Error(), errCode)
	return router.Decision{}, false
}

func (g *Gateway) reserveTeamBudget(
	w http.ResponseWriter, ctx context.Context, reqID string, start time.Time,
	inboundProtocol, toolName, requestedModel, teamID string, reserveMicros int64,
	decision router.Decision, piiRes redact.MessagesResult, piiAction, promptHash string,
) bool {
	if _, err := g.store.ReserveBudget(ctx, teamID, reserveMicros, start); err != nil {
		statusCode, errType, errCode, auditStatus := http.StatusTooManyRequests, "insufficient_quota", "budget_exceeded", types.StatusBlockedBudget
		if errors.Is(err, store.ErrTeamDisabled) {
			statusCode, errType, errCode, auditStatus = http.StatusForbidden, "policy_error", "team_disabled", types.StatusBlockedPolicy
		} else if !errors.Is(err, store.ErrBudgetExceeded) {
			statusCode, errType, errCode = http.StatusInternalServerError, "server_error", "budget_store_error"
		}
		g.recordAuditBestEffort(ctx, types.AuditEntry{
			ID: reqID, CreatedAt: start, TeamID: teamID, ToolName: toolName,
			InboundProtocol: inboundProtocol, RequestedModel: requestedModel,
			RoutedModel: decision.Primary.ID, Provider: decision.Primary.Provider,
			RouteReason: decision.Reason, Status: auditStatus, PIIDetected: piiRes.Total,
			PIIFindings: piiRes.Findings, PIIAction: piiAction,
			PromptSHA256: promptHash, LatencyMs: elapsedMs(g.now(), start), ErrorMessage: err.Error(),
		})
		writeOpenAIError(w, statusCode, errType, err.Error(), errCode)
		return false
	}
	return true
}

func maxReserveMicros(candidates []router.ModelSpec, promptTokens, maxTokens int) int64 {
	var best int64
	for _, cand := range candidates {
		if est := router.EstimateReserveMicros(cand, promptTokens, maxTokens); est > best {
			best = est
		}
	}
	return best
}

func (g *Gateway) filterProviderCaps(ctx context.Context, candidates []router.ModelSpec, now time.Time) ([]router.ModelSpec, bool) {
	if g.providerCaps == nil || len(candidates) == 0 {
		return candidates, false
	}
	var allowed []router.ModelSpec
	for _, c := range candidates {
		if !g.providerCaps.IsCapped(ctx, g.store, c.Provider, now) {
			allowed = append(allowed, c)
		}
	}
	bypassed := len(allowed) > 0 && allowed[0].ID != candidates[0].ID
	return allowed, bypassed
}
