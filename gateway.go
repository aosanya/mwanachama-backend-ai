package ai

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/aosanya/mwanachama-backend-ai/models"
)

const (
	maxGatewayBody = 1 << 20
	modelUnreached = "the model could not be reached"
	modelErrored   = "the model answered with an error"
)

type GatewayCaller func(r *http.Request) (agencyID, agencyRoleID string, ok bool)

type gateway struct {
	m      Manager
	caller GatewayCaller
	client *http.Client
}

func NewGateway(m Manager, caller GatewayCaller, client *http.Client) http.Handler {
	if client == nil {
		client = http.DefaultClient
	}
	return &gateway{m: m, caller: caller, client: client}
}

func refuse(w http.ResponseWriter, status int, sentence string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": sentence}})
}

func (g *gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		refuse(w, http.StatusMethodNotAllowed, "only POST is served here")
		return
	}
	agencyID, agencyRoleID, ok := g.caller(r)
	if !ok {
		refuse(w, http.StatusUnauthorized, "the caller is not known")
		return
	}
	var payload map[string]any
	if err := json.NewDecoder(io.LimitReader(r.Body, maxGatewayBody)).Decode(&payload); err != nil {
		refuse(w, http.StatusBadRequest, "the request is not valid JSON")
		return
	}
	if stream, _ := payload["stream"].(bool); stream {
		refuse(w, http.StatusBadRequest, "streaming is not supported")
		return
	}
	res, err := g.m.ResolveAgencyRole(r.Context(), agencyID, agencyRoleID)
	if err != nil {
		refuse(w, http.StatusNotFound, "no AI role speaks for this seat")
		return
	}
	if res.Account == nil {
		refuse(w, http.StatusConflict, noAccountMapped)
		return
	}
	if res.Account.Status != "enabled" || !gatewayKinds[res.Account.Provider] {
		refuse(w, http.StatusConflict, "this AI role's account cannot be used through the gateway")
		return
	}
	d, hold, err := g.m.HoldRequest(r.Context(), agencyID, CheckInput{AccountID: res.Account.ID, RoleID: res.AIRoleID})
	if err != nil {
		refuse(w, http.StatusInternalServerError, "the limits could not be checked")
		return
	}
	if !d.Allowed {
		refuse(w, http.StatusTooManyRequests, d.Reason)
		return
	}
	defer hold.Release()
	clampMaxTokens(payload, d.MaxTokens)
	payload["model"] = res.Account.ModelName
	key, err := g.m.AccountKey(r.Context(), agencyID, res.Account.ID)
	if err != nil {
		refuse(w, http.StatusConflict, "this AI role's account has no usable key")
		return
	}
	relay := g.relayOpenAI
	if res.Account.Provider == models.ProviderAnthropic {
		relay = g.relayClaude
	}
	answer, fail := relay(r, *res.Account, key, payload, d.MaxTokens)
	if fail != nil {
		refuse(w, fail.status, fail.sentence)
		return
	}
	_, _ = hold.Settle(r.Context(), UsageInput{TokensIn: answer.tokensIn, TokensOut: answer.tokensOut})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(answer.body)
}

type answered struct {
	body                []byte
	tokensIn, tokensOut int
}

type failure struct {
	status   int
	sentence string
}

var gatewayKinds = map[string]bool{models.ProviderOpenAICompatible: true, models.ProviderAnthropic: true}

func (g *gateway) relayOpenAI(r *http.Request, a Account, key string, payload map[string]any, _ int) (answered, *failure) {
	adapter, _ := AdapterFor(a.Provider)
	out, err := adapter.BuildRequest(r.Context(), a, key, Request{})
	if err != nil {
		return answered{}, &failure{http.StatusBadGateway, "the request to the model could not be made"}
	}
	body, _ := json.Marshal(payload)
	out.Body, out.ContentLength = io.NopCloser(bytes.NewReader(body)), int64(len(body))
	resp, err := g.client.Do(out)
	if err != nil {
		return answered{}, &failure{http.StatusBadGateway, modelUnreached}
	}
	defer resp.Body.Close()
	answer, err := io.ReadAll(io.LimitReader(resp.Body, maxReplyBytes))
	if err != nil || resp.StatusCode/100 != 2 {
		return answered{}, &failure{http.StatusBadGateway, modelErrored}
	}
	in, outTok := answerUsage(answer)
	return answered{body: answer, tokensIn: in, tokensOut: outTok}, nil
}

func clampMaxTokens(payload map[string]any, limit int) {
	if limit <= 0 {
		return
	}
	if asked, _ := payload["max_tokens"].(float64); asked <= 0 || int(asked) > limit {
		payload["max_tokens"] = limit
	}
}

func answerUsage(answer []byte) (int, int) {
	var v struct {
		Usage struct {
			Prompt     int `json:"prompt_tokens"`
			Completion int `json:"completion_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(answer, &v)
	return v.Usage.Prompt, v.Usage.Completion
}
