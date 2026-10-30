package ai_test

import (
	"errors"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func defaultsOf(t *testing.T, m ai.Manager, agency string) []string {
	t.Helper()
	list, err := m.ListAccounts(ctx, agency)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, a := range list {
		if a.IsDefault {
			out = append(out, a.Name)
		}
	}
	return out
}

func TestWithNoDefaultMarkedTheLookupSaysWhereToMarkOne(t *testing.T) {
	m, _, _ := newTestManager(t)
	mustAdd(t, m, "ag-1", "main")
	_, err := m.GetDefaultAccount(ctx, "ag-1")
	if !errors.Is(err, ai.ErrNoDefaultAccount) {
		t.Fatalf("got %v", err)
	}
	if err.Error() != "Mark a default AI account under Models before running a test" {
		t.Fatalf("message %q", err.Error())
	}
	if got := defaultsOf(t, m, "ag-1"); len(got) != 0 {
		t.Fatalf("a new account is not the default: %v", got)
	}
}

func TestMarkingASecondAccountMovesTheDefault(t *testing.T) {
	m, _, _ := newTestManager(t)
	first := mustAdd(t, m, "ag-1", "first")
	second := mustAdd(t, m, "ag-1", "second")
	other := mustAdd(t, m, "ag-2", "elsewhere")
	if _, err := m.SetDefaultAccount(ctx, "ag-2", other.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := m.SetDefaultAccount(ctx, "ag-1", first.ID); err != nil || !got.IsDefault {
		t.Fatalf("%+v %v", got, err)
	}
	if got, err := m.GetDefaultAccount(ctx, "ag-1"); err != nil || got.ID != first.ID {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := m.SetDefaultAccount(ctx, "ag-1", second.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := m.GetDefaultAccount(ctx, "ag-1"); err != nil || got.ID != second.ID {
		t.Fatalf("%+v %v", got, err)
	}
	if got := defaultsOf(t, m, "ag-1"); len(got) != 1 || got[0] != "second" {
		t.Fatalf("one default at a time: %v", got)
	}
	if got := defaultsOf(t, m, "ag-2"); len(got) != 1 {
		t.Fatalf("another agency's default moved: %v", got)
	}
	if read, _ := m.GetAccount(ctx, "ag-1", first.ID); read.IsDefault {
		t.Fatal("the first account still reads as the default")
	}
}

func TestMarkingAnAccountThatIsNotThereIsRefusedAndKeepsTheDefault(t *testing.T) {
	m, _, _ := newTestManager(t)
	first := mustAdd(t, m, "ag-1", "first")
	foreign := mustAdd(t, m, "ag-2", "foreign")
	if _, err := m.SetDefaultAccount(ctx, "ag-1", first.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"missing", foreign.ID} {
		if _, err := m.SetDefaultAccount(ctx, "ag-1", id); !errors.Is(err, ai.ErrAccountNotFound) {
			t.Fatalf("%s: %v", id, err)
		}
	}
	if got, err := m.GetDefaultAccount(ctx, "ag-1"); err != nil || got.ID != first.ID {
		t.Fatalf("a refusal moved the default: %+v %v", got, err)
	}
}

func TestSavingAnAccountNeitherMarksNorUnmarksTheDefault(t *testing.T) {
	m, _, _ := newTestManager(t)
	a := mustAdd(t, m, "ag-1", "main")
	if _, err := m.SetDefaultAccount(ctx, "ag-1", a.ID); err != nil {
		t.Fatal(err)
	}
	in := validAccount("main")
	in.ModelName = "changed"
	if got, err := m.UpdateAccount(ctx, "ag-1", a.ID, in); err != nil || !got.IsDefault {
		t.Fatalf("update dropped the mark: %+v %v", got, err)
	}
	if got, err := m.SetAccountLimits(ctx, "ag-1", a.ID, ai.Limits{}); err != nil || !got.IsDefault {
		t.Fatalf("limits dropped the mark: %+v %v", got, err)
	}
	in = validAccount("sneaky")
	in.IsDefault = true
	made, err := m.CreateAccount(ctx, "ag-1", in)
	if err != nil || made.IsDefault {
		t.Fatalf("a new account claimed the default: %+v %v", made, err)
	}
	if got, _ := m.GetDefaultAccount(ctx, "ag-1"); got.ID != a.ID {
		t.Fatalf("the default moved: %+v", got)
	}
}

func TestRemovingTheDefaultLeavesNoneMarked(t *testing.T) {
	m, _, _ := newTestManager(t)
	a := mustAdd(t, m, "ag-1", "main")
	if _, err := m.SetDefaultAccount(ctx, "ag-1", a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.DeleteAccount(ctx, "ag-1", a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetDefaultAccount(ctx, "ag-1"); !errors.Is(err, ai.ErrNoDefaultAccount) {
		t.Fatalf("got %v", err)
	}
}
