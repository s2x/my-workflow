package db

import (
	"testing"
	"time"

	"github.com/piotr-halas/decodo-workflow/internal/models"
)

func TestCreateTaskLog(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := createTestTicket(t, db, project.ID)
	workflow := createTestWorkflow(t, db, project.ID, ticket.ID)
	task := createTestTask(t, db, workflow.ID)

	log := &models.TaskLog{
		TaskID:   task.ID,
		LogLevel: models.LogLevelInfo,
		Message:  "Test log message",
	}

	err := db.CreateTaskLog(log)
	if err != nil {
		t.Fatalf("CreateTaskLog failed: %v", err)
	}

	if log.ID == "" {
		t.Error("Expected ID to be generated")
	}

	if log.Timestamp.IsZero() {
		t.Error("Expected Timestamp to be set")
	}
}

func TestGetTaskLogs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := createTestTicket(t, db, project.ID)
	workflow := createTestWorkflow(t, db, project.ID, ticket.ID)
	task := createTestTask(t, db, workflow.ID)

	logs := []models.TaskLog{
		{TaskID: task.ID, LogLevel: models.LogLevelInfo, Message: "First log"},
		{TaskID: task.ID, LogLevel: models.LogLevelDebug, Message: "Second log"},
		{TaskID: task.ID, LogLevel: models.LogLevelError, Message: "Third log"},
	}

	for i := range logs {
		time.Sleep(10 * time.Millisecond)
		if err := db.CreateTaskLog(&logs[i]); err != nil {
			t.Fatalf("CreateTaskLog failed: %v", err)
		}
	}

	retrieved, err := db.GetTaskLogs(task.ID)
	if err != nil {
		t.Fatalf("GetTaskLogs failed: %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("Expected 3 logs, got %d", len(retrieved))
	}

	for i, log := range retrieved {
		if log.Message != logs[i].Message {
			t.Errorf("Log %d: expected message %q, got %q", i, logs[i].Message, log.Message)
		}
		if log.LogLevel != logs[i].LogLevel {
			t.Errorf("Log %d: expected level %q, got %q", i, logs[i].LogLevel, log.LogLevel)
		}
	}
}

func TestGetTaskLogsAfter(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := createTestTicket(t, db, project.ID)
	workflow := createTestWorkflow(t, db, project.ID, ticket.ID)
	task := createTestTask(t, db, workflow.ID)

	log1 := &models.TaskLog{TaskID: task.ID, LogLevel: models.LogLevelInfo, Message: "First log"}
	db.CreateTaskLog(log1)
	time.Sleep(100 * time.Millisecond)

	timestampBetween := time.Now()
	time.Sleep(100 * time.Millisecond)

	log2 := &models.TaskLog{TaskID: task.ID, LogLevel: models.LogLevelInfo, Message: "Second log"}
	db.CreateTaskLog(log2)

	log3 := &models.TaskLog{TaskID: task.ID, LogLevel: models.LogLevelInfo, Message: "Third log"}
	db.CreateTaskLog(log3)

	retrieved, err := db.GetTaskLogsAfter(task.ID, timestampBetween)
	if err != nil {
		t.Fatalf("GetTaskLogsAfter failed: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 logs after timestamp, got %d", len(retrieved))
	}

	if len(retrieved) > 0 && retrieved[0].Message != "Second log" {
		t.Errorf("Expected first log to be 'Second log', got %q", retrieved[0].Message)
	}
}

func setupTestDB(t *testing.T) *DB {
	db, err := New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	return db
}

func createTestProject(t *testing.T, db *DB) *models.Project {
	project := &models.Project{
		Name:     "test-project",
		RepoPath: "/tmp/test",
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}
	return project
}

func createTestTicket(t *testing.T, db *DB, projectID string) *models.Ticket {
	ticket := &models.Ticket{
		ProjectID: projectID,
		Summary:   "Test ticket",
	}
	if err := db.CreateTicket(ticket); err != nil {
		t.Fatalf("Failed to create test ticket: %v", err)
	}
	return ticket
}

