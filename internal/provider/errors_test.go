package provider

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestProviderErrorAndToolResponsePaths(t *testing.T) {
	errClient := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 500,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"error":"upstream fail"}`)),
		}, nil
	})}
	req := types.ChatCompletionRequest{Messages: []types.ChatMessage{{Role: "user", Content: "hi"}}}

	// Missing API key errors
	if _, err := (&AnthropicProvider{}).Chat(context.Background(), router.ModelSpec{ID: "claude"}, req); err == nil {
		t.Fatal("expected error for missing anthropic key")
	}
	if _, err := (&GeminiProvider{}).Chat(context.Background(), router.ModelSpec{ID: "gemini"}, req); err == nil {
		t.Fatal("expected error for missing gemini key")
	}
	if _, err := (&WorkersAIRESTProvider{}).Chat(context.Background(), router.ModelSpec{ID: "llama"}, req); err == nil {
		t.Fatal("expected error for missing workers ai rest creds")
	}

	// Upstream 500 errors
	if _, err := (&AnthropicProvider{APIKey: "k", HTTPClient: errClient}).Chat(context.Background(), router.ModelSpec{ID: "claude"}, req); err == nil {
		t.Fatal("expected anthropic 500 error")
	}
	if _, err := (&GeminiProvider{APIKey: "k", HTTPClient: errClient}).Chat(context.Background(), router.ModelSpec{ID: "gemini"}, req); err == nil {
		t.Fatal("expected gemini 500 error")
	}
	if _, err := (&OpenAICompatProvider{BaseURL: "http://example.invalid", APIKey: "k", HTTPClient: errClient}).Chat(context.Background(), router.ModelSpec{ID: "gpt-4o"}, req); err == nil {
		t.Fatal("expected openai compat 500 error")
	}

	// Tool call parsing in Anthropic and Gemini responses
	antRaw := []byte(`{"content":[{"type":"text","text":"ok"},{"type":"tool_use","id":"t1","name":"fn","input":{"a":1}}],"stop_reason":"tool_use","usage":{"input_tokens":3,"output_tokens":4}}`)
	if res, err := parseAnthropicResponse(antRaw, req.Messages); err != nil || len(res.ToolCalls) != 1 {
		t.Fatalf("parseAnthropicResponse: %+v err=%v", res, err)
	}
	gemRaw := []byte(`{"candidates":[{"finishReason":"STOP","content":{"parts":[{"text":"ok"},{"functionCall":{"name":"fn","args":{"a":1}}}]}}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":4}}`)
	if res, err := parseGeminiResponse(gemRaw, req.Messages); err != nil || len(res.ToolCalls) != 1 {
		t.Fatalf("parseGeminiResponse: %+v err=%v", res, err)
	}

	// MultiProvider DynamicLookup & HasProvider
	mp := &MultiProvider{
		HasFunc: func(p string) bool { return p == "dyn" },
		DynamicLookup: func(p string) Provider {
			if p == "dyn" {
				return &MockProvider{}
			}
			return nil
		},
	}
	if !mp.HasProvider("dyn") || mp.HasProvider("missing") {
		t.Fatal("unexpected HasProvider result")
	}
	if _, err := mp.Chat(context.Background(), router.ModelSpec{Provider: "dyn", ID: "m"}, req); err != nil {
		t.Fatalf("MultiProvider DynamicLookup Chat: %v", err)
	}
	if _, err := mp.Chat(context.Background(), router.ModelSpec{Provider: "missing", ID: "m"}, req); err == nil {
		t.Fatal("expected error for missing provider")
	}
}
