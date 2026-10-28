package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aosanya/mwanachama-backend-ai/models"
)

const (
	maxChatMessages = 200
	maxChatTools    = 128
	maxChatBytes    = 1 << 20
)

func invalidChat(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidChat, fmt.Sprintf(format, args...))
}

func checkChat(a Account, in ChatInput) error {
	if a.Status != models.StatusEnabled {
		return ErrAccountOff
	}
	if !chatProviders[a.Provider] {
		return ErrChatUnsupported
	}
	return validChat(in)
}

func validChat(in ChatInput) error {
	switch {
	case len(in.Messages) == 0:
		return invalidChat("messages are required")
	case len(in.Messages) > maxChatMessages:
		return invalidChat("at most %d messages", maxChatMessages)
	case len(in.Tools) > maxChatTools:
		return invalidChat("at most %d tools", maxChatTools)
	case in.MaxTokens < 0:
		return invalidChat("max_tokens cannot be negative")
	}
	size := 0
	for i, m := range in.Messages {
		if err := validMessage(i+1, m); err != nil {
			return err
		}
		size += len(m.Content)
	}
	if size > maxChatBytes {
		return invalidChat("the conversation is too long")
	}
	return validTools(in.Tools)
}

func validMessage(n int, m ChatMessage) error {
	if m.Role != roleAssistant && len(m.ToolCalls) > 0 {
		return invalidChat("message %d: only an assistant message carries tool_calls", n)
	}
	switch m.Role {
	case roleSystem, roleUser:
		if strings.TrimSpace(m.Content) == "" {
			return invalidChat("message %d: content is required", n)
		}
	case roleAssistant:
		if strings.TrimSpace(m.Content) == "" && len(m.ToolCalls) == 0 {
			return invalidChat("message %d: an assistant message needs content or tool_calls", n)
		}
		return validCalls(n, m.ToolCalls)
	case roleTool:
		if m.ToolCallID == "" {
			return invalidChat("message %d: a tool message needs its tool_call_id", n)
		}
	default:
		return invalidChat("message %d: role must be system, user, assistant or tool", n)
	}
	return nil
}

func validCalls(n int, calls []ChatToolCall) error {
	for _, c := range calls {
		if c.ID == "" || c.Name == "" {
			return invalidChat("message %d: each tool call needs an id and a name", n)
		}
		var object map[string]json.RawMessage
		if len(c.Arguments) > 0 && string(c.Arguments) != "null" && (json.Unmarshal(c.Arguments, &object) != nil || object == nil) {
			return invalidChat("message %d: tool call %s arguments must be an object", n, c.ID)
		}
	}
	return nil
}

func validTools(tools []ChatTool) error {
	seen := map[string]bool{}
	for _, t := range tools {
		switch {
		case t.Name == "":
			return invalidChat("each tool needs a name")
		case seen[t.Name]:
			return invalidChat("tool %s is listed twice", t.Name)
		}
		seen[t.Name] = true
	}
	return nil
}
