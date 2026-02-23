package agent

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
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
	runner := NewRunner("qwen", "echo", logger)
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
	runner := NewRunner("qwen", "echo", logger)
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
	runner := NewRunner("qwen", "echo", logger)

	result := runner.RunWithTaskID("test", "test message", ".", "task-no-writer")

	if result.Error != nil {
		t.Fatalf("Runner failed: %v", result.Error)
	}

	if !strings.Contains(result.Output, "test message") {
		t.Errorf("Expected output to contain 'test message', got: %q", result.Output)
	}
}

func TestRunnerHandlesErrorInLogWriter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	runner := NewRunner("qwen", "echo", logger)
	mockWriter := &mockFailingLogWriter{}
	runner.SetLogWriter(mockWriter)

	taskID := "test-task-error"
	result := runner.RunWithTaskID("test", "test output", ".", taskID)

	if result.Error != nil {
		t.Fatalf("Runner should not fail when log writer fails: %v", result.Error)
	}

	if !strings.Contains(result.Output, "test output") {
		t.Errorf("Expected output to contain 'test output', got: %q", result.Output)
	}
}

func TestRunnerHandlesEmptyOutput(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	runner := NewRunner("qwen", "echo", logger)
	mockWriter := &mockLogWriter{}
	runner.SetLogWriter(mockWriter)

	taskID := "test-task-empty"
	result := runner.RunWithTaskID("test", "", ".", taskID)

	if result.Error != nil {
		t.Fatalf("Runner failed: %v", result.Error)
	}
}

func TestRunnerHandlesCommandFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	runner := NewRunner("qwen", "false", logger)
	mockWriter := &mockLogWriter{}
	runner.SetLogWriter(mockWriter)

	taskID := "test-task-fail"
	result := runner.RunWithTaskID("test", "test", ".", taskID)

	if result.Error == nil {
		t.Error("Expected error for failing command")
	}

	if result.ExitCode == 0 {
		t.Error("Expected non-zero exit code")
	}
}

type mockFailingLogWriter struct{}

func (m *mockFailingLogWriter) WriteLog(taskID string, level models.LogLevel, message string) error {
	return fmt.Errorf("mock write failed")
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init command %v failed: %v\nOutput: %s", args, err, string(out))
		}
	}
}

func TestDeployFailsWhenNotOnFeatureBranch(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	runner := NewRunner("", "", logger)
	mockWriter := &mockLogWriter{}
	runner.SetLogWriter(mockWriter)

	result := runner.Deploy(dir, "feature/my-feature", "master", "task-1")

	if result.Error == nil {
		t.Fatal("Expected error when not on feature branch, got nil")
	}
	if !strings.Contains(result.Error.Error(), "not a feature branch") {
		t.Errorf("Expected 'not a feature branch' error, got: %v", result.Error)
	}
}

func TestDeployFailsWhenOnMainBranch(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	runner := NewRunner("", "", logger)

	result := runner.Deploy(dir, "feature/my-feature", "master", "task-2")

	if result.Error == nil {
		t.Fatal("Expected error when on main branch, got nil")
	}
}

func TestDeploySuccessfulMerge(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	setupCmds := [][]string{
		{"git", "checkout", "-b", "feature/test-deploy"},
		{"git", "commit", "--allow-empty", "-m", "feature commit"},
		{"git", "checkout", "master"},
	}
	for _, args := range setupCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup command %v failed: %v\nOutput: %s", args, err, string(out))
		}
	}

	checkoutFeatureCmd := exec.Command("git", "checkout", "feature/test-deploy")
	checkoutFeatureCmd.Dir = dir
	if out, err := checkoutFeatureCmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout feature branch failed: %v\nOutput: %s", err, string(out))
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	runner := NewRunner("", "", logger)
	mockWriter := &mockLogWriter{}
	runner.SetLogWriter(mockWriter)

	customRunner := &deployTestRunner{Runner: runner, dir: dir}
	result := customRunner.deployWithoutPush("feature/test-deploy", "master", "task-3")

	if result.Error != nil {
		t.Fatalf("Expected successful deploy, got error: %v", result.Error)
	}

	currentBranchCmd := exec.Command("git", "branch", "--show-current")
	currentBranchCmd.Dir = dir
	out, err := currentBranchCmd.Output()
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	currentBranch := strings.TrimSpace(string(out))
	if currentBranch != "master" {
		t.Errorf("Expected to be on master after deploy, got: %s", currentBranch)
	}
}

