package ai_test

import (
	"sync"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func TestTwoRecordsAtTheSameInstantBothCountAndNeitherFails(t *testing.T) {
	m, _, _ := newTestManager(t)
	a, err := m.CreateAccount(ctx, "ag-1", validAccount("w"))
	if err != nil {
		t.Fatal(err)
	}
	const calls = 8
	var wg sync.WaitGroup
	errs := make(chan error, calls)
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.RecordUsage(ctx, "ag-1", ai.UsageInput{AccountID: a.ID, TokensIn: 10, TokensOut: 5})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("a record failed: %v", err)
		}
	}
	u, err := m.RecordUsage(ctx, "ag-1", ai.UsageInput{AccountID: a.ID, TokensIn: 10, TokensOut: 5})
	if err != nil {
		t.Fatal(err)
	}
	if u.Requests != calls+1 || u.TokensIn != 10*(calls+1) {
		t.Errorf("day row = %d requests and %d tokens in, want %d and %d", u.Requests, u.TokensIn, calls+1, 10*(calls+1))
	}
}
