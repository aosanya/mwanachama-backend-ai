package ai_test

import (
	"context"
	"reflect"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

type partsSource struct {
	fakeSource
	parts []ai.InstructionPart
}

func (p *partsSource) AgencyInstructionParts(context.Context, string, string) ([]ai.InstructionPart, error) {
	return p.parts, nil
}

func TestPartsFollowTheSourceThenFineTuning(t *testing.T) {
	src := &partsSource{fakeSource: fakeSource{text: "whole"}, parts: []ai.InstructionPart{
		{Source: "seat_description", Text: "seat"},
		{Source: "objectives", Text: ""},
		{Source: "values_rules", Text: "rules"},
	}}
	m, _, _ := newTestManager(t, ai.WithInstructionSource(src))
	r := mustRole(t, m, "ag", roleIn("p"))
	got, err := m.ResolveAIRole(ctx, "ag", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []ai.InstructionPart{{Source: "seat_description", Text: "seat"}, {Source: "values_rules", Text: "rules"}, {Source: "fine_tuning", Text: "be brief"}}
	if !reflect.DeepEqual(got.Parts, want) {
		t.Fatalf("parts: %+v", got.Parts)
	}
}

func TestPartsFallBackToOneAgencyPart(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithInstructionSource(&fakeSource{text: "whole\n"}))
	r := mustRole(t, m, "ag", roleIn("p"))
	got, _ := m.ResolveAIRole(ctx, "ag", r.ID)
	want := []ai.InstructionPart{{Source: "agency", Text: "whole"}, {Source: "fine_tuning", Text: "be brief"}}
	if !reflect.DeepEqual(got.Parts, want) {
		t.Fatalf("parts: %+v", got.Parts)
	}
}

func TestPartsWithoutSourceOrSwitchHoldOnlyFineTuning(t *testing.T) {
	m, _, _ := newTestManager(t)
	r := mustRole(t, m, "ag", roleIn("p"))
	got, _ := m.ResolveAIRole(ctx, "ag", r.ID)
	if !reflect.DeepEqual(got.Parts, []ai.InstructionPart{{Source: "fine_tuning", Text: "be brief"}}) {
		t.Fatalf("parts: %+v", got.Parts)
	}
}

func TestLimitSourceNamesWhoSetsIt(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	r, _ := mappedRole(t, m, "ag", "one", "ar-1", 500, 200)
	got, _ := m.ResolveAIRole(ctx, "ag", r.ID)
	if got.Limits[0].Source != "account" || got.Limits[3].Source != "account" {
		t.Fatalf("account lower: %+v", got.Limits)
	}
	r2, _ := mappedRole(t, m, "ag", "two", "ar-2", 100, 0)
	got, _ = m.ResolveAIRole(ctx, "ag", r2.ID)
	if got.Limits[0].Source != "role" || got.Limits[4].Source != "none" {
		t.Fatalf("role or none: %+v", got.Limits)
	}
	r3, _ := mappedRole(t, m, "ag", "three", "ar-3", 0, 300)
	got, _ = m.ResolveAIRole(ctx, "ag", r3.ID)
	if got.Limits[0].Source != "account" {
		t.Fatalf("account only: %+v", got.Limits)
	}
}
