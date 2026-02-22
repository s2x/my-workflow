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
