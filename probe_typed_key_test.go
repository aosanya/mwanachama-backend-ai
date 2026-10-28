package ai_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

const typedKey = "sk-TYPED-not-saved-9f8e7d6c"

func keyedAccount(t *testing.T, f *fakeProber) (ai.Manager, ai.Account) {
	t.Helper()
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithProber(f))
	a := mustAdd(t, m, "ag-1", "p")
	if _, err := m.SetAccountKey(ctx, "ag-1", a.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
		t.Fatal(err)
	}
	return m, a
}

func TestATypedKeyIsTheOneTheModelIsAsked(t *testing.T) {
	f := &fakeProber{out: ai.ProbeOK, reply: "OK", elapsed: 90 * time.Millisecond}
	m, a := keyedAccount(t, f)
	got, err := m.TestAccount(ctx, "ag-1", a.ID, ai.KeyInput{Key: typedKey})
	if err != nil || f.seen != typedKey {
		t.Fatalf("prober saw %q, err %v", f.seen, err)
	}
	if got.LastTestResult != ai.TestWorking || got.LastTestReply != "OK" || got.LastTestMs != 90 || got.LastTestPrompt != sentPrompt {
		t.Fatalf("answer %+v", got)
	}
}

func TestATypedKeyTestLeavesTheSavedKeyAndTheStoredResultAlone(t *testing.T) {
	f := &fakeProber{out: ai.ProbeRefused, elapsed: 30 * time.Millisecond}
	m, a := keyedAccount(t, f)
	before, err := m.GetAccount(ctx, "ag-1", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.TestAccount(ctx, "ag-1", a.ID, ai.KeyInput{Key: typedKey}); err != nil {
		t.Fatal(err)
	}
	after, _ := m.GetAccount(ctx, "ag-1", a.ID)
	if after != before {
		t.Fatalf("stored account changed: %+v then %+v", before, after)
	}
	saved, err := m.AccountKey(ctx, "ag-1", a.ID)
	if err != nil || saved != syntheticKey {
		t.Fatalf("saved key %q %v", saved, err)
	}
}

func TestATypedKeyCanBeTestedBeforeAnyKeyIsSavedAndWithoutAMasterKey(t *testing.T) {
	f := &fakeProber{out: ai.ProbeOK, reply: "OK"}
	m, _, _ := newTestManager(t, ai.WithProber(f))
	a := mustAdd(t, m, "ag-1", "p")
	got, err := m.TestAccount(ctx, "ag-1", a.ID, ai.KeyInput{Key: typedKey})
	if err != nil || got.LastTestResult != ai.TestWorking || f.seen != typedKey {
		t.Fatalf("got %+v %v, prober saw %q", got, err, f.seen)
	}
}

func TestABlankOrOversizedTypedKeyIsRefusedWithoutAsking(t *testing.T) {
	for name, key := range map[string]string{"blank": " \t ", "oversized": strings.Repeat("k", 4097)} {
		t.Run(name, func(t *testing.T) {
			f := &fakeProber{out: ai.ProbeOK}
			m, a := keyedAccount(t, f)
			if _, err := m.TestAccount(ctx, "ag-1", a.ID, ai.KeyInput{Key: key}); !errors.Is(err, ai.ErrInvalidKey) {
				t.Fatalf("err %v", err)
			}
			if f.seen != "" {
				t.Fatalf("prober was asked with %q", f.seen)
			}
		})
	}
}

func TestATypedKeyTestOfAnotherAgencysAccountIsNotFound(t *testing.T) {
	f := &fakeProber{out: ai.ProbeOK}
	m, a := keyedAccount(t, f)
	if _, err := m.TestAccount(ctx, "ag-2", a.ID, ai.KeyInput{Key: typedKey}); !errors.Is(err, ai.ErrAccountNotFound) {
		t.Fatalf("err %v", err)
	}
	if f.seen != "" {
		t.Fatalf("prober was asked with %q", f.seen)
	}
}

func TestAReplyThatEchoesTheTypedKeyIsDropped(t *testing.T) {
	f := &fakeProber{out: ai.ProbeOK, reply: "your key is " + typedKey}
	m, a := keyedAccount(t, f)
	got, err := m.TestAccount(ctx, "ag-1", a.ID, ai.KeyInput{Key: typedKey})
	if err != nil || got.LastTestReply != "" || strings.Contains(got.LastTestResult, typedKey) {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestNoTypedKeyStillTestsTheSavedKeyAndStoresTheResult(t *testing.T) {
	f := &fakeProber{out: ai.ProbeOK, reply: "OK"}
	m, a := keyedAccount(t, f)
	if _, err := m.TestAccount(ctx, "ag-1", a.ID, ai.KeyInput{}); err != nil || f.seen != syntheticKey {
		t.Fatalf("prober saw %q, err %v", f.seen, err)
	}
	held, _ := m.GetAccount(ctx, "ag-1", a.ID)
	if held.LastTestResult != ai.TestWorking || held.LastTestedAt == "" {
		t.Fatalf("stored %+v", held)
	}
}
