package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestAnthropicAndGeminiToolCodecs(t *testing.T) {
	tools := []types.ToolDefinition{{
		Type: "function",
		Function: types.ToolFunction{
			Name:        "get_weather",
			Description: "Get weather",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
		},
	}}
	antTools := buildAnthropicTools(tools)
	if len(antTools) != 1 || antTools[0]["name"] != "get_weather" {
		t.Fatalf("unexpected antTools: %#v", antTools)
	}
	for _, tc := range []any{"auto", "none", "required", map[string]any{"type": "function", "function": map[string]any{"name": "get_weather"}}} {
		raw, _ := json.Marshal(tc)
		if mapOpenAIToolChoiceToAnthropic(raw) == nil {
			t.Fatalf("expected non-nil tool choice for %s", string(raw))
		}
	}

	msgs := []types.ChatMessage{
		{Role: "system", Content: "Be helpful"},
		{Role: "user", Content: "Weather in NYC?"},
		{Role: "assistant", ToolCalls: []types.ToolCall{{
			ID: "call_1", Type: "function",
			Function: types.ToolCallFunction{Name: "get_weather", Arguments: `{"city":"NYC"}`},
		}}},
		{Role: "tool", ToolCallID: "call_1", Name: "get_weather", Content: `{"temp":72}`},
	}
	sys, antMsgs := buildAnthropicMessages(msgs)
	if len(sys) != 1 || len(antMsgs) != 3 {
		t.Fatalf("unexpected anthropic messages: sys=%v len=%d", sys, len(antMsgs))
	}

	gemTools := buildGeminiTools(tools)
	if len(gemTools) != 1 {
		t.Fatalf("expected 1 gemini tool declaration")
	}
	gemSys, gemContents := buildGeminiContents(msgs)
	if len(gemSys) != 1 || len(gemContents) != 3 {
		t.Fatalf("unexpected gemini contents: sys=%v len=%d", gemSys, len(gemContents))
	}
}

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWorkersAIRESTProviderChat(t *testing.T) {
	body := `{"success":true,"result":{"response":"hello from workers ai rest","usage":{"prompt_tokens":5,"completion_tokens":4,"total_tokens":9}}}`
	p := &WorkersAIRESTProvider{
		AccountID: "acct1",
		APIToken:  "tok1",
		HTTPClient: &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			}, nil
		})},
	}

	res, err := p.Chat(context.Background(), router.ModelSpec{ID: "@cf/meta/llama-3.1-8b-instruct-fast"}, types.ChatCompletionRequest{
		Model:    "@cf/meta/llama-3.1-8b-instruct-fast",
		Messages: []types.ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if res.Content != "hello from workers ai rest" {
		t.Fatalf("unexpected response: %#v", res)
	}
	if got := truncateString("abcdefghij", 5); got == "abcdefghij" {
		t.Fatalf("expected truncated string, got %q", got)
	}
	mp := &MultiProvider{Providers: map[string]Provider{"anthropic": &MockProvider{}}}
	if mp.Name() == "" {
		t.Fatal("expected non-empty MultiProvider name")
	}
}
