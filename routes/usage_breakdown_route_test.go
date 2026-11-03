package routes_test

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
	"github.com/aosanya/mwanachama-backend-ai/routes"
)

func TestTheBreakdownIsOneToolTakingTheSpanAndOptionallyTheGroupRoleAndModel(t *testing.T) {
	for _, tool := range routes.Tools(struct{ ai.Manager }{}) {
		if tool.Name != "ai_usage_breakdown" {
			continue
		}
		var schema struct {
			Properties map[string]json.RawMessage `json:"properties"`
			Required   []string                   `json:"required"`
		}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Fatal(err)
		}
		names := []string{}
		for name := range schema.Properties {
			names = append(names, name)
		}
		sort.Strings(names)
		sort.Strings(schema.Required)
		if tool.Action != "ai.usage.breakdown" || strings.Join(names, ",") != "agencyID,group_by,model,role_id,window" || strings.Join(schema.Required, ",") != "agencyID,window" {
			t.Fatalf("%s %v required %v", tool.Action, names, schema.Required)
		}
		return
	}
	t.Fatal("no ai_usage_breakdown tool")
}
