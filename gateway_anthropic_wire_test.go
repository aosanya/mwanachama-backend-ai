package ai_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const (
	onlySystemSentence = "this model needs at least one message besides the system prompt"
	badShapeSentence   = "the request is not in the OpenAI chat-completions shape"
	badArgsSentence    = "a tool call's arguments are not a JSON object"
)

func requireValidClaudeMessages(t *testing.T, sent any) []map[string]any {
	t.Helper()
	list, ok := sent.([]any)
	if !ok || len(list) == 0 {
		t.Fatalf("messages = %v, want a non-empty list", sent)
	}
	out := make([]map[string]any, 0, len(list))
	for i, item := range list {
		m, _ := item.(map[string]any)
		out = append(out, m)
		switch content := m["content"].(type) {
		case string:
			if strings.TrimSpace(content) == "" {
				t.Fatalf("message %d has empty text: %v", i, list)
			}
		case []any:
			if len(content) == 0 {
				t.Fatalf("message %d has no blocks: %v", i, list)
			}
			for _, b := range content {
				block, _ := b.(map[string]any)
				if text, _ := block["text"].(string); block["type"] == "text" && strings.TrimSpace(text) == "" {
					t.Fatalf("message %d has an empty text block: %v", i, list)
				}
			}
		default:
			t.Fatalf("message %d content = %v (%T): %v", i, m["content"], m["content"], list)
		}
	}
	return out
}

func TestAClaudeConversationOfOnlyASystemPromptIsRefusedBeforeAnythingLeaves(t *testing.T) {
	bodies := map[string]string{
		"a system message":                  `{"messages":[{"role":"system","content":"be terse"}]}`,
		"a developer message":               `{"messages":[{"role":"developer","content":"be terse"}]}`,
		"a system message and an empty one": `{"messages":[{"role":"system","content":"x"},{"role":"user","content":""}]}`,
		"only empty turns":                  `{"messages":[{"role":"user","content":""},{"role":"assistant","content":null}]}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			fc := &fakeClaude{reply: claudeReply}
			h, _, _, _ := gatewaySetupKind(t, "anthropic", fc, 0)
			rec := post(h, body)
			if rec.Code != http.StatusBadRequest || fc.calls != 0 || !strings.Contains(rec.Body.String(), onlySystemSentence) {
				t.Fatalf("%d %s, calls %d", rec.Code, rec.Body, fc.calls)
			}
		})
	}
}

func TestAClaudeBodyNeverCarriesAnEmptyTurn(t *testing.T) {
	bodies := map[string]string{
		"an assistant turn with null content":    `{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":null},{"role":"user","content":"again"}]}`,
		"a last assistant turn with null":        `{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":null}]}`,
		"an assistant turn with an empty string": `{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":""}]}`,
		"an assistant turn with spaces":          `{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"  \n"}]}`,
		"a user turn with an empty string":       `{"messages":[{"role":"user","content":""},{"role":"user","content":"hi"}]}`,
		"a user turn with an empty list":         `{"messages":[{"role":"user","content":[]},{"role":"user","content":"hi"}]}`,
		"a user turn with an empty text part":    `{"messages":[{"role":"user","content":[{"type":"text","text":""}]},{"role":"user","content":"hi"}]}`,
		"a tool call with spaces as its text":    `{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":" ","tool_calls":[{"id":"c1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},{"role":"tool","tool_call_id":"c1","content":"ok"}]}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			fc := &fakeClaude{reply: claudeReply}
			h, _, _, _ := gatewaySetupKind(t, "anthropic", fc, 0)
			if rec := post(h, body); rec.Code != http.StatusOK {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			requireValidClaudeMessages(t, fc.body["messages"])
		})
	}
}

func TestAClaudeToolWithoutAnObjectSchemaIsSentWithOne(t *testing.T) {
	tools := map[string]string{
		"empty parameters":       `{}`,
		"properties only":        `{"properties":{"q":{"type":"integer"}}}`,
		"parameters left out":    ``,
		"a schema with its type": `{"type":"object","properties":{}}`,
	}
	for name, parameters := range tools {
		t.Run(name, func(t *testing.T) {
			function := `{"name":"lookup","description":"finds"`
			if parameters != "" {
				function += `,"parameters":` + parameters
			}
			function += `}`
			fc := &fakeClaude{reply: claudeReply}
			h, _, _, _ := gatewaySetupKind(t, "anthropic", fc, 0)
			rec := post(h, `{"messages":[{"role":"user","content":"x"}],"tools":[{"type":"function","function":`+function+`}]}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			sent, _ := fc.body["tools"].([]any)
			schema, _ := sent[0].(map[string]any)["input_schema"].(map[string]any)
			if schema["type"] != "object" {
				t.Fatalf("input_schema = %v", schema)
			}
		})
	}
}

