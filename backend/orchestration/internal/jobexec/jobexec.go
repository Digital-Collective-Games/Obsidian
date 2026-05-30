package jobexec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/config"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/controlplane"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/jobs"
)

const RunCodexExecActivityName = "codex.exec.activity"

type CommandPlan struct {
	Executable       string
	Args             []string
	RunRoot          string
	EventLogPath     string
	FinalMessagePath string
	StderrPath       string
}

type codexExecActivity struct {
	cfg config.Config
}

func Register(w worker.Worker, cfg config.Config) {
	activityImpl := &codexExecActivity{cfg: cfg}
	w.RegisterWorkflowWithOptions(CodexExecWorkflow, workflow.RegisterOptions{Name: "codex.exec.job"})
	w.RegisterActivityWithOptions(activityImpl.Execute, activity.RegisterOptions{Name: RunCodexExecActivityName})
}

func CodexExecWorkflow(ctx workflow.Context, request controlplane.JobRunRequest) (controlplane.JobRunResult, error) {
	info := workflow.GetInfo(ctx)
	request.WorkflowID = info.WorkflowExecution.ID
	request.RunID = info.WorkflowExecution.RunID
	if request.RequestedAt.IsZero() {
		request.RequestedAt = workflow.Now(ctx).UTC()
	}

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 6 * time.Hour,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	})

	var result controlplane.JobRunResult
	if err := workflow.ExecuteActivity(ctx, RunCodexExecActivityName, request).Get(ctx, &result); err != nil {
		return result, err
	}
	return result, nil
}

func BuildCommandPlan(cfg config.Config, request controlplane.JobRunRequest) (CommandPlan, error) {
	if request.Spec.Executor.Cwd == "" {
		return CommandPlan{}, fmt.Errorf("job %s executor cwd is empty", request.JobID)
	}

	runRoot := filepath.Join(cfg.RunsRoot, sanitizePathSegment(request.JobID), sanitizePathSegment(request.WorkflowID))
	eventLogPath := filepath.Join(runRoot, "events.jsonl")
	stderrPath := filepath.Join(runRoot, "stderr.txt")
	plan := CommandPlan{
		RunRoot:      runRoot,
		EventLogPath: eventLogPath,
		StderrPath:   stderrPath,
	}

	switch request.Spec.Executor.Type {
	case jobs.ExecutorTypeCodexExec:
		finalMessagePath := filepath.Join(runRoot, "final-message.txt")
		plan.Executable = cfg.CodexExecutable
		plan.Args = []string{
			"exec",
			"--dangerously-bypass-approvals-and-sandbox",
			"--skip-git-repo-check",
			"--json",
			"-C", request.Spec.Executor.Cwd,
			"-o", finalMessagePath,
			buildPrompt(request),
		}
		plan.FinalMessagePath = finalMessagePath
	case jobs.ExecutorTypePowerShellScript:
		if request.Spec.Executor.ScriptPath == "" {
			return CommandPlan{}, fmt.Errorf("job %s powershell_script executor script_path is empty", request.JobID)
		}
		plan.Executable = "powershell.exe"
		plan.Args = []string{
			"-NoProfile",
			"-ExecutionPolicy", "Bypass",
			"-File", request.Spec.Executor.ScriptPath,
		}
		plan.Args = append(plan.Args, request.Spec.Executor.Args...)
		plan.Args = append(plan.Args, triggerArgs(request)...)
	default:
		return CommandPlan{}, fmt.Errorf("job %s uses unsupported executor type %q", request.JobID, request.Spec.Executor.Type)
	}

	return plan, nil
}

