package provider

import (
	"context"
	"fmt"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

// Result contains the model output, optional tool calls, and token accounting from an upstream provider.
type Result struct {
	Content      string
	ToolCalls    []types.ToolCall
	FinishReason string
	Usage        types.Usage
}

// Provider executes chat completion inference for a resolved ModelSpec.
type Provider interface {
	Name() string
	Chat(ctx context.Context, model router.ModelSpec, req types.ChatCompletionRequest) (Result, error)
}

// MultiProvider routes calls to the appropriate backend provider based on model.Provider.
type MultiProvider struct {
	WorkersAI     Provider
	OpenAICompat  Provider
	Providers     map[string]Provider
	DynamicLookup func(providerName string) Provider
	HasFunc       func(providerName string) bool
}

func (m *MultiProvider) Name() string {
	if m.WorkersAI != nil {
		return m.WorkersAI.Name()
	}
	if len(m.Providers) > 0 {
		return "multi-provider"
	}
	if m.OpenAICompat != nil {
		return m.OpenAICompat.Name()
	}
	return "none"
}

// HasProvider reports whether the named upstream provider is configured and callable.
func (m *MultiProvider) HasProvider(name string) bool {
	if m.HasFunc != nil {
		return m.HasFunc(name)
	}
	return m.resolve(name) != nil
}

func (m *MultiProvider) resolve(name string) Provider {
	if m.Providers != nil {
		if p, ok := m.Providers[name]; ok && p != nil {
			return p
		}
	}
	switch name {
	case router.ProviderWorkersAI:
		if m.WorkersAI != nil {
			return m.WorkersAI
		}
	case router.ProviderOpenAICompat:
		if m.OpenAICompat != nil {
			return m.OpenAICompat
		}
	}
	if m.DynamicLookup != nil {
		return m.DynamicLookup(name)
	}
	return nil
}

func (m *MultiProvider) Chat(ctx context.Context, model router.ModelSpec, req types.ChatCompletionRequest) (Result, error) {
	p := m.resolve(model.Provider)
	if p == nil {
		return Result{}, fmt.Errorf("provider %q is not configured", model.Provider)
	}
	return p.Chat(ctx, model, req)
}