func createTestWorkflow(t *testing.T, db *DB, projectID, ticketID string) *models.Workflow {
	workflow := &models.Workflow{
		ProjectID: projectID,
		TicketID:  ticketID,
	}
	if err := db.CreateWorkflow(workflow); err != nil {
		t.Fatalf("Failed to create test workflow: %v", err)
	}
	return workflow
}

func createTestTask(t *testing.T, db *DB, workflowID string) *models.Task {
	task := &models.Task{
		WorkflowID: workflowID,
		Type:       models.TaskCode,
		Agent:      "test-agent",
	}
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("Failed to create test task: %v", err)
	}
	return task
}

func TestCreateTaskLogWithEmptyMessage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := createTestTicket(t, db, project.ID)
	workflow := createTestWorkflow(t, db, project.ID, ticket.ID)
	task := createTestTask(t, db, workflow.ID)

	log := &models.TaskLog{
		TaskID:   task.ID,
		LogLevel: models.LogLevelInfo,
		Message:  "",
	}

	err := db.CreateTaskLog(log)
	if err != nil {
		t.Fatalf("CreateTaskLog should handle empty message: %v", err)
	}

	retrieved, err := db.GetTaskLogs(task.ID)
	if err != nil {
		t.Fatalf("GetTaskLogs failed: %v", err)
	}

	if len(retrieved) != 1 || retrieved[0].Message != "" {
		t.Error("Expected empty message to be stored")
	}
}

func TestGetTaskLogsReturnsEmptyForNonexistentTask(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logs, err := db.GetTaskLogs("nonexistent-task-id")
	if err != nil {
		t.Fatalf("GetTaskLogs should not error for nonexistent task: %v", err)
	}

	if len(logs) != 0 {
		t.Errorf("Expected 0 logs for nonexistent task, got %d", len(logs))
	}
}

func TestGetTaskLogsAfterReturnsEmptyWhenNone(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := createTestTicket(t, db, project.ID)
	workflow := createTestWorkflow(t, db, project.ID, ticket.ID)
	task := createTestTask(t, db, workflow.ID)

	log1 := &models.TaskLog{TaskID: task.ID, LogLevel: models.LogLevelInfo, Message: "Only log"}
	db.CreateTaskLog(log1)

	futureTime := time.Now().Add(1 * time.Hour)
	retrieved, err := db.GetTaskLogsAfter(task.ID, futureTime)
	if err != nil {
		t.Fatalf("GetTaskLogsAfter failed: %v", err)
	}

	if len(retrieved) != 0 {
		t.Errorf("Expected 0 logs after future time, got %d", len(retrieved))
	}
}

func TestTaskLogLevelTypes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := createTestTicket(t, db, project.ID)
	workflow := createTestWorkflow(t, db, project.ID, ticket.ID)
	task := createTestTask(t, db, workflow.ID)

	levels := []models.LogLevel{
		models.LogLevelInfo,
		models.LogLevelDebug,
		models.LogLevelError,
	}

	for _, level := range levels {
		log := &models.TaskLog{
			TaskID:   task.ID,
			LogLevel: level,
			Message:  "Test message for " + string(level),
		}
		if err := db.CreateTaskLog(log); err != nil {
			t.Fatalf("CreateTaskLog failed for level %s: %v", level, err)
		}
	}

	logs, err := db.GetTaskLogs(task.ID)
	if err != nil {
		t.Fatalf("GetTaskLogs failed: %v", err)
	}

	if len(logs) != len(levels) {
		t.Errorf("Expected %d logs, got %d", len(levels), len(logs))
	}

	for i, log := range logs {
		if log.LogLevel != levels[i] {
			t.Errorf("Log %d: expected level %s, got %s", i, levels[i], log.LogLevel)
		}
	}
}

