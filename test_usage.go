package ai

import (
	"context"
	"fmt"
	"sort"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-ai/models"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

const roleTestRunUsage = "test_run_usage"

const testBudgetRefusal = "this test's tokens per test limit is reached"

type TestRunUsage = models.TestRunUsage
type TestRunReport = models.TestRunReport

type testLimits struct{ tokensPerTest, scenariosAtOnce int }

func effectiveTestLimits(account, role Limits) testLimits {
	return testLimits{
		tokensPerTest:   lowerNonZero(models.Count(account.TokensPerTest), models.Count(role.TokensPerTest)),
		scenariosAtOnce: lowerNonZero(models.Count(account.ScenariosAtOnce), models.Count(role.ScenariosAtOnce)),
	}
}

func (m *accountManager) recordTestRun(ctx context.Context, agencyID string, in UsageInput) error {
	id := fmt.Sprintf("%s|%s|%s|%s|%s", agencyID, in.TestRunID, in.Step, in.AccountID, in.RoleID)
	var n int64
	if err := m.st.Query(ctx, roleTestRunUsage).Where("id = ?", id).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		row := TestRunUsage{ID: id, AgencyID: agencyID, AccountID: in.AccountID, RoleID: in.RoleID, TestRunID: in.TestRunID, Step: in.Step}
		if err := m.st.Insert(ctx, roleTestRunUsage, row); err != nil {
			return err
		}
	}
	return m.st.Query(ctx, roleTestRunUsage).Where("id = ?", id).Updates(map[string]any{
		"requests":   gorm.Expr("requests + 1"),
		"tokens_in":  gorm.Expr("tokens_in + ?", in.TokensIn),
		"tokens_out": gorm.Expr("tokens_out + ?", in.TokensOut),
	}).Error
}

func (m *accountManager) testRunTokens(ctx context.Context, agencyID, testRunID string) (int, error) {
	var total int
	err := m.st.Query(ctx, roleTestRunUsage).
		Where("agency_id = ? AND test_run_id = ?", agencyID, testRunID).
		Select("COALESCE(SUM(tokens_in + tokens_out),0)").Scan(&total).Error
	return total, err
}

func (m *accountManager) ListTestRunUsage(ctx context.Context, agencyID, testRunID string) (TestRunReport, error) {
	q := m.st.Query(ctx, roleTestRunUsage).Where("agency_id = ? AND test_run_id = ?", agencyID, testRunID)
	rows, err := specstore.List[TestRunUsage](m.st, q, roleTestRunUsage)
	if err != nil {
		return TestRunReport{}, fmt.Errorf("ListTestRunUsage: %w", err)
	}
	return foldTestRun(testRunID, rows), nil
}

func foldTestRun(testRunID string, rows []TestRunUsage) TestRunReport {
	rep := TestRunReport{TestRunID: testRunID, Steps: []models.TestRunStepUsage{}}
	byStep := map[string]*models.TestRunStepUsage{}
	for _, r := range rows {
		s := byStep[r.Step]
		if s == nil {
			s = &models.TestRunStepUsage{Step: r.Step}
			byStep[r.Step] = s
		}
		s.Requests += r.Requests
		s.TokensIn += r.TokensIn
		s.TokensOut += r.TokensOut
		s.Tokens += r.TokensIn + r.TokensOut
		rep.Requests += r.Requests
		rep.TokensIn += r.TokensIn
		rep.TokensOut += r.TokensOut
		rep.Tokens += r.TokensIn + r.TokensOut
	}
	for _, s := range byStep {
		rep.Steps = append(rep.Steps, *s)
	}
	sort.Slice(rep.Steps, func(i, j int) bool { return rep.Steps[i].Step < rep.Steps[j].Step })
	return rep
}
