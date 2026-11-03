package ai_test

import (
	"errors"
	"testing"
	"time"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func at(c *clockBox, day string) {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		panic(err)
	}
	c.t = t.Add(10 * time.Hour)
}

func breakdownSetup(t *testing.T) (ai.Manager, ai.Account, ai.AIRole, *clockBox) {
	t.Helper()
	m, _, a, c := dayManager(t, ai.Limits{})
	role := mustRole(t, m, "ag-1", roleIn("scribe"))
	at(c, "2026-09-30")
	recordFor(t, m, a.ID, role.ID, "chat", "actor-1", 100, 50)
	at(c, "2026-10-03")
	recordFor(t, m, a.ID, role.ID, "chat", "actor-1", 10, 5)
	recordFor(t, m, a.ID, "", "background_job", "", 4, 1)
	at(c, "2026-10-21")
	recordFor(t, m, a.ID, role.ID, "test_run", "actor-2", 20, 10)
	at(c, "2026-10-27")
	recordFor(t, m, a.ID, role.ID, "chat", "actor-1", 30, 15)
	recordFor(t, m, a.ID, role.ID, "", "", 1, 1)
	return m, a, role, c
}

func total(rows []ai.BreakdownRow) (int, int, int) {
	var req, in, out int
	for _, r := range rows {
		req, in, out = req+r.Requests, in+r.TokensIn, out+r.TokensOut
	}
	return req, in, out
}

func TestTheBreakdownOfEachWindowCoversExactlyItsDays(t *testing.T) {
	m, _, _, _ := breakdownSetup(t)
	cases := []struct {
		window, from, to string
		requests, in     int
	}{
		{"today", "2026-10-27", "2026-10-27", 2, 31},
		{"7d", "2026-10-21", "2026-10-27", 3, 51},
		{"month", "2026-10-01", "2026-10-27", 5, 65},
		{"last_month", "2026-09-01", "2026-09-30", 1, 100},
	}
	for _, c := range cases {
		got, err := m.UsageBreakdown(ctx, "ag-1", c.window, "agent", "", "")
		if err != nil {
			t.Fatalf("%s: %v", c.window, err)
		}
		req, in, _ := total(got.Rows)
		if got.Window != c.window || got.From != c.from || got.To != c.to || req != c.requests || in != c.in {
			t.Fatalf("%s: %s to %s, %d requests, %d in, rows %+v", c.window, got.From, got.To, req, in, got.Rows)
		}
	}
}

func TestTheRowsOfTheMonthAddUpToWhatTheMonthlyLimitCountsSee(t *testing.T) {
	m, a, _, _ := breakdownSetup(t)
	got, err := m.UsageBreakdown(ctx, "ag-1", "month", "agent", "", "")
	if err != nil {
		t.Fatal(err)
	}
	report, err := m.ListUsage(ctx, "ag-1", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, in, out := total(got.Rows)
	for _, c := range report.Counts {
		if c.Field == "tokens_per_month" && c.Used != in+out {
			t.Fatalf("limits count %d, rows add up to %d", c.Used, in+out)
		}
	}
}

func TestEachRowNamesItsRoleItsKindItsPersonAndItsModel(t *testing.T) {
	m, _, role, _ := breakdownSetup(t)
	got, err := m.UsageBreakdown(ctx, "ag-1", "today", "person", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 2 {
		t.Fatalf("%+v", got.Rows)
	}
	chat, other := got.Rows[0], got.Rows[1]
	if chat.Day != "2026-10-27" || chat.RoleID != role.ID || chat.Role != "scribe" || chat.Kind != "chat" || chat.Person != "actor-1" || chat.Model != "synthetic-model" || chat.Requests != 1 || chat.TokensIn != 30 || chat.TokensOut != 15 {
		t.Fatalf("%+v", chat)
	}
	if other.Kind != "other" || other.Person != "" {
		t.Fatalf("a call that named no kind reads as other: %+v", other)
	}
}

func TestTheBreakdownKeepsOnlyTheRoleAndModelAsked(t *testing.T) {
	m, _, role, _ := breakdownSetup(t)
	got, _ := m.UsageBreakdown(ctx, "ag-1", "month", "agent", role.ID, "")
	if req, _, _ := total(got.Rows); req != 4 {
		t.Fatalf("role: %d requests", req)
	}
	got, _ = m.UsageBreakdown(ctx, "ag-1", "month", "agent", "", "no-such-model")
	if len(got.Rows) != 0 {
		t.Fatalf("model: %+v", got.Rows)
	}
}

func TestTheBreakdownNeverShowsAnotherAgencysUse(t *testing.T) {
	m, _, _, _ := breakdownSetup(t)
	got, err := m.UsageBreakdown(ctx, "ag-2", "month", "agent", "", "")
	if err != nil || len(got.Rows) != 0 {
		t.Fatalf("%v %+v", err, got.Rows)
	}
}

func TestTheBreakdownRefusesAWindowOrGroupItDoesNotKnow(t *testing.T) {
	m, _, _, _ := breakdownSetup(t)
	for _, c := range [][2]string{{"", "agent"}, {"year", "agent"}, {"month", "team"}} {
		if _, err := m.UsageBreakdown(ctx, "ag-1", c[0], c[1], "", ""); !errors.Is(err, ai.ErrInvalidAccount) {
			t.Fatalf("%v: %v", c, err)
		}
	}
}

func TestTheRowsComeOldestDayFirst(t *testing.T) {
	m, _, _, _ := breakdownSetup(t)
	got, _ := m.UsageBreakdown(ctx, "ag-1", "month", "agent", "", "")
	for i := 1; i < len(got.Rows); i++ {
		if got.Rows[i-1].Day > got.Rows[i].Day {
			t.Fatalf("%+v", got.Rows)
		}
	}
}

func TestTheBreakdownIsNotCutAtAHundredRows(t *testing.T) {
	m, _, a, _ := dayManager(t, ai.Limits{})
	for i := 0; i < 130; i++ {
		recordFor(t, m, a.ID, "", "chat", "actor-"+string(rune('A'+i/26))+string(rune('a'+i%26)), 1, 1)
	}
	got, err := m.UsageBreakdown(ctx, "ag-1", "today", "person", "", "")
	if err != nil || len(got.Rows) != 130 {
		t.Fatalf("%v: %d rows", err, len(got.Rows))
	}
}
