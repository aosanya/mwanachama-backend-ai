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
	TokensPerCall         *int   `json:"tokens_per_call"`
	TokensPerDay          *int   `json:"tokens_per_day"`
	TokensPerMonth        *int   `json:"tokens_per_month"`
	RequestsPerDay        *int   `json:"requests_per_day"`
	RequestsPerMinute     *int   `json:"requests_per_minute"`
	TokensPerWeek         *int   `json:"tokens_per_week"`
	RequestsPerWeek       *int   `json:"requests_per_week"`
	TokensPerTest         *int   `json:"tokens_per_test"`
	ScenariosAtOnce       *int   `json:"scenarios_at_once"`
	AccountID             string `json:"account_id"`
	AgencyRoleID          string `json:"agency_role_id"`
	CreatedAt             string `json:"created_at"`
	UpdatedAt             string `json:"updated_at"`
}

type AIRoleInput struct {
	Name                  string `json:"name"`
	Description           string `json:"description"`
	FineTuning            string `json:"fine_tuning"`
	UseAgencyInstructions *bool  `json:"use_agency_instructions,omitempty"`
	Status                string `json:"status"`
	TokensPerCall         *int   `json:"tokens_per_call"`
	TokensPerDay          *int   `json:"tokens_per_day"`
	TokensPerMonth        *int   `json:"tokens_per_month"`
	RequestsPerDay        *int   `json:"requests_per_day"`
	RequestsPerMinute     *int   `json:"requests_per_minute"`
	TokensPerWeek         *int   `json:"tokens_per_week"`
	RequestsPerWeek       *int   `json:"requests_per_week"`
	TokensPerTest         *int   `json:"tokens_per_test"`
	ScenariosAtOnce       *int   `json:"scenarios_at_once"`
}

type AIRoleRevision struct {
	ID                    string `json:"id"`
	AgencyID              string `json:"agency_id"`
	RoleID                string `json:"role_id"`
	FineTuning            string `json:"fine_tuning"`
	UseAgencyInstructions bool   `json:"use_agency_instructions"`
	TokensPerCall         *int   `json:"tokens_per_call"`
	TokensPerDay          *int   `json:"tokens_per_day"`
	TokensPerMonth        *int   `json:"tokens_per_month"`
	RequestsPerDay        *int   `json:"requests_per_day"`
	RequestsPerMinute     *int   `json:"requests_per_minute"`
	TokensPerWeek         *int   `json:"tokens_per_week"`
	RequestsPerWeek       *int   `json:"requests_per_week"`
	TokensPerTest         *int   `json:"tokens_per_test"`
	ScenariosAtOnce       *int   `json:"scenarios_at_once"`
	SavedBy               string `json:"saved_by"`
	SavedAt               string `json:"saved_at"`
}

func Count(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func (r AIRole) Limits() Limits {
	return Limits{TokensPerCall: r.TokensPerCall, TokensPerDay: r.TokensPerDay, TokensPerMonth: r.TokensPerMonth, RequestsPerDay: r.RequestsPerDay, RequestsPerMinute: r.RequestsPerMinute, TokensPerWeek: r.TokensPerWeek, RequestsPerWeek: r.RequestsPerWeek, TokensPerTest: r.TokensPerTest, ScenariosAtOnce: r.ScenariosAtOnce}
}

type AIRoleMappingInput struct {
	AccountID    string `json:"account_id"`
	AgencyRoleID string `json:"agency_role_id"`
}

type LimitInForce struct {
	Limit  string `json:"limit"`
	Max    int    `json:"max"`
	Used   int    `json:"used"`
	Source string `json:"source"`
}

type InstructionPart struct {
	Source string `json:"source"`
	Text   string `json:"text"`
}

type Resolution struct {
	AIRoleID      string            `json:"ai_role_id"`
	AgencyRoleID  string            `json:"agency_role_id"`
	Instructions  string            `json:"instructions"`
	Parts         []InstructionPart `json:"parts"`
	Account       *Account          `json:"account,omitempty"`
	AccountSource string            `json:"account_source,omitempty"`
	Limits        []LimitInForce    `json:"limits,omitempty"`
	Message       string            `json:"message,omitempty"`
}
