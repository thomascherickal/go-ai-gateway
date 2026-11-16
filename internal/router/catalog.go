package router

import "github.com/thomas/go-ai-gateway/internal/types"

const (
	ProviderWorkersAI    = "workers-ai"
	ProviderAnthropic    = "anthropic"
	ProviderGemini       = "gemini"
	ProviderGrok         = "grok"
	ProviderDeepSeek     = "deepseek"
	ProviderQwen         = "qwen"
	ProviderOpenAI       = "openai"
	ProviderMistral      = "mistral"
	ProviderOllama       = "ollama"
	ProviderOpenAICompat = "openai-compat"
)

// ModelSpec defines a routable model, its upstream provider, tier, pricing, and fallback.
type ModelSpec struct {
	ID                string          `json:"id"`
	Aliases           []string        `json:"aliases,omitempty"`
	Provider          string          `json:"provider"`
	Tier              types.ModelTier `json:"tier"`
	InputMicrosPer1M  int64           `json:"input_micros_per_1m"`
	OutputMicrosPer1M int64           `json:"output_micros_per_1m"`
	DefaultMaxTokens  int             `json:"default_max_tokens"`
	FallbackID        string          `json:"fallback_id,omitempty"`
	Description       string          `json:"description"`
}

func defaultCatalog() []ModelSpec {
	return []ModelSpec{
		// --- Cloudflare Workers AI (Meta, Qwen, DeepSeek, Mistral) ---
		{
			ID:                "@cf/meta/llama-3.1-8b-instruct-fast",
			Aliases:           []string{"fast", "llama-8b", "llama-3.1-8b", "meta/llama-3.1-8b"},
			Provider:          ProviderWorkersAI,
			Tier:              types.TierFast,
			InputMicrosPer1M:  45000,
			OutputMicrosPer1M: 384000,
			DefaultMaxTokens:  512,
			FallbackID:        "@cf/mistral/mistral-7b-instruct-v0.2",
			Description:       "Meta Llama 3.1 8B Fast on Workers AI — ultra-low latency & cost",
		},
		{
			ID:                "@cf/mistral/mistral-7b-instruct-v0.2",
			Aliases:           []string{"mistral-7b", "workers/mistral-7b"},
			Provider:          ProviderWorkersAI,
			Tier:              types.TierFast,
			InputMicrosPer1M:  110000,
			OutputMicrosPer1M: 190000,
			DefaultMaxTokens:  512,
			Description:       "Mistral 7B v0.2 on Workers AI — compact fallback model",
		},
		{
			ID:                "@cf/meta/llama-3.3-70b-instruct-fp8-fast",
			Aliases:           []string{"balanced", "llama-70b", "llama-3.3-70b", "meta/llama-3.3-70b"},
			Provider:          ProviderWorkersAI,
			Tier:              types.TierBalanced,
			InputMicrosPer1M:  293000,
			OutputMicrosPer1M: 2253000,
			DefaultMaxTokens:  1024,
			FallbackID:        "@cf/meta/llama-3.1-8b-instruct-fast",
			Description:       "Meta Llama 3.3 70B FP8 Fast on Workers AI — general-purpose reasoning & synthesis",
		},
		{
			ID:                "@cf/meta/llama-4-scout-17b-16e-instruct",
			Aliases:           []string{"llama-4", "llama-4-scout", "meta/llama-4-scout"},
			Provider:          ProviderWorkersAI,
			Tier:              types.TierBalanced,
			InputMicrosPer1M:  270000,
			OutputMicrosPer1M: 850000,
			DefaultMaxTokens:  1024,
			FallbackID:        "@cf/meta/llama-3.3-70b-instruct-fp8-fast",
			Description:       "Meta Llama 4 Scout 17B MoE on Workers AI",
		},
		{
			ID:                "@cf/qwen/qwen2.5-coder-32b-instruct",
			Aliases:           []string{"coder", "qwen-coder", "qwen-2.5-coder"},
			Provider:          ProviderWorkersAI,
			Tier:              types.TierBalanced,
			InputMicrosPer1M:  660000,
			OutputMicrosPer1M: 1000000,
			DefaultMaxTokens:  1536,
			FallbackID:        "@cf/meta/llama-3.3-70b-instruct-fp8-fast",
			Description:       "Qwen 2.5 Coder 32B on Workers AI — code generation, SQL, and debugging",
		},
		{
			ID:                "@cf/deepseek-ai/deepseek-r1-distill-qwen-32b",
			Aliases:           []string{"reasoning", "deepseek-r1", "r1"},
			Provider:          ProviderWorkersAI,
			Tier:              types.TierReasoning,
			InputMicrosPer1M:  497000,
			OutputMicrosPer1M: 4881000,
			DefaultMaxTokens:  2048,
			FallbackID:        "@cf/meta/llama-3.3-70b-instruct-fp8-fast",
			Description:       "DeepSeek R1 Distill Qwen 32B on Workers AI — multi-step reasoning",
		},

		// --- Anthropic ---
		{
			ID:                "claude-haiku-4-5",
			Aliases:           []string{"claude-haiku", "claude-3-5-haiku", "claude-3-5-haiku-latest", "anthropic/claude-haiku-4-5"},
			Provider:          ProviderAnthropic,
			Tier:              types.TierFast,
			InputMicrosPer1M:  800000,
			OutputMicrosPer1M: 4000000,
			DefaultMaxTokens:  1024,
			FallbackID:        "@cf/meta/llama-3.1-8b-instruct-fast",
			Description:       "Anthropic Claude Haiku 4.5 (with Workers AI fallback)",
		},
		{
			ID:                "claude-sonnet-4-5",
			Aliases:           []string{"claude-sonnet", "claude-3-7-sonnet", "claude-3-5-sonnet", "claude-sonnet-4-0", "anthropic/claude-sonnet-4-5"},
			Provider:          ProviderAnthropic,
			Tier:              types.TierBalanced,
			InputMicrosPer1M:  3000000,
			OutputMicrosPer1M: 15000000,
			DefaultMaxTokens:  2048,
			FallbackID:        "@cf/meta/llama-3.3-70b-instruct-fp8-fast",
			Description:       "Anthropic Claude Sonnet 4.5 (with Workers AI fallback)",
		},
		{
			ID:                "claude-opus-4-1",
			Aliases:           []string{"claude-opus", "anthropic/claude-opus-4-1"},
			Provider:          ProviderAnthropic,
			Tier:              types.TierReasoning,
			InputMicrosPer1M:  15000000,
			OutputMicrosPer1M: 75000000,
			DefaultMaxTokens:  2048,
			FallbackID:        "@cf/deepseek-ai/deepseek-r1-distill-qwen-32b",
			Description:       "Anthropic Claude Opus 4.1 (with DeepSeek R1 Workers AI fallback)",
		},

		// --- Google Gemini ---
		{
			ID:                "gemini-2.5-flash",
			Aliases:           []string{"gemini-flash", "gemini-2.0-flash", "google/gemini-2.5-flash", "models/gemini-2.5-flash"},
			Provider:          ProviderGemini,
			Tier:              types.TierFast,
			InputMicrosPer1M:  150000,
			OutputMicrosPer1M: 600000,
			DefaultMaxTokens:  1024,
			FallbackID:        "@cf/meta/llama-3.1-8b-instruct-fast",
			Description:       "Google Gemini 2.5 Flash (with Workers AI fallback)",
		},
		{
			ID:                "gemini-2.5-pro",
			Aliases:           []string{"gemini-pro", "google/gemini-2.5-pro", "models/gemini-2.5-pro"},
			Provider:          ProviderGemini,
			Tier:              types.TierReasoning,
			InputMicrosPer1M:  1250000,
			OutputMicrosPer1M: 10000000,
			DefaultMaxTokens:  2048,
			FallbackID:        "@cf/deepseek-ai/deepseek-r1-distill-qwen-32b",
			Description:       "Google Gemini 2.5 Pro (with DeepSeek R1 Workers AI fallback)",
		},

		// --- xAI Grok ---
		{
			ID:                "grok-3-mini",
			Aliases:           []string{"grok-mini", "xai/grok-3-mini"},
			Provider:          ProviderGrok,
			Tier:              types.TierFast,
			InputMicrosPer1M:  300000,
			OutputMicrosPer1M: 500000,
			DefaultMaxTokens:  1024,
			FallbackID:        "@cf/meta/llama-3.1-8b-instruct-fast",
			Description:       "xAI Grok 3 Mini (with Workers AI fallback)",
		},
		{
			ID:                "grok-3",
			Aliases:           []string{"grok", "grok-2", "xai/grok-3"},
			Provider:          ProviderGrok,
			Tier:              types.TierReasoning,
			InputMicrosPer1M:  3000000,
			OutputMicrosPer1M: 15000000,
			DefaultMaxTokens:  2048,
			FallbackID:        "@cf/meta/llama-3.3-70b-instruct-fp8-fast",
			Description:       "xAI Grok 3 (with Llama 3.3 70B Workers AI fallback)",
		},

		// --- DeepSeek API ---
		{
			ID:                "deepseek-chat",
			Aliases:           []string{"deepseek", "deepseek-v3", "deepseek/deepseek-chat"},
			Provider:          ProviderDeepSeek,
			Tier:              types.TierBalanced,
			InputMicrosPer1M:  270000,
			OutputMicrosPer1M: 1100000,
			DefaultMaxTokens:  1536,
			FallbackID:        "@cf/meta/llama-3.3-70b-instruct-fp8-fast",
			Description:       "DeepSeek V3 Chat API (with Workers AI fallback)",
		},
		{
			ID:                "deepseek-reasoner",
			Aliases:           []string{"deepseek/deepseek-reasoner", "deepseek-r1-api"},
			Provider:          ProviderDeepSeek,
			Tier:              types.TierReasoning,
			InputMicrosPer1M:  550000,
			OutputMicrosPer1M: 2190000,
			DefaultMaxTokens:  2048,
			FallbackID:        "@cf/deepseek-ai/deepseek-r1-distill-qwen-32b",
			Description:       "DeepSeek R1 Reasoner API (with Workers AI R1 Distill fallback)",
		},

		// --- Qwen (Alibaba DashScope) ---
		{
			ID:                "qwen-turbo",
			Aliases:           []string{"alibaba/qwen-turbo"},
			Provider:          ProviderQwen,
			Tier:              types.TierFast,
			InputMicrosPer1M:  50000,
			OutputMicrosPer1M: 200000,
			DefaultMaxTokens:  1024,
			FallbackID:        "@cf/meta/llama-3.1-8b-instruct-fast",
			Description:       "Alibaba Qwen Turbo (with Workers AI fallback)",
		},
		{
			ID:                "qwen-plus",
			Aliases:           []string{"qwen", "alibaba/qwen-plus"},
			Provider:          ProviderQwen,
			Tier:              types.TierBalanced,
			InputMicrosPer1M:  400000,
			OutputMicrosPer1M: 1200000,
			DefaultMaxTokens:  1536,
			FallbackID:        "@cf/qwen/qwen2.5-coder-32b-instruct",
			Description:       "Alibaba Qwen Plus (with Workers AI Qwen 2.5 fallback)",
		},
		{
			ID:                "qwen-max",
			Aliases:           []string{"alibaba/qwen-max"},
			Provider:          ProviderQwen,
			Tier:              types.TierReasoning,
			InputMicrosPer1M:  1600000,
			OutputMicrosPer1M: 6400000,
			DefaultMaxTokens:  2048,
			FallbackID:        "@cf/deepseek-ai/deepseek-r1-distill-qwen-32b",
			Description:       "Alibaba Qwen Max (with Workers AI fallback)",
		},

		// --- OpenAI ---
		{
			ID:                "gpt-4o-mini",
			Aliases:           []string{"openai/gpt-4o-mini", "gpt-3.5-turbo"},
			Provider:          ProviderOpenAI,
			Tier:              types.TierFast,
			InputMicrosPer1M:  150000,
			OutputMicrosPer1M: 600000,
			DefaultMaxTokens:  1024,
			FallbackID:        "@cf/meta/llama-3.1-8b-instruct-fast",
			Description:       "OpenAI GPT-4o Mini (with Workers AI fallback)",
		},
		{
			ID:                "gpt-4o",
			Aliases:           []string{"openai/gpt-4o"},
			Provider:          ProviderOpenAI,
			Tier:              types.TierBalanced,
			InputMicrosPer1M:  2500000,
			OutputMicrosPer1M: 10000000,
			DefaultMaxTokens:  1536,
			FallbackID:        "@cf/meta/llama-3.3-70b-instruct-fp8-fast",
			Description:       "OpenAI GPT-4o (with Workers AI fallback)",
		},
		{
			ID:                "o3-mini",
			Aliases:           []string{"o1", "openai/o3-mini"},
			Provider:          ProviderOpenAI,
			Tier:              types.TierReasoning,
			InputMicrosPer1M:  1100000,
			OutputMicrosPer1M: 4400000,
			DefaultMaxTokens:  2048,
			FallbackID:        "@cf/deepseek-ai/deepseek-r1-distill-qwen-32b",
			Description:       "OpenAI o3-mini (with DeepSeek R1 Workers AI fallback)",
		},

		// --- Mistral API ---
		{
			ID:                "mistral-large-latest",
			Aliases:           []string{"mistral-large", "mistral/mistral-large-latest"},
			Provider:          ProviderMistral,
			Tier:              types.TierBalanced,
			InputMicrosPer1M:  2000000,
			OutputMicrosPer1M: 6000000,
			DefaultMaxTokens:  1536,
			FallbackID:        "@cf/meta/llama-3.3-70b-instruct-fp8-fast",
			Description:       "Mistral Large (with Workers AI fallback)",
		},

		// --- Local Ollama ($0/token overflow or private inference) ---
		{
			ID:                "llama3.2",
			Aliases:           []string{"ollama/llama3.2", "local-fast"},
			Provider:          ProviderOllama,
			Tier:              types.TierFast,
			InputMicrosPer1M:  0,
			OutputMicrosPer1M: 0,
			DefaultMaxTokens:  1024,
			FallbackID:        "@cf/meta/llama-3.1-8b-instruct-fast",
			Description:       "Local Ollama Llama 3.2 ($0/token local inference)",
		},
	}
}

