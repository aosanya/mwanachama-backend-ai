package ai_test

import (
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func TestAddingASecondModelKeepsTheFirstWithItsKeyAndItsSettings(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	first := validAccount("Muse Spark 1.3")
	first.TokensPerDay = ip(200000)
	first.ChatEnabled = true
	muse, err := m.CreateAccount(ctx, "ag", first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetAccountKey(ctx, "ag", muse.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
		t.Fatal(err)
	}
	glm := validAccount("GLM-5.3 (Hugging Face)")
	glm.ModelName = "zai-org/GLM-5.3:novita"
	added, err := m.CreateAccount(ctx, "ag", glm)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetAccountKey(ctx, "ag", added.ID, ai.KeyInput{Key: syntheticKey + "-2"}); err != nil {
		t.Fatal(err)
	}
	list, err := m.ListAccounts(ctx, "ag")
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %d, %v; want both models", len(list), err)
	}
	if list[0].ID != muse.ID || !list[0].KeySet || iv(list[0].TokensPerDay) != 200000 || !list[0].ChatEnabled {
		t.Fatalf("the first model changed: %+v", list[0])
	}
	if key, err := m.AccountKey(ctx, "ag", muse.ID); err != nil || key != syntheticKey {
		t.Fatalf("the first model's key changed: %q, %v", key, err)
	}
}

func TestTheSameModelFromTwoProvidersIsTwoAccounts(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	hf := validAccount("GLM-5.3 on Hugging Face")
	hf.ModelName = "GLM-5.3"
	baseten := validAccount("GLM-5.3 on Baseten")
	baseten.ModelName = "GLM-5.3"
	baseten.BaseURL = "https://baseten.example.test/v1"
	for _, a := range []ai.Account{hf, baseten} {
		if _, err := m.CreateAccount(ctx, "ag", a); err != nil {
			t.Fatalf("%s: %v", a.Name, err)
		}
	}
	if list, _ := m.ListAccounts(ctx, "ag"); len(list) != 2 {
		t.Fatalf("list = %d, want 2", len(list))
	}
}

func roleOnDefault(t *testing.T, m ai.Manager, agency, name string) ai.AIRole {
	t.Helper()
	return mustRole(t, m, agency, roleIn(name))
}

func TestARoleWithNoModelUsesTheHubsDefaultModel(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	main := mustAdd(t, m, "ag", "main")
	if _, err := m.SetDefaultAccount(ctx, "ag", main.ID); err != nil {
		t.Fatal(err)
	}
	r := roleOnDefault(t, m, "ag", "scribe")
	got, err := m.ResolveAIRole(ctx, "ag", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Account == nil || got.Account.ID != main.ID || got.AccountSource != "default" || got.Message != "" {
		t.Fatalf("got %+v", got)
	}
	if len(got.Limits) == 0 {
		t.Fatal("the limits in force are read off the default model too")
	}
}

func TestARoleWithItsOwnModelKeepsItEvenWhenTheHubHasADefault(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	main := mustAdd(t, m, "ag", "main")
	other := mustAdd(t, m, "ag", "other")
	if _, err := m.SetDefaultAccount(ctx, "ag", main.ID); err != nil {
		t.Fatal(err)
	}
	r := roleOnDefault(t, m, "ag", "scribe")
	if _, err := m.SetAIRoleMapping(ctx, "ag", r.ID, ai.AIRoleMappingInput{AccountID: other.ID}); err != nil {
		t.Fatal(err)
	}
	got, _ := m.ResolveAIRole(ctx, "ag", r.ID)
	if got.Account == nil || got.Account.ID != other.ID || got.AccountSource != "role" {
		t.Fatalf("got %+v", got)
	}
}

func TestARoleWithNoModelAndNoDefaultStillSaysNoAccountIsMapped(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	mustAdd(t, m, "ag", "main")
	r := roleOnDefault(t, m, "ag", "scribe")
	got, _ := m.ResolveAIRole(ctx, "ag", r.ID)
	if got.Account != nil || got.AccountSource != "" || got.Message != "no account is mapped" {
		t.Fatalf("got %+v", got)
	}
}

func TestAnotherAgencysDefaultModelIsNeverUsed(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	theirs := mustAdd(t, m, "ag-2", "theirs")
	if _, err := m.SetDefaultAccount(ctx, "ag-2", theirs.ID); err != nil {
		t.Fatal(err)
	}
	r := roleOnDefault(t, m, "ag-1", "scribe")
	got, _ := m.ResolveAIRole(ctx, "ag-1", r.ID)
	if got.Account != nil {
		t.Fatalf("got %+v", got)
	}
}

func TestRemovingAModelAgentsUseSendsThemToTheDefaultAndKeepsTheRoles(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	main := mustAdd(t, m, "ag", "main")
	used := mustAdd(t, m, "ag", "used")
	if _, err := m.SetDefaultAccount(ctx, "ag", main.ID); err != nil {
		t.Fatal(err)
	}
	r := roleOnDefault(t, m, "ag", "scribe")
	if _, err := m.SetAIRoleMapping(ctx, "ag", r.ID, ai.AIRoleMappingInput{AccountID: used.ID, AgencyRoleID: "ar-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.DeleteAccount(ctx, "ag", used.ID); err != nil {
		t.Fatal(err)
	}
	held, err := m.GetAIRole(ctx, "ag", r.ID)
	if err != nil || held.AccountID != "" || held.AgencyRoleID != "ar-1" {
		t.Fatalf("the role after the removal: %+v, %v", held, err)
	}
	got, _ := m.ResolveAIRole(ctx, "ag", r.ID)
	if got.Account == nil || got.Account.ID != main.ID || got.AccountSource != "default" {
		t.Fatalf("got %+v", got)
	}
}

func TestRemovingAModelOnlyReleasesTheRolesOfItsOwnAgency(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	used := mustAdd(t, m, "ag-1", "used")
	r := roleOnDefault(t, m, "ag-1", "scribe")
	if _, err := m.SetAIRoleMapping(ctx, "ag-1", r.ID, ai.AIRoleMappingInput{AccountID: used.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.DeleteAccount(ctx, "ag-2", used.ID); err == nil {
		t.Fatal("another agency removed the account")
	}
	held, _ := m.GetAIRole(ctx, "ag-1", r.ID)
	if held.AccountID != used.ID {
		t.Fatalf("the mapping changed: %+v", held)
	}
}
