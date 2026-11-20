package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func parseGeminiResponse(respBytes []byte, reqMsgs []types.ChatMessage) (Result, error) {
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []gemPart `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return Result{}, fmt.Errorf("invalid gemini JSON: %w", err)
	}
	if len(parsed.Candidates) == 0 {
		return Result{}, errors.New("gemini returned no candidates")
	}
	var outText strings.Builder
	var toolCalls []types.ToolCall
	for idx, pt := range parsed.Candidates[0].Content.Parts {
		if pt.Text != "" {
			outText.WriteString(pt.Text)
		}
		if pt.FunctionCall != nil {
			argsBytes, _ := json.Marshal(pt.FunctionCall.Args)
			if len(argsBytes) == 0 || string(argsBytes) == "null" {
				argsBytes = []byte("{}")
			}
			toolCalls = append(toolCalls, types.ToolCall{
				ID: fmt.Sprintf("call_gem_%d", idx+1), Type: "function",
				Function: types.ToolCallFunction{Name: pt.FunctionCall.Name, Arguments: string(argsBytes)},
			})
		}
	}
	content := outText.String()
	if content == "" && len(toolCalls) == 0 {
		return Result{}, errors.New("gemini returned empty candidate text")
	}
	finish := "stop"
	if len(toolCalls) > 0 {
		finish = "tool_calls"
	} else if strings.EqualFold(parsed.Candidates[0].FinishReason, "MAX_TOKENS") {
		finish = "length"
	}
	u := types.Usage{
		PromptTokens:     parsed.UsageMetadata.PromptTokenCount,
		CompletionTokens: parsed.UsageMetadata.CandidatesTokenCount,
		TotalTokens:      parsed.UsageMetadata.TotalTokenCount,
	}
	return Result{Content: content, ToolCalls: toolCalls, FinishReason: finish, Usage: ensureUsage(u, reqMsgs, content)}, nil
}
