package ai_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

type fakeProber struct {
	out     ai.ProbeOutcome
	reply   string
	elapsed time.Duration
	seen    string
}

func (f *fakeProber) Probe(_ context.Context, _ ai.Account, key string) ai.ProbeResult {
	f.seen = key
	return ai.ProbeResult{Outcome: f.out, Reply: f.reply, Elapsed: f.elapsed}
}

func TestAccountTestEachReason(t *testing.T) {
	cases := []struct {
		name   string
		out    ai.ProbeOutcome
		setKey bool
		want   string
	}{
		{"working", ai.ProbeOK, true, "working"},
		{"nokey", ai.ProbeOK, false, "no key is set for this account"},
		{"refused", ai.ProbeRefused, true, "the model refused the key"},
		{"unreached", ai.ProbeUnreached, true, "the model could not be reached"},
		{"error", ai.ProbeError, true, "the model answered with an error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeProber{out: c.out}
			m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithProber(f))
			a := mustAdd(t, m, "ag-1", "p")
			if c.setKey {
				if _, err := m.SetAccountKey(ctx, "ag-1", a.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
					t.Fatal(err)
				}
			}
			got, err := m.TestAccount(ctx, "ag-1", a.ID)
			if err != nil || got.LastTestResult != c.want || got.LastTestedAt == "" {
				t.Fatalf("got %+v %v", got, err)
			}
			held, _ := m.GetAccount(ctx, "ag-1", a.ID)
			if held.LastTestResult != c.want {
				t.Fatalf("stored %q", held.LastTestResult)
			}
			b, _ := json.Marshal(held)
			if strings.Contains(string(b), syntheticKey) || strings.Contains(got.LastTestResult, syntheticKey) {
				t.Fatal("key leaked")
			}
			if c.setKey && f.seen != syntheticKey {
				t.Fatal("prober was not given the key")
			}
		})
	}
}
