package taskrun

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/testfixtures/taskrepo"
)

type fakeRuntime struct {
	activeByTask map[string]TaskRunView
	byRunID      map[string]TaskRunView
	started      []StartTaskRunRequest
	reconciled   []TaskDefinitionSnapshot
}

func newFakeRuntime() *fakeRuntime {
	return &fakeRuntime{
		activeByTask: map[string]TaskRunView{},
		byRunID:      map[string]TaskRunView{},
	}
}

func (f *fakeRuntime) StartTaskRun(_ context.Context, request StartTaskRunRequest) (TaskRunView, error) {
	f.started = append(f.started, request)
	state := StateDispatching
	reasonCode := "dispatch_started"
	stateSummary := "Run is dispatching in an owned checkout."
	nextOwner := "backend"
	nextExpectedEvent := "Execution worker records the next task-run state update."
	attention := AttentionPriority{Level: AttentionWatch, Reason: "Run is active.", SortKey: "50-dispatching"}
	lastProgressSummary := "Captured task docs and provisioned an owned checkout."
	actions := map[string]ActionAvailability{}
	if request.RepoLane.CurrentCommit != "" {
		state = StateRunning
		reasonCode = "owned_lane_bootstrapped"
		stateSummary = "Run bootstrapped the owned checkout and is ready for backend execution."
		nextOwner = "backend_worker"
		nextExpectedEvent = "Execution worker records the next progress checkpoint."
		attention = AttentionPriority{Level: AttentionWatch, Reason: "Run is active after owned-lane bootstrap.", SortKey: "45-owned_lane_bootstrapped"}
		lastProgressSummary = "Bootstrapped the owned checkout and recorded its current commit."
		actions = map[string]ActionAvailability{
			ActionDispatch: {
				Allowed: false,
				BlockReasons: []ActionBlockReason{{
					Code:    "active_run_exists",
					Summary: "Dispatch is blocked while this run owns the current live story.",
				}},
			},
			ActionPoke: {
				Allowed: false,
				BlockReasons: []ActionBlockReason{{
					Code:    "run_not_suspicious_yet",
					Summary: "Poke stays blocked until the run misses its next expected progress deadline.",
				}},
			},
			ActionInterrupt: {Allowed: true},
		}
	}
	run := TaskRunView{
		RunID:                  request.RunID,
		TaskID:                 request.TaskID,
		WorkflowID:             request.RunID,
		TemporalExecutionRunID: "temporal-run-id",
		Status:                 "active",
		StateEnvelope: StateEnvelope{
			State:             state,
			ReasonCode:        reasonCode,
			StateSummary:      stateSummary,
			NextOwner:         nextOwner,
			NextExpectedEvent: nextExpectedEvent,
			SuspiciousAfter:   request.DispatchRequestedAt.Add(15 * time.Minute),
		},
		MeaningSummary:             request.MeaningSummary,
		Attention:                  attention,
		Actions:                    actions,
		RepoLane:                   request.RepoLane,
		LastProgressAt:             request.DispatchRequestedAt,
		LastProgressSummary:        lastProgressSummary,
		CapturedTaskSnapshot:       request.CapturedTaskSnapshot,
		DocRuntimeDivergenceStatus: "in_sync",
		DeepContext:                request.ContextSnapshot,
	}
	f.activeByTask[request.TaskID] = run
	f.byRunID[request.RunID] = run
	return run, nil
}

func (f *fakeRuntime) GetTaskRun(_ context.Context, runID string) (TaskRunView, error) {
	run, ok := f.byRunID[runID]
	if !ok {
		return TaskRunView{}, ErrRunNotFound
	}
	return run, nil
}

func (f *fakeRuntime) GetActiveTaskRun(_ context.Context, runID string) (TaskRunView, error) {
	for taskID, run := range f.activeByTask {
		if ActiveRunID(taskID) == runID {
			return run, nil
		}
	}
	return TaskRunView{}, ErrRunNotFound
}

func (f *fakeRuntime) ReconcileTaskSnapshot(_ context.Context, runID string, snapshot TaskDefinitionSnapshot) (TaskRunView, error) {
	run, ok := f.byRunID[runID]
	if !ok {
		return TaskRunView{}, ErrRunNotFound
	}
	f.reconciled = append(f.reconciled, snapshot)
	previousRevision := run.CapturedTaskSnapshot.DeclaredTaskRevision
	run.CapturedTaskSnapshot = snapshot
	run.DocRuntimeDivergenceStatus = "reconciled"
	if previousRevision != "" && previousRevision != snapshot.DeclaredTaskRevision {
		run.DocRuntimeDivergenceSummary = "Runtime reconciled the active run from declared task revision " + previousRevision + " to " + snapshot.DeclaredTaskRevision + " during task readback."
	} else {
		run.DocRuntimeDivergenceSummary = "Runtime captured newer task docs during task readback."
	}
	f.byRunID[runID] = run
	f.activeByTask[run.TaskID] = run
	return run, nil
}

func (f *fakeRuntime) UpdateTaskRun(_ context.Context, runID string, update TaskRunUpdate) (TaskRunView, error) {
	run, ok := f.byRunID[runID]
	if !ok {
		return TaskRunView{}, ErrRunNotFound
	}
	if update.State != "" {
		run.StateEnvelope.State = update.State
	}
	if update.ReasonCode != "" {
		run.StateEnvelope.ReasonCode = update.ReasonCode
	}
	if update.StateSummary != "" {
		run.StateEnvelope.StateSummary = update.StateSummary
	}
	if update.NextOwner != "" {
		run.StateEnvelope.NextOwner = update.NextOwner
	}
	if update.NextExpectedEvent != "" {
		run.StateEnvelope.NextExpectedEvent = update.NextExpectedEvent
	}
	if !update.SuspiciousAfter.IsZero() {
		run.StateEnvelope.SuspiciousAfter = update.SuspiciousAfter
	}
	if update.WaitContract != nil {
		run.WaitContract = update.WaitContract
	} else if update.State != "" && update.State != StateWaitingForHuman {
		run.WaitContract = nil
	}
	if update.Attention != nil {
		run.Attention = *update.Attention
	}
	if update.FollowUp != nil {
		if isEmptyRunFollowUp(update.FollowUp) {
			run.FollowUp = nil
		} else {
			run.FollowUp = update.FollowUp
		}
	}
	if update.Resolution != nil {
		run.Resolution = update.Resolution
	}
	if update.Actions != nil {
		run.Actions = update.Actions
		run.StateEnvelope.ActionBlockReasons = collectActionBlockReasons(update.Actions)
	}
	if update.RepoLane != nil {
		run.RepoLane = *update.RepoLane
	}
	if update.LastProgressSummary != "" {
		run.LastProgressSummary = update.LastProgressSummary
		run.LastProgressAt = time.Now().UTC()
	}
	if !update.CompletedAt.IsZero() {
		run.LastProgressAt = update.CompletedAt
	}
	if update.FailureSummary != "" {
		run.FailureSummary = update.FailureSummary
	} else if update.State != "" && update.State != StateBlocked && update.State != StateFailed {
		run.FailureSummary = ""
	}
	switch run.StateEnvelope.State {
	case StateCompleted:
		run.Status = "completed"
	case StateFailed:
		run.Status = "failed"
	case StateInterrupted:
		run.Status = "interrupted"
	default:
		run.Status = "active"
	}
	f.byRunID[runID] = run
	f.activeByTask[run.TaskID] = run
	return run, nil
}

