package routes_test

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

var limitKeyNames = []string{
	"tokens_per_call", "tokens_per_day", "tokens_per_month",
	"requests_per_day", "requests_per_minute",
	"tokens_per_week", "requests_per_week",
	"tokens_per_test", "scenarios_at_once",
}

const (
	accountFields   = `"name":"main","provider":"openai_compatible","base_url":"https://m.example.test/v1","model_name":"synthetic","status":"enabled"`
	roleFields      = `"name":"farmers","description":"helps farmers","fine_tuning":"be brief","status":"enabled"`
	allNineHeld     = `"tokens_per_call":11,"tokens_per_day":12,"tokens_per_month":13,"requests_per_day":14,"requests_per_minute":15,"tokens_per_week":16,"requests_per_week":17,"tokens_per_test":18,"scenarios_at_once":19`
	someKeysSent    = `"tokens_per_day":5,"requests_per_day":0,"tokens_per_week":null`
	accountsAddress = "/agencies/ag/accounts"
	rolesAddress    = "/agencies/ag/ai-roles"
)

var allNineHeldValues = map[string]any{
	"tokens_per_call": 11.0, "tokens_per_day": 12.0, "tokens_per_month": 13.0,
	"requests_per_day": 14.0, "requests_per_minute": 15.0,
	"tokens_per_week": 16.0, "requests_per_week": 17.0,
	"tokens_per_test": 18.0, "scenarios_at_once": 19.0,
}

var onlyTheKeysSentSurvive = map[string]any{"tokens_per_day": 5.0, "requests_per_day": 0.0}

func decoded(t *testing.T, body string) map[string]any {
	t.Helper()
	out := map[string]any{}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("not an object: %s", body)
	}
	return out
}

func requireLimits(t *testing.T, where, body string, want map[string]any) {
	t.Helper()
	got := decoded(t, body)
	for _, key := range limitKeyNames {
		value, present := got[key]
		if !present {
			t.Errorf("%s: %s is not in the answer", where, key)
			continue
		}
		if !reflect.DeepEqual(value, want[key]) {
			t.Errorf("%s: %s is %v, want %v", where, key, value, want[key])
		}
	}
}

func createdID(t *testing.T, srv *httptest.Server, address, body string) string {
	t.Helper()
	code, out := asPerson(t, srv, "ann", "POST", address, body)
	if code != 201 {
		t.Fatalf("create %d %s", code, out)
	}
	return decoded(t, out)["id"].(string)
}

func readBack(t *testing.T, srv *httptest.Server, address, id string) string {
	t.Helper()
	code, out := asPerson(t, srv, "ann", "GET", address+"/"+id, "")
	if code != 200 {
		t.Fatalf("read %d %s", code, out)
	}
	return out
}

func TestSetLimitsReplacesAllNineSoAKeyLeftOutIsClearedANullIsClearedAndAZeroIsKept(t *testing.T) {
	srv := newPersonServer(t)
	id := createdID(t, srv, accountsAddress, `{`+accountFields+`,`+allNineHeld+`}`)
	requireLimits(t, "held before", readBack(t, srv, accountsAddress, id), allNineHeldValues)

	code, out := asPerson(t, srv, "ann", "PUT", accountsAddress+"/"+id+"/limits", `{`+someKeysSent+`}`)

	if code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	requireLimits(t, "answer", out, onlyTheKeysSentSurvive)
	requireLimits(t, "read back", readBack(t, srv, accountsAddress, id), onlyTheKeysSentSurvive)
	if got := decoded(t, readBack(t, srv, accountsAddress, id)); got["name"] != "main" || got["model_name"] != "synthetic" {
		t.Fatalf("a limits write touched the settings: %v", got)
	}
}

