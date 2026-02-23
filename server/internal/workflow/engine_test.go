package workflow

import (
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/piotr-halas/decodo-workflow/internal/agent"
	"github.com/piotr-halas/decodo-workflow/internal/db"
	"github.com/piotr-halas/decodo-workflow/internal/models"
)

func setupTestEngine(t *testing.T) (*Engine, *db.DB) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	runner := agent.NewRunner("opencode", "qwen", slog.New(slog.NewTextHandler(os.Stdout, nil)))
	engine := NewEngine(database, runner, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	return engine, database
}

func createTestProjectWithFlags(t *testing.T, db *db.DB, autoTest, autoReview bool) *models.Project {
	project := &models.Project{
		Name:       "test-project",
		RepoPath:   "/tmp/test",
		AutoTest:   autoTest,
		AutoReview: autoReview,
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}
	return project
}

func createTestTicket(t *testing.T, db *db.DB, projectID string) *models.Ticket {
	ticket := &models.Ticket{
		ProjectID: projectID,
		JiraKey:   "TEST-123",
		Summary:   "Test ticket",
	}
	if err := db.CreateTicket(ticket); err != nil {
		t.Fatalf("Failed to create test ticket: %v", err)
	}
	return ticket
}

func TestWorkflowSkipsTestWhenAutoTestDisabled(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, false, true)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowCoding,
		BranchName: "feature/test-123",
		Spec:       "{}",
	}
	database.CreateWorkflow(wf)

	pb := NewPromptBuilder(project.BaseBranch)
	engine.afterCode(wf, ticket, "", pb)

	wf, _ = database.GetWorkflow(wf.ID)
	if wf.Status != models.WorkflowReviewing {
		t.Errorf("Expected status WorkflowReviewing when AutoTest=false, got %s", wf.Status)
	}

	tasks, _ := database.GetTasksByWorkflow(wf.ID)
	for _, task := range tasks {
		if task.Type == models.TaskTest {
			t.Error("Expected no test task when AutoTest=false")
		}
	}
}

func TestWorkflowSkipsReviewWhenAutoReviewDisabled(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, true, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowTesting,
		BranchName: "feature/test-123",
		Spec:       "{}",
	}
	database.CreateWorkflow(wf)

	pb := NewPromptBuilder(project.BaseBranch)
	testOutput := `{"result":"PASS"}`
	engine.afterTest(wf, ticket, testOutput, pb)

	wf, _ = database.GetWorkflow(wf.ID)
	if wf.Status != models.WorkflowAwaitingApproval {
		t.Errorf("Expected status WorkflowAwaitingApproval when AutoReview=false, got %s", wf.Status)
	}

	tasks, _ := database.GetTasksByWorkflow(wf.ID)
	for _, task := range tasks {
		if task.Type == models.TaskReview {
			t.Error("Expected no review task when AutoReview=false")
		}
	}
}

func TestWorkflowCreatesTestTaskWhenAutoTestEnabled(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, true, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowCoding,
		BranchName: "feature/test-123",
		Spec:       "{}",
	}
	database.CreateWorkflow(wf)

	pb := NewPromptBuilder(project.BaseBranch)
	engine.afterCode(wf, ticket, "", pb)

	wf, _ = database.GetWorkflow(wf.ID)
	if wf.Status != models.WorkflowTesting {
		t.Errorf("Expected status WorkflowTesting when AutoTest=true, got %s", wf.Status)
	}

	tasks, _ := database.GetTasksByWorkflow(wf.ID)
	foundTestTask := false
	for _, task := range tasks {
		if task.Type == models.TaskTest {
			foundTestTask = true
			if task.Agent != "tester" {
				t.Errorf("Expected test task agent=tester, got %s", task.Agent)
			}
		}
	}

	if !foundTestTask {
		t.Error("Expected test task to be created when AutoTest=true")
	}
}

