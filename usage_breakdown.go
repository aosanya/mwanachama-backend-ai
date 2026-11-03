package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aosanya/mwanachama-backend-ai/models"
)

func breakdownSpan(now time.Time, window string) (time.Time, time.Time, bool) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	switch window {
	case models.WindowToday:
		return today, today, true
	case models.Window7Days:
		return today.AddDate(0, 0, -6), today, true
	case models.WindowMonth:
		return firstOfMonth, today, true
	case models.WindowLastMonth:
		return firstOfMonth.AddDate(0, -1, 0), firstOfMonth.AddDate(0, 0, -1), true
	}
	return time.Time{}, time.Time{}, false
}

func checkBreakdownInputs(window, groupBy string) error {
	if _, _, ok := breakdownSpan(time.Time{}, window); !ok {
		return fmt.Errorf("%w: the window is one of %s", ErrInvalidAccount, strings.Join(models.UsageWindows, ", "))
	}
	if groupBy != "" && groupBy != models.GroupByPerson && groupBy != models.GroupByAgent {
		return fmt.Errorf("%w: group_by is person or agent", ErrInvalidAccount)
	}
	return nil
}

type dayTotal struct {
	Day       string `gorm:"column:day"`
	Model     string `gorm:"column:model"`
	RoleID    string `gorm:"column:role_id"`
	Kind      string `gorm:"column:kind"`
	Person    string `gorm:"column:person"`
	Requests  int    `gorm:"column:requests"`
	TokensIn  int    `gorm:"column:tokens_in"`
	TokensOut int    `gorm:"column:tokens_out"`
}

func (m *accountManager) UsageBreakdown(ctx context.Context, agencyID, window, groupBy, roleID, model string) (Breakdown, error) {
	if err := checkBreakdownInputs(window, groupBy); err != nil {
		return Breakdown{}, err
	}
	from, to, _ := breakdownSpan(m.instant(), window)
	out := Breakdown{Window: window, From: from.Format(models.UsageDayLayout), To: to.Format(models.UsageDayLayout), Rows: []BreakdownRow{}}
	q := m.st.Query(ctx, roleUsageDay).Where("agency_id = ? AND day >= ? AND day <= ?", agencyID, out.From, out.To)
	if roleID != "" {
		q = q.Where("role_id = ?", roleID)
	}
	if model != "" {
		q = q.Where("model = ?", model)
	}
	var totals []dayTotal
	err := q.Select("day, model, role_id, kind, person, SUM(requests) AS requests, SUM(tokens_in) AS tokens_in, SUM(tokens_out) AS tokens_out").
		Group("day, model, role_id, kind, person").Scan(&totals).Error
	if err != nil {
		return Breakdown{}, fmt.Errorf("UsageBreakdown: %w", err)
	}
	names, err := m.roleNames(ctx, agencyID)
	if err != nil {
		return Breakdown{}, err
	}
	out.Rows = breakdownRows(totals, names)
	return out, nil
}

func breakdownRows(totals []dayTotal, names map[string]string) []BreakdownRow {
	out := make([]BreakdownRow, 0, len(totals))
	for _, d := range totals {
		kind := d.Kind
		if kind == "" {
			kind = models.KindOther
		}
		out = append(out, BreakdownRow{Day: d.Day, Model: d.Model, RoleID: d.RoleID, Role: names[d.RoleID], Kind: kind, Person: d.Person, TokensIn: d.TokensIn, TokensOut: d.TokensOut, Requests: d.Requests})
	}
	sort.Slice(out, func(i, j int) bool { return rowLess(out[i], out[j]) })
	return out
}

func rowLess(a, b BreakdownRow) bool {
	if a.Day != b.Day {
		return a.Day < b.Day
	}
	for _, pair := range [][2]string{{a.Model, b.Model}, {a.RoleID, b.RoleID}, {a.Kind, b.Kind}, {a.Person, b.Person}} {
		if pair[0] != pair[1] {
			return pair[0] < pair[1]
		}
	}
	return false
}

func (m *accountManager) roleNames(ctx context.Context, agencyID string) (map[string]string, error) {
	roles, err := m.ListAIRoles(ctx, agencyID)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(roles))
	for _, r := range roles {
		names[r.ID] = r.Name
	}
	return names, nil
}
