package routes_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	ai "github.com/aosanya/mwanachama-backend-ai"
	"github.com/aosanya/mwanachama-backend-ai/routes"
)

type personKey struct{}

func newPersonServer(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	pool.SetMaxOpenConns(1)
	s := mustSpec(t)
	s.Instance = "personstest"
	if err := ai.Provision(db, s); err != nil {
		t.Fatal(err)
	}
	m, err := ai.NewManager(db, s)
	if err != nil {
		t.Fatal(err)
	}
	rs, err := routes.BuildFor(m, routes.Mount{Caller: func(ctx context.Context) string {
		person, _ := ctx.Value(personKey{}).(string)
		return person
	}})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	for _, rt := range rs {
		handler := rt.Handler
		mux.HandleFunc(rt.Pattern(""), func(w http.ResponseWriter, r *http.Request) {
			handler(w, r.WithContext(context.WithValue(r.Context(), personKey{}, r.Header.Get("X-Person"))))
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func asPerson(t *testing.T, srv *httptest.Server, person, method, path, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person", person)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(raw)
}

func addChatAccount(t *testing.T, srv *httptest.Server, name, label string, chat bool) string {
	t.Helper()
	flag := "false"
	if chat {
		flag = "true"
	}
	body := `{"name":"` + name + `","provider":"openai_compatible","base_url":"https://m.example.test/v1","model_name":"` + name + `","status":"enabled","chat_enabled":` + flag + `,"chat_label":"` + label + `"}`
	code, out := asPerson(t, srv, "ann", "POST", "/agencies/ag/accounts", body)
	if code != 201 {
		t.Fatalf("create %d %s", code, out)
	}
	return between(out, `"id":"`, `"`)
}

func TestTheChatModelPickIsKeptPerSignedInPersonOverHTTP(t *testing.T) {
	srv := newPersonServer(t)
	addChatAccount(t, srv, "one", "One", true)
	two := addChatAccount(t, srv, "two", "Two", true)
	hidden := addChatAccount(t, srv, "hidden", "", false)

	code, out := asPerson(t, srv, "ann", "PUT", "/agencies/ag/chat-models/pick", `{"account_id":"`+two+`"}`)
	if code != 200 || !strings.Contains(out, `"label":"Two","picked":true`) {
		t.Fatalf("pick %d %s", code, out)
	}
	if code, out = asPerson(t, srv, "bob", "GET", "/agencies/ag/chat-models", ""); code != 200 || !strings.Contains(out, `"label":"One","picked":true`) {
		t.Fatalf("bob %d %s", code, out)
	}
	if code, out = asPerson(t, srv, "ann", "GET", "/agencies/ag/chat-models", ""); code != 200 || !strings.Contains(out, `"label":"Two","picked":true`) {
		t.Fatalf("ann %d %s", code, out)
	}
	if code, _ = asPerson(t, srv, "ann", "PUT", "/agencies/ag/chat-models/pick", `{"account_id":"`+hidden+`"}`); code != 409 {
		t.Fatalf("a model the chat may not use got %d", code)
	}
	if code, _ = asPerson(t, srv, "ann", "PUT", "/agencies/other/chat-models/pick", `{"account_id":"`+two+`"}`); code != 404 {
		t.Fatalf("another agency's model got %d", code)
	}
	if code, _ = asPerson(t, srv, "", "PUT", "/agencies/ag/chat-models/pick", `{"account_id":"`+two+`"}`); code != 400 {
		t.Fatalf("no person got %d", code)
	}
}

func TestThePersonOfAPickIsNeverReadFromTheBody(t *testing.T) {
	srv := newPersonServer(t)
	addChatAccount(t, srv, "one", "One", true)
	two := addChatAccount(t, srv, "two", "Two", true)
	code, _ := asPerson(t, srv, "ann", "PUT", "/agencies/ag/chat-models/pick", `{"account_id":"`+two+`","person_id":"bob"}`)
	if code == 200 {
		if _, out := asPerson(t, srv, "bob", "GET", "/agencies/ag/chat-models", ""); !strings.Contains(out, `"label":"One","picked":true`) {
			t.Fatalf("the body chose bob's pick: %s", out)
		}
	}
}
