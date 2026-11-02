package mcp_test

import (
	"context"
	"strings"
	"testing"
)

func TestEveryToolThatReplacesLimitsSaysALimitLeftOutIsCleared(t *testing.T) {
	cs := connect(t)
	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	descriptions := map[string]string{}
	for _, tool := range list.Tools {
		descriptions[tool.Name] = tool.Description
	}
	const sentence = "A limit left out of the body is cleared, so send the ones to keep."
	for _, name := range []string{"ai_set_limits", "ai_update_account", "ai_update_role", "ai_set_role_limits"} {
		if !strings.Contains(descriptions[name], sentence) {
			t.Errorf("%s does not say a limit left out is cleared: %q", name, descriptions[name])
		}
	}
}
