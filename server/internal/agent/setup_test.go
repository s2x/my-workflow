package agent

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

type AgentConfig struct {
	Mode        string `json:"mode"`
	Description string `json:"description"`
	Model       string `json:"model"`
}

type OpencodeConfig struct {
	Agent map[string]AgentConfig `json:"agent"`
}

func TestEmbeddedOpencodeJSONHasPrimaryAgents(t *testing.T) {
	var config OpencodeConfig
	if err := json.Unmarshal(EmbeddedOpencodeJSON, &config); err != nil {
		t.Fatalf("Failed to parse embedded opencode.json: %v", err)
	}

	requiredAgents := []string{"descriptor", "coder", "tester", "reviewer", "deployer"}

	for _, agentName := range requiredAgents {
		agent, ok := config.Agent[agentName]
		if !ok {
			t.Errorf("Agent %q not found in config", agentName)
			continue
		}

		if agent.Mode != "primary" {
			t.Errorf("Agent %q has mode %q, expected 'primary'", agentName, agent.Mode)
		}
	}
}

func TestSetupTargetRepoCreatesAgentsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	err := SetupTargetRepo(tmpDir, logger)
	if err != nil {
		t.Fatalf("SetupTargetRepo failed: %v", err)
	}

	agentsDir := filepath.Join(tmpDir, ".opencode", "agents")
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		t.Errorf("Expected agents directory to be created at %s", agentsDir)
	}
}

func TestSetupTargetRepoCreatesOpencodeJSON(t *testing.T) {
	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	err := SetupTargetRepo(tmpDir, logger)
	if err != nil {
		t.Fatalf("SetupTargetRepo failed: %v", err)
	}

	configPath := filepath.Join(tmpDir, "opencode.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Expected opencode.json to be created at %s", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read opencode.json: %v", err)
	}

	var config OpencodeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse opencode.json: %v", err)
	}

	if len(config.Agent) == 0 {
		t.Error("Expected at least one agent in config")
	}
}

func TestSetupTargetRepoDeploysAgentFiles(t *testing.T) {
	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	err := SetupTargetRepo(tmpDir, logger)
	if err != nil {
		t.Fatalf("SetupTargetRepo failed: %v", err)
	}

	agentFiles := []string{"descriptor.md", "coder.md", "tester.md", "reviewer.md", "deployer.md"}
	for _, agentFile := range agentFiles {
		agentPath := filepath.Join(tmpDir, ".opencode", "agents", agentFile)
		if _, err := os.Stat(agentPath); os.IsNotExist(err) {
			t.Errorf("Expected agent file %s to be deployed", agentFile)
		}
	}
}

func TestSetupAllProjectsHandlesMultipleRepos(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	err := SetupAllProjects([]string{tmpDir1, tmpDir2}, logger)
	if err != nil {
		t.Fatalf("SetupAllProjects failed: %v", err)
	}

	for _, dir := range []string{tmpDir1, tmpDir2} {
		configPath := filepath.Join(dir, "opencode.json")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Errorf("Expected opencode.json in %s", dir)
		}
	}
}
