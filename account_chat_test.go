package ai_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func chatAccount(name string, enabled bool, label string) ai.Account {
	a := validAccount(name)
	a.ChatEnabled, a.ChatLabel = enabled, label
	return a
}

func decodedAccount(t *testing.T, body string) ai.Account {
	t.Helper()
	var a ai.Account
	if err := json.Unmarshal([]byte(body), &a); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return a
}

const withoutChat = `{"name":"main","provider":"openai_compatible","base_url":"https://models.example.test/v1","model_name":"synthetic-model","status":"enabled"}`

func TestNewAccountIsOffTheChatWithNoLabel(t *testing.T) {
	m, _, _ := newTestManager(t)
	a := mustAdd(t, m, "ag-1", "plain")
	if a.ChatEnabled || a.ChatLabel != "" {
		t.Fatalf("created %+v", a)
	}
	got, _ := m.GetAccount(ctx, "ag-1", a.ID)
	if got.ChatEnabled || got.ChatLabel != "" {
		t.Fatalf("read %+v", got)
	}
}

func TestChatFieldsAreStoredOnCreateAndReadBackByGetAndList(t *testing.T) {
	m, _, _ := newTestManager(t)
	made, err := m.CreateAccount(ctx, "ag-1", chatAccount("baseten", true, "GLM-5.3 (Baseten)"))
	if err != nil || !made.ChatEnabled || made.ChatLabel != "GLM-5.3 (Baseten)" {
		t.Fatalf("create: %+v %v", made, err)
	}
	if _, err := m.CreateAccount(ctx, "ag-1", chatAccount("hf", false, "GLM-5.3 (Hugging Face)")); err != nil {
		t.Fatal(err)
	}
	got, err := m.GetAccount(ctx, "ag-1", made.ID)
	if err != nil || !got.ChatEnabled || got.ChatLabel != "GLM-5.3 (Baseten)" {
		t.Fatalf("get: %+v %v", got, err)
	}
	list, err := m.ListAccounts(ctx, "ag-1")
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %+v %v", list, err)
	}
	if !list[0].ChatEnabled || list[0].ChatLabel != "GLM-5.3 (Baseten)" || list[1].ChatEnabled || list[1].ChatLabel != "GLM-5.3 (Hugging Face)" {
		t.Fatalf("list lost the chat fields: %+v", list)
	}
}

func TestUpdateChangesTheChatSwitchAndTheLabel(t *testing.T) {
	m, _, _ := newTestManager(t)
	made := mustAdd(t, m, "ag-1", "main")
	on, err := m.UpdateAccount(ctx, "ag-1", made.ID, chatAccount("main", true, "Main model"))
	if err != nil || !on.ChatEnabled || on.ChatLabel != "Main model" {
		t.Fatalf("switch on: %+v %v", on, err)
	}
	read, _ := m.GetAccount(ctx, "ag-1", made.ID)
	if !read.ChatEnabled || read.ChatLabel != "Main model" {
		t.Fatalf("not stored: %+v", read)
	}
	relabelled := read
	relabelled.ChatLabel = "Renamed"
	if got, err := m.UpdateAccount(ctx, "ag-1", made.ID, relabelled); err != nil || !got.ChatEnabled || got.ChatLabel != "Renamed" {
		t.Fatalf("label only: %+v %v", got, err)
	}
	off := relabelled
	off.ChatEnabled, off.ChatLabel = false, ""
	if got, err := m.UpdateAccount(ctx, "ag-1", made.ID, off); err != nil || got.ChatEnabled || got.ChatLabel != "" {
		t.Fatalf("switch off and clear: %+v %v", got, err)
	}
	final, _ := m.GetAccount(ctx, "ag-1", made.ID)
	if final.ChatEnabled || final.ChatLabel != "" {
		t.Fatalf("not cleared: %+v", final)
	}
}

