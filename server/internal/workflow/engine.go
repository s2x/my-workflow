package workflow

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/piotr-halas/decodo-workflow/internal/agent"
	"github.com/piotr-halas/decodo-workflow/internal/db"
	"github.com/piotr-halas/decodo-workflow/internal/models"
)

const maxRetries = 3

type Engine struct {
	db     *db.DB
	runner *agent.Runner
	logger *slog.Logger
}

func NewEngine(database *db.DB, runner *agent.Runner, logger *slog.Logger) *Engine {
	e := &Engine{
		db:     database,
		runner: runner,
		logger: logger,
	}
	runner.SetLogWriter(e)
	return e
}

func (e *Engine) WriteLog(taskID string, level models.LogLevel, message string) error {
	return e.db.CreateTaskLog(&models.TaskLog{
		TaskID:   taskID,
		LogLevel: level,
		Message:  message,
	})
}

func (e *Engine) StartWorkflow(project *models.Project, ticket *models.Ticket) (*models.Workflow, error) {
	wf := &models.Workflow{
		ProjectID: project.ID,
		TicketID:  ticket.ID,
		Status:    models.WorkflowCreated,
	}
	if err := e.db.CreateWorkflow(wf); err != nil {
		return nil, fmt.Errorf("creating workflow: %w", err)
	}

	e.chatMsg(wf.ID, "system", "", fmt.Sprintf("Workflow started for ticket %s: %s", ticket.JiraKey, ticket.Summary))

	pb := NewPromptBuilder(project.BaseBranch)
	prompt := pb.BuildDescribePrompt(ticket)
	task := &models.Task{
		WorkflowID: wf.ID,
		Type:       models.TaskDescribe,
		Status:     models.TaskQueued,
		Agent:      "descriptor",
		Prompt:     prompt,
	}
	if err := e.db.CreateTask(task); err != nil {
		return nil, fmt.Errorf("creating describe task: %w", err)
	}

	if err := e.db.UpdateWorkflowStatus(wf.ID, models.WorkflowDescribing, ""); err != nil {
		return nil, err
	}
	wf.Status = models.WorkflowDescribing

	return wf, nil
}

func (e *Engine) ProcessQueuedTasks() {
	tasks, err := e.db.GetQueuedTasks()
	if err != nil {
		e.logger.Error("failed to get queued tasks", "error", err)
		return
	}

	for _, task := range tasks {
		e.processTask(task)
	}
}

func (e *Engine) getProjectForWorkflow(workflowID string) (*models.Project, error) {
	wf, err := e.db.GetWorkflow(workflowID)
	if err != nil || wf == nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}
	project, err := e.db.GetProject(wf.ProjectID)
	if err != nil || project == nil {
		return nil, fmt.Errorf("project not found for workflow: %s", workflowID)
	}
	return project, nil
}

func (e *Engine) processTask(task models.Task) {
	e.logger.Info("processing task", "task_id", task.ID, "type", task.Type, "agent", task.Agent)

	if err := e.db.UpdateTaskStatus(task.ID, models.TaskRunning); err != nil {
		e.logger.Error("failed to update task status", "error", err)
		return
	}

	e.chatMsg(task.WorkflowID, "agent", task.Agent, fmt.Sprintf("Agent **%s** started working on %s task...", task.Agent, task.Type))

	project, err := e.getProjectForWorkflow(task.WorkflowID)
	if err != nil {
		e.logger.Error("failed to get project for task", "error", err)
		e.db.UpdateTaskError(task.ID, err.Error())
		e.handleTaskFailure(task)
		return
	}

	result := e.runner.RunWithTaskID(task.Agent, task.Prompt, project.RepoPath, task.ID)

	if result.Error != nil {
		e.logger.Error("agent failed", "agent", task.Agent, "error", result.Error)
		e.db.UpdateTaskError(task.ID, result.Error.Error())
		e.chatMsg(task.WorkflowID, "agent", task.Agent, fmt.Sprintf("Agent **%s** failed: %s", task.Agent, result.Error))
		e.handleTaskFailure(task)
		return
	}

	e.db.UpdateTaskOutput(task.ID, result.Output, result.Output)
	e.db.UpdateTaskStatus(task.ID, models.TaskCompleted)
	e.chatMsg(task.WorkflowID, "agent", task.Agent, fmt.Sprintf("Agent **%s** completed %s task (took %s)", task.Agent, task.Type, result.Duration.Round(time.Second)))

	e.handleTaskCompletion(task, result.Output)
}

