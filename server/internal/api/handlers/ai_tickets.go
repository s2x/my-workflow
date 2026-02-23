package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	aiservice "github.com/piotr-halas/decodo-workflow/internal/ai"
)

type AITicketHandler struct {
	aiService aiservice.AITicketService

	generateMu    sync.Mutex
	generateUsage map[string][]time.Time

	refineMu    sync.Mutex
	refineUsage map[string][]time.Time
}

func NewAITicketHandler(aiSvc aiservice.AITicketService) *AITicketHandler {
	return &AITicketHandler{
		aiService:     aiSvc,
		generateUsage: make(map[string][]time.Time),
		refineUsage:   make(map[string][]time.Time),
	}
}

func (h *AITicketHandler) checkRateLimit(mu *sync.Mutex, usage map[string][]time.Time, key string, limit int) bool {
	mu.Lock()
	defer mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-1 * time.Hour)

	times := usage[key]
	var recent []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= limit {
		usage[key] = recent
		return false
	}

	usage[key] = append(recent, now)
	return true
}

type GenerateTicketRequest struct {
	Description string `json:"description"`
	ProjectID   string `json:"project_id"`
}

type RefineTicketRequest struct {
	CurrentDescription string `json:"current_description"`
	RefinementNotes    string `json:"refinement_notes"`
}

func (h *AITicketHandler) HandleGenerateTicket(w http.ResponseWriter, r *http.Request) {
	var req GenerateTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ProjectID == "" {
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}

	if len(req.Description) < 10 {
		http.Error(w, "description must be at least 10 characters", http.StatusBadRequest)
		return
	}
	if len(req.Description) > 2000 {
		http.Error(w, "description must not exceed 2000 characters", http.StatusBadRequest)
		return
	}

	clientIP := r.RemoteAddr
	if !h.checkRateLimit(&h.generateMu, h.generateUsage, clientIP, 10) {
		http.Error(w, "rate limit exceeded: max 10 generate requests per hour", http.StatusTooManyRequests)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	ticket, err := h.aiService.GenerateTicket(ctx, req.Description)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
			http.Error(w, "AI request timed out", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "AI generation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ticket)
}

func (h *AITicketHandler) HandleRefineTicket(w http.ResponseWriter, r *http.Request) {
	var req RefineTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.CurrentDescription == "" {
		http.Error(w, "current_description is required", http.StatusBadRequest)
		return
	}

	if req.RefinementNotes == "" {
		http.Error(w, "refinement_notes is required", http.StatusBadRequest)
		return
	}

	clientIP := r.RemoteAddr
	if !h.checkRateLimit(&h.refineMu, h.refineUsage, clientIP, 20) {
		http.Error(w, "rate limit exceeded: max 20 refine requests per hour", http.StatusTooManyRequests)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	ticket, err := h.aiService.RefineTicket(ctx, req.CurrentDescription, req.RefinementNotes)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
			http.Error(w, "AI request timed out", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "AI refinement failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ticket)
}
