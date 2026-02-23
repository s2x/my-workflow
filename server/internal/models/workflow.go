package models

import "time"

type WorkflowStatus string

const (
	WorkflowCreated          WorkflowStatus = "CREATED"
	WorkflowDescribing       WorkflowStatus = "DESCRIBING"
	WorkflowCoding           WorkflowStatus = "CODING"
	WorkflowTesting          WorkflowStatus = "TESTING"
	WorkflowReviewing        WorkflowStatus = "REVIEWING"
	WorkflowAwaitingApproval WorkflowStatus = "AWAITING_APPROVAL"
	WorkflowDeploying        WorkflowStatus = "DEPLOYING"
	WorkflowDone             WorkflowStatus = "DONE"
	WorkflowFailed           WorkflowStatus = "FAILED"
)

type Workflow struct {
	ID            string         `json:"id"`
	ProjectID     string         `json:"project_id"`
	TicketID      string         `json:"ticket_id"`
	Status        WorkflowStatus `json:"status"`
	BranchName    string         `json:"branch_name"`
	Spec          string         `json:"spec"`
	RetryCount    int            `json:"retry_count"`
	Error         string         `json:"error"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	TicketSummary string         `json:"ticket_summary"`
	TicketJiraKey string         `json:"ticket_jira_key"`
}

type TaskType string

const (
	TaskDescribe TaskType = "DESCRIBE"
	TaskCode     TaskType = "CODE"
	TaskTest     TaskType = "TEST"
	TaskReview   TaskType = "REVIEW"
	TaskDeploy   TaskType = "DEPLOY"
	TaskFix      TaskType = "FIX"
)

type TaskStatus string

const (
	TaskQueued    TaskStatus = "QUEUED"
	TaskRunning   TaskStatus = "RUNNING"
	TaskCompleted TaskStatus = "COMPLETED"
	TaskFailed    TaskStatus = "FAILED"
)

type Task struct {
	ID           string     `json:"id"`
	WorkflowID   string     `json:"workflow_id"`
	ParentTaskID string     `json:"parent_task_id"`
	Type         TaskType   `json:"type"`
	Status       TaskStatus `json:"status"`
	Agent        string     `json:"agent"`
	Prompt       string     `json:"prompt"`
	Output       string     `json:"output"`
	Parsed       string     `json:"parsed"`
	Error        string     `json:"error"`
	CreatedAt    time.Time  `json:"created_at"`
	StartedAt    *time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
}
