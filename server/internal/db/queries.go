package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/piotr-halas/decodo-workflow/internal/models"
)

func (db *DB) CreateProject(p *models.Project) error {
	p.ID = uuid.New().String()
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.BaseBranch == "" {
		p.BaseBranch = "main"
	}
	if p.Runner == "" {
		p.Runner = "qwen"
	}

	_, err := db.conn.Exec(`
		INSERT INTO projects (id, name, repo_path, base_branch, auto_test, auto_review, runner, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ID, p.Name, p.RepoPath, p.BaseBranch, p.AutoTest, p.AutoReview, p.Runner, p.CreatedAt, p.UpdatedAt)
	return err
}

func (db *DB) GetProjects() ([]models.Project, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, repo_path, base_branch, auto_test, auto_review, runner, created_at, updated_at
		FROM projects ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.RepoPath, &p.BaseBranch, &p.AutoTest, &p.AutoReview, &p.Runner, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (db *DB) GetProject(id string) (*models.Project, error) {
	var p models.Project
	err := db.conn.QueryRow(`
		SELECT id, name, repo_path, base_branch, auto_test, auto_review, runner, created_at, updated_at
		FROM projects WHERE id = ?
	`, id).Scan(&p.ID, &p.Name, &p.RepoPath, &p.BaseBranch, &p.AutoTest, &p.AutoReview, &p.Runner, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (db *DB) UpdateProject(p *models.Project) error {
	p.UpdatedAt = time.Now()
	if p.Runner == "" {
		p.Runner = "qwen"
	}
	_, err := db.conn.Exec(`
		UPDATE projects SET name = ?, repo_path = ?, base_branch = ?, auto_test = ?, auto_review = ?, runner = ?, updated_at = ? WHERE id = ?
	`, p.Name, p.RepoPath, p.BaseBranch, p.AutoTest, p.AutoReview, p.Runner, p.UpdatedAt, p.ID)
	return err
}

func (db *DB) DeleteProject(id string) (bool, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM projects WHERE id = ?`, id).Scan(&count); err != nil {
		return false, err
	}
	if count == 0 {
		return false, nil
	}

	statements := []string{
		`DELETE FROM task_logs WHERE task_id IN (SELECT t.id FROM tasks t JOIN workflows w ON t.workflow_id = w.id WHERE w.project_id = ?)`,
		`DELETE FROM chat_messages WHERE workflow_id IN (SELECT id FROM workflows WHERE project_id = ?)`,
		`DELETE FROM tasks WHERE workflow_id IN (SELECT id FROM workflows WHERE project_id = ?)`,
		`DELETE FROM workflows WHERE project_id = ?`,
		`DELETE FROM tickets WHERE project_id = ?`,
		`DELETE FROM projects WHERE id = ?`,
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt, id); err != nil {
			return false, err
		}
	}

	return true, tx.Commit()
}

func (db *DB) UpsertTicket(t *models.Ticket) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	now := time.Now()

	_, err := db.conn.Exec(`
		INSERT INTO tickets (id, project_id, jira_key, summary, description, status, priority, assignee, labels, acceptance_criteria, ticket_type, project_key, source, raw_json, jira_updated_at, synced_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(jira_key) DO UPDATE SET
			summary = excluded.summary,
			description = excluded.description,
			status = excluded.status,
			priority = excluded.priority,
			assignee = excluded.assignee,
			labels = excluded.labels,
			acceptance_criteria = excluded.acceptance_criteria,
			ticket_type = excluded.ticket_type,
			project_key = excluded.project_key,
			raw_json = excluded.raw_json,
			jira_updated_at = excluded.jira_updated_at,
			synced_at = excluded.synced_at,
			updated_at = excluded.updated_at
	`,
		t.ID, t.ProjectID, t.JiraKey, t.Summary, t.Description, t.Status,
		t.Priority, t.Assignee, t.Labels, t.AcceptanceCriteria,
		t.TicketType, t.ProjectKey, t.Source, t.RawJSON,
		t.JiraUpdatedAt, now, now, now,
	)
	return err
}

