package routes_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	ai "github.com/aosanya/mwanachama-backend-ai"
	"github.com/aosanya/mwanachama-backend-ai/routes"
	"github.com/aosanya/mwanachama-backend-shared/spec"
)

const syntheticKey = "sk-SYNTHETIC-route-4d3c2b1a"

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(1)
	s, err := ai.LoadSpec("../ai.wakala.json")
	if err != nil {
		t.Fatal(err)
	}
	s.Instance = "routestest"
	if err := ai.Provision(db, s); err != nil {
		t.Fatal(err)
	}
	m, err := ai.NewManager(db, s, ai.WithMasterKey([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	rs, err := routes.Build(m)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	for _, rt := range rs {
		mux.HandleFunc(rt.Pattern(""), rt.Handler)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func do(t *testing.T, srv *httptest.Server, method, path, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(raw)
}

func TestRoutesAreTheDeclaredTable(t *testing.T) {
	var got []string
	m, _ := ai.NewManager(func() *gorm.DB {
		db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		return db
	}(), mustSpec(t))
	for _, rt := range routes.Routes(m) {
		got = append(got, rt.Pattern(""))
	}
	sort.Strings(got)
	want := []string{
		"DELETE /agencies/{agencyID}/accounts/{accountID}",
		"GET /agencies/{agencyID}/accounts",
		"GET /agencies/{agencyID}/accounts/{accountID}",
		"POST /agencies/{agencyID}/accounts",
		"POST /agencies/{agencyID}/accounts/{accountID}/chat",
		"POST /agencies/{agencyID}/accounts/{accountID}/test",
		"GET /agencies/{agencyID}/accounts/{accountID}/usage",
		"GET /agencies/{agencyID}/test-runs/{testRunID}/usage",
		"POST /agencies/{agencyID}/limits/check",
		"POST /agencies/{agencyID}/usage",
		"PUT /agencies/{agencyID}/accounts/{accountID}",
		"PUT /agencies/{agencyID}/accounts/{accountID}/key",
		"PUT /agencies/{agencyID}/accounts/{accountID}/limits",
		"PUT /agencies/{agencyID}/accounts/{accountID}/default",
		"GET /agencies/{agencyID}/default-account",
		"DELETE /agencies/{agencyID}/ai-roles/{roleID}",
		"GET /agencies/{agencyID}/ai-roles",
		"GET /agencies/{agencyID}/ai-roles/{roleID}",
		"GET /agencies/{agencyID}/ai-roles/{roleID}/revisions",
		"POST /agencies/{agencyID}/ai-roles",
		"POST /agencies/{agencyID}/ai-roles/{roleID}/revisions/{revisionID}/restore",
		"PUT /agencies/{agencyID}/ai-roles/{roleID}",
		"PUT /agencies/{agencyID}/ai-roles/{roleID}/mapping",
		"GET /agencies/{agencyID}/ai-roles/{roleID}/resolve",
		"GET /agencies/{agencyID}/agency-roles/{agencyRoleID}/resolve",
		"GET /agencies/{agencyID}/models",
		"POST /agencies/{agencyID}/accounts/{accountID}/models/refresh",
		"GET /agencies/{agencyID}/chat-models",
		"PUT /agencies/{agencyID}/chat-models/pick",
		"PUT /agencies/{agencyID}/ai-roles/{roleID}/limits",
	}
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func mustSpec(t *testing.T) *spec.Spec {
	t.Helper()
	s, err := ai.LoadSpec("../ai.wakala.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestKeyNeverComesBackFromAnyAddress(t *testing.T) {
	srv := newServer(t)
	base := "/agencies/ag-1/accounts"
	code, body := do(t, srv, "POST", base, `{"name":"main","provider":"openai_compatible","base_url":"https://m.example.test/v1","model_name":"synthetic","status":"enabled"}`)
	if code != 201 {
		t.Fatalf("create %d %s", code, body)
	}
	id := between(body, `"id":"`, `"`)
	outputs := []string{body}
	for _, c := range []struct{ method, path, body string }{
		{"PUT", base + "/" + id + "/key", `{"key":"` + syntheticKey + `"}`},
		{"GET", base + "/" + id, ""},
		{"GET", base, ""},
		{"PUT", base + "/" + id, `{"name":"main","provider":"openai_compatible","base_url":"https://m.example.test/v1","model_name":"synthetic","status":"enabled"}`},
		{"PUT", "/agencies/ag-2/accounts/" + id + "/key", `{"key":"` + syntheticKey + `"}`},
		{"PUT", base + "/missing/key", `{"key":"` + syntheticKey + `"}`},
		{"PUT", base + "/" + id + "/key", `{"key":"` + syntheticKey + `","extra":1}`},
		{"POST", base, `{"name":"bad","provider":"nope","key":"` + syntheticKey + `"}`},
	} {
		_, out := do(t, srv, c.method, c.path, c.body)
		outputs = append(outputs, out)
	}
	for _, o := range outputs {
		if strings.Contains(o, syntheticKey) {
			t.Fatalf("key leaked: %s", o)
		}
	}
	code, body = do(t, srv, "GET", base+"/"+id, "")
	if code != 200 || !strings.Contains(body, `"key_set":true`) {
		t.Fatalf("key_set not reported: %d %s", code, body)
	}
	if code, _ := do(t, srv, "GET", "/agencies/ag-2/accounts/"+id, ""); code != 404 {
		t.Fatalf("other agency got %d", code)
	}
}

func between(s, a, b string) string {
	i := strings.Index(s, a)
	if i < 0 {
		return ""
	}
	s = s[i+len(a):]
	return s[:strings.Index(s, b)]
}

func TestSetLimitsRoute(t *testing.T) {
	srv := newServer(t)
	_, body := do(t, srv, "POST", "/agencies/ag-1/accounts", `{"name":"main","provider":"openai_compatible","base_url":"https://m.example.test/v1","model_name":"synthetic","status":"enabled"}`)
	id := between(body, `"id":"`, `"`)
	code, out := do(t, srv, "PUT", "/agencies/ag-1/accounts/"+id+"/limits", `{"tokens_per_call":7,"tokens_per_day":8,"tokens_per_month":9,"requests_per_day":10,"requests_per_minute":11}`)
	if code != 200 || !strings.Contains(out, `"tokens_per_month":9`) || !strings.Contains(out, `"name":"main"`) {
		t.Fatalf("%d %s", code, out)
	}
	if code, _ := do(t, srv, "PUT", "/agencies/ag-2/accounts/"+id+"/limits", `{"tokens_per_day":1}`); code != 404 {
		t.Fatalf("other agency got %d", code)
	}
	if code, _ := do(t, srv, "PUT", "/agencies/ag-1/accounts/"+id+"/limits", `{"name":"renamed"}`); code == 200 {
		t.Fatalf("a body naming more than limits was accepted")
	}
	if code, _ := do(t, srv, "PUT", "/agencies/ag-1/accounts/"+id+"/limits", `{"tokens_per_day":-1}`); code != 400 {
		t.Fatalf("negative got %d", code)
	}
	_, got := do(t, srv, "GET", "/agencies/ag-1/accounts/"+id, "")
	if !strings.Contains(got, `"tokens_per_day":8`) {
		t.Fatalf("refusals changed limits: %s", got)
	}
}

func TestDefaultAccountRoutes(t *testing.T) {
	srv := newServer(t)
	code, out := do(t, srv, "GET", "/agencies/ag-1/default-account", "")
	if code != 404 || !strings.Contains(out, "Mark a default AI account under Models before running a test") {
		t.Fatalf("none marked: %d %s", code, out)
	}
	_, body := do(t, srv, "POST", "/agencies/ag-1/accounts", `{"name":"main","provider":"openai_compatible","base_url":"https://m.example.test/v1","model_name":"synthetic","status":"enabled"}`)
	id := between(body, `"id":"`, `"`)
	if code, out := do(t, srv, "PUT", "/agencies/ag-1/accounts/"+id+"/default", ""); code != 200 || !strings.Contains(out, `"default":true`) {
		t.Fatalf("mark: %d %s", code, out)
	}
	if code, out := do(t, srv, "GET", "/agencies/ag-1/default-account", ""); code != 200 || !strings.Contains(out, `"id":"`+id+`"`) {
		t.Fatalf("read: %d %s", code, out)
	}
	if code, _ := do(t, srv, "PUT", "/agencies/ag-1/accounts/nope/default", ""); code != 404 {
		t.Fatalf("unknown account got %d", code)
	}
	if code, _ := do(t, srv, "PUT", "/agencies/ag-2/accounts/"+id+"/default", ""); code != 404 {
		t.Fatalf("other agency got %d", code)
	}
}
