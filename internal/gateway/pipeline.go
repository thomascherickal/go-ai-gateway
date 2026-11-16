package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/thomas/go-ai-gateway/internal/provider"
	"github.com/thomas/go-ai-gateway/internal/redact"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func (g *Gateway) executeChatPipeline(
	w http.ResponseWriter,
	r *http.Request,
	inboundProtocol string,
	apiKey string,
	req types.ChatCompletionRequest,
) (types.ChatCompletionResponse, bool) {
	start := g.now()
	ctx := r.Context()
	reqID := g.newRequestID()
	w.Header().Set("X-Request-ID", reqID)
	w.Header().Set("X-Gateway-Protocol", inboundProtocol)

	toolName := strings.TrimSpace(r.Header.Get("X-Tool-Name"))

	team, ok := g.authenticateTeam(w, ctx, reqID, start, inboundProtocol, toolName, apiKey)
	if !ok {
		return types.ChatCompletionResponse{}, false
	}

	if len(req.Messages) == 0 {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "messages must not be empty", "missing_messages")
		return types.ChatCompletionResponse{}, false
	}
	if toolName == "" {
		toolName = strings.TrimSpace(req.User)
	}
	if req.RouteHint == "" {
		req.RouteHint = strings.TrimSpace(r.Header.Get("X-Gateway-Route"))
	}
	if req.ProviderHint == "" {
		req.ProviderHint = strings.TrimSpace(r.Header.Get("X-Gateway-Provider"))
	}
	requestedModel := req.Model
	if requestedModel == "" {
		requestedModel = "auto"
	}

	piiRes, piiAction, promptHash, ok := g.inspectPII(w, ctx, reqID, start, inboundProtocol, toolName, requestedModel, team, req.Messages)
	if !ok {
		return types.ChatCompletionResponse{}, false
	}
	req.Messages = piiRes.Messages

	strictProvider := strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Gateway-Strict-Provider")), "true")
	decision, ok := g.resolveRoute(w, ctx, reqID, start, inboundProtocol, toolName, requestedModel, team, req, strictProvider, piiRes, piiAction, promptHash)
	if !ok {
		return types.ChatCompletionResponse{}, false
	}

	candidates := append([]router.ModelSpec{decision.Primary}, decision.Fallbacks...)
	candidates, capBypassed := g.filterProviderCaps(ctx, candidates, start)
	if len(candidates) == 0 {
		g.recordAuditBestEffort(ctx, types.AuditEntry{
			ID: reqID, CreatedAt: start, TeamID: team.TeamID, ToolName: toolName,
			InboundProtocol: inboundProtocol, RequestedModel: requestedModel,
			RoutedModel: decision.Primary.ID, Provider: decision.Primary.Provider,
			RouteReason: decision.Reason, Status: types.StatusBlockedBudget,
			PIIDetected: piiRes.Total, PIIFindings: piiRes.Findings, PIIAction: piiAction,
			PromptSHA256: promptHash, LatencyMs: elapsedMs(g.now(), start),
			ErrorMessage: "all candidate providers reached monthly USD cap",
		})
		writeOpenAIError(w, http.StatusTooManyRequests, "insufficient_quota",
			"all candidate providers reached their monthly spend cap", "provider_budget_exceeded")
		return types.ChatCompletionResponse{}, false
	}
	if capBypassed {
		decision.Reason += "+provider_cap_fallback"
	}

	reserveMicros := maxReserveMicros(candidates, decision.PromptTokens, req.MaxTokens)
	if !g.reserveTeamBudget(w, ctx, reqID, start, inboundProtocol, toolName, requestedModel, team.TeamID, reserveMicros, decision, piiRes, piiAction, promptHash) {
		return types.ChatCompletionResponse{}, false
	}

	result, usedSpec, fallbackUsed, err := g.dispatchWithFallback(ctx, candidates, req)
	if err != nil {
		_, _ = g.store.ReconcileBudget(ctx, team.TeamID, -reserveMicros, g.now())
		g.recordAuditBestEffort(ctx, types.AuditEntry{
			ID: reqID, CreatedAt: start, TeamID: team.TeamID, ToolName: toolName,
			InboundProtocol: inboundProtocol, RequestedModel: requestedModel,
			RoutedModel: decision.Primary.ID, Provider: decision.Primary.Provider,
			RouteReason: decision.Reason, FallbackUsed: len(candidates) > 1,
			Status: types.StatusUpstreamError, PIIDetected: piiRes.Total,
			PIIFindings: piiRes.Findings, PIIAction: piiAction,
			PromptSHA256: promptHash, LatencyMs: elapsedMs(g.now(), start), ErrorMessage: err.Error(),
		})
		writeOpenAIError(w, http.StatusBadGateway, "upstream_error", err.Error(), "upstream_failed")
		return types.ChatCompletionResponse{}, false
	}

	return g.finalizeResponse(w, ctx, reqID, start, inboundProtocol, toolName, requestedModel, team.TeamID, reserveMicros, decision, usedSpec, fallbackUsed, result, piiRes, piiAction, promptHash), true
}

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

