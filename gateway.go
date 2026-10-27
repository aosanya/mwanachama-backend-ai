package ai

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/aosanya/mwanachama-backend-ai/models"
)

const maxGatewayBody = 1 << 20

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
	if res.Account.Status != "enabled" || res.Account.Provider != models.ProviderOpenAICompatible {
		refuse(w, http.StatusConflict, "this AI role's account cannot be used through the gateway")
		return
	}
	d, err := g.m.CheckLimits(r.Context(), agencyID, CheckInput{AccountID: res.Account.ID, RoleID: res.AIRoleID})
	if err != nil {
		refuse(w, http.StatusInternalServerError, "the limits could not be checked")
		return
	}
	if !d.Allowed {
		refuse(w, http.StatusTooManyRequests, d.Reason)
		return
	}
	clampMaxTokens(payload, d.MaxTokens)
	payload["model"] = res.Account.ModelName
	key, err := g.m.AccountKey(r.Context(), agencyID, res.Account.ID)
	if err != nil {
		refuse(w, http.StatusConflict, "this AI role's account has no usable key")
		return
	}
	adapter, _ := AdapterFor(res.Account.Provider)
	out, err := adapter.BuildRequest(r.Context(), *res.Account, key, Request{})
	if err != nil {
		refuse(w, http.StatusBadGateway, "the request to the model could not be made")
		return
	}
	body, _ := json.Marshal(payload)
	out.Body, out.ContentLength = io.NopCloser(bytes.NewReader(body)), int64(len(body))
	resp, err := g.client.Do(out)
	if err != nil {
		refuse(w, http.StatusBadGateway, "the model could not be reached")
		return
	}
	defer resp.Body.Close()
	answer, err := io.ReadAll(io.LimitReader(resp.Body, maxReplyBytes))
	if err != nil || resp.StatusCode/100 != 2 {
		refuse(w, http.StatusBadGateway, "the model answered with an error")
		return
	}
	in, outTok := answerUsage(answer)
	_, _ = g.m.RecordUsage(r.Context(), agencyID, UsageInput{AccountID: res.Account.ID, RoleID: res.AIRoleID, TokensIn: in, TokensOut: outTok})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(answer)
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
