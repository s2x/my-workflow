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
