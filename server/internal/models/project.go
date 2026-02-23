package models

import "time"

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	RepoPath    string    `json:"repo_path"`
	BaseBranch  string    `json:"base_branch"`
	AutoTest    bool      `json:"auto_test"`
	AutoReview  bool      `json:"auto_review"`
	Runner      string    `json:"runner"`
	ModelHigh   string    `json:"model_high"`
	ModelMedium string    `json:"model_medium"`
	ModelLow    string    `json:"model_low"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (p *Project) ModelForPriority(priority string) string {
	switch priority {
	case "high":
		return p.ModelHigh
	case "medium":
		return p.ModelMedium
	case "low":
		return p.ModelLow
	default:
		return p.ModelHigh
	}
}
