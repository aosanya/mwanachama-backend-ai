package ai_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	ai "github.com/aosanya/mwanachama-backend-ai"
	"github.com/aosanya/mwanachama-backend-ai/models"
)

func dayRows(t *testing.T, db *gorm.DB, m ai.Manager) []models.UsageDay {
	t.Helper()
	s := testSpec(t)
	var found []models.UsageDay
	if err := db.Table(s.TableFor(mustObject(t, s, "usage_day"))).Order("day, model, kind, person").Find(&found).Error; err != nil {
		t.Fatalf("usage_day rows: %v", err)
	}
	return found
}

func dayManager(t *testing.T, lim ai.Limits) (ai.Manager, *gorm.DB, ai.Account, *clockBox) {
	t.Helper()
	c := &clockBox{t: time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)}
	m, db, _ := newTestManager(t, ai.WithClock(c.now))
	in := validAccount("u")
	in.TokensPerDay = lim.TokensPerDay
	a, err := m.CreateAccount(ctx, "ag-1", in)
	if err != nil {
		t.Fatal(err)
	}
	return m, db, a, c
}

func recordFor(t *testing.T, m ai.Manager, acc, role, kind, person string, in, out int) {
	t.Helper()
	if _, err := m.RecordUsage(ctx, "ag-1", ai.UsageInput{AccountID: acc, RoleID: role, Kind: kind, Person: person, TokensIn: in, TokensOut: out}); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}
}

func TestUsageIsKeptPerDayByKindPersonAndModel(t *testing.T) {
	m, db, a, c := dayManager(t, ai.Limits{})
	recordFor(t, m, a.ID, "", "chat", "actor-1", 10, 5)
	recordFor(t, m, a.ID, "", "chat", "actor-1", 20, 5)
	recordFor(t, m, a.ID, "", "test_run", "actor-2", 100, 50)
	recordFor(t, m, a.ID, "", "background_job", "", 7, 3)

	renamed := validAccount("u")
	renamed.ModelName = "second-model"
	if _, err := m.UpdateAccount(ctx, "ag-1", a.ID, renamed); err != nil {
		t.Fatal(err)
	}
	recordFor(t, m, a.ID, "", "chat", "actor-1", 1, 1)
	c.t = c.t.Add(24 * time.Hour)
	recordFor(t, m, a.ID, "", "chat", "actor-1", 2, 2)

	rows := dayRows(t, db, m)
	if len(rows) != 5 {
		t.Fatalf("%d rows: %+v", len(rows), rows)
	}
	want := map[string][3]int{
		"2026-10-27|synthetic-model|chat|actor-1":     {2, 30, 10},
		"2026-10-27|synthetic-model|test_run|actor-2": {1, 100, 50},
		"2026-10-27|synthetic-model|background_job|":  {1, 7, 3},
		"2026-10-27|second-model|chat|actor-1":        {1, 1, 1},
		"2026-10-28|second-model|chat|actor-1":        {1, 2, 2},
	}
	for _, r := range rows {
		key := r.Day + "|" + r.Model + "|" + r.Kind + "|" + r.Person
		got, ok := want[key]
		if !ok || got != [3]int{r.Requests, r.TokensIn, r.TokensOut} {
			t.Fatalf("%s: %+v want %v", key, r, got)
		}
	}
}

func TestUsageRefusesAKindThatIsNotOneOfTheFive(t *testing.T) {
	m, _, a, _ := dayManager(t, ai.Limits{})
	_, err := m.RecordUsage(ctx, "ag-1", ai.UsageInput{AccountID: a.ID, Kind: "gossip", TokensIn: 1})
	if err == nil || !strings.Contains(err.Error(), "kind of work") {
		t.Fatalf("got %v", err)
	}
}

func TestUsageWithNoKindIsCountedAsOtherAndStillCountsAgainstTheLimits(t *testing.T) {
	m, db, a, _ := dayManager(t, ai.Limits{TokensPerDay: ip(100)})
	recordFor(t, m, a.ID, "", "", "", 60, 60)
	rows := dayRows(t, db, m)
	if len(rows) != 1 || rows[0].Kind != "" {
		t.Fatalf("%+v", rows)
	}
	if d := check(t, m, "ag-1", a.ID, "", ai.Limits{}); d.Allowed {
		t.Fatal("the daily token limit did not count the call")
	}
}

