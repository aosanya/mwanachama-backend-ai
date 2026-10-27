package ai_test

import (
	"testing"
	"time"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func TestOnlyAWeeklyLimitApplies(t *testing.T) {
	m, a, c := usageSetup(t, ai.Limits{TokensPerWeek: ip(100)})
	rec(t, m, "ag-1", a.ID, "", 60, 0)
	if d := check(t, m, "ag-1", a.ID, "", ai.Limits{}); !d.Allowed {
		t.Fatalf("under the weekly limit: %+v", d)
	}
	rec(t, m, "ag-1", a.ID, "", 60, 0)
	d := check(t, m, "ag-1", a.ID, "", ai.Limits{})
	if d.Allowed || d.Reason != "this account's weekly token limit is reached" {
		t.Fatalf("over the weekly limit: %+v", d)
	}
	c.t = c.t.Add(26 * time.Hour)
	if d := check(t, m, "ag-1", a.ID, "", ai.Limits{}); d.Allowed {
		t.Fatalf("a day later the week still counts: %+v", d)
	}
}

func TestWeeklyWindowRollsOffAfterSevenDays(t *testing.T) {
	m, a, c := usageSetup(t, ai.Limits{RequestsPerWeek: ip(1)})
	rec(t, m, "ag-1", a.ID, "", 1, 0)
	c.t = c.t.Add(6 * 24 * time.Hour)
	d := check(t, m, "ag-1", a.ID, "", ai.Limits{})
	if d.Allowed || d.Reason != "this account's weekly request limit is reached" {
		t.Fatalf("day six: %+v", d)
	}
	c.t = c.t.Add(24 * time.Hour)
	if d := check(t, m, "ag-1", a.ID, "", ai.Limits{}); !d.Allowed {
		t.Fatalf("day seven: %+v", d)
	}
}

func TestARoleCanHoldOnlyAWeeklyLimit(t *testing.T) {
	m, a, _ := usageSetup(t, ai.Limits{})
	rec(t, m, "ag-1", a.ID, "r1", 1, 0)
	d := check(t, m, "ag-1", a.ID, "r1", ai.Limits{RequestsPerWeek: ip(1)})
	if d.Allowed || d.Reason != "this AI role's weekly request limit is reached" {
		t.Fatalf("%+v", d)
	}
	if d := check(t, m, "ag-1", a.ID, "r2", ai.Limits{RequestsPerWeek: ip(1)}); !d.Allowed {
		t.Fatalf("another role is counted apart: %+v", d)
	}
}

func TestSetLimitsKeepsNullDistinctFromZero(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	made, err := m.CreateAccount(ctx, "ag-1", validAccount("n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetAccountLimits(ctx, "ag-1", made.ID, ai.Limits{TokensPerWeek: ip(5), TokensPerDay: ip(0)}); err != nil {
		t.Fatal(err)
	}
	read, err := m.GetAccount(ctx, "ag-1", made.ID)
	if err != nil || read.TokensPerDay == nil || *read.TokensPerDay != 0 || read.RequestsPerDay != nil || read.TokensPerWeek == nil || *read.TokensPerWeek != 5 {
		t.Fatalf("%+v %v", read, err)
	}
}
