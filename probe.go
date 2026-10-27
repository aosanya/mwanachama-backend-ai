package ai

import (
	"context"
	"net/http"
	"time"
)

const (
	TestWorking    = "working"
	TestNoKey      = "no key is set for this account"
	TestRefused    = "the model refused the key"
	TestUnreached  = "the model could not be reached"
	TestModelError = "the model answered with an error"
	probePrompt    = "Reply with the single word OK."
)

type ProbeOutcome int

const (
	ProbeOK ProbeOutcome = iota
	ProbeRefused
	ProbeUnreached
	ProbeError
)

type Prober interface {
	Probe(ctx context.Context, a Account, key string) ProbeOutcome
}

type httpProber struct{ client *http.Client }

func NewHTTPProber() Prober { return httpProber{client: &http.Client{Timeout: 20 * time.Second}} }

func (p httpProber) Probe(ctx context.Context, a Account, key string) ProbeOutcome {
	_, out := Call(ctx, p.client, a, key, Request{Prompt: probePrompt, MaxTokens: 8})
	return out
}

func WithProber(p Prober) Option {
	return func(m *accountManager) { m.prober = p }
}

func (m *accountManager) TestAccount(ctx context.Context, agencyID, accountID string) (Account, error) {
	held, err := m.GetAccount(ctx, agencyID, accountID)
	if err != nil {
		return Account{}, err
	}
	result := m.probeResult(ctx, held)
	held.LastTestResult = result
	held.LastTestedAt = m.now()
	if err := m.update(ctx, roleAccount, held.ID, held); err != nil {
		return Account{}, err
	}
	return held, nil
}

func (m *accountManager) probeResult(ctx context.Context, a Account) string {
	key, err := m.AccountKey(ctx, a.AgencyID, a.ID)
	if err != nil || key == "" {
		return TestNoKey
	}
	prober := m.prober
	if prober == nil {
		prober = NewHTTPProber()
	}
	switch prober.Probe(ctx, a, key) {
	case ProbeOK:
		return TestWorking
	case ProbeRefused:
		return TestRefused
	case ProbeUnreached:
		return TestUnreached
	}
	return TestModelError
}
