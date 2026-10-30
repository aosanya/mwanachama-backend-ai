package models

type ChatModel struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Picked bool   `json:"picked"`
}

type ChatPick struct {
	ID        string `json:"id"`
	AgencyID  string `json:"agency_id"`
	PersonID  string `json:"person_id"`
	AccountID string `json:"account_id"`
	UpdatedAt string `json:"updated_at"`
}

type ChatPickInput struct {
	AccountID string `json:"account_id"`
}
