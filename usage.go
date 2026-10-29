package ai

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-ai/models"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

const roleUsage = "usage"

type (
	Limits      = models.Limits
	Usage       = models.Usage
	UsageInput  = models.UsageInput
	CheckInput  = models.CheckInput
	Decision    = models.Decision
	UsageReport = models.UsageReport
)

type UsageManager interface {
	RecordUsage(ctx context.Context, agencyID string, in UsageInput) (Usage, error)
	CheckLimits(ctx context.Context, agencyID string, in CheckInput) (Decision, error)
	ListUsage(ctx context.Context, agencyID, accountID string) (UsageReport, error)
	ListRoleUsage(ctx context.Context, agencyID, accountID, roleID string) (UsageReport, error)
	ListTestRunUsage(ctx context.Context, agencyID, testRunID string) (TestRunReport, error)
}

func WithClock(clock func() time.Time) Option {
	return func(m *accountManager) {
		m.clock = clock
		m.now = func() string { return clock().UTC().Format(specstore.TimeLayout) }
	}
}

func (m *accountManager) instant() time.Time {
	if m.clock != nil {
		return m.clock().UTC()
	}
	return time.Now().UTC()
}

func windowStarts(t time.Time) map[string]string {
	const layout = "2006-01-02T15:04:05Z"
	return map[string]string{
		"minute":   t.Truncate(time.Minute).Format(layout),
		"day":      time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Format(layout),
		"month":    time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).Format(layout),
		"weekFrom": time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -6).Format(layout),
	}
}

func (m *accountManager) RecordUsage(ctx context.Context, agencyID string, in UsageInput) (Usage, error) {
	if _, err := m.GetAccount(ctx, agencyID, in.AccountID); err != nil {
		return Usage{}, err
	}
	if in.TokensIn < 0 || in.TokensOut < 0 {
		return Usage{}, fmt.Errorf("%w: tokens cannot be negative", ErrInvalidAccount)
	}
	var last Usage
	for window, start := range windowStarts(m.instant()) {
		if window == "weekFrom" {
			continue
		}
		id := fmt.Sprintf("%s|%s|%s|%s|%s", agencyID, in.AccountID, in.RoleID, window, start)
		var n int64
		q := m.st.Query(ctx, roleUsage).Where("id = ?", id)
		if err := q.Count(&n).Error; err != nil {
			return Usage{}, err
		}
		if n == 0 {
			row := Usage{ID: id, AgencyID: agencyID, AccountID: in.AccountID, RoleID: in.RoleID, WindowKind: window, WindowStart: start}
			if err := m.st.Insert(ctx, roleUsage, row); err != nil {
				return Usage{}, err
			}
		}
		err := m.st.Query(ctx, roleUsage).Where("id = ?", id).Updates(map[string]any{
			"requests":   gorm.Expr("requests + 1"),
			"tokens_in":  gorm.Expr("tokens_in + ?", in.TokensIn),
			"tokens_out": gorm.Expr("tokens_out + ?", in.TokensOut),
		}).Error
		if err != nil {
			return Usage{}, err
		}
		if window == "day" {
			if err := m.st.Take(m.st.Query(ctx, roleUsage).Where("id = ?", id), roleUsage, &last, ErrAccountNotFound); err != nil {
				return Usage{}, err
			}
		}
	}
	if in.TestRunID != "" {
		if err := m.recordTestRun(ctx, agencyID, in); err != nil {
			return Usage{}, err
		}
	}
	return last, nil
}

type tally struct{ Requests, Tokens int }

func (m *accountManager) sum(ctx context.Context, agencyID, accountID, roleID, window string) (int, int, error) {
	starts := windowStarts(m.instant())
	q := m.st.Query(ctx, roleUsage).Where("agency_id = ? AND account_id = ? AND window_kind = ? AND window_start = ?", agencyID, accountID, window, starts[window])
	if window == "week" {
		q = m.st.Query(ctx, roleUsage).Where("agency_id = ? AND account_id = ? AND window_kind = ? AND window_start >= ?", agencyID, accountID, "day", starts["weekFrom"])
	}
	if roleID != "" {
		q = q.Where("role_id = ?", roleID)
	}
	var t tally
	err := q.Select("COALESCE(SUM(requests),0) AS requests, COALESCE(SUM(tokens_in + tokens_out),0) AS tokens").Scan(&t).Error
	return t.Requests, t.Tokens, err
}

type limitSpec struct {
	name, window string
	tokens       bool
	max          func(Limits) *int
}