func (e *Engine) handleTaskCompletion(task models.Task, output string) {
	wf, err := e.db.GetWorkflow(task.WorkflowID)
	if err != nil || wf == nil {
		e.logger.Error("workflow not found", "workflow_id", task.WorkflowID)
		return
	}

	ticket, _ := e.db.GetTicketByID(wf.TicketID)
	if ticket == nil {
		e.logger.Error("ticket not found", "ticket_id", wf.TicketID)
		return
	}

	project, _ := e.db.GetProject(wf.ProjectID)
	if project == nil {
		e.logger.Error("project not found", "project_id", wf.ProjectID)
		return
	}

	pb := NewPromptBuilder(project.BaseBranch)

	if task.ParentTaskID != "" {
		e.handleSubtaskCompletion(task)
		return
	}

	switch task.Type {
	case models.TaskDescribe:
		e.afterDescribe(wf, ticket, output, pb)
	case models.TaskCode:
		e.afterCode(wf, ticket, output, pb)
	case models.TaskTest:
		e.afterTest(wf, ticket, output, pb)
	case models.TaskReview:
		e.afterReview(wf, ticket, output, pb)
	case models.TaskDeploy:
		e.afterDeploy(wf, output)
	case models.TaskFix:
		e.afterCode(wf, ticket, output, pb)
	}
}

func (e *Engine) afterDescribe(wf *models.Workflow, ticket *models.Ticket, output string, pb *PromptBuilder) {
	e.db.UpdateWorkflowSpec(wf.ID, output)

	var spec struct {
		Complexity string `json:"complexity"`
		Subtasks   []struct {
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Files       []string `json:"files"`
			Type        string   `json:"type"`
		} `json:"subtasks"`
	}

	if err := json.Unmarshal([]byte(output), &spec); err == nil && len(spec.Subtasks) > 1 && spec.Complexity != "simple" {
		e.chatMsg(wf.ID, "system", "", fmt.Sprintf("Task decomposed into %d subtasks", len(spec.Subtasks)))

		parentTask := &models.Task{
			WorkflowID: wf.ID,
			Type:       models.TaskCode,
			Status:     models.TaskRunning,
			Agent:      "coder",
		}
		e.db.CreateTask(parentTask)

		for _, st := range spec.Subtasks {
			subtaskPrompt := pb.BuildCodePrompt(ticket, output, fmt.Sprintf("### %s\n%s\nFiles: %s", st.Title, st.Description, strings.Join(st.Files, ", ")))
			sub := &models.Task{
				WorkflowID:   wf.ID,
				ParentTaskID: parentTask.ID,
				Type:         models.TaskCode,
				Status:       models.TaskQueued,
				Agent:        "coder",
				Prompt:       subtaskPrompt,
			}
			e.db.CreateTask(sub)
		}

		e.db.UpdateWorkflowStatus(wf.ID, models.WorkflowCoding, "")
		return
	}

	prompt := pb.BuildCodePrompt(ticket, output, "")
	codeTask := &models.Task{
		WorkflowID: wf.ID,
		Type:       models.TaskCode,
		Status:     models.TaskQueued,
		Agent:      "coder",
		Prompt:     prompt,
	}
	e.db.CreateTask(codeTask)
	e.db.UpdateWorkflowStatus(wf.ID, models.WorkflowCoding, "")
}

func (e *Engine) afterCode(wf *models.Workflow, ticket *models.Ticket, output string, pb *PromptBuilder) {
	branchName := fmt.Sprintf("feature/%s", strings.ToLower(ticket.JiraKey))
	e.db.UpdateWorkflowBranch(wf.ID, branchName)

	project, _ := e.db.GetProject(wf.ProjectID)
	if project == nil {
		e.logger.Error("project not found", "project_id", wf.ProjectID)
		return
	}

	if !project.AutoTest {
		e.chatMsg(wf.ID, "system", "", "Auto-testing disabled, skipping test phase")
		e.afterTest(wf, ticket, `{"result":"SKIP"}`, pb)
		return
	}

	prompt := pb.BuildTestPrompt(ticket, wf.Spec, branchName)
	testTask := &models.Task{
		WorkflowID: wf.ID,
		Type:       models.TaskTest,
		Status:     models.TaskQueued,
		Agent:      "tester",
		Prompt:     prompt,
	}
	e.db.CreateTask(testTask)
	e.db.UpdateWorkflowStatus(wf.ID, models.WorkflowTesting, "")
}

