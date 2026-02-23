package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/piotr-halas/decodo-workflow/internal/config"
)

func TestConfigGetWithJiraURL(t *testing.T) {
	cfg := &config.Config{
		JiraURL: "https://example.atlassian.net",
	}
	handler := NewConfigHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["jira_url"] != "https://example.atlassian.net" {
		t.Errorf("Expected jira_url %q, got %q", "https://example.atlassian.net", resp["jira_url"])
	}
}

func TestConfigGetWithEmptyJiraURL(t *testing.T) {
	cfg := &config.Config{
		JiraURL: "",
	}
	handler := NewConfigHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["jira_url"] != "" {
		t.Errorf("Expected empty jira_url, got %q", resp["jira_url"])
	}
}
