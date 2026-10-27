package models

const MaxFineTuningChars = 20000

type AIRole struct {
	ID                    string `json:"id"`
	AgencyID              string `json:"agency_id"`
	Name                  string `json:"name"`
	Description           string `json:"description"`
	FineTuning            string `json:"fine_tuning"`
	UseAgencyInstructions bool   `json:"use_agency_instructions"`
	Status                string `json:"status"`
	TokensPerCall         int    `json:"tokens_per_call"`
	TokensPerDay          int    `json:"tokens_per_day"`
	TokensPerMonth        int    `json:"tokens_per_month"`
	RequestsPerDay        int    `json:"requests_per_day"`
	RequestsPerMinute     int    `json:"requests_per_minute"`
	CreatedAt             string `json:"created_at"`
	UpdatedAt             string `json:"updated_at"`
}

type AIRoleInput struct {
	Name                  string `json:"name"`
	Description           string `json:"description"`
	FineTuning            string `json:"fine_tuning"`
	UseAgencyInstructions *bool  `json:"use_agency_instructions,omitempty"`
	Status                string `json:"status"`
	TokensPerCall         int    `json:"tokens_per_call"`
	TokensPerDay          int    `json:"tokens_per_day"`
	TokensPerMonth        int    `json:"tokens_per_month"`
	RequestsPerDay        int    `json:"requests_per_day"`
	RequestsPerMinute     int    `json:"requests_per_minute"`
}

type AIRoleRevision struct {
	ID                    string `json:"id"`
	AgencyID              string `json:"agency_id"`
	RoleID                string `json:"role_id"`
	FineTuning            string `json:"fine_tuning"`
	UseAgencyInstructions bool   `json:"use_agency_instructions"`
	TokensPerCall         int    `json:"tokens_per_call"`
	TokensPerDay          int    `json:"tokens_per_day"`
	TokensPerMonth        int    `json:"tokens_per_month"`
	RequestsPerDay        int    `json:"requests_per_day"`
	RequestsPerMinute     int    `json:"requests_per_minute"`
	SavedBy               string `json:"saved_by"`
	SavedAt               string `json:"saved_at"`
}

func (r AIRole) Limits() Limits {
	return Limits{TokensPerCall: r.TokensPerCall, TokensPerDay: r.TokensPerDay, TokensPerMonth: r.TokensPerMonth, RequestsPerDay: r.RequestsPerDay, RequestsPerMinute: r.RequestsPerMinute}
}
