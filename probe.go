package ai

import (
	"context"
	"net/http"
	"strings"
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

type ProbeResult struct {
	Outcome ProbeOutcome
	Reply   string
	Elapsed time.Duration
}

type Prober interface {
	Probe(ctx context.Context, a Account, key string) ProbeResult
}

type httpProber struct{ client *http.Client }

func NewHTTPProber() Prober { return httpProber{client: &http.Client{Timeout: 20 * time.Second}} }

func (p httpProber) Probe(ctx context.Context, a Account, key string) ProbeResult {
	started := time.Now()
	reply, out := Call(ctx, p.client, a, key, Request{Prompt: probePrompt, MaxTokens: 8})
	return ProbeResult{Outcome: out, Reply: reply.Text, Elapsed: time.Since(started)}
}

func WithProber(p Prober) Option {
	return func(m *accountManager) { m.prober = p }
}

func (m *accountManager) TestAccount(ctx context.Context, agencyID, accountID string, in KeyInput) (Account, error) {
	held, err := m.GetAccount(ctx, agencyID, accountID)
	if err != nil {
		return Account{}, err
	}
	if in.Key != "" {
		return m.testTypedKey(ctx, held, in.Key)
	}
	held = m.withTest(held, m.probeStoredKey(ctx, held))
	if err := m.update(ctx, roleAccount, held.ID, held); err != nil {
		return Account{}, err
	}
	return held, nil
}

func (m *accountManager) testTypedKey(ctx context.Context, held Account, key string) (Account, error) {
	if strings.TrimSpace(key) == "" || len(key) > maxKeyBytes {
		return Account{}, ErrInvalidKey
	}
	return m.withTest(held, m.probeWith(ctx, held, key)), nil
}

func (m *accountManager) withTest(a Account, rec testRecord) Account {
	a.LastTestResult, a.LastTestPrompt, a.LastTestReply, a.LastTestMs = rec.result, rec.prompt, rec.reply, rec.ms
	a.LastTestedAt = m.now()
	return a
}

type testRecord struct {
	result string
	prompt string
	reply  string
	ms     int
}

func (m *accountManager) probeStoredKey(ctx context.Context, a Account) testRecord {
	key, err := m.AccountKey(ctx, a.AgencyID, a.ID)
	if err != nil || key == "" {
		return testRecord{result: TestNoKey}
	}
	return m.probeWith(ctx, a, key)
}

func (m *accountManager) probeWith(ctx context.Context, a Account, key string) testRecord {
	prober := m.prober
	if prober == nil {
		prober = NewHTTPProber()
	}
	got := prober.Probe(ctx, a, key)
	rec := testRecord{result: resultOf(got.Outcome), prompt: probePrompt, ms: wholeMilliseconds(got.Elapsed)}
	if got.Outcome == ProbeOK {
		rec.reply = shownReply(got.Reply, key)
	}
	return rec
}

func resultOf(out ProbeOutcome) string {
	switch out {
	case ProbeOK:
		return TestWorking
	case ProbeRefused:
		return TestRefused
	case ProbeUnreached:
		return TestUnreached
	}
	return TestModelError
}
