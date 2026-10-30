package ai_test

import (
	"errors"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func TestSettingARolesLimitsReplacesOnlyTheLimitsAndKeepsARevision(t *testing.T) {
	m, _, _ := newTestManager(t)
	in := roleIn("farmers")
	in.Description = "answers farmers"
	in.TokensPerDay = ip(100000)
	role := mustRole(t, m, "ag", in)

	got, err := m.SetAIRoleLimits(ctx, "ag", role.ID, "ann", ai.Limits{TokensPerDay: ip(150000), RequestsPerMinute: ip(30)})
	if err != nil {
		t.Fatal(err)
	}
	if iv(got.TokensPerDay) != 150000 || iv(got.RequestsPerMinute) != 30 || got.RequestsPerDay != nil {
		t.Fatalf("limits after: %+v", got)
	}
	if got.Name != "farmers" || got.Description != "answers farmers" || got.FineTuning != "be brief" || !got.UseAgencyInstructions || got.Status != "enabled" {
		t.Fatalf("something besides the limits changed: %+v", got)
	}
	revisions, _ := m.ListAIRoleRevisions(ctx, "ag", role.ID)
	if len(revisions) != 1 || revisions[0].SavedBy != "ann" || iv(revisions[0].TokensPerDay) != 100000 {
		t.Fatalf("revisions: %+v", revisions)
	}
}

func TestSettingTheSameLimitsAgainKeepsNoRevision(t *testing.T) {
	m, _, _ := newTestManager(t)
	role := mustRole(t, m, "ag", roleIn("farmers"))
	for i := 0; i < 2; i++ {
		if _, err := m.SetAIRoleLimits(ctx, "ag", role.ID, "ann", ai.Limits{TokensPerDay: ip(5)}); err != nil {
			t.Fatal(err)
		}
	}
	if revisions, _ := m.ListAIRoleRevisions(ctx, "ag", role.ID); len(revisions) != 1 {
		t.Fatalf("revisions = %d, want 1", len(revisions))
	}
}

func TestARolesNewLimitIsInForceOnTheNextResolveAndRestoreBringsTheOldBack(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	role, _ := mappedRole(t, m, "ag", "farmers", "ar-1", 100, 1000)
	if _, err := m.SetAIRoleLimits(ctx, "ag", role.ID, "ann", ai.Limits{TokensPerDay: ip(500)}); err != nil {
		t.Fatal(err)
	}
	got, _ := m.ResolveAIRole(ctx, "ag", role.ID)
	for _, l := range got.Limits {
		if l.Limit == "daily token" && (l.Max != 500 || l.Source != "role") {
			t.Fatalf("daily token in force: %+v", l)
		}
	}
	revisions, _ := m.ListAIRoleRevisions(ctx, "ag", role.ID)
	if _, err := m.RestoreAIRoleRevision(ctx, "ag", role.ID, revisions[0].ID, "ann"); err != nil {
		t.Fatal(err)
	}
	got, _ = m.ResolveAIRole(ctx, "ag", role.ID)
	for _, l := range got.Limits {
		if l.Limit == "daily token" && l.Max != 100 {
			t.Fatalf("after the restore: %+v", l)
		}
	}
}

func TestSettingARolesLimitsRefusesNegativesAnotherAgencyAndAMissingRole(t *testing.T) {
	m, _, _ := newTestManager(t)
	role := mustRole(t, m, "ag-1", roleIn("farmers"))
	if _, err := m.SetAIRoleLimits(ctx, "ag-1", role.ID, "ann", ai.Limits{TokensPerDay: ip(-1)}); !errors.Is(err, ai.ErrInvalidRole) {
		t.Fatalf("negative: %v", err)
	}
	if _, err := m.SetAIRoleLimits(ctx, "ag-2", role.ID, "ann", ai.Limits{TokensPerDay: ip(1)}); !errors.Is(err, ai.ErrRoleNotFound) {
		t.Fatalf("another agency: %v", err)
	}
	if _, err := m.SetAIRoleLimits(ctx, "ag-1", "nope", "ann", ai.Limits{}); !errors.Is(err, ai.ErrRoleNotFound) {
		t.Fatalf("missing: %v", err)
	}
	held, _ := m.GetAIRole(ctx, "ag-1", role.ID)
	if held.TokensPerDay != nil {
		t.Fatalf("a refused call changed the role: %+v", held)
	}
}