func (db *DB) CreateTicket(t *models.Ticket) error {
	t.ID = uuid.New().String()
	if t.Source == "" {
		t.Source = "manual"
	}
	if t.JiraKey == "" {
		t.JiraKey = "LOCAL-" + t.ID[:8]
	}
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	t.JiraUpdatedAt = now
	t.SyncedAt = now

	_, err := db.conn.Exec(`
		INSERT INTO tickets (id, project_id, jira_key, summary, description, status, priority, assignee, labels, acceptance_criteria, ticket_type, project_key, source, raw_json, jira_updated_at, synced_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		t.ID, t.ProjectID, t.JiraKey, t.Summary, t.Description, t.Status,
		t.Priority, t.Assignee, t.Labels, t.AcceptanceCriteria,
		t.TicketType, t.ProjectKey, t.Source, t.RawJSON,
		t.JiraUpdatedAt, t.SyncedAt, t.CreatedAt, t.UpdatedAt,
	)
	return err
}

func (db *DB) UpdateTicketStatus(id string, status string) error {
	_, err := db.conn.Exec(`
		UPDATE tickets SET status = ?, updated_at = ? WHERE id = ?
	`, status, time.Now(), id)
	return err
}

func (db *DB) GetTicketsByProject(projectID string) ([]models.Ticket, error) {
	rows, err := db.conn.Query(`
		SELECT id, project_id, jira_key, summary, description, status, priority, assignee, labels,
		       acceptance_criteria, ticket_type, project_key, source, raw_json, jira_updated_at,
		       synced_at, created_at, updated_at
		FROM tickets
		WHERE project_id = ? AND status != 'done'
		ORDER BY updated_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []models.Ticket
	for rows.Next() {
		var t models.Ticket
		if err := rows.Scan(
			&t.ID, &t.ProjectID, &t.JiraKey, &t.Summary, &t.Description, &t.Status,
			&t.Priority, &t.Assignee, &t.Labels, &t.AcceptanceCriteria,
			&t.TicketType, &t.ProjectKey, &t.Source, &t.RawJSON, &t.JiraUpdatedAt,
			&t.SyncedAt, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, rows.Err()
}

func (db *DB) GetTickets() ([]models.Ticket, error) {
	rows, err := db.conn.Query(`
		SELECT id, project_id, jira_key, summary, description, status, priority, assignee, labels,
		       acceptance_criteria, ticket_type, project_key, source, raw_json, jira_updated_at,
		       synced_at, created_at, updated_at
		FROM tickets
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []models.Ticket
	for rows.Next() {
		var t models.Ticket
		if err := rows.Scan(
			&t.ID, &t.ProjectID, &t.JiraKey, &t.Summary, &t.Description, &t.Status,
			&t.Priority, &t.Assignee, &t.Labels, &t.AcceptanceCriteria,
			&t.TicketType, &t.ProjectKey, &t.Source, &t.RawJSON, &t.JiraUpdatedAt,
			&t.SyncedAt, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, rows.Err()
}

func (db *DB) GetTicketByKey(jiraKey string) (*models.Ticket, error) {
	var t models.Ticket
	err := db.conn.QueryRow(`
		SELECT id, project_id, jira_key, summary, description, status, priority, assignee, labels,
		       acceptance_criteria, ticket_type, project_key, source, raw_json, jira_updated_at,
		       synced_at, created_at, updated_at
		FROM tickets
		WHERE jira_key = ?
	`, jiraKey).Scan(
		&t.ID, &t.ProjectID, &t.JiraKey, &t.Summary, &t.Description, &t.Status,
		&t.Priority, &t.Assignee, &t.Labels, &t.AcceptanceCriteria,
		&t.TicketType, &t.ProjectKey, &t.Source, &t.RawJSON, &t.JiraUpdatedAt,
		&t.SyncedAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (db *DB) GetTicketByID(id string) (*models.Ticket, error) {
	var t models.Ticket
	err := db.conn.QueryRow(`
		SELECT id, project_id, jira_key, summary, description, status, priority, assignee, labels,
		       acceptance_criteria, ticket_type, project_key, source, raw_json, jira_updated_at,
		       synced_at, created_at, updated_at
		FROM tickets
		WHERE id = ?
	`, id).Scan(
		&t.ID, &t.ProjectID, &t.JiraKey, &t.Summary, &t.Description, &t.Status,
		&t.Priority, &t.Assignee, &t.Labels, &t.AcceptanceCriteria,
		&t.TicketType, &t.ProjectKey, &t.Source, &t.RawJSON, &t.JiraUpdatedAt,
		&t.SyncedAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (db *DB) UpdateTicket(t *models.Ticket) error {
	t.UpdatedAt = time.Now()
	_, err := db.conn.Exec(`
		UPDATE tickets SET summary = ?, description = ?, acceptance_criteria = ?, priority = ?, labels = ?, updated_at = ? WHERE id = ?
	`, t.Summary, t.Description, t.AcceptanceCriteria, t.Priority, t.Labels, t.UpdatedAt, t.ID)
	return err
}

func (db *DB) GetWorkflowsByTicket(ticketID string) ([]models.Workflow, error) {
	rows, err := db.conn.Query(`
		SELECT w.id, w.project_id, w.ticket_id, w.status, w.branch_name, w.spec, w.retry_count, w.error, w.created_at, w.updated_at,
		       COALESCE(t.summary, '') AS ticket_summary, COALESCE(t.jira_key, '') AS ticket_jira_key
		FROM workflows w
		LEFT JOIN tickets t ON w.ticket_id = t.id
		WHERE w.ticket_id = ? ORDER BY w.created_at DESC
	`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workflows []models.Workflow
	for rows.Next() {
		var w models.Workflow
		if err := rows.Scan(&w.ID, &w.ProjectID, &w.TicketID, &w.Status, &w.BranchName, &w.Spec, &w.RetryCount, &w.Error, &w.CreatedAt, &w.UpdatedAt, &w.TicketSummary, &w.TicketJiraKey); err != nil {
			return nil, err
		}
		workflows = append(workflows, w)
	}
	return workflows, rows.Err()
}

func (db *DB) CreateWorkflow(w *models.Workflow) error {
	w.ID = uuid.New().String()
	now := time.Now()
	w.CreatedAt = now
	w.UpdatedAt = now
	if w.Status == "" {
		w.Status = models.WorkflowCreated
	}

	_, err := db.conn.Exec(`
		INSERT INTO workflows (id, project_id, ticket_id, status, branch_name, spec, retry_count, error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, w.ID, w.ProjectID, w.TicketID, w.Status, w.BranchName, w.Spec, w.RetryCount, w.Error, w.CreatedAt, w.UpdatedAt)
	return err
}

func (db *DB) UpdateWorkflowStatus(id string, status models.WorkflowStatus, errMsg string) error {
	_, err := db.conn.Exec(`
		UPDATE workflows SET status = ?, error = ?, updated_at = ? WHERE id = ?
	`, status, errMsg, time.Now(), id)
	return err
}

func (db *DB) UpdateWorkflowSpec(id string, spec string) error {
	_, err := db.conn.Exec(`
		UPDATE workflows SET spec = ?, updated_at = ? WHERE id = ?
	`, spec, time.Now(), id)
	return err
}

func (db *DB) UpdateWorkflowBranch(id string, branch string) error {
	_, err := db.conn.Exec(`
		UPDATE workflows SET branch_name = ?, updated_at = ? WHERE id = ?
	`, branch, time.Now(), id)
	return err
}

func (db *DB) GetWorkflow(id string) (*models.Workflow, error) {
	var w models.Workflow
	err := db.conn.QueryRow(`
		SELECT id, project_id, ticket_id, status, branch_name, spec, retry_count, error, created_at, updated_at
		FROM workflows WHERE id = ?
	`, id).Scan(&w.ID, &w.ProjectID, &w.TicketID, &w.Status, &w.BranchName, &w.Spec, &w.RetryCount, &w.Error, &w.CreatedAt, &w.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (db *DB) GetWorkflowsByProject(projectID string) ([]models.Workflow, error) {
	rows, err := db.conn.Query(`
		SELECT w.id, w.project_id, w.ticket_id, w.status, w.branch_name, w.spec, w.retry_count, w.error, w.created_at, w.updated_at,
		       COALESCE(t.summary, '') AS ticket_summary, COALESCE(t.jira_key, '') AS ticket_jira_key
		FROM workflows w
		LEFT JOIN tickets t ON w.ticket_id = t.id
		WHERE w.project_id = ? AND w.status != 'DONE' ORDER BY w.created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workflows []models.Workflow
	for rows.Next() {
		var w models.Workflow
		if err := rows.Scan(&w.ID, &w.ProjectID, &w.TicketID, &w.Status, &w.BranchName, &w.Spec, &w.RetryCount, &w.Error, &w.CreatedAt, &w.UpdatedAt, &w.TicketSummary, &w.TicketJiraKey); err != nil {
			return nil, err
		}
		workflows = append(workflows, w)
	}
	return workflows, rows.Err()
}

func (db *DB) GetWorkflows() ([]models.Workflow, error) {
	rows, err := db.conn.Query(`
		SELECT id, project_id, ticket_id, status, branch_name, spec, retry_count, error, created_at, updated_at
		FROM workflows ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workflows []models.Workflow
	for rows.Next() {
		var w models.Workflow
		if err := rows.Scan(&w.ID, &w.ProjectID, &w.TicketID, &w.Status, &w.BranchName, &w.Spec, &w.RetryCount, &w.Error, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		workflows = append(workflows, w)
	}
	return workflows, rows.Err()
}

func (db *DB) CreateTask(t *models.Task) error {
	t.ID = uuid.New().String()
	now := time.Now()
	t.CreatedAt = now
	if t.Status == "" {
		t.Status = models.TaskQueued
	}

	_, err := db.conn.Exec(`
		INSERT INTO tasks (id, workflow_id, parent_task_id, type, status, agent, prompt, output, parsed, error, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.WorkflowID, t.ParentTaskID, t.Type, t.Status, t.Agent, t.Prompt, t.Output, t.Parsed, t.Error, t.CreatedAt)
	return err
}

func (db *DB) UpdateTaskStatus(id string, status models.TaskStatus) error {
	now := time.Now()
	switch status {
	case models.TaskRunning:
		_, err := db.conn.Exec(`UPDATE tasks SET status = ?, started_at = ?, error = '' WHERE id = ?`, status, now, id)
		return err
	case models.TaskCompleted, models.TaskFailed:
		_, err := db.conn.Exec(`UPDATE tasks SET status = ?, completed_at = ? WHERE id = ?`, status, now, id)
		return err
	default:
		_, err := db.conn.Exec(`UPDATE tasks SET status = ? WHERE id = ?`, status, id)
		return err
	}
}

func (db *DB) UpdateTaskOutput(id string, output string, parsed string) error {
	_, err := db.conn.Exec(`UPDATE tasks SET output = ?, parsed = ? WHERE id = ?`, output, parsed, id)
	return err
}

func (db *DB) UpdateTaskError(id string, errMsg string) error {
	_, err := db.conn.Exec(`UPDATE tasks SET error = ?, status = 'FAILED', completed_at = ? WHERE id = ?`, errMsg, time.Now(), id)
	return err
}

func (db *DB) GetQueuedTasks() ([]models.Task, error) {
	rows, err := db.conn.Query(`
		SELECT id, workflow_id, parent_task_id, type, status, agent, prompt, output, parsed, error, created_at, started_at, completed_at
		FROM tasks WHERE status = 'QUEUED' ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.ParentTaskID, &t.Type, &t.Status, &t.Agent, &t.Prompt, &t.Output, &t.Parsed, &t.Error, &t.CreatedAt, &t.StartedAt, &t.CompletedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (db *DB) GetTasksByWorkflow(workflowID string) ([]models.Task, error) {
	rows, err := db.conn.Query(`
		SELECT id, workflow_id, parent_task_id, type, status, agent, prompt, output, parsed, error, created_at, started_at, completed_at
		FROM tasks WHERE workflow_id = ? ORDER BY created_at ASC
	`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.ParentTaskID, &t.Type, &t.Status, &t.Agent, &t.Prompt, &t.Output, &t.Parsed, &t.Error, &t.CreatedAt, &t.StartedAt, &t.CompletedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (db *DB) GetSubtasks(parentTaskID string) ([]models.Task, error) {
	rows, err := db.conn.Query(`
		SELECT id, workflow_id, parent_task_id, type, status, agent, prompt, output, parsed, error, created_at, started_at, completed_at
		FROM tasks WHERE parent_task_id = ? ORDER BY created_at ASC
	`, parentTaskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.ParentTaskID, &t.Type, &t.Status, &t.Agent, &t.Prompt, &t.Output, &t.Parsed, &t.Error, &t.CreatedAt, &t.StartedAt, &t.CompletedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (db *DB) AllSubtasksCompleted(parentTaskID string) (bool, error) {
	var total, completed int
	err := db.conn.QueryRow(`SELECT COUNT(*), COUNT(CASE WHEN status = 'COMPLETED' THEN 1 END) FROM tasks WHERE parent_task_id = ?`, parentTaskID).Scan(&total, &completed)
	if err != nil {
		return false, err
	}
	return total > 0 && total == completed, nil
}

func (db *DB) AnySubtaskFailed(parentTaskID string) (bool, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE parent_task_id = ? AND status = 'FAILED'`, parentTaskID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (db *DB) CreateChatMessage(msg *models.ChatMessage) error {
	msg.ID = uuid.New().String()
	now := time.Now()
	msg.CreatedAt = now

	_, err := db.conn.Exec(`
		INSERT INTO chat_messages (id, workflow_id, role, agent_type, content, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, msg.ID, msg.WorkflowID, msg.Role, msg.AgentType, msg.Content, msg.Metadata, msg.CreatedAt)
	return err
}

func (db *DB) GetChatMessages(workflowID string) ([]models.ChatMessage, error) {
	rows, err := db.conn.Query(`
		SELECT id, workflow_id, role, agent_type, content, metadata, created_at
		FROM chat_messages WHERE workflow_id = ? ORDER BY created_at ASC
	`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.ChatMessage
	for rows.Next() {
		var m models.ChatMessage
		if err := rows.Scan(&m.ID, &m.WorkflowID, &m.Role, &m.AgentType, &m.Content, &m.Metadata, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (db *DB) IncrementWorkflowRetry(id string) error {
	_, err := db.conn.Exec(`UPDATE workflows SET retry_count = retry_count + 1, updated_at = ? WHERE id = ?`, time.Now(), id)
	return err
}

func (db *DB) CreateTaskLog(log *models.TaskLog) error {
	log.ID = uuid.New().String()
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}
	if log.LogLevel == "" {
		log.LogLevel = models.LogLevelInfo
	}

	_, err := db.conn.Exec(`
		INSERT INTO task_logs (id, task_id, timestamp, log_level, message)
		VALUES (?, ?, ?, ?, ?)
	`, log.ID, log.TaskID, log.Timestamp, log.LogLevel, log.Message)
	return err
}

func (db *DB) GetTaskLogs(taskID string) ([]models.TaskLog, error) {
	rows, err := db.conn.Query(`
		SELECT id, task_id, timestamp, log_level, message
		FROM task_logs WHERE task_id = ? ORDER BY timestamp ASC
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.TaskLog
	for rows.Next() {
		var l models.TaskLog
		if err := rows.Scan(&l.ID, &l.TaskID, &l.Timestamp, &l.LogLevel, &l.Message); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func (db *DB) GetTaskLogsAfter(taskID string, afterTimestamp time.Time) ([]models.TaskLog, error) {
	rows, err := db.conn.Query(`
		SELECT id, task_id, timestamp, log_level, message
		FROM task_logs WHERE task_id = ? AND timestamp > ? ORDER BY timestamp ASC
	`, taskID, afterTimestamp)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.TaskLog
	for rows.Next() {
		var l models.TaskLog
		if err := rows.Scan(&l.ID, &l.TaskID, &l.Timestamp, &l.LogLevel, &l.Message); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func (db *DB) ResetWorkflow(id string) error {
	_, err := db.conn.Exec(`
		UPDATE workflows SET status = 'CREATED', branch_name = '', retry_count = 0, error = '', updated_at = ? WHERE id = ?
	`, time.Now(), id)
	return err
}

func (db *DB) DeleteTasksByWorkflow(workflowID string) error {
	_, err := db.conn.Exec(`DELETE FROM task_logs WHERE task_id IN (SELECT id FROM tasks WHERE workflow_id = ?)`, workflowID)
	if err != nil {
		return err
	}
	_, err = db.conn.Exec(`DELETE FROM tasks WHERE workflow_id = ?`, workflowID)
	return err
}

func (db *DB) RetryTask(id string) error {
	_, err := db.conn.Exec(`
		UPDATE tasks 
		SET status = 'QUEUED', error = '', started_at = NULL, completed_at = NULL 
		WHERE id = ?
	`, id)
	return err
}

func (db *DB) GetTask(id string) (*models.Task, error) {
	var t models.Task
	err := db.conn.QueryRow(`
		SELECT id, workflow_id, parent_task_id, type, status, agent, prompt, output, parsed, error, created_at, started_at, completed_at
		FROM tasks WHERE id = ?
	`, id).Scan(&t.ID, &t.WorkflowID, &t.ParentTaskID, &t.Type, &t.Status, &t.Agent, &t.Prompt, &t.Output, &t.Parsed, &t.Error, &t.CreatedAt, &t.StartedAt, &t.CompletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}
