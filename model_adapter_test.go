package ai_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
)

func TestModelListsAreAskedFromEachKindInItsOwnWay(t *testing.T) {
	cases := []struct {
		kind, answer, wantURL, keyHeader, keyValue string
		want                                       []string
	}{
		{"openai_compatible", `{"data":[{"id":"o-1"},{"id":"o-2"}]}`, "/models", "Authorization", "Bearer " + syntheticKey, []string{"o-1", "o-2"}},
		{"anthropic", `{"data":[{"id":"c-1"}],"has_more":false}`, "/v1/models?limit=1000", "x-api-key", syntheticKey, []string{"c-1"}},
		{"gemini", `{"models":[{"name":"models/g-1","supportedGenerationMethods":["generateContent"]},{"name":"models/embed","supportedGenerationMethods":["embedContent"]},{"name":"models/g-2"}]}`, "/v1beta/models?pageSize=1000", "x-goog-api-key", syntheticKey, []string{"g-1", "g-2"}},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			srv, seen := fakeProvider(t, 200, c.answer)
			var method string
			wrapped := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method = r.Method
				srv.Config.Handler.ServeHTTP(w, r)
			}))
			t.Cleanup(wrapped.Close)
			got := ai.NewHTTPModelLister().FetchModels(ctx, ai.Account{Provider: c.kind, BaseURL: wrapped.URL + "/"}, syntheticKey)
			if got.Outcome != ai.ProbeOK || strings.Join(got.Names, ",") != strings.Join(c.want, ",") {
				t.Fatalf("got %+v", got)
			}
			if method != http.MethodGet || seen.rawURL != c.wantURL {
				t.Fatalf("asked %s %q", method, seen.rawURL)
			}
			if seen.headers.Get(c.keyHeader) != c.keyValue {
				t.Fatalf("key header %q", seen.headers.Get(c.keyHeader))
			}
			if strings.Contains(seen.rawURL, syntheticKey) || strings.Contains(seen.body, syntheticKey) {
				t.Fatal("key travelled outside its header")
			}
		})
	}
}

func TestModelListStatusesMapToOutcomes(t *testing.T) {
	cases := map[int]ai.ProbeOutcome{401: ai.ProbeRefused, 403: ai.ProbeRefused, 404: ai.ProbeError, 500: ai.ProbeError}
	for status, want := range cases {
		srv, _ := fakeProvider(t, status, `{"error":"`+syntheticKey+`"}`)
		got := ai.NewHTTPModelLister().FetchModels(ctx, ai.Account{Provider: "openai_compatible", BaseURL: srv.URL}, syntheticKey)
		if got.Outcome != want || len(got.Names) != 0 {
			t.Fatalf("%d: %+v", status, got)
		}
	}
}

func TestModelListUnparseableOrUnreachableOrUnknownKind(t *testing.T) {
	srv, _ := fakeProvider(t, 200, `not json`)
	lister := ai.NewHTTPModelLister()
	if got := lister.FetchModels(ctx, ai.Account{Provider: "openai_compatible", BaseURL: srv.URL}, "k"); got.Outcome != ai.ProbeError {
		t.Fatalf("unparseable: %+v", got)
	}
	closed := httptest.NewServer(http.NotFoundHandler())
	url := closed.URL
	closed.Close()
	if got := lister.FetchModels(ctx, ai.Account{Provider: "openai_compatible", BaseURL: url}, "k"); got.Outcome != ai.ProbeUnreached {
		t.Fatalf("unreached: %+v", got)
	}
	if got := lister.FetchModels(ctx, ai.Account{Provider: "nope", BaseURL: url}, "k"); got.Outcome != ai.ProbeError {
		t.Fatalf("unknown kind: %+v", got)
	}
}

func TestEveryKindWithAChatAdapterCanListItsModels(t *testing.T) {
	for _, kind := range []string{"openai_compatible", "anthropic", "gemini"} {
		if !ai.HasAdapter(kind) {
			t.Fatalf("%s lost its adapter", kind)
		}
		srv, _ := fakeProvider(t, 200, `{"data":[{"id":"x"}],"models":[{"name":"x"}]}`)
		got := ai.NewHTTPModelLister().FetchModels(ctx, ai.Account{Provider: kind, BaseURL: srv.URL}, "k")
		if got.Outcome != ai.ProbeOK || len(got.Names) != 1 {
			t.Fatalf("%s: %+v", kind, got)
		}
	}
}
