package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
	"github.com/aosanya/mwanachama-backend-ai/routes"
)

func providerAnswering(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func keyedAccountOn(t *testing.T, srv *httptest.Server, providerURL string) string {
	t.Helper()
	_, body := do(t, srv, "POST", "/agencies/ag-1/accounts", `{"name":"main","provider":"openai_compatible","base_url":"`+providerURL+`","model_name":"synthetic","status":"enabled"}`)
	id := between(body, `"id":"`, `"`)
	do(t, srv, "PUT", "/agencies/ag-1/accounts/"+id+"/key", `{"key":"`+syntheticKey+`"}`)
	return id
}

const askedOnce = `{"messages":[{"role":"user","content":"hello"}]}`

func TestChatRouteReturnsTheModelsNextMessageAndTokens(t *testing.T) {
	provider := providerAnswering(t, 200, `{"choices":[{"message":{"content":"Hi there","tool_calls":[{"id":"c1","function":{"name":"lookup","arguments":"{\"q\":1}"}}]}}],"usage":{"prompt_tokens":6,"completion_tokens":2}}`)
	srv := newServer(t)
	id := keyedAccountOn(t, srv, provider.URL)

	code, out := do(t, srv, "POST", "/agencies/ag-1/accounts/"+id+"/chat", askedOnce)

	want := `{"message":{"role":"assistant","content":"Hi there","tool_calls":[{"id":"c1","name":"lookup","arguments":{"q":1}}]},"input_tokens":6,"output_tokens":2}`
	var got, expected any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%d %s", code, out)
	}
	_ = json.Unmarshal([]byte(want), &expected)
	if code != 200 || !jsonEqual(got, expected) || strings.Contains(out, syntheticKey) {
		t.Fatalf("%d %s", code, out)
	}
}

func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func TestChatRouteAnswersEachRefusalWithItsStatusAndSentence(t *testing.T) {
	cases := []struct {
		name     string
		provider int
		body     string
		want     int
		sentence string
	}{
		{"model refuses the key", 401, "", 502, "the model refused the key"},
		{"model fails", 500, "down", 502, "the model answered with an error"},
		{"conversation malformed", 200, "", 400, "invalid chat request: messages are required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider := providerAnswering(t, c.provider, c.body)
			srv := newServer(t)
			id := keyedAccountOn(t, srv, provider.URL)
			body := askedOnce
			if c.want == 400 {
				body = `{"messages":[]}`
			}

			code, out := do(t, srv, "POST", "/agencies/ag-1/accounts/"+id+"/chat", body)

			if code != c.want || !strings.Contains(out, c.sentence) || strings.Contains(out, syntheticKey) {
				t.Fatalf("%d %s", code, out)
			}
		})
	}
}

func TestChatRouteAnswersNotFoundForAnotherInstancesAccount(t *testing.T) {
	provider := providerAnswering(t, 200, `{"choices":[{"message":{"content":"x"}}]}`)
	srv := newServer(t)
	id := keyedAccountOn(t, srv, provider.URL)

	code, out := do(t, srv, "POST", "/agencies/ag-2/accounts/"+id+"/chat", askedOnce)

	if code != 404 || !strings.Contains(out, "account not found") {
		t.Fatalf("%d %s", code, out)
	}
}

func TestChatToolTakesTheAccountAndTheConversationAndNothingElse(t *testing.T) {
	for _, tool := range routes.Tools(struct{ ai.Manager }{}) {
		if tool.Name != "ai_chat_account" {
			continue
		}
		var schema struct {
			Properties           map[string]json.RawMessage `json:"properties"`
			Required             []string                   `json:"required"`
			AdditionalProperties bool                       `json:"additionalProperties"`
		}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Fatal(err)
		}
		names := []string{}
		for name := range schema.Properties {
			names = append(names, name)
		}
		if tool.Action != "ai.account.chat" || len(names) != 5 || schema.AdditionalProperties || strings.Join(schema.Required, ",") != "accountID,agencyID" {
			t.Fatalf("%s %v required %v", tool.Action, names, schema.Required)
		}
		for _, want := range []string{"accountID", "agencyID", "messages", "tools", "max_tokens"} {
			if _, ok := schema.Properties[want]; !ok {
				t.Fatalf("no %s in %v", want, names)
			}
		}
		return
	}
	t.Fatal("no ai_chat_account tool")
}
