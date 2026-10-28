package models

const (
	ProviderOpenAICompatible = "openai_compatible"
	ProviderAnthropic        = "anthropic"
	ProviderGemini           = "gemini"
)

const (
	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"
)

type Account struct {
	ID                string `json:"id"`
	AgencyID          string `json:"agency_id"`
	Name              string `json:"name"`
	Provider          string `json:"provider"`
	BaseURL           string `json:"base_url"`
	ModelName         string `json:"model_name"`
	Status            string `json:"status"`
	KeySet            bool   `json:"key_set"`
	LastTestResult    string `json:"last_test_result,omitempty"`
	LastTestedAt      string `json:"last_tested_at,omitempty"`
	LastTestPrompt    string `json:"last_test_prompt"`
	LastTestReply     string `json:"last_test_reply"`
	LastTestMs        int    `json:"last_test_ms"`
	ChatEnabled       bool   `json:"chat_enabled"`
	ChatLabel         string `json:"chat_label"`
	TokensPerCall     *int   `json:"tokens_per_call"`
	TokensPerDay      *int   `json:"tokens_per_day"`
	TokensPerMonth    *int   `json:"tokens_per_month"`
	RequestsPerDay    *int   `json:"requests_per_day"`
	RequestsPerMinute *int   `json:"requests_per_minute"`
	TokensPerWeek     *int   `json:"tokens_per_week"`
	RequestsPerWeek   *int   `json:"requests_per_week"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`

	chatEnabledOmitted bool
	chatLabelOmitted   bool
}

func (a *Account) ForgetTest() {
	a.LastTestResult, a.LastTestedAt = "", ""
	a.LastTestPrompt, a.LastTestReply, a.LastTestMs = "", "", 0
}

type KeyInput struct {
	Key string `json:"key"`
}

type Limits struct {
	TokensPerCall     *int `json:"tokens_per_call"`
	TokensPerDay      *int `json:"tokens_per_day"`
	TokensPerMonth    *int `json:"tokens_per_month"`
	RequestsPerDay    *int `json:"requests_per_day"`
	RequestsPerMinute *int `json:"requests_per_minute"`
	TokensPerWeek     *int `json:"tokens_per_week"`
	RequestsPerWeek   *int `json:"requests_per_week"`
}

type Usage struct {
	ID          string `json:"id"`
	AgencyID    string `json:"agency_id"`
	AccountID   string `json:"account_id"`
	RoleID      string `json:"role_id"`
	WindowKind  string `json:"window"`
	WindowStart string `json:"window_start"`
	Requests    int    `json:"requests"`
	TokensIn    int    `json:"tokens_in"`
	TokensOut   int    `json:"tokens_out"`
}

type UsageInput struct {
	AccountID string `json:"account_id"`
	RoleID    string `json:"role_id,omitempty"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
}

type CheckInput struct {
	AccountID  string `json:"account_id"`
	RoleID     string `json:"role_id,omitempty"`
	RoleLimits Limits `json:"role_limits"`
}

type Decision struct {
	Allowed   bool   `json:"allowed"`
	Reason    string `json:"reason,omitempty"`
	MaxTokens int    `json:"max_tokens"`
}

type LimitCount struct {
	Limit string `json:"limit"`
	Max   int    `json:"max"`
	Used  int    `json:"used"`
}

type UsageReport struct {
	AccountID string       `json:"account_id"`
	RoleID    string       `json:"role_id,omitempty"`
	Counts    []LimitCount `json:"counts"`
}