func TestAChatAsAnAgentRoleWithNoKindIsRefusedBeforeTheModelIsCalled(t *testing.T) {
	srv, seen := fakeProvider(t, 200, textAnswer)
	m, _, a := chatManager(t, "openai_compatible", srv.URL, nil)
	role := mustRole(t, m, "ag-1", roleIn("scribe"))
	if _, err := m.SetAIRoleMapping(ctx, "ag-1", role.ID, ai.AIRoleMappingInput{AccountID: a.ID, AgencyRoleID: "ar-1"}); err != nil {
		t.Fatal(err)
	}
	in := listRolesConversation()
	in.RoleID = role.ID
	if _, err := m.ChatAccount(ctx, "ag-1", a.ID, in); err == nil || !strings.Contains(err.Error(), "kind of work") {
		t.Fatalf("got %v", err)
	}
	if seen.body != "" {
		t.Fatalf("the model was called with %q", seen.body)
	}
}

func TestAChatRecordsItsKindAndThePersonWhoStartedIt(t *testing.T) {
	srv, _ := fakeProvider(t, 200, textAnswer)
	m, db, a := chatManager(t, "openai_compatible", srv.URL, nil)
	role := mustRole(t, m, "ag-1", roleIn("scribe"))
	if _, err := m.SetAIRoleMapping(ctx, "ag-1", role.ID, ai.AIRoleMappingInput{AccountID: a.ID, AgencyRoleID: "ar-1"}); err != nil {
		t.Fatal(err)
	}
	in := listRolesConversation()
	in.RoleID, in.Kind, in.Person = role.ID, "workflow_run", "actor-7"
	if _, err := m.ChatAccount(ctx, "ag-1", a.ID, in); err != nil {
		t.Fatal(err)
	}
	rows := dayRows(t, db, m)
	if len(rows) != 1 || rows[0].Kind != "workflow_run" || rows[0].Person != "actor-7" || rows[0].RoleID != role.ID || rows[0].TokensIn != 11 || rows[0].TokensOut != 7 {
		t.Fatalf("%+v", rows)
	}
}

func TestTheGatewayRefusesACallThatNamesNoKindOfWork(t *testing.T) {
	fm := &fakeModel{reply: modelReply}
	h, _, _, _ := gatewaySetup(t, fm, 0)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"messages":[]}`)))
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "kind of work") || fm.calls != 0 {
		t.Fatalf("%d %s calls %d", rec.Code, rec.Body, fm.calls)
	}
}

func gatewayWithDB(t *testing.T) (http.Handler, *gorm.DB, ai.AIRole) {
	t.Helper()
	srv := httptest.NewServer(&fakeModel{reply: modelReply})
	t.Cleanup(srv.Close)
	m, db, _ := newTestManager(t, ai.WithMasterKey(master))
	acc, err := m.CreateAccount(ctx, "ag", ai.Account{Name: "a", Provider: "openai_compatible", BaseURL: srv.URL, ModelName: "synthetic", Status: "enabled"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetAccountKey(ctx, "ag", acc.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
		t.Fatal(err)
	}
	role := mustRole(t, m, "ag", roleIn("scribe"))
	if _, err := m.SetAIRoleMapping(ctx, "ag", role.ID, ai.AIRoleMappingInput{AccountID: acc.ID, AgencyRoleID: "ar-1"}); err != nil {
		t.Fatal(err)
	}
	h := ai.NewGateway(m, func(*http.Request) (string, string, bool) { return "ag", "ar-1", true }, nil)
	return h, db, role
}

func TestTheGatewayRecordsTheKindAndPersonItWasToldOf(t *testing.T) {
	h, db, role := gatewayWithDB(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"messages":[]}`))
	req.Header.Set("X-Usage-Kind", "agency_review")
	req.Header.Set("X-Usage-Person", "actor-3")
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rows := dayRows(t, db, nil)
	if len(rows) != 1 || rows[0].Kind != "agency_review" || rows[0].Person != "actor-3" || rows[0].RoleID != role.ID || rows[0].Model != "synthetic" {
		t.Fatalf("%+v", rows)
	}
}
