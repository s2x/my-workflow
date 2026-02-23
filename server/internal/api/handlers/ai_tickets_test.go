package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	aiservice "github.com/piotr-halas/decodo-workflow/internal/ai"
	"github.com/piotr-halas/decodo-workflow/internal/models"
)

type mockAIService struct {
	generateFn func(ctx context.Context, description string) (*models.GeneratedTicket, error)
	refineFn   func(ctx context.Context, currentDescription, refinementNotes string) (*models.GeneratedTicket, error)
}

func (m *mockAIService) GenerateTicket(ctx context.Context, description string) (*models.GeneratedTicket, error) {
	return m.generateFn(ctx, description)
}

func (m *mockAIService) RefineTicket(ctx context.Context, currentDescription, refinementNotes string) (*models.GeneratedTicket, error) {
	return m.refineFn(ctx, currentDescription, refinementNotes)
}

var _ aiservice.AITicketService = (*mockAIService)(nil)

func goodTicket() *models.GeneratedTicket {
	return &models.GeneratedTicket{
		Title:                "Test Ticket",
		BusinessDescription:  "Business value",
		TechnicalDescription: "Technical details",
		Priority:             "Medium",
		Complexity:           "moderate",
		AffectedComponents:   []string{"api", "db"},
	}
}

func setupAITicketHandler(svc aiservice.AITicketService) *AITicketHandler {
	return NewAITicketHandler(svc)
}

