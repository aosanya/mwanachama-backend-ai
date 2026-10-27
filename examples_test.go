package ai_test

import (
	"path/filepath"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

var allRoles = []string{"account", "account_key"}

func shippedSpecs(t *testing.T) []string {
	t.Helper()
	examples, err := filepath.Glob("spec/examples/*.json")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return append([]string{"ai.wakala.json"}, examples...)
}

func TestEveryExampleFitsTheTypes(t *testing.T) {
	for _, path := range shippedSpecs(t) {
		t.Run(path, func(t *testing.T) {
			s, err := ai.LoadSpec(path)
			if err != nil {
				t.Fatalf("LoadSpec: %v", err)
			}
			db := openDB(t)
			if err := ai.Provision(db, s); err != nil {
				t.Fatalf("Provision: %v", err)
			}
			if _, err := ai.NewManager(db, s); err != nil {
				t.Fatalf("NewManager: %v", err)
			}
		})
	}
}

func TestTwoDomainsCoexist(t *testing.T) {
	db := openDB(t)
	tables := map[string]bool{}
	for _, path := range shippedSpecs(t) {
		s, err := ai.LoadSpec(path)
		if err != nil {
			t.Fatalf("LoadSpec(%s): %v", path, err)
		}
		if err := ai.Provision(db, s); err != nil {
			t.Fatalf("Provision(%s): %v", path, err)
		}
		for _, o := range s.Objects {
			name := s.TableFor(o)
			if tables[name] {
				t.Fatalf("%s: %s collides", path, s.RawNameFor(o))
			}
			tables[name] = true
		}
	}
	if len(tables) < 2*len(allRoles) {
		t.Fatalf("expected two domains, got %d tables", len(tables))
	}
}

func TestRequiredFieldsHaveNoDefault(t *testing.T) {
	b, err := ai.Blueprint()
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range b.Objects {
		for _, f := range o.Fields {
			if f.Required && f.Default != "" {
				t.Errorf("%s.%s is both required and defaulted", o.Role, f.Name)
			}
		}
	}
}

func TestEveryRoleArrivesWithItsFields(t *testing.T) {
	s, err := ai.SpecFor("app")
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range allRoles {
		o, ok := s.ByRole(role)
		if !ok || len(o.Fields) == 0 {
			t.Errorf("role %q missing or without fields", role)
		}
	}
}
