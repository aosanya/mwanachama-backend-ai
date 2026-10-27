package ai_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

type fakeModel struct {
	mu     sync.Mutex
	calls  int
	auth   string
	body   map[string]any
	status int
	reply  string
}

func (f *fakeModel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.auth = r.Header.Get("Authorization")
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &f.body)
	if f.status != 0 {
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte("upstream failed for " + f.auth))
		return
	}
	_, _ = w.Write([]byte(f.reply))
}

const modelReply = `{"choices":[{"message":{"content":"hi"}}],"usage":{"prompt_tokens":5,"completion_tokens":7}}`

func gatewaySetup(t *testing.T, fm *fakeModel, roleLimit int) (http.Handler, ai.Manager, ai.AIRole, ai.Account) {
	t.Helper()
	srv := httptest.NewServer(fm)
	t.Cleanup(srv.Close)
	m, _, _ := newTestManager(t, ai.WithMasterKey(master))
	acc, err := m.CreateAccount(ctx, "ag", ai.Account{Name: "a", Provider: "openai_compatible", BaseURL: srv.URL, ModelName: "synthetic", Status: "enabled"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetAccountKey(ctx, "ag", acc.ID, ai.KeyInput{Key: syntheticKey}); err != nil {
		t.Fatal(err)
	}
	in := roleIn("scribe")
	in.TokensPerCall = ip(roleLimit)
	r := mustRole(t, m, "ag", in)
	r, err = m.SetAIRoleMapping(ctx, "ag", r.ID, ai.AIRoleMappingInput{AccountID: acc.ID, AgencyRoleID: "ar-1"})
	if err != nil {
		t.Fatal(err)
	}
	h := ai.NewGateway(m, func(*http.Request) (string, string, bool) { return "ag", "ar-1", true }, nil)
	return h, m, r, acc
}

func post(h http.Handler, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))
	return rec
}

func TestGatewayKeyReachesTheModelAndNeverTheCaller(t *testing.T) {
	fm := &fakeModel{reply: modelReply}
	h, _, _, _ := gatewaySetup(t, fm, 0)
	rec := post(h, `{"messages":[{"role":"user","content":"x"}]}`)
	if rec.Code != 200 || rec.Body.String() != modelReply {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if fm.auth != "Bearer "+syntheticKey {
		t.Fatalf("model got %q", fm.auth)
	}
	if strings.Contains(rec.Body.String(), syntheticKey) || strings.Contains(rec.Header().Get("Authorization"), syntheticKey) {
		t.Fatal("key reached the caller")
	}
}

func TestGatewayRefusesAtALimitBeforeAnythingLeaves(t *testing.T) {
	fm := &fakeModel{reply: modelReply}
	h, m, r, acc := gatewaySetup(t, fm, 0)
	if _, err := m.SetAccountLimits(ctx, "ag", acc.ID, ai.Limits{RequestsPerDay: ip(1)}); err != nil {
		t.Fatal(err)
	}
	if rec := post(h, `{"messages":[]}`); rec.Code != 200 {
		t.Fatalf("first: %d", rec.Code)
	}
	rec := post(h, `{"messages":[]}`)
	if rec.Code != 429 || !strings.Contains(rec.Body.String(), "this account's daily request limit is reached") {
		t.Fatalf("second: %d %s", rec.Code, rec.Body)
	}
	if fm.calls != 1 {
		t.Fatalf("the model was called %d times", fm.calls)
	}
	_ = r
}

func TestGatewayClampsMaxTokens(t *testing.T) {
	fm := &fakeModel{reply: modelReply}
	h, _, _, _ := gatewaySetup(t, fm, 50)
	post(h, `{"messages":[],"max_tokens":9000}`)
	if fm.body["max_tokens"].(float64) != 50 {
		t.Fatalf("max_tokens = %v", fm.body["max_tokens"])
	}
	post(h, `{"messages":[]}`)
	if fm.body["max_tokens"].(float64) != 50 {
		t.Fatalf("absent max_tokens = %v", fm.body["max_tokens"])
	}
	post(h, `{"messages":[],"max_tokens":10}`)
	if fm.body["max_tokens"].(float64) != 10 {
		t.Fatalf("lower ask kept = %v", fm.body["max_tokens"])
	}
}

func TestGatewayRecordsUseOncePerCall(t *testing.T) {
	fm := &fakeModel{reply: modelReply}
	h, m, r, acc := gatewaySetup(t, fm, 0)
	post(h, `{"messages":[]}`)
	rep, _ := m.ListUsage(ctx, "ag", acc.ID)
	for _, c := range rep.Counts {
		if c.Limit == "requests per day" && c.Used != 1 {
			t.Fatalf("account requests used = %d", c.Used)
		}
	}
	got, _ := m.ResolveAIRole(ctx, "ag", r.ID)
	for _, l := range got.Limits {
		if l.Limit == "requests per day" && l.Used != 1 {
			t.Fatalf("requests used = %d", l.Used)
		}
		if l.Limit == "daily token" && l.Used != 12 {
			t.Fatalf("tokens used = %d", l.Used)
		}
	}
}

func TestGatewayDoesNotEchoAnUpstreamError(t *testing.T) {
	fm := &fakeModel{status: 500}
	h, _, _, _ := gatewaySetup(t, fm, 0)
	rec := post(h, `{"messages":[]}`)
	if rec.Code != 502 || strings.Contains(rec.Body.String(), syntheticKey) || strings.Contains(rec.Body.String(), "upstream failed") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
