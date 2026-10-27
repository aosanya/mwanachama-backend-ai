package ai

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-ai/models"
)

type (
	AIRole             = models.AIRole
	AIRoleInput        = models.AIRoleInput
	AIRoleRevision     = models.AIRoleRevision
	AIRoleMappingInput = models.AIRoleMappingInput
	Resolution         = models.Resolution
	LimitInForce       = models.LimitInForce
	InstructionPart    = models.InstructionPart
)

type RoleManager interface {
	CreateAIRole(ctx context.Context, agencyID string, in AIRoleInput) (AIRole, error)
	ListAIRoles(ctx context.Context, agencyID string) ([]AIRole, error)
	GetAIRole(ctx context.Context, agencyID, roleID string) (AIRole, error)
	UpdateAIRole(ctx context.Context, agencyID, roleID, savedBy string, in AIRoleInput) (AIRole, error)
	DeleteAIRole(ctx context.Context, agencyID, roleID string) (AIRole, error)
	ListAIRoleRevisions(ctx context.Context, agencyID, roleID string) ([]AIRoleRevision, error)
	RestoreAIRoleRevision(ctx context.Context, agencyID, roleID, revisionID, savedBy string) (AIRole, error)
	Instructions(ctx context.Context, agencyID, roleID string) (string, error)
	SetAIRoleMapping(ctx context.Context, agencyID, roleID string, in AIRoleMappingInput) (AIRole, error)
	ResolveAIRole(ctx context.Context, agencyID, roleID string) (Resolution, error)
	ResolveAgencyRole(ctx context.Context, agencyID, agencyRoleID string) (Resolution, error)
}

type InstructionSource interface {
	AgencyInstructions(ctx context.Context, agencyID, roleID string) (string, error)
}

type PartsSource interface {
	AgencyInstructionParts(ctx context.Context, agencyID, roleID string) ([]InstructionPart, error)
}

func WithInstructionSource(src InstructionSource) Option {
	return func(m *accountManager) { m.source = src }
}

func (m *accountManager) validRole(r AIRole) error {
	if err := m.checks(roleAIRole, r, ErrInvalidRole); err != nil {
		return err
	}
	if utf8.RuneCountInString(r.FineTuning) > models.MaxFineTuningChars {
		return fmt.Errorf("%w: the fine-tuning is longer than %d characters", ErrInvalidRole, models.MaxFineTuningChars)
	}
	if !r.UseAgencyInstructions && r.FineTuning == "" {
		return fmt.Errorf("%w: the fine-tuning is required when the agency's instructions are switched off", ErrInvalidRole)
	}
	for _, n := range []int{r.TokensPerCall, r.TokensPerDay, r.TokensPerMonth, r.RequestsPerDay, r.RequestsPerMinute} {
		if n < 0 {
			return fmt.Errorf("%w: a limit cannot be negative", ErrInvalidRole)
		}
	}
	return nil
}

func (m *accountManager) roleNameTaken(ctx context.Context, agencyID, name, except string) (bool, error) {
	var n int64
	q := m.st.Query(ctx, roleAIRole).Where("agency_id = ? AND name = ?", agencyID, name)
	if except != "" {
		q = q.Where("id <> ?", except)
	}
	err := q.Count(&n).Error
	return n > 0, err
}

func applyRoleInput(r *AIRole, in AIRoleInput) {
	r.Name, r.Description, r.FineTuning, r.Status = in.Name, in.Description, in.FineTuning, in.Status
	if in.UseAgencyInstructions != nil {
		r.UseAgencyInstructions = *in.UseAgencyInstructions
	}
	r.TokensPerCall, r.TokensPerDay, r.TokensPerMonth = in.TokensPerCall, in.TokensPerDay, in.TokensPerMonth
	r.RequestsPerDay, r.RequestsPerMinute = in.RequestsPerDay, in.RequestsPerMinute
}

func (m *accountManager) CreateAIRole(ctx context.Context, agencyID string, in AIRoleInput) (AIRole, error) {
	r := AIRole{AgencyID: agencyID, UseAgencyInstructions: true}
	applyRoleInput(&r, in)
	if err := m.validRole(r); err != nil {
		return AIRole{}, err
	}
	if taken, err := m.roleNameTaken(ctx, agencyID, r.Name, ""); err != nil {
		return AIRole{}, fmt.Errorf("CreateAIRole: %w", err)
	} else if taken {
		return AIRole{}, ErrDuplicateRoleName
	}
	r.ID = newID()
	r.CreatedAt = m.now()
	r.UpdatedAt = r.CreatedAt
	if err := m.st.Insert(ctx, roleAIRole, r); err != nil {
		return AIRole{}, fmt.Errorf("CreateAIRole: %w", err)
	}
	return r, nil
}

func (m *accountManager) ListAIRoles(ctx context.Context, agencyID string) ([]AIRole, error) {
	q := m.st.Query(ctx, roleAIRole).Where("agency_id = ?", agencyID).Order("created_at asc")
	out, err := specstore.List[AIRole](m.st, q, roleAIRole)
	if err != nil {
		return nil, fmt.Errorf("ListAIRoles: %w", err)
	}
	return out, nil
}

func (m *accountManager) GetAIRole(ctx context.Context, agencyID, roleID string) (AIRole, error) {
	var out AIRole
	q := m.st.Query(ctx, roleAIRole).Where("agency_id = ? AND id = ?", agencyID, roleID)
	if err := m.st.Take(q, roleAIRole, &out, ErrRoleNotFound); err != nil {
		return AIRole{}, err
	}
	return out, nil
}

func (m *accountManager) DeleteAIRole(ctx context.Context, agencyID, roleID string) (AIRole, error) {
	held, err := m.GetAIRole(ctx, agencyID, roleID)
	if err != nil {
		return AIRole{}, err
	}
	for _, role := range []string{roleRevision} {
		if err := m.st.Query(ctx, role).Where("agency_id = ? AND role_id = ?", agencyID, roleID).Delete(&struct{}{}).Error; err != nil {
			return AIRole{}, fmt.Errorf("DeleteAIRole: %w", err)
		}
	}
	if err := m.st.Query(ctx, roleAIRole).Where("id = ?", roleID).Delete(&struct{}{}).Error; err != nil {
		return AIRole{}, fmt.Errorf("DeleteAIRole: %w", err)
	}
	return held, nil
}

func (m *accountManager) roleLimitsFor(ctx context.Context, agencyID string, in CheckInput) (Limits, error) {
	if in.RoleID == "" {
		return in.RoleLimits, nil
	}
	r, err := m.GetAIRole(ctx, agencyID, in.RoleID)
	if err != nil {
		if err == ErrRoleNotFound {
			return in.RoleLimits, nil
		}
		return Limits{}, err
	}
	return r.Limits(), nil
}
