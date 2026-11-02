package ai_test

import (
	"errors"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func chatTogether(t *testing.T, m ai.Manager, accountID string, model *heldModel, n int) (answered, refused int) {
	t.Helper()
	var wg sync.WaitGroup
	var stopped, succeeded atomic.Int32
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.ChatAccount(ctx, "ag-1", accountID, oneQuestion())
			switch {
			case err == nil:
				succeeded.Add(1)
			case errors.Is(err, ai.ErrLimitReached):
				stopped.Add(1)
			default:
				t.Errorf("chat: %v", err)
			}
		}()
	}
	deadline := time.Now().Add(5 * time.Second)
	for int(model.calls.Load()+stopped.Load()) < n && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	close(model.gate)
	wg.Wait()
	return int(succeeded.Load()), int(stopped.Load())
}

func heldChat(t *testing.T, perMinute int) (ai.Manager, ai.Account, *heldModel) {
	t.Helper()
	model := &heldModel{gate: make(chan struct{}), reply: textAnswer}
	srv := httptest.NewServer(model)
	t.Cleanup(srv.Close)
	m, _, a := chatManager(t, "openai_compatible", srv.URL, nil)
	if _, err := m.SetAccountLimits(ctx, "ag-1", a.ID, ai.Limits{RequestsPerMinute: ip(perMinute)}); err != nil {
		t.Fatal(err)
	}
	return m, a, model
}

func TestSixChatCallsAtOnceAgainstOneRequestPerMinuteReachTheModelOnce(t *testing.T) {
	m, a, model := heldChat(t, 1)

	answered, refused := chatTogether(t, m, a.ID, model, 6)

	if got := model.calls.Load(); got != 1 {
		t.Fatalf("the model was reached %d times, want 1 (answered %d, refused %d)", got, answered, refused)
	}
	if answered != 1 || refused != 5 {
		t.Fatalf("answered %d, refused %d, want 1 and 5", answered, refused)
	}
}

func TestThreeChatCallsAtOnceAgainstThreeRequestsPerMinuteAllReachTheModel(t *testing.T) {
	m, a, model := heldChat(t, 3)

	answered, refused := chatTogether(t, m, a.ID, model, 3)

	if model.calls.Load() != 3 || answered != 3 || refused != 0 {
		t.Fatalf("calls %d, answered %d, refused %d", model.calls.Load(), answered, refused)
	}
}

func TestAFailedChatCallReleasesItsHoldSoTheNextReachesTheModel(t *testing.T) {
	srv, seen := fakeProvider(t, 500, "down")
	m, _, a := chatManager(t, "openai_compatible", srv.URL, nil)
	if _, err := m.SetAccountLimits(ctx, "ag-1", a.ID, ai.Limits{RequestsPerMinute: ip(1)}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		seen.path = ""
		_, err := m.ChatAccount(ctx, "ag-1", a.ID, oneQuestion())
		if errors.Is(err, ai.ErrLimitReached) || seen.path == "" {
			t.Fatalf("call %d did not reach the model: %v", i, err)
		}
	}
}

func TestAChatHoldCountsAgainstAGatewayCallToTheSameAccount(t *testing.T) {
	model := &heldModel{gate: make(chan struct{}), reply: modelReply}
	h, m, _, acc := gatewaySetupFor(t, model, 0)
	if _, err := m.SetAccountLimits(ctx, "ag", acc.ID, ai.Limits{RequestsPerMinute: ip(1)}); err != nil {
		t.Fatal(err)
	}
	chatting := make(chan error, 1)
	go func() {
		_, err := m.ChatAccount(ctx, "ag", acc.ID, oneQuestion())
		chatting <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for model.calls.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	besides := make(chan int, 1)
	go func() { besides <- post(h, `{"messages":[]}`).Code }()

	var code int
	select {
	case code = <-besides:
	case <-time.After(2 * time.Second):
		code = -1
	}
	close(model.gate)
	<-chatting
	if code != 429 {
		t.Fatalf("a gateway call beside a chat call in flight answered %d, want 429", code)
	}
}
