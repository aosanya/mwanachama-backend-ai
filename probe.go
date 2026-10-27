package ai

import (
	"bytes"
	"context"
	"encoding/json"
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

type Prober interface {
	Probe(ctx context.Context, a Account, key string) ProbeOutcome
}

type httpProber struct{ client *http.Client }

func NewHTTPProber() Prober { return httpProber{client: &http.Client{Timeout: 20 * time.Second}} }

func (p httpProber) Probe(ctx context.Context, a Account, key string) ProbeOutcome {
	body, _ := json.Marshal(map[string]any{
		"model":      a.ModelName,
		"max_tokens": 8,
		"messages":   []map[string]string{{"role": "user", "content": probePrompt}},
	})
	url := strings.TrimRight(a.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return ProbeUnreached
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := p.client.Do(req)
	if err != nil {
		return ProbeUnreached
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return ProbeRefused
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return ProbeOK
	}
	return ProbeError
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
