package ai

import (
	"context"
	"io"
	"net/http"
)

func Call(ctx context.Context, client *http.Client, a Account, key string, r Request) (Reply, ProbeOutcome) {
	ad, ok := AdapterFor(a.Provider)
	if !ok {
		return Reply{}, ProbeError
	}
	req, err := ad.BuildRequest(ctx, a, key, r)
	if err != nil {
		return Reply{}, ProbeUnreached
	}
	body, outcome := send(client, req)
	if outcome != ProbeOK {
		return Reply{}, outcome
	}
	reply, err := ad.ParseReply(body)
	if err != nil {
		return Reply{}, ProbeError
	}
	return reply, ProbeOK
}

var keyHeaders = []string{"x-api-key", "x-goog-api-key"}

func carriesKeyHeader(req *http.Request) bool {
	for _, name := range keyHeaders {
		if req.Header.Get(name) != "" {
			return true
		}
	}
	return false
}

func withoutRedirects(client *http.Client) *http.Client {
	guarded := *client
	guarded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &guarded
}

func send(client *http.Client, req *http.Request) ([]byte, ProbeOutcome) {
	guarded := carriesKeyHeader(req)
	if guarded {
		client = withoutRedirects(client)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, ProbeUnreached
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, ProbeRefused
	case guarded && resp.StatusCode >= 300 && resp.StatusCode < 400:
		return nil, ProbeRedirected
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, ProbeError
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReplyBytes))
	if err != nil {
		return nil, ProbeUnreached
	}
	return body, ProbeOK
}
