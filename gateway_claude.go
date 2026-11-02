package ai

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	errUncarriable        = errors.New("the request cannot be carried to this model")
	errNotChatCompletions = errors.New("the request is not in the OpenAI chat-completions shape")
	errNoMessages         = errors.New("the request has no messages this model can take")
	errOnlySystem         = errors.New("this model needs at least one message besides the system prompt")
	errBadArguments       = errors.New("a tool call's arguments are not a JSON object")
	errNotFunction        = errors.New("only function tools can be carried to this model")
	errToolNoName         = errors.New("a tool has no name")
	errCallNoID           = errors.New("a tool call has no id")
	errCallNoName         = errors.New("a tool call has no name")
	errToolMessageNoID    = errors.New("a tool message has no tool_call_id")
)

const (
	roleFunction     = "function"
	toolTypeFunction = "function"
)

type openAIBody struct {
	Messages            []openAIChatMessage `json:"messages"`
	Tools               []openAIChatTool    `json:"tools"`
	MaxTokens           int                 `json:"max_tokens"`
	MaxCompletionTokens int                 `json:"max_completion_tokens"`
}

type openAIChatMessage struct {
	Role       string          `json:"role"`
	Name       string          `json:"name"`
	Content    json.RawMessage `json:"content"`
	ToolCallID string          `json:"tool_call_id"`
	ToolCalls  []struct {
		ID       string `json:"id"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls"`
}

type openAIChatTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

func (g *gateway) relayClaude(r *http.Request, a Account, key string, payload map[string]any, limit int) (answered, *failure) {
	request, err := claudeRequest(payload, limit)
	if err != nil {
		return answered{}, &failure{http.StatusBadRequest, err.Error()}
	}
	reply, outcome := Call(r.Context(), g.client, a, key, request)
	switch outcome {
	case ProbeOK:
		body, _ := json.Marshal(openAIAnswer(a.ModelName, reply))
		return answered{body: body, tokensIn: reply.InputTokens, tokensOut: reply.OutputTokens}, nil
	case ProbeUnreached:
		return answered{}, &failure{http.StatusBadGateway, modelUnreached}
	case ProbeRedirected:
		return answered{}, &failure{http.StatusBadGateway, TestRedirected}
	}
	return answered{}, &failure{http.StatusBadGateway, modelErrored}
}

func claudeRequest(payload map[string]any, limit int) (Request, error) {
	raw, _ := json.Marshal(payload)
	var body openAIBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return Request{}, errNotChatCompletions
	}
	if len(body.Messages) == 0 {
		return Request{}, errNoMessages
	}
	messages, err := claudeMessages(body.Messages)
	if err != nil {
		return Request{}, err
	}
	tools, err := claudeTools(body.Tools)
	if err != nil {
		return Request{}, err
	}
	return Request{Messages: messages, Tools: tools, MaxTokens: chatMaxTokens(smallerAsk(body.MaxTokens, body.MaxCompletionTokens), limit)}, nil
}

func claudeTools(in []openAIChatTool) ([]Tool, error) {
	tools := make([]Tool, 0, len(in))
	for _, t := range in {
		switch {
		case t.Type != "" && t.Type != toolTypeFunction:
			return nil, errNotFunction
		case blank(t.Function.Name):
			return nil, errToolNoName
		}
		tools = append(tools, Tool{Name: t.Function.Name, Description: t.Function.Description, Parameters: parametersOrEmpty(t.Function.Parameters)})
	}
	return tools, nil
}

func blank(text string) bool { return strings.TrimSpace(text) == "" }

func smallerAsk(first, second int) int {
	switch {
	case first <= 0:
		return second
	case second <= 0:
		return first
	}
	return min(first, second)
}

func claudeMessages(in []openAIChatMessage) ([]Message, error) {
	messages := make([]Message, 0, len(in))
	spoken := false
	for _, m := range in {
		message, err := claudeMessage(m)
		if err != nil {
			return nil, err
		}
		if carriesNothing(message) {
			continue
		}
		spoken = spoken || message.Role != roleSystem
		messages = append(messages, message)
	}
	if !spoken {
		return nil, errOnlySystem
	}
	return messages, nil
}

func claudeMessage(m openAIChatMessage) (Message, error) {
	text, err := contentText(m.Content)
	if err != nil {
		return Message{}, err
	}
	if m.Role == roleTool && blank(m.ToolCallID) {
		return Message{}, errToolMessageNoID
	}
	message := Message{Role: m.Role, Content: text, ToolCallID: m.ToolCallID}
	switch m.Role {
	case "developer":
		message.Role = roleSystem
	case roleFunction:
		message.Role, message.Content = roleUser, functionResult(m.Name, text)
	}
	for _, c := range m.ToolCalls {
		switch {
		case blank(c.ID):
			return Message{}, errCallNoID
		case blank(c.Function.Name):
			return Message{}, errCallNoName
		}
		arguments, err := callArguments(c.Function.Arguments)
		if err != nil {
			return Message{}, fmt.Errorf("%w: the call %q (id %s)", err, c.Function.Name, c.ID)
		}
		message.ToolCalls = append(message.ToolCalls, ToolCall{ID: c.ID, Name: c.Function.Name, Arguments: arguments})
	}
	return message, nil
}

func carriesNothing(m Message) bool {
	return m.Role != roleTool && len(m.ToolCalls) == 0 && strings.TrimSpace(m.Content) == ""
}

func functionResult(name, text string) string {
	if name == "" {
		return text
	}
	return name + ": " + text
}

func callArguments(text string) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || trimmed == "null" {
		return json.RawMessage("{}"), nil
	}
	var object map[string]json.RawMessage
	if json.Unmarshal([]byte(trimmed), &object) != nil || object == nil {
		return nil, errBadArguments
	}
	return json.RawMessage(trimmed), nil
}

func contentText(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text, nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return "", errUncarriable
	}
	texts := make([]string, 0, len(parts))
	for _, p := range parts {
		if p.Type != "text" {
			return "", errors.New("only text can be carried to this model")
		}
		texts = append(texts, p.Text)
	}
	return strings.Join(texts, "\n"), nil
}

func openAIAnswer(model string, reply Reply) map[string]any {
	message := map[string]any{"role": roleAssistant, "content": nil}
	if reply.Text != "" {
		message["content"] = reply.Text
	}
	if len(reply.ToolCalls) > 0 {
		calls := make([]map[string]any, 0, len(reply.ToolCalls))
		for i, c := range reply.ToolCalls {
			id := c.ID
			if id == "" {
				id = fmt.Sprintf("call-%d", i+1)
			}
			function := map[string]any{"name": c.Name, "arguments": argumentsText(objectOrEmpty(c.Arguments))}
			calls = append(calls, map[string]any{"id": id, "type": "function", "function": function})
		}
		message["tool_calls"] = calls
	}
	return map[string]any{
		"id":      "chatcmpl-" + randomHex(),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]any{{"index": 0, "message": message, "finish_reason": openAIFinish(reply.FinishReason)}},
		"usage":   map[string]int{"prompt_tokens": reply.InputTokens, "completion_tokens": reply.OutputTokens, "total_tokens": reply.InputTokens + reply.OutputTokens},
	}
}

func openAIFinish(claudeReason string) string {
	switch claudeReason {
	case "tool_use":
		return "tool_calls"
	case "max_tokens":
		return "length"
	case "refusal":
		return "content_filter"
	}
	return "stop"
}

func randomHex() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
