package ai_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"gorm.io/gorm"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func TestChatAnswersWithAReasonAndKeepsTheKeyOutOfEveryFailure(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
		reason string
	}{
		{"key refused", 401, "bad key " + syntheticKey, ai.ErrModelRefused, "the model refused the key"},
		{"forbidden", 403, "Bearer " + syntheticKey, ai.ErrModelRefused, "the model refused the key"},
		{"provider error", 500, "boom " + syntheticKey, ai.ErrModelError, "the model answered with an error"},
		{"answer unreadable", 200, "not json " + syntheticKey, ai.ErrModelError, "the model answered with an error"},
		{"answer without choices", 200, `{"choices":[]}`, ai.ErrModelError, "the model answered with an error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			logged := captureLogs(t)
			srv, _ := fakeProvider(t, c.status, c.body)
			m, _, a := chatManager(t, "openai_compatible", srv.URL, nil)

			got, err := m.ChatAccount(ctx, "ag-1", a.ID, oneQuestion())

			if !errors.Is(err, c.want) || err.Error() != c.reason {
				t.Fatalf("err %v", err)
			}
			assertNoKey(t, got, err, logged)
		})
	}
}

func TestChatAnswersTheModelCouldNotBeReachedAndKeepsTheKeyOut(t *testing.T) {
	logged := captureLogs(t)
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	m, _, a := chatManager(t, "openai_compatible", url, nil)

	got, err := m.ChatAccount(ctx, "ag-1", a.ID, oneQuestion())

	if !errors.Is(err, ai.ErrModelUnreached) || err.Error() != "the model could not be reached" {
		t.Fatalf("err %v", err)
	}
	assertNoKey(t, got, err, logged)
}

func TestChatNeverShowsTheKeyOnASuccessEither(t *testing.T) {
	logged := captureLogs(t)
	srv, _ := fakeProvider(t, 200, textAnswer)
	m, _, a := chatManager(t, "openai_compatible", srv.URL, nil)

	got, err := m.ChatAccount(ctx, "ag-1", a.ID, listRolesConversation())

	if err != nil {
		t.Fatal(err)
	}
	assertNoKey(t, got, err, logged)
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		slog.SetDefault(previous)
	})
	return &buf
}

func assertNoKey(t *testing.T, reply ai.ChatReply, err error, logged *bytes.Buffer) {
	t.Helper()
	out, _ := json.Marshal(reply)
	text := string(out)
	if err != nil {
		text += err.Error() + fmt.Sprintf("%v %+v", err, err)
	}
	text += logged.String()
	if strings.Contains(text, syntheticKey) || strings.Contains(text, "SYNTHETIC") {
		t.Fatalf("the key leaked: %s", text)
	}
}

func TestChatKeepsNoTextOfTheConversationAnywhere(t *testing.T) {
	answer := `{"choices":[{"message":{"content":"REPLY-text-77c1"}}],"usage":{"prompt_tokens":4,"completion_tokens":2}}`
	srv, _ := fakeProvider(t, 200, answer)
	m, db, a := chatManager(t, "openai_compatible", srv.URL, nil)
	before, _ := m.GetAccount(ctx, "ag-1", a.ID)
	in := ai.ChatInput{
		Messages: []ai.ChatMessage{
			{Role: "system", Content: "SYSTEM-text-77c1"},
			{Role: "user", Content: "USER-text-77c1"},
			{Role: "assistant", ToolCalls: []ai.ChatToolCall{{ID: "CALL-77c1", Name: "TOOL-77c1", Arguments: json.RawMessage(`{"q":"ARGS-77c1"}`)}}},
			{Role: "tool", ToolCallID: "CALL-77c1", Content: "RESULT-text-77c1"},
		},
		Tools: []ai.ChatTool{{Name: "TOOL-77c1", Description: "DESC-77c1"}},
	}

	if _, err := m.ChatAccount(ctx, "ag-1", a.ID, in); err != nil {
		t.Fatal(err)
	}

	held, rows := everythingStored(t, db)
	if rows == 0 || !strings.Contains(held, "chat") {
		t.Fatalf("the scan read %d rows and did not find the account", rows)
	}
	if strings.Contains(held, "77c1") {
		t.Fatalf("conversation text was stored: %s", held)
	}
	after, _ := m.GetAccount(ctx, "ag-1", a.ID)
	if after != before {
		t.Fatalf("the account changed: %+v then %+v", before, after)
	}
}

func everythingStored(t *testing.T, db *gorm.DB) (string, int) {
	t.Helper()
	tables, err := db.Migrator().GetTables()
	if err != nil {
		t.Fatal(err)
	}
	var all strings.Builder
	rows := 0
	for _, table := range tables {
		var found []map[string]any
		if err := db.Table(table).Find(&found).Error; err != nil {
			t.Fatal(err)
		}
		rows += len(found)
		all.WriteString(fmt.Sprint(found))
	}
	return all.String(), rows
}

func TestChatAnswersAccountNotFoundForAnAccountOfAnotherInstance(t *testing.T) {
	srv, seen := fakeProvider(t, 200, textAnswer)
	m, _, a := chatManager(t, "openai_compatible", srv.URL, nil)
	other, err := m.CreateAccount(ctx, "ag-2", ai.Account{Name: "theirs", Provider: "openai_compatible", BaseURL: srv.URL, ModelName: "m", Status: "enabled"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetAccountKey(ctx, "ag-2", other.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{a.ID, other.ID} {
		_, err := m.ChatAccount(ctx, "ag-3", id, oneQuestion())

		if !errors.Is(err, ai.ErrAccountNotFound) || err.Error() != "account not found" {
			t.Fatalf("account %s from another instance: %v", id, err)
		}
	}
	if _, err := m.ChatAccount(ctx, "ag-2", a.ID, oneQuestion()); !errors.Is(err, ai.ErrAccountNotFound) {
		t.Fatalf("ag-1's account asked as ag-2: %v", err)
	}
	if seen.path != "" {
		t.Fatalf("the provider was asked %q", seen.path)
	}
}
