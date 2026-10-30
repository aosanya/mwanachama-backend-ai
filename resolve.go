package ai

import (
	"context"
	"errors"
	"fmt"
	"github.com/aosanya/mwanachama-backend-ai/models"

	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

const (
	noAccountMapped    = "no account is mapped"
	accountFromRole    = "role"
	accountFromDefault = "default"
)

func (m *accountManager) SetAIRoleMapping(ctx context.Context, agencyID, roleID string, in AIRoleMappingInput) (AIRole, error) {
	held, err := m.GetAIRole(ctx, agencyID, roleID)
	if err != nil {
		return AIRole{}, err
	}
	if in.AccountID != "" {
		if _, err := m.GetAccount(ctx, agencyID, in.AccountID); err != nil {
			return AIRole{}, err
		}
	}
	next := held
	next.AccountID, next.AgencyRoleID = in.AccountID, in.AgencyRoleID
	next.UpdatedAt = m.now()
	if err := m.update(ctx, roleAIRole, next.ID, next); err != nil {
		return AIRole{}, fmt.Errorf("SetAIRoleMapping: %w", err)
	}
	return m.GetAIRole(ctx, agencyID, roleID)
}

func (m *accountManager) ResolveAgencyRole(ctx context.Context, agencyID, agencyRoleID string) (Resolution, error) {
	if agencyRoleID == "" {
		return Resolution{}, ErrRoleNotFound
	}
	q := m.st.Query(ctx, roleAIRole).Where("agency_id = ? AND agency_role_id = ?", agencyID, agencyRoleID).Order("created_at asc, id asc")
	found, err := specstore.List[AIRole](m.st, q, roleAIRole)
	if err != nil {
		return Resolution{}, fmt.Errorf("ResolveAgencyRole: %w", err)
	}
	if len(found) == 0 {
		return Resolution{}, ErrRoleNotFound
	}
	return m.ResolveAIRole(ctx, agencyID, found[0].ID)
}

func (m *accountManager) ResolveAIRole(ctx context.Context, agencyID, roleID string) (Resolution, error) {
	r, err := m.GetAIRole(ctx, agencyID, roleID)
	if err != nil {
		return Resolution{}, err
	}
	out := Resolution{AIRoleID: r.ID, AgencyRoleID: r.AgencyRoleID}
	if out.Instructions, err = m.Instructions(ctx, agencyID, roleID); err != nil {
		return Resolution{}, err
	}
	if out.Parts, err = m.instructionParts(ctx, agencyID, r); err != nil {
		return Resolution{}, err
	}
	a, source, err := m.accountOf(ctx, agencyID, r)
	if err != nil {
		return Resolution{}, err
	}
	if a == nil {
		out.Message = noAccountMapped
		return out, nil
	}
	out.Account, out.AccountSource = a, source
	out.Limits, err = m.limitsInForce(ctx, agencyID, r, *a)
	return out, err
}

func (m *accountManager) accountOf(ctx context.Context, agencyID string, r AIRole) (*Account, string, error) {
	if r.AccountID != "" {
		a, err := m.GetAccount(ctx, agencyID, r.AccountID)
		if err == nil {
			return &a, accountFromRole, nil
		}
		if !errors.Is(err, ErrAccountNotFound) {
			return nil, "", err
		}
	}
	a, err := m.GetDefaultAccount(ctx, agencyID)
	if errors.Is(err, ErrNoDefaultAccount) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	return &a, accountFromDefault, nil
}

func (m *accountManager) limitsInForce(ctx context.Context, agencyID string, r AIRole, a Account) ([]LimitInForce, error) {
	rl, al := r.Limits(), accountLimits(a)
	out := []LimitInForce{{Limit: "tokens per call", Field: "tokens_per_call", Max: lowerNonZero(models.Count(al.TokensPerCall), models.Count(rl.TokensPerCall)), Source: limitSource(models.Count(al.TokensPerCall), models.Count(rl.TokensPerCall))}}
	for _, ls := range limitSpecs {
		req, tok, err := m.sum(ctx, agencyID, a.ID, r.ID, ls.window)
		if err != nil {
			return nil, err
		}
		used := req
		if ls.tokens {
			used = tok
		}
		out = append(out, LimitInForce{Limit: ls.name, Field: ls.field, Max: lowerNonZero(models.Count(ls.max(al)), models.Count(ls.max(rl))), Used: used, Source: limitSource(models.Count(ls.max(al)), models.Count(ls.max(rl)))})
	}
	return out, nil
}

func limitSource(account, role int) string {
	switch {
	case account == 0 && role == 0:
		return "none"
	case account != 0 && (role == 0 || account < role):
		return "account"
	}
	return "role"
}
