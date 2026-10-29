package ai_test

import (
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func recRun(t *testing.T, m ai.Manager, acc, role, run, step string, in, out int) {
	t.Helper()
	if _, err := m.RecordUsage(ctx, "ag-1", ai.UsageInput{AccountID: acc, RoleID: role, TestRunID: run, Step: step, TokensIn: in, TokensOut: out}); err != nil {
		t.Fatal(err)
	}
}

func checkRun(t *testing.T, m ai.Manager, acc, role, run string, rl ai.Limits) ai.Decision {
	t.Helper()
	d, err := m.CheckLimits(ctx, "ag-1", ai.CheckInput{AccountID: acc, RoleID: role, TestRunID: run, RoleLimits: rl})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestARunThatReachesItsTokensPerTestStopsWithBudgetReached(t *testing.T) {
	m, a, _ := usageSetup(t, ai.Limits{TokensPerTest: ip(100)})
	recRun(t, m, a.ID, "", "run-1", "step-1", 40, 20)
	if d := checkRun(t, m, a.ID, "", "run-1", ai.Limits{}); !d.Allowed || d.BudgetReached {
		t.Fatalf("under the budget: %+v", d)
	}
	recRun(t, m, a.ID, "", "run-1", "step-2", 30, 10)
	d := checkRun(t, m, a.ID, "", "run-1", ai.Limits{})
	if d.Allowed || !d.BudgetReached || d.Reason != "this test's tokens per test limit is reached" || d.TokensPerTest != 100 {
		t.Fatalf("at the budget: %+v", d)
	}
	if d := checkRun(t, m, a.ID, "", "run-2", ai.Limits{}); !d.Allowed {
		t.Fatalf("another run is counted apart: %+v", d)
	}
	if d := check(t, m, "ag-1", a.ID, "", ai.Limits{}); !d.Allowed || d.BudgetReached {
		t.Fatalf("a call for no run has no budget: %+v", d)
	}
}

func TestAStoppedRunKeepsTheStepsItDid(t *testing.T) {
	m, a, _ := usageSetup(t, ai.Limits{TokensPerTest: ip(50)})
	recRun(t, m, a.ID, "", "run-1", "step-1", 30, 0)
	recRun(t, m, a.ID, "", "run-1", "step-2", 30, 0)
	if d := checkRun(t, m, a.ID, "", "run-1", ai.Limits{}); !d.BudgetReached {
		t.Fatalf("%+v", d)
	}
	rep, err := m.ListTestRunUsage(ctx, "ag-1", "run-1")
	if err != nil || len(rep.Steps) != 2 || rep.Tokens != 60 {
		t.Fatalf("%+v %v", rep, err)
	}
}

func TestBlankAndZeroTokensPerTestDoNotStopARun(t *testing.T) {
	for name, limit := range map[string]*int{"blank": nil, "zero": ip(0)} {
		m, a, _ := usageSetup(t, ai.Limits{TokensPerTest: limit})
		recRun(t, m, a.ID, "", "run-1", "step-1", 1000000, 0)
		if d := checkRun(t, m, a.ID, "", "run-1", ai.Limits{}); !d.Allowed || d.BudgetReached || d.TokensPerTest != 0 {
			t.Fatalf("%s: %+v", name, d)
		}
	}
}

func TestBlankAndZeroStayWhatWasSaved(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	made, err := m.CreateAccount(ctx, "ag-1", validAccount("t"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetAccountLimits(ctx, "ag-1", made.ID, ai.Limits{TokensPerTest: ip(0), ScenariosAtOnce: nil}); err != nil {
		t.Fatal(err)
	}
	read, err := m.GetAccount(ctx, "ag-1", made.ID)
	if err != nil || read.TokensPerTest == nil || *read.TokensPerTest != 0 || read.ScenariosAtOnce != nil {
		t.Fatalf("%+v %v", read, err)
	}
	if _, err := m.SetAccountLimits(ctx, "ag-1", made.ID, ai.Limits{TokensPerTest: ip(-1)}); err == nil {
		t.Fatal("a negative tokens per test was accepted")
	}
	if _, err := m.SetAccountLimits(ctx, "ag-1", made.ID, ai.Limits{ScenariosAtOnce: ip(-1)}); err == nil {
		t.Fatal("a negative scenarios at once was accepted")
	}
}

func TestTheLowerOfTheAccountAndRoleTokensPerTestApplies(t *testing.T) {
	m, a, _ := usageSetup(t, ai.Limits{TokensPerTest: ip(100), ScenariosAtOnce: ip(4)})
	role := ai.Limits{TokensPerTest: ip(50), ScenariosAtOnce: ip(2)}
	recRun(t, m, a.ID, "r1", "run-1", "step-1", 50, 0)
	d := checkRun(t, m, a.ID, "r1", "run-1", role)
	if d.Allowed || !d.BudgetReached || d.TokensPerTest != 50 || d.ScenariosAtOnce != 2 {
		t.Fatalf("role is lower: %+v", d)
	}
	d = checkRun(t, m, a.ID, "r2", "run-1", ai.Limits{})
	if !d.Allowed || d.TokensPerTest != 100 || d.ScenariosAtOnce != 4 {
		t.Fatalf("a role with none leaves the account's: %+v", d)
	}
}

func TestScenariosAtOnceIsAnswered(t *testing.T) {
	m, a, _ := usageSetup(t, ai.Limits{ScenariosAtOnce: ip(3)})
	if d := check(t, m, "ag-1", a.ID, "", ai.Limits{}); !d.Allowed || d.ScenariosAtOnce != 3 {
		t.Fatalf("%+v", d)
	}
	m, a, _ = usageSetup(t, ai.Limits{})
	if d := check(t, m, "ag-1", a.ID, "", ai.Limits{}); d.ScenariosAtOnce != 0 {
		t.Fatalf("blank means no limit: %+v", d)
	}
}

func TestTestRunsCountInTheHubsOneTokenPool(t *testing.T) {
	m, a, _ := usageSetup(t, ai.Limits{TokensPerDay: ip(100)})
	recRun(t, m, a.ID, "", "run-1", "step-1", 60, 0)
	rec(t, m, "ag-1", a.ID, "agent", 30, 0)
	if d := check(t, m, "ag-1", a.ID, "", ai.Limits{}); !d.Allowed {
		t.Fatalf("90 of 100: %+v", d)
	}
	recRun(t, m, a.ID, "", "run-1", "step-2", 20, 0)
	d := check(t, m, "ag-1", a.ID, "agent", ai.Limits{})
	if d.Allowed || d.Reason != "this account's daily token limit is reached" {
		t.Fatalf("a test run spent the pool the agents share: %+v", d)
	}
	if d := checkRun(t, m, a.ID, "", "run-2", ai.Limits{}); d.Allowed {
		t.Fatalf("a test run is held to the pool too: %+v", d)
	}
}

func TestEachStepAndTheRunRecordTheirTokens(t *testing.T) {
	m, a, _ := usageSetup(t, ai.Limits{})
	recRun(t, m, a.ID, "", "run-1", "step-2", 5, 7)
	recRun(t, m, a.ID, "", "run-1", "step-1", 10, 20)
	recRun(t, m, a.ID, "", "run-1", "step-1", 1, 2)
	recRun(t, m, a.ID, "", "run-9", "step-1", 100, 100)
	rep, err := m.ListTestRunUsage(ctx, "ag-1", "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Steps) != 2 || rep.Steps[0].Step != "step-1" || rep.Steps[0].Tokens != 33 || rep.Steps[0].Requests != 2 || rep.Steps[1].Tokens != 12 {
		t.Fatalf("steps: %+v", rep.Steps)
	}
	if rep.Tokens != 45 || rep.TokensIn != 16 || rep.TokensOut != 29 || rep.Requests != 3 {
		t.Fatalf("run total: %+v", rep)
	}
	other, _ := m.ListTestRunUsage(ctx, "ag-2", "run-1")
	if other.Tokens != 0 || len(other.Steps) != 0 {
		t.Fatalf("another agency reads this run: %+v", other)
	}
}

func TestARoleKeepsTokensPerTestAndScenariosAtOnceThroughRevisions(t *testing.T) {
	m, _, _ := newTestManager(t)
	in := roleIn("tester")
	in.TokensPerTest, in.ScenariosAtOnce = ip(500), ip(2)
	r := mustRole(t, m, "ag", in)
	if r.TokensPerTest == nil || *r.TokensPerTest != 500 || r.ScenariosAtOnce == nil || *r.ScenariosAtOnce != 2 {
		t.Fatalf("%+v", r)
	}
	in.TokensPerTest, in.ScenariosAtOnce = ip(900), nil
	if _, err := m.UpdateAIRole(ctx, "ag", r.ID, "bob", in); err != nil {
		t.Fatal(err)
	}
	revs, _ := m.ListAIRoleRevisions(ctx, "ag", r.ID)
	if len(revs) != 1 || revs[0].TokensPerTest == nil || *revs[0].TokensPerTest != 500 {
		t.Fatalf("a change to the test limits is a revision: %+v", revs)
	}
	back, err := m.RestoreAIRoleRevision(ctx, "ag", r.ID, revs[0].ID, "bob")
	if err != nil || back.TokensPerTest == nil || *back.TokensPerTest != 500 || back.ScenariosAtOnce == nil || *back.ScenariosAtOnce != 2 {
		t.Fatalf("%+v %v", back, err)
	}
}
