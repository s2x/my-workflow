package workflow

import (
	"log/slog"
	"os"
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
