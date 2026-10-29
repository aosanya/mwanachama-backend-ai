package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/aosanya/mwanachama-backend-ai/models"
)

const modelsPageSize = "1000"

type modelsAdapter interface {
	BuildModelsRequest(ctx context.Context, a Account, key string) (*http.Request, error)
	ParseModels(body []byte) ([]string, error)
}

var modelsAdapterByKind = map[string]modelsAdapter{
	models.ProviderOpenAICompatible: openAIModels{},
	models.ProviderAnthropic:        anthropicModels{},
	models.ProviderGemini:           geminiModels{},
}

func getRequest(ctx context.Context, endpoint string) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
}

type openAIModels struct{}

func (openAIModels) BuildModelsRequest(ctx context.Context, a Account, key string) (*http.Request, error) {
	req, err := getRequest(ctx, base(a)+"/models")
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	return req, nil
}

func (openAIModels) ParseModels(body []byte) ([]string, error) { return idsOfData(body) }

type anthropicModels struct{}

func (anthropicModels) BuildModelsRequest(ctx context.Context, a Account, key string) (*http.Request, error) {
	req, err := getRequest(ctx, base(a)+"/v1/models?limit="+modelsPageSize)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", anthropicVersion)
	return req, nil
}

func (anthropicModels) ParseModels(body []byte) ([]string, error) { return idsOfData(body) }

func idsOfData(body []byte) ([]string, error) {
	var v struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(v.Data))
	for _, d := range v.Data {
		out = append(out, d.ID)
	}
	return out, nil
}

type geminiModels struct{}

func (geminiModels) BuildModelsRequest(ctx context.Context, a Account, key string) (*http.Request, error) {
	req, err := getRequest(ctx, base(a)+"/v1beta/models?pageSize="+modelsPageSize)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-goog-api-key", key)
	return req, nil
}

func (geminiModels) ParseModels(body []byte) ([]string, error) {
	var v struct {
		Models []struct {
			Name    string   `json:"name"`
			Methods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(v.Models))
	for _, m := range v.Models {
		if len(m.Methods) > 0 && !contains(m.Methods, "generateContent") {
			continue
		}
		out = append(out, strings.TrimPrefix(m.Name, "models/"))
	}
	return out, nil
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

type ModelListing struct {
	Outcome ProbeOutcome
	Names   []string
}

type ModelLister interface {
	FetchModels(ctx context.Context, a Account, key string) ModelListing
}

type httpModelLister struct{ client *http.Client }

func NewHTTPModelLister() ModelLister {
	return httpModelLister{client: &http.Client{Timeout: 20 * time.Second}}
}

func (l httpModelLister) FetchModels(ctx context.Context, a Account, key string) ModelListing {
	ad, ok := modelsAdapterByKind[a.Provider]
	if !ok {
		return ModelListing{Outcome: ProbeError}
	}
	req, err := ad.BuildModelsRequest(ctx, a, key)
	if err != nil {
		return ModelListing{Outcome: ProbeUnreached}
	}
	body, outcome := send(l.client, req)
	if outcome != ProbeOK {
		return ModelListing{Outcome: outcome}
	}
	names, err := ad.ParseModels(body)
	if err != nil {
		return ModelListing{Outcome: ProbeError}
	}
	return ModelListing{Outcome: ProbeOK, Names: names}
}
