package models

const ProviderOpenAICompatible = "openai_compatible"

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
	TokensPerCall     int    `json:"tokens_per_call"`
	TokensPerDay      int    `json:"tokens_per_day"`
	TokensPerMonth    int    `json:"tokens_per_month"`
	RequestsPerDay    int    `json:"requests_per_day"`
	RequestsPerMinute int    `json:"requests_per_minute"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

type KeyInput struct {
	Key string `json:"key"`
}
