package router

import (
	"testing"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestRouterMultiVendorRouting(t *testing.T) {
	fullTeam := types.TeamBudget{TeamID: "platform", MaxTier: types.TierReasoning}
	multi := New(Options{
		IsProviderAvailable: func(p string) bool { return true },
	})

	cases := []struct {
		reqModel string
		hint     string
		wantProv string
		wantID   string
	}{
		{"claude-sonnet-4-5", "", ProviderAnthropic, "claude-sonnet-4-5"},
		{"gemini-2.5-flash", "", ProviderGemini, "gemini-2.5-flash"},
		{"grok-3", "", ProviderGrok, "grok-3"},
		{"llama-4-scout", "", ProviderWorkersAI, "@cf/meta/llama-4-scout-17b-16e-instruct"},
		{"deepseek-chat", "", ProviderDeepSeek, "deepseek-chat"},
		{"qwen-plus", "", ProviderQwen, "qwen-plus"},
		{"auto", "anthropic", ProviderAnthropic, "claude-haiku-4-5"},
		{"auto", "gemini", ProviderGemini, "gemini-2.5-flash"},
	}

	for _, tc := range cases {
		dec, err := multi.Resolve(types.ChatCompletionRequest{
			Model:        tc.reqModel,
			ProviderHint: tc.hint,
			Messages:     []types.ChatMessage{{Role: "user", Content: "hello"}},
		}, fullTeam)
		if err != nil {
			t.Fatalf("%s/%s unexpected error: %v", tc.reqModel, tc.hint, err)
		}
		if dec.Primary.Provider != tc.wantProv || dec.Primary.ID != tc.wantID {
			t.Fatalf("%s/%s got (%s, %s), want (%s, %s)",
				tc.reqModel, tc.hint, dec.Primary.Provider, dec.Primary.ID, tc.wantProv, tc.wantID)
		}
	}
}

func TestComputeCostMicros(t *testing.T) {
	spec := ModelSpec{
		InputMicrosPer1M:  500000,
		OutputMicrosPer1M: 1000000,
	}
	if got := ComputeCostMicros(spec, 1000, 2000); got != 2500 {
		t.Fatalf("ComputeCostMicros = %d, want 2500", got)
	}
	if tiny := ComputeCostMicros(spec, 1, 0); tiny != 1 {
		t.Fatalf("tiny ComputeCostMicros = %d, want 1", tiny)
	}
}