func (e *Engine) afterTest(wf *models.Workflow, ticket *models.Ticket, output string, pb *PromptBuilder) {
	var testResult struct {
		Result string   `json:"result"`
		Issues []string `json:"issues"`
	}

	if err := json.Unmarshal([]byte(output), &testResult); err == nil && testResult.Result == "FAIL" {
		if wf.RetryCount >= maxRetries {
			e.db.UpdateWorkflowStatus(wf.ID, models.WorkflowFailed, "max retries exceeded after test failures")
			e.chatMsg(wf.ID, "system", "", "Workflow failed: max retries exceeded")
			return
		}

		e.db.IncrementWorkflowRetry(wf.ID)
		issues := strings.Join(testResult.Issues, "\n- ")
		prompt := pb.BuildFixPrompt(ticket, wf.Spec, wf.BranchName, issues)
		fixTask := &models.Task{
			WorkflowID: wf.ID,
			Type:       models.TaskFix,
			Status:     models.TaskQueued,
			Agent:      "coder",
			Prompt:     prompt,
		}
		e.db.CreateTask(fixTask)
		e.db.UpdateWorkflowStatus(wf.ID, models.WorkflowCoding, "")
		e.chatMsg(wf.ID, "system", "", fmt.Sprintf("Tests failed, sending back to coder for fixes (retry %d/%d)", wf.RetryCount+1, maxRetries))
		return
	}

	project, _ := e.db.GetProject(wf.ProjectID)
	if project == nil {
		e.logger.Error("project not found", "project_id", wf.ProjectID)
		return
	}

	if !project.AutoReview {
		e.chatMsg(wf.ID, "system", "", "Auto-review disabled, skipping review phase")
		e.db.UpdateWorkflowStatus(wf.ID, models.WorkflowAwaitingApproval, "")
		e.chatMsg(wf.ID, "system", "", fmt.Sprintf("Code is ready for deployment on branch **%s**. Awaiting your approval.", wf.BranchName))
		return
	}

	prompt := pb.BuildReviewPrompt(ticket, wf.BranchName)
	reviewTask := &models.Task{
		WorkflowID: wf.ID,
		Type:       models.TaskReview,
		Status:     models.TaskQueued,
		Agent:      "reviewer",
		Prompt:     prompt,
	}
	e.db.CreateTask(reviewTask)
	e.db.UpdateWorkflowStatus(wf.ID, models.WorkflowReviewing, "")
}

func (e *Engine) afterReview(wf *models.Workflow, ticket *models.Ticket, output string, pb *PromptBuilder) {
	var reviewResult struct {
		Result   string `json:"result"`
		Comments []struct {
			Severity string `json:"severity"`
			Message  string `json:"message"`
		} `json:"comments"`
		Summary string `json:"summary"`
	}

	if err := json.Unmarshal([]byte(output), &reviewResult); err == nil && reviewResult.Result == "CHANGES_REQUESTED" {
		hasCritical := false
		var issues []string
		for _, c := range reviewResult.Comments {
			if c.Severity == "critical" {
				hasCritical = true
			}
			issues = append(issues, fmt.Sprintf("[%s] %s", c.Severity, c.Message))
		}

		if hasCritical {
			if wf.RetryCount >= maxRetries {
				e.db.UpdateWorkflowStatus(wf.ID, models.WorkflowFailed, "max retries exceeded after review")
				e.chatMsg(wf.ID, "system", "", "Workflow failed: max retries exceeded after critical review issues")
				return
			}

			e.db.IncrementWorkflowRetry(wf.ID)
			prompt := pb.BuildFixPrompt(ticket, wf.Spec, wf.BranchName, strings.Join(issues, "\n"))
			fixTask := &models.Task{
				WorkflowID: wf.ID,
				Type:       models.TaskFix,
				Status:     models.TaskQueued,
				Agent:      "coder",
				Prompt:     prompt,
			}
			e.db.CreateTask(fixTask)
			e.db.UpdateWorkflowStatus(wf.ID, models.WorkflowCoding, "")
			e.chatMsg(wf.ID, "system", "", fmt.Sprintf("Review found critical issues, sending back to coder (retry %d/%d)", wf.RetryCount+1, maxRetries))
			return
		}
	}

	e.db.UpdateWorkflowStatus(wf.ID, models.WorkflowAwaitingApproval, "")
	e.chatMsg(wf.ID, "system", "", fmt.Sprintf("Code is ready for deployment on branch **%s**. Awaiting your approval.", wf.BranchName))
}

