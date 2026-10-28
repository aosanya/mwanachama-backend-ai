package mcp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const typedKey = "sk-TYPED-mcp-not-saved-2e1d0c9b"

type recordingProvider struct {
	*httptest.Server
	mu      sync.Mutex
	bearers []string
}

func newRecordingProvider(t *testing.T) *recordingProvider {
	t.Helper()
	p := &recordingProvider{}
	p.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		p.mu.Lock()
		p.bearers = append(p.bearers, got)
		p.mu.Unlock()
		if got == typedKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"OK"}}]}`))
	}))
	t.Cleanup(p.Close)
	return p
}

func (p *recordingProvider) lastBearer() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.bearers[len(p.bearers)-1]
}

func idOf(t *testing.T, out string) string {
	t.Helper()
	var parsed struct {
		StructuredContent struct {
			ID string `json:"id"`
		} `json:"structuredContent"`
	}
	_ = json.Unmarshal([]byte(out), &parsed)
	if parsed.StructuredContent.ID == "" {
		t.Fatalf("no id: %s", out)
	}
	return parsed.StructuredContent.ID
}

func TestTheTestToolAsksTheModelWithATypedKeyAndKeepsTheSavedResult(t *testing.T) {
	provider := newRecordingProvider(t)
	cs := connect(t)
	created, isErr := call(t, cs, "ai_create_account", accountArgs(map[string]any{"base_url": provider.URL}))
	if isErr {
		t.Fatalf("create: %s", created)
	}
	id := idOf(t, created)
	call(t, cs, "ai_set_account_key", map[string]any{"agencyID": "ag-1", "accountID": id, "key": syntheticKey})

	saved, isErr := call(t, cs, "ai_test_account", map[string]any{"agencyID": "ag-1", "accountID": id})
	if isErr || !strings.Contains(saved, `"last_test_result":"working"`) || provider.lastBearer() != syntheticKey {
		t.Fatalf("saved key test: %s, provider saw %q", saved, provider.lastBearer())
	}

	typed, isErr := call(t, cs, "ai_test_account", map[string]any{"agencyID": "ag-1", "accountID": id, "key": typedKey})
	if isErr || !strings.Contains(typed, `"last_test_result":"the model refused the key"`) || provider.lastBearer() != typedKey {
		t.Fatalf("typed key test: %s, provider saw %q", typed, provider.lastBearer())
	}

	read, _ := call(t, cs, "ai_get_account", map[string]any{"agencyID": "ag-1", "accountID": id})
	if !strings.Contains(read, `"last_test_result":"working"`) {
		t.Fatalf("the typed key's result was stored: %s", read)
	}
	for _, out := range []string{saved, typed, read} {
		if strings.Contains(out, syntheticKey) || strings.Contains(out, typedKey) {
			t.Fatalf("key leaked: %s", out)
		}
	}
}

func TestTheTestToolRefusesABlankTypedKey(t *testing.T) {
	cs := connect(t)
	created, _ := call(t, cs, "ai_create_account", accountArgs(nil))
	id := idOf(t, created)
	if out, isErr := call(t, cs, "ai_test_account", map[string]any{"agencyID": "ag-1", "accountID": id, "key": "   "}); !isErr {
		t.Fatalf("a blank key was tested: %s", out)
	}
}
