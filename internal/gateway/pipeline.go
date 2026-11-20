package gateway

import (
	"net/http"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func (g *Gateway) executeChatPipeline(
	w http.ResponseWriter, r *http.Request, inboundProtocol, apiKey string, req types.ChatCompletionRequest,
) (types.ChatCompletionResponse, bool) {
	start, ctx, reqID := g.now(), r.Context(), g.newRequestID()
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

	candidates, capBypassed := g.filterProviderCaps(ctx, append([]router.ModelSpec{decision.Primary}, decision.Fallbacks...), start)
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
		writeOpenAIError(w, http.StatusTooManyRequests, "insufficient_quota", "all candidate providers reached their monthly spend cap", "provider_budget_exceeded")
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
