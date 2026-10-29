package ai_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

type fakeLister struct {
	out   ai.ProbeOutcome
	names []string
	calls int
	seen  ai.Account
	key   string
}

func (f *fakeLister) FetchModels(_ context.Context, a ai.Account, key string) ai.ModelListing {
	f.calls++
	f.seen, f.key = a, key
	return ai.ModelListing{Outcome: f.out, Names: f.names}
}

func withKey(t *testing.T, m ai.Manager, agency, name string) ai.Account {
	t.Helper()
	a := mustAdd(t, m, agency, name)
	if _, err := m.SetAccountKey(ctx, agency, a.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
		t.Fatal(err)
	}
	return a
}

func shown(entries []ai.ModelEntry) string {
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		parts = append(parts, e.Provider+"/"+e.ModelName+"/"+e.Source)
	}
	return strings.Join(parts, " ")
}

func TestRefreshedModelsStandAboveTheStartingList(t *testing.T) {
	f := &fakeLister{names: []string{"m-b", "m-c"}}
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithModelLister(f))
	if err := m.SetStartingModels(ctx, "openai_compatible", []string{"m-a", "m-b"}); err != nil {
		t.Fatal(err)
	}
	a := withKey(t, m, "ag-1", "main")
	got, err := m.RefreshModels(ctx, "ag-1", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := "openai_compatible/m-b/provider openai_compatible/m-c/provider openai_compatible/m-a/starting"
	if shown(got) != want {
		t.Fatalf("got %s", shown(got))
	}
	read, _ := m.ListModels(ctx, "ag-1", "")
	if shown(read) != want {
		t.Fatalf("read %s", shown(read))
	}
}

func TestStartingListIsAllAnAgencyHasBeforeItRefreshes(t *testing.T) {
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithModelLister(&fakeLister{}))
	if err := m.SetStartingModels(ctx, "anthropic", []string{"s-1", "s-2"}); err != nil {
		t.Fatal(err)
	}
	got, err := m.ListModels(ctx, "ag-new", "")
	if err != nil || shown(got) != "anthropic/s-1/starting anthropic/s-2/starting" {
		t.Fatalf("got %s %v", shown(got), err)
	}
}

func TestRefreshedListBelongsToTheAgencyThatKeyedIt(t *testing.T) {
	f := &fakeLister{names: []string{"private-tune"}}
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithModelLister(f))
	a := withKey(t, m, "ag-1", "main")
	if _, err := m.RefreshModels(ctx, "ag-1", a.ID); err != nil {
		t.Fatal(err)
	}
	other, err := m.ListModels(ctx, "ag-2", "")
	if err != nil || len(other) != 0 {
		t.Fatalf("another agency saw %s %v", shown(other), err)
	}
	if _, err := m.RefreshModels(ctx, "ag-2", a.ID); !errors.Is(err, ai.ErrAccountNotFound) {
		t.Fatalf("another agency refreshed through the account: %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("lister called %d times", f.calls)
	}
}

func TestRefreshReplacesTheWholeListingOfThatProviderOnly(t *testing.T) {
	f := &fakeLister{names: []string{"old-1", "old-2"}}
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithModelLister(f))
	a := withKey(t, m, "ag-1", "main")
	if _, err := m.RefreshModels(ctx, "ag-1", a.ID); err != nil {
		t.Fatal(err)
	}
	g := validAccount("claude")
	g.Provider = "anthropic"
	other, err := m.CreateAccount(ctx, "ag-1", g)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetAccountKey(ctx, "ag-1", other.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
		t.Fatal(err)
	}
	f.names = []string{"claude-x"}
	if _, err := m.RefreshModels(ctx, "ag-1", other.ID); err != nil {
		t.Fatal(err)
	}
	f.names = []string{"new-1"}
	got, err := m.RefreshModels(ctx, "ag-1", a.ID)
	if err != nil || shown(got) != "openai_compatible/new-1/provider" {
		t.Fatalf("got %s %v", shown(got), err)
	}
	all, _ := m.ListModels(ctx, "ag-1", "")
	if shown(all) != "anthropic/claude-x/provider openai_compatible/new-1/provider" {
		t.Fatalf("all %s", shown(all))
	}
}

func TestListCanBeNarrowedToOneKind(t *testing.T) {
	m, _, _ := newTestManager(t)
	_ = m.SetStartingModels(ctx, "anthropic", []string{"a1"})
	_ = m.SetStartingModels(ctx, "gemini", []string{"g1"})
	got, err := m.ListModels(ctx, "ag-1", "gemini")
	if err != nil || shown(got) != "gemini/g1/starting" {
		t.Fatalf("got %s %v", shown(got), err)
	}
	if _, err := m.ListModels(ctx, "ag-1", "nope"); !errors.Is(err, ai.ErrUnsupportedKind) {
		t.Fatalf("unknown kind: %v", err)
	}
}

func TestAFailedRefreshLeavesTheListAsItWas(t *testing.T) {
	cases := []struct {
		name  string
		out   ai.ProbeOutcome
		names []string
		want  error
	}{
		{"refused", ai.ProbeRefused, nil, ai.ErrModelRefused},
		{"unreached", ai.ProbeUnreached, nil, ai.ErrModelUnreached},
		{"error", ai.ProbeError, nil, ai.ErrModelError},
		{"empty answer", ai.ProbeOK, nil, ai.ErrModelError},
		{"only unusable names", ai.ProbeOK, []string{"", "  ", "bad\nname"}, ai.ErrModelError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeLister{names: []string{"kept"}}
			m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithModelLister(f))
			a := withKey(t, m, "ag-1", "main")
			if _, err := m.RefreshModels(ctx, "ag-1", a.ID); err != nil {
				t.Fatal(err)
			}
			f.out, f.names = c.out, c.names
			if _, err := m.RefreshModels(ctx, "ag-1", a.ID); !errors.Is(err, c.want) {
				t.Fatalf("got %v want %v", err, c.want)
			}
			got, _ := m.ListModels(ctx, "ag-1", "")
			if shown(got) != "openai_compatible/kept/provider" {
				t.Fatalf("list changed to %s", shown(got))
			}
		})
	}
}

