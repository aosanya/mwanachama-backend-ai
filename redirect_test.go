package ai_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

const redirectedSentence = "the model's address redirected the request, which is not followed"

var redirectStatuses = []int{
	http.StatusMovedPermanently,
	http.StatusFound,
	http.StatusTemporaryRedirect,
	http.StatusPermanentRedirect,
}

type redirectsTo struct {
	target string
	status int
}

func (r redirectsTo) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, r.target+req.URL.Path, r.status)
}

type keyCatcher struct {
	mu    sync.Mutex
	hits  int
	heard []string
	reply string
}

func (k *keyCatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	k.mu.Lock()
	k.hits++
	k.heard = append(k.heard, r.Header.Get("x-api-key")+r.Header.Get("x-goog-api-key"))
	k.mu.Unlock()
	_, _ = w.Write([]byte(k.reply))
}

func otherHost(t *testing.T, reply string) (*httptest.Server, *keyCatcher) {
	t.Helper()
	catcher := &keyCatcher{reply: reply}
	srv := httptest.NewServer(catcher)
	t.Cleanup(srv.Close)
	return srv, catcher
}

func TestAClaudeKeyIsNeverSentOnToAnotherHostByTheGateway(t *testing.T) {
	for _, status := range redirectStatuses {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			other, catcher := otherHost(t, claudeReply)
			h, _, _, _ := gatewaySetupKind(t, "anthropic", redirectsTo{other.URL, status}, 0)
			rec := post(h, `{"messages":[{"role":"user","content":"x"}]}`)
			if catcher.hits != 0 {
				t.Fatalf("the other host was reached %d times with %q", catcher.hits, catcher.heard)
			}
			if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), redirectedSentence) {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			if strings.Contains(rec.Body.String(), syntheticKey) || strings.Contains(rec.Body.String(), other.URL) {
				t.Fatalf("the answer carries a key or the target: %s", rec.Body)
			}
		})
	}
}

func TestACallToAKeyInHeaderKindDoesNotFollowARedirect(t *testing.T) {
	answers := map[string]string{"anthropic": anthropicAnswer, "gemini": geminiAnswer}
	for kind, answer := range answers {
		for _, status := range redirectStatuses {
			t.Run(kind+"/"+strconv.Itoa(status), func(t *testing.T) {
				other, catcher := otherHost(t, answer)
				first := httptest.NewServer(redirectsTo{other.URL, status})
				t.Cleanup(first.Close)
				acct := ai.Account{Provider: kind, BaseURL: first.URL, ModelName: "m"}
				_, out := ai.Call(ctx, http.DefaultClient, acct, syntheticKey, ai.Request{Prompt: "x", MaxTokens: 1})
				if out != ai.ProbeRedirected || catcher.hits != 0 {
					t.Fatalf("outcome %v, the other host was reached %d times with %q", out, catcher.hits, catcher.heard)
				}
			})
		}
	}
}

func TestAnOpenAICompatibleCallStillFollowsARedirect(t *testing.T) {
	other, catcher := otherHost(t, openAIAnswer)
	first := httptest.NewServer(redirectsTo{other.URL, http.StatusTemporaryRedirect})
	t.Cleanup(first.Close)
	acct := ai.Account{Provider: "openai_compatible", BaseURL: first.URL, ModelName: "m"}
	reply, out := ai.Call(ctx, http.DefaultClient, acct, syntheticKey, ai.Request{Prompt: "x", MaxTokens: 1})
	if out != ai.ProbeOK || catcher.hits != 1 || reply.Text != "hi" {
		t.Fatalf("outcome %v, hits %d, reply %+v", out, catcher.hits, reply)
	}
}