func (a *codexExecActivity) Execute(ctx context.Context, request controlplane.JobRunRequest) (controlplane.JobRunResult, error) {
	plan, err := BuildCommandPlan(a.cfg, request)
	if err != nil {
		return controlplane.JobRunResult{}, err
	}

	if err := os.MkdirAll(plan.RunRoot, 0o755); err != nil {
		return controlplane.JobRunResult{}, fmt.Errorf("create run root: %w", err)
	}

	eventLogFile, err := os.Create(plan.EventLogPath)
	if err != nil {
		return controlplane.JobRunResult{}, fmt.Errorf("create event log: %w", err)
	}
	defer eventLogFile.Close()

	stderrFile, err := os.Create(plan.StderrPath)
	if err != nil {
		return controlplane.JobRunResult{}, fmt.Errorf("create stderr log: %w", err)
	}
	defer stderrFile.Close()

	cmd := exec.CommandContext(ctx, plan.Executable, plan.Args...)
	cmd.Dir = request.Spec.Executor.Cwd
	cmd.Stdout = eventLogFile
	cmd.Stderr = stderrFile

	startedAt := time.Now().UTC()
	runErr := cmd.Run()
	completedAt := time.Now().UTC()

	result := controlplane.JobRunResult{
		JobID:            request.JobID,
		TriggerType:      request.TriggerType,
		DesiredSpecHash:  request.DesiredSpecHash,
		RequestedAt:      request.RequestedAt,
		WorkflowID:       request.WorkflowID,
		RunID:            request.RunID,
		StartedAt:        startedAt,
		CompletedAt:      completedAt,
		EventLogPath:     plan.EventLogPath,
		FinalMessagePath: plan.FinalMessagePath,
		StderrPath:       plan.StderrPath,
		Command:          append([]string{plan.Executable}, plan.Args...),
	}

	result.ExitCode = exitCode(runErr)

	if plan.FinalMessagePath != "" {
		if rawFinalMessage, err := os.ReadFile(plan.FinalMessagePath); err == nil {
			result.FinalMessage = strings.TrimSpace(string(rawFinalMessage))
		}
	}

	// Surface notify-worthy outcomes (a @@JOB-ALERT@@ line in the run's output, or a failed/non-zero run)
	// to the operator as ONE digest per run. Fully defensive: alerting never alters the job result below.
	a.maybeSendRunAlert(ctx, request, result, runErr != nil)

	if runErr == nil {
		return result, nil
	}
	return result, fmt.Errorf("codex exec failed with exit code %d: %w", result.ExitCode, runErr)
}

func buildPrompt(request controlplane.JobRunRequest) string {
	argsText := "(none)"
	if len(request.Spec.Executor.Args) > 0 {
		argsText = strings.Join(request.Spec.Executor.Args, " ")
	}

	lines := []string{
		fmt.Sprintf("Use the `%s` skill or task entrypoint available in this Codex environment.", request.Spec.Executor.Entrypoint),
		fmt.Sprintf("Run it with arguments: %s.", argsText),
		fmt.Sprintf("Job id: %s.", request.JobID),
		fmt.Sprintf("Trigger type: %s.", request.TriggerType),
		fmt.Sprintf("Desired spec hash: %s.", request.DesiredSpecHash),
		fmt.Sprintf("Temporal workflow id: %s.", request.WorkflowID),
		fmt.Sprintf("Temporal run id: %s.", request.RunID),
	}
	if request.TriggerPath != "" {
		lines = append(lines, fmt.Sprintf("Webhook path: %s.", request.TriggerPath))
	}
	lines = append(lines, "Follow the entrypoint's normal behavior and finish with a concise summary of what happened and any operator follow-up required.")
	return strings.Join(lines, "\n")
}

func triggerArgs(request controlplane.JobRunRequest) []string {
	switch request.TriggerType {
	case jobs.TriggerTypeManual:
		return append([]string(nil), request.Spec.Executor.ManualArgs...)
	case jobs.TriggerTypeWebhook:
		return append([]string(nil), request.Spec.Executor.WebhookArgs...)
	default:
		return nil
	}
}

func sanitizePathSegment(value string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_")
	return replacer.Replace(value)
}

func exitCode(runErr error) int {
	if runErr == nil {
		return 0
	}
	var exitError *exec.ExitError
	if errors.As(runErr, &exitError) {
		return exitError.ExitCode()
	}
	return 1
}
