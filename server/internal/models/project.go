package models

import "time"

type Project struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	RepoPath   string    `json:"repo_path"`
	BaseBranch string    `json:"base_branch"`
	AutoTest   bool      `json:"auto_test"`
	AutoReview bool      `json:"auto_review"`
	Runner     string    `json:"runner"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
