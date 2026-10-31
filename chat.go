package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aosanya/mwanachama-backend-ai/models"
)

type (
	ChatInput    = models.ChatInput
	ChatMessage  = models.ChatMessage
	ChatTool     = models.ChatTool
	ChatToolCall = models.ChatToolCall
	ChatReply    = models.ChatReply
)

const (
	defaultChatTokens = 1024
	maxChatTokens     = 16384
	chatTimeout       = 2 * time.Minute
)

var chatProviders = map[string]bool{
	models.ProviderOpenAICompatible: true,
	models.ProviderAnthropic:        true,
}

type chatSettings struct {
	client *http.Client
	writes usageWrites
}

func WithChatClient(c *http.Client) Option {
	return func(m *accountManager) { m.chat.client = c }
}

func (m *accountManager) chatClient() *http.Client {
	if m.chat.client != nil {
		return m.chat.client
	}
	return &http.Client{Timeout: chatTimeout}
}

func (m *accountManager) ChatAccount(ctx context.Context, agencyID, accountID string, in ChatInput) (ChatReply, error) {
	held, err := m.GetAccount(ctx, agencyID, accountID)
	if err != nil {
		return ChatReply{}, err
	}
	if err := checkChat(held, in); err != nil {
		return ChatReply{}, err
	}
	key, err := m.AccountKey(ctx, agencyID, accountID)
	if err != nil {
		return ChatReply{}, err
	}
	decision, err := m.CheckLimits(ctx, agencyID, CheckInput{AccountID: accountID})
	if err != nil {
		return ChatReply{}, err
	}
	if !decision.Allowed {
		return ChatReply{}, fmt.Errorf("%w: %s", ErrLimitReached, decision.Reason)
	}
	reply, outcome := Call(ctx, m.chatClient(), held, key, chatRequest(in, decision.MaxTokens))
	if err := outcomeError(outcome); err != nil {
		return ChatReply{}, err
	}
	used := UsageInput{AccountID: accountID, TokensIn: reply.InputTokens, TokensOut: reply.OutputTokens}
	recording := context.WithoutCancel(ctx)
	m.chat.writes.after(agencyID, accountID, func() { _, _ = m.RecordUsage(recording, agencyID, used) })
	return chatReply(reply), nil
}

func outcomeError(outcome ProbeOutcome) error {
	switch outcome {
	case ProbeOK:
		return nil
	case ProbeRefused:
		return ErrModelRefused
	case ProbeUnreached:
		return ErrModelUnreached
	}
	return ErrModelError
}

func chatRequest(in ChatInput, limit int) Request {
	messages := make([]Message, 0, len(in.Messages))
	for _, m := range in.Messages {
		calls := make([]ToolCall, 0, len(m.ToolCalls))
		for _, c := range m.ToolCalls {
			calls = append(calls, ToolCall{ID: c.ID, Name: c.Name, Arguments: objectOrEmpty(c.Arguments)})
		}
		messages = append(messages, Message{Role: m.Role, Content: m.Content, ToolCalls: calls, ToolCallID: m.ToolCallID})
	}
	tools := make([]Tool, 0, len(in.Tools))
	for _, t := range in.Tools {
		tools = append(tools, Tool{Name: t.Name, Description: t.Description, Parameters: parametersOrEmpty(t.Parameters)})
	}
	return Request{Messages: messages, Tools: tools, MaxTokens: chatMaxTokens(in.MaxTokens, limit)}
}

func chatMaxTokens(asked, limit int) int {
	n := asked
	if n <= 0 {
		n = defaultChatTokens
	}
	if n > maxChatTokens {
		n = maxChatTokens
	}
	if limit > 0 && n > limit {
		n = limit
	}
	return n
}

func chatReply(reply Reply) ChatReply {
	message := ChatMessage{Role: roleAssistant, Content: reply.Text}
	for i, c := range reply.ToolCalls {
		id := c.ID
		if id == "" {
			id = fmt.Sprintf("call-%d", i+1)
		}
		message.ToolCalls = append(message.ToolCalls, ChatToolCall{ID: id, Name: c.Name, Arguments: objectOrEmpty(c.Arguments)})
	}
	return ChatReply{Message: message, InputTokens: reply.InputTokens, OutputTokens: reply.OutputTokens, FinishReason: reply.FinishReason}
}

func objectOrEmpty(raw json.RawMessage) json.RawMessage {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return json.RawMessage("{}")
	}
	return raw
}

func parametersOrEmpty(parameters map[string]any) map[string]any {
	if parameters == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return parameters
}