func (g *Gateway) dispatchWithFallback(
	ctx context.Context, candidates []router.ModelSpec, req types.ChatCompletionRequest,
) (provider.Result, router.ModelSpec, bool, error) {
	var lastErr error
	for idx, cand := range candidates {
		res, err := g.provider.Chat(ctx, cand, req)
		if err == nil {
			return res, cand, idx > 0, nil
		}
		lastErr = err
	}
	return provider.Result{}, router.ModelSpec{}, false, lastErr
}

func (g *Gateway) finalizeResponse(
	w http.ResponseWriter, ctx context.Context, reqID string, start time.Time,
	inboundProtocol, toolName, requestedModel, teamID string, reserveMicros int64,
	decision router.Decision, usedSpec router.ModelSpec, fallbackUsed bool, result provider.Result,
	piiRes redact.MessagesResult, piiAction, promptHash string,
) types.ChatCompletionResponse {
	actualCost := router.ComputeCostMicros(usedSpec, result.Usage.PromptTokens, result.Usage.CompletionTokens)
	updatedTeam, _ := g.store.ReconcileBudget(ctx, teamID, actualCost-reserveMicros, g.now())
	g.providerCaps.RecordSpend(ctx, g.store, usedSpec.Provider, actualCost, start)
	latencyMs := elapsedMs(g.now(), start)

	routeReason := decision.Reason
	if fallbackUsed && !strings.Contains(routeReason, "fallback") {
		routeReason += "+fallback"
	}

	g.recordAuditBestEffort(ctx, types.AuditEntry{
		ID: reqID, CreatedAt: start, TeamID: teamID, ToolName: toolName,
		InboundProtocol: inboundProtocol, RequestedModel: requestedModel,
		RoutedModel: usedSpec.ID, Provider: usedSpec.Provider,
		RouteReason: routeReason, FallbackUsed: fallbackUsed,
		Status: types.StatusOK, PIIDetected: piiRes.Total,
		PIIFindings: piiRes.Findings, PIIAction: piiAction,
		PromptTokens: result.Usage.PromptTokens, CompletionTokens: result.Usage.CompletionTokens,
		TotalTokens: result.Usage.TotalTokens, CostMicros: actualCost,
		LatencyMs: latencyMs, PromptSHA256: promptHash,
	})

	w.Header().Set("X-Gateway-Routed-Model", usedSpec.ID)
	w.Header().Set("X-Gateway-Provider", usedSpec.Provider)
	w.Header().Set("X-Gateway-Cost-Micros", strconv.FormatInt(actualCost, 10))
	w.Header().Set("X-Gateway-Remaining-Micros", strconv.FormatInt(updatedTeam.RemainingMicros, 10))

	return types.ChatCompletionResponse{
		ID:      reqID,
		Object:  "chat.completion",
		Created: start.Unix(),
		Model:   usedSpec.ID,
		Choices: []types.ChatChoice{
			{Index: 0, Message: types.ChatMessage{Role: "assistant", Content: result.Content, ToolCalls: result.ToolCalls}, FinishReason: result.FinishReason},
		},
		Usage: result.Usage,
		Gateway: &types.GatewayMeta{
			RequestID: reqID, TeamID: teamID, ToolName: toolName,
			InboundProtocol: inboundProtocol, RequestedModel: requestedModel,
			RoutedModel: usedSpec.ID, Provider: usedSpec.Provider,
			RouteReason: routeReason, FallbackUsed: fallbackUsed,
			PIIAction: piiAction, PIIDetected: piiRes.Total, PIIFindings: piiRes.Findings,
			CostMicros: actualCost, RemainingMicros: updatedTeam.RemainingMicros, LatencyMs: latencyMs,
		},
	}
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

func (g *Gateway) recordAuditBestEffort(ctx context.Context, entry types.AuditEntry) {
	_ = g.store.RecordAudit(ctx, entry)
}

