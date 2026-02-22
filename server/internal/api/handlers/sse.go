package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/piotr-halas/decodo-workflow/internal/db"
)

type SSEHandler struct {
	db *db.DB
}

func NewSSEHandler(db *db.DB) *SSEHandler {
	return &SSEHandler{db: db}
}

func (h *SSEHandler) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			workflows, _ := h.db.GetWorkflows()
			data, _ := json.Marshal(map[string]any{
				"type":      "state",
				"workflows": workflows,
				"timestamp": time.Now(),
			})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
