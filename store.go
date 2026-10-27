package ai

import (
	"context"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

const (
	roleAccount    = "account"
	roleAccountKey = "account_key"
)

type store = specstore.Store

func newStore(db *gorm.DB, s *spec.Spec, carriers map[string]any) (*store, error) {
	return specstore.New(db, s, carriers)
}

func newID() string { return specstore.NewID() }

func columnName(field string) string { return specstore.ColumnName(field) }

func encode(o spec.Object, v any) (map[string]any, error) { return specstore.Encode(o, v) }

func (m *accountManager) update(ctx context.Context, role, id string, v any) error {
	row, err := encode(m.st.Object(role), v)
	if err != nil {
		return err
	}
	delete(row, "id")
	return m.st.Query(ctx, role).Where("id = ?", id).Updates(row).Error
}
