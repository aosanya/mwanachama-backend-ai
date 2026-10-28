package ai_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

const sentPrompt = "Reply with the single word OK."

func testedAccount(t *testing.T, f *fakeProber, setKey bool) (ai.Manager, ai.Account) {
	t.Helper()
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithProber(f))
	a := mustAdd(t, m, "ag-1", "p")
	if setKey {
		if _, err := m.SetAccountKey(ctx, "ag-1", a.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := m.TestAccount(ctx, "ag-1", a.ID, ai.KeyInput{})
	if err != nil {
		t.Fatal(err)
	}
	return m, got
}

func TestEachTestOutcomeRecordsThePromptTheReplyAndTheTime(t *testing.T) {
	cases := []struct {
		name      string
		fake      fakeProber
		setKey    bool
		wantReply string
		wantPrmpt string
		wantMs    int
	}{
		{"working", fakeProber{out: ai.ProbeOK, reply: "OK", elapsed: 42 * time.Millisecond}, true, "OK", sentPrompt, 42},
		{"refused", fakeProber{out: ai.ProbeRefused, reply: "must not show", elapsed: 7 * time.Millisecond}, true, "", sentPrompt, 7},
		{"unreached", fakeProber{out: ai.ProbeUnreached, reply: "must not show", elapsed: 20 * time.Second}, true, "", sentPrompt, 20000},
		{"error", fakeProber{out: ai.ProbeError, reply: "must not show", elapsed: 300 * time.Millisecond}, true, "", sentPrompt, 300},
		{"no key", fakeProber{out: ai.ProbeOK, reply: "must not show", elapsed: 5 * time.Second}, false, "", "", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := c.fake
			m, answer := testedAccount(t, &f, c.setKey)
			held, _ := m.GetAccount(ctx, "ag-1", answer.ID)
			list, _ := m.ListAccounts(ctx, "ag-1")
			for where, got := range map[string]ai.Account{"answer": answer, "stored": held, "listed": list[0]} {
				if got.LastTestPrompt != c.wantPrmpt || got.LastTestReply != c.wantReply || got.LastTestMs != c.wantMs {
					t.Errorf("%s: prompt %q reply %q ms %d", where, got.LastTestPrompt, got.LastTestReply, got.LastTestMs)
				}
			}
		})
	}
}

func TestAnAccountNeverTestedHasEmptyTestFields(t *testing.T) {
	m, _, _ := newTestManager(t)
	a := mustAdd(t, m, "ag-1", "fresh")
	if a.LastTestPrompt != "" || a.LastTestReply != "" || a.LastTestMs != 0 {
		t.Fatalf("%+v", a)
	}
	b, _ := json.Marshal(a)
	for _, key := range []string{`"last_test_prompt":""`, `"last_test_reply":""`, `"last_test_ms":0`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("%s missing from %s", key, b)
		}
	}
}

func TestTheShownReplyIsTrimmedWithoutControlCharactersAndCapped(t *testing.T) {
	long := strings.Repeat("é", 130)
	cases := map[string]struct{ raw, want string }{
		"trimmed":            {"  \n OK \t\n", "OK"},
		"control characters": {"O\x00K\x1b[0m\u0085!", "OK[0m!"},
		"newline inside":     {"one\ntwo", "onetwo"},
		"capped at 120":      {long, strings.Repeat("é", 120)},
		"cap then trimmed":   {strings.Repeat("a", 119) + "  b", strings.Repeat("a", 119)},
		"exactly 120":        {strings.Repeat("b", 120), strings.Repeat("b", 120)},
		"blanks first":       {strings.Repeat(" ", 10) + strings.Repeat("c", 120), strings.Repeat("c", 120)},
		"broken utf-8":       {"O\xffK", "OK"},
		"only blanks":        {" \n\t ", ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, got := testedAccount(t, &fakeProber{out: ai.ProbeOK, reply: c.raw}, true)
			if got.LastTestReply != c.want {
				t.Fatalf("got %q want %q", got.LastTestReply, c.want)
			}
		})
	}
}

func TestTheElapsedTimeIsWholeMillisecondsAndNeverNegative(t *testing.T) {
	for name, c := range map[string]struct {
		elapsed time.Duration
		want    int
	}{
		"rounds down": {1999 * time.Microsecond, 1},
		"under one":   {900 * time.Microsecond, 0},
		"negative":    {-5 * time.Second, 0},
	} {
		t.Run(name, func(t *testing.T) {
			_, got := testedAccount(t, &fakeProber{out: ai.ProbeOK, reply: "OK", elapsed: c.elapsed}, true)
			if got.LastTestMs != c.want {
				t.Fatalf("got %d want %d", got.LastTestMs, c.want)
			}
		})
	}
}

func TestAReplyHoldingTheKeyIsDroppedWhole(t *testing.T) {
	for name, reply := range map[string]string{
		"alone":  syntheticKey,
		"inside": "here is " + syntheticKey + " for you",
		"padded": strings.Repeat("x", 200) + syntheticKey,
	} {
		t.Run(name, func(t *testing.T) {
			m, got := testedAccount(t, &fakeProber{out: ai.ProbeOK, reply: reply}, true)
			held, _ := m.GetAccount(ctx, "ag-1", got.ID)
			b, _ := json.Marshal([]ai.Account{got, held})
			if got.LastTestReply != "" || strings.Contains(string(b), syntheticKey) {
				t.Fatalf("reply %q in %s", got.LastTestReply, b)
			}
			if got.LastTestResult != ai.TestWorking {
				t.Fatalf("result %q", got.LastTestResult)
			}
		})
	}
}

