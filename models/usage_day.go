package models

const (
	KindChat          = "chat"
	KindWorkflowRun   = "workflow_run"
	KindAgencyReview  = "agency_review"
	KindTestRun       = "test_run"
	KindBackgroundJob = "background_job"
	KindOther         = "other"
	WindowToday       = "today"
	Window7Days       = "7d"
	WindowMonth       = "month"
	WindowLastMonth   = "last_month"
	GroupByPerson     = "person"
	GroupByAgent      = "agent"
	UsageDayLayout    = "2006-01-02"
)

var UsageKinds = []string{KindChat, KindWorkflowRun, KindAgencyReview, KindTestRun, KindBackgroundJob}

var UsageWindows = []string{WindowToday, Window7Days, WindowMonth, WindowLastMonth}

func ValidKind(kind string) bool {
	for _, k := range UsageKinds {
		if k == kind {
			return true
		}
	}
	return false
}

type UsageDay struct {
	ID        string `json:"id"`
	AgencyID  string `json:"agency_id"`
	AccountID string `json:"account_id"`
	RoleID    string `json:"role_id"`
	Day       string `json:"day"`
	Model     string `json:"model"`
	Kind      string `json:"kind"`
	Person    string `json:"person"`
	Requests  int    `json:"requests"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
}

type BreakdownRow struct {
	Day       string `json:"day"`
	Model     string `json:"model"`
	RoleID    string `json:"role_id"`
	Role      string `json:"role"`
	Kind      string `json:"kind"`
	Person    string `json:"person"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
	Requests  int    `json:"requests"`
}

type Breakdown struct {
	Window string         `json:"window"`
	From   string         `json:"from"`
	To     string         `json:"to"`
	Rows   []BreakdownRow `json:"rows"`
}
