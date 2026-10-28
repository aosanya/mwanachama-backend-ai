package ai_test

import (
	"encoding/json"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func TestUsageRowsStoreTheWindowInTheWindowKindColumn(t *testing.T) {
	m, db, s := newTestManager(t)
	a, err := m.CreateAccount(ctx, "ag-1", validAccount("w"))
	if err != nil {
		t.Fatal(err)
	}
	rec(t, m, "ag-1", a.ID, "", 1, 1)
	o, ok := s.ByRole("usage")
	if !ok {
		t.Fatal("no usage object")
	}
	var kinds []string
	if err := db.Raw("select window_kind from " + s.TableFor(o) + " order by window_kind").Scan(&kinds).Error; err != nil {
		t.Fatalf("select window_kind: %v", err)
	}
	if len(kinds) != 3 || kinds[0] != "day" || kinds[1] != "minute" || kinds[2] != "month" {
		t.Fatalf("want the day, minute and month rows, got %v", kinds)
	}
}

func TestRecordUsageAnswerKeepsItsWindowKey(t *testing.T) {
	m, _, _ := newTestManager(t)
	a, err := m.CreateAccount(ctx, "ag-1", validAccount("w"))
	if err != nil {
		t.Fatal(err)
	}
	u, err := m.RecordUsage(ctx, "ag-1", ai.UsageInput{AccountID: a.ID, TokensIn: 1, TokensOut: 1})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	var answer map[string]any
	if err := json.Unmarshal(raw, &answer); err != nil {
		t.Fatal(err)
	}
	if answer["window"] != "day" {
		t.Fatalf("the answer's window key moved: %s", raw)
	}
	if _, renamed := answer["window_kind"]; renamed {
		t.Fatalf("the storage rename leaked into the answer: %s", raw)
	}
}
