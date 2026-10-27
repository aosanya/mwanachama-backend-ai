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
