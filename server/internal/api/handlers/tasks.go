package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/piotr-halas/decodo-workflow/internal/db"
	"github.com/piotr-halas/decodo-workflow/internal/models"
)

type TaskHandler struct {
	db *db.DB
}

func NewTaskHandler(db *db.DB) *TaskHandler {
	return &TaskHandler{db: db}
}

func (h *TaskHandler) StreamLogs(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	task, err := h.db.GetTask(taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if task == nil {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	historicalLogs, err := h.db.GetTaskLogs(taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, log := range historicalLogs {
		data, _ := json.Marshal(log)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	if task.Status != models.TaskRunning {
		return
	}

	lastTimestamp := time.Now()
	if len(historicalLogs) > 0 {
		lastTimestamp = historicalLogs[len(historicalLogs)-1].Timestamp
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(10 * time.Minute)

	for {
		select {
		case <-r.Context().Done():
			return
		case <-timeout:
			return
		case <-ticker.C:
			task, err := h.db.GetTask(taskID)
			if err != nil || task == nil {
				return
			}

			if task.Status != models.TaskRunning {
				return
			}

			newLogs, err := h.db.GetTaskLogsAfter(taskID, lastTimestamp)
			if err != nil {
				return
			}

			for _, log := range newLogs {
				data, _ := json.Marshal(log)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
				lastTimestamp = log.Timestamp
			}
		}
	}
}
