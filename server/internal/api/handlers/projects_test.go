package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/piotr-halas/decodo-workflow/internal/db"
	"github.com/piotr-halas/decodo-workflow/internal/models"
)

func setupTestHandler(t *testing.T) (*ProjectHandler, *db.DB) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	handler := NewProjectHandler(database)
	return handler, database
}

func TestCreateProjectWithAutoTest(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	payload := CreateProjectRequest{
		Name:       "test-project",
		RepoPath:   "/tmp/test",
		AutoTest:   true,
		AutoReview: false,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var project models.Project
	json.NewDecoder(w.Body).Decode(&project)

	if !project.AutoTest {
		t.Errorf("Expected AutoTest=true, got %v", project.AutoTest)
	}
	if project.AutoReview {
		t.Errorf("Expected AutoReview=false, got %v", project.AutoReview)
	}
}

func TestCreateProjectDefaultAutoFlags(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	payload := CreateProjectRequest{
		Name:     "test-project",
		RepoPath: "/tmp/test",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var project models.Project
	json.NewDecoder(w.Body).Decode(&project)

	if project.AutoTest {
		t.Errorf("Expected AutoTest default=false, got %v", project.AutoTest)
	}
	if project.AutoReview {
		t.Errorf("Expected AutoReview default=false, got %v", project.AutoReview)
	}
}

func TestUpdateProjectAutoFlags(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	project := &models.Project{
		Name:       "test-project",
		RepoPath:   "/tmp/test",
		AutoTest:   false,
		AutoReview: false,
	}
	database.CreateProject(project)

	payload := CreateProjectRequest{
		Name:       "test-project",
		RepoPath:   "/tmp/test",
		AutoTest:   true,
		AutoReview: true,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/projects/"+project.ID, bytes.NewReader(body))
	req.SetPathValue("projectId", project.ID)
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var updated models.Project
	json.NewDecoder(w.Body).Decode(&updated)

	if !updated.AutoTest {
		t.Errorf("Expected AutoTest=true after update, got %v", updated.AutoTest)
	}
	if !updated.AutoReview {
		t.Errorf("Expected AutoReview=true after update, got %v", updated.AutoReview)
	}
}

func TestGetProjectReturnsAutoFlags(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	project := &models.Project{
		Name:       "test-project",
		RepoPath:   "/tmp/test",
		AutoTest:   true,
		AutoReview: false,
	}
	database.CreateProject(project)

	req := httptest.NewRequest(http.MethodGet, "/projects/"+project.ID, nil)
	req.SetPathValue("projectId", project.ID)
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var retrieved models.Project
	json.NewDecoder(w.Body).Decode(&retrieved)

	if !retrieved.AutoTest {
		t.Errorf("Expected AutoTest=true, got %v", retrieved.AutoTest)
	}
	if retrieved.AutoReview {
		t.Errorf("Expected AutoReview=false, got %v", retrieved.AutoReview)
	}
}

func TestListProjectsReturnsAutoFlags(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	p1 := &models.Project{Name: "p1", RepoPath: "/tmp/p1", AutoTest: true, AutoReview: false}
	p2 := &models.Project{Name: "p2", RepoPath: "/tmp/p2", AutoTest: false, AutoReview: true}

	database.CreateProject(p1)
	database.CreateProject(p2)

	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var projects []models.Project
	json.NewDecoder(w.Body).Decode(&projects)

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
		t.Error("Not all projects found in List")
	}
}

func TestUpdateProjectPartialAutoFlags(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	project := &models.Project{
		Name:       "test-project",
		RepoPath:   "/tmp/test",
		AutoTest:   true,
		AutoReview: true,
	}
	database.CreateProject(project)

	payload := CreateProjectRequest{
		AutoTest:   false,
		AutoReview: false,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/projects/"+project.ID, bytes.NewReader(body))
	req.SetPathValue("projectId", project.ID)
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var updated models.Project
	json.NewDecoder(w.Body).Decode(&updated)

	if updated.AutoTest {
		t.Errorf("Expected AutoTest=false after update, got %v", updated.AutoTest)
	}
	if updated.AutoReview {
		t.Errorf("Expected AutoReview=false after update, got %v", updated.AutoReview)
	}
}

func TestCreateProjectWithInvalidData(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	tests := []struct {
		name     string
		payload  CreateProjectRequest
		wantCode int
	}{
		{
			name:     "missing name",
			payload:  CreateProjectRequest{RepoPath: "/tmp/test"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "missing repo_path",
			payload:  CreateProjectRequest{Name: "test"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "empty name",
			payload:  CreateProjectRequest{Name: "", RepoPath: "/tmp/test"},
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewReader(body))
			w := httptest.NewRecorder()

			handler.Create(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("Expected status %d, got %d", tt.wantCode, w.Code)
			}
		})
	}
}

func TestUpdateProjectNotFound(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	payload := CreateProjectRequest{
		Name:     "test-project",
		RepoPath: "/tmp/test",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/projects/nonexistent", bytes.NewReader(body))
	req.SetPathValue("projectId", "nonexistent")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestGetProjectNotFound(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	req := httptest.NewRequest(http.MethodGet, "/projects/nonexistent", nil)
	req.SetPathValue("projectId", "nonexistent")
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestDeleteProjectReturns204(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	project := &models.Project{Name: "to-delete", RepoPath: "/tmp/del"}
	database.CreateProject(project)

	req := httptest.NewRequest(http.MethodDelete, "/projects/"+project.ID, nil)
	req.SetPathValue("projectId", project.ID)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}
}

func TestDeleteProjectNotFound(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	req := httptest.NewRequest(http.MethodDelete, "/projects/nonexistent", nil)
	req.SetPathValue("projectId", "nonexistent")
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestDeleteProjectCascadesTickets(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	project := &models.Project{Name: "proj", RepoPath: "/tmp/p"}
	database.CreateProject(project)

	ticket := &models.Ticket{ProjectID: project.ID, Summary: "ticket-1"}
	database.CreateTicket(ticket)

	req := httptest.NewRequest(http.MethodDelete, "/projects/"+project.ID, nil)
	req.SetPathValue("projectId", project.ID)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("Expected 204, got %d", w.Code)
	}

	tickets, err := database.GetTicketsByProject(project.ID)
	if err != nil {
		t.Fatalf("GetTicketsByProject error: %v", err)
	}
	if len(tickets) != 0 {
		t.Errorf("Expected 0 tickets after delete, got %d", len(tickets))
	}
}

func TestDeleteProjectCascadesWorkflows(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	project := &models.Project{Name: "proj", RepoPath: "/tmp/p"}
	database.CreateProject(project)

	workflow := &models.Workflow{ProjectID: project.ID, Status: models.WorkflowCreated}
	database.CreateWorkflow(workflow)

	req := httptest.NewRequest(http.MethodDelete, "/projects/"+project.ID, nil)
	req.SetPathValue("projectId", project.ID)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("Expected 204, got %d", w.Code)
	}

	workflows, err := database.GetWorkflowsByProject(project.ID)
	if err != nil {
		t.Fatalf("GetWorkflowsByProject error: %v", err)
	}
	if len(workflows) != 0 {
		t.Errorf("Expected 0 workflows after delete, got %d", len(workflows))
	}
}

func TestCreateProjectWithModel(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	payload := CreateProjectRequest{
		Name:     "model-project",
		RepoPath: "/tmp/test",
		Runner:   "opencode",
		Model:    "claude-3-5-sonnet",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var project models.Project
	json.NewDecoder(w.Body).Decode(&project)

	if project.Model != "claude-3-5-sonnet" {
		t.Errorf("Expected Model='claude-3-5-sonnet', got %q", project.Model)
	}
}

func TestCreateProjectDefaultModel(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	payload := CreateProjectRequest{
		Name:     "no-model-project",
		RepoPath: "/tmp/test",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var project models.Project
	json.NewDecoder(w.Body).Decode(&project)

	if project.Model != "" {
		t.Errorf("Expected Model default='', got %q", project.Model)
	}
}

func TestUpdateProjectModel(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	project := &models.Project{
		Name:     "update-model-project",
		RepoPath: "/tmp/test",
		Runner:   "opencode",
		Model:    "old-model",
	}
	database.CreateProject(project)

	payload := CreateProjectRequest{
		Name:     "update-model-project",
		RepoPath: "/tmp/test",
		Runner:   "opencode",
		Model:    "new-model",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/projects/"+project.ID, bytes.NewReader(body))
	req.SetPathValue("projectId", project.ID)
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var updated models.Project
	json.NewDecoder(w.Body).Decode(&updated)

	if updated.Model != "new-model" {
		t.Errorf("Expected Model='new-model' after update, got %q", updated.Model)
	}
}

func TestGetModelsReturnsNotFoundForMissingProject(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	req := httptest.NewRequest(http.MethodGet, "/projects/nonexistent/models", nil)
	req.SetPathValue("projectId", "nonexistent")
	w := httptest.NewRecorder()

	handler.GetModels(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestGetModelsReturnsFallbackWhenBinaryFails(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	project := &models.Project{
		Name:     "test-models-project",
		RepoPath: "/tmp/test",
		Runner:   "opencode",
	}
	database.CreateProject(project)

	req := httptest.NewRequest(http.MethodGet, "/projects/"+project.ID+"/models", nil)
	req.SetPathValue("projectId", project.ID)
	w := httptest.NewRecorder()

	handler.GetModels(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp struct {
		Models []string `json:"models"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Models == nil {
		t.Error("Expected models to be a non-nil slice")
	}
}

func TestDeleteProjectCascadesTasksAndLogs(t *testing.T) {
	handler, database := setupTestHandler(t)
	defer database.Close()

	project := &models.Project{Name: "proj", RepoPath: "/tmp/p"}
	database.CreateProject(project)

	workflow := &models.Workflow{ProjectID: project.ID, Status: models.WorkflowCreated}
	database.CreateWorkflow(workflow)

	task := &models.Task{WorkflowID: workflow.ID, Type: "coder", Agent: "test"}
	database.CreateTask(task)

	taskLog := &models.TaskLog{TaskID: task.ID, Message: "log entry"}
	database.CreateTaskLog(taskLog)

	req := httptest.NewRequest(http.MethodDelete, "/projects/"+project.ID, nil)
	req.SetPathValue("projectId", project.ID)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("Expected 204, got %d", w.Code)
	}

	tasks, err := database.GetTasksByWorkflow(workflow.ID)
	if err != nil {
		t.Fatalf("GetTasksByWorkflow error: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks after delete, got %d", len(tasks))
	}

	logs, err := database.GetTaskLogs(task.ID)
	if err != nil {
		t.Fatalf("GetTaskLogs error: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("Expected 0 task_logs after delete, got %d", len(logs))
	}
}
