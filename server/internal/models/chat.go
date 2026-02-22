package models

import "time"

type ChatMessage struct {
	ID         string    `json:"id"`
	WorkflowID string    `json:"workflow_id"`
	Role       string    `json:"role"`
	AgentType  string    `json:"agent_type"`
	Content    string    `json:"content"`
	Metadata   string    `json:"metadata"`
	CreatedAt  time.Time `json:"created_at"`
}
