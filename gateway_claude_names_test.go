package ai_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const (
	notFunctionSentence = "only function tools can be carried to this model"
	toolNoNameSentence  = "a tool has no name"
	callNoIDSentence    = "a tool call has no id"
	callNoNameSentence  = "a tool call has no name"
	resultNoIDSentence  = "a tool message has no tool_call_id"
)

func claudeBodyWithTool(tool string) string {
	return `{"messages":[{"role":"user","content":"x"}],"tools":[` + tool + `]}`
}

func claudeBodyWithCall(call string) string {
	return `{"messages":[{"role":"user","content":"x"},{"role":"assistant","content":null,"tool_calls":[` + call + `]},{"role":"tool","tool_call_id":"c1","content":"ok"}]}`
}

func requireRefusedBeforeAnythingLeaves(t *testing.T, body, sentence string) {
	t.Helper()
	fc := &fakeClaude{reply: claudeReply}
	h, m, _, acc := gatewaySetupKind(t, "anthropic", fc, 0)
	rec := post(h, body)
	if rec.Code != http.StatusBadRequest || fc.calls != 0 || !strings.Contains(rec.Body.String(), sentence) {
		t.Fatalf("%d %s, calls %d", rec.Code, rec.Body, fc.calls)
	}
	rep, _ := m.ListUsage(ctx, "ag", acc.ID)
	for _, c := range rep.Counts {
		if c.Field == "requests_per_day" && c.Used != 0 {
			t.Fatalf("a refused body was counted: %d", c.Used)
		}
	}
}

func TestAClaudeToolThatIsNotAFunctionIsRefusedBeforeAnythingLeaves(t *testing.T) {
	tools := map[string]string{
		"a custom tool":                        `{"type":"custom","custom":{"name":"lookup"}}`,
		"a web search tool":                    `{"type":"web_search"}`,
		"another type beside a function":       `{"type":"retrieval","function":{"name":"lookup","parameters":{"type":"object"}}}`,
		"a non-function tool after a function": `{"type":"function","function":{"name":"ok"}},{"type":"mcp","function":{"name":"lookup"}}`,
	}
	for name, tool := range tools {
		t.Run(name, func(t *testing.T) {
			requireRefusedBeforeAnythingLeaves(t, claudeBodyWithTool(tool), notFunctionSentence)
		})
	}
}

func TestAClaudeToolWithNoNameIsRefusedBeforeAnythingLeaves(t *testing.T) {
	tools := map[string]string{
		"a name left out":         `{"type":"function","function":{"description":"d"}}`,
		"an empty name":           `{"type":"function","function":{"name":""}}`,
		"spaces as a name":        `{"type":"function","function":{"name":"  "}}`,
		"no function":             `{"type":"function"}`,
		"no type and no function": `{}`,
	}
	for name, tool := range tools {
		t.Run(name, func(t *testing.T) {
			requireRefusedBeforeAnythingLeaves(t, claudeBodyWithTool(tool), toolNoNameSentence)
		})
	}
}

func TestAClaudeToolWithNoTypeButAFunctionIsStillSent(t *testing.T) {
	fc := &fakeClaude{reply: claudeReply}
	h, _, _, _ := gatewaySetupKind(t, "anthropic", fc, 0)
	rec := post(h, claudeBodyWithTool(`{"function":{"name":"lookup","parameters":{"type":"object"}}}`))
	sent, _ := fc.body["tools"].([]any)
	if rec.Code != http.StatusOK || len(sent) != 1 || sent[0].(map[string]any)["name"] != "lookup" {
		t.Fatalf("%d %s, tools %v", rec.Code, rec.Body, fc.body["tools"])
	}
}

func TestAClaudeToolCallWithNoIDOrNameIsRefusedBeforeAnythingLeaves(t *testing.T) {
	cases := map[string]struct{ call, sentence string }{
		"an id left out":     {`{"type":"function","function":{"name":"lookup","arguments":"{}"}}`, callNoIDSentence},
		"an empty id":        {`{"id":"","type":"function","function":{"name":"lookup","arguments":"{}"}}`, callNoIDSentence},
		"spaces as an id":    {`{"id":" ","type":"function","function":{"name":"lookup","arguments":"{}"}}`, callNoIDSentence},
		"a name left out":    {`{"id":"c1","type":"function","function":{"arguments":"{}"}}`, callNoNameSentence},
		"an empty name":      {`{"id":"c1","type":"function","function":{"name":"","arguments":"{}"}}`, callNoNameSentence},
		"spaces as a name":   {`{"id":"c1","type":"function","function":{"name":"  ","arguments":"{}"}}`, callNoNameSentence},
		"no function":        {`{"id":"c1","type":"function"}`, callNoNameSentence},
		"a second call bare": {`{"id":"c1","type":"function","function":{"name":"lookup","arguments":"{}"}},{"id":"c2","type":"function","function":{"name":"","arguments":"{}"}}`, callNoNameSentence},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			requireRefusedBeforeAnythingLeaves(t, claudeBodyWithCall(c.call), c.sentence)
		})
	}
}

func TestAClaudeToolMessageWithNoToolCallIDIsRefusedBeforeAnythingLeaves(t *testing.T) {
	messages := map[string]string{
		"an id left out":  `{"role":"tool","content":"ok"}`,
		"an empty id":     `{"role":"tool","tool_call_id":"","content":"ok"}`,
		"spaces as an id": `{"role":"tool","tool_call_id":" ","content":"ok"}`,
	}
	for name, message := range messages {
		t.Run(name, func(t *testing.T) {
			body := `{"messages":[{"role":"user","content":"x"},{"role":"assistant","content":null,"tool_calls":[{"id":"c1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},` + message + `]}`
			requireRefusedBeforeAnythingLeaves(t, body, resultNoIDSentence)
		})
	}
}

func TestTheDeprecatedFunctionFieldsAreIgnoredAndTheRequestGoesOutWithoutThem(t *testing.T) {
	const functions = `"functions":[{"name":"lookup","description":"finds","parameters":{"type":"object"}}],"function_call":"auto"`
	cases := map[string]struct{ body, carried string }{
		"a function_call beside text": {
			`{"messages":[{"role":"user","content":"look it up"},{"role":"assistant","content":"one moment","function_call":{"name":"lookup","arguments":"{\"q\":0}"}},{"role":"function","name":"lookup","content":"zero"}],` + functions + `}`,
			"one moment",
		},
		"a function_call and nothing else": {
			`{"messages":[{"role":"user","content":"look it up"},{"role":"assistant","content":null,"function_call":{"name":"lookup","arguments":"{\"q\":0}"}},{"role":"function","name":"lookup","content":"zero"}],` + functions + `}`,
			"lookup: zero",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			fc := &fakeClaude{reply: claudeReply}
			h, _, _, _ := gatewaySetupKind(t, "anthropic", fc, 0)
			rec := post(h, c.body)
			if rec.Code != http.StatusOK {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			requireValidClaudeMessages(t, fc.body["messages"])
			sent, _ := json.Marshal(fc.body)
			for _, forbidden := range []string{"functions", "function_call", "tool_use", `"tools"`, "input_schema"} {
				if strings.Contains(string(sent), forbidden) {
					t.Fatalf("the request went out with %s: %s", forbidden, sent)
				}
			}
			if !strings.Contains(string(sent), c.carried) {
				t.Fatalf("%q was not carried: %s", c.carried, sent)
			}
		})
	}
}
