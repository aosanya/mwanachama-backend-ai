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

const modelRedirected = "the model's address redirected the request, which is not followed"

var errUncarriable = errors.New("the request cannot be carried to this model")

type openAIBody struct {
	Messages            []openAIChatMessage `json:"messages"`
	Tools               []openAIChatTool    `json:"tools"`
	MaxTokens           int                 `json:"max_tokens"`
	MaxCompletionTokens int                 `json:"max_completion_tokens"`
}

type openAIChatMessage struct {
	Role       string          `json:"role"`
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
		return answered{}, &failure{http.StatusBadGateway, modelRedirected}
	}
	return answered{}, &failure{http.StatusBadGateway, modelErrored}
}

func claudeRequest(payload map[string]any, limit int) (Request, error) {
	raw, _ := json.Marshal(payload)
	var body openAIBody
	if err := json.Unmarshal(raw, &body); err != nil || len(body.Messages) == 0 {
		return Request{}, errors.New("the request has no messages this model can take")
	}
	messages := make([]Message, 0, len(body.Messages))
	for _, m := range body.Messages {
		text, err := contentText(m.Content)
		if err != nil {
			return Request{}, err
		}
		message := Message{Role: m.Role, Content: text, ToolCallID: m.ToolCallID}
		if m.Role == "developer" {
			message.Role = roleSystem
		}
		for _, c := range m.ToolCalls {
			message.ToolCalls = append(message.ToolCalls, ToolCall{ID: c.ID, Name: c.Function.Name, Arguments: objectOrEmpty(json.RawMessage(c.Function.Arguments))})
		}
		messages = append(messages, message)
	}
	tools := make([]Tool, 0, len(body.Tools))
	for _, t := range body.Tools {
		tools = append(tools, Tool{Name: t.Function.Name, Description: t.Function.Description, Parameters: parametersOrEmpty(t.Function.Parameters)})
	}
	asked := body.MaxTokens
	if asked <= 0 {
		asked = body.MaxCompletionTokens
	}
	return Request{Messages: messages, Tools: tools, MaxTokens: chatMaxTokens(asked, limit)}, nil
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
