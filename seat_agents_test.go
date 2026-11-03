package ai_test

import (
	"errors"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func TestTwoAgentsOnOneSeatAreBothListedOldestFirst(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	first, _ := mappedRole(t, m, "ag", "Farmers Agent 1", "seat-farmers", 0, 0)
	second, _ := mappedRole(t, m, "ag", "Farmers Agent 2", "seat-farmers", 0, 0)
	mappedRole(t, m, "ag", "Suppliers Agent 1", "seat-suppliers", 0, 0)

	listed, err := m.ListAIRolesOfSeat(ctx, "ag", "seat-farmers")
	if err != nil || len(listed) != 2 || listed[0].ID != first.ID || listed[1].ID != second.ID {
		t.Fatalf("listed %+v, %v", listed, err)
	}
	if all, _ := m.ListAIRolesOfSeat(ctx, "ag", ""); len(all) != 3 {
		t.Fatalf("unfiltered = %d", len(all))
	}
	if none, _ := m.ListAIRolesOfSeat(ctx, "ag", "seat-nobody"); len(none) != 0 {
		t.Fatalf("empty seat = %d", len(none))
	}
	if other, _ := m.ListAIRolesOfSeat(ctx, "other", "seat-farmers"); len(other) != 0 {
		t.Fatalf("another agency = %d", len(other))
	}
}

func TestTheSecondAgentOnASeatResolvesItsOwnSetup(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	first, firstAccount := mappedRole(t, m, "ag", "Farmers Agent 1", "seat-farmers", 500, 1000)
	second, secondAccount := mappedRole(t, m, "ag", "Farmers Agent 2", "seat-farmers", 200, 1000)
	if _, err := m.UpdateAIRole(ctx, "ag", second.ID, "ann", ai.AIRoleInput{Name: second.Name, Status: second.Status, FineTuning: "second agent only", UseAgencyInstructions: boolPtr(false), TokensPerCall: ip(200), TokensPerDay: ip(200)}); err != nil {
		t.Fatal(err)
	}

	got, err := m.ResolveAIRole(ctx, "ag", second.ID)
	if err != nil || got.AIRoleID != second.ID || got.Account == nil || got.Account.ID != secondAccount.ID {
		t.Fatalf("second: %+v, %v", got, err)
	}
	if got.Instructions != "second agent only" {
		t.Fatalf("instructions = %q", got.Instructions)
	}
	if perCall(got) != 200 {
		t.Fatalf("second agent's tokens per call = %d, want its own 200", perCall(got))
	}

	byseat, err := m.ResolveAgencyRole(ctx, "ag", "seat-farmers")
	if err != nil || byseat.AIRoleID != first.ID || byseat.Account.ID != firstAccount.ID || perCall(byseat) != 500 {
		t.Fatalf("by seat: %+v, %v", byseat, err)
	}
}

func perCall(r ai.Resolution) int {
	for _, limit := range r.Limits {
		if limit.Field == "tokens_per_call" {
			return limit.Max
		}
	}
	return -1
}

func boolPtr(b bool) *bool { return &b }

func TestChatAsASpecificAgentUsesItsLimitsAndCountsItsUsage(t *testing.T) {
	srv, seen := fakeProvider(t, 200, textAnswer)
	m, _, a := chatManager(t, "openai_compatible", srv.URL, nil)
	agent := mustRole(t, m, "ag-1", roleIn("Farmers Agent 2"))
	if _, err := m.SetAIRoleMapping(ctx, "ag-1", agent.ID, ai.AIRoleMappingInput{AccountID: a.ID, AgencyRoleID: "seat-farmers"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetAIRoleLimits(ctx, "ag-1", agent.ID, "ann", ai.Limits{TokensPerCall: ip(64), TokensPerDay: ip(10000)}); err != nil {
		t.Fatal(err)
	}
	in := listRolesConversation()
	in.RoleID = agent.ID

	if _, err := m.ChatAccount(ctx, "ag-1", a.ID, in); err != nil {
		t.Fatal(err)
	}

	if sent := asked(t, seen.body); sent["max_tokens"] != float64(64) {
		t.Fatalf("max_tokens = %v, want the agent's 64", sent["max_tokens"])
	}
	report, err := m.ListRoleUsage(ctx, "ag-1", a.ID, agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	used := 0
	for _, count := range report.Counts {
		if count.Field == "tokens_per_day" {
			used = count.Used
		}
	}
	if used != 18 {
		t.Fatalf("the agent's tokens today = %d, want 18", used)
	}
}

func TestChatAsAnAgentThatIsNotThereOrSpeaksThroughAnotherAccountIsRefused(t *testing.T) {
	srv, _ := fakeProvider(t, 200, textAnswer)
	m, _, a := chatManager(t, "openai_compatible", srv.URL, nil)
	other, err := m.CreateAccount(ctx, "ag-1", ai.Account{Name: "other", Provider: "openai_compatible", BaseURL: srv.URL, ModelName: "m2", Status: "enabled"})
	if err != nil {
		t.Fatal(err)
	}
	agent := mustRole(t, m, "ag-1", roleIn("Farmers Agent 3"))
	if _, err := m.SetAIRoleMapping(ctx, "ag-1", agent.ID, ai.AIRoleMappingInput{AccountID: other.ID, AgencyRoleID: "seat-farmers"}); err != nil {
		t.Fatal(err)
	}

	in := listRolesConversation()
	in.RoleID = "no-such-agent"
	if _, err := m.ChatAccount(ctx, "ag-1", a.ID, in); !errors.Is(err, ai.ErrRoleNotFound) {
		t.Fatalf("unknown agent: %v", err)
	}
	in.RoleID = agent.ID
	if _, err := m.ChatAccount(ctx, "ag-1", a.ID, in); !errors.Is(err, ai.ErrInvalidChat) {
		t.Fatalf("another account's agent: %v", err)
	}
}