func (f *fakeRuntime) RetryTaskRunWorkload(_ context.Context, runID string, request WorkloadRetryRequest) (TaskRunView, error) {
	run, ok := f.byRunID[runID]
	if !ok {
		return TaskRunView{}, ErrRunNotFound
	}
	run.CapturedTaskSnapshot = request.CapturedTaskSnapshot
	run.RepoLane = request.RepoLane
	run.Status = "active"
	run.StateEnvelope.State = StateRunning
	run.StateEnvelope.ReasonCode = "workload_retry_requested"
	run.StateEnvelope.StateSummary = "Backend reprovisioned a fresh owned lane and is retrying workload execution."
	run.StateEnvelope.NextOwner = "backend_worker"
	run.StateEnvelope.NextExpectedEvent = "Execution worker reruns the owned-lane workload path."
	run.StateEnvelope.SuspiciousAfter = request.RetryRequestedAt.Add(15 * time.Minute)
	run.Actions = actionsForRunState(StateRunning)
	run.StateEnvelope.ActionBlockReasons = collectActionBlockReasons(run.Actions)
	run.FailureSummary = ""
	run.FollowUp = nil
	run.Resolution = nil
	run.LastProgressAt = request.RetryRequestedAt
	run.LastProgressSummary = "Backend requested a workload retry with a fresh owned lane."
	run.Attention = AttentionPriority{
		Level:   AttentionWatch,
		Reason:  "Run is active after the backend reprovisioned a fresh owned lane.",
		SortKey: "36-workload_retry_requested",
	}
	f.byRunID[runID] = run
	f.activeByTask[run.TaskID] = run
	return run, nil
}

func TestListTasksParsesMeaningAndReadyState(t *testing.T) {
	worktreeRoot := writeTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0000",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T16:27:00-04:00"
}`,
			planMD:        "# approved plan\n",
			handoffMD:     "# handoff\n",
			constraintsMD: "# constraints\n",
		},
	})

	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), nil)
	tasks, err := service.ListTasks(context.Background())
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("task count = %d, want 1", len(tasks))
	}
	task := tasks[0]
	if task.TaskID != "Task-0008" {
		t.Fatalf("task id = %q", task.TaskID)
	}
	if task.MeaningSummary != "Create the durable backend task-run contract so later clients do not guess state." {
		t.Fatalf("meaning summary = %q", task.MeaningSummary)
	}
	if task.StateEnvelope.State != StateReady {
		t.Fatalf("state = %q, want %q", task.StateEnvelope.State, StateReady)
	}
	if task.DispatchReadiness.Ready {
		t.Fatal("dispatch readiness should stay false until the durable dispatch lane exists")
	}
	if task.Actions[ActionDispatch].Allowed {
		t.Fatal("dispatch action should stay blocked until the durable dispatch lane exists")
	}
	if task.CurrentStory.Status != "no_active_run" {
		t.Fatalf("current story status = %q", task.CurrentStory.Status)
	}
	if task.DeepContext == nil || task.DeepContext.PreferredLaunchTarget == nil {
		t.Fatalf("task deep context = %#v", task.DeepContext)
	}
	if task.DeepContext.PreferredLaunchTarget.Kind != "task_artifact" {
		t.Fatalf("preferred launch target kind = %q", task.DeepContext.PreferredLaunchTarget.Kind)
	}
	if task.DeepContext.PreferredLaunchTarget.Label != "Task" {
		t.Fatalf("preferred launch target label = %q, want Task", task.DeepContext.PreferredLaunchTarget.Label)
	}
	if !strings.HasSuffix(filepath.ToSlash(task.DeepContext.PreferredLaunchTarget.URI), "/TASK.md") {
		t.Fatalf("preferred launch target uri = %q, want TASK.md", task.DeepContext.PreferredLaunchTarget.URI)
	}
	if len(task.DeepContext.PreferredLaunchTarget.Command) < 2 ||
		!strings.HasSuffix(filepath.ToSlash(task.DeepContext.PreferredLaunchTarget.Command[1]), "/TASK.md") {
		t.Fatalf("preferred launch target command = %#v, want TASK.md", task.DeepContext.PreferredLaunchTarget.Command)
	}
}

func TestCanonicalTaskFixtureRepoStartsAndDispatchesTask8And9(t *testing.T) {
	repo := taskrepo.WriteCanonicalGitRepo(t)
	runtime := newFakeRuntime()
	service := NewService(repo.Root, filepath.Join(repo.Root, ".runs"), runtime)

	tasks, err := service.ListTasks(context.Background())
	if err != nil {
		t.Fatalf("list canonical fixture tasks: %v", err)
	}

	byID := map[string]TaskView{}
	for _, task := range tasks {
		byID[task.TaskID] = task
	}
	task8 := byID["Task-0008"]
	task9 := byID["Task-0009"]
	if task8.TaskID == "" || task9.TaskID == "" {
		t.Fatalf("canonical fixture tasks = %#v, want Task-0008 and Task-0009", byID)
	}
	if task8.DeclaredGitRevision != repo.InitialCommit {
		t.Fatalf("Task-0008 git revision = %q, want %q", task8.DeclaredGitRevision, repo.InitialCommit)
	}
	if task9.StateEnvelope.State != StateCompleted {
		t.Fatalf("Task-0009 state = %q, want %q", task9.StateEnvelope.State, StateCompleted)
	}
	if !task8.DispatchReadiness.Ready {
		t.Fatalf("Task-0008 should be dispatch-ready, blockers = %#v", task8.DispatchReadiness.BlockReasons)
	}

	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch Task-0008: %v", err)
	}
	if len(runtime.started) != 1 {
		t.Fatalf("runtime started count = %d, want 1", len(runtime.started))
	}
	if runtime.started[0].CapturedTaskSnapshot.DeclaredGitRevision != repo.InitialCommit {
		t.Fatalf("captured git revision = %q, want %q", runtime.started[0].CapturedTaskSnapshot.DeclaredGitRevision, repo.InitialCommit)
	}
	if run.RepoLane.OwnedRepoRoot == "" {
		t.Fatal("dispatch should provision an owned repo lane")
	}

	taskAfterDispatch, err := service.Task(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("task detail after dispatch: %v", err)
	}
	if taskAfterDispatch.LatestRun == nil {
		t.Fatal("Task-0008 should expose latest run after dispatch")
	}
	if !taskAfterDispatch.LatestRun.Actions[ActionInterrupt].Allowed {
		t.Fatal("Task-0008 active run should expose interrupt availability for the Tasks tab to render as Pause")
	}
	if taskAfterDispatch.Actions[ActionDispatch].Allowed {
		t.Fatal("Task-0008 dispatch should be blocked while the active story is owned by the run")
	}
	if taskAfterDispatch.CurrentStory.Status != "active_run" {
		t.Fatalf("current story = %q, want active_run", taskAfterDispatch.CurrentStory.Status)
	}
}

func TestCompleteTaskStateIsTerminalAndNotDispatchReady(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0001": {
			taskMD: `# Task 0001

## Title

Completed task.

## Summary

