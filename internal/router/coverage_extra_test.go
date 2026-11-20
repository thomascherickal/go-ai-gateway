package router

import (
	"testing"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestRouterHelpersAndFallbacks(t *testing.T) {
	r := New(Options{
		IsProviderAvailable: func(p string) bool {
			return p == "workers-ai" || p == "anthropic"
		},
	})

	for _, hint := range []string{"anthropic", "claude", "google", "gemini", "xai", "grok", "openai", "deepseek", "qwen", "mistral", "ollama", "workers-ai", "unknown"} {
		_ = normalizeProviderHint(hint)
	}

	for _, tier := range []types.ModelTier{types.TierFast, types.TierBalanced, types.TierReasoning, types.ModelTier("other")} {
		if m := r.tierDefaultModel(tier, ""); m.ID == "" {
			t.Fatalf("empty default for tier %s", tier)
		}
		_, _ = r.modelForProviderTier("anthropic", tier)
		_, _ = r.modelForProviderTier("workers-ai", tier)
	}

	// Unconfigured provider (openai) should fall back to available workers-ai model.
	dec, err := r.Resolve(types.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []types.ChatMessage{{Role: "user", Content: "hello"}},
	}, types.TeamBudget{MaxTier: types.TierReasoning})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if dec.Primary.Provider != "workers-ai" {
		t.Fatalf("expected fallback to workers-ai, got %s", dec.Primary.Provider)
	}

	if EstimateTextTokens("") != 0 || EstimateTextTokens("hello world") <= 0 {
		t.Fatalf("unexpected EstimateTextTokens")
	}
}
