package provider

import (
	"context"
	"fmt"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

// MockProvider simulates deterministic responses for local development and tests.
type MockProvider struct {
	Handler func(ctx context.Context, model router.ModelSpec, req types.ChatCompletionRequest) (Result, error)
}

func (p *MockProvider) Name() string { return "mock-local" }

func (p *MockProvider) Chat(ctx context.Context, model router.ModelSpec, req types.ChatCompletionRequest) (Result, error) {
	if p.Handler != nil {
		return p.Handler(ctx, model, req)
	}
	lastUser := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastUser = req.Messages[i].Content
			break
		}
	}
	reply := fmt.Sprintf("[local-mock %s] Processed redacted input (%d chars): %s",
		model.ID, len(lastUser), truncateString(lastUser, 120))
	return Result{
		Content:      reply,
		FinishReason: "stop",
		Usage:        ensureUsage(types.Usage{}, req.Messages, reply),
	}, nil
}
