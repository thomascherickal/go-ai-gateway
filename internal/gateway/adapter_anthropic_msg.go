package gateway

import (
	"encoding/json"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func convertAnthropicMessage(m anthropicMessageIn) []types.ChatMessage {
	trimmed := strings.TrimSpace(string(m.Content))
	if trimmed == "" || trimmed == "null" {
		return []types.ChatMessage{{Role: m.Role, Content: ""}}
	}
	var asString string
	if err := json.Unmarshal(m.Content, &asString); err == nil {
		return []types.ChatMessage{{Role: m.Role, Content: asString}}
	}

	var blocks []struct {
		Type      string          `json:"type"`
		Text      string          `json:"text"`
		ID        string          `json:"id"`
		Name      string          `json:"name"`
		Input     json.RawMessage `json:"input"`
		ToolUseID string          `json:"tool_use_id"`
		Content   json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(m.Content, &blocks); err != nil {
		return []types.ChatMessage{{Role: m.Role, Content: ""}}
	}

	var textBuf strings.Builder
	var toolCalls []types.ToolCall
	var toolResults []types.ChatMessage
	for _, blk := range blocks {
		switch blk.Type {
		case "tool_use":
			args := strings.TrimSpace(string(blk.Input))
			if args == "" {
				args = "{}"
			}
			toolCalls = append(toolCalls, types.ToolCall{
				ID: blk.ID, Type: "function", Function: types.ToolCallFunction{Name: blk.Name, Arguments: args},
			})
		case "tool_result":
			resText := extractAnthropicSystemText(blk.Content)
			if resText == "" && len(blk.Content) > 0 {
				resText = string(blk.Content)
			}
			toolResults = append(toolResults, types.ChatMessage{Role: "tool", ToolCallID: blk.ToolUseID, Content: resText})
		default:
			if blk.Text != "" {
				if textBuf.Len() > 0 {
					textBuf.WriteByte('\n')
				}
				textBuf.WriteString(blk.Text)
			}
		}
	}

	var out []types.ChatMessage
	if textBuf.Len() > 0 || len(toolCalls) > 0 || len(toolResults) == 0 {
		out = append(out, types.ChatMessage{Role: m.Role, Content: textBuf.String(), ToolCalls: toolCalls})
	}
	return append(out, toolResults...)
}
