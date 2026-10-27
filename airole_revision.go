package ai

import (
	"context"
	"fmt"

	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

func tunedFieldsDiffer(a, b AIRole) bool {
	return a.FineTuning != b.FineTuning || a.UseAgencyInstructions != b.UseAgencyInstructions || a.Limits() != b.Limits()
}

func (m *accountManager) saveRole(ctx context.Context, before, after AIRole, savedBy string) error {
	after.UpdatedAt = m.now()
	if tunedFieldsDiffer(before, after) {
		rev := AIRoleRevision{
			ID: newID(), AgencyID: before.AgencyID, RoleID: before.ID,
			FineTuning: before.FineTuning, UseAgencyInstructions: before.UseAgencyInstructions,
			TokensPerCall: before.TokensPerCall, TokensPerDay: before.TokensPerDay, TokensPerMonth: before.TokensPerMonth,
			RequestsPerDay: before.RequestsPerDay, RequestsPerMinute: before.RequestsPerMinute,
			SavedBy: savedBy, SavedAt: after.UpdatedAt,
		}
		if err := m.st.Insert(ctx, roleRevision, rev); err != nil {
			return err
		}
	}
	return m.update(ctx, roleAIRole, after.ID, after)
}

func (m *accountManager) UpdateAIRole(ctx context.Context, agencyID, roleID, savedBy string, in AIRoleInput) (AIRole, error) {
	held, err := m.GetAIRole(ctx, agencyID, roleID)
	if err != nil {
		return AIRole{}, err
	}
	next := held
	applyRoleInput(&next, in)
	if err := m.validRole(next); err != nil {
		return AIRole{}, err
	}
	if taken, err := m.roleNameTaken(ctx, agencyID, next.Name, next.ID); err != nil {
		return AIRole{}, fmt.Errorf("UpdateAIRole: %w", err)
	} else if taken {
		return AIRole{}, ErrDuplicateRoleName
	}
	if err := m.saveRole(ctx, held, next, savedBy); err != nil {
		return AIRole{}, fmt.Errorf("UpdateAIRole: %w", err)
	}
	return m.GetAIRole(ctx, agencyID, roleID)
}

func (m *accountManager) ListAIRoleRevisions(ctx context.Context, agencyID, roleID string) ([]AIRoleRevision, error) {
	if _, err := m.GetAIRole(ctx, agencyID, roleID); err != nil {
		return nil, err
	}
	q := m.st.Query(ctx, roleRevision).Where("agency_id = ? AND role_id = ?", agencyID, roleID).Order("saved_at desc, id desc")
	out, err := specstore.List[AIRoleRevision](m.st, q, roleRevision)
	if err != nil {
		return nil, fmt.Errorf("ListAIRoleRevisions: %w", err)
	}
	return out, nil
}

func (m *accountManager) RestoreAIRoleRevision(ctx context.Context, agencyID, roleID, revisionID, savedBy string) (AIRole, error) {
	held, err := m.GetAIRole(ctx, agencyID, roleID)
	if err != nil {
		return AIRole{}, err
	}
	var rev AIRoleRevision
	q := m.st.Query(ctx, roleRevision).Where("agency_id = ? AND role_id = ? AND id = ?", agencyID, roleID, revisionID)
	if err := m.st.Take(q, roleRevision, &rev, ErrRevisionNotFound); err != nil {
		return AIRole{}, err
	}
	next := held
	next.FineTuning, next.UseAgencyInstructions = rev.FineTuning, rev.UseAgencyInstructions
	next.TokensPerCall, next.TokensPerDay, next.TokensPerMonth = rev.TokensPerCall, rev.TokensPerDay, rev.TokensPerMonth
	next.RequestsPerDay, next.RequestsPerMinute = rev.RequestsPerDay, rev.RequestsPerMinute
	if err := m.validRole(next); err != nil {
		return AIRole{}, err
	}
	if err := m.saveRole(ctx, held, next, savedBy); err != nil {
		return AIRole{}, fmt.Errorf("RestoreAIRoleRevision: %w", err)
	}
	return m.GetAIRole(ctx, agencyID, roleID)
}
