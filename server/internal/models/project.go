package models

import "time"

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	RepoPath    string    `json:"repo_path"`
	BaseBranch  string    `json:"base_branch"`
	StageBranch string    `json:"stage_branch"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
