package provider

import (
	"encoding/json"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/types"
)

type antBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type antMsg struct {
	Role    string     `json:"role"`
	Content []antBlock `json:"content"`
}

func buildAnthropicTools(tools []types.ToolDefinition) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		schema := t.Function.Parameters
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, map[string]any{
			"name":         t.Function.Name,
			"description":  t.Function.Description,
			"input_schema": schema,
		})
	}
	return out
}

func buildAnthropicMessages(in []types.ChatMessage) ([]string, []antMsg) {
	var sysParts []string
	var msgs []antMsg
	appendMsg := func(role string, blocks []antBlock) {
		if len(blocks) == 0 {
			return
		}
		if len(msgs) > 0 && msgs[len(msgs)-1].Role == role {
			msgs[len(msgs)-1].Content = append(msgs[len(msgs)-1].Content, blocks...)
			return
		}
		msgs = append(msgs, antMsg{Role: role, Content: blocks})
	}
	for _, m := range in {
		switch strings.ToLower(strings.TrimSpace(m.Role)) {
		case "system":
			if m.Content != "" {
				sysParts = append(sysParts, m.Content)
			}
		case "tool":
			appendMsg("user", []antBlock{{Type: "tool_result", ToolUseID: m.ToolCallID, Content: m.Content}})
		case "assistant":
			var blocks []antBlock
			if m.Content != "" {
				blocks = append(blocks, antBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				rawArgs := json.RawMessage(strings.TrimSpace(tc.Function.Arguments))
				if len(rawArgs) == 0 || !json.Valid(rawArgs) {
					rawArgs = json.RawMessage(`{}`)
				}
				blocks = append(blocks, antBlock{Type: "tool_use", ID: tc.ID, Name: tc.Function.Name, Input: rawArgs})
			}
			appendMsg("assistant", blocks)
		default:
			appendMsg("user", []antBlock{{Type: "text", Text: m.Content}})
		}
	}
	if len(msgs) == 0 {
		msgs = append(msgs, antMsg{Role: "user", Content: []antBlock{{Type: "text", Text: ""}}})
	}
	return sysParts, msgs
}

func mapOpenAIToolChoiceToAnthropic(raw json.RawMessage) any {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch s {
		case "required":
			return map[string]string{"type": "any"}
		case "none", "auto":
			return map[string]string{"type": s}
		}
	}
	return json.RawMessage(raw)
}
