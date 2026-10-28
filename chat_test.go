package ai_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"gorm.io/gorm"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

const textAnswer = `{"choices":[{"message":{"content":"None yet."}}],"usage":{"prompt_tokens":11,"completion_tokens":7}}`

func chatManager(t *testing.T, provider, baseURL string, patch func(*ai.Account), opts ...ai.Option) (ai.Manager, *gorm.DB, ai.Account) {
	t.Helper()
	m, db, _ := newTestManager(t, append([]ai.Option{ai.WithMasterKey(master)}, opts...)...)
	account := ai.Account{Name: "chat", Provider: provider, BaseURL: baseURL, ModelName: "synthetic-model", Status: "enabled"}
	if patch != nil {
		patch(&account)
	}
	made, err := m.CreateAccount(ctx, "ag-1", account)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := m.SetAccountKey(ctx, "ag-1", made.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
		t.Fatalf("SetAccountKey: %v", err)
	}
	return m, db, made
}

func asked(t *testing.T, body string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("the provider got %q: %v", body, err)
	}
	return out
}

func listRolesTool() ai.ChatTool {
	return ai.ChatTool{
		Name: "ai_list_roles", Description: "List roles",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{"limit": map[string]any{"type": "integer"}}},
	}
}

func listRolesConversation() ai.ChatInput {
	return ai.ChatInput{
		Messages: []ai.ChatMessage{
			{Role: "system", Content: "You help hub managers."},
			{Role: "user", Content: "Which agents use Muse Spark?"},
			{Role: "assistant", ToolCalls: []ai.ChatToolCall{{ID: "call-9", Name: "ai_list_roles", Arguments: json.RawMessage(`{"limit":5}`)}}},
			{Role: "tool", ToolCallID: "call-9", Content: `{"items":[]}`},
		},
		Tools: []ai.ChatTool{listRolesTool()},
	}
}

func TestChatSendsTheConversationAndToolsInTheOpenAIWireFormat(t *testing.T) {
	srv, seen := fakeProvider(t, 200, textAnswer)
	m, _, a := chatManager(t, "openai_compatible", srv.URL, nil)

	got, err := m.ChatAccount(ctx, "ag-1", a.ID, listRolesConversation())
	if err != nil {
		t.Fatal(err)
	}

	sent := asked(t, seen.body)
	if seen.path != "/chat/completions" || seen.headers.Get("Authorization") != "Bearer "+syntheticKey {
		t.Fatalf("path %q auth %q", seen.path, seen.headers.Get("Authorization"))
	}
	if sent["model"] != "synthetic-model" || sent["max_tokens"] != float64(1024) {
		t.Fatalf("model and limit: %v", sent)
	}
	want := `[{"role":"system","content":"You help hub managers."},
	{"role":"user","content":"Which agents use Muse Spark?"},
	{"role":"assistant","content":null,"tool_calls":[{"id":"call-9","type":"function","function":{"name":"ai_list_roles","arguments":"{\"limit\":5}"}}]},
	{"role":"tool","tool_call_id":"call-9","content":"{\"items\":[]}"}]`
	if !sameJSON(t, sent["messages"], want) {
		t.Fatalf("messages: %v", sent["messages"])
	}
	wantTools := `[{"type":"function","function":{"name":"ai_list_roles","description":"List roles","parameters":{"type":"object","properties":{"limit":{"type":"integer"}}}}}]`
	if !sameJSON(t, sent["tools"], wantTools) {
		t.Fatalf("tools: %v", sent["tools"])
	}
	if got.Message.Role != "assistant" || got.Message.Content != "None yet." || len(got.Message.ToolCalls) != 0 || got.InputTokens != 11 || got.OutputTokens != 7 {
		t.Fatalf("reply %+v", got)
	}
}

func sameJSON(t *testing.T, got any, want string) bool {
	t.Helper()
	var expected any
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatal(err)
	}
	return reflect.DeepEqual(got, expected)
}

