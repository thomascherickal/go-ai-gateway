package router

import (
	"testing"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestRouterCatalogOllamaAndReserve(t *testing.T) {
	r := New(Options{IsProviderAvailable: func(string) bool { return true }})
	if len(r.Catalog()) < 15 {
		t.Fatalf("expected rich catalog, got %d", len(r.Catalog()))
	}
	team := types.TeamBudget{TeamID: "p", MaxTier: types.TierReasoning}
	dec, err := r.Resolve(types.ChatCompletionRequest{
		Model: "ollama/qwen2.5:7b", Messages: []types.ChatMessage{{Role: "user", Content: "hi"}},
	}, team)
	if err != nil || dec.Primary.Provider != ProviderOllama || dec.Primary.ID != "qwen2.5:7b" {
		t.Fatalf("unexpected ollama decision: %+v, err=%v", dec, err)
	}
	dec2, err := r.Resolve(types.ChatCompletionRequest{
		Model: "custom/my-model", Messages: []types.ChatMessage{{Role: "user", Content: "hi"}},
	}, team)
	if err != nil || dec2.Primary.Provider != ProviderOpenAICompat {
		t.Fatalf("unexpected compat passthrough: %+v, err=%v", dec2, err)
	}
	if est := EstimateReserveMicros(dec.Primary, 100, 0); est != 0 {
		t.Fatalf("ollama reserve = %d, want 0", est)
	}
	if tok := EstimateTextTokens(""); tok != 0 {
		t.Fatalf("empty text tokens = %d, want 0", tok)
	}
}

func BenchmarkRouterResolve(b *testing.B) {
	r := New(Options{})
	team := types.TeamBudget{TeamID: "p", MaxTier: types.TierReasoning}
	req := types.ChatCompletionRequest{Model: "auto", Messages: []types.ChatMessage{{Role: "user", Content: "Refactor Golang SQL query"}}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.Resolve(req, team)
	}
}
