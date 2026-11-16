package provider

import (
	"net/http"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/router"
)

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

// IsVendorConfigured reports whether the required credentials/URL env vars exist for a vendor.
func IsVendorConfigured(name string, getenv func(string) string) bool {
	switch name {
	case router.ProviderAnthropic:
		return firstEnvValue(getenv, "ANTHROPIC_API_KEY") != ""
	case router.ProviderGemini:
		return firstEnvValue(getenv, "GEMINI_API_KEY") != ""
	case router.ProviderOllama:
		return firstEnvValue(getenv, "OLLAMA_BASE_URL") != ""
	case router.ProviderOpenAICompat:
		return firstEnvValue(getenv, "UPSTREAM_BASE_URL") != ""
	default:
		cfg, ok := openAIVendorTable[name]
		return ok && firstEnvValue(getenv, cfg.keyEnv...) != ""
	}
}

// BuildVendorProvider constructs an upstream Provider from environment variables, or returns nil if unconfigured.
func BuildVendorProvider(name string, getenv func(string) string, httpClient *http.Client) Provider {
	switch name {
	case router.ProviderAnthropic:
		key := firstEnvValue(getenv, "ANTHROPIC_API_KEY")
		if key == "" {
			return nil
		}
		return &AnthropicProvider{
			BaseURL:    firstEnvValue(getenv, "ANTHROPIC_BASE_URL"),
			APIKey:     key,
			HTTPClient: httpClient,
		}
	case router.ProviderGemini:
		key := firstEnvValue(getenv, "GEMINI_API_KEY")
		if key == "" {
			return nil
		}
		return &GeminiProvider{
			BaseURL:    firstEnvValue(getenv, "GEMINI_BASE_URL"),
			APIKey:     key,
			HTTPClient: httpClient,
		}
	case router.ProviderOllama:
		base := firstEnvValue(getenv, "OLLAMA_BASE_URL")
		if base == "" {
			return nil
		}
		return &OpenAICompatProvider{
			Label:      "ollama-local",
			BaseURL:    base,
			HTTPClient: httpClient,
		}
	case router.ProviderOpenAICompat:
		base := firstEnvValue(getenv, "UPSTREAM_BASE_URL")
		if base == "" {
			return nil
		}
		return &OpenAICompatProvider{
			Label:      "openai-compat",
			BaseURL:    base,
			APIKey:     firstEnvValue(getenv, "UPSTREAM_API_KEY"),
			HTTPClient: httpClient,
		}
	default:
		cfg, ok := openAIVendorTable[name]
		if !ok {
			return nil
		}
		key := firstEnvValue(getenv, cfg.keyEnv...)
		if key == "" {
			return nil
		}
		base := firstEnvValue(getenv, cfg.baseEnv)
		if base == "" {
			base = cfg.defaultBaseURL
		}
		return &OpenAICompatProvider{
			Label:      cfg.label,
			BaseURL:    base,
			APIKey:     key,
			HTTPClient: httpClient,
		}
	}
}

func firstEnvValue(getenv func(string) string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

