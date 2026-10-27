package ai_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

type fakeSource struct{ text string }

func (f *fakeSource) AgencyInstructions(context.Context, string, string) (string, error) {
	return f.text, nil
}

func roleIn(name string) ai.AIRoleInput {
	return ai.AIRoleInput{Name: name, Status: "enabled", FineTuning: "be brief"}
}

func mustRole(t *testing.T, m ai.Manager, agency string, in ai.AIRoleInput) ai.AIRole {
	t.Helper()
	r, err := m.CreateAIRole(ctx, agency, in)
	if err != nil {
		t.Fatalf("CreateAIRole: %v", err)
	}
	return r
}

func TestRoleCrudAndUniqueName(t *testing.T) {
	m, _, _ := newTestManager(t)
	r := mustRole(t, m, "ag-1", roleIn("scribe"))
	if !r.UseAgencyInstructions {
		t.Fatal("the switch should default to on")
	}
	if _, err := m.CreateAIRole(ctx, "ag-1", roleIn("scribe")); !errors.Is(err, ai.ErrDuplicateRoleName) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := m.CreateAIRole(ctx, "ag-2", roleIn("scribe")); err != nil {
		t.Fatalf("other agency: %v", err)
	}
	if _, err := m.GetAIRole(ctx, "ag-2", r.ID); !errors.Is(err, ai.ErrRoleNotFound) {
		t.Fatalf("cross agency: %v", err)
	}
	if l, _ := m.ListAIRoles(ctx, "ag-1"); len(l) != 1 {
		t.Fatalf("list = %d", len(l))
	}
	if _, err := m.DeleteAIRole(ctx, "ag-1", r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetAIRole(ctx, "ag-1", r.ID); !errors.Is(err, ai.ErrRoleNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestAgencyPartIsReadLiveOnEveryResolve(t *testing.T) {
	src := &fakeSource{text: "Agency v1"}
	m, _, _ := newTestManager(t, ai.WithInstructionSource(src))
	r := mustRole(t, m, "ag", roleIn("scribe"))
	got, _ := m.Instructions(ctx, "ag", r.ID)
	if got != "Agency v1\n\nbe brief" {
		t.Fatalf("first resolve: %q", got)
	}
	src.text = "Agency v2"
	got, _ = m.Instructions(ctx, "ag", r.ID)
	if got != "Agency v2\n\nbe brief" {
		t.Fatalf("after the agency changed: %q", got)
	}
	revs, _ := m.ListAIRoleRevisions(ctx, "ag", r.ID)
	if len(revs) != 0 {
		t.Fatalf("an agency change must not touch the role: %d revisions", len(revs))
	}
}

func TestSwitchOffDropsTheAgencyPart(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithInstructionSource(&fakeSource{text: "Agency"}))
	r := mustRole(t, m, "ag", roleIn("scribe"))
	off := false
	in := roleIn("scribe")
	in.UseAgencyInstructions = &off
	if _, err := m.UpdateAIRole(ctx, "ag", r.ID, "ed", in); err != nil {
		t.Fatal(err)
	}
	got, _ := m.Instructions(ctx, "ag", r.ID)
	if got != "be brief" {
		t.Fatalf("got %q", got)
	}
}

func TestFineTuningRequiredWhenSwitchOff(t *testing.T) {
	m, _, _ := newTestManager(t)
	off := false
	in := roleIn("scribe")
	in.FineTuning = ""
	in.UseAgencyInstructions = &off
	if _, err := m.CreateAIRole(ctx, "ag", in); !errors.Is(err, ai.ErrInvalidRole) {
		t.Fatalf("create: %v", err)
	}
	r := mustRole(t, m, "ag", roleIn("scribe"))
	in.Name = "scribe"
	if _, err := m.UpdateAIRole(ctx, "ag", r.ID, "ed", in); !errors.Is(err, ai.ErrInvalidRole) {
		t.Fatalf("update: %v", err)
	}
}

func TestFineTuningOverTheCapIsRefused(t *testing.T) {
	m, _, _ := newTestManager(t)
	in := roleIn("scribe")
	in.FineTuning = strings.Repeat("é", 20001)
	_, err := m.CreateAIRole(ctx, "ag", in)
	if !errors.Is(err, ai.ErrInvalidRole) || !strings.Contains(err.Error(), "longer than 20000 characters") {
		t.Fatalf("got %v", err)
	}
	in.FineTuning = strings.Repeat("é", 20000)
	if _, err := m.CreateAIRole(ctx, "ag", in); err != nil {
		t.Fatalf("at the cap: %v", err)
	}
}

func TestEachEditAddsOneRevisionWithItsEditor(t *testing.T) {
	m, _, _ := newTestManager(t)
	r := mustRole(t, m, "ag", roleIn("scribe"))
	in := roleIn("scribe")
	in.FineTuning = "be long"
	if _, err := m.UpdateAIRole(ctx, "ag", r.ID, "alice", in); err != nil {
		t.Fatal(err)
	}
	in.TokensPerDay = 10
	if _, err := m.UpdateAIRole(ctx, "ag", r.ID, "bob", in); err != nil {
		t.Fatal(err)
	}
	in.Description = "only a description"
	if _, err := m.UpdateAIRole(ctx, "ag", r.ID, "carol", in); err != nil {
		t.Fatal(err)
	}
	revs, _ := m.ListAIRoleRevisions(ctx, "ag", r.ID)
	if len(revs) != 2 {
		t.Fatalf("revisions = %d", len(revs))
	}
	by := map[string]ai.AIRoleRevision{}
	for _, v := range revs {
		by[v.SavedBy] = v
	}
	if by["alice"].FineTuning != "be brief" || by["bob"].FineTuning != "be long" || by["bob"].TokensPerDay != 0 {
		t.Fatalf("previous values wrong: %+v", by)
	}
}

func TestRestoreBringsTheOldTextBackAndIsARevision(t *testing.T) {
	m, _, _ := newTestManager(t)
	r := mustRole(t, m, "ag", roleIn("scribe"))
	in := roleIn("scribe")
	in.FineTuning = "be long"
	m.UpdateAIRole(ctx, "ag", r.ID, "alice", in)
	revs, _ := m.ListAIRoleRevisions(ctx, "ag", r.ID)
	got, err := m.RestoreAIRoleRevision(ctx, "ag", r.ID, revs[0].ID, "bob")
	if err != nil || got.FineTuning != "be brief" {
		t.Fatalf("restore: %+v %v", got, err)
	}
	after, _ := m.ListAIRoleRevisions(ctx, "ag", r.ID)
	if len(after) != 2 {
		t.Fatalf("revisions = %d", len(after))
	}
	found := false
	for _, v := range after {
		if v.SavedBy == "bob" && v.FineTuning == "be long" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the restore left no revision: %+v", after)
	}
	if _, err := m.RestoreAIRoleRevision(ctx, "ag", r.ID, "nope", "bob"); !errors.Is(err, ai.ErrRevisionNotFound) {
		t.Fatalf("unknown revision: %v", err)
	}
}

func TestTheLowerOfRoleAndAccountWinsInEachLimit(t *testing.T) {
	cases := []struct {
		name         string
		role, acct   ai.Limits
		prime        func(t *testing.T, m ai.Manager, acc, role string)
		wantRefusal  bool
		wantMaxToken int
	}{
		{"tokens per call, role lower", ai.Limits{TokensPerCall: 50}, ai.Limits{TokensPerCall: 100}, nil, false, 50},
		{"tokens per call, account lower", ai.Limits{TokensPerCall: 100}, ai.Limits{TokensPerCall: 50}, nil, false, 50},
		{"tokens per day, role lower", ai.Limits{TokensPerDay: 10}, ai.Limits{TokensPerDay: 1000}, use(1, 10, 0), true, 0},
		{"tokens per day, account lower", ai.Limits{TokensPerDay: 1000}, ai.Limits{TokensPerDay: 10}, use(1, 10, 0), true, 0},
		{"tokens per month, role lower", ai.Limits{TokensPerMonth: 10}, ai.Limits{TokensPerMonth: 1000}, use(1, 10, 0), true, 0},
		{"tokens per month, account lower", ai.Limits{TokensPerMonth: 1000}, ai.Limits{TokensPerMonth: 10}, use(1, 10, 0), true, 0},
		{"requests per day, role lower", ai.Limits{RequestsPerDay: 1}, ai.Limits{RequestsPerDay: 100}, use(1, 0, 0), true, 0},
		{"requests per day, account lower", ai.Limits{RequestsPerDay: 100}, ai.Limits{RequestsPerDay: 1}, use(1, 0, 0), true, 0},
		{"requests per minute, role lower", ai.Limits{RequestsPerMinute: 1}, ai.Limits{RequestsPerMinute: 100}, use(1, 0, 0), true, 0},
		{"requests per minute, account lower", ai.Limits{RequestsPerMinute: 100}, ai.Limits{RequestsPerMinute: 1}, use(1, 0, 0), true, 0},
		{"both higher stays allowed", ai.Limits{RequestsPerMinute: 100}, ai.Limits{RequestsPerMinute: 100}, use(1, 0, 0), false, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, _, _ := newTestManager(t)
			acc := mustAdd(t, m, "ag", "main")
			if _, err := m.SetAccountLimits(ctx, "ag", acc.ID, c.acct); err != nil {
				t.Fatal(err)
			}
			in := roleIn("scribe")
			in.TokensPerCall, in.TokensPerDay, in.TokensPerMonth = c.role.TokensPerCall, c.role.TokensPerDay, c.role.TokensPerMonth
			in.RequestsPerDay, in.RequestsPerMinute = c.role.RequestsPerDay, c.role.RequestsPerMinute
			r := mustRole(t, m, "ag", in)
			if c.prime != nil {
				c.prime(t, m, acc.ID, r.ID)
			}
			d, err := m.CheckLimits(ctx, "ag", ai.CheckInput{AccountID: acc.ID, RoleID: r.ID})
			if err != nil {
				t.Fatal(err)
			}
			if d.Allowed == c.wantRefusal {
				t.Fatalf("decision %+v", d)
			}
			if c.wantMaxToken != 0 && d.MaxTokens != c.wantMaxToken {
				t.Fatalf("max tokens = %d", d.MaxTokens)
			}
		})
	}
}

func use(calls, in, out int) func(*testing.T, ai.Manager, string, string) {
	return func(t *testing.T, m ai.Manager, acc, role string) {
		t.Helper()
		for i := 0; i < calls; i++ {
			if _, err := m.RecordUsage(ctx, "ag", ai.UsageInput{AccountID: acc, RoleID: role, TokensIn: in, TokensOut: out}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestALimitEditTakesEffectOnTheNextCheck(t *testing.T) {
	m, _, _ := newTestManager(t)
	acc := mustAdd(t, m, "ag", "main")
	r := mustRole(t, m, "ag", roleIn("scribe"))
	use(1, 0, 0)(t, m, acc.ID, r.ID)
	if d, _ := m.CheckLimits(ctx, "ag", ai.CheckInput{AccountID: acc.ID, RoleID: r.ID}); !d.Allowed {
		t.Fatalf("before: %+v", d)
	}
	in := roleIn("scribe")
	in.RequestsPerDay = 1
	m.UpdateAIRole(ctx, "ag", r.ID, "ed", in)
	if d, _ := m.CheckLimits(ctx, "ag", ai.CheckInput{AccountID: acc.ID, RoleID: r.ID}); d.Allowed {
		t.Fatalf("after: %+v", d)
	}
}