func TestLabelOfSixtyCharactersIsKeptAndSixtyOneIsRefused(t *testing.T) {
	m, _, _ := newTestManager(t)
	sixty := strings.Repeat("é", 60)
	made, err := m.CreateAccount(ctx, "ag-1", chatAccount("main", true, sixty))
	if err != nil || made.ChatLabel != sixty {
		t.Fatalf("sixty multibyte characters: %+v %v", made, err)
	}
	tooLong := strings.Repeat("a", 61)
	if _, err := m.CreateAccount(ctx, "ag-1", chatAccount("other", true, tooLong)); !errors.Is(err, ai.ErrInvalidAccount) {
		t.Fatalf("create with 61: %v", err)
	}
	if _, err := m.UpdateAccount(ctx, "ag-1", made.ID, chatAccount("main", true, tooLong)); !errors.Is(err, ai.ErrInvalidAccount) {
		t.Fatalf("update with 61: %v", err)
	}
	kept, _ := m.GetAccount(ctx, "ag-1", made.ID)
	if kept.ChatLabel != sixty {
		t.Fatalf("a refused update changed the label: %q", kept.ChatLabel)
	}
}

func TestLabelWithAControlCharacterOrBrokenUTF8IsRefused(t *testing.T) {
	m, _, _ := newTestManager(t)
	made := mustAdd(t, m, "ag-1", "main")
	for name, label := range map[string]string{
		"newline": "GLM\n5.3", "tab": "GLM\t5.3", "carriage return": "GLM\r5.3",
		"null": "GLM\x005.3", "escape": "GLM\x1b[31m", "delete": "GLM\x7f",
		"next line": "GLM\u00855.3", "broken utf-8": "GLM\xff5.3",
	} {
		if _, err := m.CreateAccount(ctx, "ag-1", chatAccount("c-"+name, true, label)); !errors.Is(err, ai.ErrInvalidAccount) {
			t.Errorf("create with %s: %v", name, err)
		}
		if _, err := m.UpdateAccount(ctx, "ag-1", made.ID, chatAccount("main", true, label)); !errors.Is(err, ai.ErrInvalidAccount) {
			t.Errorf("update with %s: %v", name, err)
		}
	}
	kept, _ := m.GetAccount(ctx, "ag-1", made.ID)
	if kept.ChatEnabled || kept.ChatLabel != "" {
		t.Fatalf("refused updates changed the account: %+v", kept)
	}
}

func TestPlainPunctuationAndSpacesInALabelAreAllowed(t *testing.T) {
	m, _, _ := newTestManager(t)
	for _, label := range []string{"GLM-5.3 (Hugging Face)", "Llama 3 · 70B", "模型 α"} {
		if _, err := m.CreateAccount(ctx, "ag-1", chatAccount("named "+label, true, label)); err != nil {
			t.Errorf("%q refused: %v", label, err)
		}
	}
}

func TestUpdateThatOmitsTheChatFieldsKeepsThem(t *testing.T) {
	m, _, _ := newTestManager(t)
	made, err := m.CreateAccount(ctx, "ag-1", chatAccount("main", true, "Main model"))
	if err != nil {
		t.Fatal(err)
	}
	changed, err := m.UpdateAccount(ctx, "ag-1", made.ID, decodedAccount(t, withoutChat))
	if err != nil || !changed.ChatEnabled || changed.ChatLabel != "Main model" {
		t.Fatalf("returned %+v %v", changed, err)
	}
	read, _ := m.GetAccount(ctx, "ag-1", made.ID)
	if !read.ChatEnabled || read.ChatLabel != "Main model" {
		t.Fatalf("stored %+v", read)
	}
}