func TestReplacingTheKeyClearsThePromptTheReplyAndTheTime(t *testing.T) {
	f := &fakeProber{out: ai.ProbeOK, reply: "OK", elapsed: 30 * time.Millisecond}
	m, tested := testedAccount(t, f, true)
	if tested.LastTestPrompt == "" || tested.LastTestReply != "OK" || tested.LastTestMs != 30 {
		t.Fatalf("test: %+v", tested)
	}
	replaced, err := m.SetAccountKey(ctx, "ag-1", tested.ID, ai.KeyInput{Key: "sk-NEW-2"})
	if err != nil {
		t.Fatal(err)
	}
	held, _ := m.GetAccount(ctx, "ag-1", tested.ID)
	for where, got := range map[string]ai.Account{"answer": replaced, "stored": held} {
		if got.LastTestPrompt != "" || got.LastTestReply != "" || got.LastTestMs != 0 {
			t.Errorf("%s still carries the old test: %+v", where, got)
		}
	}
}

func TestARefusedKeyReplacementKeepsThePromptTheReplyAndTheTime(t *testing.T) {
	f := &fakeProber{out: ai.ProbeOK, reply: "OK", elapsed: 30 * time.Millisecond}
	m, tested := testedAccount(t, f, true)
	if _, err := m.SetAccountKey(ctx, "ag-1", tested.ID, ai.KeyInput{Key: " "}); err == nil {
		t.Fatal("a blank key was accepted")
	}
	held, _ := m.GetAccount(ctx, "ag-1", tested.ID)
	if held.LastTestPrompt != sentPrompt || held.LastTestReply != "OK" || held.LastTestMs != 30 {
		t.Fatalf("a refused key wiped the test: %+v", held)
	}
}

func TestOnlyTheTestOperationWritesTheTestFields(t *testing.T) {
	f := &fakeProber{out: ai.ProbeOK, reply: "OK", elapsed: 30 * time.Millisecond}
	m, tested := testedAccount(t, f, true)
	forged := validAccount("p")
	forged.LastTestPrompt, forged.LastTestReply, forged.LastTestMs = "forged", "forged", 99999
	updated, err := m.UpdateAccount(ctx, "ag-1", tested.ID, forged)
	if err != nil || updated.LastTestPrompt != sentPrompt || updated.LastTestReply != "OK" || updated.LastTestMs != 30 {
		t.Fatalf("update: %+v %v", updated, err)
	}
	created := validAccount("second")
	created.LastTestPrompt, created.LastTestReply, created.LastTestMs = "forged", "forged", 99999
	made, err := m.CreateAccount(ctx, "ag-1", created)
	if err != nil || made.LastTestPrompt != "" || made.LastTestReply != "" || made.LastTestMs != 0 {
		t.Fatalf("create: %+v %v", made, err)
	}
}

func TestTheHTTPProberReturnsTheModelsReplyAndTheTime(t *testing.T) {
	for kind, answer := range map[string]string{"openai_compatible": openAIAnswer, "anthropic": anthropicAnswer, "gemini": geminiAnswer} {
		srv, _ := fakeProvider(t, 200, answer)
		m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithProber(ai.NewHTTPProber()))
		made, err := m.CreateAccount(ctx, "ag-1", ai.Account{Name: kind, Provider: kind, BaseURL: srv.URL, ModelName: "m", Status: "enabled"})
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if _, err := m.SetAccountKey(ctx, "ag-1", made.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
			t.Fatal(err)
		}
		got, err := m.TestAccount(ctx, "ag-1", made.ID, ai.KeyInput{})
		if err != nil || got.LastTestResult != ai.TestWorking || got.LastTestReply != "hi" || got.LastTestPrompt != sentPrompt || got.LastTestMs < 0 {
			t.Fatalf("%s: %+v %v", kind, got, err)
		}
	}
}

func TestTheHTTPProberTimesAnAnswerThatTookAWhile(t *testing.T) {
	srv, _ := slowProvider(t, 60*time.Millisecond, openAIAnswer)
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithProber(ai.NewHTTPProber()))
	made, _ := m.CreateAccount(ctx, "ag-1", ai.Account{Name: "slow", Provider: "openai_compatible", BaseURL: srv.URL, ModelName: "m", Status: "enabled"})
	if _, err := m.SetAccountKey(ctx, "ag-1", made.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
		t.Fatal(err)
	}
	got, err := m.TestAccount(ctx, "ag-1", made.ID, ai.KeyInput{})
	if err != nil || got.LastTestMs < 60 || got.LastTestMs > 5000 {
		t.Fatalf("ms %d %v", got.LastTestMs, err)
	}
}

func slowProvider(t *testing.T, wait time.Duration, answer string) (*httptest.Server, *seenRequest) {
	t.Helper()
	seen := &seenRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(wait)
		seen.path = r.URL.Path
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(srv.Close)
	return srv, seen
}
