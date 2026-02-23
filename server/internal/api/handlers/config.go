package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/piotr-halas/decodo-workflow/internal/config"
)

type ConfigHandler struct {
	cfg *config.Config
}

func NewConfigHandler(cfg *config.Config) *ConfigHandler {
	return &ConfigHandler{cfg: cfg}
}

func (h *ConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"jira_url": h.cfg.JiraURL,
	})
}