func TestWorkflowCreatesReviewTaskWhenAutoReviewEnabled(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, false, true)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowTesting,
		BranchName: "feature/test-123",
		Spec:       "{}",
	}
	database.CreateWorkflow(wf)

	pb := NewPromptBuilder(project.BaseBranch)
	testOutput := `{"result":"PASS"}`
	engine.afterTest(wf, ticket, testOutput, pb)

	wf, _ = database.GetWorkflow(wf.ID)
	if wf.Status != models.WorkflowReviewing {
		t.Errorf("Expected status WorkflowReviewing when AutoReview=true, got %s", wf.Status)
	}

	tasks, _ := database.GetTasksByWorkflow(wf.ID)
	foundReviewTask := false
	for _, task := range tasks {
		if task.Type == models.TaskReview {
			foundReviewTask = true
			if task.Agent != "reviewer" {
				t.Errorf("Expected review task agent=reviewer, got %s", task.Agent)
			}
		}
	}

	if !foundReviewTask {
		t.Error("Expected review task to be created when AutoReview=true")
	}
}

func TestWorkflowSkipsBothTestAndReviewWhenDisabled(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, false, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowCoding,
		BranchName: "feature/test-123",
		Spec:       "{}",
	}
	database.CreateWorkflow(wf)

	pb := NewPromptBuilder(project.BaseBranch)
	engine.afterCode(wf, ticket, "", pb)

	wf, _ = database.GetWorkflow(wf.ID)
	if wf.Status != models.WorkflowAwaitingApproval {
		t.Errorf("Expected status WorkflowAwaitingApproval when both flags=false, got %s", wf.Status)
	}

	tasks, _ := database.GetTasksByWorkflow(wf.ID)
	for _, task := range tasks {
		if task.Type == models.TaskTest || task.Type == models.TaskReview {
			t.Errorf("Expected no test/review tasks when both flags=false, got task type %s", task.Type)
		}
	}
}

func TestWorkflowExecutesBothTestAndReviewWhenEnabled(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, true, true)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowCoding,
		BranchName: "feature/test-123",
		Spec:       "{}",
	}
	database.CreateWorkflow(wf)

	pb := NewPromptBuilder(project.BaseBranch)

	engine.afterCode(wf, ticket, "", pb)
	wf, _ = database.GetWorkflow(wf.ID)
	if wf.Status != models.WorkflowTesting {
		t.Errorf("Expected status WorkflowTesting, got %s", wf.Status)
	}

	testOutput := `{"result":"PASS"}`
	engine.afterTest(wf, ticket, testOutput, pb)
	wf, _ = database.GetWorkflow(wf.ID)
	if wf.Status != models.WorkflowReviewing {
		t.Errorf("Expected status WorkflowReviewing, got %s", wf.Status)
	}

	tasks, _ := database.GetTasksByWorkflow(wf.ID)
	foundTest, foundReview := false, false
	for _, task := range tasks {
		if task.Type == models.TaskTest {
			foundTest = true
		}
		if task.Type == models.TaskReview {
			foundReview = true
		}
	}

	if !foundTest {
		t.Error("Expected test task when AutoTest=true")
	}
	if !foundReview {
		t.Error("Expected review task when AutoReview=true")
	}
}

func TestAfterDeploySetsDoneStatusOnTicket(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, false, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowDeploying,
		BranchName: "feature/test-123",
	}
	database.CreateWorkflow(wf)

	deployOutput := `{"result":"DEPLOYED"}`
	engine.afterDeploy(wf, deployOutput)

	updatedTicket, err := database.GetTicketByID(ticket.ID)
	if err != nil {
		t.Fatalf("GetTicketByID failed: %v", err)
	}
	if updatedTicket.Status != "done" {
		t.Errorf("Expected ticket status 'done' after afterDeploy, got %q", updatedTicket.Status)
	}
}

