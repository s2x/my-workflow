package handlers

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/piotr-halas/decodo-workflow/internal/agent"
	"github.com/piotr-halas/decodo-workflow/internal/models"
	"github.com/piotr-halas/decodo-workflow/internal/workflow"
)

func createTestProjectAndTicket(t *testing.T, database interface {
	CreateProject(*models.Project) error
	CreateTicket(*models.Ticket) error
}) (*models.Project, *models.Ticket) {
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

	return project, ticket
}

func TestListByProjectExcludesDoneWorkflows(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	project, ticket := createTestProjectAndTicket(t, database)

	wfDone := &models.Workflow{ProjectID: project.ID, TicketID: ticket.ID}
	if err := database.CreateWorkflow(wfDone); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}
	if err := database.UpdateWorkflowStatus(wfDone.ID, models.WorkflowDone, ""); err != nil {
		t.Fatalf("Failed to update workflow status: %v", err)
	}

	wfActive := &models.Workflow{ProjectID: project.ID, TicketID: ticket.ID}
	if err := database.CreateWorkflow(wfActive); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	handler := NewWorkflowHandler(database, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+project.ID+"/workflows", nil)
	req.SetPathValue("projectId", project.ID)
	w := httptest.NewRecorder()

	handler.ListByProject(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var workflows []models.Workflow
	if err := json.NewDecoder(w.Body).Decode(&workflows); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(workflows) != 1 {
		t.Fatalf("Expected 1 workflow (DONE excluded), got %d", len(workflows))
	}

	if workflows[0].ID != wfActive.ID {
		t.Errorf("Expected active workflow ID %q, got %q", wfActive.ID, workflows[0].ID)
	}
}

