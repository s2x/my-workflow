package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/piotr-halas/decodo-workflow/internal/db"
	"github.com/piotr-halas/decodo-workflow/internal/models"
)

type ProjectHandler struct {
	db *db.DB
}

func NewProjectHandler(db *db.DB) *ProjectHandler {
	return &ProjectHandler{db: db}
}

type CreateProjectRequest struct {
	Name        string `json:"name"`
	RepoPath    string `json:"repo_path"`
	BaseBranch  string `json:"base_branch"`
	StageBranch string `json:"stage_branch"`
	AutoTest    bool   `json:"auto_test"`
	AutoReview  bool   `json:"auto_review"`
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.RepoPath == "" {
		http.Error(w, "name and repo_path are required", http.StatusBadRequest)
		return
	}

	project := &models.Project{
		Name:        req.Name,
		RepoPath:    req.RepoPath,
		BaseBranch:  req.BaseBranch,
		StageBranch: req.StageBranch,
		AutoTest:    req.AutoTest,
		AutoReview:  req.AutoReview,
	}

	if err := h.db.CreateProject(project); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(project)
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	projects, err := h.db.GetProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if projects == nil {
		projects = []models.Project{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(projects)
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("projectId")
	project, err := h.db.GetProject(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if project == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(project)
}

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("projectId")
	existing, err := h.db.GetProject(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.RepoPath != "" {
		existing.RepoPath = req.RepoPath
	}
	if req.BaseBranch != "" {
		existing.BaseBranch = req.BaseBranch
	}
	if req.StageBranch != "" {
		existing.StageBranch = req.StageBranch
	}
	existing.AutoTest = req.AutoTest
	existing.AutoReview = req.AutoReview

	if err := h.db.UpdateProject(existing); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existing)
}