func TestAfterDeployDoesNotSetDoneWhenDeployFails(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, false, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowDeploying,
		BranchName: "feature/test-123",
	}
	database.CreateWorkflow(wf)

	deployOutput := `{"result":"FAILED"}`
	engine.afterDeploy(wf, deployOutput)

	updatedTicket, err := database.GetTicketByID(ticket.ID)
	if err != nil {
		t.Fatalf("GetTicketByID failed: %v", err)
	}
	if updatedTicket.Status == "done" {
		t.Error("Expected ticket status NOT 'done' when deployment fails")
	}
}

func TestRejectDeploymentTransitionsToCoding(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, false, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowAwaitingApproval,
		BranchName: "feature/test-123",
		Spec:       "{}",
		RetryCount: 0,
	}
	database.CreateWorkflow(wf)

	if err := engine.RejectDeployment(wf.ID, "Zły kod"); err != nil {
		t.Fatalf("RejectDeployment failed: %v", err)
	}

	wf, _ = database.GetWorkflow(wf.ID)
	if wf.Status != models.WorkflowCoding {
		t.Errorf("Expected status WorkflowCoding after reject, got %s", wf.Status)
	}
}

func TestRejectDeploymentCreatesFixTask(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, false, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowAwaitingApproval,
		BranchName: "feature/test-123",
		Spec:       "{}",
		RetryCount: 0,
	}
	database.CreateWorkflow(wf)

	comment := "Kod nie spełnia wymagań"
	if err := engine.RejectDeployment(wf.ID, comment); err != nil {
		t.Fatalf("RejectDeployment failed: %v", err)
	}

	tasks, _ := database.GetTasksByWorkflow(wf.ID)
	foundFixTask := false
	for _, task := range tasks {
		if task.Type == models.TaskFix {
			foundFixTask = true
			if task.Agent != "coder" {
				t.Errorf("Expected fix task agent=coder, got %s", task.Agent)
			}
		}
	}

	if !foundFixTask {
		t.Error("Expected fix task to be created after reject")
	}
}

func TestRejectDeploymentIncrementsRetryCount(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, false, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowAwaitingApproval,
		BranchName: "feature/test-123",
		Spec:       "{}",
		RetryCount: 0,
	}
	database.CreateWorkflow(wf)

	if err := engine.RejectDeployment(wf.ID, "Problem"); err != nil {
		t.Fatalf("RejectDeployment failed: %v", err)
	}

	wf, _ = database.GetWorkflow(wf.ID)
	if wf.RetryCount != 1 {
		t.Errorf("Expected RetryCount=1 after reject, got %d", wf.RetryCount)
	}
}

func TestRejectDeploymentFailsWhenMaxRetriesExceeded(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, false, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowAwaitingApproval,
		BranchName: "feature/test-123",
		Spec:       "{}",
		RetryCount: maxRetries,
	}
	database.CreateWorkflow(wf)

	if err := engine.RejectDeployment(wf.ID, "Za dużo prób"); err != nil {
		t.Fatalf("RejectDeployment failed: %v", err)
	}

	wf, _ = database.GetWorkflow(wf.ID)
	if wf.Status != models.WorkflowFailed {
		t.Errorf("Expected status WorkflowFailed when maxRetries exceeded, got %s", wf.Status)
	}
}

func TestRestartWorkflowFromFailed(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, false, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowFailed,
		BranchName: "",
		RetryCount: 2,
	}
	database.CreateWorkflow(wf)

	oldTask := &models.Task{
		WorkflowID: wf.ID,
		Type:       models.TaskCode,
		Status:     models.TaskFailed,
		Agent:      "coder",
		Prompt:     "old prompt",
	}
	database.CreateTask(oldTask)

	if err := engine.RestartWorkflow(wf.ID); err != nil {
		t.Fatalf("RestartWorkflow failed: %v", err)
	}

	wf, _ = database.GetWorkflow(wf.ID)
	if wf.Status != models.WorkflowDescribing {
		t.Errorf("Expected status DESCRIBING after restart, got %s", wf.Status)
	}
	if wf.RetryCount != 0 {
		t.Errorf("Expected retry_count=0 after restart, got %d", wf.RetryCount)
	}
	if wf.BranchName != "" {
		t.Errorf("Expected empty branch_name after restart, got %q", wf.BranchName)
	}

	tasks, _ := database.GetTasksByWorkflow(wf.ID)
	if len(tasks) != 1 {
		t.Fatalf("Expected exactly 1 task (DESCRIBE) after restart, got %d", len(tasks))
	}
	if tasks[0].Type != models.TaskDescribe {
		t.Errorf("Expected DESCRIBE task after restart, got %s", tasks[0].Type)
	}
}

