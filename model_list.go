package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-ai/models"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

const (
	roleModelEntry   = "model_entry"
	maxModelNameRune = 200
	maxListedModels  = 500
)

type ModelEntry = models.ModelEntry

type ModelManager interface {
	ListModels(ctx context.Context, agencyID, provider string) ([]ModelEntry, error)
	RefreshModels(ctx context.Context, agencyID, accountID string) ([]ModelEntry, error)
	SetStartingModels(ctx context.Context, provider string, names []string) error
}

func WithModelLister(l ModelLister) Option {
	return func(m *accountManager) { m.lister = l }
}

func (m *accountManager) modelLister() ModelLister {
	if m.lister != nil {
		return m.lister
	}
	return NewHTTPModelLister()
}

func (m *accountManager) ListModels(ctx context.Context, agencyID, provider string) ([]ModelEntry, error) {
	if provider != "" && !HasAdapter(provider) {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedKind, provider)
	}
	q := m.st.Query(ctx, roleModelEntry).Where(
		"((agency_id = ? AND source = ?) OR (agency_id = ? AND source = ?))",
		agencyID, models.ModelFromProvider, "", models.ModelFromStarting)
	if provider != "" {
		q = q.Where("provider = ?", provider)
	}
	rows, err := specstore.List[ModelEntry](m.st, q, roleModelEntry)
	if err != nil {
		return nil, fmt.Errorf("ListModels: %w", err)
	}
	return arranged(rows), nil
}

func (m *accountManager) RefreshModels(ctx context.Context, agencyID, accountID string) ([]ModelEntry, error) {
	held, err := m.GetAccount(ctx, agencyID, accountID)
	if err != nil {
		return nil, err
	}
	key, err := m.AccountKey(ctx, agencyID, accountID)
	if err != nil {
		return nil, err
	}
	got := m.modelLister().FetchModels(ctx, held, key)
	if err := outcomeError(got.Outcome); err != nil {
		return nil, err
	}
	names := cleanModelNames(got.Names, key)
	if len(names) == 0 {
		return nil, ErrModelError
	}
	if err := m.replaceEntries(ctx, agencyID, held.Provider, models.ModelFromProvider, names); err != nil {
		return nil, fmt.Errorf("RefreshModels: %w", err)
	}
	return m.ListModels(ctx, agencyID, held.Provider)
}

func (m *accountManager) SetStartingModels(ctx context.Context, provider string, names []string) error {
	if !HasAdapter(provider) {
		return fmt.Errorf("%w: %q", ErrUnsupportedKind, provider)
	}
	if len(names) > maxListedModels {
		return fmt.Errorf("%w: more than %d models", ErrInvalidModelList, maxListedModels)
	}
	for _, n := range names {
		if !usableModelName(strings.TrimSpace(n)) {
			return fmt.Errorf("%w: %q is not a usable model name", ErrInvalidModelList, n)
		}
	}
	if err := m.replaceEntries(ctx, "", provider, models.ModelFromStarting, cleanModelNames(names, "")); err != nil {
		return fmt.Errorf("SetStartingModels: %w", err)
	}
	return nil
}

func (m *accountManager) replaceEntries(ctx context.Context, agencyID, provider, source string, names []string) error {
	listedAt := m.now()
	rows := make([]map[string]any, 0, len(names))
	for _, n := range names {
		row, err := encode(m.st.Object(roleModelEntry), ModelEntry{
			ID: newID(), AgencyID: agencyID, Provider: provider, ModelName: n, Source: source, ListedAt: listedAt,
		})
		if err != nil {
			return err
		}
		rows = append(rows, row)
	}
	table := m.st.Table(roleModelEntry)
	return m.st.Query(ctx, roleModelEntry).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(table).Where("agency_id = ? AND provider = ? AND source = ?", agencyID, provider, source).Delete(&struct{}{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Table(table).Create(rows).Error
	})
}

func usableModelName(n string) bool {
	if n == "" || !utf8.ValidString(n) || utf8.RuneCountInString(n) > maxModelNameRune {
		return false
	}
	for _, r := range n {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func cleanModelNames(names []string, withheld string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(names))
	for _, raw := range names {
		n := strings.TrimSpace(raw)
		if !usableModelName(n) || seen[n] || (withheld != "" && strings.Contains(n, withheld)) {
			continue
		}
		seen[n] = true
		out = append(out, n)
		if len(out) == maxListedModels {
			break
		}
	}
	return out
}

func arranged(rows []ModelEntry) []ModelEntry {
	provided := map[string]bool{}
	for _, r := range rows {
		if r.Source == models.ModelFromProvider {
			provided[r.Provider+"\x00"+r.ModelName] = true
		}
	}
	out := make([]ModelEntry, 0, len(rows))
	for _, r := range rows {
		if r.Source == models.ModelFromStarting && provided[r.Provider+"\x00"+r.ModelName] {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Provider != b.Provider {
			return a.Provider < b.Provider
		}
		if a.Source != b.Source {
			return a.Source == models.ModelFromProvider
		}
		return a.ModelName < b.ModelName
	})
	return out
}
