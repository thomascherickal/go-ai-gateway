package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

type badBodyReader struct{}

func (badBodyReader) Read([]byte) (int, error) { return 0, errors.New("read err") }
func (badBodyReader) Close() error             { return nil }

func TestGatewayCodecsSSEAndUtilsFull(t *testing.T) {
	// convertAnthropicMessage: null, invalid JSON, tool_use (empty input), tool_result (non-string), multi-text
	_ = convertAnthropicMessage(anthropicMessageIn{Role: "user", Content: json.RawMessage(`null`)})
	_ = convertAnthropicMessage(anthropicMessageIn{Role: "user", Content: json.RawMessage(`123`)})
	_ = convertAnthropicMessage(anthropicMessageIn{Role: "assistant", Content: json.RawMessage(`[
		{"type":"text","text":"a"},{"type":"text","text":"b"},
		{"type":"tool_use","id":"1","name":"fn"},
		{"type":"tool_result","tool_use_id":"1","content":{"k":"v"}}
	]`)})
	_ = extractAnthropicSystemText(json.RawMessage(`123`))

	// anthropicStopReason + buildAnthropicResponse (length + tool_calls with bad args)
	if anthropicStopReason("length", false) != "max_tokens" {
		t.Fatal("expected max_tokens")
	}
	_ = buildAnthropicResponse(types.ChatCompletionResponse{
		Choices: []types.ChatChoice{{
			FinishReason: "tool_calls",
			Message: types.ChatMessage{
				ToolCalls: []types.ToolCall{{ID: "1", Function: types.ToolCallFunction{Name: "f", Arguments: "{bad"}}},
			},
		}},
	})

	// convertGeminiContent (multi-text, functionCall with nil args, functionResponse) + buildGeminiResponse (length + toolCalls)
	_ = convertGeminiContent(0, "model", []geminiPartIn{
		{Text: "a"}, {Text: "b"},
		{FunctionCall: &geminiFunctionCallIn{Name: "fn"}},
		{FunctionResponse: &geminiFunctionResponseIn{Name: "fn", Response: json.RawMessage(`{"ok":true}`)}},
	})
	_ = buildGeminiResponse(types.ChatCompletionResponse{
		Choices: []types.ChatChoice{{
			FinishReason: "length",
			Message: types.ChatMessage{
				ToolCalls: []types.ToolCall{{ID: "1", Function: types.ToolCallFunction{Name: "f", Arguments: `{"a":1}`}}},
			},
		}},
	})

	// writeOpenAISSE (tool_calls + empty finish reason) + writeAnthropicSSE (invalid tool args) + sse helpers
	rec := httptest.NewRecorder()
	writeOpenAISSE(rec, types.ChatCompletionResponse{
		Choices: []types.ChatChoice{{
			Message: types.ChatMessage{
				Content:   "hi",
				ToolCalls: []types.ToolCall{{ID: "1", Type: "function", Function: types.ToolCallFunction{Name: "f", Arguments: "{}"}}},
			},
		}},
	})
	writeAnthropicSSE(httptest.NewRecorder(), types.ChatCompletionResponse{
		Choices: []types.ChatChoice{{
			Message: types.ChatMessage{
				ToolCalls: []types.ToolCall{{ID: "1", Function: types.ToolCallFunction{Name: "f", Arguments: ""}}},
			},
		}},
	})
	writeSSEData(httptest.NewRecorder(), "", make(chan int))
	if splitSSEChunks("", 5) != nil || len(splitSSEChunks("abc", 0)) != 1 {
		t.Fatal("unexpected splitSSEChunks")
	}

	// http_util + provider_caps_parse
	reqBad := httptest.NewRequest(http.MethodPost, "/", badBodyReader{})
	if _, ok := readLimitedBody(httptest.NewRecorder(), reqBad); ok {
		t.Fatal("expected readLimitedBody failure")
	}
	if extractBearerOrHeader(httptest.NewRequest(http.MethodGet, "/", nil), "") != "" {
		t.Fatal("expected empty token")
	}
	now := time.Now()
	if elapsedMs(now.Add(-time.Second), now) != 0 {
		t.Fatal("expected 0 for negative elapsedMs")
	}
	caps := ParseProviderLimitsUSD(" ,bad,noeq,=-1,anthropic=5")
	if caps["anthropic"] != 5_000_000 {
		t.Fatalf("unexpected caps: %v", caps)
	}
}
