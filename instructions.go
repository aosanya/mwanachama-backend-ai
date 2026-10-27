package ai

import (
	"context"
	"fmt"
	"strings"
)

func (m *accountManager) Instructions(ctx context.Context, agencyID, roleID string) (string, error) {
	r, err := m.GetAIRole(ctx, agencyID, roleID)
	if err != nil {
		return "", err
	}
	if !r.UseAgencyInstructions || m.source == nil {
		return r.FineTuning, nil
	}
	base, err := m.source.AgencyInstructions(ctx, agencyID, roleID)
	if err != nil {
		return "", fmt.Errorf("Instructions: %w", err)
	}
	return strings.TrimRight(base, "\n") + joiner(base, r.FineTuning) + r.FineTuning, nil
}

func joiner(base, tuning string) string {
	if base == "" || tuning == "" {
		return ""
	}
	return "\n\n"
}

const (
	PartAgency = "agency"
	PartTuning = "fine_tuning"
)

func (m *accountManager) instructionParts(ctx context.Context, agencyID string, r AIRole) ([]InstructionPart, error) {
	parts := []InstructionPart{}
	if r.UseAgencyInstructions && m.source != nil {
		agency, err := m.agencyParts(ctx, agencyID, r.ID)
		if err != nil {
			return nil, fmt.Errorf("Instructions: %w", err)
		}
		parts = append(parts, agency...)
	}
	if r.FineTuning != "" {
		parts = append(parts, InstructionPart{Source: PartTuning, Text: r.FineTuning})
	}
	return parts, nil
}

func (m *accountManager) agencyParts(ctx context.Context, agencyID, roleID string) ([]InstructionPart, error) {
	if ps, ok := m.source.(PartsSource); ok {
		got, err := ps.AgencyInstructionParts(ctx, agencyID, roleID)
		if err != nil {
			return nil, err
		}
		kept := []InstructionPart{}
		for _, p := range got {
			if p.Text != "" {
				kept = append(kept, p)
			}
		}
		if len(kept) > 0 {
			return kept, nil
		}
	}
	base, err := m.source.AgencyInstructions(ctx, agencyID, roleID)
	if err != nil || base == "" {
		return nil, err
	}
	return []InstructionPart{{Source: PartAgency, Text: strings.TrimRight(base, "\n")}}, nil
}
