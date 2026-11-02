package ai_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func TestAStalledModelIsLetGoAfterTheGatewaysOwnTimeoutAndItsHoldIsDropped(t *testing.T) {
	for _, kind := range []string{"openai_compatible", "anthropic"} {
		t.Run(kind, func(t *testing.T) {
			stalled := make(chan struct{})
			model := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				select {
				case <-stalled:
				case <-r.Context().Done():
				}
			})
			h, m, r, acc := gatewaySetupWithClient(t, kind, model, 0, &http.Client{Timeout: 150 * time.Millisecond})
			t.Cleanup(func() { close(stalled) })
			if _, err := m.SetAccountLimits(ctx, "ag", acc.ID, ai.Limits{RequestsPerMinute: ip(1)}); err != nil {
				t.Fatal(err)
			}

			answered := make(chan *httptest.ResponseRecorder, 1)
			go func() { answered <- post(h, `{"messages":[{"role":"user","content":"x"}]}`) }()

			select {
			case rec := <-answered:
				if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "the model could not be reached") {
					t.Fatalf("%d %s", rec.Code, rec.Body)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("a model that never answers still holds the call after the gateway's timeout")
			}

			d, hold, err := m.HoldRequest(ctx, "ag", ai.CheckInput{AccountID: acc.ID, RoleID: r.ID})
			if err != nil || hold == nil {
				t.Fatalf("the stalled call left its hold behind: %+v %v", d, err)
			}
			hold.Release()
		})
	}
}
