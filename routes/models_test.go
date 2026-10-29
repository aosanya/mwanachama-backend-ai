package routes_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeModelProvider(t *testing.T, status int, answer string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func keyedAccountAt(t *testing.T, srv *httptest.Server, agency, providerURL string) string {
	t.Helper()
	body := `{"name":"main","provider":"openai_compatible","base_url":"` + providerURL + `","model_name":"synthetic","status":"enabled"}`
	code, out := do(t, srv, "POST", "/agencies/"+agency+"/accounts", body)
	if code != 201 {
		t.Fatalf("create %d %s", code, out)
	}
	id := between(out, `"id":"`, `"`)
	if code, out := do(t, srv, "PUT", "/agencies/"+agency+"/accounts/"+id+"/key", `{"key":"`+syntheticKey+`"}`); code != 200 {
		t.Fatalf("key %d %s", code, out)
	}
	return id
}

func TestRefreshAndListModelsOverHTTP(t *testing.T) {
	srv := newServer(t)
	provider := fakeModelProvider(t, 200, `{"data":[{"id":"m-1"},{"id":"m-2"}]}`)
	id := keyedAccountAt(t, srv, "ag-1", provider.URL)

	code, out := do(t, srv, "POST", "/agencies/ag-1/accounts/"+id+"/models/refresh", "")
	if code != 200 || !strings.Contains(out, `"model_name":"m-1"`) || !strings.Contains(out, `"source":"provider"`) {
		t.Fatalf("refresh %d %s", code, out)
	}
	code, out = do(t, srv, "GET", "/agencies/ag-1/models?provider=openai_compatible", "")
	if code != 200 || !strings.Contains(out, `"model_name":"m-2"`) {
		t.Fatalf("list %d %s", code, out)
	}
	if code, out = do(t, srv, "GET", "/agencies/ag-2/models", ""); code != 200 || strings.Contains(out, "m-1") {
		t.Fatalf("another agency %d %s", code, out)
	}
	if code, _ = do(t, srv, "GET", "/agencies/ag-1/models?provider=nope", ""); code != 400 {
		t.Fatalf("unknown kind got %d", code)
	}
	if code, _ = do(t, srv, "POST", "/agencies/ag-2/accounts/"+id+"/models/refresh", ""); code != 404 {
		t.Fatalf("another agency's account got %d", code)
	}
}

func TestRefreshModelsRefusalsOverHTTP(t *testing.T) {
	srv := newServer(t)
	refusing := fakeModelProvider(t, 401, `{"error":"`+syntheticKey+`"}`)
	id := keyedAccountAt(t, srv, "ag-1", refusing.URL)
	code, out := do(t, srv, "POST", "/agencies/ag-1/accounts/"+id+"/models/refresh", "")
	if code != 502 || strings.Contains(out, syntheticKey) {
		t.Fatalf("refused provider %d %s", code, out)
	}

	body := `{"name":"keyless","provider":"openai_compatible","base_url":"` + refusing.URL + `","model_name":"synthetic","status":"enabled"}`
	_, created := do(t, srv, "POST", "/agencies/ag-1/accounts", body)
	keyless := between(created, `"id":"`, `"`)
	if code, _ := do(t, srv, "POST", "/agencies/ag-1/accounts/"+keyless+"/models/refresh", ""); code != 404 {
		t.Fatalf("no key got %d", code)
	}
}