func TestRestartWorkflowFromAwaitingApproval(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, false, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowAwaitingApproval,
		BranchName: "",
		RetryCount: 1,
	}
	database.CreateWorkflow(wf)

	if err := engine.RestartWorkflow(wf.ID); err != nil {
		t.Fatalf("RestartWorkflow failed: %v", err)
	}

	wf, _ = database.GetWorkflow(wf.ID)
	if wf.Status != models.WorkflowDescribing {
		t.Errorf("Expected status DESCRIBING, got %s", wf.Status)
	}
	if wf.RetryCount != 0 {
		t.Errorf("Expected retry_count=0, got %d", wf.RetryCount)
	}
}

func TestRestartWorkflowFromCoding(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, false, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowCoding,
		BranchName: "",
	}
	database.CreateWorkflow(wf)

	if err := engine.RestartWorkflow(wf.ID); err != nil {
		t.Fatalf("RestartWorkflow from CODING failed: %v", err)
	}

	wf, _ = database.GetWorkflow(wf.ID)
	if wf.Status != models.WorkflowDescribing {
		t.Errorf("Expected status DESCRIBING, got %s", wf.Status)
	}
}

func TestRestartWorkflowDeployingForbidden(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, false, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID: project.ID,
		TicketID:  ticket.ID,
		Status:    models.WorkflowDeploying,
	}
	database.CreateWorkflow(wf)

	err := engine.RestartWorkflow(wf.ID)
	if err == nil {
		t.Fatal("Expected error when restarting DEPLOYING workflow")
	}

	wf, _ = database.GetWorkflow(wf.ID)
	if wf.Status != models.WorkflowDeploying {
		t.Errorf("Expected workflow status unchanged (DEPLOYING), got %s", wf.Status)
	}
}

func TestRestartWorkflowDoneForbidden(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, false, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID: project.ID,
		TicketID:  ticket.ID,
		Status:    models.WorkflowDone,
	}
	database.CreateWorkflow(wf)

	err := engine.RestartWorkflow(wf.ID)
	if err == nil {
		t.Fatal("Expected error when restarting DONE workflow")
	}
}

func TestRestartWorkflowNonExistent(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	err := engine.RestartWorkflow("nonexistent-id")
	if err == nil {
		t.Fatal("Expected error for nonexistent workflow")
	}
}

func TestRestartWorkflowDeletesOldTasks(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, false, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID: project.ID,
		TicketID:  ticket.ID,
		Status:    models.WorkflowFailed,
	}
	database.CreateWorkflow(wf)

	for i := 0; i < 3; i++ {
		database.CreateTask(&models.Task{
			WorkflowID: wf.ID,
			Type:       models.TaskCode,
			Status:     models.TaskFailed,
			Agent:      "coder",
		})
	}

	if err := engine.RestartWorkflow(wf.ID); err != nil {
		t.Fatalf("RestartWorkflow failed: %v", err)
	}

	tasks, _ := database.GetTasksByWorkflow(wf.ID)
	for _, task := range tasks {
		if task.Type != models.TaskDescribe {
			t.Errorf("Expected only DESCRIBE task after restart, found %s", task.Type)
		}
	}
}

