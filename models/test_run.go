package models

type TestRunUsage struct {
	ID        string `json:"id"`
	AgencyID  string `json:"agency_id"`
	AccountID string `json:"account_id"`
	RoleID    string `json:"role_id"`
	TestRunID string `json:"test_run_id"`
	Step      string `json:"step"`
	Requests  int    `json:"requests"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
}

type TestRunStepUsage struct {
	Step      string `json:"step"`
	Requests  int    `json:"requests"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
	Tokens    int    `json:"tokens"`
}

type TestRunReport struct {
	TestRunID string             `json:"test_run_id"`
	Steps     []TestRunStepUsage `json:"steps"`
	Requests  int                `json:"requests"`
	TokensIn  int                `json:"tokens_in"`
	TokensOut int                `json:"tokens_out"`
	Tokens    int                `json:"tokens"`
}
