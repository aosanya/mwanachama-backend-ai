package ai_test

import (
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

type heldModel struct {
	calls atomic.Int32
	gate  chan struct{}
	reply string
}

func (h *heldModel) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.calls.Add(1)
	<-h.gate
	_, _ = w.Write([]byte(h.reply))
}

func callsTogether(t *testing.T, h http.Handler, model *heldModel, n int) map[int]int {
	t.Helper()
	var refused atomic.Int32
	var wg sync.WaitGroup
	var mu sync.Mutex
	codes := map[int]int{}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code := post(h, `{"messages":[]}`).Code
			if code == http.StatusTooManyRequests {
				refused.Add(1)
			}
			mu.Lock()
			codes[code]++
			mu.Unlock()
		}()
	}
	deadline := time.Now().Add(5 * time.Second)
	for int(model.calls.Load()+refused.Load()) < n && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	close(model.gate)
	wg.Wait()
	return codes
}

func TestSixCallsAtOnceAgainstOneRequestPerMinuteReachTheModelOnce(t *testing.T) {
	limits := map[string]func(*testing.T, ai.Manager, ai.AIRole, ai.Account){
		"role": func(t *testing.T, m ai.Manager, r ai.AIRole, _ ai.Account) {
			if _, err := m.SetAIRoleLimits(ctx, "ag", r.ID, "ann", ai.Limits{RequestsPerMinute: ip(1)}); err != nil {
				t.Fatal(err)
			}
		},
		"account": func(t *testing.T, m ai.Manager, _ ai.AIRole, a ai.Account) {
			if _, err := m.SetAccountLimits(ctx, "ag", a.ID, ai.Limits{RequestsPerMinute: ip(1)}); err != nil {
				t.Fatal(err)
			}
		},
	}
	for owner, set := range limits {
		t.Run(owner, func(t *testing.T) {
			model := &heldModel{gate: make(chan struct{}), reply: modelReply}
			h, m, r, acc := gatewaySetupFor(t, model, 0)
			set(t, m, r, acc)
			codes := callsTogether(t, h, model, 6)
			if got := model.calls.Load(); got != 1 {
				t.Fatalf("the model was reached %d times, want 1 (codes %v)", got, codes)
			}
			if codes[http.StatusOK] != 1 || codes[http.StatusTooManyRequests] != 5 {
				t.Fatalf("codes = %v, want one 200 and five 429", codes)
			}
		})
	}
}

func TestThreeCallsAtOnceAgainstThreeRequestsPerMinuteAllReachTheModel(t *testing.T) {
	model := &heldModel{gate: make(chan struct{}), reply: modelReply}
	h, m, r, _ := gatewaySetupFor(t, model, 0)
	if _, err := m.SetAIRoleLimits(ctx, "ag", r.ID, "ann", ai.Limits{RequestsPerMinute: ip(3)}); err != nil {
		t.Fatal(err)
	}
	codes := callsTogether(t, h, model, 3)
	if model.calls.Load() != 3 || codes[http.StatusOK] != 3 {
		t.Fatalf("calls %d, codes %v", model.calls.Load(), codes)
	}
}

func TestAFailedCallReleasesItsHoldSoTheNextReachesTheModel(t *testing.T) {
	fm := &fakeModel{status: 500}
	h, m, r, _ := gatewaySetup(t, fm, 0)
	if _, err := m.SetAIRoleLimits(ctx, "ag", r.ID, "ann", ai.Limits{RequestsPerMinute: ip(1)}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if rec := post(h, `{"messages":[]}`); rec.Code != http.StatusBadGateway {
			t.Fatalf("call %d: %d %s", i, rec.Code, rec.Body)
		}
	}
	if fm.calls != 3 {
		t.Fatalf("the model was reached %d times, want 3", fm.calls)
	}
}

func TestAHoldIsReplacedByItsRecordedUseNotAddedToIt(t *testing.T) {
	m, _, _ := newTestManager(t)
	acc, err := m.CreateAccount(ctx, "ag", validAccount("held"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetAccountLimits(ctx, "ag", acc.ID, ai.Limits{RequestsPerMinute: ip(2)}); err != nil {
		t.Fatal(err)
	}
	in := ai.CheckInput{AccountID: acc.ID, RoleID: "r1"}
	_, first, err := m.HoldRequest(ctx, "ag", in)
	if err != nil || first == nil {
		t.Fatalf("first hold: %v", err)
	}
	if _, err := first.Settle(ctx, ai.UsageInput{TokensIn: 3, TokensOut: 4}); err != nil {
		t.Fatal(err)
	}
	d, second, err := m.HoldRequest(ctx, "ag", in)
	if err != nil || second == nil {
		t.Fatalf("second hold: %v %+v", err, d)
	}
	if d, third, _ := m.HoldRequest(ctx, "ag", in); third != nil || d.Allowed {
		t.Fatalf("third hold passed with one recorded and one held against a limit of two: %+v", d)
	}
	second.Release()
	if d, fourth, _ := m.HoldRequest(ctx, "ag", in); fourth == nil || !d.Allowed {
		t.Fatalf("a released hold still counted: %+v", d)
	}
}