func TestCreateProjectWithAutoTestAndAutoReview(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := &models.Project{
		Name:       "test-auto-project",
		RepoPath:   "/tmp/test",
		AutoTest:   true,
		AutoReview: false,
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	retrieved, err := db.GetProject(project.ID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}

	if retrieved.AutoTest != true {
		t.Errorf("Expected AutoTest=true, got %v", retrieved.AutoTest)
	}
	if retrieved.AutoReview != false {
		t.Errorf("Expected AutoReview=false, got %v", retrieved.AutoReview)
	}
}

func TestCreateProjectDefaultAutoFlags(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := &models.Project{
		Name:     "test-default-project",
		RepoPath: "/tmp/test",
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	retrieved, err := db.GetProject(project.ID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}

	if retrieved.AutoTest != false {
		t.Errorf("Expected AutoTest default=false, got %v", retrieved.AutoTest)
	}
	if retrieved.AutoReview != false {
		t.Errorf("Expected AutoReview default=false, got %v", retrieved.AutoReview)
	}
}

func TestUpdateProjectAutoFlags(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := &models.Project{
		Name:       "test-update-project",
		RepoPath:   "/tmp/test",
		AutoTest:   false,
		AutoReview: false,
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	project.AutoTest = true
	project.AutoReview = true
	if err := db.UpdateProject(project); err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}

	retrieved, err := db.GetProject(project.ID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}

	if retrieved.AutoTest != true {
		t.Errorf("Expected AutoTest=true after update, got %v", retrieved.AutoTest)
	}
	if retrieved.AutoReview != true {
		t.Errorf("Expected AutoReview=true after update, got %v", retrieved.AutoReview)
	}
}

func TestCreateProjectDefaultRunner(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := &models.Project{
		Name:     "test-default-runner-project",
		RepoPath: "/tmp/test",
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	retrieved, err := db.GetProject(project.ID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}

	if retrieved.Runner != "qwen" {
		t.Errorf("Expected Runner default='qwen', got %q", retrieved.Runner)
	}
}

func TestCreateProjectWithRunner(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := &models.Project{
		Name:     "test-opencode-runner-project",
		RepoPath: "/tmp/test",
		Runner:   "opencode",
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	retrieved, err := db.GetProject(project.ID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}

	if retrieved.Runner != "opencode" {
		t.Errorf("Expected Runner='opencode', got %q", retrieved.Runner)
	}
}

func TestUpdateProjectRunner(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := &models.Project{
		Name:     "test-update-runner-project",
		RepoPath: "/tmp/test",
		Runner:   "qwen",
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	project.Runner = "opencode"
	if err := db.UpdateProject(project); err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}

	retrieved, err := db.GetProject(project.ID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}

	if retrieved.Runner != "opencode" {
		t.Errorf("Expected Runner='opencode' after update, got %q", retrieved.Runner)
	}
}

func TestGetProjectsReturnsRunner(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p1 := &models.Project{Name: "runner-p1", RepoPath: "/tmp/p1", Runner: "opencode"}
	p2 := &models.Project{Name: "runner-p2", RepoPath: "/tmp/p2", Runner: "qwen"}

	db.CreateProject(p1)
	db.CreateProject(p2)

	projects, err := db.GetProjects()
	if err != nil {
		t.Fatalf("GetProjects failed: %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("Expected 2 projects, got %d", len(projects))
	}

	foundP1, foundP2 := false, false
	for _, p := range projects {
		if p.Name == "runner-p1" {
			foundP1 = true
			if p.Runner != "opencode" {
				t.Errorf("runner-p1: expected Runner='opencode', got %q", p.Runner)
			}
		}
		if p.Name == "runner-p2" {
			foundP2 = true
			if p.Runner != "qwen" {
				t.Errorf("runner-p2: expected Runner='qwen', got %q", p.Runner)
			}
		}
	}

	if !foundP1 || !foundP2 {
		t.Error("Not all projects found in GetProjects")
	}
}

func TestGetProjectsReturnsAutoFlags(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p1 := &models.Project{Name: "p1", RepoPath: "/tmp/p1", AutoTest: true, AutoReview: false}
	p2 := &models.Project{Name: "p2", RepoPath: "/tmp/p2", AutoTest: false, AutoReview: true}

	db.CreateProject(p1)
	db.CreateProject(p2)

	projects, err := db.GetProjects()
	if err != nil {
		t.Fatalf("GetProjects failed: %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("Expected 2 projects, got %d", len(projects))
	}

	foundP1, foundP2 := false, false
	for _, p := range projects {
		if p.Name == "p1" {
			foundP1 = true
			if !p.AutoTest || p.AutoReview {
				t.Errorf("p1: expected AutoTest=true, AutoReview=false, got %v, %v", p.AutoTest, p.AutoReview)
			}
		}
		if p.Name == "p2" {
			foundP2 = true
			if p.AutoTest || !p.AutoReview {
				t.Errorf("p2: expected AutoTest=false, AutoReview=true, got %v, %v", p.AutoTest, p.AutoReview)
			}
		}
	}

	if !foundP1 || !foundP2 {
		t.Error("Not all projects found in GetProjects")
	}
}

func TestGetWorkflowsByProjectReturnsTicketInfo(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := &models.Ticket{
		ProjectID: project.ID,
		Summary:   "Fix login bug",
		JiraKey:   "LOCAL-abc123",
	}
	if err := db.CreateTicket(ticket); err != nil {
		t.Fatalf("Failed to create ticket: %v", err)
	}

	workflow := &models.Workflow{
		ProjectID: project.ID,
		TicketID:  ticket.ID,
	}
	if err := db.CreateWorkflow(workflow); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	workflows, err := db.GetWorkflowsByProject(project.ID)
	if err != nil {
		t.Fatalf("GetWorkflowsByProject failed: %v", err)
	}

	if len(workflows) != 1 {
		t.Fatalf("Expected 1 workflow, got %d", len(workflows))
	}

	wf := workflows[0]
	if wf.TicketSummary != "Fix login bug" {
		t.Errorf("Expected TicketSummary %q, got %q", "Fix login bug", wf.TicketSummary)
	}
	if wf.TicketJiraKey != ticket.JiraKey {
		t.Errorf("Expected TicketJiraKey %q, got %q", ticket.JiraKey, wf.TicketJiraKey)
	}
}

func TestGetWorkflowsByProjectExcludesDoneWorkflows(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := createTestTicket(t, db, project.ID)

	wfDone := &models.Workflow{ProjectID: project.ID, TicketID: ticket.ID}
	if err := db.CreateWorkflow(wfDone); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}
	if err := db.UpdateWorkflowStatus(wfDone.ID, models.WorkflowDone, ""); err != nil {
		t.Fatalf("Failed to update workflow status: %v", err)
	}

	wfActive := &models.Workflow{ProjectID: project.ID, TicketID: ticket.ID}
	if err := db.CreateWorkflow(wfActive); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	workflows, err := db.GetWorkflowsByProject(project.ID)
	if err != nil {
		t.Fatalf("GetWorkflowsByProject failed: %v", err)
	}

	if len(workflows) != 1 {
		t.Fatalf("Expected 1 workflow (DONE excluded), got %d", len(workflows))
	}

	if workflows[0].ID != wfActive.ID {
		t.Errorf("Expected active workflow ID %q, got %q", wfActive.ID, workflows[0].ID)
	}
}

func TestGetWorkflowsByProjectIncludesNonDoneStatuses(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := createTestTicket(t, db, project.ID)

	statuses := []models.WorkflowStatus{
		models.WorkflowCreated,
		models.WorkflowCoding,
		models.WorkflowFailed,
		models.WorkflowAwaitingApproval,
	}

	for _, status := range statuses {
		wf := &models.Workflow{ProjectID: project.ID, TicketID: ticket.ID}
		if err := db.CreateWorkflow(wf); err != nil {
			t.Fatalf("Failed to create workflow: %v", err)
		}
		if status != models.WorkflowCreated {
			if err := db.UpdateWorkflowStatus(wf.ID, status, ""); err != nil {
				t.Fatalf("Failed to update workflow status: %v", err)
			}
		}
	}

	workflows, err := db.GetWorkflowsByProject(project.ID)
	if err != nil {
		t.Fatalf("GetWorkflowsByProject failed: %v", err)
	}

	if len(workflows) != len(statuses) {
		t.Errorf("Expected %d workflows, got %d", len(statuses), len(workflows))
	}
}

func TestGetWorkflowsByProjectAllDoneReturnsEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := createTestTicket(t, db, project.ID)

	for i := 0; i < 3; i++ {
		wf := &models.Workflow{ProjectID: project.ID, TicketID: ticket.ID}
		if err := db.CreateWorkflow(wf); err != nil {
			t.Fatalf("Failed to create workflow: %v", err)
		}
		if err := db.UpdateWorkflowStatus(wf.ID, models.WorkflowDone, ""); err != nil {
			t.Fatalf("Failed to update workflow status: %v", err)
		}
	}

	workflows, err := db.GetWorkflowsByProject(project.ID)
	if err != nil {
		t.Fatalf("GetWorkflowsByProject failed: %v", err)
	}

	if len(workflows) != 0 {
		t.Errorf("Expected 0 workflows when all are DONE, got %d", len(workflows))
	}
}

func TestUpdateTicketStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := createTestTicket(t, db, project.ID)

	if err := db.UpdateTicketStatus(ticket.ID, "done"); err != nil {
		t.Fatalf("UpdateTicketStatus failed: %v", err)
	}

	retrieved, err := db.GetTicketByID(ticket.ID)
	if err != nil {
		t.Fatalf("GetTicketByID failed: %v", err)
	}
	if retrieved.Status != "done" {
		t.Errorf("Expected status 'done', got %q", retrieved.Status)
	}
}

func TestCreateProjectSavesModel(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := &models.Project{
		Name:     "model-project",
		RepoPath: "/tmp/test",
		Runner:   "opencode",
		Model:    "claude-3-5-sonnet",
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	retrieved, err := db.GetProject(project.ID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}

	if retrieved.Model != "claude-3-5-sonnet" {
		t.Errorf("Expected Model='claude-3-5-sonnet', got %q", retrieved.Model)
	}
}

func TestGetProjectReadsModel(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := &models.Project{
		Name:     "get-model-project",
		RepoPath: "/tmp/test",
		Model:    "gpt-4o",
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	retrieved, err := db.GetProject(project.ID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}

	if retrieved.Model != "gpt-4o" {
		t.Errorf("Expected Model='gpt-4o', got %q", retrieved.Model)
	}
}

func TestUpdateProjectModel(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := &models.Project{
		Name:     "update-model-project",
		RepoPath: "/tmp/test",
		Model:    "old-model",
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	project.Model = "new-model"
	if err := db.UpdateProject(project); err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}

	retrieved, err := db.GetProject(project.ID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}

	if retrieved.Model != "new-model" {
		t.Errorf("Expected Model='new-model' after update, got %q", retrieved.Model)
	}
}

func TestCreateProjectDefaultModel(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := &models.Project{
		Name:     "default-model-project",
		RepoPath: "/tmp/test",
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	retrieved, err := db.GetProject(project.ID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}

	if retrieved.Model != "" {
		t.Errorf("Expected Model default='', got %q", retrieved.Model)
	}
}

func TestGetTicketsByProjectExcludesDone(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)

	ticketDone := &models.Ticket{ProjectID: project.ID, Summary: "Done ticket"}
	if err := db.CreateTicket(ticketDone); err != nil {
		t.Fatalf("Failed to create done ticket: %v", err)
	}
	if err := db.UpdateTicketStatus(ticketDone.ID, "done"); err != nil {
		t.Fatalf("Failed to set done status: %v", err)
	}

	ticketActive := &models.Ticket{ProjectID: project.ID, Summary: "Active ticket", Status: "in_progress"}
	if err := db.CreateTicket(ticketActive); err != nil {
		t.Fatalf("Failed to create active ticket: %v", err)
	}

	tickets, err := db.GetTicketsByProject(project.ID)
	if err != nil {
		t.Fatalf("GetTicketsByProject failed: %v", err)
	}

	if len(tickets) != 1 {
		t.Fatalf("Expected 1 ticket (done excluded), got %d", len(tickets))
	}
	if tickets[0].ID != ticketActive.ID {
		t.Errorf("Expected active ticket, got ticket ID %q", tickets[0].ID)
	}
}

func TestGetTicketsByProjectIncludesNonDone(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)

	statuses := []string{"todo", "in_progress", "review", ""}
	for _, status := range statuses {
		t := &models.Ticket{ProjectID: project.ID, Summary: "Ticket " + status, Status: status}
		db.CreateTicket(t)
	}

	tickets, err := db.GetTicketsByProject(project.ID)
	if err != nil {
		t.Fatalf("GetTicketsByProject failed: %v", err)
	}

	if len(tickets) != len(statuses) {
		t.Errorf("Expected %d tickets, got %d", len(statuses), len(tickets))
	}
}

func TestUpdateTicket(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := createTestTicket(t, db, project.ID)

	ticket.Summary = "Updated summary"
	ticket.Description = "Updated description"
	ticket.AcceptanceCriteria = "- Done"
	ticket.Priority = "High"
	ticket.Labels = "backend"

	if err := db.UpdateTicket(ticket); err != nil {
		t.Fatalf("UpdateTicket failed: %v", err)
	}

	retrieved, err := db.GetTicketByID(ticket.ID)
	if err != nil {
		t.Fatalf("GetTicketByID failed: %v", err)
	}

	if retrieved.Summary != "Updated summary" {
		t.Errorf("Expected summary %q, got %q", "Updated summary", retrieved.Summary)
	}
	if retrieved.Description != "Updated description" {
		t.Errorf("Expected description %q, got %q", "Updated description", retrieved.Description)
	}
	if retrieved.AcceptanceCriteria != "- Done" {
		t.Errorf("Expected acceptance_criteria %q, got %q", "- Done", retrieved.AcceptanceCriteria)
	}
	if retrieved.Priority != "High" {
		t.Errorf("Expected priority %q, got %q", "High", retrieved.Priority)
	}
	if retrieved.Labels != "backend" {
		t.Errorf("Expected labels %q, got %q", "backend", retrieved.Labels)
	}
}

func TestGetWorkflowsByTicket(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := createTestTicket(t, db, project.ID)

	wf1 := &models.Workflow{ProjectID: project.ID, TicketID: ticket.ID}
	wf2 := &models.Workflow{ProjectID: project.ID, TicketID: ticket.ID}
	if err := db.CreateWorkflow(wf1); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}
	if err := db.CreateWorkflow(wf2); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	workflows, err := db.GetWorkflowsByTicket(ticket.ID)
	if err != nil {
		t.Fatalf("GetWorkflowsByTicket failed: %v", err)
	}

	if len(workflows) != 2 {
		t.Errorf("Expected 2 workflows, got %d", len(workflows))
	}

	for _, wf := range workflows {
		if wf.TicketSummary != ticket.Summary {
			t.Errorf("Expected TicketSummary %q, got %q", ticket.Summary, wf.TicketSummary)
		}
	}
}

func TestGetWorkflowsByTicketReturnsEmptyWhenNone(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := createTestTicket(t, db, project.ID)

	workflows, err := db.GetWorkflowsByTicket(ticket.ID)
	if err != nil {
		t.Fatalf("GetWorkflowsByTicket failed: %v", err)
	}

	if workflows != nil {
		t.Errorf("Expected nil/empty slice, got %d workflows", len(workflows))
	}
}

func TestGetWorkflowsByProjectNoTicketReturnsEmptyStrings(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)

	workflow := &models.Workflow{
		ProjectID: project.ID,
		TicketID:  "",
	}
	if err := db.CreateWorkflow(workflow); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	workflows, err := db.GetWorkflowsByProject(project.ID)
	if err != nil {
		t.Fatalf("GetWorkflowsByProject failed: %v", err)
	}

	if len(workflows) != 1 {
		t.Fatalf("Expected 1 workflow, got %d", len(workflows))
	}

	wf := workflows[0]
	if wf.TicketSummary != "" {
		t.Errorf("Expected empty TicketSummary, got %q", wf.TicketSummary)
	}
	if wf.TicketJiraKey != "" {
		t.Errorf("Expected empty TicketJiraKey, got %q", wf.TicketJiraKey)
	}
}

func TestGetWorkflowReturnsTicketSummaryAndJiraKey(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := &models.Ticket{
		ProjectID: project.ID,
		Summary:   "Implement feature X",
		JiraKey:   "PROJ-42",
	}
	if err := db.CreateTicket(ticket); err != nil {
		t.Fatalf("Failed to create ticket: %v", err)
	}

	workflow := createTestWorkflow(t, db, project.ID, ticket.ID)

	wf, err := db.GetWorkflow(workflow.ID)
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if wf == nil {
		t.Fatal("Expected workflow, got nil")
	}

	if wf.TicketSummary != "Implement feature X" {
		t.Errorf("Expected TicketSummary %q, got %q", "Implement feature X", wf.TicketSummary)
	}
	if wf.TicketJiraKey != ticket.JiraKey {
		t.Errorf("Expected TicketJiraKey %q, got %q", ticket.JiraKey, wf.TicketJiraKey)
	}
}

func TestGetWorkflowWithoutTicketReturnsEmptyStrings(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)

	workflow := &models.Workflow{
		ProjectID: project.ID,
		TicketID:  "",
	}
	if err := db.CreateWorkflow(workflow); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	wf, err := db.GetWorkflow(workflow.ID)
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if wf == nil {
		t.Fatal("Expected workflow, got nil")
	}

	if wf.TicketSummary != "" {
		t.Errorf("Expected empty TicketSummary, got %q", wf.TicketSummary)
	}
	if wf.TicketJiraKey != "" {
		t.Errorf("Expected empty TicketJiraKey, got %q", wf.TicketJiraKey)
	}
}

func TestCreateTicketSavesAIGeneratedTrue(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := &models.Ticket{
		ProjectID:   project.ID,
		Summary:     "AI Ticket",
		AIGenerated: true,
		AIMetadata:  `{"title":"AI Ticket"}`,
	}
	if err := db.CreateTicket(ticket); err != nil {
		t.Fatalf("CreateTicket failed: %v", err)
	}

	retrieved, err := db.GetTicketByID(ticket.ID)
	if err != nil {
		t.Fatalf("GetTicketByID failed: %v", err)
	}
	if !retrieved.AIGenerated {
		t.Error("Expected AIGenerated=true, got false")
	}
}

func TestCreateTicketSavesAIMetadataJSON(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	meta := `{"title":"Test","priority":"High"}`
	ticket := &models.Ticket{
		ProjectID:  project.ID,
		Summary:    "AI Ticket with metadata",
		AIMetadata: meta,
	}
	if err := db.CreateTicket(ticket); err != nil {
		t.Fatalf("CreateTicket failed: %v", err)
	}

	retrieved, err := db.GetTicketByID(ticket.ID)
	if err != nil {
		t.Fatalf("GetTicketByID failed: %v", err)
	}
	if retrieved.AIMetadata != meta {
		t.Errorf("Expected AIMetadata %q, got %q", meta, retrieved.AIMetadata)
	}
}

func TestCreateTicketSavesRefinementCount(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	ticket := &models.Ticket{
		ProjectID:       project.ID,
		Summary:         "AI Ticket with refinement",
		RefinementCount: 3,
	}
	if err := db.CreateTicket(ticket); err != nil {
		t.Fatalf("CreateTicket failed: %v", err)
	}

	retrieved, err := db.GetTicketByID(ticket.ID)
	if err != nil {
		t.Fatalf("GetTicketByID failed: %v", err)
	}
	if retrieved.RefinementCount != 3 {
		t.Errorf("Expected RefinementCount=3, got %d", retrieved.RefinementCount)
	}
}

func TestGetTicketByIDReturnsAIFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	project := createTestProject(t, db)
	meta := `{"title":"X","complexity":"complex"}`
	ticket := &models.Ticket{
		ProjectID:       project.ID,
		Summary:         "Full AI Ticket",
		AIGenerated:     true,
		AIMetadata:      meta,
		RefinementCount: 2,
	}
	if err := db.CreateTicket(ticket); err != nil {
		t.Fatalf("CreateTicket failed: %v", err)
	}

	retrieved, err := db.GetTicketByID(ticket.ID)
	if err != nil {
		t.Fatalf("GetTicketByID failed: %v", err)
	}
	if !retrieved.AIGenerated {
		t.Error("Expected AIGenerated=true")
	}
	if retrieved.AIMetadata != meta {
		t.Errorf("Expected AIMetadata %q, got %q", meta, retrieved.AIMetadata)
	}
	if retrieved.RefinementCount != 2 {
		t.Errorf("Expected RefinementCount=2, got %d", retrieved.RefinementCount)
	}
}