func TestRestartWorkflowAddsChatMessage(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, false, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID: project.ID,
		TicketID:  ticket.ID,
		Status:    models.WorkflowFailed,
	}
	database.CreateWorkflow(wf)

	if err := engine.RestartWorkflow(wf.ID); err != nil {
		t.Fatalf("RestartWorkflow failed: %v", err)
	}

	msgs, _ := database.GetChatMessages(wf.ID)
	if len(msgs) == 0 {
		t.Fatal("Expected chat message after restart, got none")
	}
	found := false
	for _, m := range msgs {
		if strings.Contains(m.Content, "restart") || strings.Contains(m.Content, "Restart") {
			found = true
		}
	}
	if !found {
		t.Error("Expected a chat message mentioning restart")
	}
}

func TestWorkflowTestFailureRetry(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := createTestProjectWithFlags(t, database, true, false)
	ticket := createTestTicket(t, database, project.ID)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowTesting,
		BranchName: "feature/test-123",
		Spec:       "{}",
		RetryCount: 0,
	}
	database.CreateWorkflow(wf)

	pb := NewPromptBuilder(project.BaseBranch)
	testOutput := `{"result":"FAIL","issues":["Test failed"]}`
	engine.afterTest(wf, ticket, testOutput, pb)

	wf, _ = database.GetWorkflow(wf.ID)
	if wf.RetryCount != 1 {
		t.Errorf("Expected RetryCount=1 after test failure, got %d", wf.RetryCount)
	}
	if wf.Status != models.WorkflowCoding {
		t.Errorf("Expected status WorkflowCoding after test failure, got %s", wf.Status)
	}

	tasks, _ := database.GetTasksByWorkflow(wf.ID)
	foundFixTask := false
	for _, task := range tasks {
		if task.Type == models.TaskFix {
			foundFixTask = true
		}
	}

	if !foundFixTask {
		t.Error("Expected fix task after test failure")
	}
}

func initGitRepoWithRemote(t *testing.T) (repoDir, remoteDir string) {
	t.Helper()

	remoteDir = t.TempDir()
	repoDir = t.TempDir()

	runCmd := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v in %s failed: %v\nOutput: %s", args, dir, err, string(out))
		}
	}

	runCmd(remoteDir, "git", "init", "--bare")

	runCmd(repoDir, "git", "init")
	runCmd(repoDir, "git", "config", "user.email", "test@test.com")
	runCmd(repoDir, "git", "config", "user.name", "Test")
	runCmd(repoDir, "git", "remote", "add", "origin", remoteDir)
	runCmd(repoDir, "git", "commit", "--allow-empty", "-m", "init")
	runCmd(repoDir, "git", "push", "-u", "origin", "master")

	return repoDir, remoteDir
}

func setupTestEngineWithRepo(t *testing.T) (*Engine, *db.DB, string) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	runner := agent.NewRunner("opencode", "qwen", slog.New(slog.NewTextHandler(os.Stdout, nil)))
	engine := NewEngine(database, runner, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	repoDir, _ := initGitRepoWithRemote(t)
	return engine, database, repoDir
}

