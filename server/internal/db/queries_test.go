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
