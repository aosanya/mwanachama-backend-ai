package ai

import (
	"context"
	"errors"
	"fmt"
	"github.com/aosanya/mwanachama-backend-ai/models"
	"strings"

	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

const maxKeyBytes = 4096

func (m *accountManager) valid(a Account) error {
	if err := m.checks(roleAccount, a, ErrInvalidAccount); err != nil {
		return err
	}
	if !HasAdapter(a.Provider) {
		return fmt.Errorf("%w: %q", ErrUnsupportedKind, a.Provider)
	}
	if m.baseURLPolicy != nil {
		if err := m.baseURLPolicy(a.BaseURL); err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidAccount, err)
		}
	}
	for _, n := range []*int{a.TokensPerCall, a.TokensPerDay, a.TokensPerMonth, a.RequestsPerDay, a.RequestsPerMinute, a.TokensPerWeek, a.RequestsPerWeek, a.TokensPerTest, a.ScenariosAtOnce} {
		if models.Count(n) < 0 {
			return fmt.Errorf("%w: a limit cannot be negative", ErrInvalidAccount)
		}
	}
	return nil
}

func (m *accountManager) nameTaken(ctx context.Context, agencyID, name, except string) (bool, error) {
	var n int64
	q := m.st.Query(ctx, roleAccount).Where("agency_id = ? AND name = ?", agencyID, name)
	if except != "" {
		q = q.Where("id <> ?", except)
	}
	err := q.Count(&n).Error
	return n > 0, err
}

func (m *accountManager) CreateAccount(ctx context.Context, agencyID string, in Account) (Account, error) {
	in.AgencyID = agencyID
	in.KeySet = false
	in.IsDefault = false
	in.ForgetTest()
	if err := m.valid(in); err != nil {
		return Account{}, err
	}
	if taken, err := m.nameTaken(ctx, agencyID, in.Name, ""); err != nil {
		return Account{}, fmt.Errorf("CreateAccount: %w", err)
	} else if taken {
		return Account{}, ErrDuplicateName
	}
	in.ID = newID()
	in.CreatedAt = m.now()
	in.UpdatedAt = in.CreatedAt
	if err := m.st.Insert(ctx, roleAccount, in); err != nil {
		return Account{}, fmt.Errorf("CreateAccount: %w", err)
	}
	return in, nil
}

func (m *accountManager) ListAccounts(ctx context.Context, agencyID string) ([]Account, error) {
	q := m.st.Query(ctx, roleAccount).Where("agency_id = ?", agencyID).Order("created_at asc")
	out, err := specstore.List[Account](m.st, q, roleAccount)
	if err != nil {
		return nil, fmt.Errorf("ListAccounts: %w", err)
	}
	return out, nil
}

func (m *accountManager) GetAccount(ctx context.Context, agencyID, accountID string) (Account, error) {
	var out Account
	q := m.st.Query(ctx, roleAccount).Where("agency_id = ? AND id = ?", agencyID, accountID)
	if err := m.st.Take(q, roleAccount, &out, ErrAccountNotFound); err != nil {
		return Account{}, err
	}
	return out, nil
}

func (m *accountManager) UpdateAccount(ctx context.Context, agencyID, accountID string, in Account) (Account, error) {
	held, err := m.GetAccount(ctx, agencyID, accountID)
	if err != nil {
		return Account{}, err
	}
	held.Name, held.Provider, held.BaseURL, held.ModelName, held.Status = in.Name, in.Provider, in.BaseURL, in.ModelName, in.Status
	held.TokensPerCall, held.TokensPerDay, held.TokensPerMonth = in.TokensPerCall, in.TokensPerDay, in.TokensPerMonth
	held.RequestsPerDay, held.RequestsPerMinute = in.RequestsPerDay, in.RequestsPerMinute
	held.TokensPerWeek, held.RequestsPerWeek = in.TokensPerWeek, in.RequestsPerWeek
	held.TokensPerTest, held.ScenariosAtOnce = in.TokensPerTest, in.ScenariosAtOnce
	held = held.WithChatOf(in)
	if err := m.valid(held); err != nil {
		return Account{}, err
	}
	if taken, err := m.nameTaken(ctx, agencyID, held.Name, held.ID); err != nil {
		return Account{}, fmt.Errorf("UpdateAccount: %w", err)
	} else if taken {
		return Account{}, ErrDuplicateName
	}
	held.UpdatedAt = m.now()
	if err := m.update(ctx, roleAccount, held.ID, held); err != nil {
		return Account{}, fmt.Errorf("UpdateAccount: %w", err)
	}
	return held, nil
}

