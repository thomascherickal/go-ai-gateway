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
		if dec.Primary.ID != "@cf/meta/llama-3.1-8b-instruct-fast" || len(dec.Fallbacks) == 0 {
			t.Fatalf("unexpected decision: %+v", dec)
		}
	})

	t.Run("auto code prompt routes to coder", func(t *testing.T) {
		dec, err := r.Resolve(types.ChatCompletionRequest{
			Model:    "auto",
			Messages: []types.ChatMessage{{Role: "user", Content: "Refactor this Golang HTTP handler"}},
		}, fullTeam)
		if err != nil || dec.Primary.ID != "@cf/qwen/qwen2.5-coder-32b-instruct" {
			t.Fatalf("got (%s, %v), want qwen2.5-coder-32b-instruct", dec.Primary.ID, err)
		}
	})

	t.Run("auto clamps down to team max_tier", func(t *testing.T) {
		dec, err := r.Resolve(types.ChatCompletionRequest{
			Model:    "auto",
			Messages: []types.ChatMessage{{Role: "user", Content: "Explain root cause step by step"}},
		}, fastOnlyTeam)
		if err != nil || dec.Primary.Tier != types.TierFast {
			t.Fatalf("expected clamped fast tier, got (%s, %v)", dec.Primary.Tier, err)
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
		if err != nil || dec.Primary.Provider != ProviderWorkersAI {
			t.Fatalf("expected workers-ai fallback, got (%s, %v)", dec.Primary.Provider, err)
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
