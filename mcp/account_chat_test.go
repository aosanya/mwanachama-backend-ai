package mcp_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func accountArgs(extra map[string]any) map[string]any {
	args := map[string]any{"agencyID": "ag-1", "name": "main", "provider": "openai_compatible", "base_url": "https://m.example.test/v1", "model_name": "synthetic", "status": "enabled"}
	for k, v := range extra {
		args[k] = v
	}
	return args
}

func TestChatFieldsTravelThroughTheAccountTools(t *testing.T) {
	cs := connect(t)
	out, isErr := call(t, cs, "ai_create_account", accountArgs(map[string]any{"chat_enabled": true, "chat_label": "GLM-5.3 (Hugging Face)"}))
	if isErr || !strings.Contains(out, `"chat_enabled":true`) || !strings.Contains(out, `"chat_label":"GLM-5.3 (Hugging Face)"`) {
		t.Fatalf("create: %s", out)
	}
	var created struct {
		StructuredContent struct {
			ID string `json:"id"`
		} `json:"structuredContent"`
	}
	_ = json.Unmarshal([]byte(out), &created)
	id := created.StructuredContent.ID

	changed, isErr := call(t, cs, "ai_update_account", accountArgs(map[string]any{"accountID": id, "name": "renamed"}))
	if isErr || !strings.Contains(changed, `"name":"renamed"`) || !strings.Contains(changed, `"chat_enabled":true`) || !strings.Contains(changed, `"chat_label":"GLM-5.3 (Hugging Face)"`) {
		t.Fatalf("update without the chat fields: %s", changed)
	}
	off, isErr := call(t, cs, "ai_update_account", accountArgs(map[string]any{"accountID": id, "chat_enabled": false, "chat_label": ""}))
	if isErr || !strings.Contains(off, `"chat_enabled":false`) || !strings.Contains(off, `"chat_label":""`) {
		t.Fatalf("update with the chat fields: %s", off)
	}
	read, _ := call(t, cs, "ai_get_account", map[string]any{"agencyID": "ag-1", "accountID": id})
	if !strings.Contains(read, `"chat_enabled":false`) {
		t.Fatalf("get: %s", read)
	}
	if _, isErr := call(t, cs, "ai_update_account", accountArgs(map[string]any{"accountID": id, "chat_label": strings.Repeat("a", 61)})); !isErr {
		t.Fatal("a label of 61 characters was accepted")
	}
	if _, isErr := call(t, cs, "ai_update_account", accountArgs(map[string]any{"accountID": id, "chat_label": "a\nb"})); !isErr {
		t.Fatal("a label with a newline was accepted")
	}
}
