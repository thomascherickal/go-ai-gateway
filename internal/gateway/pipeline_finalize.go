package gateway

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/thomas/go-ai-gateway/internal/provider"
	"github.com/thomas/go-ai-gateway/internal/redact"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

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
		ID: reqID, Object: "chat.completion", Created: start.Unix(), Model: usedSpec.ID,
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

func (g *Gateway) dispatchWithFallback(ctx context.Context, candidates []router.ModelSpec, req types.ChatCompletionRequest) (provider.Result, router.ModelSpec, bool, error) {
	var lastErr error
	for idx, cand := range candidates {
		if res, err := g.provider.Chat(ctx, cand, req); err == nil {
			return res, cand, idx > 0, nil
		} else {
			lastErr = err
		}
	}
	return provider.Result{}, router.ModelSpec{}, false, lastErr
}

func (g *Gateway) recordAuditBestEffort(ctx context.Context, entry types.AuditEntry) {
	_ = g.store.RecordAudit(ctx, entry)
}
