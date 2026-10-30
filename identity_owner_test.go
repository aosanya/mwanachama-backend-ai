package ai_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/identity"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

var _ identity.Owner = ai.Manager(nil)

type ownedThings struct {
	byObject map[string]string
}

func seedOwnedThings(t *testing.T, m ai.Manager) ownedThings {
	t.Helper()
	account, err := m.CreateAccount(ctx, "ag-1", validAccount("main"))
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	role := mustRole(t, m, "ag-1", roleIn("scribe"))
	edit := roleIn("scribe")
	edit.FineTuning = "be shorter"
	if _, err := m.UpdateAIRole(ctx, "ag-1", role.ID, "owner", edit); err != nil {
		t.Fatalf("UpdateAIRole: %v", err)
	}
	revisions, err := m.ListAIRoleRevisions(ctx, "ag-1", role.ID)
	if err != nil || len(revisions) == 0 {
		t.Fatalf("ListAIRoleRevisions: %v, %d revisions", err, len(revisions))
	}
	return ownedThings{byObject: map[string]string{"account": account.ID, "ai_role": role.ID, "ai_role_revision": revisions[0].ID}}
}

func TestStableIdentityIsTheRawAiInstanceObjectAndIdOfEachObject(t *testing.T) {
	m, _, _ := newTestManager(t)
	things := seedOwnedThings(t, m)
	for object, id := range things.byObject {
		got, err := m.StableIdentity(ctx, object, id)
		if err != nil {
			t.Fatalf("StableIdentity(%s): %v", object, err)
		}
		if want := "ai/test/" + object + "-" + id; got != want {
			t.Errorf("StableIdentity(%s) = %q, want %q", object, got, want)
		}
		if _, err := identity.Parse(got); err != nil {
			t.Errorf("StableIdentity(%s) = %q is not a raw: %v", object, got, err)
		}
	}
}

func TestResolveGivesBackTheObjectAndIdStableIdentityWasAskedFor(t *testing.T) {
	m, _, _ := newTestManager(t)
	for object, id := range seedOwnedThings(t, m).byObject {
		raw, err := m.StableIdentity(ctx, object, id)
		if err != nil {
			t.Fatalf("StableIdentity(%s): %v", object, err)
		}
		gotObject, gotID, err := m.Resolve(ctx, raw)
		if err != nil || gotObject != object || gotID != id {
			t.Errorf("Resolve(%q) = %q, %q, %v; want %q, %q", raw, gotObject, gotID, err, object, id)
		}
	}
}

func TestStableIdentityRefusesAnObjectItDoesNotMintAndAMissingRow(t *testing.T) {
	m, _, _ := newTestManager(t)
	things := seedOwnedThings(t, m)
	if _, err := m.StableIdentity(ctx, "account_key", things.byObject["account"]); !errors.Is(err, ai.ErrNoStableIdentity) {
		t.Errorf("account_key: %v, want ErrNoStableIdentity", err)
	}
	if _, err := m.StableIdentity(ctx, "usage", "x"); !errors.Is(err, ai.ErrNoStableIdentity) {
		t.Errorf("usage: %v, want ErrNoStableIdentity", err)
	}
	if _, err := m.StableIdentity(ctx, "account", "no-such-id"); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("missing account: %v, want identity.ErrNotFound", err)
	}
}

func TestResolveNamesAMissExactly(t *testing.T) {
	m, _, _ := newTestManager(t)
	things := seedOwnedThings(t, m)
	id := things.byObject["account"]
	cases := []struct {
		name string
		raw  string
		want error
	}{
		{"not a raw", "nonsense", identity.ErrInvalidRaw},
		{"another module", "agency/test/account-" + id, identity.ErrNotOwned},
		{"another instance", "ai/other/account-" + id, identity.ErrNotOwned},
		{"an object ai does not mint", "ai/test/account_key-" + id, identity.ErrNotFound},
		{"a prefix nothing mints", "ai/test/widget-" + id, identity.ErrNotFound},
		{"an identity with no dash", "ai/test/" + strings.Repeat("a", 8), identity.ErrNotFound},
		{"a row that is gone", "ai/test/account-no-such-id", identity.ErrNotFound},
		{"an id of another object", "ai/test/ai_role-" + id, identity.ErrNotFound},
	}
	for _, c := range cases {
		if _, _, err := m.Resolve(ctx, c.raw); !errors.Is(err, c.want) {
			t.Errorf("%s: Resolve(%q) = %v, want %v", c.name, c.raw, err, c.want)
		}
	}
}

func TestAResolvedAccountIsGoneOnceItIsDeleted(t *testing.T) {
	m, _, _ := newTestManager(t)
	account, err := m.CreateAccount(ctx, "ag-1", validAccount("main"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := m.StableIdentity(ctx, "account", account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.DeleteAccount(ctx, "ag-1", account.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Resolve(ctx, raw); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("Resolve after delete = %v, want identity.ErrNotFound", err)
	}
}