This task is already done.
`,
			taskState: `{
  "task_id": "Task-0001",
  "status": "complete",
  "phase": "closure",
  "plan_approved": true,
  "current_pass": "PASS-0006",
  "current_gate": "closure",
  "blockers": [],
  "updated_at": "2026-04-27T00:30:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), newFakeRuntime())

	task, err := service.Task(context.Background(), "Task-0001")
	if err != nil {
		t.Fatalf("task detail: %v", err)
	}

	if task.StateEnvelope.State != StateCompleted {
		t.Fatalf("state = %q, want %q", task.StateEnvelope.State, StateCompleted)
	}
	if task.StateEnvelope.ReasonCode != "task_complete" {
		t.Fatalf("reason code = %q, want task_complete", task.StateEnvelope.ReasonCode)
	}
	if task.LatestRun != nil {
		t.Fatalf("completed task should not expose a live run, got %#v", task.LatestRun)
	}
	if task.DispatchReadiness.Ready {
		t.Fatal("completed task should not be dispatch-ready")
	}
	if !hasBlockReason(task.DispatchReadiness.BlockReasons, "task_terminal") {
		t.Fatalf("dispatch blockers = %#v, want task_terminal", task.DispatchReadiness.BlockReasons)
	}
	if task.Actions[ActionDispatch].Allowed {
		t.Fatal("completed task should not allow dispatch")
	}
	if task.Actions[ActionInterrupt].Allowed {
		t.Fatal("completed task should not expose interrupt")
	}
	if task.Attention.Level != AttentionNone {
		t.Fatalf("attention level = %q, want %q", task.Attention.Level, AttentionNone)
	}
	if task.CurrentStory.Status != "no_active_run" {
		t.Fatalf("current story = %q, want no_active_run", task.CurrentStory.Status)
	}
}