func (e *Engine) afterDeploy(wf *models.Workflow, output string) {
	var deployResult struct {
		Result string `json:"result"`
	}

	if err := json.Unmarshal([]byte(output), &deployResult); err == nil && deployResult.Result == "DEPLOYED" {
		e.db.UpdateWorkflowStatus(wf.ID, models.WorkflowDone, "")
		e.chatMsg(wf.ID, "system", "", "Deployment successful!")
	} else {
		e.db.UpdateWorkflowStatus(wf.ID, models.WorkflowFailed, "deployment failed")
		e.chatMsg(wf.ID, "system", "", "Deployment failed. Manual intervention required.")
	}
}

func (e *Engine) handleSubtaskCompletion(task models.Task) {
	allDone, _ := e.db.AllSubtasksCompleted(task.ParentTaskID)
	anyFailed, _ := e.db.AnySubtaskFailed(task.ParentTaskID)

	if anyFailed {
		e.db.UpdateTaskStatus(task.ParentTaskID, models.TaskFailed)
		e.handleTaskFailure(task)
		return
	}

	if allDone {
		e.db.UpdateTaskStatus(task.ParentTaskID, models.TaskCompleted)
		parentTask := models.Task{
			ID:         task.ParentTaskID,
			WorkflowID: task.WorkflowID,
			Type:       task.Type,
			Agent:      task.Agent,
		}
		e.handleTaskCompletion(parentTask, "")
	}
}

func (e *Engine) handleTaskFailure(task models.Task) {
	wf, err := e.db.GetWorkflow(task.WorkflowID)
	if err != nil || wf == nil {
		return
	}

	if wf.RetryCount >= maxRetries {
		e.db.UpdateWorkflowStatus(wf.ID, models.WorkflowFailed, fmt.Sprintf("task %s failed after max retries", task.Type))
		e.chatMsg(wf.ID, "system", "", "Workflow failed: max retries exceeded")
		return
	}

	e.db.IncrementWorkflowRetry(wf.ID)
	retryTask := &models.Task{
		WorkflowID: task.WorkflowID,
		Type:       task.Type,
		Status:     models.TaskQueued,
		Agent:      task.Agent,
		Prompt:     task.Prompt,
	}
	e.db.CreateTask(retryTask)
	e.chatMsg(wf.ID, "system", "", fmt.Sprintf("Task %s failed, retrying (%d/%d)...", task.Type, wf.RetryCount+1, maxRetries))
}

func (e *Engine) ApproveDeployment(workflowID string) error {
	wf, err := e.db.GetWorkflow(workflowID)
	if err != nil {
		return err
	}
	if wf == nil {
		return fmt.Errorf("workflow not found")
	}
	if wf.Status != models.WorkflowAwaitingApproval {
		return fmt.Errorf("workflow is not awaiting approval (status: %s)", wf.Status)
	}

	project, err := e.db.GetProject(wf.ProjectID)
	if err != nil || project == nil {
		return fmt.Errorf("project not found for workflow")
	}

	pb := NewPromptBuilder(project.BaseBranch)
	prompt := pb.BuildDeployPrompt(wf.BranchName)
	deployTask := &models.Task{
		WorkflowID: wf.ID,
		Type:       models.TaskDeploy,
		Status:     models.TaskQueued,
		Agent:      "deployer",
		Prompt:     prompt,
	}
	if err := e.db.CreateTask(deployTask); err != nil {
		return err
	}

	e.chatMsg(wf.ID, "user", "", "Deployment approved!")
	return e.db.UpdateWorkflowStatus(wf.ID, models.WorkflowDeploying, "")
}

func (e *Engine) RejectDeployment(workflowID string) error {
	wf, err := e.db.GetWorkflow(workflowID)
	if err != nil {
		return err
	}
	if wf == nil {
		return fmt.Errorf("workflow not found")
	}

	e.chatMsg(wf.ID, "user", "", "Deployment rejected.")
	return e.db.UpdateWorkflowStatus(wf.ID, models.WorkflowFailed, "rejected by user")
}

func (e *Engine) chatMsg(workflowID, role, agentType, content string) {
	e.db.CreateChatMessage(&models.ChatMessage{
		WorkflowID: workflowID,
		Role:       role,
		AgentType:  agentType,
		Content:    content,
		Metadata:   "{}",
	})
}

func (e *Engine) RunLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			e.ProcessQueuedTasks()
		}
	}
}