func TestChangeAccountReplacesAllNineSoAKeyLeftOutIsClearedWhileTheChatSwitchAndLabelStayWhenLeftOut(t *testing.T) {
	srv := newPersonServer(t)
	id := createdID(t, srv, accountsAddress, `{`+accountFields+`,"chat_enabled":true,"chat_label":"Muse",`+allNineHeld+`}`)

	code, out := asPerson(t, srv, "ann", "PUT", accountsAddress+"/"+id, `{`+accountFields+`,`+someKeysSent+`}`)

	if code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	requireLimits(t, "answer", out, onlyTheKeysSentSurvive)
	got := readBack(t, srv, accountsAddress, id)
	requireLimits(t, "read back", got, onlyTheKeysSentSurvive)
	if kept := decoded(t, got); kept["chat_enabled"] != true || kept["chat_label"] != "Muse" {
		t.Fatalf("a chat switch and label left out were not kept: %v", kept)
	}

	code, out = asPerson(t, srv, "ann", "PUT", accountsAddress+"/"+id, `{`+accountFields+`,"chat_enabled":false,"chat_label":""}`)
	if replaced := decoded(t, out); code != 200 || replaced["chat_enabled"] != false || replaced["chat_label"] != "" {
		t.Fatalf("a chat switch and label sent were not replaced: %d %s", code, out)
	}
}

func TestChangeAccountWithOnlyLimitsIsRefusedAndLeavesTheLimitsAlone(t *testing.T) {
	srv := newPersonServer(t)
	id := createdID(t, srv, accountsAddress, `{`+accountFields+`,`+allNineHeld+`}`)

	code, out := asPerson(t, srv, "ann", "PUT", accountsAddress+"/"+id, `{`+someKeysSent+`}`)

	if code != 400 {
		t.Fatalf("a change naming no settings got %d %s", code, out)
	}
	requireLimits(t, "read back", readBack(t, srv, accountsAddress, id), allNineHeldValues)
}

func TestSetRoleLimitsReplacesAllNineSoAKeyLeftOutIsClearedANullIsClearedAndAZeroIsKept(t *testing.T) {
	srv := newPersonServer(t)
	id := createdID(t, srv, rolesAddress, `{`+roleFields+`,`+allNineHeld+`}`)
	requireLimits(t, "held before", readBack(t, srv, rolesAddress, id), allNineHeldValues)

	code, out := asPerson(t, srv, "bob", "PUT", rolesAddress+"/"+id+"/limits", `{`+someKeysSent+`}`)

	if code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	requireLimits(t, "answer", out, onlyTheKeysSentSurvive)
	got := readBack(t, srv, rolesAddress, id)
	requireLimits(t, "read back", got, onlyTheKeysSentSurvive)
	if kept := decoded(t, got); kept["name"] != "farmers" || kept["description"] != "helps farmers" || kept["fine_tuning"] != "be brief" {
		t.Fatalf("a limits write touched the role: %v", kept)
	}
	code, revisions := asPerson(t, srv, "bob", "GET", rolesAddress+"/"+id+"/revisions", "")
	if code != 200 || !strings.Contains(revisions, `"saved_by":"bob"`) || !strings.Contains(revisions, `"tokens_per_test":18`) || !strings.Contains(revisions, `"scenarios_at_once":19`) {
		t.Fatalf("the nine limits cleared were not kept as a revision: %d %s", code, revisions)
	}
}

func TestChangeRoleReplacesAllNineSoAKeyLeftOutIsClearedWhileTheSwitchStaysAndTheDescriptionIsBlanked(t *testing.T) {
	srv := newPersonServer(t)
	id := createdID(t, srv, rolesAddress, `{`+roleFields+`,"use_agency_instructions":false,`+allNineHeld+`}`)

	code, out := asPerson(t, srv, "bob", "PUT", rolesAddress+"/"+id, `{"name":"farmers","fine_tuning":"be brief","status":"enabled",`+someKeysSent+`}`)

	if code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	requireLimits(t, "answer", out, onlyTheKeysSentSurvive)
	got := readBack(t, srv, rolesAddress, id)
	requireLimits(t, "read back", got, onlyTheKeysSentSurvive)
	kept := decoded(t, got)
	if kept["use_agency_instructions"] != false {
		t.Fatalf("a switch left out was not kept: %v", kept)
	}
	if kept["description"] != "" {
		t.Fatalf("a description left out was not blanked: %v", kept)
	}
}