func TestCancelledTaskStateIsTerminalAndNotDispatchReady(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0004": {
			taskMD: `# Task 0004

## Title

Cancelled task.

## Summary

This task was cancelled before dispatch.
`,
			taskState: `{
  "task_id": "Task-0004",
  "status": "cancelled",
  "phase": "closure",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "closure",
  "blockers": [],
  "updated_at": "2026-04-27T00:31:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), newFakeRuntime())

	task, err := service.Task(context.Background(), "Task-0004")
	if err != nil {
		t.Fatalf("task detail: %v", err)
	}

	if task.StateEnvelope.State != StateCancelled {
		t.Fatalf("state = %q, want %q", task.StateEnvelope.State, StateCancelled)
	}
	if task.StateEnvelope.ReasonCode != "task_cancelled" {
		t.Fatalf("reason code = %q, want task_cancelled", task.StateEnvelope.ReasonCode)
	}
	if task.LatestRun != nil {
		t.Fatalf("cancelled task should not expose a live run, got %#v", task.LatestRun)
	}
	if task.DispatchReadiness.Ready {
		t.Fatal("cancelled task should not be dispatch-ready")
	}
	if !hasBlockReason(task.DispatchReadiness.BlockReasons, "task_terminal") {
		t.Fatalf("dispatch blockers = %#v, want task_terminal", task.DispatchReadiness.BlockReasons)
	}
	if task.Actions[ActionDispatch].Allowed {
		t.Fatal("cancelled task should not allow dispatch")
	}
	if task.Actions[ActionInterrupt].Allowed {
		t.Fatal("cancelled task should not expose interrupt")
	}
	if task.Attention.Level != AttentionNone {
		t.Fatalf("attention level = %q, want %q", task.Attention.Level, AttentionNone)
	}
}

func TestTerminalTaskStateOverridesStaleActiveRun(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Completed task with stale runtime.

## Summary

The durable state has closed even if runtime still has an active record.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "complete",
  "phase": "closure",
  "plan_approved": true,
  "current_pass": "PASS-0005",
  "current_gate": "closure",
  "blockers": [],
  "updated_at": "2026-04-27T00:32:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})
	runtime := newFakeRuntime()
	staleRun := TaskRunView{
		RunID:  "stale-run",
		TaskID: "Task-0008",
		Status: "active",
		StateEnvelope: StateEnvelope{
			State:        StateRunning,
			ReasonCode:   "stale_active_run",
			StateSummary: "Stale runtime still thinks this task is running.",
		},
		Actions: map[string]ActionAvailability{
			ActionInterrupt: {Allowed: true},
		},
		CapturedTaskSnapshot: TaskDefinitionSnapshot{DeclaredTaskRevision: "stale-revision"},
	}
	runtime.activeByTask["Task-0008"] = staleRun
	runtime.byRunID["stale-run"] = staleRun
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)

	task, err := service.Task(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("task detail: %v", err)
	}

	if task.StateEnvelope.State != StateCompleted {
		t.Fatalf("state = %q, want %q", task.StateEnvelope.State, StateCompleted)
	}
	if task.LatestRun != nil {
		t.Fatalf("terminal durable state should suppress stale active run, got %#v", task.LatestRun)
	}
	if task.Actions[ActionInterrupt].Allowed {
		t.Fatal("terminal durable state should not expose Pause/interrupt from stale runtime")
	}
	if len(runtime.reconciled) != 0 {
		t.Fatalf("terminal task should not reconcile stale active runtime, got %#v", runtime.reconciled)
	}
	if task.CurrentStory.Status != "no_active_run" {
		t.Fatalf("current story = %q, want no_active_run", task.CurrentStory.Status)
	}
}

func TestTaskUsesWaitingForHumanWhenPlanIsNotApproved(t *testing.T) {
	worktreeRoot := writeTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0012": {
			taskMD: `# Task 0012

## Title

Ship the next task.

## Summary

Need approval before implementation starts.
`,
			taskState: `{
  "task_id": "Task-0012",
  "status": "in_progress",
  "phase": "planning",
  "plan_approved": false,
  "current_pass": "",
  "current_gate": "planning",
  "blockers": [],
  "updated_at": "2026-04-24T16:27:00-04:00"
}`,
			planMD: "# plan\n",
		},
	})

	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), nil)
	task, err := service.Task(context.Background(), "Task-0012")
	if err != nil {
		t.Fatalf("task detail: %v", err)
	}

	if task.StateEnvelope.State != StateWaitingForHuman {
		t.Fatalf("state = %q, want %q", task.StateEnvelope.State, StateWaitingForHuman)
	}
	if task.StateEnvelope.ReasonCode != "plan_approval_required" {
		t.Fatalf("reason code = %q", task.StateEnvelope.ReasonCode)
	}
	if task.Actions[ActionDispatch].Allowed {
		t.Fatal("dispatch should not be allowed while plan approval is missing")
	}
}

func TestDispatchProvisionOwnedLaneAndCaptureBaselineCommit(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0001",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T16:44:31-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	runsRoot := filepath.Join(worktreeRoot, ".runs")
	service := NewService(worktreeRoot, runsRoot, runtime)

	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if len(runtime.started) != 1 {
		t.Fatalf("started requests = %d, want 1", len(runtime.started))
	}
	request := runtime.started[0]
	if request.TaskID != "Task-0008" {
		t.Fatalf("task id = %q", request.TaskID)
	}
	if request.RepoLane.BaselineCommit == "" {
		t.Fatal("expected baseline commit to be captured")
	}
	if request.RepoLane.CurrentCommit != request.RepoLane.BaselineCommit {
		t.Fatalf("current commit = %q, want baseline %q", request.RepoLane.CurrentCommit, request.RepoLane.BaselineCommit)
	}
	if request.RepoLane.ApprovedRestoreCommit != request.RepoLane.BaselineCommit {
		t.Fatalf("approved restore commit = %q, want baseline %q", request.RepoLane.ApprovedRestoreCommit, request.RepoLane.BaselineCommit)
	}
	if request.RepoLane.OwnedRepoRoot == "" {
		t.Fatal("expected owned repo root to be set")
	}
	if _, err := os.Stat(request.RepoLane.OwnedRepoRoot); err != nil {
		t.Fatalf("owned repo root missing: %v", err)
	}
	if request.RepoLane.RunArtifactRoot == "" {
		t.Fatal("expected run artifact root to be set")
	}
	if request.RepoLane.BootstrapArtifactPath == "" {
		t.Fatal("expected bootstrap artifact path to be set")
	}
	rawBootstrap, err := os.ReadFile(request.RepoLane.BootstrapArtifactPath)
	if err != nil {
		t.Fatalf("read bootstrap artifact: %v", err)
	}
	var bootstrap ownedLaneBootstrapRecord
	if err := json.Unmarshal(rawBootstrap, &bootstrap); err != nil {
		t.Fatalf("decode bootstrap artifact: %v", err)
	}
	if bootstrap.CurrentCommit != request.RepoLane.CurrentCommit {
		t.Fatalf("bootstrap current commit = %q, want %q", bootstrap.CurrentCommit, request.RepoLane.CurrentCommit)
	}
	if bootstrap.OwnedRepoRoot != request.RepoLane.OwnedRepoRoot {
		t.Fatalf("bootstrap owned repo root = %q, want %q", bootstrap.OwnedRepoRoot, request.RepoLane.OwnedRepoRoot)
	}
	if run.RunID != ActiveRunID("Task-0008") {
		t.Fatalf("run id = %q", run.RunID)
	}
	if run.StateEnvelope.State != StateRunning {
		t.Fatalf("state = %q, want %q", run.StateEnvelope.State, StateRunning)
	}
	if run.StateEnvelope.ReasonCode != "owned_lane_bootstrapped" {
		t.Fatalf("reason code = %q, want owned_lane_bootstrapped", run.StateEnvelope.ReasonCode)
	}
	if run.DeepContext == nil || len(run.DeepContext.LaunchTargets) == 0 {
		t.Fatalf("run deep context = %#v", run.DeepContext)
	}
	if !hasLaunchTargetEnding(run.DeepContext.LaunchTargets, "Task", "TASK.md") {
		t.Fatalf("run launch targets = %#v, want Task target ending in TASK.md", run.DeepContext.LaunchTargets)
	}
}

func TestDispatchWorkloadFailureExerciseCapturesOneShotDirective(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T16:44:31-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)

	run, err := service.DispatchWorkloadFailureExercise(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch workload failure exercise: %v", err)
	}
	if len(runtime.started) != 1 {
		t.Fatalf("started requests = %d, want 1", len(runtime.started))
	}
	request := runtime.started[0]
	if request.ExecutionDirective == nil {
		t.Fatal("expected execution directive")
	}
	if request.ExecutionDirective.FailureMode != ExecutionFailureModeTask0008WorkloadFailureOnce {
		t.Fatalf("failure mode = %q", request.ExecutionDirective.FailureMode)
	}
	if run.StateEnvelope.ReasonCode != "owned_lane_bootstrapped" {
		t.Fatalf("reason code = %q, want owned_lane_bootstrapped", run.StateEnvelope.ReasonCode)
	}
}

func TestUpdateRunAppliesRicherStateContract(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0001",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T16:44:31-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	runsRoot := filepath.Join(worktreeRoot, ".runs")
	service := NewService(worktreeRoot, runsRoot, runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	updated, err := service.UpdateRun(context.Background(), run.RunID, TaskRunUpdate{
		State:               StateWaitingForHuman,
		ReasonCode:          "review_required",
		StateSummary:        "Run is waiting for human review.",
		NextOwner:           "human",
		NextExpectedEvent:   "Approve the next backend action.",
		LastProgressSummary: "Run recorded a review checkpoint.",
		WaitContract: &WaitContract{
			WaitingOn:           "human_review",
			WhyBlocked:          "The next backend action needs human approval.",
			ResumeWhen:          "The human approves the next backend action.",
			HumanActionRequired: true,
			HumanActionTarget: &HumanActionTarget{
				Kind:  "approval_action",
				Label: "Approve backend review step",
				URI:   "approval://taskrun/Task-0008",
			},
		},
	})
	if err != nil {
		t.Fatalf("update run: %v", err)
	}

	if updated.StateEnvelope.State != StateWaitingForHuman {
		t.Fatalf("updated state = %q", updated.StateEnvelope.State)
	}
	if updated.WaitContract == nil || updated.WaitContract.HumanActionTarget == nil {
		t.Fatal("expected wait contract with explicit human action target")
	}
	if updated.Attention.Level != AttentionNeedsAttention {
		t.Fatalf("attention level = %q, want %q", updated.Attention.Level, AttentionNeedsAttention)
	}
}

func TestRunReadSupervisesStaleProgressIntoSleepingState(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T17:10:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if _, err := service.UpdateRun(context.Background(), run.RunID, TaskRunUpdate{
		State:             StateRunning,
		ReasonCode:        "worker_started",
		StateSummary:      "Run is actively executing.",
		SuspiciousAfter:   time.Now().UTC().Add(-1 * time.Minute),
		NextOwner:         "backend",
		NextExpectedEvent: "Execution worker records the next progress checkpoint.",
	}); err != nil {
		t.Fatalf("prime run: %v", err)
	}

	supervised, err := service.Run(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("run detail: %v", err)
	}
	if supervised.StateEnvelope.State != StateSleepingOrStalled {
		t.Fatalf("state = %q, want %q", supervised.StateEnvelope.State, StateSleepingOrStalled)
	}
	if !supervised.Actions[ActionPoke].Allowed {
		t.Fatal("poke should be allowed after supervision marks the run stalled")
	}
	if !supervised.Actions[ActionInterrupt].Allowed {
		t.Fatal("interrupt should stay allowed for a stalled run")
	}
}

func TestTaskReadReconcilesDeclaredDocDriftWhileRunStaysActive(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0003",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T22:00:00-04:00"
}`,
			planMD:    "# approved plan\n",
			handoffMD: "# handoff\n\n## Current Status\n\ninitial handoff\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	oldRevision := run.CapturedTaskSnapshot.DeclaredTaskRevision

	writeFile(t, filepath.Join(worktreeRoot, "Tracking", "Task-0008", "HANDOFF.md"), "# handoff\n\n## Current Status\n\ndrifted handoff\n")

	task, err := service.Task(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("task detail after drift: %v", err)
	}
	if task.LatestRun == nil {
		t.Fatal("expected latest run after drift")
	}
	if task.CurrentStory.Status != "active_run" {
		t.Fatalf("current story = %q, want active_run", task.CurrentStory.Status)
	}
	if task.LatestRun.DocRuntimeDivergenceStatus != "reconciled" {
		t.Fatalf("divergence status = %q, want reconciled", task.LatestRun.DocRuntimeDivergenceStatus)
	}
	if task.LatestRun.CapturedTaskSnapshot.DeclaredTaskRevision == oldRevision {
		t.Fatalf("captured revision = %q, want new revision not %q", task.LatestRun.CapturedTaskSnapshot.DeclaredTaskRevision, oldRevision)
	}
	if !strings.Contains(task.LatestRun.DocRuntimeDivergenceSummary, oldRevision) {
		t.Fatalf("divergence summary = %q, want old revision", task.LatestRun.DocRuntimeDivergenceSummary)
	}
	if !strings.Contains(task.LatestRun.DocRuntimeDivergenceSummary, task.LatestRun.CapturedTaskSnapshot.DeclaredTaskRevision) {
		t.Fatalf("divergence summary = %q, want new revision", task.LatestRun.DocRuntimeDivergenceSummary)
	}
	if task.LatestRun.StateEnvelope.ReasonCode != "owned_lane_bootstrapped" {
		t.Fatalf("reason code = %q, want owned_lane_bootstrapped", task.LatestRun.StateEnvelope.ReasonCode)
	}
	if task.DeclaredTaskRevision != task.LatestRun.CapturedTaskSnapshot.DeclaredTaskRevision {
		t.Fatalf("task declared revision = %q, run captured revision = %q", task.DeclaredTaskRevision, task.LatestRun.CapturedTaskSnapshot.DeclaredTaskRevision)
	}
}

