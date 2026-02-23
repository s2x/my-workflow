package models

import "time"

const (
	TicketStatusOpen = "Open"
	TicketStatusDone = "done"
)

type Ticket struct {
	ID                 string    `json:"id"`
	ProjectID          string    `json:"project_id"`
	JiraKey            string    `json:"jira_key"`
	Summary            string    `json:"summary"`
	Description        string    `json:"description"`
	Status             string    `json:"status"`
	Priority           string    `json:"priority"`
	Assignee           string    `json:"assignee"`
	Labels             string    `json:"labels"`
	AcceptanceCriteria string    `json:"acceptance_criteria"`
	TicketType         string    `json:"ticket_type"`
	ProjectKey         string    `json:"project_key"`
	Source             string    `json:"source"`
	RawJSON            string    `json:"raw_json"`
	JiraUpdatedAt      time.Time `json:"jira_updated_at"`
	SyncedAt           time.Time `json:"synced_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	AIGenerated        bool      `json:"ai_generated"`
	AIMetadata         string    `json:"ai_metadata"`
	RefinementCount    int       `json:"refinement_count"`
}

type GeneratedTicket struct {
	Title                string   `json:"title"`
	BusinessDescription  string   `json:"business_description"`
	TechnicalDescription string   `json:"technical_description"`
	Priority             string   `json:"priority"`
	Complexity           string   `json:"complexity"`
	AffectedComponents   []string `json:"affected_components"`
}
