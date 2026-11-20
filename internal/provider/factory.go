package provider

import (
	"net/http"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/router"
)

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
		return &OpenAICompatProvider{Label: "ollama-local", BaseURL: base, HTTPClient: httpClient}
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
		return &OpenAICompatProvider{Label: cfg.label, BaseURL: base, APIKey: key, HTTPClient: httpClient}
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
