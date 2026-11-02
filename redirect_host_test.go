package ai_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

const redirectedResult = "the model's address redirected the request, which is not followed"

var redirectingStatuses = []int{
	http.StatusMovedPermanently,
	http.StatusFound,
	http.StatusSeeOther,
	http.StatusTemporaryRedirect,
	http.StatusPermanentRedirect,
}

type sameHostRedirect struct {
	mu     sync.Mutex
	status int
	answer string
	keys   []string
	hops   int
}

func (s *sameHostRedirect) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys = append(s.keys, r.Header.Get("x-api-key")+r.Header.Get("x-goog-api-key"))
	hop, _ := strconv.Atoi(r.URL.Query().Get("hop"))
	if hop >= s.hops {
		_, _ = w.Write([]byte(s.answer))
		return
	}
	q := r.URL.Query()
	q.Set("hop", strconv.Itoa(hop+1))
	http.Redirect(w, r, r.URL.Path+"?"+q.Encode(), s.status)
}

func sameHostServer(t *testing.T, status int, answer string, hops int) (*httptest.Server, *sameHostRedirect) {
	t.Helper()
	h := &sameHostRedirect{status: status, answer: answer, hops: hops}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, h
}

func TestACallToAKeyInHeaderKindFollowsARedirectThatStaysOnItsHost(t *testing.T) {
	answers := map[string]string{"anthropic": anthropicAnswer, "gemini": geminiAnswer}
	for kind, answer := range answers {
		for _, status := range redirectingStatuses {
			t.Run(kind+"/"+strconv.Itoa(status), func(t *testing.T) {
				srv, seen := sameHostServer(t, status, answer, 1)
				acct := ai.Account{Provider: kind, BaseURL: srv.URL, ModelName: "m"}
				reply, out := ai.Call(ctx, http.DefaultClient, acct, syntheticKey, ai.Request{Prompt: "x", MaxTokens: 1})
				if out != ai.ProbeOK || reply.Text != "hi" {
					t.Fatalf("outcome %v, reply %+v, keys %q", out, reply, seen.keys)
				}
				if len(seen.keys) != 2 || seen.keys[0] != syntheticKey || seen.keys[1] != syntheticKey {
					t.Fatalf("the host heard %q", seen.keys)
				}
			})
		}
	}
}

func TestAModelListBehindASameHostRedirectIsRead(t *testing.T) {
	cases := map[string]string{
		"anthropic": `{"data":[{"id":"c-1"}]}`,
		"gemini":    `{"models":[{"name":"models/g-1"}]}`,
	}
	for kind, answer := range cases {
		for _, status := range redirectingStatuses {
			t.Run(kind+"/"+strconv.Itoa(status), func(t *testing.T) {
				srv, _ := sameHostServer(t, status, answer, 1)
				got := ai.NewHTTPModelLister().FetchModels(ctx, ai.Account{Provider: kind, BaseURL: srv.URL}, syntheticKey)
				if got.Outcome != ai.ProbeOK || len(got.Names) != 1 {
					t.Fatalf("got %+v", got)
				}
			})
		}
	}
}

func TestARedirectLoopOnOneHostEndsUnreachedAfterTenHops(t *testing.T) {
	srv, seen := sameHostServer(t, http.StatusTemporaryRedirect, anthropicAnswer, 1000)
	acct := ai.Account{Provider: "anthropic", BaseURL: srv.URL, ModelName: "m"}
	_, out := ai.Call(ctx, http.DefaultClient, acct, syntheticKey, ai.Request{Prompt: "x", MaxTokens: 1})
	if out != ai.ProbeUnreached || len(seen.keys) > 11 {
		t.Fatalf("outcome %v after %d requests", out, len(seen.keys))
	}
}

func TestARedirectedAccountTestSaysTheAddressRedirects(t *testing.T) {
	other, catcher := otherHost(t, anthropicAnswer)
	first := httptest.NewServer(redirectsTo{other.URL, http.StatusTemporaryRedirect})
	t.Cleanup(first.Close)
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	acc, err := m.CreateAccount(ctx, "ag", ai.Account{Name: "a", Provider: "anthropic", BaseURL: first.URL, ModelName: "synthetic", Status: "enabled"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.TestAccount(ctx, "ag", acc.ID, ai.KeyInput{Key: syntheticKey})
	if err != nil {
		t.Fatal(err)
	}
	if got.LastTestResult != redirectedResult || catcher.hits != 0 {
		t.Fatalf("result %q, the other host was reached %d times with %q", got.LastTestResult, catcher.hits, catcher.heard)
	}
}

func TestARefreshBehindACrossHostRedirectIsRefusedWithTheRedirectSentence(t *testing.T) {
	other, catcher := otherHost(t, `{"data":[{"id":"c-1"}]}`)
	first := httptest.NewServer(redirectsTo{other.URL, http.StatusTemporaryRedirect})
	t.Cleanup(first.Close)
	m, _, _ := newTestManager(t, ai.WithMasterKey(master), ai.WithModelLister(ai.NewHTTPModelLister()))
	acc, err := m.CreateAccount(ctx, "ag", ai.Account{Name: "a", Provider: "anthropic", BaseURL: first.URL, ModelName: "synthetic", Status: "enabled"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetAccountKey(ctx, "ag", acc.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
		t.Fatal(err)
	}
	_, err = m.RefreshModels(ctx, "ag", acc.ID)
	if err == nil || err.Error() != redirectedResult || catcher.hits != 0 {
		t.Fatalf("err %v, the other host was reached %d times", err, catcher.hits)
	}
}
