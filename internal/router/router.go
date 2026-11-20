package router

import (
	"errors"
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