func TestChatHandsBackTheToolCallsTheModelAsksFor(t *testing.T) {
	answer := `{"choices":[{"message":{"content":null,"tool_calls":[{"id":"call-1","function":{"name":"ai_list_roles","arguments":"{\"limit\":2}"}}]}}],"usage":{"prompt_tokens":5,"completion_tokens":3}}`
	srv, _ := fakeProvider(t, 200, answer)
	m, _, a := chatManager(t, "openai_compatible", srv.URL, nil)

	got, err := m.ChatAccount(ctx, "ag-1", a.ID, ai.ChatInput{Messages: []ai.ChatMessage{{Role: "user", Content: "list"}}, Tools: []ai.ChatTool{listRolesTool()}})
	if err != nil {
		t.Fatal(err)
	}

	out, _ := json.Marshal(got)
	want := `{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call-1","name":"ai_list_roles","arguments":{"limit":2}}]},"input_tokens":5,"output_tokens":3}`
	if !sameJSON(t, asked(t, string(out)), want) {
		t.Fatalf("reply %s", out)
	}
}

func TestChatGivesAToolCallWithoutAnIdOneAndMakesBadArgumentsAnEmptyObject(t *testing.T) {
	answer := `{"choices":[{"message":{"content":"","tool_calls":[{"function":{"name":"a","arguments":"not json"}},{"function":{"name":"b","arguments":""}}]}}]}`
	srv, _ := fakeProvider(t, 200, answer)
	m, _, a := chatManager(t, "openai_compatible", srv.URL, nil)

	got, err := m.ChatAccount(ctx, "ag-1", a.ID, ai.ChatInput{Messages: []ai.ChatMessage{{Role: "user", Content: "go"}}})
	if err != nil {
		t.Fatal(err)
	}

	calls := got.Message.ToolCalls
	if len(calls) != 2 || calls[0].ID != "call-1" || calls[1].ID != "call-2" || string(calls[0].Arguments) != "{}" || string(calls[1].Arguments) != "{}" {
		t.Fatalf("calls %+v", calls)
	}
	if _, err := json.Marshal(got); err != nil {
		t.Fatalf("the reply cannot be written out: %v", err)
	}
}

func TestChatSendsTheConversationAndToolsInTheAnthropicWireFormat(t *testing.T) {
	answer := `{"content":[{"type":"text","text":"Checking."},{"type":"tool_use","id":"tu-1","name":"ai_list_roles","input":{"limit":1}}],"usage":{"input_tokens":9,"output_tokens":4}}`
	srv, seen := fakeProvider(t, 200, answer)
	m, _, a := chatManager(t, "anthropic", srv.URL, nil)
	in := ai.ChatInput{
		Messages: []ai.ChatMessage{
			{Role: "system", Content: "You help hub managers."},
			{Role: "user", Content: "Which agents use Muse Spark?"},
			{Role: "assistant", Content: "Let me look.", ToolCalls: []ai.ChatToolCall{
				{ID: "tu-8", Name: "ai_list_roles", Arguments: json.RawMessage(`{"limit":5}`)},
				{ID: "tu-9", Name: "ai_list_roles", Arguments: json.RawMessage(`{}`)},
			}},
			{Role: "tool", ToolCallID: "tu-8", Content: "first"},
			{Role: "tool", ToolCallID: "tu-9", Content: "second"},
		},
		Tools: []ai.ChatTool{listRolesTool()},
	}

	got, err := m.ChatAccount(ctx, "ag-1", a.ID, in)
	if err != nil {
		t.Fatal(err)
	}

	sent := asked(t, seen.body)
	if seen.path != "/v1/messages" || seen.headers.Get("x-api-key") != syntheticKey || sent["system"] != "You help hub managers." {
		t.Fatalf("path %q key %q system %v", seen.path, seen.headers.Get("x-api-key"), sent["system"])
	}
	want := `[{"role":"user","content":"Which agents use Muse Spark?"},
	{"role":"assistant","content":[{"type":"text","text":"Let me look."},{"type":"tool_use","id":"tu-8","name":"ai_list_roles","input":{"limit":5}},{"type":"tool_use","id":"tu-9","name":"ai_list_roles","input":{}}]},
	{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu-8","content":"first"},{"type":"tool_result","tool_use_id":"tu-9","content":"second"}]}]`
	if !sameJSON(t, sent["messages"], want) {
		t.Fatalf("messages: %v", sent["messages"])
	}
	wantTools := `[{"name":"ai_list_roles","description":"List roles","input_schema":{"type":"object","properties":{"limit":{"type":"integer"}}}}]`
	if !sameJSON(t, sent["tools"], wantTools) {
		t.Fatalf("tools: %v", sent["tools"])
	}
	if got.Message.Content != "Checking." || len(got.Message.ToolCalls) != 1 || got.Message.ToolCalls[0].ID != "tu-1" || got.InputTokens != 9 {
		t.Fatalf("reply %+v", got)
	}
}