func TestTaskReadReconcilesGitRollbackIntoActiveRun(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0001",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T22:00:00-04:00"
}`,
			planMD:    "# approved plan\n",
			handoffMD: "# handoff\n\n## Current Status\n\ninitial handoff\n",
		},
	})
	initialCommit := runOutput(t, worktreeRoot, "git", "rev-parse", "HEAD")

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	initialRevision := run.CapturedTaskSnapshot.DeclaredTaskRevision
	if run.CapturedTaskSnapshot.DeclaredGitRevision != initialCommit {
		t.Fatalf("initial git revision = %q, want %q", run.CapturedTaskSnapshot.DeclaredGitRevision, initialCommit)
	}

	writeFile(t, filepath.Join(worktreeRoot, "Tracking", "Task-0008", "TASK-STATE.json"), `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T23:00:00-04:00"
}`)
	runCommand(t, worktreeRoot, "git", "add", "Tracking/Task-0008/TASK-STATE.json")
	runCommand(t, worktreeRoot, "git", "commit", "-m", "advance task state")
	advancedCommit := runOutput(t, worktreeRoot, "git", "rev-parse", "HEAD")

	advancedTask, err := service.Task(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("task detail after advance: %v", err)
	}
	if advancedTask.CurrentPass != "PASS-0002" || advancedTask.CurrentGate != "implementation" {
		t.Fatalf("advanced task state = pass %q gate %q", advancedTask.CurrentPass, advancedTask.CurrentGate)
	}
	if advancedTask.LatestRun == nil {
		t.Fatal("expected latest run after advance")
	}
	if advancedTask.LatestRun.CapturedTaskSnapshot.DeclaredTaskRevision == initialRevision {
		t.Fatalf("advanced run snapshot revision did not change from %q", initialRevision)
	}
	if advancedTask.LatestRun.CapturedTaskSnapshot.DeclaredGitRevision != advancedCommit {
		t.Fatalf("advanced git revision = %q, want %q", advancedTask.LatestRun.CapturedTaskSnapshot.DeclaredGitRevision, advancedCommit)
	}
	if len(runtime.reconciled) != 1 {
		t.Fatalf("reconcile count after advance = %d, want 1", len(runtime.reconciled))
	}
	if runtime.reconciled[0].DeclaredGitRevision != advancedCommit {
		t.Fatalf("first reconciled git revision = %q, want %q", runtime.reconciled[0].DeclaredGitRevision, advancedCommit)
	}

	runCommand(t, worktreeRoot, "git", "reset", "--hard", initialCommit)

	rolledBackTask, err := service.Task(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("task detail after rollback: %v", err)
	}
	if rolledBackTask.CurrentPass != "PASS-0001" || rolledBackTask.CurrentGate != "implementation" {
		t.Fatalf("rolled-back task state = pass %q gate %q", rolledBackTask.CurrentPass, rolledBackTask.CurrentGate)
	}
	if rolledBackTask.LatestRun == nil {
		t.Fatal("expected latest run after rollback")
	}
	if rolledBackTask.LatestRun.CapturedTaskSnapshot.DeclaredTaskRevision != initialRevision {
		t.Fatalf("rolled-back run snapshot revision = %q, want initial %q", rolledBackTask.LatestRun.CapturedTaskSnapshot.DeclaredTaskRevision, initialRevision)
	}
	if rolledBackTask.LatestRun.CapturedTaskSnapshot.DeclaredGitRevision != initialCommit {
		t.Fatalf("rolled-back git revision = %q, want %q", rolledBackTask.LatestRun.CapturedTaskSnapshot.DeclaredGitRevision, initialCommit)
	}
	if len(runtime.reconciled) != 2 {
		t.Fatalf("reconcile count after rollback = %d, want 2", len(runtime.reconciled))
	}
	if runtime.reconciled[1].DeclaredTaskRevision != initialRevision {
		t.Fatalf("second reconciled revision = %q, want %q", runtime.reconciled[1].DeclaredTaskRevision, initialRevision)
	}
	if rolledBackTask.LatestRun.DocRuntimeDivergenceStatus != "reconciled" {
		t.Fatalf("rollback divergence status = %q, want reconciled", rolledBackTask.LatestRun.DocRuntimeDivergenceStatus)
	}
	if !strings.Contains(rolledBackTask.LatestRun.DocRuntimeDivergenceSummary, advancedTask.LatestRun.CapturedTaskSnapshot.DeclaredTaskRevision) {
		t.Fatalf("rollback divergence summary = %q, want advanced revision", rolledBackTask.LatestRun.DocRuntimeDivergenceSummary)
	}
	if !strings.Contains(rolledBackTask.LatestRun.DocRuntimeDivergenceSummary, initialRevision) {
		t.Fatalf("rollback divergence summary = %q, want initial revision", rolledBackTask.LatestRun.DocRuntimeDivergenceSummary)
	}
}

func TestInterruptRunRestoresOwnedLaneAndMarksInterrupted(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T17:10:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	writeFile(t, filepath.Join(run.RepoLane.OwnedRepoRoot, "scratch.txt"), "temporary\n")
	writeFile(t, filepath.Join(run.RepoLane.OwnedRepoRoot, "Tracking", "Task-0008", "TASK.md"), "# changed\n")

	interrupted, err := service.InterruptRun(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("interrupt run: %v", err)
	}
	if interrupted.StateEnvelope.State != StateInterrupted {
		t.Fatalf("state = %q, want %q", interrupted.StateEnvelope.State, StateInterrupted)
	}
	if interrupted.RepoLane.ResetStatus != "restored" {
		t.Fatalf("reset status = %q, want restored", interrupted.RepoLane.ResetStatus)
	}
	if interrupted.Status != "interrupted" {
		t.Fatalf("status = %q, want interrupted", interrupted.Status)
	}

	cmd := exec.Command("git", "-C", interrupted.RepoLane.OwnedRepoRoot, "status", "--short")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status in owned lane: %v\n%s", err, string(output))
	}
	if got := string(output); got != "" {
		t.Fatalf("owned lane should be clean after interrupt reset, got %q", got)
	}

	task, err := service.Task(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("task detail after interrupt: %v", err)
	}
	if task.CurrentStory.Status != "no_active_run" {
		t.Fatalf("current story after interrupt = %q, want no_active_run", task.CurrentStory.Status)
	}
	if task.DispatchReadiness.Ready {
		t.Fatal("dispatch should stay blocked until interrupt review is resolved")
	}
	if task.StateEnvelope.ReasonCode != "interrupt_review_pending" {
		t.Fatalf("reason code = %q, want interrupt_review_pending", task.StateEnvelope.ReasonCode)
	}
}

func TestRunReadEscalatesStaleHumanWait(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T17:10:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	_, err = service.UpdateRun(context.Background(), run.RunID, TaskRunUpdate{
		State:             StateWaitingForHuman,
		ReasonCode:        "review_required",
		StateSummary:      "Run is waiting for human review.",
		NextOwner:         "human",
		NextExpectedEvent: "Approve the next backend action.",
		WaitContract: &WaitContract{
			WaitingOn:           "human_review",
			WhyBlocked:          "The next backend action needs human approval.",
			ResumeWhen:          "The human approves the next backend action.",
			HumanActionRequired: true,
			HumanActionTarget: &HumanActionTarget{
				Kind:  "approval_action",
				Label: "Approve backend review step",
				URI:   "approval://taskrun/Task-0008",
			},
			StaleAfter: time.Now().UTC().Add(-1 * time.Minute),
		},
	})
	if err != nil {
		t.Fatalf("update run: %v", err)
	}

	supervised, err := service.Run(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("run detail: %v", err)
	}
	if supervised.StateEnvelope.ReasonCode != "human_wait_stale" {
		t.Fatalf("reason code = %q, want human_wait_stale", supervised.StateEnvelope.ReasonCode)
	}
	if supervised.Attention.Level != AttentionUrgent {
		t.Fatalf("attention level = %q, want urgent", supervised.Attention.Level)
	}
	if supervised.Actions[ActionInterrupt].Allowed != true {
		t.Fatal("interrupt should remain allowed for a stale human wait")
	}
	if supervised.Actions[ActionPoke].Allowed {
		t.Fatal("poke should stay blocked for a stale human wait")
	}
	if supervised.Actions[ActionPoke].BlockReasons[0].Code != "waiting_for_human_stale" {
		t.Fatalf("poke block reason = %q", supervised.Actions[ActionPoke].BlockReasons[0].Code)
	}
}

func TestInterruptRunSurfacesCleanupBlockedReadback(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T17:10:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	badRoot := filepath.Join(worktreeRoot, "outside-owned-lane")
	if err := os.MkdirAll(badRoot, 0o755); err != nil {
		t.Fatalf("mkdir bad root: %v", err)
	}
	current := runtime.byRunID[run.RunID]
	current.StateEnvelope.State = StateRunning
	current.Actions = actionsForRunState(StateRunning)
	current.RepoLane.OwnedRepoRoot = badRoot
	runtime.byRunID[run.RunID] = current
	runtime.activeByTask[run.TaskID] = current

	blocked, err := service.InterruptRun(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("interrupt run: %v", err)
	}
	if blocked.StateEnvelope.ReasonCode != "interrupt_cleanup_blocked" {
		t.Fatalf("reason code = %q", blocked.StateEnvelope.ReasonCode)
	}
	if blocked.RepoLane.ResetStatus != "cleanup_blocked" {
		t.Fatalf("reset status = %q, want cleanup_blocked", blocked.RepoLane.ResetStatus)
	}
	if blocked.RepoLane.ResetFailureSummary == "" {
		t.Fatal("expected reset failure summary")
	}
	if blocked.FailureSummary == "" {
		t.Fatal("expected run failure summary to be populated")
	}
}

func TestRetryCleanupRunRepairsCleanupBlockedRunIntoInterruptReview(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T17:10:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	validOwnedRoot := run.RepoLane.OwnedRepoRoot

	badRoot := filepath.Join(worktreeRoot, "outside-owned-lane")
	if err := os.MkdirAll(badRoot, 0o755); err != nil {
		t.Fatalf("mkdir bad root: %v", err)
	}
	current := runtime.byRunID[run.RunID]
	current.StateEnvelope.State = StateRunning
	current.Actions = actionsForRunState(StateRunning)
	current.RepoLane.OwnedRepoRoot = badRoot
	runtime.byRunID[run.RunID] = current
	runtime.activeByTask[run.TaskID] = current

	blocked, err := service.InterruptRun(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("interrupt run: %v", err)
	}
	if blocked.StateEnvelope.ReasonCode != "interrupt_cleanup_blocked" {
		t.Fatalf("reason code = %q", blocked.StateEnvelope.ReasonCode)
	}

	repairedSeed := runtime.byRunID[run.RunID]
	repairedSeed.RepoLane.OwnedRepoRoot = validOwnedRoot
	runtime.byRunID[run.RunID] = repairedSeed
	runtime.activeByTask[run.TaskID] = repairedSeed

	repaired, err := service.RetryCleanupRun(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("retry cleanup: %v", err)
	}
	if repaired.StateEnvelope.State != StateInterrupted {
		t.Fatalf("state = %q, want %q", repaired.StateEnvelope.State, StateInterrupted)
	}
	if repaired.StateEnvelope.ReasonCode != "interrupt_cleanup_repaired" {
		t.Fatalf("reason code = %q", repaired.StateEnvelope.ReasonCode)
	}
	if repaired.RepoLane.ResetStatus != "restored" {
		t.Fatalf("reset status = %q, want restored", repaired.RepoLane.ResetStatus)
	}
	if repaired.FollowUp == nil || repaired.FollowUp.Kind != "interrupt_review" || repaired.FollowUp.Status != "pending" {
		t.Fatalf("follow-up = %#v", repaired.FollowUp)
	}
	if repaired.FailureSummary != "" {
		t.Fatalf("failure summary = %q, want empty", repaired.FailureSummary)
	}
	if repaired.Status != "interrupted" {
		t.Fatalf("status = %q, want interrupted", repaired.Status)
	}
}

func TestRetryCleanupRunRejectsNonCleanupBlockedRun(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T17:10:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if _, err := service.RetryCleanupRun(context.Background(), run.RunID); err == nil {
		t.Fatal("expected retry cleanup to reject a non-cleanup-blocked run")
	}
}

func TestRetryWorkloadRunReprovisionsOwnedLane(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T17:10:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	oldOwnedRoot := run.RepoLane.OwnedRepoRoot
	if _, err := service.UpdateRun(context.Background(), run.RunID, TaskRunUpdate{
		State:               StateBlocked,
		ReasonCode:          "workload_execution_failed",
		StateSummary:        "Run could not execute the prepared workload step inside the owned lane.",
		NextOwner:           "human_or_supervisor",
		NextExpectedEvent:   "Retry the workload path with a fresh owned lane.",
		LastProgressSummary: "The prepared workload step failed during execution inside the owned lane.",
		FailureSummary:      "simulated workload execution failure",
	}); err != nil {
		t.Fatalf("seed workload failure: %v", err)
	}
	seeded, err := service.Run(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("read seeded run: %v", err)
	}
	if seeded.FollowUp == nil || seeded.FollowUp.Kind != "workload_recovery" || seeded.FollowUp.Status != "pending" {
		t.Fatalf("seeded follow-up = %#v", seeded.FollowUp)
	}

	retried, err := service.RetryWorkloadRun(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("retry workload: %v", err)
	}
	if retried.StateEnvelope.State != StateRunning {
		t.Fatalf("state = %q, want %q", retried.StateEnvelope.State, StateRunning)
	}
	if retried.StateEnvelope.ReasonCode != "workload_retry_requested" {
		t.Fatalf("reason = %q, want workload_retry_requested", retried.StateEnvelope.ReasonCode)
	}
	if retried.RepoLane.OwnedRepoRoot == "" {
		t.Fatal("expected fresh owned repo root")
	}
	if retried.RepoLane.OwnedRepoRoot == oldOwnedRoot {
		t.Fatalf("owned repo root = %q, want fresh lane", retried.RepoLane.OwnedRepoRoot)
	}
	if retried.FailureSummary != "" {
		t.Fatalf("failure summary = %q, want cleared", retried.FailureSummary)
	}
	if retried.FollowUp != nil {
		t.Fatalf("follow-up should be cleared after retry, got %#v", retried.FollowUp)
	}
	if _, err := os.Stat(oldOwnedRoot); !os.IsNotExist(err) {
		t.Fatalf("old owned lane should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(retried.RepoLane.OwnedRepoRoot); err != nil {
		t.Fatalf("fresh owned lane missing: %v", err)
	}
	if retried.RepoLane.BaselineCommit == "" {
		t.Fatal("expected baseline commit in fresh owned lane")
	}
}

func TestRetryWorkloadRunRejectsNonWorkloadFailure(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T17:10:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if _, err := service.RetryWorkloadRun(context.Background(), run.RunID); err == nil {
		t.Fatal("expected workload retry to reject a non-workload-failed run")
	}
}

func TestTaskReadBlocksDispatchWhileInterruptReviewIsPending(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T17:10:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	interrupted, err := service.InterruptRun(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("interrupt run: %v", err)
	}
	if interrupted.FollowUp == nil || interrupted.FollowUp.Kind != "interrupt_review" {
		t.Fatalf("follow-up = %#v", interrupted.FollowUp)
	}

	task, err := service.Task(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("task detail: %v", err)
	}
	if task.StateEnvelope.ReasonCode != "interrupt_review_pending" {
		t.Fatalf("reason code = %q", task.StateEnvelope.ReasonCode)
	}
	if task.DispatchReadiness.Ready {
		t.Fatal("dispatch should stay blocked while interrupt review is pending")
	}
	if task.Actions[ActionDispatch].Allowed {
		t.Fatal("dispatch action should stay blocked while interrupt review is pending")
	}
	if task.Attention.Level != AttentionNeedsAttention {
		t.Fatalf("attention level = %q, want %q", task.Attention.Level, AttentionNeedsAttention)
	}
}

func TestResolveInterruptReviewUnblocksDispatch(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T17:10:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	originalOwnedRoot := run.RepoLane.OwnedRepoRoot

	if _, err := service.InterruptRun(context.Background(), run.RunID); err != nil {
		t.Fatalf("interrupt run: %v", err)
	}

	resolved, err := service.ResolveInterruptReview(context.Background(), run.RunID, InterruptReviewResolution{
		Decision:   "redispatch_ready",
		Summary:    "Human review approved another dispatch attempt.",
		ResolvedBy: "human",
	})
	if err != nil {
		t.Fatalf("resolve interrupt review: %v", err)
	}
	if resolved.FollowUp == nil || resolved.FollowUp.Status != "completed" {
		t.Fatalf("follow-up = %#v", resolved.FollowUp)
	}
	if resolved.Resolution == nil || resolved.Resolution.Decision != "redispatch_ready" {
		t.Fatalf("resolution = %#v", resolved.Resolution)
	}
	if resolved.Attention.Level != AttentionNone {
		t.Fatalf("attention level = %q, want %q", resolved.Attention.Level, AttentionNone)
	}
	if resolved.RepoLane.ResetStatus != "released" {
		t.Fatalf("reset status = %q, want released", resolved.RepoLane.ResetStatus)
	}
	if resolved.RepoLane.OwnedRepoRoot != "" {
		t.Fatalf("owned repo root = %q, want empty after release", resolved.RepoLane.OwnedRepoRoot)
	}
	if resolved.RepoLane.LastResetTargetCommit == "" {
		t.Fatal("expected last reset target commit after release")
	}
	if _, err := os.Stat(originalOwnedRoot); !os.IsNotExist(err) {
		t.Fatalf("expected resolved review to remove the prior owned root, stat err = %v", err)
	}
	if !strings.Contains(resolved.LastProgressSummary, "Backend released the prior owned lane.") {
		t.Fatalf("last progress summary = %q", resolved.LastProgressSummary)
	}

	task, err := service.Task(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("task detail: %v", err)
	}
	if !task.DispatchReadiness.Ready {
		t.Fatal("dispatch should become ready after interrupt review resolves for redispatch")
	}
	if !task.Actions[ActionDispatch].Allowed {
		t.Fatal("dispatch action should be allowed after interrupt review resolution")
	}
}

func TestResolveInterruptReviewKeepClosedReleasesOwnedLane(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T17:10:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	originalOwnedRoot := run.RepoLane.OwnedRepoRoot

	if _, err := service.InterruptRun(context.Background(), run.RunID); err != nil {
		t.Fatalf("interrupt run: %v", err)
	}

	resolved, err := service.ResolveInterruptReview(context.Background(), run.RunID, InterruptReviewResolution{
		Decision:   "keep_closed",
		Summary:    "Human review closed this run.",
		ResolvedBy: "human",
	})
	if err != nil {
		t.Fatalf("resolve interrupt review: %v", err)
	}
	if resolved.Resolution == nil || resolved.Resolution.Decision != "keep_closed" {
		t.Fatalf("resolution = %#v", resolved.Resolution)
	}
	if resolved.RepoLane.ResetStatus != "released" {
		t.Fatalf("reset status = %q, want released", resolved.RepoLane.ResetStatus)
	}
	if resolved.RepoLane.OwnedRepoRoot != "" {
		t.Fatalf("owned repo root = %q, want empty after release", resolved.RepoLane.OwnedRepoRoot)
	}
	if _, err := os.Stat(originalOwnedRoot); !os.IsNotExist(err) {
		t.Fatalf("expected resolved keep_closed review to remove the prior owned root, stat err = %v", err)
	}

	task, err := service.Task(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("task detail: %v", err)
	}
	if !task.DispatchReadiness.Ready {
		t.Fatal("task should remain dispatchable after keep_closed review resolution")
	}
	if task.CurrentStory.Status != "no_active_run" {
		t.Fatalf("current story = %q, want no_active_run", task.CurrentStory.Status)
	}
}

func TestRedispatchReleasesPreviousTerminalOwnedLane(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T17:10:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	firstRun, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	originalOwnedRoot := firstRun.RepoLane.OwnedRepoRoot
	if _, err := os.Stat(originalOwnedRoot); err != nil {
		t.Fatalf("expected original owned root to exist: %v", err)
	}

	if _, err := service.InterruptRun(context.Background(), firstRun.RunID); err != nil {
		t.Fatalf("interrupt run: %v", err)
	}
	if _, err := service.ResolveInterruptReview(context.Background(), firstRun.RunID, InterruptReviewResolution{
		Decision:   "redispatch_ready",
		Summary:    "Human review approved another dispatch attempt.",
		ResolvedBy: "human",
	}); err != nil {
		t.Fatalf("resolve interrupt review: %v", err)
	}
	if _, err := os.Stat(originalOwnedRoot); !os.IsNotExist(err) {
		t.Fatalf("expected previous owned root to be removed before redispatch, stat err = %v", err)
	}

	secondRun, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("redispatch: %v", err)
	}
	if secondRun.RepoLane.OwnedRepoRoot == originalOwnedRoot {
		t.Fatalf("expected a fresh owned root, still got %q", secondRun.RepoLane.OwnedRepoRoot)
	}
	if _, err := os.Stat(originalOwnedRoot); !os.IsNotExist(err) {
		t.Fatalf("expected previous owned root to be removed, stat err = %v", err)
	}
	if _, err := os.Stat(secondRun.RepoLane.OwnedRepoRoot); err != nil {
		t.Fatalf("expected new owned root to exist: %v", err)
	}
}

func TestResolveInterruptReviewRejectsRunWithoutPendingReview(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T17:10:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if _, err := service.ResolveInterruptReview(context.Background(), run.RunID, InterruptReviewResolution{Decision: "redispatch_ready"}); err == nil {
		t.Fatal("expected interrupt review resolution to reject a run without pending review")
	}
}

func TestPokeRunCreatesPendingWorkerFollowUp(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T17:10:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	_, err = service.UpdateRun(context.Background(), run.RunID, TaskRunUpdate{
		State:           StateSleepingOrStalled,
		ReasonCode:      "progress_stale",
		StateSummary:    "Run has gone quiet past its expected progress window.",
		SuspiciousAfter: time.Now().UTC().Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("prime stalled run: %v", err)
	}

	poked, err := service.PokeRun(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("poke run: %v", err)
	}
	if poked.FollowUp == nil {
		t.Fatal("expected follow-up after poke")
	}
	if poked.FollowUp.Kind != "poke_worker_check" || poked.FollowUp.Status != "pending" {
		t.Fatalf("follow-up = %#v", poked.FollowUp)
	}
	if poked.Actions[ActionPoke].Allowed {
		t.Fatal("poke should be blocked while follow-up is pending")
	}
}

func TestUpdateRunCompletesPendingWorkerFollowUp(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T17:10:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	_, err = service.UpdateRun(context.Background(), run.RunID, TaskRunUpdate{
		State:        StateSleepingOrStalled,
		ReasonCode:   "poke_requested",
		StateSummary: "Run was poked and is waiting for a fresh backend progress signal.",
		FollowUp: &RunFollowUp{
			Kind:        "poke_worker_check",
			Owner:       "backend_worker",
			Status:      "pending",
			Summary:     "Execution worker should acknowledge the poke.",
			RequestedAt: time.Now().UTC(),
			DueAt:       time.Now().UTC().Add(5 * time.Minute),
		},
	})
	if err != nil {
		t.Fatalf("seed follow-up: %v", err)
	}

	updated, err := service.UpdateRun(context.Background(), run.RunID, TaskRunUpdate{
		State:               StateRunning,
		ReasonCode:          "worker_resumed",
		StateSummary:        "Run resumed after the worker follow-up.",
		LastProgressSummary: "Execution worker acknowledged the poke and resumed progress.",
	})
	if err != nil {
		t.Fatalf("complete follow-up: %v", err)
	}
	if updated.FollowUp == nil || updated.FollowUp.Status != "completed" {
		t.Fatalf("follow-up should be completed, got %#v", updated.FollowUp)
	}
}

func TestRunReadMarksPendingWorkerFollowUpOverdue(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0002",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T17:10:00-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	_, err = service.UpdateRun(context.Background(), run.RunID, TaskRunUpdate{
		State:        StateSleepingOrStalled,
		ReasonCode:   "poke_requested",
		StateSummary: "Run was poked and is waiting for a fresh backend progress signal.",
		FollowUp: &RunFollowUp{
			Kind:        "poke_worker_check",
			Owner:       "backend_worker",
			Status:      "pending",
			Summary:     "Execution worker should acknowledge the poke.",
			RequestedAt: time.Now().UTC().Add(-10 * time.Minute),
			DueAt:       time.Now().UTC().Add(-5 * time.Minute),
		},
	})
	if err != nil {
		t.Fatalf("seed overdue follow-up: %v", err)
	}

	supervised, err := service.Run(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("run detail: %v", err)
	}
	if supervised.FollowUp == nil || supervised.FollowUp.Status != "overdue" {
		t.Fatalf("expected overdue follow-up, got %#v", supervised.FollowUp)
	}
	if supervised.Attention.Level != AttentionUrgent {
		t.Fatalf("attention level = %q, want urgent", supervised.Attention.Level)
	}
}

func hasBlockReason(reasons []ActionBlockReason, code string) bool {
	for _, reason := range reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func hasLaunchTargetEnding(targets []LaunchTarget, label string, suffix string) bool {
	for _, target := range targets {
		if target.Label == label && strings.HasSuffix(filepath.ToSlash(target.URI), "/"+suffix) {
			return true
		}
	}
	return false
}

// closureRequestedAtRoot is the TEST-ONLY auto-close closure read: it reports true
// only when the OWNED worktree's Tracking/<taskID>/TASK-STATE.json sets current_gate
// to "closure", false for any other value, and false (no error) when the file is
// missing — so the consumer's auto-close never wedges on an absent announcement.
func TestClosureRequestedAtRoot(t *testing.T) {
	const taskID = "Task-0008"
	withState := func(t *testing.T, currentGate string) string {
		t.Helper()
		root := t.TempDir()
		taskDir := filepath.Join(root, "Tracking", taskID)
		if err := os.MkdirAll(taskDir, 0o755); err != nil {
			t.Fatalf("mkdir task dir: %v", err)
		}
		writeFile(t, filepath.Join(taskDir, "TASK-STATE.json"), `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0001",
  "current_gate": "`+currentGate+`",
  "blockers": [],
  "updated_at": "2026-04-24T16:27:00-04:00"
}`)
		return root
	}

	// current_gate=="closure" => announced completion.
	requested, err := closureRequestedAtRoot(withState(t, "closure"), taskID)
	if err != nil {
		t.Fatalf("closureRequestedAtRoot(closure): %v", err)
	}
	if !requested {
		t.Fatal("current_gate=closure must report ClosureRequested=true")
	}

	// any other gate => not announced.
	requested, err = closureRequestedAtRoot(withState(t, "implementation"), taskID)
	if err != nil {
		t.Fatalf("closureRequestedAtRoot(implementation): %v", err)
	}
	if requested {
		t.Fatal("current_gate=implementation must report ClosureRequested=false")
	}

	// missing TASK-STATE.json => false, nil (not an error).
	requested, err = closureRequestedAtRoot(t.TempDir(), taskID)
	if err != nil {
		t.Fatalf("closureRequestedAtRoot(missing file): %v", err)
	}
	if requested {
		t.Fatal("missing state file must report ClosureRequested=false without error")
	}
}

// ClosureRequested with no active owned lane is false, nil (the auto-close treats an
// absent lane as not-yet-announced rather than an error).
func TestClosureRequestedNoActiveLaneIsFalse(t *testing.T) {
	worktreeRoot := writeTaskTrackingRoot(t, map[string]taskFixture{})
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), nil)

	requested, err := service.ClosureRequested("Task-0008")
	if err != nil {
		t.Fatalf("ClosureRequested with no active lane: %v", err)
	}
	if requested {
		t.Fatal("ClosureRequested must be false when the task has no active owned lane")
	}
}

type taskFixture struct {
	taskMD        string
	taskState     string
	planMD        string
	handoffMD     string
	constraintsMD string
}

func writeTaskTrackingRoot(t *testing.T, tasks map[string]taskFixture) string {
	t.Helper()
	worktreeRoot := t.TempDir()
	trackingRoot := filepath.Join(worktreeRoot, "Tracking")
	if err := os.MkdirAll(trackingRoot, 0o755); err != nil {
		t.Fatalf("mkdir tracking root: %v", err)
	}

	for taskID, fixture := range tasks {
		taskRoot := filepath.Join(trackingRoot, taskID)
		if err := os.MkdirAll(taskRoot, 0o755); err != nil {
			t.Fatalf("mkdir task root: %v", err)
		}
		writeFile(t, filepath.Join(taskRoot, "TASK.md"), fixture.taskMD)
		writeFile(t, filepath.Join(taskRoot, "TASK-STATE.json"), fixture.taskState)
		if fixture.planMD != "" {
			writeFile(t, filepath.Join(taskRoot, "PLAN.md"), fixture.planMD)
		}
		if fixture.handoffMD != "" {
			writeFile(t, filepath.Join(taskRoot, "HANDOFF.md"), fixture.handoffMD)
		}
		if fixture.constraintsMD != "" {
			writeFile(t, filepath.Join(taskRoot, "CONSTRAINTS.md"), fixture.constraintsMD)
		}
	}

	return worktreeRoot
}

func writeGitTaskTrackingRoot(t *testing.T, tasks map[string]taskFixture) string {
	t.Helper()
	worktreeRoot := writeTaskTrackingRoot(t, tasks)
	runCommand(t, worktreeRoot, "git", "init")
	runCommand(t, worktreeRoot, "git", "config", "user.email", "taskrun-tests@example.com")
	runCommand(t, worktreeRoot, "git", "config", "user.name", "TaskRun Tests")
	runCommand(t, worktreeRoot, "git", "add", ".")
	runCommand(t, worktreeRoot, "git", "commit", "-m", "initial task fixtures")
	return worktreeRoot
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runCommand(t *testing.T, dir string, exe string, args ...string) {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", exe, args, err, string(output))
	}
}

func runOutput(t *testing.T, dir string, exe string, args ...string) string {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", exe, args, err, string(output))
	}
	return strings.TrimSpace(string(output))
}
