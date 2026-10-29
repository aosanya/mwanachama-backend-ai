package models

const (
	ModelFromProvider = "provider"
	ModelFromStarting = "starting"
)

type ModelEntry struct {
	ID        string `json:"id"`
	AgencyID  string `json:"agency_id"`
	Provider  string `json:"provider"`
	ModelName string `json:"model_name"`
	Source    string `json:"source"`
	ListedAt  string `json:"listed_at"`
}