func TestStartWorkflowPrepareBranchCalledWithCorrectName(t *testing.T) {
	engine, database, repoDir := setupTestEngineWithRepo(t)
	defer database.Close()

	project := &models.Project{
		Name:       "test-project",
		RepoPath:   repoDir,
		BaseBranch: "master",
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	ticket := &models.Ticket{
		ProjectID: project.ID,
		JiraKey:   "TEST-456",
		Summary:   "Test ticket",
	}
	if err := database.CreateTicket(ticket); err != nil {
		t.Fatalf("Failed to create ticket: %v", err)
	}

	wf, err := engine.StartWorkflow(project, ticket)
	if err != nil {
		t.Fatalf("StartWorkflow failed: %v", err)
	}

	expectedBranch := "feature/test-456"
	if wf.BranchName != expectedBranch {
		t.Errorf("Expected branch %q, got %q", expectedBranch, wf.BranchName)
	}

	dbWf, _ := database.GetWorkflow(wf.ID)
	if dbWf.BranchName != expectedBranch {
		t.Errorf("Expected DB branch %q, got %q", expectedBranch, dbWf.BranchName)
	}

	checkCmd := exec.Command("git", "rev-parse", "--verify", expectedBranch)
	checkCmd.Dir = repoDir
	if err := checkCmd.Run(); err != nil {
		t.Errorf("Expected branch %q to exist locally after StartWorkflow, got error: %v", expectedBranch, err)
	}
}

func TestStartWorkflowCreatesBranchBeforeDescribeTask(t *testing.T) {
	engine, database, repoDir := setupTestEngineWithRepo(t)
	defer database.Close()

	project := &models.Project{
		Name:       "test-project",
		RepoPath:   repoDir,
		BaseBranch: "master",
	}
	database.CreateProject(project)

	ticket := &models.Ticket{
		ProjectID: project.ID,
		JiraKey:   "LOCAL-999",
		Summary:   "Test ticket",
	}
	database.CreateTicket(ticket)

	wf, err := engine.StartWorkflow(project, ticket)
	if err != nil {
		t.Fatalf("StartWorkflow failed: %v", err)
	}

	if wf.BranchName == "" {
		t.Error("Expected branch name to be set before describe task")
	}

	if wf.Status != models.WorkflowDescribing {
		t.Errorf("Expected WorkflowDescribing status, got %s", wf.Status)
	}

	tasks, _ := database.GetTasksByWorkflow(wf.ID)
	if len(tasks) != 1 || tasks[0].Type != models.TaskDescribe {
		t.Errorf("Expected exactly one DESCRIBE task, got %d tasks", len(tasks))
	}
}

func TestStartWorkflowFailsWhenPrepareBranchFails(t *testing.T) {
	engine, database := setupTestEngine(t)
	defer database.Close()

	project := &models.Project{
		Name:       "test-project",
		RepoPath:   "/nonexistent/path",
		BaseBranch: "master",
	}
	database.CreateProject(project)

	ticket := &models.Ticket{
		ProjectID: project.ID,
		JiraKey:   "FAIL-001",
		Summary:   "Test ticket",
	}
	database.CreateTicket(ticket)

	_, err := engine.StartWorkflow(project, ticket)
	if err == nil {
		t.Fatal("Expected error when PrepareBranch fails, got nil")
	}
}

func TestApproveDeploymentDoesNotUseDeployerAgent(t *testing.T) {
	engine, database, repoDir := setupTestEngineWithRepo(t)
	defer database.Close()

	featureBranch := "feature/deploy-test"
	setupCmds := [][]string{
		{"git", "checkout", "-b", featureBranch},
		{"git", "commit", "--allow-empty", "-m", "feature commit"},
		{"git", "push", "-u", "origin", featureBranch},
		{"git", "checkout", "master"},
	}
	for _, args := range setupCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup command %v failed: %v\nOutput: %s", args, err, string(out))
		}
	}

	project := &models.Project{
		Name:       "test-project",
		RepoPath:   repoDir,
		BaseBranch: "master",
	}
	database.CreateProject(project)

	ticket := &models.Ticket{
		ProjectID: project.ID,
		JiraKey:   "DEPLOY-001",
		Summary:   "Test ticket",
	}
	database.CreateTicket(ticket)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowAwaitingApproval,
		BranchName: featureBranch,
	}
	database.CreateWorkflow(wf)

	if err := engine.ApproveDeployment(wf.ID); err != nil {
		t.Fatalf("ApproveDeployment failed: %v", err)
	}

	tasks, _ := database.GetTasksByWorkflow(wf.ID)
	for _, task := range tasks {
		if task.Agent == "deployer" {
			t.Error("Expected no task with agent 'deployer', but found one")
		}
		if task.Type == models.TaskDeploy && task.Agent != "system" {
			t.Errorf("Expected deploy task agent='system', got %q", task.Agent)
		}
	}
}

