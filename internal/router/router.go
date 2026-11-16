package router

import (
	"errors"
	"fmt"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/types"
)

var (
	ErrUnknownModel        = errors.New("unknown or unsupported model")
	ErrTierNotAllowed      = errors.New("requested model tier exceeds team policy")
	ErrProviderUnavailable = errors.New("requested model provider is not configured")
)

// Decision describes the resolved primary model, fallback chain, and selection reason.
type Decision struct {
	Primary      ModelSpec
	Fallbacks    []ModelSpec
	Reason       string
	PromptTokens int
}

// ResolveOptions configures per-request routing behavior.
type ResolveOptions struct {
	StrictProvider bool
}

// Router resolves incoming model requests against catalog, provider availability, heuristics, and team policies.
type Router struct {
	models      map[string]ModelSpec
	aliases     map[string]string
	order       []string
	isAvailable func(provider string) bool
}

// Options configures router construction.
type Options struct {
	EnableExternalUpstream bool
	IsProviderAvailable    func(provider string) bool
}

// New creates a Router preloaded with the multi-vendor model catalog.
func New(opts Options) *Router {
	avail := opts.IsProviderAvailable
	if avail == nil {
		avail = func(provider string) bool {
			return provider == ProviderWorkersAI || opts.EnableExternalUpstream
		}
	}

	catalog := defaultCatalog()
	r := &Router{
		models:      make(map[string]ModelSpec, len(catalog)),
		aliases:     make(map[string]string),
		order:       make([]string, 0, len(catalog)),
		isAvailable: avail,
	}

	for _, m := range catalog {
		r.models[m.ID] = m
		r.order = append(r.order, m.ID)
		r.aliases[strings.ToLower(m.ID)] = m.ID
		for _, a := range m.Aliases {
			r.aliases[strings.ToLower(a)] = m.ID
		}
	}
	return r
}

// Catalog returns all configured models in display order.
func (r *Router) Catalog() []ModelSpec {
	out := make([]ModelSpec, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.models[id])
	}
	return out
}

// Resolve picks the primary model and fallback chain for a request and team policy.
func (r *Router) Resolve(req types.ChatCompletionRequest, team types.TeamBudget) (Decision, error) {
	return r.ResolveWithOptions(req, team, ResolveOptions{})
}

// ResolveWithOptions picks the primary model and fallback chain with strict/fallback controls.
func (r *Router) ResolveWithOptions(req types.ChatCompletionRequest, team types.TeamBudget, opts ResolveOptions) (Decision, error) {
	promptTokens := EstimateMessagesTokens(req.Messages)
	requested := strings.TrimSpace(strings.ToLower(req.Model))
	hint := strings.TrimSpace(strings.ToLower(req.RouteHint))
	providerHint := normalizeProviderHint(req.ProviderHint)

	var primary ModelSpec
	var reason string
	autoSelected := false

	switch {
	case requested == "" || requested == "auto":
		autoSelected = true
		primary, reason = r.selectAuto(req.Messages, promptTokens, hint, providerHint)
	default:
		if id, ok := r.aliases[requested]; ok {
			primary = r.models[id]
			if strings.EqualFold(id, req.Model) {
				reason = "explicit_model"
			} else {
				reason = "alias_mapped:" + requested
			}
		} else if strings.HasPrefix(requested, "ollama/") && r.isAvailable(ProviderOllama) {
			primary = ModelSpec{
				ID:                strings.TrimPrefix(req.Model, "ollama/"),
				Provider:          ProviderOllama,
				Tier:              types.TierBalanced,
				InputMicrosPer1M:  0,
				OutputMicrosPer1M: 0,
				DefaultMaxTokens:  1024,
				FallbackID:        "@cf/meta/llama-3.1-8b-instruct-fast",
			}
			reason = "local_ollama_model"
		} else if r.isAvailable(ProviderOpenAICompat) {
			primary = ModelSpec{
				ID:                req.Model,
				Provider:          ProviderOpenAICompat,
				Tier:              types.TierBalanced,
				InputMicrosPer1M:  500000,
				OutputMicrosPer1M: 1500000,
				DefaultMaxTokens:  1024,
				FallbackID:        "@cf/meta/llama-3.3-70b-instruct-fp8-fast",
			}
			reason = "custom_openai_compat_passthrough"
		} else {
			return Decision{}, fmt.Errorf("%w: %s", ErrUnknownModel, req.Model)
		}
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

func (r *Router) findAvailableFallback(primary ModelSpec, maxTier types.ModelTier) (ModelSpec, bool) {
	if chain := r.walkChain(primary, maxTier, true); len(chain) > 0 {
		return chain[0], true
	}
	if r.isAvailable(ProviderWorkersAI) {
		tier := primary.Tier
		if maxTier != "" && types.TierRank(tier) > types.TierRank(maxTier) {
			tier = maxTier
		}
		return r.tierDefaultModel(tier, ""), true
	}
	return ModelSpec{}, false
}

func (r *Router) walkChain(primary ModelSpec, maxTier types.ModelTier, firstOnly bool) []ModelSpec {
	var chain []ModelSpec
	seen := map[string]bool{primary.ID: true}
	cur := primary
	for cur.FallbackID != "" {
		if seen[cur.FallbackID] {
			break
		}
		seen[cur.FallbackID] = true
		fb, ok := r.models[cur.FallbackID]
		if !ok {
			break
		}
		if r.isAvailable(fb.Provider) && (maxTier == "" || types.TierRank(fb.Tier) <= types.TierRank(maxTier)) {
			chain = append(chain, fb)
			if firstOnly {
				return chain
			}
		}
		cur = fb
	}
	return chain
}

func normalizeProviderHint(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "anthropic", "claude":
		return ProviderAnthropic
	case "gemini", "google":
		return ProviderGemini
	case "grok", "xai":
		return ProviderGrok
	case "deepseek":
		return ProviderDeepSeek
	case "qwen", "alibaba", "dashscope":
		return ProviderQwen
	case "openai":
		return ProviderOpenAI
	case "mistral":
		return ProviderMistral
	case "ollama", "local":
		return ProviderOllama
	case "meta", "workers-ai", "workersai", "cf":
		return ProviderWorkersAI
	default:
		return ""
	}
}

func joinMessages(msgs []types.ChatMessage) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(m.Content)
		b.WriteByte(' ')
	}
	return b.String()
}

func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}
