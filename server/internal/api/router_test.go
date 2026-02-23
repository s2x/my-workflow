package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/piotr-halas/decodo-workflow/internal/db"
)

func setupTestRouter(t *testing.T) http.Handler {
	t.Helper()
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewRouter(database, nil)
}

func TestRouterHealthEndpoint(t *testing.T) {
	router := setupTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET /api/health: expected 200, got %d", w.Code)
	}
}

func TestRouterAPIProjectsEndpoint(t *testing.T) {
	router := setupTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET /api/projects: expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("GET /api/projects: expected JSON content-type, got %s", ct)
	}
}

func TestRouterRootServesIndexHTML(t *testing.T) {
	router := setupTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET /: expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "<html") {
		t.Errorf("GET /: expected HTML body, got: %s", body[:min(100, len(body))])
	}
}

func TestRouterSPACatchAllProjectDetail(t *testing.T) {
	router := setupTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/projects/some-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET /projects/some-id: expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "<html") {
		t.Errorf("GET /projects/some-id: expected HTML body (index.html), got: %s", body[:min(100, len(body))])
	}
}

func TestRouterSPACatchAllWorkflowDetail(t *testing.T) {
	router := setupTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/projects/some-id/workflows/wf-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET /projects/some-id/workflows/wf-id: expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "<html") {
		t.Errorf("GET /projects/some-id/workflows/wf-id: expected HTML body (index.html), got: %s", body[:min(100, len(body))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