func TestUpdateThatSendsOneChatFieldKeepsTheOther(t *testing.T) {
	m, _, _ := newTestManager(t)
	made, _ := m.CreateAccount(ctx, "ag-1", chatAccount("main", true, "Main model"))
	onlySwitch := strings.Replace(withoutChat, `"status"`, `"chat_enabled":false,"status"`, 1)
	if got, err := m.UpdateAccount(ctx, "ag-1", made.ID, decodedAccount(t, onlySwitch)); err != nil || got.ChatEnabled || got.ChatLabel != "Main model" {
		t.Fatalf("switch only: %+v %v", got, err)
	}
	onlyLabel := strings.Replace(withoutChat, `"status"`, `"chat_label":"New","status"`, 1)
	if got, err := m.UpdateAccount(ctx, "ag-1", made.ID, decodedAccount(t, onlyLabel)); err != nil || got.ChatEnabled || got.ChatLabel != "New" {
		t.Fatalf("label only: %+v %v", got, err)
	}
	both := strings.Replace(withoutChat, `"status"`, `"chat_enabled":true,"chat_label":"","status"`, 1)
	if got, err := m.UpdateAccount(ctx, "ag-1", made.ID, decodedAccount(t, both)); err != nil || !got.ChatEnabled || got.ChatLabel != "" {
		t.Fatalf("both sent, label cleared: %+v %v", got, err)
	}
}

func TestAccountBodyStillRefusesAFieldItDoesNotDeclare(t *testing.T) {
	var a ai.Account
	if err := json.Unmarshal([]byte(strings.Replace(withoutChat, `"status"`, `"surprise":1,"status"`, 1)), &a); err != nil {
		return
	}
	t.Fatalf("an undeclared field was accepted: %+v", a)
}

func TestChangingLimitsAndSettingTheKeyKeepTheChatFields(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	made, _ := m.CreateAccount(ctx, "ag-1", chatAccount("main", true, "Main model"))
	limited, err := m.SetAccountLimits(ctx, "ag-1", made.ID, ai.Limits{TokensPerDay: ip(10)})
	if err != nil || !limited.ChatEnabled || limited.ChatLabel != "Main model" {
		t.Fatalf("limits: %+v %v", limited, err)
	}
	keyed, err := m.SetAccountKey(ctx, "ag-1", made.ID, ai.KeyInput{Key: syntheticKey})
	if err != nil || !keyed.ChatEnabled || keyed.ChatLabel != "Main model" {
		t.Fatalf("key: %+v %v", keyed, err)
	}
}

func TestCreateWithoutTheChatFieldsLeavesThemOff(t *testing.T) {
	m, _, _ := newTestManager(t)
	made, err := m.CreateAccount(ctx, "ag-1", decodedAccount(t, withoutChat))
	if err != nil || made.ChatEnabled || made.ChatLabel != "" {
		t.Fatalf("%+v %v", made, err)
	}
}

func TestRowStoredBeforeTheChatFieldsReadsAsOffWithNoLabel(t *testing.T) {
	m, db, s := newTestManager(t)
	o, _ := s.ByRole("account")
	table := s.TableFor(o)
	for _, column := range []string{"chat_enabled", "chat_label"} {
		if err := db.Exec("alter table " + table + " drop column " + column).Error; err != nil {
			t.Fatalf("drop %s: %v", column, err)
		}
	}
	insert := "insert into " + table + " (id, agency_id, name, provider, base_url, model_name, status, key_set, created_at, updated_at) values (?, 'ag-old', ?, 'openai_compatible', 'https://models.example.test/v1', 'synthetic-model', 'enabled', false, 'then', 'then')"
	if err := db.Exec(insert, "old-1", "kept").Error; err != nil {
		t.Fatal(err)
	}
	for _, add := range []string{"chat_enabled boolean", "chat_label text"} {
		if err := db.Exec("alter table " + table + " add column " + add).Error; err != nil {
			t.Fatalf("add %s: %v", add, err)
		}
	}
	got, err := m.GetAccount(ctx, "ag-old", "old-1")
	if err != nil || got.Name != "kept" || got.ChatEnabled || got.ChatLabel != "" {
		t.Fatalf("get: %+v %v", got, err)
	}
	list, err := m.ListAccounts(ctx, "ag-old")
	if err != nil || len(list) != 1 || list[0].ChatEnabled || list[0].ChatLabel != "" {
		t.Fatalf("list: %+v %v", list, err)
	}
	if up, err := m.UpdateAccount(ctx, "ag-old", "old-1", decodedAccount(t, withoutChat)); err != nil || up.ChatEnabled || up.ChatLabel != "" {
		t.Fatalf("update: %+v %v", up, err)
	}
}
