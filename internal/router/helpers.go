package router

import (
	"strings"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func normalizeProviderHint(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "anthropic", "claude":
		return ProviderAnthropic
	case "gemini", "google":
		return ProviderGemini
	case "grok", "xai":
		return ProviderGrok
	case "deepseek":
		return ProviderDeepSeek
	case "qwen", "alibaba", "dashscope":
		return ProviderQwen
	case "openai":
		return ProviderOpenAI
	case "mistral":
		return ProviderMistral
	case "ollama", "local":
		return ProviderOllama
	case "meta", "workers-ai", "workersai", "cf":
		return ProviderWorkersAI
	default:
		return ""
	}
}

func joinMessages(msgs []types.ChatMessage) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(m.Content)
		b.WriteByte(' ')
	}
	return b.String()
}

func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}
