package routes_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTestAccountRouteReturnsThePromptTheReplyAndTheTimeButNeverTheKey(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"  OK\n"}}]}`))
	}))
	t.Cleanup(provider.Close)
	srv := newServer(t)
	_, body := do(t, srv, "POST", "/agencies/ag-1/accounts", `{"name":"main","provider":"openai_compatible","base_url":"`+provider.URL+`","model_name":"synthetic","status":"enabled"}`)
	id := between(body, `"id":"`, `"`)
	do(t, srv, "PUT", "/agencies/ag-1/accounts/"+id+"/key", `{"key":"`+syntheticKey+`"}`)
	code, out := do(t, srv, "POST", "/agencies/ag-1/accounts/"+id+"/test", "{}")
	if code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	for _, want := range []string{`"last_test_result":"working"`, `"last_test_prompt":"Reply with the single word OK."`, `"last_test_reply":"OK"`, `"last_test_ms":`} {
		if !strings.Contains(out, want) {
			t.Errorf("%s missing from %s", want, out)
		}
	}
	for _, path := range []string{"/agencies/ag-1/accounts/" + id, "/agencies/ag-1/accounts"} {
		_, got := do(t, srv, "GET", path, "")
		if !strings.Contains(got, `"last_test_reply":"OK"`) || strings.Contains(got, syntheticKey) {
			t.Errorf("GET %s: %s", path, got)
		}
	}
	if strings.Contains(out, syntheticKey) {
		t.Fatalf("key in %s", out)
	}
}

func TestTestAccountRouteUsesATypedKeyWhenTheBodyCarriesOne(t *testing.T) {
	const typed = "sk-TYPED-route-not-saved-5a4b3c2d"
	var bearers []string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearers = append(bearers, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"OK"}}]}`))
	}))
	t.Cleanup(provider.Close)
	srv := newServer(t)
	_, body := do(t, srv, "POST", "/agencies/ag-1/accounts", `{"name":"main","provider":"openai_compatible","base_url":"`+provider.URL+`","model_name":"synthetic","status":"enabled"}`)
	id := between(body, `"id":"`, `"`)
	do(t, srv, "PUT", "/agencies/ag-1/accounts/"+id+"/key", `{"key":"`+syntheticKey+`"}`)
	path := "/agencies/ag-1/accounts/" + id + "/test"
	for _, c := range []struct{ body, want string }{{`{"key":"` + typed + `"}`, typed}, {`{}`, syntheticKey}} {
		code, out := do(t, srv, "POST", path, c.body)
		if code != 200 || bearers[len(bearers)-1] != c.want {
			t.Fatalf("body %s: %d %s, provider saw %q", c.body, code, out, bearers[len(bearers)-1])
		}
		if strings.Contains(out, typed) || strings.Contains(out, syntheticKey) {
			t.Fatalf("key in %s", out)
		}
	}
	if code, out := do(t, srv, "POST", path, `{"key":"   "}`); code == 200 {
		t.Fatalf("a blank key was tested: %s", out)
	}
}
