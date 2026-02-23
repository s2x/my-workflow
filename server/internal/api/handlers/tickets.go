package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/piotr-halas/decodo-workflow/internal/db"
	"github.com/piotr-halas/decodo-workflow/internal/models"
)

type TicketHandler struct {
	db *db.DB
}

func NewTicketHandler(db *db.DB) *TicketHandler {
	return &TicketHandler{db: db}
}

type CreateTicketRequest struct {
	Summary            string `json:"summary"`
	Description        string `json:"description"`
	Priority           string `json:"priority"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	TicketType         string `json:"ticket_type"`
	Labels             string `json:"labels"`
	AIGenerated        bool   `json:"ai_generated"`
	AIMetadata         string `json:"ai_metadata"`
	RefinementCount    int    `json:"refinement_count"`
}

func (h *TicketHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	var req CreateTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Summary == "" {
		http.Error(w, "summary is required", http.StatusBadRequest)
		return
	}

	source := "manual"
	if req.AIGenerated {
		source = "ai"
	}

	aiMetadata := req.AIMetadata
	if aiMetadata == "" {
		aiMetadata = "{}"
	}

	ticket := &models.Ticket{
		ProjectID:          projectID,
		Summary:            req.Summary,
		Description:        req.Description,
		Priority:           req.Priority,
		AcceptanceCriteria: req.AcceptanceCriteria,
		TicketType:         req.TicketType,
		Labels:             req.Labels,
		Status:             models.TicketStatusOpen,
		Source:             source,
		RawJSON:            "{}",
		AIGenerated:        req.AIGenerated,
		AIMetadata:         aiMetadata,
		RefinementCount:    req.RefinementCount,
	}

	if err := h.db.CreateTicket(ticket); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ticket)
}

func (h *TicketHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	tickets, err := h.db.GetTicketsByProject(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tickets == nil {
		tickets = []models.Ticket{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tickets)
}

func (h *TicketHandler) List(w http.ResponseWriter, r *http.Request) {
	tickets, err := h.db.GetTickets()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tickets == nil {
		tickets = []models.Ticket{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tickets)
}

type UpdateTicketRequest struct {
	Summary            string `json:"summary"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	Priority           string `json:"priority"`
	Labels             string `json:"labels"`
	Status             string `json:"status"`
}

func (h *TicketHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	ticket, err := h.db.GetTicketByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ticket == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ticket)
}

func (h *TicketHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	var req UpdateTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Summary == "" {
		http.Error(w, "summary is required", http.StatusBadRequest)
		return
	}

	ticket, err := h.db.GetTicketByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ticket == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	ticket.Summary = req.Summary
	ticket.Description = req.Description
	ticket.AcceptanceCriteria = req.AcceptanceCriteria
	ticket.Priority = req.Priority
	ticket.Labels = req.Labels

	if req.Status != "" {
		if req.Status != models.TicketStatusOpen && req.Status != models.TicketStatusDone {
			http.Error(w, "invalid status value", http.StatusBadRequest)
			return
		}
		if err := h.db.UpdateTicketStatus(id, req.Status); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		ticket.Status = req.Status
	}

	if err := h.db.UpdateTicket(ticket); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ticket)
}

func (h *TicketHandler) GetWorkflows(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	workflows, err := h.db.GetWorkflowsByTicket(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if workflows == nil {
		workflows = []models.Workflow{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workflows)
}

func (h *TicketHandler) GetByKey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}

	ticket, err := h.db.GetTicketByKey(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ticket == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ticket)
}

func (h *TicketHandler) MarkDone(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	ticket, err := h.db.GetTicketByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ticket == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	project, err := h.db.GetProject(ticket.ProjectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if project == nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}

	branchName := fmt.Sprintf("feature/%s", strings.ToLower(ticket.JiraKey))

	if project.RepoPath != "" {
		deleteLocal := exec.Command("git", "branch", "-D", branchName)
		deleteLocal.Dir = project.RepoPath
		deleteLocal.Run()

		deleteRemote := exec.Command("git", "push", "origin", "--delete", branchName)
		deleteRemote.Dir = project.RepoPath
		deleteRemote.Run()
	}

	if err := h.db.UpdateTicketStatus(id, models.TicketStatusDone); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ticket.Status = models.TicketStatusDone
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ticket)
}
