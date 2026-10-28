package ai

import (
	"encoding/json"
	"errors"
	"strings"
)

const (
	roleSystem    = "system"
	roleUser      = "user"
	roleAssistant = "assistant"
	roleTool      = "tool"
)

var errNoHistory = errors.New("this provider does not take a conversation yet")

type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

func argumentsText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	return string(raw)
}

func openAIMessages(r Request) []map[string]any {
	if len(r.Messages) == 0 {
		return []map[string]any{{"role": roleUser, "content": r.Prompt}}
	}
	out := make([]map[string]any, 0, len(r.Messages))
	for _, m := range r.Messages {
		out = append(out, openAIMessage(m))
	}
	return out
}

func openAIMessage(m Message) map[string]any {
	if m.Role == roleTool {
		return map[string]any{"role": roleTool, "tool_call_id": m.ToolCallID, "content": m.Content}
	}
	if m.Role != roleAssistant || len(m.ToolCalls) == 0 {
		return map[string]any{"role": m.Role, "content": m.Content}
	}
	calls := make([]map[string]any, 0, len(m.ToolCalls))
	for _, c := range m.ToolCalls {
		function := map[string]any{"name": c.Name, "arguments": argumentsText(c.Arguments)}
		calls = append(calls, map[string]any{"id": c.ID, "type": "function", "function": function})
	}
	out := map[string]any{"role": roleAssistant, "tool_calls": calls, "content": nil}
	if m.Content != "" {
		out["content"] = m.Content
	}
	return out
}

func anthropicMessages(r Request) (string, []map[string]any) {
	if len(r.Messages) == 0 {
		return "", []map[string]any{{"role": roleUser, "content": r.Prompt}}
	}
	var system []string
	var out []map[string]any
	for _, m := range r.Messages {
		switch m.Role {
		case roleSystem:
			system = append(system, m.Content)
		case roleTool:
			result := map[string]any{"type": "tool_result", "tool_use_id": m.ToolCallID, "content": m.Content}
			out = withToolResult(out, result)
		case roleAssistant:
			out = append(out, map[string]any{"role": roleAssistant, "content": anthropicBlocks(m)})
		default:
			out = append(out, map[string]any{"role": roleUser, "content": m.Content})
		}
	}
	return strings.Join(system, "\n\n"), out
}

func withToolResult(messages []map[string]any, result map[string]any) []map[string]any {
	if n := len(messages); n > 0 {
		if blocks, ok := messages[n-1]["content"].([]map[string]any); ok && messages[n-1]["role"] == roleUser {
			messages[n-1]["content"] = append(blocks, result)
			return messages
		}
	}
	return append(messages, map[string]any{"role": roleUser, "content": []map[string]any{result}})
}

func anthropicBlocks(m Message) []map[string]any {
	var blocks []map[string]any
	if m.Content != "" {
		blocks = append(blocks, map[string]any{"type": "text", "text": m.Content})
	}
	for _, c := range m.ToolCalls {
		blocks = append(blocks, map[string]any{"type": "tool_use", "id": c.ID, "name": c.Name, "input": json.RawMessage(argumentsText(c.Arguments))})
	}
	return blocks
}
