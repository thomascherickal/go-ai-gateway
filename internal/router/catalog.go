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
	var all []ModelSpec
	all = append(all, workersAICatalog()...)
	all = append(all, anthropicGeminiGrokCatalog()...)
	all = append(all, deepSeekQwenOpenAICatalog()...)
	return all
}
