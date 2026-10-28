package mcp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTheChatToolRunsAToolCallingTurnOverMCPAndNeverShowsTheKey(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"c1","function":{"name":"ai_list_roles","arguments":"{}"}}]}}],"usage":{"prompt_tokens":8,"completion_tokens":3}}`))
	}))
	t.Cleanup(provider.Close)
	cs := connect(t)
	created, _ := call(t, cs, "ai_create_account", accountArgs(map[string]any{"base_url": provider.URL}))
	id := idOf(t, created)
	call(t, cs, "ai_set_account_key", map[string]any{"agencyID": "ag-1", "accountID": id, "key": syntheticKey})

	out, isErr := call(t, cs, "ai_chat_account", map[string]any{
		"agencyID": "ag-1", "accountID": id,
		"messages": []any{map[string]any{"role": "user", "content": "Which roles are there?"}},
		"tools":    []any{map[string]any{"name": "ai_list_roles", "description": "List roles", "parameters": map[string]any{"type": "object"}}},
	})

	var got struct {
		StructuredContent struct {
			Message struct {
				Role      string `json:"role"`
				ToolCalls []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"tool_calls"`
			} `json:"message"`
			InputTokens int `json:"input_tokens"`
		} `json:"structuredContent"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	message := got.StructuredContent.Message
	if message.Role != "assistant" || len(message.ToolCalls) != 1 || message.ToolCalls[0].ID != "c1" || message.ToolCalls[0].Name != "ai_list_roles" || got.StructuredContent.InputTokens != 8 {
		t.Fatalf("%s", out)
	}
	if isErr || strings.Contains(out, syntheticKey) {
		t.Fatalf("%s", out)
	}
}

func TestTheChatToolRefusesAnAccountOfAnotherAgencyAndAMalformedConversation(t *testing.T) {
	cs := connect(t)
	created, _ := call(t, cs, "ai_create_account", accountArgs(nil))
	id := idOf(t, created)
	call(t, cs, "ai_set_account_key", map[string]any{"agencyID": "ag-1", "accountID": id, "key": syntheticKey})

	other, isErr := call(t, cs, "ai_chat_account", map[string]any{"agencyID": "ag-2", "accountID": id, "messages": []any{map[string]any{"role": "user", "content": "hi"}}})
	if !isErr || !strings.Contains(other, "account not found") {
		t.Fatalf("another agency's account: %s", other)
	}
	empty, isErr := call(t, cs, "ai_chat_account", map[string]any{"agencyID": "ag-1", "accountID": id, "messages": []any{}})
	if !isErr || !strings.Contains(empty, "messages are required") {
		t.Fatalf("no messages: %s", empty)
	}
}
