package routes_test

import (
	"strings"
	"testing"
)

const accountWithoutChat = `"name":"main","provider":"openai_compatible","base_url":"https://m.example.test/v1","model_name":"synthetic","status":"enabled"`

func TestChatFieldsTravelThroughTheAccountRoutes(t *testing.T) {
	srv := newServer(t)
	code, body := do(t, srv, "POST", "/agencies/ag-1/accounts", `{`+accountWithoutChat+`,"chat_enabled":true,"chat_label":"GLM-5.3 (Baseten)"}`)
	if code != 201 || !strings.Contains(body, `"chat_enabled":true`) || !strings.Contains(body, `"chat_label":"GLM-5.3 (Baseten)"`) {
		t.Fatalf("create %d %s", code, body)
	}
	id := between(body, `"id":"`, `"`)
	for _, path := range []string{"/agencies/ag-1/accounts/" + id, "/agencies/ag-1/accounts"} {
		code, got := do(t, srv, "GET", path, "")
		if code != 200 || !strings.Contains(got, `"chat_enabled":true`) || !strings.Contains(got, `"chat_label":"GLM-5.3 (Baseten)"`) {
			t.Fatalf("GET %s %d %s", path, code, got)
		}
	}
}

func TestPutThatLeavesOutTheChatFieldsKeepsThem(t *testing.T) {
	srv := newServer(t)
	_, body := do(t, srv, "POST", "/agencies/ag-1/accounts", `{`+accountWithoutChat+`,"chat_enabled":true,"chat_label":"Kept"}`)
	id := between(body, `"id":"`, `"`)
	code, out := do(t, srv, "PUT", "/agencies/ag-1/accounts/"+id, `{`+strings.Replace(accountWithoutChat, `"main"`, `"renamed"`, 1)+`}`)
	if code != 200 || !strings.Contains(out, `"name":"renamed"`) || !strings.Contains(out, `"chat_enabled":true`) || !strings.Contains(out, `"chat_label":"Kept"`) {
		t.Fatalf("put %d %s", code, out)
	}
	_, got := do(t, srv, "GET", "/agencies/ag-1/accounts/"+id, "")
	if !strings.Contains(got, `"chat_enabled":true`) || !strings.Contains(got, `"chat_label":"Kept"`) {
		t.Fatalf("stored %s", got)
	}
	code, out = do(t, srv, "PUT", "/agencies/ag-1/accounts/"+id, `{`+accountWithoutChat+`,"chat_enabled":false,"chat_label":""}`)
	if code != 200 || !strings.Contains(out, `"chat_enabled":false`) || !strings.Contains(out, `"chat_label":""`) {
		t.Fatalf("explicit off %d %s", code, out)
	}
}

func TestRoutesRefuseABadLabelAndAnUndeclaredField(t *testing.T) {
	srv := newServer(t)
	_, body := do(t, srv, "POST", "/agencies/ag-1/accounts", `{`+accountWithoutChat+`}`)
	id := between(body, `"id":"`, `"`)
	for name, label := range map[string]string{"too long": strings.Repeat("a", 61), "newline": `a\nb`} {
		if code, out := do(t, srv, "POST", "/agencies/ag-1/accounts", `{`+strings.Replace(accountWithoutChat, `"main"`, `"x"`, 1)+`,"chat_label":"`+label+`"}`); code != 400 {
			t.Errorf("create with %s got %d %s", name, code, out)
		}
		if code, out := do(t, srv, "PUT", "/agencies/ag-1/accounts/"+id, `{`+accountWithoutChat+`,"chat_label":"`+label+`"}`); code != 400 {
			t.Errorf("put with %s got %d %s", name, code, out)
		}
	}
	if code, out := do(t, srv, "PUT", "/agencies/ag-1/accounts/"+id, `{`+accountWithoutChat+`,"surprise":1}`); code == 200 {
		t.Fatalf("an undeclared field was accepted: %s", out)
	}
}
