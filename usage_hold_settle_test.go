package ai_test

import (
	"sync"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func requestsPerDay(t *testing.T, m ai.Manager, accountID string) int {
	t.Helper()
	report, err := m.ListUsage(ctx, "ag", accountID)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range report.Counts {
		if c.Field == "requests_per_day" {
			return c.Used
		}
	}
	t.Fatal("no daily request count")
	return 0
}

func TestSettlesMadeAtTheSameMomentOnOneHoldRecordItOnce(t *testing.T) {
	const trials, settlers = 25, 4
	for trial := 0; trial < trials; trial++ {
		m, _, _ := newTestManager(t)
		acc, err := m.CreateAccount(ctx, "ag", validAccount("held"))
		if err != nil {
			t.Fatal(err)
		}
		_, hold, err := m.HoldRequest(ctx, "ag", ai.CheckInput{AccountID: acc.ID, RoleID: "r1"})
		if err != nil || hold == nil {
			t.Fatalf("hold: %v", err)
		}
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := 0; i < settlers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, _ = hold.Settle(ctx, ai.UsageInput{TokensIn: 3, TokensOut: 4})
			}()
		}
		close(start)
		wg.Wait()
		if got := requestsPerDay(t, m, acc.ID); got != 1 {
			t.Fatalf("trial %d: %d settles on one hold recorded %d requests, want 1", trial, settlers, got)
		}
	}
}

func TestASettledHoldDropsItsCountExactlyOnceWhenSettlesOverlap(t *testing.T) {
	m, _, _ := newTestManager(t)
	acc, err := m.CreateAccount(ctx, "ag", validAccount("held"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetAccountLimits(ctx, "ag", acc.ID, ai.Limits{RequestsPerMinute: ip(2)}); err != nil {
		t.Fatal(err)
	}
	in := ai.CheckInput{AccountID: acc.ID, RoleID: "r1"}
	_, first, _ := m.HoldRequest(ctx, "ag", in)
	_, second, _ := m.HoldRequest(ctx, "ag", in)
	if first == nil || second == nil {
		t.Fatal("two holds should fit a limit of two")
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _ = first.Settle(ctx, ai.UsageInput{TokensIn: 1, TokensOut: 1})
		}()
	}
	close(start)
	wg.Wait()
	second.Release()
	if d, third, _ := m.HoldRequest(ctx, "ag", in); third == nil || !d.Allowed {
		t.Fatalf("one recorded request against a limit of two left no room: %+v", d)
	}
}
