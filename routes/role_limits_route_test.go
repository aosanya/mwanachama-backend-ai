package routes_test

import (
	"strings"
	"testing"
)

func TestSettingARolesLimitsOverHTTPKeepsTheCallerAsWhoSavedIt(t *testing.T) {
	srv := newPersonServer(t)
	code, out := asPerson(t, srv, "ann", "POST", "/agencies/ag/ai-roles", `{"name":"farmers","status":"enabled","fine_tuning":"be brief","tokens_per_day":100000}`)
	if code != 201 {
		t.Fatalf("create %d %s", code, out)
	}
	id := between(out, `"id":"`, `"`)

	code, out = asPerson(t, srv, "bob", "PUT", "/agencies/ag/ai-roles/"+id+"/limits", `{"tokens_per_day":150000,"requests_per_minute":30}`)
	if code != 200 || !strings.Contains(out, `"tokens_per_day":150000`) || !strings.Contains(out, `"requests_per_minute":30`) || !strings.Contains(out, `"name":"farmers"`) {
		t.Fatalf("set %d %s", code, out)
	}
	code, out = asPerson(t, srv, "bob", "GET", "/agencies/ag/ai-roles/"+id+"/revisions", "")
	if code != 200 || !strings.Contains(out, `"saved_by":"bob"`) || !strings.Contains(out, `"tokens_per_day":100000`) {
		t.Fatalf("revisions %d %s", code, out)
	}
	if code, _ = asPerson(t, srv, "bob", "PUT", "/agencies/other/ai-roles/"+id+"/limits", `{"tokens_per_day":1}`); code != 404 {
		t.Fatalf("another agency got %d", code)
	}
	if code, _ = asPerson(t, srv, "bob", "PUT", "/agencies/ag/ai-roles/"+id+"/limits", `{"tokens_per_day":-5}`); code != 400 {
		t.Fatalf("a negative limit got %d", code)
	}
}
