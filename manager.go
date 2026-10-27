package ai

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-ai/models"
)

type (
	Account  = models.Account
	KeyInput = models.KeyInput
)

type AccountManager interface {
	CreateAccount(ctx context.Context, agencyID string, in Account) (Account, error)
	ListAccounts(ctx context.Context, agencyID string) ([]Account, error)
	GetAccount(ctx context.Context, agencyID, accountID string) (Account, error)
	UpdateAccount(ctx context.Context, agencyID, accountID string, in Account) (Account, error)
	DeleteAccount(ctx context.Context, agencyID, accountID string) (Account, error)
	SetAccountLimits(ctx context.Context, agencyID, accountID string, in Limits) (Account, error)
	SetAccountKey(ctx context.Context, agencyID, accountID string, in KeyInput) (Account, error)
	TestAccount(ctx context.Context, agencyID, accountID string) (Account, error)
}

type KeyReader interface {
	AccountKey(ctx context.Context, agencyID, accountID string) (string, error)
}

type Manager interface {
	AccountManager
	UsageManager
	KeyReader
}

type Option func(*accountManager)

func WithMasterKey(master []byte) Option {
	return func(m *accountManager) { m.master = master }
}

type accountManager struct {
	st     *store
	now    func() string
	master []byte
	prober Prober
	clock  func() time.Time
}

type accountKeyRow struct {
	ID        string
	AgencyID  string
	Sealed    string
	UpdatedAt string
}

func NewManager(db *gorm.DB, s *spec.Spec, opts ...Option) (Manager, error) {
	if db == nil {
		return nil, fmt.Errorf("NewManager: db must not be nil")
	}
	st, err := newStore(db, s, map[string]any{
		roleAccount:    models.Account{},
		roleAccountKey: accountKeyRow{},
		roleUsage:      models.Usage{},
	})
	if err != nil {
		return nil, fmt.Errorf("NewManager: %w", err)
	}
	m := &accountManager{st: st, now: nowStamp}
	for _, opt := range opts {
		opt(m)
	}
	return m, nil
}

func nowStamp() string { return time.Now().UTC().Format(specstore.TimeLayout) }
