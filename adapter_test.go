package ai_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

type seenRequest struct {
	path    string
	rawURL  string
	headers http.Header
	body    string
}

func fakeProvider(t *testing.T, status int, answer string) (*httptest.Server, *seenRequest) {
	t.Helper()
	seen := &seenRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen.path, seen.rawURL, seen.headers, seen.body = r.URL.Path, r.URL.String(), r.Header.Clone(), string(b)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(srv.Close)
	return srv, seen
}

var weatherTool = ai.Tool{Name: "lookup", Description: "d", Parameters: map[string]any{"type": "object"}}

const (
	openAIAnswer    = `{"choices":[{"message":{"content":"hi","tool_calls":[{"id":"c1","function":{"name":"lookup","arguments":"{\"q\":1}"}}]}}],"usage":{"prompt_tokens":11,"completion_tokens":7}}`
	anthropicAnswer = `{"content":[{"type":"text","text":"hi"},{"type":"tool_use","id":"c1","name":"lookup","input":{"q":1}}],"usage":{"input_tokens":11,"output_tokens":7}}`
	geminiAnswer    = `{"candidates":[{"content":{"parts":[{"text":"hi"},{"functionCall":{"name":"lookup","args":{"q":1}}}]}}],"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":7}}`
)

func TestAdaptersRoundTrip(t *testing.T) {
	cases := []struct {
		kind, answer, path, keyHeader string
		keyValue                      string
	}{
		{"openai_compatible", openAIAnswer, "/chat/completions", "Authorization", "Bearer " + syntheticKey},
		{"anthropic", anthropicAnswer, "/v1/messages", "x-api-key", syntheticKey},
		{"gemini", geminiAnswer, "/v1beta/models/synthetic-model:generateContent", "x-goog-api-key", syntheticKey},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			srv, seen := fakeProvider(t, 200, c.answer)
			acct := ai.Account{Provider: c.kind, BaseURL: srv.URL + "/", ModelName: "synthetic-model"}
			reply, out := ai.Call(ctx, srv.Client(), acct, syntheticKey, ai.Request{Prompt: "hello", Tools: []ai.Tool{weatherTool}, MaxTokens: 16})
			if out != ai.ProbeOK {
				t.Fatalf("outcome %v", out)
			}
			if seen.path != c.path {
				t.Fatalf("path %q", seen.path)
			}
			if got := seen.headers.Get(c.keyHeader); got != c.keyValue {
				t.Fatalf("key header %q", got)
			}
			if strings.Contains(seen.rawURL, syntheticKey) || strings.Contains(seen.body, syntheticKey) {
				t.Fatal("key reached the url or body")
			}
			if !strings.Contains(seen.body, "hello") || !strings.Contains(seen.body, "lookup") {
				t.Fatalf("body %s", seen.body)
			}
			if reply.Text != "hi" || reply.InputTokens != 11 || reply.OutputTokens != 7 {
				t.Fatalf("reply %+v", reply)
			}
			if len(reply.ToolCalls) != 1 || reply.ToolCalls[0].Name != "lookup" {
				t.Fatalf("tool calls %+v", reply.ToolCalls)
			}
			var args map[string]int
			if err := json.Unmarshal(reply.ToolCalls[0].Arguments, &args); err != nil || args["q"] != 1 {
				t.Fatalf("arguments %s", reply.ToolCalls[0].Arguments)
			}
		})
	}
}

func TestAnthropicSendsItsVersionHeader(t *testing.T) {
	srv, seen := fakeProvider(t, 200, anthropicAnswer)
	acct := ai.Account{Provider: "anthropic", BaseURL: srv.URL, ModelName: "m"}
	ai.Call(ctx, srv.Client(), acct, syntheticKey, ai.Request{Prompt: "x", MaxTokens: 1})
	if seen.headers.Get("anthropic-version") == "" {
		t.Fatal("no anthropic-version header")
	}
	if seen.headers.Get("Authorization") != "" {
		t.Fatal("bearer sent to anthropic")
	}
}

func TestEachKindMapsStatusesToOutcomes(t *testing.T) {
	for _, kind := range []string{"openai_compatible", "anthropic", "gemini"} {
		for status, want := range map[int]ai.ProbeOutcome{401: ai.ProbeRefused, 403: ai.ProbeRefused, 500: ai.ProbeError, 429: ai.ProbeError} {
			srv, _ := fakeProvider(t, status, `{"error":"`+syntheticKey+`"}`)
			acct := ai.Account{Provider: kind, BaseURL: srv.URL, ModelName: "m"}
			if _, out := ai.Call(ctx, srv.Client(), acct, syntheticKey, ai.Request{Prompt: "x", MaxTokens: 1}); out != want {
				t.Fatalf("%s %d: %v", kind, status, out)
			}
		}
	}
}

func TestUnreachedAndUnparseable(t *testing.T) {
	srv, _ := fakeProvider(t, 200, `not json`)
	for _, kind := range []string{"openai_compatible", "anthropic", "gemini"} {
		acct := ai.Account{Provider: kind, BaseURL: srv.URL, ModelName: "m"}
		if _, out := ai.Call(ctx, srv.Client(), acct, "k", ai.Request{Prompt: "x", MaxTokens: 1}); out != ai.ProbeError {
			t.Fatalf("%s unparseable: %v", kind, out)
		}
	}
	dead := httptest.NewServer(http.NotFoundHandler())
	url := dead.URL
	dead.Close()
	acct := ai.Account{Provider: "gemini", BaseURL: url, ModelName: "m"}
	if _, out := ai.Call(ctx, http.DefaultClient, acct, "k", ai.Request{Prompt: "x", MaxTokens: 1}); out != ai.ProbeUnreached {
		t.Fatalf("unreached: %v", out)
	}
}

func TestAccountTestRunsThroughTheKindsAdapter(t *testing.T) {
	for kind, answer := range map[string]string{"anthropic": anthropicAnswer, "gemini": geminiAnswer} {
		srv, seen := fakeProvider(t, 200, answer)
		m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithProber(ai.NewHTTPProber()))
		a := ai.Account{Name: kind, Provider: kind, BaseURL: srv.URL, ModelName: "m", Status: "enabled"}
		made, err := m.CreateAccount(ctx, "ag-1", a)
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if _, err := m.SetAccountKey(ctx, "ag-1", made.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
			t.Fatal(err)
		}
		got, err := m.TestAccount(ctx, "ag-1", made.ID)
		if err != nil || got.LastTestResult != "working" {
			t.Fatalf("%s: %+v %v", kind, got, err)
		}
		b, _ := json.Marshal(got)
		if strings.Contains(string(b), syntheticKey) || seen.path == "" {
			t.Fatalf("%s: leak or no call", kind)
		}
	}
}
