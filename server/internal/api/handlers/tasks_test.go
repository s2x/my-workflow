package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/piotr-halas/decodo-workflow/internal/db"
	"github.com/piotr-halas/decodo-workflow/internal/models"
)

func TestStreamLogsReturnsSSE(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	task := createTestTaskForHandler(t, database)
	handler := NewTaskHandler(database)

	req := httptest.NewRequest("GET", "/api/tasks/"+task.ID+"/logs/stream", nil)
	w := httptest.NewRecorder()

	req.SetPathValue("id", task.ID)

	go func() {
		time.Sleep(100 * time.Millisecond)
		database.UpdateTaskStatus(task.ID, models.TaskCompleted)
	}()

	handler.StreamLogs(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", contentType)
	}
}

func TestStreamLogsReturnsHistoricalLogs(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	task := createTestTaskForHandler(t, database)

	database.CreateTaskLog(&models.TaskLog{
		TaskID:   task.ID,
		LogLevel: models.LogLevelInfo,
		Message:  "Historical log 1",
	})

	database.CreateTaskLog(&models.TaskLog{
		TaskID:   task.ID,
		LogLevel: models.LogLevelInfo,
		Message:  "Historical log 2",
	})

	database.UpdateTaskStatus(task.ID, models.TaskCompleted)

	handler := NewTaskHandler(database)
	req := httptest.NewRequest("GET", "/api/tasks/"+task.ID+"/logs/stream", nil)
	w := httptest.NewRecorder()

	req.SetPathValue("id", task.ID)

	handler.StreamLogs(w, req)

	body := w.Body.String()

	if !containsLogMessage(body, "Historical log 1") {
		t.Error("Expected response to contain 'Historical log 1'")
	}

	if !containsLogMessage(body, "Historical log 2") {
		t.Error("Expected response to contain 'Historical log 2'")
	}
}

func TestStreamLogsReturns404ForNonexistentTask(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewTaskHandler(database)
	req := httptest.NewRequest("GET", "/api/tasks/nonexistent/logs/stream", nil)
	w := httptest.NewRecorder()

	req.SetPathValue("id", "nonexistent")

	handler.StreamLogs(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func setupTestDB(t *testing.T) *db.DB {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	return database
}

func createTestTaskForHandler(t *testing.T, database *db.DB) *models.Task {
	project := &models.Project{
		Name:     "test-project",
		RepoPath: "/tmp/test",
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	ticket := &models.Ticket{
		ProjectID: project.ID,
		Summary:   "Test ticket",
	}
	if err := database.CreateTicket(ticket); err != nil {
		t.Fatalf("Failed to create test ticket: %v", err)
	}

	workflow := &models.Workflow{
		ProjectID: project.ID,
		TicketID:  ticket.ID,
	}
	if err := database.CreateWorkflow(workflow); err != nil {
		t.Fatalf("Failed to create test workflow: %v", err)
	}

	task := &models.Task{
		WorkflowID: workflow.ID,
		Type:       models.TaskCode,
		Agent:      "test-agent",
		Status:     models.TaskRunning,
	}
	if err := database.CreateTask(task); err != nil {
		t.Fatalf("Failed to create test task: %v", err)
	}

	return task
}

func containsLogMessage(body, message string) bool {
	lines := splitSSEData(body)
	for _, line := range lines {
		var log models.TaskLog
		if err := json.Unmarshal([]byte(line), &log); err == nil {
			if log.Message == message {
				return true
			}
		}
	}
	return false
}

func splitSSEData(body string) []string {
	var result []string
	lines := []rune(body)
	var current []rune
	prefix := "data: "

	for i := 0; i < len(lines); i++ {
		if i+len(prefix) < len(lines) && string(lines[i:i+len(prefix)]) == prefix {
			i += len(prefix)
			current = []rune{}
			for i < len(lines) && lines[i] != '\n' {
				current = append(current, lines[i])
				i++
			}
			if len(current) > 0 {
				result = append(result, string(current))
			}
		}
	}
	return result
}

func TestStreamLogsHandlesNewLogsWhileRunning(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	task := createTestTaskForHandler(t, database)

	database.CreateTaskLog(&models.TaskLog{
		TaskID:   task.ID,
		LogLevel: models.LogLevelInfo,
		Message:  "Initial log",
	})

	handler := NewTaskHandler(database)

	done := make(chan bool)
	go func() {
		time.Sleep(100 * time.Millisecond)
		database.CreateTaskLog(&models.TaskLog{
			TaskID:   task.ID,
			LogLevel: models.LogLevelInfo,
			Message:  "New log while running",
		})
		time.Sleep(700 * time.Millisecond)
		database.UpdateTaskStatus(task.ID, models.TaskCompleted)
		done <- true
	}()

	req := httptest.NewRequest("GET", "/api/tasks/"+task.ID+"/logs/stream", nil)
	w := httptest.NewRecorder()
	req.SetPathValue("id", task.ID)

	handler.StreamLogs(w, req)
	<-done

	body := w.Body.String()

	if !containsLogMessage(body, "Initial log") {
		t.Error("Expected response to contain 'Initial log'")
	}

	if !containsLogMessage(body, "New log while running") {
		t.Error("Expected response to contain 'New log while running'")
	}
}

func TestStreamLogsClosesWhenTaskCompletes(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	task := createTestTaskForHandler(t, database)

	handler := NewTaskHandler(database)

	go func() {
		time.Sleep(100 * time.Millisecond)
		database.UpdateTaskStatus(task.ID, models.TaskCompleted)
	}()

	req := httptest.NewRequest("GET", "/api/tasks/"+task.ID+"/logs/stream", nil)
	w := httptest.NewRecorder()
	req.SetPathValue("id", task.ID)

	handler.StreamLogs(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestStreamLogsWithDatabaseError(t *testing.T) {
	database := setupTestDB(t)
	database.Close()

	handler := NewTaskHandler(database)
	req := httptest.NewRequest("GET", "/api/tasks/some-id/logs/stream", nil)
	w := httptest.NewRecorder()
	req.SetPathValue("id", "some-id")

	handler.StreamLogs(w, req)

	resp := w.Result()
	if resp.StatusCode == http.StatusOK {
		t.Error("Expected error status when database is closed")
	}
}

func TestStreamLogsWithMultipleLevels(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	task := createTestTaskForHandler(t, database)

	database.CreateTaskLog(&models.TaskLog{
		TaskID:   task.ID,
		LogLevel: models.LogLevelInfo,
		Message:  "Info message",
	})

	database.CreateTaskLog(&models.TaskLog{
		TaskID:   task.ID,
		LogLevel: models.LogLevelDebug,
		Message:  "Debug message",
	})

	database.CreateTaskLog(&models.TaskLog{
		TaskID:   task.ID,
		LogLevel: models.LogLevelError,
		Message:  "Error message",
	})

	database.UpdateTaskStatus(task.ID, models.TaskCompleted)

	handler := NewTaskHandler(database)
	req := httptest.NewRequest("GET", "/api/tasks/"+task.ID+"/logs/stream", nil)
	w := httptest.NewRecorder()
	req.SetPathValue("id", task.ID)

	handler.StreamLogs(w, req)

	body := w.Body.String()

	if !containsLogMessage(body, "Info message") {
		t.Error("Expected response to contain 'Info message'")
	}

	if !containsLogMessage(body, "Debug message") {
		t.Error("Expected response to contain 'Debug message'")
	}

	if !containsLogMessage(body, "Error message") {
		t.Error("Expected response to contain 'Error message'")
	}
}
