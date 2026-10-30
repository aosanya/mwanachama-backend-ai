package ai

import (
	"context"
	"errors"
	"fmt"

	"github.com/aosanya/mwanachama-backend-ai/models"
)

const roleChatPick = "chat_pick"

var errNoPick = errors.New("no pick")

type (
	ChatModel     = models.ChatModel
	ChatPick      = models.ChatPick
	ChatPickInput = models.ChatPickInput
)

type ChatModelManager interface {
	ListChatModels(ctx context.Context, agencyID, personID string) ([]ChatModel, error)
	PickChatModel(ctx context.Context, agencyID, personID string, in ChatPickInput) ([]ChatModel, error)
}

func chatLabel(a Account) string {
	if a.ChatLabel != "" {
		return a.ChatLabel
	}
	return a.ModelName
}

func chatUsable(a Account) bool {
	return a.ChatEnabled && a.Status == models.StatusEnabled
}

func chatPickID(agencyID, personID string) string { return agencyID + "|" + personID }

func (m *accountManager) ListChatModels(ctx context.Context, agencyID, personID string) ([]ChatModel, error) {
	accounts, err := m.ListAccounts(ctx, agencyID)
	if err != nil {
		return nil, fmt.Errorf("ListChatModels: %w", err)
	}
	pick, err := m.pickedAccount(ctx, agencyID, personID)
	if err != nil {
		return nil, fmt.Errorf("ListChatModels: %w", err)
	}
	items := []ChatModel{}
	for _, a := range accounts {
		if chatUsable(a) {
			items = append(items, ChatModel{ID: a.ID, Label: chatLabel(a)})
		}
	}
	chosen := 0
	for i, it := range items {
		if it.ID == pick {
			chosen = i
		}
	}
	if len(items) > 0 {
		items[chosen].Picked = true
	}
	return items, nil
}

func (m *accountManager) PickChatModel(ctx context.Context, agencyID, personID string, in ChatPickInput) ([]ChatModel, error) {
	if personID == "" {
		return nil, fmt.Errorf("%w: a pick belongs to a signed-in person", ErrInvalidAccount)
	}
	a, err := m.GetAccount(ctx, agencyID, in.AccountID)
	if err != nil {
		return nil, err
	}
	if !chatUsable(a) {
		return nil, ErrChatModelUnavailable
	}
	id := chatPickID(agencyID, personID)
	if err := m.st.Query(ctx, roleChatPick).Where("id = ?", id).Delete(&struct{}{}).Error; err != nil {
		return nil, fmt.Errorf("PickChatModel: %w", err)
	}
	row := ChatPick{ID: id, AgencyID: agencyID, PersonID: personID, AccountID: a.ID, UpdatedAt: m.now()}
	if err := m.st.Insert(ctx, roleChatPick, row); err != nil {
		return nil, fmt.Errorf("PickChatModel: %w", err)
	}
	return m.ListChatModels(ctx, agencyID, personID)
}

func (m *accountManager) pickedAccount(ctx context.Context, agencyID, personID string) (string, error) {
	if personID == "" {
		return "", nil
	}
	var row ChatPick
	q := m.st.Query(ctx, roleChatPick).Where("id = ?", chatPickID(agencyID, personID))
	switch err := m.st.Take(q, roleChatPick, &row, errNoPick); err {
	case nil:
		return row.AccountID, nil
	case errNoPick:
		return "", nil
	default:
		return "", err
	}
}

func (m *accountManager) forgetPicks(ctx context.Context, agencyID, accountID string) error {
	return m.st.Query(ctx, roleChatPick).
		Where("agency_id = ? AND account_id = ?", agencyID, accountID).
		Delete(&struct{}{}).Error
}