func TestDeployFailsOnMergeConflict(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile := func(name, content string) {
		f, err := os.Create(fmt.Sprintf("%s/%s", dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		f.WriteString(content)
	}

	writeFile("file.txt", "original content")
	addCommit := [][]string{
		{"git", "add", "file.txt"},
		{"git", "commit", "-m", "add file"},
		{"git", "checkout", "-b", "feature/conflict-test"},
	}
	for _, args := range addCommit {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\nOutput: %s", args, err, string(out))
		}
	}

	writeFile("file.txt", "feature branch change")
	featureCommit := [][]string{
		{"git", "add", "file.txt"},
		{"git", "commit", "-m", "feature change"},
		{"git", "checkout", "master"},
	}
	for _, args := range featureCommit {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\nOutput: %s", args, err, string(out))
		}
	}

	writeFile("file.txt", "master branch conflicting change")
	masterCommit := [][]string{
		{"git", "add", "file.txt"},
		{"git", "commit", "-m", "master conflicting change"},
	}
	for _, args := range masterCommit {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\nOutput: %s", args, err, string(out))
		}
	}

	checkoutFeatureCmd := exec.Command("git", "checkout", "feature/conflict-test")
	checkoutFeatureCmd.Dir = dir
	if out, err := checkoutFeatureCmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout feature branch failed: %v\nOutput: %s", err, string(out))
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	runner := NewRunner("", "", logger)
	mockWriter := &mockLogWriter{}
	runner.SetLogWriter(mockWriter)

	customRunner := &deployTestRunner{Runner: runner, dir: dir}
	result := customRunner.deployWithoutPush("feature/conflict-test", "master", "task-4")

	if result.Error == nil {
		t.Fatal("Expected error on merge conflict, got nil")
	}
	if !strings.Contains(result.Error.Error(), "failed to merge") {
		t.Errorf("Expected 'failed to merge' error, got: %v", result.Error)
	}

	abortCmd := exec.Command("git", "merge", "--abort")
	abortCmd.Dir = dir
	abortCmd.Run()
}

func TestDeleteBranchFailsWithEmptyName(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	runner := NewRunner("", "", logger)

	err := runner.DeleteBranch("/tmp", "", "master")
	if err == nil {
		t.Fatal("Expected error for empty featureBranch")
	}
}

func TestDeleteBranchFailsWithNonFeaturePrefix(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	runner := NewRunner("", "", logger)

	err := runner.DeleteBranch("/tmp", "main", "master")
	if err == nil {
		t.Fatal("Expected error for branch not starting with 'feature/'")
	}
	if !strings.Contains(err.Error(), "feature/") {
		t.Errorf("Expected error mentioning 'feature/', got: %v", err)
	}
}

func TestDeleteBranchWhenBranchDoesNotExistLocally(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	runner := NewRunner("", "", logger)

	err := runner.DeleteBranch(dir, "feature/nonexistent", "master")
	if err != nil {
		t.Fatalf("Expected no error when branch does not exist locally, got: %v", err)
	}

	currentBranchCmd := exec.Command("git", "branch", "--show-current")
	currentBranchCmd.Dir = dir
	out, _ := currentBranchCmd.Output()
	currentBranch := strings.TrimSpace(string(out))
	if currentBranch != "feature/nonexistent" {
		t.Errorf("Expected to be on feature/nonexistent after DeleteBranch, got: %s", currentBranch)
	}
}

func TestDeleteBranchRecreatesFromBase(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	setupCmds := [][]string{
		{"git", "checkout", "-b", "feature/to-delete"},
		{"git", "commit", "--allow-empty", "-m", "feature commit"},
		{"git", "checkout", "master"},
	}
	for _, args := range setupCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup command %v failed: %v\nOutput: %s", args, err, string(out))
		}
	}

	checkoutFeature := exec.Command("git", "checkout", "feature/to-delete")
	checkoutFeature.Dir = dir
	if out, err := checkoutFeature.CombinedOutput(); err != nil {
		t.Fatalf("checkout feature failed: %v\nOutput: %s", err, string(out))
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	runner := NewRunner("", "", logger)

	err := runner.DeleteBranch(dir, "feature/to-delete", "master")
	if err != nil {
		t.Fatalf("DeleteBranch failed: %v", err)
	}

	currentBranchCmd := exec.Command("git", "branch", "--show-current")
	currentBranchCmd.Dir = dir
	out, _ := currentBranchCmd.Output()
	currentBranch := strings.TrimSpace(string(out))
	if currentBranch != "feature/to-delete" {
		t.Errorf("Expected to be on feature/to-delete after recreation, got: %s", currentBranch)
	}
}

type deployTestRunner struct {
	*Runner
	dir string
}

func (d *deployTestRunner) deployWithoutPush(featureBranch, baseBranch, taskID string) RunResult {
	currentBranchCmd := exec.Command("git", "branch", "--show-current")
	currentBranchCmd.Dir = d.dir
	currentBranchOutput, err := currentBranchCmd.Output()
	if err != nil {
		return RunResult{Error: fmt.Errorf("failed to get current branch: %w", err)}
	}
	currentBranch := strings.TrimSpace(string(currentBranchOutput))

	if !strings.HasPrefix(currentBranch, "feature/") {
		return RunResult{Error: fmt.Errorf("current branch %q is not a feature branch (must start with 'feature/')", currentBranch)}
	}

	if err := d.Runner.gitCheckoutBranch(d.dir, baseBranch, taskID); err != nil {
		return RunResult{Error: err}
	}

	mergeCmd := exec.Command("git", "merge", featureBranch)
	mergeCmd.Dir = d.dir
	mergeOutput, err := mergeCmd.CombinedOutput()
	if err != nil {
		return RunResult{Error: fmt.Errorf("failed to merge %s: %w", featureBranch, err), Output: string(mergeOutput)}
	}

	return RunResult{Output: fmt.Sprintf("Deploy completed: merged %s into %s", featureBranch, baseBranch)}
}
