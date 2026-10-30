package ai_test

import (
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func TestARefusalNamesTheLimitWhoseItIsAndHowFarItHasGone(t *testing.T) {
	m, a, _ := usageSetup(t, ai.Limits{TokensPerMonth: ip(4000000)})
	rec(t, m, "ag-1", a.ID, "", 4000000, 0)
	d := check(t, m, "ag-1", a.ID, "", ai.Limits{})
	if d.Allowed || d.Limit != "tokens_per_month" || d.Owner != "account" || d.Max != 4000000 || d.Used != 4000000 {
		t.Fatalf("decision: %+v", d)
	}
	if d.Reason != "this account's monthly token limit is reached" {
		t.Fatalf("the sentence changed: %q", d.Reason)
	}
}

func TestARoleRefusalNamesTheRoleAsTheOwner(t *testing.T) {
	m, a, _ := usageSetup(t, ai.Limits{})
	role := mustRole(t, m, "ag-1", roleIn("farmers"))
	if _, err := m.SetAIRoleLimits(ctx, "ag-1", role.ID, "ann", ai.Limits{RequestsPerMinute: ip(2)}); err != nil {
		t.Fatal(err)
	}
	rec(t, m, "ag-1", a.ID, role.ID, 1, 0)
	rec(t, m, "ag-1", a.ID, role.ID, 1, 0)
	d := check(t, m, "ag-1", a.ID, role.ID, ai.Limits{})
	if d.Allowed || d.Limit != "requests_per_minute" || d.Owner != "role" || d.Max != 2 || d.Used != 2 {
		t.Fatalf("decision: %+v", d)
	}
}

func TestEachLimitTheCheckCanRefuseOnIsNamedByTheKeyAnAccountSavesItUnder(t *testing.T) {
	cases := []struct {
		lim  ai.Limits
		in   int
		want string
	}{
		{ai.Limits{TokensPerDay: ip(10)}, 10, "tokens_per_day"},
		{ai.Limits{TokensPerMonth: ip(10)}, 10, "tokens_per_month"},
		{ai.Limits{TokensPerWeek: ip(10)}, 10, "tokens_per_week"},
		{ai.Limits{RequestsPerDay: ip(1)}, 1, "requests_per_day"},
		{ai.Limits{RequestsPerMinute: ip(1)}, 1, "requests_per_minute"},
		{ai.Limits{RequestsPerWeek: ip(1)}, 1, "requests_per_week"},
	}
	for _, c := range cases {
		m, a, _ := usageSetup(t, c.lim)
		rec(t, m, "ag-1", a.ID, "", c.in, 0)
		if d := check(t, m, "ag-1", a.ID, "", ai.Limits{}); d.Limit != c.want {
			t.Errorf("%s: decision names %q", c.want, d.Limit)
		}
	}
}

func TestAnAllowedDecisionNamesNoLimit(t *testing.T) {
	m, a, _ := usageSetup(t, ai.Limits{TokensPerDay: ip(100)})
	d := check(t, m, "ag-1", a.ID, "", ai.Limits{})
	if !d.Allowed || d.Limit != "" || d.Owner != "" || d.Max != 0 || d.Used != 0 {
		t.Fatalf("decision: %+v", d)
	}
}

func TestUsageAndResolutionRowsCarryTheKeyToSetEachLimitUnder(t *testing.T) {
	m, a, _ := usageSetup(t, ai.Limits{TokensPerDay: ip(100)})
	report, err := m.ListUsage(ctx, "ag-1", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{}
	for _, c := range report.Counts {
		fields[c.Limit] = c.Field
	}
	want := map[string]string{
		"requests per minute": "requests_per_minute", "requests per day": "requests_per_day", "daily token": "tokens_per_day",
		"monthly token": "tokens_per_month", "weekly request": "requests_per_week", "weekly token": "tokens_per_week",
	}
	for limit, field := range want {
		if fields[limit] != field {
			t.Errorf("usage %q has field %q, want %q", limit, fields[limit], field)
		}
	}
	keyed, _, _ := newTestManager(t, ai.WithMasterKey(master))
	role, _ := mappedRole(t, keyed, "ag-1", "farmers", "ar-1", 0, 0)
	got, _ := keyed.ResolveAIRole(ctx, "ag-1", role.ID)
	inForce := map[string]string{}
	for _, l := range got.Limits {
		inForce[l.Limit] = l.Field
	}
	want["tokens per call"] = "tokens_per_call"
	for limit, field := range want {
		if inForce[limit] != field {
			t.Errorf("resolution %q has field %q, want %q", limit, inForce[limit], field)
		}
	}
}
