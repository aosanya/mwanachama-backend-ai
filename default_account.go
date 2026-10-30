package ai

import (
	"context"
	"fmt"
)

func (m *accountManager) SetDefaultAccount(ctx context.Context, agencyID, accountID string) (Account, error) {
	held, err := m.GetAccount(ctx, agencyID, accountID)
	if err != nil {
		return Account{}, err
	}
	if err := m.clearDefault(ctx, agencyID); err != nil {
		return Account{}, fmt.Errorf("SetDefaultAccount: %w", err)
	}
	held.IsDefault = true
	held.UpdatedAt = m.now()
	if err := m.update(ctx, roleAccount, held.ID, held); err != nil {
		return Account{}, fmt.Errorf("SetDefaultAccount: %w", err)
	}
	return held, nil
}

func (m *accountManager) GetDefaultAccount(ctx context.Context, agencyID string) (Account, error) {
	var out Account
	q := m.st.Query(ctx, roleAccount).Where("agency_id = ? AND is_default = ?", agencyID, true)
	if err := m.st.Take(q, roleAccount, &out, ErrNoDefaultAccount); err != nil {
		return Account{}, err
	}
	return out, nil
}

func (m *accountManager) clearDefault(ctx context.Context, agencyID string) error {
	return m.st.Query(ctx, roleAccount).
		Where("agency_id = ? AND is_default = ?", agencyID, true).
		Updates(map[string]any{"is_default": false}).Error
}
