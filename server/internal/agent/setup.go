package agent

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

func SetupTargetRepo(repoPath string, logger *slog.Logger) error {
	agentsDir := filepath.Join(repoPath, ".opencode", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("creating .opencode/agents: %w", err)
	}

	entries, err := EmbeddedAgents.ReadDir("agents")
	if err != nil {
		return fmt.Errorf("reading embedded agents: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, err := EmbeddedAgents.ReadFile("agents/" + entry.Name())
		if err != nil {
			return fmt.Errorf("reading agent %s: %w", entry.Name(), err)
		}
		dst := filepath.Join(agentsDir, entry.Name())
		if err := os.WriteFile(dst, content, 0644); err != nil {
			return fmt.Errorf("writing agent %s: %w", entry.Name(), err)
		}
		logger.Info("deployed agent definition", "file", dst)
	}

	configDst := filepath.Join(repoPath, "opencode.json")
	if err := os.WriteFile(configDst, EmbeddedOpencodeJSON, 0644); err != nil {
		return fmt.Errorf("writing opencode.json: %w", err)
	}
	logger.Info("deployed opencode.json", "file", configDst)

	return nil
}

func SetupAllProjects(repoPaths []string, logger *slog.Logger) error {
	for _, p := range repoPaths {
		if err := SetupTargetRepo(p, logger); err != nil {
			return fmt.Errorf("setup %s: %w", p, err)
		}
	}
	return nil
}
