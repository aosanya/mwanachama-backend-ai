package ai_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

type fakeClaude struct {
	mu      sync.Mutex
	calls   int
	path    string
	headers http.Header
	body    map[string]any
	status  int
	reply   string
}

func (f *fakeClaude) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.path, f.headers = r.URL.Path, r.Header.Clone()
	raw, _ := io.ReadAll(r.Body)
	f.body = nil
	_ = json.Unmarshal(raw, &f.body)
	if f.status != 0 {
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte("upstream failed for " + r.Header.Get("x-api-key")))
		return
	}
	_, _ = w.Write([]byte(f.reply))
}

const claudeReply = `{"id":"msg_1","type":"message","role":"assistant","stop_reason":"tool_use","content":[{"type":"text","text":"hi"},{"type":"tool_use","id":"tu_1","name":"lookup","input":{"q":1}}],"usage":{"input_tokens":11,"output_tokens":7}}`

const agentBody = `{
	"model": "gpt-4o",
	"max_tokens": 9000,
	"messages": [
		{"role": "system", "content": "be terse"},
		{"role": "user", "content": [{"type": "text", "text": "look it up"}]},
		{"role": "assistant", "content": null, "tool_calls": [{"id": "c0", "type": "function", "function": {"name": "lookup", "arguments": "{\"q\":0}"}}]},
		{"role": "tool", "tool_call_id": "c0", "content": "zero"}
	],
	"tools": [{"type": "function", "function": {"name": "lookup", "description": "finds", "parameters": {"type": "object", "properties": {"q": {"type": "integer"}}}}}]
}`

func TestGatewayServesAClaudeAccountInOpenAIShape(t *testing.T) {
	fc := &fakeClaude{reply: claudeReply}
	h, m, r, acc := gatewaySetupKind(t, "anthropic", fc, 50)
	rec := post(h, agentBody)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if fc.path != "/v1/messages" || fc.headers.Get("x-api-key") != syntheticKey || fc.headers.Get("Authorization") != "" || fc.headers.Get("anthropic-version") == "" {
		t.Fatalf("outgoing request: %s %v", fc.path, fc.headers)
	}
	if fc.body["model"] != "synthetic" || fc.body["max_tokens"].(float64) != 50 || fc.body["system"] != "be terse" {
		t.Fatalf("outgoing body: %v", fc.body)
	}
	sent, _ := json.Marshal(fc.body["messages"])
	if !strings.Contains(string(sent), `"tool_use_id":"c0"`) || !strings.Contains(string(sent), `"look it up"`) || !strings.Contains(string(sent), `"input":{"q":0}`) {
		t.Fatalf("outgoing messages: %s", sent)
	}
	var answer struct {
		Object  string `json:"object"`
		Model   string `json:"model"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Role      string  `json:"role"`
				Content   *string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatalf("%v: %s", err, rec.Body)
	}
	c := answer.Choices[0]
	if answer.Object != "chat.completion" || c.Message.Role != "assistant" || c.Message.Content == nil || *c.Message.Content != "hi" || c.FinishReason != "tool_calls" {
		t.Fatalf("answer: %s", rec.Body)
	}
	call := c.Message.ToolCalls[0]
	if call.ID != "tu_1" || call.Type != "function" || call.Function.Name != "lookup" || call.Function.Arguments != `{"q":1}` {
		t.Fatalf("tool call: %+v", call)
	}
	if answer.Usage.PromptTokens != 11 || answer.Usage.CompletionTokens != 7 || answer.Usage.TotalTokens != 18 {
		t.Fatalf("usage: %+v", answer.Usage)
	}
	if strings.Contains(rec.Body.String(), syntheticKey) {
		t.Fatal("key reached the caller")
	}
	got, _ := m.ResolveAIRole(ctx, "ag", r.ID)
	for _, l := range got.Limits {
		if l.Limit == "daily token" && l.Used != 18 {
			t.Fatalf("tokens used = %d", l.Used)
		}
	}
	rep, _ := m.ListUsage(ctx, "ag", acc.ID)
	for _, c := range rep.Counts {
		if c.Field == "requests_per_day" && c.Used != 1 {
			t.Fatalf("requests used = %d", c.Used)
		}
	}
}

func TestGatewayAnswersAClaudeEndOfTurnAsStop(t *testing.T) {
	fc := &fakeClaude{reply: `{"stop_reason":"end_turn","content":[{"type":"text","text":"done"}],"usage":{"input_tokens":1,"output_tokens":2}}`}
	h, _, _, _ := gatewaySetupKind(t, "anthropic", fc, 0)
	rec := post(h, `{"messages":[{"role":"user","content":"x"}]}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"finish_reason":"stop"`) || !strings.Contains(rec.Body.String(), `"content":"done"`) || strings.Contains(rec.Body.String(), "tool_calls") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if fc.body["max_tokens"].(float64) <= 0 {
		t.Fatalf("Claude needs max_tokens, sent %v", fc.body["max_tokens"])
	}
}

func TestGatewayDoesNotEchoAClaudeError(t *testing.T) {
	fc := &fakeClaude{status: 500}
	h, m, _, acc := gatewaySetupKind(t, "anthropic", fc, 0)
	rec := post(h, `{"messages":[{"role":"user","content":"x"}]}`)
	if rec.Code != 502 || strings.Contains(rec.Body.String(), syntheticKey) || strings.Contains(rec.Body.String(), "upstream failed") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rep, _ := m.ListUsage(ctx, "ag", acc.ID)
	for _, c := range rep.Counts {
		if c.Field == "requests_per_day" && c.Used != 0 {
			t.Fatalf("a failed call was counted: %d", c.Used)
		}
	}
}

func TestGatewayRefusesAClaudeAccountAtALimitBeforeAnythingLeaves(t *testing.T) {
	fc := &fakeClaude{reply: claudeReply}
	h, m, _, acc := gatewaySetupKind(t, "anthropic", fc, 0)
	if _, err := m.SetAccountLimits(ctx, "ag", acc.ID, ai.Limits{RequestsPerDay: ip(1)}); err != nil {
		t.Fatal(err)
	}
	body := `{"messages":[{"role":"user","content":"x"}]}`
	if rec := post(h, body); rec.Code != 200 {
		t.Fatalf("first: %d %s", rec.Code, rec.Body)
	}
	if rec := post(h, body); rec.Code != 429 {
		t.Fatalf("second: %d %s", rec.Code, rec.Body)
	}
	if fc.calls != 1 {
		t.Fatalf("Claude was reached %d times", fc.calls)
	}
}

func TestGatewayRefusesAClaudeBodyItCannotCarry(t *testing.T) {
	cases := map[string]string{
		"no messages": `{"messages":[]}`,
		"an image":    `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"x"}}]}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			fc := &fakeClaude{reply: claudeReply}
			h, _, _, _ := gatewaySetupKind(t, "anthropic", fc, 0)
			if rec := post(h, body); rec.Code != 400 || fc.calls != 0 {
				t.Fatalf("%d %s, calls %d", rec.Code, rec.Body, fc.calls)
			}
		})
	}
}

func TestGatewayStillRefusesAKindItHasNoTranslationFor(t *testing.T) {
	fc := &fakeClaude{reply: claudeReply}
	h, _, _, _ := gatewaySetupKind(t, "gemini", fc, 0)
	if rec := post(h, `{"messages":[{"role":"user","content":"x"}]}`); rec.Code != 409 || fc.calls != 0 {
		t.Fatalf("%d %s, calls %d", rec.Code, rec.Body, fc.calls)
	}
}
