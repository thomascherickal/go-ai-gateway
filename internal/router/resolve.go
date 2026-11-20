package router

import (
	"fmt"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/types"
)

// ResolveWithOptions picks the primary model and fallback chain with strict/fallback controls.
func (r *Router) ResolveWithOptions(req types.ChatCompletionRequest, team types.TeamBudget, opts ResolveOptions) (Decision, error) {
	promptTokens := EstimateMessagesTokens(req.Messages)
	requested := strings.TrimSpace(strings.ToLower(req.Model))
	hint := strings.TrimSpace(strings.ToLower(req.RouteHint))
	providerHint := normalizeProviderHint(req.ProviderHint)

	primary, reason, autoSelected, err := r.resolvePrimary(req, requested, promptTokens, hint, providerHint)
	if err != nil {
		return Decision{}, err
	}

	if team.MaxTier != "" && types.TierRank(primary.Tier) > types.TierRank(team.MaxTier) {
		if autoSelected {
			primary = r.tierDefaultModel(team.MaxTier, providerHint)
			reason = fmt.Sprintf("clamped_to_team_tier(%s)", team.MaxTier)
		} else {
			return Decision{}, fmt.Errorf("%w: model %s (%s) exceeds team max_tier (%s)",
				ErrTierNotAllowed, primary.ID, primary.Tier, team.MaxTier)
		}
	}

	if !r.isAvailable(primary.Provider) {
		if opts.StrictProvider {
			return Decision{}, fmt.Errorf("%w: %s", ErrProviderUnavailable, primary.Provider)
		}
		origProvider := primary.Provider
		fb, ok := r.findAvailableFallback(primary, team.MaxTier)
		if !ok {
			return Decision{}, fmt.Errorf("%w: %s (no available fallback)", ErrProviderUnavailable, origProvider)
		}
		primary = fb
		reason = fmt.Sprintf("provider_unconfigured_fallback(%s->%s)", origProvider, fb.Provider)
	}

	return Decision{
		Primary:      primary,
		Fallbacks:    r.walkChain(primary, team.MaxTier, false),
		Reason:       reason,
		PromptTokens: promptTokens,
	}, nil
}

func (r *Router) resolvePrimary(
	req types.ChatCompletionRequest, requested string, promptTokens int, hint, providerHint string,
) (ModelSpec, string, bool, error) {
	if requested == "" || requested == "auto" {
		spec, reason := r.selectAuto(req.Messages, promptTokens, hint, providerHint)
		return spec, reason, true, nil
	}
	if id, ok := r.aliases[requested]; ok {
		reason := "alias_mapped:" + requested
		if strings.EqualFold(id, req.Model) {
			reason = "explicit_model"
		}
		return r.models[id], reason, false, nil
	}
	if strings.HasPrefix(requested, "ollama/") && r.isAvailable(ProviderOllama) {
		return ModelSpec{
			ID:               strings.TrimPrefix(req.Model, "ollama/"),
			Provider:         ProviderOllama,
			Tier:             types.TierBalanced,
			DefaultMaxTokens: 1024,
			FallbackID:       "@cf/meta/llama-3.1-8b-instruct-fast",
		}, "local_ollama_model", false, nil
	}
	if r.isAvailable(ProviderOpenAICompat) {
		return ModelSpec{
			ID:                req.Model,
			Provider:          ProviderOpenAICompat,
			Tier:              types.TierBalanced,
			InputMicrosPer1M:  500000,
			OutputMicrosPer1M: 1500000,
			DefaultMaxTokens:  1024,
			FallbackID:        "@cf/meta/llama-3.3-70b-instruct-fp8-fast",
		}, "custom_openai_compat_passthrough", false, nil
	}
	return ModelSpec{}, "", false, fmt.Errorf("%w: %s", ErrUnknownModel, req.Model)
}
