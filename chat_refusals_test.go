package ai_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func oneQuestion() ai.ChatInput {
	return ai.ChatInput{Messages: []ai.ChatMessage{{Role: "user", Content: "hello"}}}
}

func TestChatSaysSoForAProviderThatCannotChatWithToolsYetAndAsksNothing(t *testing.T) {
	srv, seen := fakeProvider(t, 200, textAnswer)
	m, _, a := chatManager(t, "gemini", srv.URL, nil)

	_, err := m.ChatAccount(ctx, "ag-1", a.ID, oneQuestion())

	if !errors.Is(err, ai.ErrChatUnsupported) || err.Error() != "this provider cannot chat with tools yet" || seen.path != "" {
		t.Fatalf("err %v, provider saw %q", err, seen.path)
	}
}

func TestChatAsksForTheDefaultTokensAndNeverMoreThanTheCapOrTheAccountsLimit(t *testing.T) {
	cases := []struct {
		name  string
		asked int
		limit *int
		want  float64
	}{
		{"none asked", 0, nil, 1024},
		{"asked", 200, nil, 200},
		{"too many asked", 100000, nil, 4096},
		{"account limit lower", 1000, ip(50), 50},
		{"account limit higher", 30, ip(50), 30},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, seen := fakeProvider(t, 200, textAnswer)
			m, _, a := chatManager(t, "openai_compatible", srv.URL, func(a *ai.Account) { a.TokensPerCall = c.limit })
			in := oneQuestion()
			in.MaxTokens = c.asked

			if _, err := m.ChatAccount(ctx, "ag-1", a.ID, in); err != nil {
				t.Fatal(err)
			}
			if got := asked(t, seen.body)["max_tokens"]; got != c.want {
				t.Fatalf("max_tokens %v, want %v", got, c.want)
			}
		})
	}
}

func TestChatRefusesAConversationThatIsNotWellFormedAndAsksNothing(t *testing.T) {
	calls := []ai.ChatToolCall{{ID: "c", Name: "n", Arguments: json.RawMessage(`{}`)}}
	cases := map[string]ai.ChatInput{
		"no messages":              {},
		"unknown role":             {Messages: []ai.ChatMessage{{Role: "robot", Content: "x"}}},
		"empty user message":       {Messages: []ai.ChatMessage{{Role: "user", Content: "  "}}},
		"tool without its id":      {Messages: []ai.ChatMessage{{Role: "user", Content: "x"}, {Role: "tool", Content: "r"}}},
		"empty assistant message":  {Messages: []ai.ChatMessage{{Role: "assistant"}}},
		"call without a name":      {Messages: []ai.ChatMessage{{Role: "assistant", ToolCalls: []ai.ChatToolCall{{ID: "c"}}}}},
		"calls on a user message":  {Messages: []ai.ChatMessage{{Role: "user", Content: "x", ToolCalls: calls}}},
		"arguments not an object":  {Messages: []ai.ChatMessage{{Role: "assistant", ToolCalls: []ai.ChatToolCall{{ID: "c", Name: "n", Arguments: json.RawMessage(`[1]`)}}}}},
		"negative max tokens":      {Messages: oneQuestion().Messages, MaxTokens: -1},
		"tool without a name":      {Messages: oneQuestion().Messages, Tools: []ai.ChatTool{{}}},
		"the same tool twice":      {Messages: oneQuestion().Messages, Tools: []ai.ChatTool{{Name: "n"}, {Name: "n"}}},
		"far too many messages":    {Messages: make([]ai.ChatMessage, 201)},
		"a conversation too large": {Messages: []ai.ChatMessage{{Role: "user", Content: strings.Repeat("x", 1<<20+1)}}},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			srv, seen := fakeProvider(t, 200, textAnswer)
			m, _, a := chatManager(t, "openai_compatible", srv.URL, nil)

			_, err := m.ChatAccount(ctx, "ag-1", a.ID, in)

			if !errors.Is(err, ai.ErrInvalidChat) || seen.path != "" {
				t.Fatalf("err %v, provider saw %q", err, seen.path)
			}
		})
	}
}

func TestChatNeedsAKeyAndAMasterKeyAndAnAccountThatIsOn(t *testing.T) {
	srv, seen := fakeProvider(t, 200, textAnswer)
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	keyless, err := m.CreateAccount(ctx, "ag-1", ai.Account{Name: "k", Provider: "openai_compatible", BaseURL: srv.URL, ModelName: "m", Status: "enabled"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.ChatAccount(ctx, "ag-1", keyless.ID, oneQuestion()); !errors.Is(err, ai.ErrNoKey) {
		t.Fatalf("no key: %v", err)
	}
	bare, _, _ := newTestManager(t)
	bareAccount, _ := bare.CreateAccount(ctx, "ag-1", ai.Account{Name: "b", Provider: "openai_compatible", BaseURL: srv.URL, ModelName: "m", Status: "enabled"})
	if _, err := bare.ChatAccount(ctx, "ag-1", bareAccount.ID, oneQuestion()); !errors.Is(err, ai.ErrNoMasterKey) {
		t.Fatalf("no master key: %v", err)
	}
	off, _, offAccount := chatManager(t, "openai_compatible", srv.URL, func(a *ai.Account) { a.Status = "disabled" })
	if _, err := off.ChatAccount(ctx, "ag-1", offAccount.ID, oneQuestion()); !errors.Is(err, ai.ErrAccountOff) {
		t.Fatalf("turned off: %v", err)
	}
	if seen.path != "" {
		t.Fatalf("the provider was asked %q", seen.path)
	}
}

func TestChatCountsEachAnswerAndStopsAtTheAccountsLimit(t *testing.T) {
	srv, seen := fakeProvider(t, 200, textAnswer)
	m, _, a := chatManager(t, "openai_compatible", srv.URL, func(a *ai.Account) { a.RequestsPerDay = ip(1) })

	if _, err := m.ChatAccount(ctx, "ag-1", a.ID, oneQuestion()); err != nil {
		t.Fatal(err)
	}
	seen.path = ""
	_, err := m.ChatAccount(ctx, "ag-1", a.ID, oneQuestion())

	if !errors.Is(err, ai.ErrLimitReached) || !strings.Contains(err.Error(), "daily request limit is reached") || seen.path != "" {
		t.Fatalf("err %v, provider saw %q", err, seen.path)
	}
	report, _ := m.ListUsage(ctx, "ag-1", a.ID)
	used := map[string]int{}
	for _, c := range report.Counts {
		used[c.Limit] = c.Used
	}
	if used["requests per day"] != 1 || used["daily token"] != 18 {
		t.Fatalf("usage %v", used)
	}
}

func TestChatCountsNothingForAnswerTheModelFailed(t *testing.T) {
	srv, _ := fakeProvider(t, 500, "down")
	m, _, a := chatManager(t, "openai_compatible", srv.URL, nil)

	_, _ = m.ChatAccount(ctx, "ag-1", a.ID, oneQuestion())

	report, _ := m.ListUsage(ctx, "ag-1", a.ID)
	for _, c := range report.Counts {
		if c.Used != 0 {
			t.Fatalf("%s used %d", c.Limit, c.Used)
		}
	}
}
