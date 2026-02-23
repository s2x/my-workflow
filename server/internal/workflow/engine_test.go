package workflow

import (
	"log/slog"
	"os"
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
