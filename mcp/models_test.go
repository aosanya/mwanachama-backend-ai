package mcp_test

import (
	"strings"
	"testing"
)

func TestListModelsToolAnswersTheStartingListShape(t *testing.T) {
	cs := connect(t)
	out, isErr := call(t, cs, "ai_list_models", map[string]any{"agencyID": "ag-1"})
	if isErr || !strings.Contains(out, `"items":[]`) {
		t.Fatalf("empty list: %s", out)
	}
	if out, isErr = call(t, cs, "ai_list_models", map[string]any{"agencyID": "ag-1", "provider": "nope"}); !isErr {
		t.Fatalf("unknown kind was served: %s", out)
	}
	if out, isErr = call(t, cs, "ai_refresh_models", map[string]any{"agencyID": "ag-1", "accountID": "missing"}); !isErr {
		t.Fatalf("refresh of a missing account: %s", out)
	}
}
