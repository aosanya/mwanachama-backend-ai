package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/aosanya/mwanachama-backend-ai/models"
)

const (
	anthropicVersion = "2023-06-01"
	maxReplyBytes    = 1 << 20
)

type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

type Request struct {
	Prompt    string
	Messages  []Message
	Tools     []Tool
	MaxTokens int
}

type Reply struct {
	Text         string
	ToolCalls    []ToolCall
	InputTokens  int
	OutputTokens int
}

type Adapter interface {
	BuildRequest(ctx context.Context, a Account, key string, r Request) (*http.Request, error)
	ParseReply(body []byte) (Reply, error)
}

var adapterByKind = map[string]Adapter{
	models.ProviderOpenAICompatible: openAIAdapter{},
	models.ProviderAnthropic:        anthropicAdapter{},
	models.ProviderGemini:           geminiAdapter{},
}

func HasAdapter(kind string) bool { _, ok := adapterByKind[kind]; return ok }

func AdapterFor(kind string) (Adapter, bool) { a, ok := adapterByKind[kind]; return a, ok }

func postJSON(ctx context.Context, endpoint string, payload any) (*http.Request, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func base(a Account) string { return strings.TrimRight(a.BaseURL, "/") }

type openAIAdapter struct{}

func (openAIAdapter) BuildRequest(ctx context.Context, a Account, key string, r Request) (*http.Request, error) {
	payload := map[string]any{
		"model":      a.ModelName,
		"max_tokens": r.MaxTokens,
		"messages":   openAIMessages(r),
	}
	if len(r.Tools) > 0 {
		tools := make([]map[string]any, 0, len(r.Tools))
		for _, t := range r.Tools {
			tools = append(tools, map[string]any{"type": "function", "function": map[string]any{
				"name": t.Name, "description": t.Description, "parameters": t.Parameters}})
		}
		payload["tools"] = tools
	}
	req, err := postJSON(ctx, base(a)+"/chat/completions", payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	return req, nil
}

func (openAIAdapter) ParseReply(body []byte) (Reply, error) {
	var v struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			Prompt     int `json:"prompt_tokens"`
			Completion int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return Reply{}, err
	}
	if len(v.Choices) == 0 {
		return Reply{}, errors.New("no choices in the answer")
	}
	out := Reply{Text: v.Choices[0].Message.Content, InputTokens: v.Usage.Prompt, OutputTokens: v.Usage.Completion}
	for _, c := range v.Choices[0].Message.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, ToolCall{ID: c.ID, Name: c.Function.Name, Arguments: json.RawMessage(c.Function.Arguments)})
	}
	return out, nil
}

type anthropicAdapter struct{}

func (anthropicAdapter) BuildRequest(ctx context.Context, a Account, key string, r Request) (*http.Request, error) {
	system, messages := anthropicMessages(r)
	payload := map[string]any{
		"model":      a.ModelName,
		"max_tokens": r.MaxTokens,
		"messages":   messages,
	}
	if system != "" {
		payload["system"] = system
	}
	if len(r.Tools) > 0 {
		tools := make([]map[string]any, 0, len(r.Tools))
		for _, t := range r.Tools {
			tools = append(tools, map[string]any{"name": t.Name, "description": t.Description, "input_schema": t.Parameters})
		}
		payload["tools"] = tools
	}
	req, err := postJSON(ctx, base(a)+"/v1/messages", payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", anthropicVersion)
	return req, nil
}

func (anthropicAdapter) ParseReply(body []byte) (Reply, error) {
	var v struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
		Usage struct {
			Input  int `json:"input_tokens"`
			Output int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return Reply{}, err
	}
	out := Reply{InputTokens: v.Usage.Input, OutputTokens: v.Usage.Output}
	for _, c := range v.Content {
		switch c.Type {
		case "text":
			out.Text += c.Text
		case "tool_use":
			out.ToolCalls = append(out.ToolCalls, ToolCall{ID: c.ID, Name: c.Name, Arguments: c.Input})
		}
	}
	return out, nil
}

type geminiAdapter struct{}

func (geminiAdapter) BuildRequest(ctx context.Context, a Account, key string, r Request) (*http.Request, error) {
	if len(r.Messages) > 0 {
		return nil, errNoHistory
	}
	payload := map[string]any{
		"contents":         []map[string]any{{"role": "user", "parts": []map[string]string{{"text": r.Prompt}}}},
		"generationConfig": map[string]any{"maxOutputTokens": r.MaxTokens},
	}
	if len(r.Tools) > 0 {
		decls := make([]map[string]any, 0, len(r.Tools))
		for _, t := range r.Tools {
			decls = append(decls, map[string]any{"name": t.Name, "description": t.Description, "parameters": t.Parameters})
		}
		payload["tools"] = []map[string]any{{"functionDeclarations": decls}}
	}
	req, err := postJSON(ctx, base(a)+"/v1beta/models/"+url.PathEscape(a.ModelName)+":generateContent", payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-goog-api-key", key)
	return req, nil
}

func (geminiAdapter) ParseReply(body []byte) (Reply, error) {
	var v struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall *struct {
						Name string          `json:"name"`
						Args json.RawMessage `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Usage struct {
			Prompt    int `json:"promptTokenCount"`
			Candidate int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return Reply{}, err
	}
	if len(v.Candidates) == 0 {
		return Reply{}, errors.New("no candidates in the answer")
	}
	out := Reply{InputTokens: v.Usage.Prompt, OutputTokens: v.Usage.Candidate}
	for _, p := range v.Candidates[0].Content.Parts {
		if p.FunctionCall != nil {
			out.ToolCalls = append(out.ToolCalls, ToolCall{Name: p.FunctionCall.Name, Arguments: p.FunctionCall.Args})
		} else {
			out.Text += p.Text
		}
	}
	return out, nil
}

func Call(ctx context.Context, client *http.Client, a Account, key string, r Request) (Reply, ProbeOutcome) {
	ad, ok := AdapterFor(a.Provider)
	if !ok {
		return Reply{}, ProbeError
	}
	req, err := ad.BuildRequest(ctx, a, key, r)
	if err != nil {
		return Reply{}, ProbeUnreached
	}
	resp, err := client.Do(req)
	if err != nil {
		return Reply{}, ProbeUnreached
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return Reply{}, ProbeRefused
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return Reply{}, ProbeError
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReplyBytes))
	if err != nil {
		return Reply{}, ProbeUnreached
	}
	reply, err := ad.ParseReply(body)
	if err != nil {
		return Reply{}, ProbeError
	}
	return reply, ProbeOK
}
