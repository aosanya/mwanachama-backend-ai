package ai_test

import (
	"net/http"
	"testing"
)

func sentNumber(body map[string]any, key string) (float64, bool) {
	n, ok := body[key].(float64)
	return n, ok
}

func TestAnOpenAICompatibleBodyKeepsTheSmallerOfItsOwnCapAndTheLimit(t *testing.T) {
	cases := []struct {
		name          string
		body          string
		maxTokens     float64
		hasMaxTokens  bool
		maxCompletion float64
		hasCompletion bool
	}{
		{"a smaller max_completion_tokens", `{"messages":[],"max_completion_tokens":10}`, 0, false, 10, true},
		{"a larger max_completion_tokens", `{"messages":[],"max_completion_tokens":5000}`, 0, false, 1000, true},
		{"a smaller max_tokens", `{"messages":[],"max_tokens":10}`, 10, true, 0, false},
		{"a larger max_tokens", `{"messages":[],"max_tokens":9000}`, 1000, true, 0, false},
		{"neither", `{"messages":[]}`, 1000, true, 0, false},
		{"both, the first smaller", `{"messages":[],"max_tokens":10,"max_completion_tokens":5000}`, 10, true, 1000, true},
		{"both, the second smaller", `{"messages":[],"max_tokens":5000,"max_completion_tokens":10}`, 1000, true, 10, true},
		{"a max_tokens of zero beside a real max_completion_tokens", `{"messages":[],"max_tokens":0,"max_completion_tokens":10}`, 0, false, 10, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fm := &fakeModel{reply: modelReply}
			h, _, _, _ := gatewaySetup(t, fm, 1000)
			if rec := post(h, c.body); rec.Code != http.StatusOK {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			gotMax, hasMax := sentNumber(fm.body, "max_tokens")
			gotCompletion, hasCompletion := sentNumber(fm.body, "max_completion_tokens")
			if hasMax != c.hasMaxTokens || gotMax != c.maxTokens || hasCompletion != c.hasCompletion || gotCompletion != c.maxCompletion {
				t.Fatalf("sent max_tokens %v (%v) and max_completion_tokens %v (%v), want %v (%v) and %v (%v)",
					gotMax, hasMax, gotCompletion, hasCompletion, c.maxTokens, c.hasMaxTokens, c.maxCompletion, c.hasCompletion)
			}
		})
	}
}

func TestAClaudeBodyKeepsTheSmallerOfItsOwnCapAndTheLimit(t *testing.T) {
	cases := []struct {
		name  string
		limit int
		body  string
		want  float64
	}{
		{"a smaller max_completion_tokens", 1000, `{"messages":[{"role":"user","content":"x"}],"max_completion_tokens":10}`, 10},
		{"a larger max_completion_tokens", 1000, `{"messages":[{"role":"user","content":"x"}],"max_completion_tokens":5000}`, 1000},
		{"a smaller max_tokens", 1000, `{"messages":[{"role":"user","content":"x"}],"max_tokens":10}`, 10},
		{"a larger max_tokens", 1000, `{"messages":[{"role":"user","content":"x"}],"max_tokens":9000}`, 1000},
		{"both, the first smaller", 1000, `{"messages":[{"role":"user","content":"x"}],"max_tokens":10,"max_completion_tokens":500}`, 10},
		{"both, the second smaller", 1000, `{"messages":[{"role":"user","content":"x"}],"max_tokens":500,"max_completion_tokens":10}`, 10},
		{"both above the limit", 1000, `{"messages":[{"role":"user","content":"x"}],"max_tokens":9000,"max_completion_tokens":5000}`, 1000},
		{"a max_tokens of zero beside a real max_completion_tokens", 1000, `{"messages":[{"role":"user","content":"x"}],"max_tokens":0,"max_completion_tokens":10}`, 10},
		{"no limit and a max_completion_tokens", 0, `{"messages":[{"role":"user","content":"x"}],"max_completion_tokens":10}`, 10},
		{"no limit and both", 0, `{"messages":[{"role":"user","content":"x"}],"max_tokens":300,"max_completion_tokens":20}`, 20},
		{"a limit and neither", 50, `{"messages":[{"role":"user","content":"x"}]}`, 50},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fc := &fakeClaude{reply: claudeReply}
			h, _, _, _ := gatewaySetupKind(t, "anthropic", fc, c.limit)
			if rec := post(h, c.body); rec.Code != http.StatusOK {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			if got, _ := sentNumber(fc.body, "max_tokens"); got != c.want {
				t.Fatalf("Claude was sent max_tokens %v, want %v", got, c.want)
			}
		})
	}
}