func (m *accountManager) SetAccountLimits(ctx context.Context, agencyID, accountID string, in Limits) (Account, error) {
	held, err := m.GetAccount(ctx, agencyID, accountID)
	if err != nil {
		return Account{}, err
	}
	held.TokensPerCall, held.TokensPerDay, held.TokensPerMonth = in.TokensPerCall, in.TokensPerDay, in.TokensPerMonth
	held.RequestsPerDay, held.RequestsPerMinute = in.RequestsPerDay, in.RequestsPerMinute
	held.TokensPerWeek, held.RequestsPerWeek = in.TokensPerWeek, in.RequestsPerWeek
	held.TokensPerTest, held.ScenariosAtOnce = in.TokensPerTest, in.ScenariosAtOnce
	if err := m.valid(held); err != nil {
		return Account{}, err
	}
	held.UpdatedAt = m.now()
	if err := m.update(ctx, roleAccount, held.ID, held); err != nil {
		return Account{}, fmt.Errorf("SetAccountLimits: %w", err)
	}
	return held, nil
}

func (m *accountManager) DeleteAccount(ctx context.Context, agencyID, accountID string) (Account, error) {
	held, err := m.GetAccount(ctx, agencyID, accountID)
	if err != nil {
		return Account{}, err
	}
	if err := m.st.Query(ctx, roleAccountKey).Where("id = ?", held.ID).Delete(&struct{}{}).Error; err != nil {
		return Account{}, fmt.Errorf("DeleteAccount: %w", err)
	}
	if err := m.st.Query(ctx, roleAccount).Where("id = ?", held.ID).Delete(&struct{}{}).Error; err != nil {
		return Account{}, fmt.Errorf("DeleteAccount: %w", err)
	}
	if err := m.releaseRoles(ctx, agencyID, held.ID); err != nil {
		return Account{}, fmt.Errorf("DeleteAccount: %w", err)
	}
	if err := m.forgetPicks(ctx, agencyID, held.ID); err != nil {
		return Account{}, fmt.Errorf("DeleteAccount: %w", err)
	}
	return held, nil
}

func (m *accountManager) releaseRoles(ctx context.Context, agencyID, accountID string) error {
	return m.st.Query(ctx, roleAIRole).
		Where("agency_id = ? AND account_id = ?", agencyID, accountID).
		Updates(map[string]any{"account_id": "", "updated_at": m.now()}).Error
}

func (m *accountManager) SetAccountKey(ctx context.Context, agencyID, accountID string, in KeyInput) (Account, error) {
	held, err := m.GetAccount(ctx, agencyID, accountID)
	if err != nil {
		return Account{}, err
	}
	if len(m.master) == 0 {
		return Account{}, ErrNoMasterKey
	}
	if strings.TrimSpace(in.Key) == "" || len(in.Key) > maxKeyBytes {
		return Account{}, ErrInvalidKey
	}
	sealed, err := seal(m.master, in.Key)
	if err != nil {
		return Account{}, errors.New("the key could not be sealed")
	}
	row := accountKeyRow{ID: held.ID, AgencyID: agencyID, Sealed: sealed, UpdatedAt: m.now()}
	if err := m.st.Query(ctx, roleAccountKey).Where("id = ?", held.ID).Delete(&struct{}{}).Error; err != nil {
		return Account{}, fmt.Errorf("SetAccountKey: %w", err)
	}
	if err := m.st.Insert(ctx, roleAccountKey, row); err != nil {
		return Account{}, fmt.Errorf("SetAccountKey: %w", err)
	}
	held.KeySet = true
	held.ForgetTest()
	held.UpdatedAt = row.UpdatedAt
	if err := m.update(ctx, roleAccount, held.ID, held); err != nil {
		return Account{}, fmt.Errorf("SetAccountKey: %w", err)
	}
	return held, nil
}

func (m *accountManager) AccountKey(ctx context.Context, agencyID, accountID string) (string, error) {
	if len(m.master) == 0 {
		return "", ErrNoMasterKey
	}
	if _, err := m.GetAccount(ctx, agencyID, accountID); err != nil {
		return "", err
	}
	var row accountKeyRow
	q := m.st.Query(ctx, roleAccountKey).Where("id = ? AND agency_id = ?", accountID, agencyID)
	if err := m.st.Take(q, roleAccountKey, &row, ErrNoKey); err != nil {
		return "", err
	}
	return unseal(m.master, row.Sealed)
}
