package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestProviderAnthropicGeminiOpenAIWorkersFull(t *testing.T) {
	ctx := context.Background()
	f := 0.5
	req := types.ChatCompletionRequest{
		MaxTokens: 10, Temperature: &f, TopP: &f,
		Tools:      []types.ToolDefinition{{Type: "function", Function: types.ToolFunction{Name: "fn"}}},
		ToolChoice: json.RawMessage(`"auto"`),
		Messages: []types.ChatMessage{
			{Role: "user", Content: "u1"},
			{Role: "user", Content: "u2"},
			{Role: "assistant", Content: "a1", ToolCalls: []types.ToolCall{{ID: "1", Function: types.ToolCallFunction{Name: "fn", Arguments: "bad"}}}},
			{Role: "assistant"},
			{Role: "tool", ToolCallID: "1", Content: "res"},
			{Role: "tool", ToolCallID: "unknown", Content: "res2"},
		},
	}

	// Anthropic Chat with Tools, ToolChoice, Temperature, TopP
	antOK := okClient(`{"content":[{"type":"text","text":"hi"}],"stop_reason":"max_tokens","usage":{"input_tokens":1,"output_tokens":1}}`)
	if _, err := (&AnthropicProvider{APIKey: "k", HTTPClient: antOK}).Chat(ctx, router.ModelSpec{ID: "c"}, req); err != nil {
		t.Fatalf("Anthropic Chat: %v", err)
	}
	_, emptyMsgs := buildAnthropicMessages(nil)
	if len(emptyMsgs) != 1 {
		t.Fatal("expected fallback empty user msg")
	}
	if _, err := parseAnthropicResponse([]byte("{bad"), nil); err == nil {
		t.Fatal("expected invalid json")
	}
	if _, err := parseAnthropicResponse([]byte(`{"content":[]}`), nil); err == nil {
		t.Fatal("expected empty content error")
	}
	if res, err := parseAnthropicResponse([]byte(`{"content":[{"type":"tool_use","id":"1","name":"f"}]}`), nil); err != nil || res.ToolCalls[0].Function.Arguments != "{}" {
		t.Fatalf("empty tool input: %+v %v", res, err)
	}

	// Gemini Chat with MaxTokens, DefaultMaxTokens, Temperature, TopP, Tools
	gemOK := okClient(`{"candidates":[{"finishReason":"MAX_TOKENS","content":{"parts":[{"text":"hi"}]}}]}`)
	if _, err := (&GeminiProvider{APIKey: "k", HTTPClient: gemOK}).Chat(ctx, router.ModelSpec{ID: "g"}, req); err != nil {
		t.Fatalf("Gemini Chat: %v", err)
	}
	reqNoMax := types.ChatCompletionRequest{Messages: nil}
	if _, err := (&GeminiProvider{APIKey: "k", HTTPClient: gemOK}).Chat(ctx, router.ModelSpec{ID: "g", DefaultMaxTokens: 16}, reqNoMax); err != nil {
		t.Fatalf("Gemini DefaultMaxTokens: %v", err)
	}
	for _, bad := range []string{"{bad", `{"candidates":[]}`, `{"candidates":[{"content":{"parts":[]}}]}`} {
		if _, err := parseGeminiResponse([]byte(bad), nil); err == nil {
			t.Fatalf("expected gemini parse error for %s", bad)
		}
	}
	if res, err := parseGeminiResponse([]byte(`{"candidates":[{"content":{"parts":[{"functionCall":{"name":"f"}}]}}],"usageMetadata":{}}`), nil); err != nil || res.ToolCalls[0].Function.Arguments != "{}" {
		t.Fatalf("gemini empty args: %+v %v", res, err)
	}

	// OpenAICompatProvider edge cases
	if _, err := (&OpenAICompatProvider{}).Chat(ctx, router.ModelSpec{}, req); err == nil {
		t.Fatal("expected empty BaseURL error")
	}
	for _, body := range []string{"{bad", `{"choices":[]}`} {
		if _, err := (&OpenAICompatProvider{BaseURL: "http://x", HTTPClient: okClient(body)}).Chat(ctx, router.ModelSpec{}, req); err == nil {
			t.Fatalf("expected error for %s", body)
		}
	}
	for _, body := range []string{`{"choices":[{"message":{"content":"ok"}}]}`, `{"choices":[{"message":{"tool_calls":[{"id":"1"}]}}]}`} {
		if _, err := (&OpenAICompatProvider{BaseURL: "http://x", HTTPClient: okClient(body)}).Chat(ctx, router.ModelSpec{}, req); err != nil {
			t.Fatalf("OpenAICompat ok: %v", err)
		}
	}

	// WorkersAIRESTProvider temperature, network error, bad json, success=false
	failHTTP := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("net") })}
	for _, c := range []*http.Client{failHTTP, okClient("{bad"), okClient(`{"success":false,"errors":[{"message":"bad"}]}`)} {
		if _, err := (&WorkersAIRESTProvider{AccountID: "a", APIToken: "t", HTTPClient: c}).Chat(ctx, router.ModelSpec{ID: "m"}, req); err == nil {
			t.Fatal("expected workers-ai-rest error")
		}
	}
}
