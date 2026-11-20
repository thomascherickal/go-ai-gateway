//go:build js && wasm

package provider

import (
	"context"
	"errors"
	"fmt"
	"syscall/js"

	"github.com/syumai/workers-go/cloudflare"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

// WorkersAIJSProvider invokes Cloudflare Workers AI via the runtime env.AI binding.
type WorkersAIJSProvider struct {
	BindingName string
}

func (p *WorkersAIJSProvider) Name() string { return "workers-ai-binding" }

func (p *WorkersAIJSProvider) Chat(_ context.Context, model router.ModelSpec, req types.ChatCompletionRequest) (Result, error) {
	bindingName := p.BindingName
	if bindingName == "" {
		bindingName = "AI"
	}
	aiBinding := cloudflare.GetBinding(bindingName)
	if aiBinding.IsUndefined() || aiBinding.IsNull() {
		return Result{}, fmt.Errorf("cloudflare %s binding not found", bindingName)
	}

	objClass := js.Global().Get("Object")
	arrClass := js.Global().Get("Array")

	jsMessages := arrClass.New(len(req.Messages))
	for i, m := range req.Messages {
		msgObj := objClass.New()
		msgObj.Set("role", m.Role)
		msgObj.Set("content", m.Content)
		jsMessages.SetIndex(i, msgObj)
	}

	inputObj := objClass.New()
	inputObj.Set("messages", jsMessages)
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = model.DefaultMaxTokens
	}
	inputObj.Set("max_tokens", maxTokens)
	if req.Temperature != nil {
		inputObj.Set("temperature", *req.Temperature)
	}
	if req.TopP != nil {
		inputObj.Set("top_p", *req.TopP)
	}

	var promise js.Value
	if gwID := cloudflare.Getenv("AI_GATEWAY_ID"); gwID != "" {
		gwInner := objClass.New()
		gwInner.Set("id", gwID)
		optsObj := objClass.New()
		optsObj.Set("gateway", gwInner)
		promise = aiBinding.Call("run", model.ID, inputObj, optsObj)
	} else {
		promise = aiBinding.Call("run", model.ID, inputObj)
	}

	res, err := awaitPromise(promise)
	if err != nil {
		return Result{}, err
	}
	if res.IsUndefined() || res.IsNull() {
		return Result{}, errors.New("workers AI returned empty response")
	}

	content := extractAIContent(res)
	if content == "" {
		return Result{}, errors.New("workers AI returned empty message content")
	}

	return Result{
		Content:      content,
		FinishReason: "stop",
		Usage:        extractAIUsage(res, req.Messages, content),
	}, nil
}
