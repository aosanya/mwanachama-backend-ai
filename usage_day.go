package ai

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-ai/models"
)

const roleUsageDay = "usage_day"

type (
	UsageDay     = models.UsageDay
	Breakdown    = models.Breakdown
	BreakdownRow = models.BreakdownRow
)

var kindSentence = "the kind of work is not one of " + strings.Join(models.UsageKinds, ", ")

func checkKind(kind string) error {
	if kind != "" && !models.ValidKind(kind) {
		return fmt.Errorf("%w: %s", ErrInvalidAccount, kindSentence)
	}
	return nil
}

func (m *accountManager) recordDay(ctx context.Context, agencyID string, account Account, in UsageInput) error {
	day := m.instant().Format(models.UsageDayLayout)
	id := strings.Join([]string{agencyID, in.AccountID, in.RoleID, day, account.ModelName, in.Kind, in.Person}, "|")
	row := UsageDay{ID: id, AgencyID: agencyID, AccountID: in.AccountID, RoleID: in.RoleID, Day: day, Model: account.ModelName, Kind: in.Kind, Person: in.Person}
	if err := m.st.InsertOnce(ctx, roleUsageDay, row); err != nil {
		return err
	}
	return m.st.Query(ctx, roleUsageDay).Where("id = ?", id).Updates(map[string]any{
		"requests":   gorm.Expr("requests + 1"),
		"tokens_in":  gorm.Expr("tokens_in + ?", in.TokensIn),
		"tokens_out": gorm.Expr("tokens_out + ?", in.TokensOut),
	}).Error
}
