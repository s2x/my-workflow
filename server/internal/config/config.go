package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	JiraEnabled      bool
	JiraURL          string
	JiraToken        string
	JiraEmail        string
	JiraSyncInterval time.Duration
	DBPath           string
	ServerPort       int
	OpencodeBin      string
	QwenBin          string
}

func Load() (*Config, error) {
	cfg := &Config{
		JiraURL:     getEnv("JIRA_URL", ""),
		JiraToken:   getEnv("JIRA_TOKEN", ""),
		JiraEmail:   getEnv("JIRA_EMAIL", ""),
		DBPath:      getEnv("DB_PATH", "decodo.db"),
		ServerPort:  getEnvInt("SERVER_PORT", 3000),
		OpencodeBin: getEnv("OPENCODE_BIN", "opencode"),
		QwenBin:     getEnv("QWEN_BIN", "qwen"),
	}

	interval := getEnvInt("JIRA_SYNC_INTERVAL_MIN", 10)
	cfg.JiraSyncInterval = time.Duration(interval) * time.Minute

	cfg.JiraEnabled = cfg.JiraURL != "" && cfg.JiraToken != "" && cfg.JiraEmail != ""

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
