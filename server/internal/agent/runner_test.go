package agent

import (
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/piotr-halas/decodo-workflow/internal/models"
)

type mockLogWriter struct {
	logs []mockLog
}

type mockLog struct {
	taskID  string
	level   models.LogLevel
	message string
}

func (m *mockLogWriter) WriteLog(taskID string, level models.LogLevel, message string) error {
	m.logs = append(m.logs, mockLog{
		taskID:  taskID,
		level:   level,
		message: message,
	})
	return nil
}

func TestRunnerWritesLogsLineByLine(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	runner := NewRunner("echo", logger)
	mockWriter := &mockLogWriter{}
	runner.SetLogWriter(mockWriter)

	taskID := "test-task-123"
	result := runner.RunWithTaskID("test", "line1\nline2\nline3", ".", taskID)

	if result.Error != nil {
		t.Fatalf("Runner failed: %v", result.Error)
	}

	if len(mockWriter.logs) == 0 {
		t.Fatal("Expected logs to be written, got none")
	}

	for _, log := range mockWriter.logs {
		if log.taskID != taskID {
			t.Errorf("Expected taskID %q, got %q", taskID, log.taskID)
		}
	}
}

func TestRunnerCapturesStdoutAndStderr(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	runner := NewRunner("echo", logger)
	mockWriter := &mockLogWriter{}
	runner.SetLogWriter(mockWriter)

	taskID := "test-task-456"
	result := runner.RunWithTaskID("test", "test output", ".", taskID)

	if result.Error != nil {
		t.Fatalf("Runner failed: %v", result.Error)
	}

	output := strings.ToLower(result.Output)
	if !strings.Contains(output, "test output") {
		t.Errorf("Expected output to contain 'test output', got: %q", result.Output)
	}

	hasInfo := false
	for _, log := range mockWriter.logs {
		if log.level == models.LogLevelInfo {
			hasInfo = true
		}
	}

	if !hasInfo {
		t.Error("Expected at least one info level log")
	}
}

func TestRunnerWithoutLogWriter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	runner := NewRunner("echo", logger)

	result := runner.RunWithTaskID("test", "test message", ".", "task-no-writer")

	if result.Error != nil {
		t.Fatalf("Runner failed: %v", result.Error)
	}

	if !strings.Contains(result.Output, "test message") {
		t.Errorf("Expected output to contain 'test message', got: %q", result.Output)
	}
}
