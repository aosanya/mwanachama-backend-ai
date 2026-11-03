package ai_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

type cancelAfterBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c cancelAfterBody) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	if err == io.EOF {
		c.cancel()
	}
	return n, err
}

type cancelsAfterAnswer struct{ cancel context.CancelFunc }

func (c cancelsAfterAnswer) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err == nil {
		resp.Body = cancelAfterBody{resp.Body, c.cancel}
	}
	return resp, err
}

func usedCounts(t *testing.T, m ai.Manager, agencyID, accountID string) map[string]int {
	t.Helper()
	report, err := m.ListUsage(ctx, agencyID, accountID)
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]int{}
	for _, c := range report.Counts {
		used[c.Limit] = c.Used
	}
	return used
}

func requireNoLeakedHold(t *testing.T, m ai.Manager, agencyID, accountID, roleID string) {
	t.Helper()
	_, first, err := m.HoldRequest(ctx, agencyID, ai.CheckInput{AccountID: accountID, RoleID: roleID})
	if err != nil || first == nil {
		t.Fatalf("a hold was left behind: %v", err)
	}
	first.Release()
}

func TestAGatewayCallCancelledAfterTheModelAnsweredStillRecordsItsUse(t *testing.T) {
	fm := &fakeModel{reply: modelReply}
	_, m, r, acc := gatewaySetup(t, fm, 0)
	if _, err := m.SetAccountLimits(ctx, "ag", acc.ID, ai.Limits{RequestsPerMinute: ip(2)}); err != nil {
		t.Fatal(err)
	}
	callerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	client := &http.Client{Transport: cancelsAfterAnswer{cancel}}
	h := ai.NewGateway(m, func(*http.Request) (string, string, bool) { return "ag", "ar-1", true }, client)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"messages":[]}`)).WithContext(callerCtx)
	req.Header.Set("X-Usage-Kind", "chat")

	h.ServeHTTP(httptest.NewRecorder(), req)

	if callerCtx.Err() == nil {
		t.Fatal("the caller was never cancelled")
	}
	used := usedCounts(t, m, "ag", acc.ID)
	if used["requests per minute"] != 1 || used["daily token"] != 12 {
		t.Fatalf("usage %v, want 1 request and 12 tokens", used)
	}
	requireNoLeakedHold(t, m, "ag", acc.ID, r.ID)
}

func TestAChatCallCancelledAfterTheModelAnsweredStillRecordsItsUse(t *testing.T) {
	srv, _ := fakeProvider(t, 200, textAnswer)
	callerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	client := &http.Client{Transport: cancelsAfterAnswer{cancel}}
	m, _, a := chatManager(t, "openai_compatible", srv.URL, nil, ai.WithChatClient(client))
	if _, err := m.SetAccountLimits(ctx, "ag-1", a.ID, ai.Limits{RequestsPerMinute: ip(2)}); err != nil {
		t.Fatal(err)
	}

	if _, err := m.ChatAccount(callerCtx, "ag-1", a.ID, oneQuestion()); err != nil {
		t.Fatal(err)
	}

	if callerCtx.Err() == nil {
		t.Fatal("the caller was never cancelled")
	}
	used := usedCounts(t, m, "ag-1", a.ID)
	if used["requests per minute"] != 1 || used["daily token"] != 18 {
		t.Fatalf("usage %v, want 1 request and 18 tokens", used)
	}
	requireNoLeakedHold(t, m, "ag-1", a.ID, "")
}
