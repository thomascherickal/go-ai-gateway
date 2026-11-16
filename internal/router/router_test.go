package router

import (
	"errors"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestRouterResolve(t *testing.T) {
	r := New(Options{})
	fullTeam := types.TeamBudget{TeamID: "platform", MaxTier: types.TierReasoning}
	fastOnlyTeam := types.TeamBudget{TeamID: "sandbox", MaxTier: types.TierFast}

	t.Run("auto short prompt uses fast model", func(t *testing.T) {
		dec, err := r.Resolve(types.ChatCompletionRequest{
			Model:    "auto",
			Messages: []types.ChatMessage{{Role: "user", Content: "Classify this ticket: login failed"}},
		}, fullTeam)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dec.Primary.ID != "@cf/meta/llama-3.1-8b-instruct-fast" {
			t.Fatalf("got %s, want llama-3.1-8b-instruct-fast", dec.Primary.ID)
		}
		if len(dec.Fallbacks) == 0 {
			t.Fatalf("expected at least one fallback model")
		}
	})

	t.Run("auto code prompt routes to coder", func(t *testing.T) {
		dec, err := r.Resolve(types.ChatCompletionRequest{
			Model:    "auto",
			Messages: []types.ChatMessage{{Role: "user", Content: "Refactor this Golang HTTP handler"}},
		}, fullTeam)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dec.Primary.ID != "@cf/qwen/qwen2.5-coder-32b-instruct" {
			t.Fatalf("got %s, want qwen2.5-coder-32b-instruct", dec.Primary.ID)
		}
	})

	t.Run("auto clamps down to team max_tier", func(t *testing.T) {
		dec, err := r.Resolve(types.ChatCompletionRequest{
			Model:    "auto",
			Messages: []types.ChatMessage{{Role: "user", Content: "Explain root cause step by step"}},
		}, fastOnlyTeam)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dec.Primary.Tier != types.TierFast {
			t.Fatalf("expected clamped fast tier, got %s (%s)", dec.Primary.ID, dec.Primary.Tier)
		}
	})

	t.Run("explicit higher-tier model rejected for restricted team", func(t *testing.T) {
		_, err := r.Resolve(types.ChatCompletionRequest{
			Model:    "deepseek-r1",
			Messages: []types.ChatMessage{{Role: "user", Content: "Hi"}},
		}, fastOnlyTeam)
		if !errors.Is(err, ErrTierNotAllowed) {
			t.Fatalf("got err %v, want ErrTierNotAllowed", err)
		}
	})

	t.Run("unconfigured external model falls back to Workers AI by default", func(t *testing.T) {
		dec, err := r.Resolve(types.ChatCompletionRequest{
			Model:    "claude-sonnet-4-5",
			Messages: []types.ChatMessage{{Role: "user", Content: "Hi"}},
		}, fullTeam)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dec.Primary.Provider != ProviderWorkersAI {
			t.Fatalf("expected workers-ai fallback, got %s (%s)", dec.Primary.Provider, dec.Primary.ID)
		}
	})

	t.Run("strict provider fails when external provider unconfigured", func(t *testing.T) {
		_, err := r.ResolveWithOptions(types.ChatCompletionRequest{
			Model:    "gemini-2.5-flash",
			Messages: []types.ChatMessage{{Role: "user", Content: "Hi"}},
		}, fullTeam, ResolveOptions{StrictProvider: true})
		if !errors.Is(err, ErrProviderUnavailable) {
			t.Fatalf("got err %v, want ErrProviderUnavailable", err)
		}
	})

	t.Run("multi-vendor routing when providers are configured", func(t *testing.T) {
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
	})

	t.Run("unknown model returns ErrUnknownModel", func(t *testing.T) {
		_, err := r.Resolve(types.ChatCompletionRequest{
			Model:    "nonexistent-model",
			Messages: []types.ChatMessage{{Role: "user", Content: "Hi"}},
		}, fullTeam)
		if !errors.Is(err, ErrUnknownModel) {
			t.Fatalf("got err %v, want ErrUnknownModel", err)
		}
	})
}

func TestComputeCostMicros(t *testing.T) {
	spec := ModelSpec{
		InputMicrosPer1M:  500000,  // $0.50 / 1M
		OutputMicrosPer1M: 1000000, // $1.00 / 1M
	}
	got := ComputeCostMicros(spec, 1000, 2000)
	if got != 2500 {
		t.Fatalf("ComputeCostMicros = %d, want 2500", got)
	}
	tiny := ComputeCostMicros(spec, 1, 0)
	if tiny != 1 {
		t.Fatalf("tiny ComputeCostMicros = %d, want 1", tiny)
	}
}
