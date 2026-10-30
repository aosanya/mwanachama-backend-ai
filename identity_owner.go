package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aosanya/mwanachama-backend-shared/identity"
)

const identityModule = "ai"

var ErrNoStableIdentity = errors.New("ai: this object has no stable identity")

var identityObjects = map[string]bool{
	roleAccount:  true,
	roleAIRole:   true,
	roleRevision: true,
}

func (m *accountManager) StableIdentity(ctx context.Context, object, id string) (string, error) {
	if !identityObjects[object] {
		return "", fmt.Errorf("%s: %w", object, ErrNoStableIdentity)
	}
	present, err := m.rowExists(ctx, object, id)
	if err != nil {
		return "", fmt.Errorf("StableIdentity: %w", err)
	}
	if !present {
		return "", fmt.Errorf("%s %s: %w", object, id, identity.ErrNotFound)
	}
	raw, err := identity.New(identityModule, m.st.Spec().Instance, object+"-"+id)
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", object, id, err)
	}
	return raw.String(), nil
}

func (m *accountManager) Resolve(ctx context.Context, raw string) (string, string, error) {
	r, err := identity.Parse(raw)
	if err != nil {
		return "", "", err
	}
	if r.Module != identityModule || r.Instance != m.st.Spec().Instance {
		return "", "", fmt.Errorf("%s: %w", raw, identity.ErrNotOwned)
	}
	object, id, found := strings.Cut(r.Identity, "-")
	if !found || id == "" || !identityObjects[object] {
		return "", "", fmt.Errorf("%s: %w", raw, identity.ErrNotFound)
	}
	present, err := m.rowExists(ctx, object, id)
	if err != nil {
		return "", "", fmt.Errorf("Resolve: %w", err)
	}
	if !present {
		return "", "", fmt.Errorf("%s: %w", raw, identity.ErrNotFound)
	}
	return object, id, nil
}

func (m *accountManager) rowExists(ctx context.Context, object, id string) (bool, error) {
	var n int64
	if err := m.st.Query(ctx, object).Where("id = ?", id).Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}
