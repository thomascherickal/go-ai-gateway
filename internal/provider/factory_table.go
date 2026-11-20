package provider

import "github.com/thomas/go-ai-gateway/internal/router"

type openAIVendorConfig struct {
	label          string
	keyEnv         []string
	baseEnv        string
	defaultBaseURL string
}

var openAIVendorTable = map[string]openAIVendorConfig{
	router.ProviderGrok: {
		label:          "xai-grok",
		keyEnv:         []string{"XAI_API_KEY"},
		baseEnv:        "XAI_BASE_URL",
		defaultBaseURL: "https://api.x.ai/v1",
	},
	router.ProviderDeepSeek: {
		label:          "deepseek",
		keyEnv:         []string{"DEEPSEEK_API_KEY"},
		baseEnv:        "DEEPSEEK_BASE_URL",
		defaultBaseURL: "https://api.deepseek.com/v1",
	},
	router.ProviderQwen: {
		label:          "qwen-dashscope",
		keyEnv:         []string{"QWEN_API_KEY", "DASHSCOPE_API_KEY"},
		baseEnv:        "QWEN_BASE_URL",
		defaultBaseURL: "https://dashscope-intl.aliyuncs.com/compatible-mode/v1",
	},
	router.ProviderOpenAI: {
		label:          "openai",
		keyEnv:         []string{"OPENAI_API_KEY"},
		baseEnv:        "OPENAI_BASE_URL",
		defaultBaseURL: "https://api.openai.com/v1",
	},
	router.ProviderMistral: {
		label:          "mistral",
		keyEnv:         []string{"MISTRAL_API_KEY"},
		baseEnv:        "MISTRAL_BASE_URL",
		defaultBaseURL: "https://api.mistral.ai/v1",
	},
}

// ExternalVendors lists all non-Workers-AI provider identifiers supported by the factory.
var ExternalVendors = []string{
	router.ProviderAnthropic,
	router.ProviderGemini,
	router.ProviderGrok,
	router.ProviderDeepSeek,
	router.ProviderQwen,
	router.ProviderOpenAI,
	router.ProviderMistral,
	router.ProviderOllama,
	router.ProviderOpenAICompat,
}

// AllVendors lists every provider identifier (Workers AI + external vendors).
var AllVendors = append([]string{router.ProviderWorkersAI}, ExternalVendors...)
