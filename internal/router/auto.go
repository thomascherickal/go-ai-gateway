package router

import (
	"strings"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func (r *Router) selectAuto(msgs []types.ChatMessage, promptTokens int, hint, providerHint string) (ModelSpec, string) {
	desiredTier := types.TierFast
	reason := "auto_fast_default"
	codeTask := false

	switch hint {
	case "latency", "cost":
		desiredTier = types.TierFast
		reason = "auto_hint_" + hint
	case "quality":
		desiredTier = types.TierReasoning
		reason = "auto_hint_quality"
	default:
		combined := strings.ToLower(joinMessages(msgs))
		if containsAny(combined, []string{"select ", "join ", "sql", "golang", "typescript", "python", "refactor", "stack trace", "compile error"}) {
			desiredTier = types.TierBalanced
			reason = "auto_code_detected"
			codeTask = true
		} else if promptTokens > 900 || containsAny(combined, []string{"step by step", "step-by-step", "architect", "trade-off", "root cause", "prove"}) {
			desiredTier = types.TierReasoning
			reason = "auto_complex_reasoning"
		} else if promptTokens > 220 || len(msgs) > 2 {
			desiredTier = types.TierBalanced
			reason = "auto_balanced_context"
		}
	}

	if providerHint != "" && r.isAvailable(providerHint) {
		if spec, ok := r.modelForProviderTier(providerHint, desiredTier); ok {
			return spec, reason + "+provider_hint:" + providerHint
		}
	}
	if codeTask {
		return r.models["@cf/qwen/qwen2.5-coder-32b-instruct"], reason
	}
	return r.tierDefaultModel(desiredTier, ""), reason
}

func (r *Router) tierDefaultModel(tier types.ModelTier, providerHint string) ModelSpec {
	if providerHint != "" && r.isAvailable(providerHint) {
		if spec, ok := r.modelForProviderTier(providerHint, tier); ok {
			return spec
		}
	}
	switch tier {
	case types.TierFast:
		return r.models["@cf/meta/llama-3.1-8b-instruct-fast"]
	case types.TierBalanced:
		return r.models["@cf/meta/llama-3.3-70b-instruct-fp8-fast"]
	default:
		return r.models["@cf/deepseek-ai/deepseek-r1-distill-qwen-32b"]
	}
}

func (r *Router) modelForProviderTier(provider string, tier types.ModelTier) (ModelSpec, bool) {
	var best ModelSpec
	found := false
	for _, id := range r.order {
		m := r.models[id]
		if m.Provider != provider {
			continue
		}
		if m.Tier == tier {
			return m, true
		}
		if types.TierRank(m.Tier) <= types.TierRank(tier) && (!found || types.TierRank(m.Tier) > types.TierRank(best.Tier)) {
			best = m
			found = true
		}
	}
	return best, found
}