func TestRefreshAsksWithTheStoredKeyOfTheAccount(t *testing.T) {
	f := &fakeLister{names: []string{"m"}}
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithModelLister(f))
	a := withKey(t, m, "ag-1", "main")
	if _, err := m.RefreshModels(ctx, "ag-1", a.ID); err != nil {
		t.Fatal(err)
	}
	if f.key != syntheticKey || f.seen.ID != a.ID || f.seen.Provider != "openai_compatible" {
		t.Fatalf("lister saw %+v", f.seen)
	}
}

func TestRefreshWithoutAKeyDoesNotAskTheProvider(t *testing.T) {
	f := &fakeLister{names: []string{"m"}}
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithModelLister(f))
	a := mustAdd(t, m, "ag-1", "main")
	if _, err := m.RefreshModels(ctx, "ag-1", a.ID); !errors.Is(err, ai.ErrNoKey) {
		t.Fatalf("got %v", err)
	}
	if _, err := m.RefreshModels(ctx, "ag-1", "missing"); !errors.Is(err, ai.ErrAccountNotFound) {
		t.Fatalf("got %v", err)
	}
	if f.calls != 0 {
		t.Fatalf("lister called %d times", f.calls)
	}
}

func TestANameHoldingTheKeyIsNeverListed(t *testing.T) {
	f := &fakeLister{names: []string{"fine", "echo-" + syntheticKey}}
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithModelLister(f))
	a := withKey(t, m, "ag-1", "main")
	got, err := m.RefreshModels(ctx, "ag-1", a.ID)
	if err != nil || shown(got) != "openai_compatible/fine/provider" {
		t.Fatalf("got %s %v", shown(got), err)
	}
}

func TestListedNamesAreTrimmedDedupedAndCapped(t *testing.T) {
	names := []string{" a ", "a", "b", ""}
	for i := 0; i < 600; i++ {
		names = append(names, "n"+strings.Repeat("x", i%7)+string(rune('A'+i%26))+strings.Repeat("y", i/26))
	}
	f := &fakeLister{names: names}
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithModelLister(f))
	a := withKey(t, m, "ag-1", "main")
	got, err := m.RefreshModels(ctx, "ag-1", a.ID)
	if err != nil || len(got) != 500 {
		t.Fatalf("got %d %v", len(got), err)
	}
	seen := map[string]bool{}
	for _, e := range got {
		if seen[e.ModelName] || e.ModelName != strings.TrimSpace(e.ModelName) || e.ModelName == "" {
			t.Fatalf("bad entry %q", e.ModelName)
		}
		seen[e.ModelName] = true
	}
}

func TestStartingListRefusesWhatCannotBeServed(t *testing.T) {
	m, _, _ := newTestManager(t)
	if err := m.SetStartingModels(ctx, "nope", []string{"a"}); !errors.Is(err, ai.ErrUnsupportedKind) {
		t.Fatalf("kind: %v", err)
	}
	long := make([]string, 501)
	for i := range long {
		long[i] = "m" + strings.Repeat("z", i%5) + string(rune('a'+i%26))
	}
	for name, names := range map[string][]string{
		"blank":     {"a", " "},
		"control":   {"a\tb"},
		"too long":  {strings.Repeat("x", 201)},
		"too many":  long,
		"bad utf-8": {"a\xffb"},
	} {
		if err := m.SetStartingModels(ctx, "gemini", names); !errors.Is(err, ai.ErrInvalidModelList) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	got, _ := m.ListModels(ctx, "ag-1", "")
	if len(got) != 0 {
		t.Fatalf("a refused list was stored: %s", shown(got))
	}
}

func TestStartingListIsReplacedAndMayBeEmptied(t *testing.T) {
	m, _, _ := newTestManager(t)
	_ = m.SetStartingModels(ctx, "gemini", []string{"g1", "g2"})
	_ = m.SetStartingModels(ctx, "anthropic", []string{"a1"})
	if err := m.SetStartingModels(ctx, "gemini", []string{"g3", "g3"}); err != nil {
		t.Fatal(err)
	}
	got, _ := m.ListModels(ctx, "ag-1", "")
	if shown(got) != "anthropic/a1/starting gemini/g3/starting" {
		t.Fatalf("got %s", shown(got))
	}
	_ = m.SetStartingModels(ctx, "gemini", nil)
	got, _ = m.ListModels(ctx, "ag-1", "")
	if shown(got) != "anthropic/a1/starting" {
		t.Fatalf("emptied: %s", shown(got))
	}
}

func TestEntriesCarryTheirSourceAndTime(t *testing.T) {
	f := &fakeLister{names: []string{"m"}}
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithModelLister(f))
	_ = m.SetStartingModels(ctx, "openai_compatible", []string{"s"})
	a := withKey(t, m, "ag-1", "main")
	got, _ := m.RefreshModels(ctx, "ag-1", a.ID)
	if len(got) != 2 || got[0].AgencyID != "ag-1" || got[1].AgencyID != "" {
		t.Fatalf("agencies %+v", got)
	}
	for _, e := range got {
		if e.ID == "" || e.ListedAt == "" {
			t.Fatalf("entry %+v", e)
		}
	}
}
