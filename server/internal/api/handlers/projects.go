package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/piotr-halas/decodo-workflow/internal/db"
	"github.com/piotr-halas/decodo-workflow/internal/models"
)

type ProjectHandler struct {
	db          *db.DB
	opencodeBin string
	qwenBin     string
}

func NewProjectHandler(db *db.DB) *ProjectHandler {
	return &ProjectHandler{db: db}
}

func NewProjectHandlerWithBins(db *db.DB, opencodeBin, qwenBin string) *ProjectHandler {
	return &ProjectHandler{db: db, opencodeBin: opencodeBin, qwenBin: qwenBin}
}

type CreateProjectRequest struct {
	Name       string `json:"name"`
	RepoPath   string `json:"repo_path"`
	BaseBranch string `json:"base_branch"`
	AutoTest   bool   `json:"auto_test"`
	AutoReview bool   `json:"auto_review"`
	Runner     string `json:"runner"`
	Model      string `json:"model"`
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

	runner := req.Runner
	if runner == "" {
		runner = "qwen"
	}
	project := &models.Project{
		Name:       req.Name,
		RepoPath:   req.RepoPath,
		BaseBranch: req.BaseBranch,
		AutoTest:   req.AutoTest,
		AutoReview: req.AutoReview,
		Runner:     runner,
		Model:      req.Model,
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

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("projectId")
	found, err := h.db.DeleteProject(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	existing.AutoTest = req.AutoTest
	existing.AutoReview = req.AutoReview
	if req.Runner == "" {
		existing.Runner = "qwen"
	} else {
		existing.Runner = req.Runner
	}
	existing.Model = req.Model

	if err := h.db.UpdateProject(existing); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existing)
}

func (h *ProjectHandler) GetModels(w http.ResponseWriter, r *http.Request) {
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

	bin := h.qwenBin
	if project.Runner == "opencode" {
		bin = h.opencodeBin
	}
	if bin == "" {
		if project.Runner == "opencode" {
			bin = "opencode"
		} else {
			bin = "qwen"
		}
	}

	type modelsResponse struct {
		Models []string `json:"models"`
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if project.Runner == "opencode" {
		cmd = exec.CommandContext(ctx, bin, "models")
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(modelsResponse{Models: []string{}})
		return
	}
	stdout, err := cmd.Output()

	if err != nil || len(strings.TrimSpace(string(stdout))) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(modelsResponse{Models: []string{}})
		return
	}

	var modelsList []string
	scanner := bufio.NewScanner(strings.NewReader(string(stdout)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			modelsList = append(modelsList, line)
		}
	}

	if len(modelsList) == 0 {
		modelsList = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(modelsResponse{Models: modelsList})
}