func TestGenerateReturns400WhenDescriptionTooShort(t *testing.T) {
	svc := &mockAIService{}
	h := setupAITicketHandler(svc)

	body, _ := json.Marshal(GenerateTicketRequest{Description: "short", ProjectID: "proj-1"})
	req := httptest.NewRequest(http.MethodPost, "/api/tickets/generate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleGenerateTicket(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestGenerateReturns400WhenDescriptionTooLong(t *testing.T) {
	svc := &mockAIService{}
	h := setupAITicketHandler(svc)

	desc := make([]byte, 2001)
	for i := range desc {
		desc[i] = 'a'
	}
	body, _ := json.Marshal(GenerateTicketRequest{Description: string(desc), ProjectID: "proj-1"})
	req := httptest.NewRequest(http.MethodPost, "/api/tickets/generate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleGenerateTicket(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestGenerateReturns400WhenMissingProjectID(t *testing.T) {
	svc := &mockAIService{}
	h := setupAITicketHandler(svc)

	body, _ := json.Marshal(GenerateTicketRequest{Description: "This is a valid description with enough chars"})
	req := httptest.NewRequest(http.MethodPost, "/api/tickets/generate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleGenerateTicket(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestGenerateReturns200WithValidJSON(t *testing.T) {
	svc := &mockAIService{
		generateFn: func(ctx context.Context, description string) (*models.GeneratedTicket, error) {
			return goodTicket(), nil
		},
	}
	h := setupAITicketHandler(svc)

	body, _ := json.Marshal(GenerateTicketRequest{Description: "This is a valid description with enough chars", ProjectID: "proj-1"})
	req := httptest.NewRequest(http.MethodPost, "/api/tickets/generate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleGenerateTicket(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result models.GeneratedTicket
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Title != "Test Ticket" {
		t.Errorf("Expected title %q, got %q", "Test Ticket", result.Title)
	}
	if result.BusinessDescription == "" {
		t.Error("Expected non-empty business_description")
	}
	if result.TechnicalDescription == "" {
		t.Error("Expected non-empty technical_description")
	}
	if result.Priority == "" {
		t.Error("Expected non-empty priority")
	}
	if result.Complexity == "" {
		t.Error("Expected non-empty complexity")
	}
}

func TestGenerateReturns504OnAITimeout(t *testing.T) {
	svc := &mockAIService{
		generateFn: func(ctx context.Context, description string) (*models.GeneratedTicket, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	h := setupAITicketHandler(svc)

	body, _ := json.Marshal(GenerateTicketRequest{Description: "Valid description with enough characters here", ProjectID: "proj-1"})
	req := httptest.NewRequest(http.MethodPost, "/api/tickets/generate", bytes.NewReader(body))

	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.HandleGenerateTicket(w, req)

	if w.Code != http.StatusGatewayTimeout {
		t.Errorf("Expected 504, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGenerateReturns429AfterRateLimitExceeded(t *testing.T) {
	svc := &mockAIService{
		generateFn: func(ctx context.Context, description string) (*models.GeneratedTicket, error) {
			return goodTicket(), nil
		},
	}
	h := setupAITicketHandler(svc)

	desc := "This is a valid description with enough characters"
	for i := 0; i < 10; i++ {
		body, _ := json.Marshal(GenerateTicketRequest{Description: desc, ProjectID: "proj-1"})
		req := httptest.NewRequest(http.MethodPost, "/api/tickets/generate", bytes.NewReader(body))
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		h.HandleGenerateTicket(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Request %d should have succeeded, got %d", i+1, w.Code)
		}
	}

	body, _ := json.Marshal(GenerateTicketRequest{Description: desc, ProjectID: "proj-1"})
	req := httptest.NewRequest(http.MethodPost, "/api/tickets/generate", bytes.NewReader(body))
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	h.HandleGenerateTicket(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 on 11th request, got %d", w.Code)
	}
}

func TestRefineReturns400WhenMissingCurrentDescription(t *testing.T) {
	svc := &mockAIService{}
	h := setupAITicketHandler(svc)

	body, _ := json.Marshal(RefineTicketRequest{RefinementNotes: "make it better"})
	req := httptest.NewRequest(http.MethodPost, "/api/tickets/refine", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleRefineTicket(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestRefineReturns400WhenMissingRefinementNotes(t *testing.T) {
	svc := &mockAIService{}
	h := setupAITicketHandler(svc)

	body, _ := json.Marshal(RefineTicketRequest{CurrentDescription: "Current description of ticket"})
	req := httptest.NewRequest(http.MethodPost, "/api/tickets/refine", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleRefineTicket(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestRefineReturns200WithValidJSON(t *testing.T) {
	svc := &mockAIService{
		refineFn: func(ctx context.Context, currentDescription, refinementNotes string) (*models.GeneratedTicket, error) {
			return goodTicket(), nil
		},
	}
	h := setupAITicketHandler(svc)

	body, _ := json.Marshal(RefineTicketRequest{CurrentDescription: "Current description", RefinementNotes: "Make it more technical"})
	req := httptest.NewRequest(http.MethodPost, "/api/tickets/refine", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleRefineTicket(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result models.GeneratedTicket
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Title == "" {
		t.Error("Expected non-empty title in response")
	}
}

func TestRefineReturns429AfterRateLimitExceeded(t *testing.T) {
	svc := &mockAIService{
		refineFn: func(ctx context.Context, currentDescription, refinementNotes string) (*models.GeneratedTicket, error) {
			return goodTicket(), nil
		},
	}
	h := setupAITicketHandler(svc)

	for i := 0; i < 20; i++ {
		body, _ := json.Marshal(RefineTicketRequest{CurrentDescription: "Current description", RefinementNotes: "notes"})
		req := httptest.NewRequest(http.MethodPost, "/api/tickets/refine", bytes.NewReader(body))
		req.RemoteAddr = "5.6.7.8:9999"
		w := httptest.NewRecorder()
		h.HandleRefineTicket(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Refine request %d should have succeeded, got %d", i+1, w.Code)
		}
	}

	body, _ := json.Marshal(RefineTicketRequest{CurrentDescription: "Current description", RefinementNotes: "notes"})
	req := httptest.NewRequest(http.MethodPost, "/api/tickets/refine", bytes.NewReader(body))
	req.RemoteAddr = "5.6.7.8:9999"
	w := httptest.NewRecorder()
	h.HandleRefineTicket(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 on 21st refine request, got %d", w.Code)
	}
}
