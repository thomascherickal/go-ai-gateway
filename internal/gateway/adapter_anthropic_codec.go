package gateway

import (
	"encoding/json"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func anthropicToCanonical(in anthropicRequestIn) types.ChatCompletionRequest {
	var msgs []types.ChatMessage
	if sys := extractAnthropicSystemText(in.System); sys != "" {
		msgs = append(msgs, types.ChatMessage{Role: "system", Content: sys})
	}
	for _, m := range in.Messages {
		msgs = append(msgs, convertAnthropicMessage(m)...)
	}

	var tools []types.ToolDefinition
	for _, t := range in.Tools {
		tools = append(tools, types.ToolDefinition{
			Type:     "function",
			Function: types.ToolFunction{Name: t.Name, Description: t.Description, Parameters: t.InputSchema},
		})
	}

	return types.ChatCompletionRequest{
		Model: in.Model, Messages: msgs, Tools: tools, ToolChoice: in.ToolChoice,
		MaxTokens: in.MaxTokens, Temperature: in.Temperature, TopP: in.TopP,
		Stream: in.Stream, User: in.Metadata.UserID,
	}
}

func extractAnthropicSystemText(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var b strings.Builder
		for _, blk := range blocks {
			if (blk.Type == "text" || blk.Type == "") && blk.Text != "" {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(blk.Text)
			}
		}
		return b.String()
	}
	return ""
}