func TestAddAccountStartsEveryLimitLeftOutOrNullAtNullAndKeepsAZero(t *testing.T) {
	srv := newPersonServer(t)

	code, out := asPerson(t, srv, "ann", "POST", accountsAddress, `{`+accountFields+`,`+someKeysSent+`}`)

	if code != 201 {
		t.Fatalf("%d %s", code, out)
	}
	requireLimits(t, "answer", out, onlyTheKeysSentSurvive)
	id := decoded(t, out)["id"].(string)
	got := readBack(t, srv, accountsAddress, id)
	requireLimits(t, "read back", got, onlyTheKeysSentSurvive)
	if started := decoded(t, got); started["chat_enabled"] != false || started["chat_label"] != "" {
		t.Fatalf("a chat switch and label left out did not start off and empty: %v", started)
	}
}

func TestAddRoleStartsEveryLimitLeftOutOrNullAtNullKeepsAZeroAndDefaultsTheSwitchToOn(t *testing.T) {
	srv := newPersonServer(t)

	code, out := asPerson(t, srv, "ann", "POST", rolesAddress, `{`+roleFields+`,`+someKeysSent+`}`)

	if code != 201 {
		t.Fatalf("%d %s", code, out)
	}
	requireLimits(t, "answer", out, onlyTheKeysSentSurvive)
	id := decoded(t, out)["id"].(string)
	got := readBack(t, srv, rolesAddress, id)
	requireLimits(t, "read back", got, onlyTheKeysSentSurvive)
	if started := decoded(t, got); started["use_agency_instructions"] != true {
		t.Fatalf("a switch left out did not default to on: %v", started)
	}
	code, out = asPerson(t, srv, "ann", "POST", rolesAddress, `{"name":"scouts","status":"enabled","fine_tuning":"x","use_agency_instructions":false}`)
	if off := decoded(t, out); code != 201 || off["use_agency_instructions"] != false {
		t.Fatalf("a switch sent as off was not kept off: %d %s", code, out)
	}
}

func TestTheLimitToolsReplaceAllNineLikeTheRoutesDo(t *testing.T) {
	c := newCalledTools(t)
	held := map[string]any{"agencyID": "ag", "name": "main", "provider": "openai_compatible", "base_url": "https://m.example.test/v1", "model_name": "synthetic", "status": "enabled"}
	for key, value := range allNineHeldValues {
		held[key] = value
	}
	created := decoded(t, c.mustCall("ai_create_account", held))
	sent := map[string]any{"agencyID": "ag", "accountID": created["id"], "tokens_per_day": 5, "requests_per_day": 0, "tokens_per_week": nil}

	requireLimits(t, "ai_set_limits", c.mustCall("ai_set_limits", sent), onlyTheKeysSentSurvive)

	role := decoded(t, c.mustCall("ai_create_role", map[string]any{"agencyID": "ag", "name": "farmers", "status": "enabled", "fine_tuning": "be brief", "tokens_per_test": 18, "scenarios_at_once": 19}))
	sent["roleID"] = role["id"]
	delete(sent, "accountID")
	requireLimits(t, "ai_set_role_limits", c.mustCall("ai_set_role_limits", sent), onlyTheKeysSentSurvive)
}

func TestChangeRoleWithOnlyLimitsIsRefusedAndALimitsWriteNamingMoreIsRefusedAndTheFineTuningLeftOutIsBlanked(t *testing.T) {
	srv := newPersonServer(t)
	id := createdID(t, srv, rolesAddress, `{`+roleFields+`,`+allNineHeld+`}`)

	if code, out := asPerson(t, srv, "bob", "PUT", rolesAddress+"/"+id, `{`+someKeysSent+`}`); code != 400 {
		t.Fatalf("a change naming no settings got %d %s", code, out)
	}
	if code, out := asPerson(t, srv, "bob", "PUT", rolesAddress+"/"+id+"/limits", `{"name":"renamed",`+someKeysSent+`}`); code != 400 {
		t.Fatalf("a limits write naming more than limits got %d %s", code, out)
	}
	requireLimits(t, "read back", readBack(t, srv, rolesAddress, id), allNineHeldValues)

	code, out := asPerson(t, srv, "bob", "PUT", rolesAddress+"/"+id, `{"name":"farmers","status":"enabled",`+allNineHeld+`}`)
	if blanked := decoded(t, out); code != 200 || blanked["fine_tuning"] != "" {
		t.Fatalf("a fine-tuning left out was not blanked: %d %s", code, out)
	}
}
