package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/piotr-halas/decodo-workflow/internal/db"
	"github.com/piotr-halas/decodo-workflow/internal/models"
	"github.com/piotr-halas/decodo-workflow/internal/workflow"
)

type WorkflowHandler struct {
	db     *db.DB
	engine *workflow.Engine
}

func NewWorkflowHandler(db *db.DB, engine *workflow.Engine) *WorkflowHandler {
	return &WorkflowHandler{db: db, engine: engine}
}

type StartWorkflowRequest struct {
	TicketID  string `json:"ticket_id"`
	TicketKey string `json:"ticket_key"`
}

func (h *WorkflowHandler) Start(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	var req StartWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	var ticket *models.Ticket
	var err error

	if req.TicketID != "" {
		ticket, err = h.db.GetTicketByID(req.TicketID)
	} else if req.TicketKey != "" {
		ticket, err = h.db.GetTicketByKey(req.TicketKey)
	} else {
		http.Error(w, "ticket_id or ticket_key required", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ticket == nil {
		http.Error(w, "ticket not found", http.StatusNotFound)
		return
	}

	project, err := h.db.GetProject(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if project == nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}

	wf, err := h.engine.StartWorkflow(project, ticket)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(wf)
}

func (h *WorkflowHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	workflows, err := h.db.GetWorkflowsByProject(projectID)
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

func (h *WorkflowHandler) List(w http.ResponseWriter, r *http.Request) {
	workflows, err := h.db.GetWorkflows()
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

func (h *WorkflowHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	wf, err := h.db.GetWorkflow(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if wf == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wf)
}

func (h *WorkflowHandler) GetTasks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tasks, err := h.db.GetTasksByWorkflow(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tasks == nil {
		tasks = []models.Task{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func (h *WorkflowHandler) GetChat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	messages, err := h.db.GetChatMessages(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if messages == nil {
		messages = []models.ChatMessage{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

func (h *WorkflowHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.engine.ApproveDeployment(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "approved"})
}

func (h *WorkflowHandler) Restart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.engine.RestartWorkflow(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "restarted"})
}

type RejectWorkflowRequest struct {
	Comment string `json:"comment"`
}

func (h *WorkflowHandler) Reject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req RejectWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Comment == "" {
		http.Error(w, "comment is required", http.StatusBadRequest)
		return
	}

	if err := h.engine.RejectDeployment(id, req.Comment); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "rejected"})
}
