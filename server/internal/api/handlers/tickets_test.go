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

func setupTicketTestHandler(t *testing.T) (*TicketHandler, *db.DB) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	handler := NewTicketHandler(database)
	return handler, database
}

func createProject(t *testing.T, database *db.DB) *models.Project {
	p := &models.Project{Name: "test-project", RepoPath: "/tmp/test"}
	if err := database.CreateProject(p); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	return p
}

func createTicket(t *testing.T, database *db.DB, projectID string) *models.Ticket {
	ticket := &models.Ticket{
		ProjectID:   projectID,
		Summary:     "Test ticket",
		Description: "Some description",
		Priority:    "Medium",
	}
	if err := database.CreateTicket(ticket); err != nil {
		t.Fatalf("Failed to create ticket: %v", err)
	}
	return ticket
}

func TestGetByIDReturns200(t *testing.T) {
	handler, database := setupTicketTestHandler(t)
	defer database.Close()

	project := createProject(t, database)
	ticket := createTicket(t, database, project.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/tickets/"+ticket.ID, nil)
	req.SetPathValue("id", ticket.ID)
	w := httptest.NewRecorder()

	handler.GetByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result models.Ticket
	json.NewDecoder(w.Body).Decode(&result)

	if result.ID != ticket.ID {
		t.Errorf("Expected ticket ID %q, got %q", ticket.ID, result.ID)
	}
	if result.Summary != ticket.Summary {
		t.Errorf("Expected summary %q, got %q", ticket.Summary, result.Summary)
	}
}

func TestGetByIDReturns404WhenNotFound(t *testing.T) {
	handler, database := setupTicketTestHandler(t)
	defer database.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/tickets/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	handler.GetByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestUpdateTicketReturns200(t *testing.T) {
	handler, database := setupTicketTestHandler(t)
	defer database.Close()

	project := createProject(t, database)
	ticket := createTicket(t, database, project.ID)

	payload := UpdateTicketRequest{
		Summary:            "Updated summary",
		Description:        "Updated description",
		AcceptanceCriteria: "- Done",
		Priority:           "High",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/tickets/"+ticket.ID, bytes.NewReader(body))
	req.SetPathValue("id", ticket.ID)
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result models.Ticket
	json.NewDecoder(w.Body).Decode(&result)

	if result.Summary != "Updated summary" {
		t.Errorf("Expected summary %q, got %q", "Updated summary", result.Summary)
	}
	if result.Priority != "High" {
		t.Errorf("Expected priority %q, got %q", "High", result.Priority)
	}
}

func TestUpdateTicketReturns400WhenSummaryEmpty(t *testing.T) {
	handler, database := setupTicketTestHandler(t)
	defer database.Close()

	project := createProject(t, database)
	ticket := createTicket(t, database, project.ID)

	payload := UpdateTicketRequest{Summary: ""}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/tickets/"+ticket.ID, bytes.NewReader(body))
	req.SetPathValue("id", ticket.ID)
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestUpdateTicketReturns404WhenNotFound(t *testing.T) {
	handler, database := setupTicketTestHandler(t)
	defer database.Close()

	payload := UpdateTicketRequest{Summary: "Some summary"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/tickets/nonexistent", bytes.NewReader(body))
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestUpdateTicketStatusToDoneReturns200(t *testing.T) {
	handler, database := setupTicketTestHandler(t)
	defer database.Close()

	project := createProject(t, database)
	ticket := createTicket(t, database, project.ID)

	payload := UpdateTicketRequest{
		Summary: "Some summary",
		Status:  "done",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/tickets/"+ticket.ID, bytes.NewReader(body))
	req.SetPathValue("id", ticket.ID)
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result models.Ticket
	json.NewDecoder(w.Body).Decode(&result)

	if result.Status != "done" {
		t.Errorf("Expected status %q, got %q", "done", result.Status)
	}
}

func TestUpdateTicketStatusInvalidReturns400(t *testing.T) {
	handler, database := setupTicketTestHandler(t)
	defer database.Close()

	project := createProject(t, database)
	ticket := createTicket(t, database, project.ID)

	payload := UpdateTicketRequest{
		Summary: "Some summary",
		Status:  "invalid",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/tickets/"+ticket.ID, bytes.NewReader(body))
	req.SetPathValue("id", ticket.ID)
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestUpdateTicketWithoutStatusDoesNotChangeStatus(t *testing.T) {
	handler, database := setupTicketTestHandler(t)
	defer database.Close()

	project := createProject(t, database)
	ticket := createTicket(t, database, project.ID)

	originalStatus := ticket.Status

	payload := UpdateTicketRequest{
		Summary:     "Updated summary",
		Description: "Updated description",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/tickets/"+ticket.ID, bytes.NewReader(body))
	req.SetPathValue("id", ticket.ID)
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	updated, err := database.GetTicketByID(ticket.ID)
	if err != nil {
		t.Fatalf("Failed to get ticket: %v", err)
	}
	if updated.Status != originalStatus {
		t.Errorf("Expected status %q, got %q", originalStatus, updated.Status)
	}
}

func TestGetWorkflowsReturnsWorkflowsForTicket(t *testing.T) {
	handler, database := setupTicketTestHandler(t)
	defer database.Close()

	project := createProject(t, database)
	ticket := createTicket(t, database, project.ID)

	wf := &models.Workflow{ProjectID: project.ID, TicketID: ticket.ID}
	if err := database.CreateWorkflow(wf); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tickets/"+ticket.ID+"/workflows", nil)
	req.SetPathValue("id", ticket.ID)
	w := httptest.NewRecorder()

	handler.GetWorkflows(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var workflows []models.Workflow
	json.NewDecoder(w.Body).Decode(&workflows)

	if len(workflows) != 1 {
		t.Errorf("Expected 1 workflow, got %d", len(workflows))
	}
	if workflows[0].ID != wf.ID {
		t.Errorf("Expected workflow ID %q, got %q", wf.ID, workflows[0].ID)
	}
}

func TestGetWorkflowsReturnsEmptyListWhenNoWorkflows(t *testing.T) {
	handler, database := setupTicketTestHandler(t)
	defer database.Close()

	project := createProject(t, database)
	ticket := createTicket(t, database, project.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/tickets/"+ticket.ID+"/workflows", nil)
	req.SetPathValue("id", ticket.ID)
	w := httptest.NewRecorder()

	handler.GetWorkflows(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var workflows []models.Workflow
	json.NewDecoder(w.Body).Decode(&workflows)

	if len(workflows) != 0 {
		t.Errorf("Expected 0 workflows, got %d", len(workflows))
	}
}
