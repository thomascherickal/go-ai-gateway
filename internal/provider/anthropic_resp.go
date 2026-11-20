package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func parseAnthropicResponse(respBytes []byte, reqMsgs []types.ChatMessage) (Result, error) {
	var parsed struct {
		Content    []antBlock `json:"content"`
		StopReason string     `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return Result{}, fmt.Errorf("invalid anthropic JSON: %w", err)
	}
	var outText strings.Builder
	var toolCalls []types.ToolCall
	for _, blk := range parsed.Content {
		if blk.Type == "tool_use" {
			args := strings.TrimSpace(string(blk.Input))
			if args == "" {
				args = "{}"
			}
			toolCalls = append(toolCalls, types.ToolCall{
				ID: blk.ID, Type: "function", Function: types.ToolCallFunction{Name: blk.Name, Arguments: args},
			})
		} else {
			outText.WriteString(blk.Text)
		}
	}
	content := outText.String()
	if content == "" && len(toolCalls) == 0 {
		return Result{}, errors.New("anthropic returned empty content")
	}
	finish := "stop"
	if parsed.StopReason == "max_tokens" {
		finish = "length"
	} else if parsed.StopReason == "tool_use" {
		finish = "tool_calls"
	}
	u := types.Usage{PromptTokens: parsed.Usage.InputTokens, CompletionTokens: parsed.Usage.OutputTokens}
	return Result{Content: content, ToolCalls: toolCalls, FinishReason: finish, Usage: ensureUsage(u, reqMsgs, content)}, nil
}