var limitSpecs = []limitSpec{
	{"requests per minute", "minute", false, func(l Limits) *int { return l.RequestsPerMinute }},
	{"requests per day", "day", false, func(l Limits) *int { return l.RequestsPerDay }},
	{"daily token", "day", true, func(l Limits) *int { return l.TokensPerDay }},
	{"monthly token", "month", true, func(l Limits) *int { return l.TokensPerMonth }},
	{"weekly request", "week", false, func(l Limits) *int { return l.RequestsPerWeek }},
	{"weekly token", "week", true, func(l Limits) *int { return l.TokensPerWeek }},
}

func accountLimits(a Account) Limits {
	return Limits{TokensPerCall: a.TokensPerCall, TokensPerDay: a.TokensPerDay, TokensPerMonth: a.TokensPerMonth, RequestsPerDay: a.RequestsPerDay, RequestsPerMinute: a.RequestsPerMinute, TokensPerWeek: a.TokensPerWeek, RequestsPerWeek: a.RequestsPerWeek, TokensPerTest: a.TokensPerTest, ScenariosAtOnce: a.ScenariosAtOnce}
}

func (m *accountManager) CheckLimits(ctx context.Context, agencyID string, in CheckInput) (Decision, error) {
	m.chat.writes.settled(agencyID, in.AccountID)
	a, err := m.GetAccount(ctx, agencyID, in.AccountID)
	if err != nil {
		return Decision{}, err
	}
	al := accountLimits(a)
	roleLimits, err := m.roleLimitsFor(ctx, agencyID, in)
	if err != nil {
		return Decision{}, err
	}
	test := effectiveTestLimits(al, roleLimits)
	if in.TestRunID != "" && test.tokensPerTest > 0 {
		used, err := m.testRunTokens(ctx, agencyID, in.TestRunID)
		if err != nil {
			return Decision{}, err
		}
		if used >= test.tokensPerTest {
			return Decision{Reason: testBudgetRefusal, BudgetReached: true, TokensPerTest: test.tokensPerTest, ScenariosAtOnce: test.scenariosAtOnce}, nil
		}
	}
	for _, owner := range []string{"this AI role's", "this account's"} {
		lim, roleID := roleLimits, in.RoleID
		if owner == "this account's" {
			lim, roleID = al, ""
		} else if in.RoleID == "" {
			continue
		}
		for _, ls := range limitSpecs {
			max := models.Count(ls.max(lim))
			if max == 0 {
				continue
			}
			req, tok, err := m.sum(ctx, agencyID, in.AccountID, roleID, ls.window)
			if err != nil {
				return Decision{}, err
			}
			used := req
			if ls.tokens {
				used = tok
			}
			if used >= max {
				return Decision{Allowed: false, Reason: refusal(owner, ls), TokensPerTest: test.tokensPerTest, ScenariosAtOnce: test.scenariosAtOnce}, nil
			}
		}
	}
	return Decision{
		Allowed:         true,
		MaxTokens:       lowerNonZero(models.Count(al.TokensPerCall), models.Count(roleLimits.TokensPerCall)),
		TokensPerTest:   test.tokensPerTest,
		ScenariosAtOnce: test.scenariosAtOnce,
	}, nil
}

func refusal(owner string, ls limitSpec) string {
	switch ls.name {
	case "requests per minute":
		return owner + " per-minute request limit is reached"
	case "requests per day":
		return owner + " daily request limit is reached"
	case "weekly request":
		return owner + " weekly request limit is reached"
	case "weekly token":
		return owner + " weekly token limit is reached"
	}
	return owner + " " + ls.name + " limit is reached"
}

func lowerNonZero(a, b int) int {
	if a == 0 {
		return b
	}
	if b == 0 || a < b {
		return a
	}
	return b
}

func (m *accountManager) ListUsage(ctx context.Context, agencyID, accountID string) (UsageReport, error) {
	return m.ListRoleUsage(ctx, agencyID, accountID, "")
}

func (m *accountManager) ListRoleUsage(ctx context.Context, agencyID, accountID, roleID string) (UsageReport, error) {
	m.chat.writes.settled(agencyID, accountID)
	a, err := m.GetAccount(ctx, agencyID, accountID)
	if err != nil {
		return UsageReport{}, err
	}
	rep := UsageReport{AccountID: accountID, RoleID: roleID}
	al := accountLimits(a)
	for _, ls := range limitSpecs {
		req, tok, err := m.sum(ctx, agencyID, accountID, roleID, ls.window)
		if err != nil {
			return UsageReport{}, err
		}
		used := req
		if ls.tokens {
			used = tok
		}
		rep.Counts = append(rep.Counts, models.LimitCount{Limit: ls.name, Max: models.Count(ls.max(al)), Used: used})
	}
	return rep, nil
}
