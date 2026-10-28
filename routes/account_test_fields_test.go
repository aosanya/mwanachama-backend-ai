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
	code, out := do(t, srv, "POST", "/agencies/ag-1/accounts/"+id+"/test", "")
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
