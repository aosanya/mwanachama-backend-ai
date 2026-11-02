package routes_test

import (
	"testing"
)

var serverOwnedAccountFields = []string{
	"id", "agency_id", "key_set", "default",
	"last_test_result", "last_tested_at", "last_test_prompt", "last_test_reply", "last_test_ms",
	"created_at", "updated_at",
}

var accountInputsTheCallersSend = []string{
	"name", "provider", "base_url", "model_name", "status", "chat_enabled", "chat_label",
	"tokens_per_call", "tokens_per_day", "tokens_per_month", "requests_per_day", "requests_per_minute",
	"tokens_per_week", "requests_per_week", "tokens_per_test", "scenarios_at_once",
}

func TestTheAccountToolsAskForNoFieldTheServerSetsAndStillTakeEveryFieldTheCallersSend(t *testing.T) {
	c := newCalledTools(t)
	for _, tool := range []string{"ai_create_account", "ai_update_account"} {
		properties := c.schemaProperties(tool)
		for _, key := range serverOwnedAccountFields {
			if _, listed := properties[key]; listed {
				t.Errorf("%s asks for %q, which the server sets", tool, key)
			}
		}
		for _, key := range append([]string{"agencyID"}, accountInputsTheCallersSend...) {
			if _, listed := properties[key]; !listed {
				t.Errorf("%s lost the input %q", tool, key)
			}
		}
	}
}

func TestACreateAccountToolCallCarryingAServerOwnedFieldIsRefusedAndTheServerStillSetsIt(t *testing.T) {
	c := newCalledTools(t)
	sent := map[string]any{"agencyID": "ag", "name": "main", "provider": "openai_compatible", "base_url": "https://m.example.test/v1", "model_name": "synthetic", "status": "enabled"}
	for _, key := range serverOwnedAccountFields {
		with := map[string]any{key: "x"}
		for k, v := range sent {
			with[k] = v
		}
		_, err := c.call("ai_create_account", with)
		if err == nil || err.Error() != `ai_create_account takes no argument "`+key+`"` {
			t.Errorf("a create sent with %s: %v", key, err)
		}
	}

	made := decoded(t, c.mustCall("ai_create_account", sent))

	if made["id"] == "" || made["agency_id"] != "ag" || made["key_set"] != false || made["default"] != false || made["created_at"] == "" {
		t.Fatalf("the server did not set what it owns: %v", made)
	}
}

func TestACreateAccountOverHTTPCarryingAServerOwnedFieldIgnoresItAndTheServerSetsIt(t *testing.T) {
	srv := newPersonServer(t)
	forged := `{` + accountFields + `,"id":"forged","agency_id":"other","key_set":true,"default":true,"last_test_result":"working","last_test_reply":"OK","last_test_ms":9,"created_at":"1999","updated_at":"1999"}`

	code, out := asPerson(t, srv, "ann", "POST", accountsAddress, forged)

	made := decoded(t, out)
	if code != 201 || made["id"] == "forged" || made["agency_id"] != "ag" || made["key_set"] != false || made["default"] != false {
		t.Fatalf("%d %s", code, out)
	}
	if made["last_test_result"] != nil && made["last_test_result"] != "" || made["last_test_reply"] != "" || made["last_test_ms"] != 0.0 || made["created_at"] == "1999" || made["updated_at"] == "1999" {
		t.Fatalf("a test result or a stamp the caller sent was kept: %s", out)
	}
}
