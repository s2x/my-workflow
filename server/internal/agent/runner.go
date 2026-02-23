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

func (r *Runner) Run(agentName string, prompt string, repoPath string) RunResult {
	return r.RunWithTaskID(agentName, prompt, repoPath, "")
}

func (r *Runner) RunWithTaskID(agentName string, prompt string, repoPath string, taskID string) RunResult {
	return r.RunWithTaskIDAndRunner(agentName, prompt, repoPath, taskID, "")
}

func (r *Runner) RunWithTaskIDAndRunner(agentName string, prompt string, repoPath string, taskID string, runnerType string) RunResult {
	start := time.Now()

	bin := r.qwenBin
	if runnerType == "opencode" {
		bin = r.opencodeBin
	}

	r.logger.Info("running agent", "agent", agentName, "prompt_len", len(prompt), "repo", repoPath, "task_id", taskID, "runner", runnerType)

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
