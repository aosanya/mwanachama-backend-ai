package ai

import (
	"context"
	"errors"
	"fmt"

	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

const noAccountMapped = "no account is mapped"

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
	if r.AccountID == "" {
		out.Message = noAccountMapped
		return out, nil
	}
	a, err := m.GetAccount(ctx, agencyID, r.AccountID)
	if errors.Is(err, ErrAccountNotFound) {
		out.Message = noAccountMapped
		return out, nil
	}
	if err != nil {
		return Resolution{}, err
	}
	out.Account = &a
	out.Limits, err = m.limitsInForce(ctx, agencyID, r, a)
	return out, err
}

func (m *accountManager) limitsInForce(ctx context.Context, agencyID string, r AIRole, a Account) ([]LimitInForce, error) {
	rl, al := r.Limits(), accountLimits(a)
	out := []LimitInForce{{Limit: "tokens per call", Max: lowerNonZero(al.TokensPerCall, rl.TokensPerCall)}}
	for _, ls := range limitSpecs {
		req, tok, err := m.sum(ctx, agencyID, a.ID, r.ID, ls.window)
		if err != nil {
			return nil, err
		}
		used := req
		if ls.tokens {
			used = tok
		}
		out = append(out, LimitInForce{Limit: ls.name, Max: lowerNonZero(ls.max(al), ls.max(rl)), Used: used})
	}
	return out, nil
}
