package routes

import "github.com/aosanya/mwanachama-backend-shared/dispatch"

var serverOwnedAccountFields = []string{
	"id", "agency_id", "key_set", "default",
	"last_test_result", "last_tested_at", "last_test_prompt", "last_test_reply", "last_test_ms",
	"created_at", "updated_at",
}

func Fields() map[string]dispatch.FieldDoc {
	out := map[string]dispatch.FieldDoc{}
	for _, key := range serverOwnedAccountFields {
		out["Account."+key] = dispatch.FieldDoc{ReadOnly: true}
	}
	return out
}
