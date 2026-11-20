package router

import "github.com/thomas/go-ai-gateway/internal/types"

// latestOpenCodeCatalogSpecs registers the latest flagship models from OpenCode's models.dev registry.
func latestOpenCodeCatalogSpecs() []ModelSpec {
	return []ModelSpec{
		{ID: "claude-opus-5-5", Aliases: []string{"claude-opus-5", "claude-opus-4-8"}, Provider: ProviderAnthropic, Tier: types.TierReasoning, InputMicrosPer1M: 15000000, OutputMicrosPer1M: 75000000, DefaultMaxTokens: 2048, FallbackID: "claude-sonnet-5-5", Description: "Anthropic Claude Opus 5.5 (latest flagship)"},
		{ID: "claude-sonnet-5-5", Aliases: []string{"claude-sonnet-4-6"}, Provider: ProviderAnthropic, Tier: types.TierBalanced, InputMicrosPer1M: 3000000, OutputMicrosPer1M: 15000000, DefaultMaxTokens: 1536, FallbackID: "@cf/meta/llama-3.3-70b-instruct-fp8-fast", Description: "Anthropic Claude Sonnet 5.5"},
		{ID: "claude-haiku-5-5", Provider: ProviderAnthropic, Tier: types.TierFast, InputMicrosPer1M: 800000, OutputMicrosPer1M: 4000000, DefaultMaxTokens: 1024, FallbackID: "@cf/meta/llama-3.1-8b-instruct-fast", Description: "Anthropic Claude Haiku 5.5"},
		{ID: "gemini-4-carbon", Provider: ProviderGemini, Tier: types.TierReasoning, InputMicrosPer1M: 2500000, OutputMicrosPer1M: 12000000, DefaultMaxTokens: 2048, FallbackID: "gemini-3.8-flash", Description: "Google Gemini 4 Carbon"},
		{ID: "gemini-3.8-flash", Aliases: []string{"gemini-3.6-flash", "gemini-3.5-flash-lite"}, Provider: ProviderGemini, Tier: types.TierFast, InputMicrosPer1M: 150000, OutputMicrosPer1M: 600000, DefaultMaxTokens: 1024, FallbackID: "@cf/meta/llama-3.1-8b-instruct-fast", Description: "Google Gemini 3.8 Flash"},
		{ID: "gemma-4-31b-it", Provider: ProviderGemini, Tier: types.TierBalanced, InputMicrosPer1M: 200000, OutputMicrosPer1M: 800000, DefaultMaxTokens: 1024, FallbackID: "@cf/meta/llama-3.3-70b-instruct-fp8-fast", Description: "Google Gemma 4 31B IT"},
		{ID: "gpt-6.2-astra", Aliases: []string{"gpt-5.2", "gpt-5-pro"}, Provider: ProviderOpenAI, Tier: types.TierReasoning, InputMicrosPer1M: 5000000, OutputMicrosPer1M: 20000000, DefaultMaxTokens: 2048, FallbackID: "gpt-5", Description: "OpenAI GPT-6.2 Astra"},
		{ID: "gpt-5.3-codex-spark", Aliases: []string{"gpt-5", "gpt-4.1"}, Provider: ProviderOpenAI, Tier: types.TierBalanced, InputMicrosPer1M: 2000000, OutputMicrosPer1M: 8000000, DefaultMaxTokens: 1536, FallbackID: "@cf/qwen/qwen2.5-coder-32b-instruct", Description: "OpenAI GPT-5.3 Codex Spark"},
		{ID: "o3-pro", Aliases: []string{"o3"}, Provider: ProviderOpenAI, Tier: types.TierReasoning, InputMicrosPer1M: 10000000, OutputMicrosPer1M: 40000000, DefaultMaxTokens: 2048, FallbackID: "o3-mini", Description: "OpenAI o3 Pro"},
		{ID: "grok-4.7", Aliases: []string{"grok-4.6", "grok-4.5", "grok-4.3"}, Provider: ProviderGrok, Tier: types.TierReasoning, InputMicrosPer1M: 3000000, OutputMicrosPer1M: 15000000, DefaultMaxTokens: 1536, FallbackID: "grok-3", Description: "xAI Grok 4.7"},
		{ID: "deepseek-v4-pro", Aliases: []string{"deepseek-v4-flash", "deepseek-flash"}, Provider: ProviderDeepSeek, Tier: types.TierReasoning, InputMicrosPer1M: 550000, OutputMicrosPer1M: 2190000, DefaultMaxTokens: 2048, FallbackID: "deepseek-chat", Description: "DeepSeek V4 Pro"},
		{ID: "qwen3.5-plus", Aliases: []string{"qwen3.5-27b", "qwen3-coder-480b-a35b-instruct", "qwen3-coder-flash"}, Provider: ProviderQwen, Tier: types.TierBalanced, InputMicrosPer1M: 400000, OutputMicrosPer1M: 1200000, DefaultMaxTokens: 1536, FallbackID: "qwen-plus", Description: "Alibaba Qwen 3.5 Plus"},
		{ID: "mistral-large-4", Aliases: []string{"mistral-medium-latest", "mistral-small-2603"}, Provider: ProviderMistral, Tier: types.TierBalanced, InputMicrosPer1M: 2000000, OutputMicrosPer1M: 6000000, DefaultMaxTokens: 1536, FallbackID: "mistral-large-latest", Description: "Mistral Large 4"},
		{ID: "@cf/qwen/qwen3.8-27b", Aliases: []string{"qwen3.8-27b", "@cf/qwen/qwen3-30b-a3b-fp8"}, Provider: ProviderWorkersAI, Tier: types.TierBalanced, InputMicrosPer1M: 350000, OutputMicrosPer1M: 900000, DefaultMaxTokens: 1024, FallbackID: "@cf/meta/llama-3.3-70b-instruct-fp8-fast", Description: "Workers AI Qwen 3.8 27B"},
		{ID: "@cf/openai/gpt-oss-120b", Aliases: []string{"gpt-oss-120b", "@cf/openai/gpt-oss-20b"}, Provider: ProviderWorkersAI, Tier: types.TierReasoning, InputMicrosPer1M: 400000, OutputMicrosPer1M: 1200000, DefaultMaxTokens: 1536, FallbackID: "@cf/meta/llama-3.3-70b-instruct-fp8-fast", Description: "Workers AI OpenAI GPT-OSS 120B"},
	}
}