func TestApproveDeploymentSetsWorkflowDone(t *testing.T) {
	engine, database, repoDir := setupTestEngineWithRepo(t)
	defer database.Close()

	featureBranch := "feature/done-test"
	setupCmds := [][]string{
		{"git", "checkout", "-b", featureBranch},
		{"git", "commit", "--allow-empty", "-m", "feature commit"},
		{"git", "push", "-u", "origin", featureBranch},
		{"git", "checkout", "master"},
	}
	for _, args := range setupCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup command %v failed: %v\nOutput: %s", args, err, string(out))
		}
	}

	project := &models.Project{
		Name:       "test-project",
		RepoPath:   repoDir,
		BaseBranch: "master",
	}
	database.CreateProject(project)

	ticket := &models.Ticket{
		ProjectID: project.ID,
		JiraKey:   "DONE-001",
		Summary:   "Test ticket",
	}
	database.CreateTicket(ticket)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowAwaitingApproval,
		BranchName: featureBranch,
	}
	database.CreateWorkflow(wf)

	if err := engine.ApproveDeployment(wf.ID); err != nil {
		t.Fatalf("ApproveDeployment failed: %v", err)
	}

	updatedWf, _ := database.GetWorkflow(wf.ID)
	if updatedWf.Status != models.WorkflowDone {
		t.Errorf("Expected WorkflowDone after successful deploy, got %s", updatedWf.Status)
	}

	updatedTicket, _ := database.GetTicketByID(ticket.ID)
	if updatedTicket.Status != "done" {
		t.Errorf("Expected ticket status 'done', got %q", updatedTicket.Status)
	}
}

func TestApproveDeploymentSetsFailedOnMergeConflict(t *testing.T) {
	engine, database, repoDir := setupTestEngineWithRepo(t)
	defer database.Close()

	writeFile := func(dir, name, content string) {
		f, err := os.Create(dir + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		f.WriteString(content)
	}

	featureBranch := "feature/conflict-deploy"

	writeFile(repoDir, "file.txt", "original")
	setupCmds := [][]string{
		{"git", "add", "file.txt"},
		{"git", "commit", "-m", "add file"},
		{"git", "push", "origin", "master"},
		{"git", "checkout", "-b", featureBranch},
	}
	for _, args := range setupCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\nOutput: %s", args, err, string(out))
		}
	}

	writeFile(repoDir, "file.txt", "feature change")
	featureCmds := [][]string{
		{"git", "add", "file.txt"},
		{"git", "commit", "-m", "feature change"},
		{"git", "push", "-u", "origin", featureBranch},
		{"git", "checkout", "master"},
	}
	for _, args := range featureCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\nOutput: %s", args, err, string(out))
		}
	}

	writeFile(repoDir, "file.txt", "master conflicting change")
	masterCmds := [][]string{
		{"git", "add", "file.txt"},
		{"git", "commit", "-m", "master conflict"},
		{"git", "push", "origin", "master"},
	}
	for _, args := range masterCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\nOutput: %s", args, err, string(out))
		}
	}

	project := &models.Project{
		Name:       "test-project",
		RepoPath:   repoDir,
		BaseBranch: "master",
	}
	database.CreateProject(project)

	ticket := &models.Ticket{
		ProjectID: project.ID,
		JiraKey:   "CONFLICT-001",
		Summary:   "Test ticket",
	}
	database.CreateTicket(ticket)

	wf := &models.Workflow{
		ProjectID:  project.ID,
		TicketID:   ticket.ID,
		Status:     models.WorkflowAwaitingApproval,
		BranchName: featureBranch,
	}
	database.CreateWorkflow(wf)

	if err := engine.ApproveDeployment(wf.ID); err != nil {
		t.Fatalf("ApproveDeployment returned unexpected error: %v", err)
	}

	updatedWf, _ := database.GetWorkflow(wf.ID)
	if updatedWf.Status != models.WorkflowFailed {
		t.Errorf("Expected WorkflowFailed after merge conflict, got %s", updatedWf.Status)
	}

	abortCmd := exec.Command("git", "merge", "--abort")
	abortCmd.Dir = repoDir
	abortCmd.Run()
}
