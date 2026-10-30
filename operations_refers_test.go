package ai

import (
	"sort"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

var markedReferences = map[string]string{
	"accountID":    "ai.account",
	"roleID":       "ai.ai_role",
	"revisionID":   "ai.ai_role_revision",
	"agencyRoleID": "agency.role",
}

func parsedOperations(t *testing.T) *dispatch.Spec {
	t.Helper()
	s, err := dispatch.Parse(Operations())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return s
}

func TestEveryPathArgumentNamingAThingIsMarkedWithWhatItRefersTo(t *testing.T) {
	s := parsedOperations(t)
	marked := map[string]int{}
	for name, op := range s.Operations {
		for _, a := range op.Args {
			want, isReference := markedReferences[a.As]
			isReference = isReference && !a.Ignored
			if isReference && a.Refers != want {
				t.Errorf("%s: argument %s refers %q, want %q", name, a.As, a.Refers, want)
			}
			if !isReference && a.Refers != "" {
				t.Errorf("%s: argument %s refers %q but names no thing the module marks", name, a.As, a.Refers)
			}
			if isReference {
				marked[a.As]++
			}
		}
	}
	for as := range markedReferences {
		if marked[as] == 0 {
			t.Errorf("no operation declares %s, so nothing proves it is marked", as)
		}
	}
}

func TestReferencesListsTheMarkedArgumentsAndNoScopeOrTestRunId(t *testing.T) {
	references := parsedOperations(t).References()
	if len(references) < 19 {
		t.Fatalf("References lists %d arguments, want every account, role, revision and agency role id the file declares", len(references))
	}
	seen := map[string]bool{}
	for _, r := range references {
		seen[r.As] = true
		if markedReferences[r.As] != r.Refers {
			t.Errorf("%s.%s is listed as referring %q", r.Operation, r.As, r.Refers)
		}
	}
	var missing []string
	for as := range markedReferences {
		if !seen[as] {
			missing = append(missing, as)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("References never lists %v", missing)
	}
	for _, scope := range []string{"agencyID", "testRunID"} {
		if seen[scope] {
			t.Errorf("%s is a scope or another module's id, not a reference this module marks", scope)
		}
	}
}
