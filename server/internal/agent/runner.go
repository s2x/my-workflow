package agent

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/piotr-halas/decodo-workflow/internal/models"
)

type Runner struct {
	opencodeBin string
	qwenBin     string
	logger      *slog.Logger
	logWriter   LogWriter
}

type LogWriter interface {
	WriteLog(taskID string, level models.LogLevel, message string) error
}

func NewRunner(opencodeBin string, qwenBin string, logger *slog.Logger) *Runner {
	return &Runner{
		opencodeBin: opencodeBin,
		qwenBin:     qwenBin,
		logger:      logger,
	}
}

func (r *Runner) SetLogWriter(lw LogWriter) {
	r.logWriter = lw
}

type RunResult struct {
	Output   string
	ExitCode int
	Duration time.Duration
	Error    error
}

// gitCheckoutBranch switches to the specified branch
func (r *Runner) gitCheckoutBranch(repoPath string, branch string, taskID string) error {
	r.logger.Info("checking out git branch", "branch", branch, "repo", repoPath)

	cmd := exec.Command("git", "checkout", branch)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		logMsg := fmt.Sprintf("git checkout %s failed: %v\nOutput: %s", branch, err, string(output))
		if taskID != "" && r.logWriter != nil {
			_ = r.logWriter.WriteLog(taskID, models.LogLevelError, logMsg)
		}
		r.logger.Error("git checkout failed", "error", err, "output", string(output))
		return fmt.Errorf("failed to checkout branch %s: %w", branch, err)
	}

	logMsg := fmt.Sprintf("git checkout %s succeeded\nOutput: %s", branch, string(output))
	if taskID != "" && r.logWriter != nil {
		_ = r.logWriter.WriteLog(taskID, models.LogLevelInfo, logMsg)
	}
	r.logger.Info("git checkout successful", "branch", branch)

	return nil
}

// gitPullBranch fetches and pulls latest changes from remote
func (r *Runner) gitPullBranch(repoPath string, branch string, taskID string) error {
	r.logger.Info("pulling git branch", "branch", branch, "repo", repoPath)

	cmd := exec.Command("git", "pull", "origin", branch)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		logMsg := fmt.Sprintf("git pull origin %s failed: %v\nOutput: %s", branch, err, string(output))
		if taskID != "" && r.logWriter != nil {
			_ = r.logWriter.WriteLog(taskID, models.LogLevelError, logMsg)
		}
		r.logger.Warn("git pull failed (might be initial checkout)", "branch", branch, "error", err)
		return nil
	}

	logMsg := fmt.Sprintf("Pulled latest changes from origin/%s\nOutput: %s", branch, string(output))
	if taskID != "" && r.logWriter != nil {
		_ = r.logWriter.WriteLog(taskID, models.LogLevelInfo, logMsg)
	}
	r.logger.Info("git pull successful", "branch", branch)

	return nil
}

// gitEnsureBranch creates branch from current branch if it doesn't exist
func (r *Runner) gitEnsureBranch(repoPath string, branch string, taskID string) error {
	r.logger.Info("ensuring git branch exists", "branch", branch, "repo", repoPath)

	// Check if branch exists locally
	checkCmd := exec.Command("git", "rev-parse", "--verify", branch)
	checkCmd.Dir = repoPath
	err := checkCmd.Run()

	if err == nil {
		// Branch exists, just checkout
		return r.gitCheckoutBranch(repoPath, branch, taskID)
	}

	// Branch doesn't exist, create it
	r.logger.Info("branch does not exist, creating", "branch", branch)
	cmd := exec.Command("git", "checkout", "-b", branch)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		logMsg := fmt.Sprintf("git checkout -b %s failed: %v\nOutput: %s", branch, err, string(output))
		if taskID != "" && r.logWriter != nil {
			_ = r.logWriter.WriteLog(taskID, models.LogLevelError, logMsg)
		}
		r.logger.Error("failed to create branch", "error", err, "output", string(output))
		return fmt.Errorf("failed to create branch %s: %w", branch, err)
	}

	logMsg := fmt.Sprintf("Created and checked out new branch: %s", branch)
	if taskID != "" && r.logWriter != nil {
		_ = r.logWriter.WriteLog(taskID, models.LogLevelInfo, logMsg)
	}
	r.logger.Info("git branch created successfully", "branch", branch)

	return nil
}

func (r *Runner) Run(agentName string, prompt string, repoPath string, baseBranch string) RunResult {
	return r.RunWithTaskIDAndRunner(agentName, prompt, repoPath, "", "", baseBranch)
}

func (r *Runner) RunWithTaskID(agentName string, prompt string, repoPath string, taskID string) RunResult {
	return r.RunWithTaskIDAndRunner(agentName, prompt, repoPath, taskID, "", "")
}

func (r *Runner) RunWithTaskIDAndRunner(agentName string, prompt string, repoPath string, taskID string, runnerType string, baseBranch string) RunResult {
	start := time.Now()

	bin := r.qwenBin
	if runnerType == "opencode" {
		bin = r.opencodeBin
	}

	r.logger.Info("running agent", "agent", agentName, "prompt_len", len(prompt), "repo", repoPath, "task_id", taskID, "runner", runnerType, "base_branch", baseBranch)

	// Ensure we're on the correct base branch before running the agent
	if baseBranch != "" {
		if err := r.gitCheckoutBranch(repoPath, baseBranch, taskID); err != nil {
			return RunResult{Error: err}
		}
		if err := r.gitPullBranch(repoPath, baseBranch, taskID); err != nil {
			r.logger.Warn("git pull failed, continuing anyway", "error", err)
		}
		r.logger.Info("ready to run agent", "base_branch", baseBranch)
	}

	cmd := exec.Command(bin, "run",
		"--agent", agentName,
		prompt,
	)
	cmd.Dir = repoPath

	var outputBuffer bytes.Buffer
	var allOutput strings.Builder

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return RunResult{Error: fmt.Errorf("failed to create stdout pipe: %w", err)}
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return RunResult{Error: fmt.Errorf("failed to create stderr pipe: %w", err)}
	}

	if err := cmd.Start(); err != nil {
		return RunResult{Error: fmt.Errorf("failed to start command: %w", err)}
	}

	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})

	go r.streamLogs(stdoutPipe, taskID, models.LogLevelInfo, &outputBuffer, &allOutput, stdoutDone)
	go r.streamLogs(stderrPipe, taskID, models.LogLevelError, &outputBuffer, &allOutput, stderrDone)

	<-stdoutDone
	<-stderrDone

	err = cmd.Wait()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	output := strings.TrimSpace(allOutput.String())

	result := RunResult{
		Output:   output,
		ExitCode: exitCode,
		Duration: duration,
	}

	if err != nil && exitCode != 0 {
		result.Error = fmt.Errorf("agent %s failed (exit %d)", agentName, exitCode)
	}

	r.logger.Info("agent finished",
		"agent", agentName,
		"exit_code", exitCode,
		"duration", duration,
		"output_len", len(output),
	)

	return result
}

func (r *Runner) streamLogs(reader io.Reader, taskID string, level models.LogLevel, buffer *bytes.Buffer, allOutput *strings.Builder, done chan struct{}) {
	defer close(done)

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		buffer.WriteString(line + "\n")
		allOutput.WriteString(line + "\n")

		if taskID != "" && r.logWriter != nil {
			if err := r.logWriter.WriteLog(taskID, level, line); err != nil {
				r.logger.Error("failed to write log", "error", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		r.logger.Error("error reading stream", "error", err)
	}
}
