package ai_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func mappedRole(t *testing.T, m ai.Manager, agency, name, agencyRole string, roleLimit, accountLimit int) (ai.AIRole, ai.Account) {
	t.Helper()
	acc, err := m.CreateAccount(ctx, agency, validAccount("acc-"+name))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetAccountKey(ctx, agency, acc.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
		t.Fatal(err)
	}
	if accountLimit > 0 {
		if _, err := m.SetAccountLimits(ctx, agency, acc.ID, ai.Limits{TokensPerCall: ip(accountLimit), TokensPerDay: ip(accountLimit)}); err != nil {
			t.Fatal(err)
		}
	}
	in := roleIn(name)
	in.TokensPerCall, in.TokensPerDay = ip(roleLimit), ip(roleLimit)
	r := mustRole(t, m, agency, in)
	r, err = m.SetAIRoleMapping(ctx, agency, r.ID, ai.AIRoleMappingInput{AccountID: acc.ID, AgencyRoleID: agencyRole})
	if err != nil {
		t.Fatal(err)
	}
	return r, acc
}

func TestResolveByAgencyRoleFindsTheRightAIRole(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	mappedRole(t, m, "ag", "one", "ar-1", 0, 0)
	two, _ := mappedRole(t, m, "ag", "two", "ar-2", 0, 0)
	got, err := m.ResolveAgencyRole(ctx, "ag", "ar-2")
	if err != nil || got.AIRoleID != two.ID || got.AgencyRoleID != "ar-2" {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got.Instructions != "be brief" || got.Account == nil {
		t.Fatalf("resolution: %+v", got)
	}
	if _, err := m.ResolveAgencyRole(ctx, "ag", "ar-9"); !errors.Is(err, ai.ErrRoleNotFound) {
		t.Fatalf("unknown: %v", err)
	}
}

func TestAnotherAgencysIdsAreRefused(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	r, acc := mappedRole(t, m, "ag-1", "one", "ar-1", 0, 0)
	if _, err := m.ResolveAIRole(ctx, "ag-2", r.ID); !errors.Is(err, ai.ErrRoleNotFound) {
		t.Fatalf("resolve other agency: %v", err)
	}
	if _, err := m.ResolveAgencyRole(ctx, "ag-2", "ar-1"); !errors.Is(err, ai.ErrRoleNotFound) {
		t.Fatalf("resolve by agency role: %v", err)
	}
	other := mustRole(t, m, "ag-2", roleIn("other"))
	if _, err := m.SetAIRoleMapping(ctx, "ag-2", other.ID, ai.AIRoleMappingInput{AccountID: acc.ID}); !errors.Is(err, ai.ErrAccountNotFound) {
		t.Fatalf("foreign account: %v", err)
	}
	if _, err := m.SetAIRoleMapping(ctx, "ag-2", r.ID, ai.AIRoleMappingInput{}); !errors.Is(err, ai.ErrRoleNotFound) {
		t.Fatalf("foreign role: %v", err)
	}
}

func TestResolveNeverCarriesTheKey(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	r, _ := mappedRole(t, m, "ag", "one", "ar-1", 0, 0)
	got, err := m.ResolveAIRole(ctx, "ag", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), syntheticKey) || strings.Contains(strings.ToLower(string(raw)), "sealed") {
		t.Fatalf("key in resolution: %s", raw)
	}
	if !got.Account.KeySet {
		t.Fatal("the account should say a key is set")
	}
}

func TestLimitInForceIsTheLowerNonZero(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	r, _ := mappedRole(t, m, "ag", "one", "ar-1", 500, 200)
	got, err := m.ResolveAIRole(ctx, "ag", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Limits) != 7 || got.Limits[0].Max != 200 || got.Limits[3].Max != 200 {
		t.Fatalf("limits: %+v", got.Limits)
	}
	r2, _ := mappedRole(t, m, "ag", "two", "ar-2", 100, 0)
	got, _ = m.ResolveAIRole(ctx, "ag", r2.ID)
	if got.Limits[0].Max != 100 {
		t.Fatalf("role only: %+v", got.Limits)
	}
	if got.Limits[4].Max != 0 {
		t.Fatalf("neither set should stay zero: %+v", got.Limits)
	}
}

func TestUsageSoFarIsCounted(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	r, acc := mappedRole(t, m, "ag", "one", "ar-1", 0, 0)
	if _, err := m.RecordUsage(ctx, "ag", ai.UsageInput{AccountID: acc.ID, RoleID: r.ID, TokensIn: 3, TokensOut: 4}); err != nil {
		t.Fatal(err)
	}
	got, _ := m.ResolveAIRole(ctx, "ag", r.ID)
	for _, l := range got.Limits {
		if l.Limit == "daily token" && l.Used != 7 {
			t.Fatalf("daily token used = %d", l.Used)
		}
	}
}

func TestNoAccountResolvesWithThePlainLine(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	r := mustRole(t, m, "ag", roleIn("bare"))
	got, err := m.ResolveAIRole(ctx, "ag", r.ID)
	if err != nil || got.Message != "no account is mapped" || got.Account != nil || got.Instructions != "be brief" {
		t.Fatalf("got %+v, %v", got, err)
	}
}
