package mcp_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	gosdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	ai "github.com/aosanya/mwanachama-backend-ai"
	aimcp "github.com/aosanya/mwanachama-backend-ai/mcp"
)

const syntheticKey = "sk-SYNTHETIC-mcp-7a6b5c4d"

func connect(t *testing.T) *gosdkmcp.ClientSession {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	s, err := ai.LoadSpec("../ai.wakala.json")
	if err != nil {
		t.Fatal(err)
	}
	s.Instance = "mcptest"
	if err := ai.Provision(db, s); err != nil {
		t.Fatal(err)
	}
	m, err := ai.NewManager(db, s, ai.WithMasterKey([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	server := gosdkmcp.NewServer(&gosdkmcp.Implementation{Name: "test", Version: "0.0.0"}, nil)
	aimcp.RegisterTools(server, m)
	ct, st := gosdkmcp.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := gosdkmcp.NewClient(&gosdkmcp.Implementation{Name: "c", Version: "0.0.0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func call(t *testing.T, cs *gosdkmcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &gosdkmcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(res)
	return string(raw), res.IsError
}

func TestToolsListAndKeyNeverReturned(t *testing.T) {
	cs := connect(t)
	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, tool := range list.Tools {
		have[tool.Name] = true
	}
	for _, want := range []string{"ai_create_account", "ai_list_accounts", "ai_get_account", "ai_update_account", "ai_delete_account", "ai_set_account_key", "ai_set_limits", "ai_check_limits", "ai_list_usage", "ai_record_usage", "ai_test_account"} {
		if !have[want] {
			t.Fatalf("missing tool %s in %v", want, have)
		}
	}
	schemas, _ := json.Marshal(list)
	if strings.Contains(string(schemas), syntheticKey) {
		t.Fatal("key in tools/list")
	}

	out, isErr := call(t, cs, "ai_create_account", map[string]any{"agencyID": "ag-1", "name": "main", "provider": "openai_compatible", "base_url": "https://m.example.test/v1", "model_name": "synthetic", "status": "enabled"})
	if isErr {
		t.Fatalf("create: %s", out)
	}
	var created struct {
		StructuredContent struct {
			ID string `json:"id"`
		} `json:"structuredContent"`
	}
	_ = json.Unmarshal([]byte(out), &created)
	id := created.StructuredContent.ID
	if id == "" {
		t.Fatalf("no id: %s", out)
	}
	outputs := []string{out}
	for _, c := range []struct {
		name string
		args map[string]any
	}{
		{"ai_set_account_key", map[string]any{"agencyID": "ag-1", "accountID": id, "key": syntheticKey}},
		{"ai_set_limits", map[string]any{"agencyID": "ag-1", "accountID": id, "tokens_per_day": 5}},
		{"ai_get_account", map[string]any{"agencyID": "ag-1", "accountID": id}},
		{"ai_list_accounts", map[string]any{"agencyID": "ag-1"}},
		{"ai_update_account", map[string]any{"agencyID": "ag-1", "accountID": id, "name": "main", "provider": "openai_compatible", "base_url": "https://m.example.test/v1", "model_name": "synthetic", "status": "enabled"}},
		{"ai_set_account_key", map[string]any{"agencyID": "ag-2", "accountID": id, "key": syntheticKey}},
		{"ai_set_limits", map[string]any{"agencyID": "ag-2", "accountID": id, "tokens_per_day": 5}},
	} {
		o, _ := call(t, cs, c.name, c.args)
		outputs = append(outputs, o)
	}
	for _, o := range outputs {
		if strings.Contains(o, syntheticKey) {
			t.Fatalf("key leaked: %s", o)
		}
	}
	if !strings.Contains(outputs[2], `"tokens_per_day":5`) {
		t.Fatalf("set_limits result: %s", outputs[2])
	}
	if _, isErr := call(t, cs, "ai_set_limits", map[string]any{"agencyID": "ag-2", "accountID": id, "tokens_per_day": 5}); !isErr {
		t.Fatal("another agency's account was changed")
	}
}

func TestSetLimitsAcceptsNullAndWeekly(t *testing.T) {
	cs := connect(t)
	out, isErr := call(t, cs, "ai_create_account", map[string]any{"agencyID": "ag-1", "name": "weekly", "provider": "openai_compatible", "base_url": "https://m.example.test/v1", "model_name": "synthetic", "status": "enabled"})
	if isErr {
		t.Fatalf("create: %s", out)
	}
	var created struct {
		StructuredContent struct {
			ID string `json:"id"`
		} `json:"structuredContent"`
	}
	_ = json.Unmarshal([]byte(out), &created)
	args := map[string]any{"agencyID": "ag-1", "accountID": created.StructuredContent.ID, "tokens_per_day": nil, "requests_per_minute": nil, "tokens_per_week": 700, "requests_per_week": 40}
	got, isErr := call(t, cs, "ai_set_limits", args)
	if isErr {
		t.Fatalf("set_limits: %s", got)
	}
	for _, want := range []string{`"tokens_per_week":700`, `"requests_per_week":40`, `"tokens_per_day":null`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
}
