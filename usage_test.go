package ai_test

import (
	"testing"
	"time"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

type clockBox struct{ t time.Time }

func (c *clockBox) now() time.Time { return c.t }

func usageSetup(t *testing.T, lim ai.Limits) (ai.Manager, ai.Account, *clockBox) {
	t.Helper()
	c := &clockBox{t: time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)}
	m, _, _ := newTestManager(t, ai.WithClock(c.now))
	in := validAccount("u")
	in.TokensPerCall, in.TokensPerDay, in.TokensPerMonth = lim.TokensPerCall, lim.TokensPerDay, lim.TokensPerMonth
	in.RequestsPerDay, in.RequestsPerMinute = lim.RequestsPerDay, lim.RequestsPerMinute
	in.TokensPerWeek, in.RequestsPerWeek = lim.TokensPerWeek, lim.RequestsPerWeek
	a, err := m.CreateAccount(ctx, "ag-1", in)
	if err != nil {
		t.Fatal(err)
	}
	return m, a, c
}

func rec(t *testing.T, m ai.Manager, ag, acc, role string, in, out int) {
	t.Helper()
	if _, err := m.RecordUsage(ctx, ag, ai.UsageInput{AccountID: acc, RoleID: role, TokensIn: in, TokensOut: out}); err != nil {
		t.Fatal(err)
	}
}

func check(t *testing.T, m ai.Manager, ag, acc, role string, rl ai.Limits) ai.Decision {
	t.Helper()
	d, err := m.CheckLimits(ctx, ag, ai.CheckInput{AccountID: acc, RoleID: role, RoleLimits: rl})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestCrossingCallIsTheFirstRefusedForEachLimit(t *testing.T) {
	cases := []struct {
		lim  ai.Limits
		in   int
		want string
	}{
		{ai.Limits{TokensPerDay: ip(100)}, 60, "this account's daily token limit is reached"},
		{ai.Limits{TokensPerMonth: ip(100)}, 60, "this account's monthly token limit is reached"},
		{ai.Limits{RequestsPerDay: ip(2)}, 1, "this account's daily request limit is reached"},
		{ai.Limits{RequestsPerMinute: ip(2)}, 1, "this account's per-minute request limit is reached"},
	}
	for _, c := range cases {
		m, a, _ := usageSetup(t, c.lim)
		rec(t, m, "ag-1", a.ID, "", c.in, 0)
		if d := check(t, m, "ag-1", a.ID, "", ai.Limits{}); !d.Allowed {
			t.Fatalf("%s: refused early: %+v", c.want, d)
		}
		rec(t, m, "ag-1", a.ID, "", c.in, 0)
		if d := check(t, m, "ag-1", a.ID, "", ai.Limits{}); d.Allowed || d.Reason != c.want {
			t.Fatalf("want %q got %+v", c.want, d)
		}
	}
}

func TestLowerOfRoleAndAccountWins(t *testing.T) {
	cases := []struct {
		acc, role ai.Limits
		in        int
		want      string
	}{
		{ai.Limits{TokensPerDay: ip(1000)}, ai.Limits{TokensPerDay: ip(10)}, 10, "this AI role's daily token limit is reached"},
		{ai.Limits{TokensPerMonth: ip(1000)}, ai.Limits{TokensPerMonth: ip(10)}, 10, "this AI role's monthly token limit is reached"},
		{ai.Limits{RequestsPerDay: ip(100)}, ai.Limits{RequestsPerDay: ip(1)}, 1, "this AI role's daily request limit is reached"},
		{ai.Limits{RequestsPerMinute: ip(100)}, ai.Limits{RequestsPerMinute: ip(1)}, 1, "this AI role's per-minute request limit is reached"},
		{ai.Limits{TokensPerDay: ip(10)}, ai.Limits{TokensPerDay: ip(1000)}, 10, "this account's daily token limit is reached"},
	}
	for _, c := range cases {
		m, a, _ := usageSetup(t, c.acc)
		rec(t, m, "ag-1", a.ID, "r1", c.in, 0)
		if d := check(t, m, "ag-1", a.ID, "r1", c.role); d.Allowed || d.Reason != c.want {
			t.Fatalf("want %q got %+v", c.want, d)
		}
	}
	m, a, _ := usageSetup(t, ai.Limits{TokensPerCall: ip(500)})
	if d := check(t, m, "ag-1", a.ID, "r1", ai.Limits{TokensPerCall: ip(200)}); !d.Allowed || d.MaxTokens != 200 {
		t.Fatalf("clamp: %+v", d)
	}
	if d := check(t, m, "ag-1", a.ID, "r1", ai.Limits{TokensPerCall: ip(900)}); d.MaxTokens != 500 {
		t.Fatalf("clamp: %+v", d)
	}
}

func TestRolloverCountersSurviveRestartAndAgenciesCountApart(t *testing.T) {
	c := &clockBox{t: time.Date(2026, 10, 31, 23, 59, 30, 0, time.UTC)}
	m, db, s := newTestManager(t, ai.WithClock(c.now))
	in := validAccount("u")
	in.TokensPerDay, in.TokensPerMonth = ip(10), ip(15)
	a, _ := m.CreateAccount(ctx, "ag-1", in)
	b, _ := m.CreateAccount(ctx, "ag-2", in)
	rec(t, m, "ag-1", a.ID, "", 10, 0)
	m2, err := ai.NewManager(db, s, ai.WithClock(c.now))
	if err != nil {
		t.Fatal(err)
	}
	if d := check(t, m2, "ag-1", a.ID, "", ai.Limits{}); d.Allowed {
		t.Fatalf("counter lost on restart: %+v", d)
	}
	if d := check(t, m2, "ag-2", b.ID, "", ai.Limits{}); !d.Allowed {
		t.Fatalf("agencies share a count: %+v", d)
	}
	c.t = time.Date(2026, 11, 1, 0, 0, 5, 0, time.UTC)
	if d := check(t, m2, "ag-1", a.ID, "", ai.Limits{}); !d.Allowed {
		t.Fatalf("day did not roll: %+v", d)
	}
	rec(t, m2, "ag-1", a.ID, "", 10, 0)
	rep, err := m2.ListUsage(ctx, "ag-1", a.ID)
	if err != nil || len(rep.Counts) != 6 || rep.Counts[2].Used != 10 || rep.Counts[3].Used != 10 || rep.Counts[3].Max != 15 {
		t.Fatalf("%+v %v", rep, err)
	}
}