func TestListByProjectWorkflowBecomingDoneDisappearsFromList(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	project, ticket := createTestProjectAndTicket(t, database)

	wf := &models.Workflow{ProjectID: project.ID, TicketID: ticket.ID}
	if err := database.CreateWorkflow(wf); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	handler := NewWorkflowHandler(database, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+project.ID+"/workflows", nil)
	req.SetPathValue("projectId", project.ID)
	w := httptest.NewRecorder()
	handler.ListByProject(w, req)

	var workflows []models.Workflow
	json.NewDecoder(w.Body).Decode(&workflows)

	if len(workflows) != 1 {
		t.Fatalf("Expected 1 workflow before DONE, got %d", len(workflows))
	}

	if err := database.UpdateWorkflowStatus(wf.ID, models.WorkflowDone, ""); err != nil {
		t.Fatalf("Failed to update workflow status: %v", err)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/projects/"+project.ID+"/workflows", nil)
	req2.SetPathValue("projectId", project.ID)
	w2 := httptest.NewRecorder()
	handler.ListByProject(w2, req2)

	var workflows2 []models.Workflow
	json.NewDecoder(w2.Body).Decode(&workflows2)

	if len(workflows2) != 0 {
		t.Fatalf("Expected 0 workflows after becoming DONE, got %d", len(workflows2))
	}
}

func TestRejectHandlerWithoutBody(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	runner := agent.NewRunner("opencode", "qwen", slog.New(slog.NewTextHandler(os.Stdout, nil)))
	eng := workflow.NewEngine(database, runner, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	handler := NewWorkflowHandler(database, eng)

	req := httptest.NewRequest(http.MethodPost, "/api/workflows/some-id/reject", nil)
	req.SetPathValue("id", "some-id")
	w := httptest.NewRecorder()

	handler.Reject(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestRejectHandlerWithEmptyComment(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	runner := agent.NewRunner("opencode", "qwen", slog.New(slog.NewTextHandler(os.Stdout, nil)))
	eng := workflow.NewEngine(database, runner, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	handler := NewWorkflowHandler(database, eng)

	body := bytes.NewBufferString(`{"comment":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/workflows/some-id/reject", body)
	req.SetPathValue("id", "some-id")
	w := httptest.NewRecorder()

	handler.Reject(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for empty comment, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "comment is required") {
		t.Errorf("Expected 'comment is required' in body, got %s", w.Body.String())
	}
}

func TestRejectHandlerWithCommentCallsEngine(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	project, ticket := createTestProjectAndTicket(t, database)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowAwaitingApproval,
		BranchName: "feature/test-123",
		Spec:       "{}",
	}
	if err := database.CreateWorkflow(wf); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	runner := agent.NewRunner("opencode", "qwen", slog.New(slog.NewTextHandler(os.Stdout, nil)))
	eng := workflow.NewEngine(database, runner, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	handler := NewWorkflowHandler(database, eng)

	body := bytes.NewBufferString(`{"comment":"Zły kod"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/workflows/"+wf.ID+"/reject", body)
	req.SetPathValue("id", wf.ID)
	w := httptest.NewRecorder()

	handler.Reject(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	updatedWf, _ := database.GetWorkflow(wf.ID)
	if updatedWf.Status != models.WorkflowCoding {
		t.Errorf("Expected workflow status CODING after reject, got %s", updatedWf.Status)
	}
}

func TestRestartHandlerSuccess(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	project, ticket := createTestProjectAndTicket(t, database)

	wf := &models.Workflow{
		ProjectID: project.ID,
		TicketID:  ticket.ID,
		Status:    models.WorkflowFailed,
	}
	if err := database.CreateWorkflow(wf); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	runner := agent.NewRunner("opencode", "qwen", slog.New(slog.NewTextHandler(os.Stdout, nil)))
	eng := workflow.NewEngine(database, runner, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	handler := NewWorkflowHandler(database, eng)

	req := httptest.NewRequest(http.MethodPost, "/api/workflows/"+wf.ID+"/restart", nil)
	req.SetPathValue("id", wf.ID)
	w := httptest.NewRecorder()

	handler.Restart(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if resp["status"] != "restarted" {
		t.Errorf("Expected status=restarted, got %q", resp["status"])
	}

	updatedWf, _ := database.GetWorkflow(wf.ID)
	if updatedWf.Status != models.WorkflowDescribing {
		t.Errorf("Expected workflow status DESCRIBING after restart, got %s", updatedWf.Status)
	}
}

func TestRestartHandlerNonExistentWorkflow(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	runner := agent.NewRunner("opencode", "qwen", slog.New(slog.NewTextHandler(os.Stdout, nil)))
	eng := workflow.NewEngine(database, runner, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	handler := NewWorkflowHandler(database, eng)

	req := httptest.NewRequest(http.MethodPost, "/api/workflows/nonexistent/restart", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	handler.Restart(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestRestartHandlerDeployingForbidden(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	project, ticket := createTestProjectAndTicket(t, database)

	wf := &models.Workflow{
		ProjectID: project.ID,
		TicketID:  ticket.ID,
		Status:    models.WorkflowDeploying,
	}
	if err := database.CreateWorkflow(wf); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	runner := agent.NewRunner("opencode", "qwen", slog.New(slog.NewTextHandler(os.Stdout, nil)))
	eng := workflow.NewEngine(database, runner, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	handler := NewWorkflowHandler(database, eng)

	req := httptest.NewRequest(http.MethodPost, "/api/workflows/"+wf.ID+"/restart", nil)
	req.SetPathValue("id", wf.ID)
	w := httptest.NewRecorder()

	handler.Restart(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for DEPLOYING workflow, got %d", w.Code)
	}
}

func TestListByProjectReturnsEmptyArrayWhenNone(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	project, _ := createTestProjectAndTicket(t, database)

	handler := NewWorkflowHandler(database, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+project.ID+"/workflows", nil)
	req.SetPathValue("projectId", project.ID)
	w := httptest.NewRecorder()
	handler.ListByProject(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var workflows []models.Workflow
	if err := json.NewDecoder(w.Body).Decode(&workflows); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(workflows) != 0 {
		t.Errorf("Expected 0 workflows, got %d", len(workflows))
	}
}