func TestAClaudeToolSchemaKeepsItsProperties(t *testing.T) {
	fc := &fakeClaude{reply: claudeReply}
	h, _, _, _ := gatewaySetupKind(t, "anthropic", fc, 0)
	post(h, `{"messages":[{"role":"user","content":"x"}],"tools":[{"type":"function","function":{"name":"lookup","parameters":{"properties":{"q":{"type":"integer"}},"required":["q"]}}}]}`)
	sent, _ := fc.body["tools"].([]any)
	raw, _ := json.Marshal(sent[0].(map[string]any)["input_schema"])
	if string(raw) != `{"properties":{"q":{"type":"integer"}},"required":["q"],"type":"object"}` {
		t.Fatalf("input_schema = %s", raw)
	}
}

func TestAFunctionResultKeepsItsNameWhenItBecomesAUserMessage(t *testing.T) {
	fc := &fakeClaude{reply: claudeReply}
	h, _, _, _ := gatewaySetupKind(t, "anthropic", fc, 0)
	rec := post(h, `{"messages":[{"role":"user","content":"look it up"},{"role":"function","name":"lookup","content":"zero"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	messages := requireValidClaudeMessages(t, fc.body["messages"])
	last := messages[len(messages)-1]
	text, _ := last["content"].(string)
	if last["role"] != "user" || !strings.Contains(text, "lookup") || !strings.Contains(text, "zero") {
		t.Fatalf("function result sent as %v", last)
	}
}

func TestAToolCallWhoseArgumentsAreNotAJSONObjectIsRefusedBeforeAnythingLeaves(t *testing.T) {
	arguments := map[string]string{
		"broken json":   `{\"q\":`,
		"an array":      `[1,2]`,
		"a bare string": `\"x\"`,
		"a number":      `7`,
	}
	for name, args := range arguments {
		t.Run(name, func(t *testing.T) {
			fc := &fakeClaude{reply: claudeReply}
			h, _, _, _ := gatewaySetupKind(t, "anthropic", fc, 0)
			body := `{"messages":[{"role":"user","content":"x"},{"role":"assistant","content":null,"tool_calls":[{"id":"c1","type":"function","function":{"name":"lookup","arguments":"` + args + `"}}]},{"role":"tool","tool_call_id":"c1","content":"ok"}]}`
			rec := post(h, body)
			if rec.Code != http.StatusBadRequest || fc.calls != 0 || !strings.Contains(rec.Body.String(), badArgsSentence) {
				t.Fatalf("%d %s, calls %d", rec.Code, rec.Body, fc.calls)
			}
		})
	}
}

func TestAToolCallWithNoArgumentsIsSentAsAnEmptyObject(t *testing.T) {
	for name, args := range map[string]string{"empty": ``, "null": `null`, "spaces": ` `} {
		t.Run(name, func(t *testing.T) {
			fc := &fakeClaude{reply: claudeReply}
			h, _, _, _ := gatewaySetupKind(t, "anthropic", fc, 0)
			body := `{"messages":[{"role":"user","content":"x"},{"role":"assistant","content":null,"tool_calls":[{"id":"c1","type":"function","function":{"name":"lookup","arguments":"` + args + `"}}]},{"role":"tool","tool_call_id":"c1","content":"ok"}]}`
			if rec := post(h, body); rec.Code != http.StatusOK {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			sent, _ := json.Marshal(fc.body["messages"])
			if !strings.Contains(string(sent), `"input":{}`) {
				t.Fatalf("messages: %s", sent)
			}
		})
	}
}

func TestABodyThatDoesNotDecodeIsToldApartFromOneWithNoMessages(t *testing.T) {
	cases := map[string]struct{ body, sentence string }{
		"messages as a string":    {`{"messages":"hello"}`, badShapeSentence},
		"max_tokens as a string":  {`{"messages":[{"role":"user","content":"x"}],"max_tokens":"ten"}`, badShapeSentence},
		"arguments as an object":  {`{"messages":[{"role":"assistant","tool_calls":[{"id":"c","function":{"name":"n","arguments":{"q":1}}}]}]}`, badShapeSentence},
		"no messages at all":      {`{"messages":[]}`, "the request has no messages this model can take"},
		"a messages key left out": {`{"model":"x"}`, "the request has no messages this model can take"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			fc := &fakeClaude{reply: claudeReply}
			h, _, _, _ := gatewaySetupKind(t, "anthropic", fc, 0)
			rec := post(h, c.body)
			if rec.Code != http.StatusBadRequest || fc.calls != 0 || !strings.Contains(rec.Body.String(), c.sentence) {
				t.Fatalf("%d %s, calls %d", rec.Code, rec.Body, fc.calls)
			}
		})
	}
}
