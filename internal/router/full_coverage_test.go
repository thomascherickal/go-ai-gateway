package router

import (
	"testing"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestRouterFullBranchCoverage(t *testing.T) {
	r := New(Options{
		IsProviderAvailable: func(p string) bool {
			return p == ProviderWorkersAI || p == ProviderAnthropic
		},
	})
	team := types.TeamBudget{MaxTier: types.TierReasoning}

	// Route hints: cost, quality, and balanced context (>2 messages)
	for _, tc := range []types.ChatCompletionRequest{
		{Model: "auto", RouteHint: "cost", Messages: []types.ChatMessage{{Role: "user", Content: "hi"}}},
		{Model: "auto", RouteHint: "quality", Messages: []types.ChatMessage{{Role: "user", Content: "hi"}}},
		{Model: "auto", Messages: []types.ChatMessage{{Role: "user", Content: "a"}, {Role: "assistant", Content: "b"}, {Role: "user", Content: "c"}}},
	} {
		if _, err := r.Resolve(tc, team); err != nil {
			t.Fatalf("Resolve %+v: %v", tc.RouteHint, err)
		}
	}

	// tierDefaultModel with provider hint
	if m := r.tierDefaultModel(types.TierFast, ProviderAnthropic); m.Provider != ProviderAnthropic {
		t.Fatalf("expected anthropic fast model, got %+v", m)
	}

	// walkChain cycle and missing fallback ID
	r.models["cycA"] = ModelSpec{ID: "cycA", Provider: ProviderWorkersAI, Tier: types.TierFast, FallbackID: "cycB"}
	r.models["cycB"] = ModelSpec{ID: "cycB", Provider: ProviderWorkersAI, Tier: types.TierFast, FallbackID: "cycA"}
	if got := r.walkChain(r.models["cycA"], types.TierFast, false); len(got) != 1 {
		t.Fatalf("expected 1 in cycle, got %d", len(got))
	}
	r.models["dangling"] = ModelSpec{ID: "dangling", Provider: ProviderWorkersAI, Tier: types.TierFast, FallbackID: "nope"}
	if got := r.walkChain(r.models["dangling"], types.TierFast, false); len(got) != 0 {
		t.Fatalf("expected 0 for dangling fallback, got %d", len(got))
	}

	// ResolveWithOptions when no provider is available at all
	rNone := New(Options{IsProviderAvailable: func(string) bool { return false }})
	if _, err := rNone.ResolveWithOptions(types.ChatCompletionRequest{
		Model: "gpt-4o", Messages: []types.ChatMessage{{Role: "user", Content: "hi"}},
	}, team, ResolveOptions{}); err == nil {
		t.Fatal("expected error when no fallback provider available")
	}
}
