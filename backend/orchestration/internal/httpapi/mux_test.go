package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/config"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/controlplane"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/jobs"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/taskrun"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/testfixtures/taskrepo"
)

type fakeBackend struct {
	schedules map[string]controlplane.RuntimeSchedule
	started   []controlplane.JobRunRequest
}

type fakeTaskRuntime struct {
	activeByTask map[string]taskrun.TaskRunView
	byRunID      map[string]taskrun.TaskRunView
	started      []taskrun.StartTaskRunRequest
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{schedules: map[string]controlplane.RuntimeSchedule{}}
}

func newFakeTaskRuntime() *fakeTaskRuntime {
	return &fakeTaskRuntime{
		activeByTask: map[string]taskrun.TaskRunView{},
		byRunID:      map[string]taskrun.TaskRunView{},
	}
}

func (b *fakeBackend) ListManagedSchedules(context.Context) ([]controlplane.RuntimeSchedule, error) {
	schedules := make([]controlplane.RuntimeSchedule, 0, len(b.schedules))
	for _, schedule := range b.schedules {
		schedules = append(schedules, schedule)
	}
	return schedules, nil
}

func (b *fakeBackend) CreateSchedule(_ context.Context, desired controlplane.DesiredSchedule) error {
	b.schedules[desired.ScheduleID] = controlplane.RuntimeSchedule{
		ScheduleID:      desired.ScheduleID,
		JobID:           desired.JobID,
		TriggerIndex:    desired.TriggerIndex,
		CronExpressions: []string{desired.Cron},
		TimeZoneName:    desired.Timezone,
		ManagedCron:     desired.Cron,
		ManagedTimezone: desired.Timezone,
		ManagedSpecHash: desired.SpecHash,
		WorkflowType:    desired.WorkflowType,
		TaskQueue:       desired.TaskQueue,
	}
	return nil
}

func (b *fakeBackend) UpdateSchedule(_ context.Context, desired controlplane.DesiredSchedule) error {
	return b.CreateSchedule(context.Background(), desired)
}

func (b *fakeBackend) DeleteSchedule(_ context.Context, scheduleID string) error {
	delete(b.schedules, scheduleID)
	return nil
}

func (b *fakeBackend) Close() error {
	return nil
}

func (b *fakeBackend) StartJobRun(_ context.Context, request controlplane.JobRunRequest) (controlplane.StartedRun, error) {
	b.started = append(b.started, request)
	return controlplane.StartedRun{
		JobID:           request.JobID,
		TriggerType:     request.TriggerType,
		TriggerPath:     request.TriggerPath,
		DesiredSpecHash: request.DesiredSpecHash,
		RequestedAt:     request.RequestedAt,
		WorkflowID:      "workflow-id",
		RunID:           "run-id",
	}, nil
}

func (f *fakeTaskRuntime) StartTaskRun(_ context.Context, request taskrun.StartTaskRunRequest) (taskrun.TaskRunView, error) {
	f.started = append(f.started, request)
	state := taskrun.StateDispatching
	reasonCode := "dispatch_started"
	stateSummary := "Run is dispatching in an owned checkout."
	nextOwner := "backend"
	nextExpectedEvent := "Execution worker records the next task-run state update."
	attention := taskrun.AttentionPriority{Level: taskrun.AttentionWatch, Reason: "Run is active.", SortKey: "50-dispatching"}
	if request.RepoLane.CurrentCommit != "" {
		state = taskrun.StateRunning
		reasonCode = "owned_lane_bootstrapped"
		stateSummary = "Run bootstrapped the owned checkout and is ready for backend execution."
		nextOwner = "backend_worker"
		nextExpectedEvent = "Execution worker records the next progress checkpoint."
		attention = taskrun.AttentionPriority{Level: taskrun.AttentionWatch, Reason: "Run is active after owned-lane bootstrap.", SortKey: "45-owned_lane_bootstrapped"}
	}
	run := taskrun.TaskRunView{
		RunID:                  request.RunID,
		TaskID:                 request.TaskID,
		WorkflowID:             request.RunID,
		TemporalExecutionRunID: "temporal-run-id",
		Status:                 "active",
		StateEnvelope: taskrun.StateEnvelope{
			State:             state,
			ReasonCode:        reasonCode,
			StateSummary:      stateSummary,
			NextOwner:         nextOwner,
			NextExpectedEvent: nextExpectedEvent,
			SuspiciousAfter:   request.DispatchRequestedAt.Add(15 * time.Minute),
		},
		MeaningSummary:       request.MeaningSummary,
		Attention:            attention,
		RepoLane:             request.RepoLane,
		CapturedTaskSnapshot: request.CapturedTaskSnapshot,
		DeepContext:          request.ContextSnapshot,
	}
	f.activeByTask[request.TaskID] = run
	f.byRunID[request.RunID] = run
	return run, nil
}

func (f *fakeTaskRuntime) GetTaskRun(_ context.Context, runID string) (taskrun.TaskRunView, error) {
	run, ok := f.byRunID[runID]
	if !ok {
		return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
	}
	return run, nil
}

func (f *fakeTaskRuntime) GetActiveTaskRun(_ context.Context, runID string) (taskrun.TaskRunView, error) {
	for taskID, run := range f.activeByTask {
		if taskrun.ActiveRunID(taskID) == runID {
			return run, nil
		}
	}
	return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
}

func (f *fakeTaskRuntime) SetRunGateState(_ context.Context, runID string, state string) (taskrun.TaskRunView, error) {
	for taskID, run := range f.activeByTask {
		if taskrun.ActiveRunID(taskID) != runID {
			continue
		}
		if run.RepoLane.Binding == nil {
			run.RepoLane.Binding = &taskrun.RepoBinding{TaskID: run.TaskID, WorktreePath: run.RepoLane.OwnedRepoRoot, RunGateState: taskrun.RunGateStateRunning}
		}
		run.RepoLane.Binding.RunGateState = state
		f.activeByTask[taskID] = run
		if run.RunID != "" {
			f.byRunID[run.RunID] = run
		}
		return run, nil
	}
	return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
}

func (f *fakeTaskRuntime) BindLaunchedSession(_ context.Context, runID string, sessionID string, transcriptPath string, pid int) (taskrun.TaskRunView, error) {
	for taskID, run := range f.activeByTask {
		if taskrun.ActiveRunID(taskID) != runID {
			continue
		}
		if run.RepoLane.Binding == nil {
			run.RepoLane.Binding = &taskrun.RepoBinding{TaskID: run.TaskID, WorktreePath: run.RepoLane.OwnedRepoRoot, RunGateState: taskrun.RunGateStateRunning}
		}
		run.RepoLane.Binding.AgentSessionID = sessionID
		run.RepoLane.Binding.SessionTranscriptPath = transcriptPath
		run.RepoLane.Binding.LaunchedPID = pid
		f.activeByTask[taskID] = run
		if run.RunID != "" {
			f.byRunID[run.RunID] = run
		}
		return run, nil
	}
	return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
}

func (f *fakeTaskRuntime) ReconcileTaskSnapshot(_ context.Context, runID string, snapshot taskrun.TaskDefinitionSnapshot) (taskrun.TaskRunView, error) {
	run, ok := f.byRunID[runID]
	if !ok {
		return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
	}
	run.CapturedTaskSnapshot = snapshot
	f.byRunID[runID] = run
	f.activeByTask[run.TaskID] = run
	return run, nil
}

func (f *fakeTaskRuntime) UpdateTaskRun(_ context.Context, runID string, update taskrun.TaskRunUpdate) (taskrun.TaskRunView, error) {
	run, ok := f.byRunID[runID]
	if !ok {
		return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
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
	} else if update.State != "" && update.State != taskrun.StateWaitingForHuman {
		run.WaitContract = nil
	}
	if update.Attention != nil {
		run.Attention = *update.Attention
	}
	if update.FollowUp != nil {
		if update.FollowUp.Kind == "" &&
			update.FollowUp.Owner == "" &&
			update.FollowUp.Status == "" &&
			update.FollowUp.Summary == "" &&
			update.FollowUp.RequestedAt.IsZero() &&
			update.FollowUp.DueAt.IsZero() &&
			update.FollowUp.CompletedAt.IsZero() {
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
		run.StateEnvelope.ActionBlockReasons = map[string][]taskrun.ActionBlockReason{}
		for action, availability := range update.Actions {
			run.StateEnvelope.ActionBlockReasons[action] = append([]taskrun.ActionBlockReason(nil), availability.BlockReasons...)
		}
	}
	if update.RepoLane != nil {
		run.RepoLane = *update.RepoLane
	}
	if !update.CompletedAt.IsZero() {
		run.LastProgressAt = update.CompletedAt
	}
	if update.FailureSummary != "" {
		run.FailureSummary = update.FailureSummary
	} else if update.State != "" && update.State != taskrun.StateBlocked && update.State != taskrun.StateFailed {
		run.FailureSummary = ""
	}
	switch run.StateEnvelope.State {
	case taskrun.StateCompleted:
		run.Status = "completed"
	case taskrun.StateFailed:
		run.Status = "failed"
	case taskrun.StateInterrupted:
		run.Status = "interrupted"
	default:
		run.Status = "active"
	}
	f.byRunID[runID] = run
	f.activeByTask[run.TaskID] = run
	return run, nil
}

func (f *fakeTaskRuntime) RetryTaskRunWorkload(_ context.Context, runID string, request taskrun.WorkloadRetryRequest) (taskrun.TaskRunView, error) {
	run, ok := f.byRunID[runID]
	if !ok {
		return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
	}
	run.CapturedTaskSnapshot = request.CapturedTaskSnapshot
	run.RepoLane = request.RepoLane
	run.Status = "active"
	run.StateEnvelope.State = taskrun.StateRunning
	run.StateEnvelope.ReasonCode = "workload_retry_requested"
	run.StateEnvelope.StateSummary = "Backend reprovisioned a fresh owned lane and is retrying workload execution."
	run.StateEnvelope.NextOwner = "backend_worker"
	run.StateEnvelope.NextExpectedEvent = "Execution worker reruns the owned-lane workload path."
	run.StateEnvelope.SuspiciousAfter = request.RetryRequestedAt.Add(15 * time.Minute)
	run.Actions = map[string]taskrun.ActionAvailability{
		taskrun.ActionDispatch: {
			Allowed: false,
			BlockReasons: []taskrun.ActionBlockReason{{
				Code:    "active_run_exists",
				Summary: "Dispatch is blocked while this run owns the current live story.",
			}},
		},
		taskrun.ActionPoke: {
			Allowed: false,
			BlockReasons: []taskrun.ActionBlockReason{{
				Code:    "run_not_suspicious_yet",
				Summary: "Poke stays blocked until the run misses its next expected progress deadline.",
			}},
		},
		taskrun.ActionInterrupt: {Allowed: true},
	}
	run.StateEnvelope.ActionBlockReasons = map[string][]taskrun.ActionBlockReason{}
	for action, availability := range run.Actions {
		run.StateEnvelope.ActionBlockReasons[action] = append([]taskrun.ActionBlockReason(nil), availability.BlockReasons...)
	}
	run.FailureSummary = ""
	run.FollowUp = nil
	run.Resolution = nil
	run.LastProgressAt = request.RetryRequestedAt
	run.LastProgressSummary = "Backend requested a workload retry with a fresh owned lane."
	run.Attention = taskrun.AttentionPriority{
		Level:   taskrun.AttentionWatch,
		Reason:  "Run is active after the backend reprovisioned a fresh owned lane.",
		SortKey: "36-workload_retry_requested",
	}
	f.byRunID[runID] = run
	f.activeByTask[run.TaskID] = run
	return run, nil
}

func TestMuxExposesHealthJobsAndSync(t *testing.T) {
	root := writeJobsRoot(t, []jobs.Spec{
		{
			APIVersion:   jobs.APIVersion,
			JobID:        "codex-daily-ue-determinism-digest",
			Label:        "UE Determinism Digest",
			Description:  "Daily digest",
			DesiredState: jobs.DesiredStateEnabled,
			Triggers: []jobs.Trigger{
				{Type: jobs.TriggerTypeSchedule, Cron: "0 4 * * *", Timezone: "America/Toronto"},
				{Type: jobs.TriggerTypeManual},
				{Type: jobs.TriggerTypeWebhook, Path: "digests/ue-determinism"},
			},
			Executor: jobs.Executor{
				Type:       jobs.ExecutorTypeCodexExec,
				Cwd:        `C:\Users\gregs\.codex`,
				Entrypoint: "ue-determinism-digest",
			},
			Runtime: jobs.RuntimeConfig{
				WorkflowType: "codex.exec.job",
				TaskQueue:    "codex-orchestration",
			},
		},
	})

	service := controlplane.NewService(root, newFakeBackend())
	taskRuntime := newFakeTaskRuntime()
	worktreeRoot := writeTaskTrackingRoot(t)
	taskService := taskrun.NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), taskRuntime)
	mux := NewMux(config.Config{
		BindAddress:     "127.0.0.1:4318",
		JobsRoot:        root,
		WorktreeRoot:    worktreeRoot,
		TrackingRoot:    filepath.Join(worktreeRoot, "Tracking"),
		Namespace:       "default",
		TaskQueue:       "codex-orchestration",
		TemporalAddress: "127.0.0.1:7233",
	}, service, taskService, nil)

	syncRequest := httptest.NewRequest(http.MethodPost, "/sync", nil)
	syncResponse := httptest.NewRecorder()
	mux.ServeHTTP(syncResponse, syncRequest)
	if syncResponse.Code != http.StatusOK {
		t.Fatalf("POST /sync status = %d, want 200", syncResponse.Code)
	}

	jobsRequest := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	jobsResponse := httptest.NewRecorder()
	mux.ServeHTTP(jobsResponse, jobsRequest)
	if jobsResponse.Code != http.StatusOK {
		t.Fatalf("GET /jobs status = %d, want 200", jobsResponse.Code)
	}
	var jobsPayload struct {
		Jobs []controlplane.JobView `json:"jobs"`
	}
	if err := json.Unmarshal(jobsResponse.Body.Bytes(), &jobsPayload); err != nil {
		t.Fatalf("decode /jobs: %v", err)
	}
	if len(jobsPayload.Jobs) != 1 {
		t.Fatalf("jobs payload count = %d, want 1", len(jobsPayload.Jobs))
	}

	jobDetailRequest := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/codex-daily-ue-determinism-digest", nil)
	jobDetailResponse := httptest.NewRecorder()
	mux.ServeHTTP(jobDetailResponse, jobDetailRequest)
	if jobDetailResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/jobs/{id} status = %d, want 200", jobDetailResponse.Code)
	}

	jobsAPIRequest := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	jobsAPIResponse := httptest.NewRecorder()
	mux.ServeHTTP(jobsAPIResponse, jobsAPIRequest)
	if jobsAPIResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/jobs status = %d, want 200", jobsAPIResponse.Code)
	}

	healthRequest := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthResponse := httptest.NewRecorder()
	mux.ServeHTTP(healthResponse, healthRequest)
	if healthResponse.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want 200", healthResponse.Code)
	}

	tasksRequest := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	tasksResponse := httptest.NewRecorder()
	mux.ServeHTTP(tasksResponse, tasksRequest)
	if tasksResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/tasks status = %d, want 200", tasksResponse.Code)
	}

	taskDetailRequest := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/Task-0008", nil)
	taskDetailResponse := httptest.NewRecorder()
	mux.ServeHTTP(taskDetailResponse, taskDetailRequest)
	if taskDetailResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/tasks/{id} status = %d, want 200", taskDetailResponse.Code)
	}
	var taskDetail taskrun.TaskView
	if err := json.Unmarshal(taskDetailResponse.Body.Bytes(), &taskDetail); err != nil {
		t.Fatalf("decode task detail: %v", err)
	}
	if taskDetail.DeepContext == nil || taskDetail.DeepContext.PreferredLaunchTarget == nil {
		t.Fatalf("task deep context = %#v", taskDetail.DeepContext)
	}

	seedWorktreeViaMux(t, mux)
	dispatchRequest := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/Task-0008/dispatch", nil)
	dispatchResponse := httptest.NewRecorder()
	mux.ServeHTTP(dispatchResponse, dispatchRequest)
	if dispatchResponse.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v1/tasks/{id}/dispatch status = %d, want 202", dispatchResponse.Code)
	}

	taskRunRequest := httptest.NewRequest(http.MethodGet, "/api/v1/task-runs/"+taskrun.ActiveRunID("Task-0008"), nil)
	taskRunResponse := httptest.NewRecorder()
	mux.ServeHTTP(taskRunResponse, taskRunRequest)
	if taskRunResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/task-runs/{id} status = %d, want 200", taskRunResponse.Code)
	}
	var taskRun taskrun.TaskRunView
	if err := json.Unmarshal(taskRunResponse.Body.Bytes(), &taskRun); err != nil {
		t.Fatalf("decode task run: %v", err)
	}
	if taskRun.DeepContext == nil || len(taskRun.DeepContext.LaunchTargets) == 0 {
		t.Fatalf("run deep context = %#v", taskRun.DeepContext)
	}

	failureDispatchResponse := httptest.NewRecorder()
	seedWorktreeViaMux(t, mux)
	mux.ServeHTTP(failureDispatchResponse, httptest.NewRequest(http.MethodPost, "/api/v1/tasks/Task-0008/dispatch-workload-failure-exercise", nil))
	if failureDispatchResponse.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/v1/tasks/{id}/dispatch-workload-failure-exercise status = %d, want 400 while active run exists", failureDispatchResponse.Code)
	}

	updateBody := strings.NewReader(`{
  "state": "waiting_for_human",
  "reason_code": "review_required",
  "state_summary": "Run is waiting for human review.",
  "next_owner": "human",
  "next_expected_event": "Approve the next backend action.",
  "wait_contract": {
    "waiting_on": "human_review",
    "why_blocked": "The next backend action needs human approval.",
    "resume_when": "The human approves the next backend action.",
    "human_action_required": true,
    "human_action_target": {
      "kind": "approval_action",
      "label": "Approve backend review step",
      "uri": "approval://taskrun/Task-0008"
    }
  }
}`)
	updateRequest := httptest.NewRequest(http.MethodPost, "/api/v1/task-runs/"+taskrun.ActiveRunID("Task-0008")+"/state", updateBody)
	updateRequest.Header.Set("Content-Type", "application/json")
	updateResponse := httptest.NewRecorder()
	mux.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v1/task-runs/{id}/state status = %d, want 202", updateResponse.Code)
	}

	pokeResponse := httptest.NewRecorder()
	mux.ServeHTTP(pokeResponse, httptest.NewRequest(http.MethodPost, "/api/v1/task-runs/"+taskrun.ActiveRunID("Task-0008")+"/poke", nil))
	if pokeResponse.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/v1/task-runs/{id}/poke status = %d, want 400 while run is waiting for human approval", pokeResponse.Code)
	}

	interruptResponse := httptest.NewRecorder()
	mux.ServeHTTP(interruptResponse, httptest.NewRequest(http.MethodPost, "/api/v1/task-runs/"+taskrun.ActiveRunID("Task-0008")+"/interrupt", nil))
	if interruptResponse.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v1/task-runs/{id}/interrupt status = %d, want 202", interruptResponse.Code)
	}

	runsRequest := httptest.NewRequest(http.MethodGet, "/runs?job_id=codex-daily-ue-determinism-digest", nil)
	runsResponse := httptest.NewRecorder()
	mux.ServeHTTP(runsResponse, runsRequest)
	if runsResponse.Code != http.StatusOK {
		t.Fatalf("GET /runs status = %d, want 200", runsResponse.Code)
	}

	runsAPIRequest := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/codex-daily-ue-determinism-digest/runs", nil)
	runsAPIResponse := httptest.NewRecorder()
	mux.ServeHTTP(runsAPIResponse, runsAPIRequest)
	if runsAPIResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/jobs/{id}/runs status = %d, want 200", runsAPIResponse.Code)
	}

	runNowRequest := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/codex-daily-ue-determinism-digest/run", nil)
	runNowResponse := httptest.NewRecorder()
	mux.ServeHTTP(runNowResponse, runNowRequest)
	if runNowResponse.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v1/jobs/{id}/run status = %d, want 202", runNowResponse.Code)
	}

	webhookRequest := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/digests/ue-determinism", nil)
	webhookResponse := httptest.NewRecorder()
	mux.ServeHTTP(webhookResponse, webhookRequest)
	if webhookResponse.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v1/webhooks/{path} status = %d, want 202", webhookResponse.Code)
	}
}

func TestTasksAPIUsesCanonicalFixtureRepoForTaskSurfaceSmoke(t *testing.T) {
	repo := taskrepo.WriteCanonicalGitRepo(t)
	taskRuntime := newFakeTaskRuntime()
	taskService := taskrun.NewService(repo.Root, filepath.Join(repo.Root, ".runs"), taskRuntime)
	mux := NewMux(config.Config{
		BindAddress:     "127.0.0.1:4318",
		JobsRoot:        t.TempDir(),
		WorktreeRoot:    repo.Root,
		TrackingRoot:    repo.TrackingRoot,
		Namespace:       "default",
		TaskQueue:       "codex-orchestration",
		TemporalAddress: "127.0.0.1:7233",
	}, controlplane.NewService(t.TempDir(), newFakeBackend()), taskService, nil)

	listResponse := httptest.NewRecorder()
	mux.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/tasks status = %d, want 200", listResponse.Code)
	}
	var listPayload struct {
		Tasks []taskrun.TaskView `json:"tasks"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("decode task list: %v", err)
	}
	byID := map[string]taskrun.TaskView{}
	for _, task := range listPayload.Tasks {
		byID[task.TaskID] = task
	}
	if byID["Task-0008"].DeclaredGitRevision != repo.InitialCommit {
		t.Fatalf("Task-0008 git revision = %q, want %q", byID["Task-0008"].DeclaredGitRevision, repo.InitialCommit)
	}
	if byID["Task-0009"].StateEnvelope.State != taskrun.StateCompleted {
		t.Fatalf("Task-0009 state = %q, want completed", byID["Task-0009"].StateEnvelope.State)
	}

	dispatchResponse := httptest.NewRecorder()
	seedWorktreeViaMux(t, mux)
	mux.ServeHTTP(dispatchResponse, httptest.NewRequest(http.MethodPost, "/api/v1/tasks/Task-0008/dispatch", nil))
	if dispatchResponse.Code != http.StatusAccepted {
		t.Fatalf("dispatch status = %d, want 202", dispatchResponse.Code)
	}

	postDispatchResponse := httptest.NewRecorder()
	mux.ServeHTTP(postDispatchResponse, httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil))
	if postDispatchResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/tasks after dispatch status = %d, want 200", postDispatchResponse.Code)
	}
	listPayload.Tasks = nil
	if err := json.Unmarshal(postDispatchResponse.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("decode post-dispatch task list: %v", err)
	}
	byID = map[string]taskrun.TaskView{}
	for _, task := range listPayload.Tasks {
		byID[task.TaskID] = task
	}
	task8 := byID["Task-0008"]
	if task8.LatestRun == nil {
		t.Fatal("Task-0008 should expose latest run after dispatch")
	}
	if !task8.LatestRun.Actions[taskrun.ActionInterrupt].Allowed {
		t.Fatal("Task-0008 latest run should expose interrupt availability for visible Pause")
	}
	if task8.Actions[taskrun.ActionDispatch].Allowed {
		t.Fatal("Task-0008 should block dispatch while active run exists")
	}
}

func TestMuxExposesRetryCleanupRoute(t *testing.T) {
	taskRuntime := newFakeTaskRuntime()
	worktreeRoot := writeTaskTrackingRoot(t)
	taskService := taskrun.NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), taskRuntime)
	mux := NewMux(config.Config{
		BindAddress:     "127.0.0.1:4318",
		JobsRoot:        t.TempDir(),
		WorktreeRoot:    worktreeRoot,
		TrackingRoot:    filepath.Join(worktreeRoot, "Tracking"),
		Namespace:       "default",
		TaskQueue:       "codex-orchestration",
		TemporalAddress: "127.0.0.1:7233",
	}, controlplane.NewService(t.TempDir(), newFakeBackend()), taskService, nil)

	dispatchResponse := httptest.NewRecorder()
	seedWorktreeViaMux(t, mux)
	mux.ServeHTTP(dispatchResponse, httptest.NewRequest(http.MethodPost, "/api/v1/tasks/Task-0008/dispatch", nil))
	if dispatchResponse.Code != http.StatusAccepted {
		t.Fatalf("dispatch status = %d, want 202", dispatchResponse.Code)
	}

	runID := taskrun.ActiveRunID("Task-0008")
	current := taskRuntime.byRunID[runID]
	validOwnedRoot := current.RepoLane.OwnedRepoRoot
	badRoot := filepath.Join(worktreeRoot, "outside-owned-lane")
	if err := os.MkdirAll(badRoot, 0o755); err != nil {
		t.Fatalf("mkdir bad root: %v", err)
	}
	current.StateEnvelope.State = taskrun.StateRunning
	current.RepoLane.OwnedRepoRoot = badRoot
	taskRuntime.byRunID[runID] = current
	taskRuntime.activeByTask[current.TaskID] = current

	interruptResponse := httptest.NewRecorder()
	mux.ServeHTTP(interruptResponse, httptest.NewRequest(http.MethodPost, "/api/v1/task-runs/"+runID+"/interrupt", nil))
	if interruptResponse.Code != http.StatusAccepted {
		t.Fatalf("interrupt status = %d, want 202", interruptResponse.Code)
	}
	var interrupted taskrun.TaskRunView
	if err := json.Unmarshal(interruptResponse.Body.Bytes(), &interrupted); err != nil {
		t.Fatalf("decode interrupt response: %v", err)
	}
	if interrupted.StateEnvelope.ReasonCode != "interrupt_cleanup_blocked" {
		t.Fatalf("interrupt reason = %q", interrupted.StateEnvelope.ReasonCode)
	}

	repairedSeed := taskRuntime.byRunID[runID]
	repairedSeed.RepoLane.OwnedRepoRoot = validOwnedRoot
	taskRuntime.byRunID[runID] = repairedSeed
	taskRuntime.activeByTask[repairedSeed.TaskID] = repairedSeed

	retryResponse := httptest.NewRecorder()
	mux.ServeHTTP(retryResponse, httptest.NewRequest(http.MethodPost, "/api/v1/task-runs/"+runID+"/retry-cleanup", nil))
	if retryResponse.Code != http.StatusAccepted {
		t.Fatalf("retry-cleanup status = %d, want 202", retryResponse.Code)
	}
	var repaired taskrun.TaskRunView
	if err := json.Unmarshal(retryResponse.Body.Bytes(), &repaired); err != nil {
		t.Fatalf("decode retry-cleanup response: %v", err)
	}
	if repaired.StateEnvelope.ReasonCode != "interrupt_cleanup_repaired" {
		t.Fatalf("retry-cleanup reason = %q", repaired.StateEnvelope.ReasonCode)
	}
	if repaired.FollowUp == nil || repaired.FollowUp.Kind != "interrupt_review" {
		t.Fatalf("follow-up = %#v", repaired.FollowUp)
	}
	if repaired.RepoLane.ResetStatus != "restored" {
		t.Fatalf("reset status = %q", repaired.RepoLane.ResetStatus)
	}
}

func TestMuxExposesWorkloadFailureExerciseDispatchRoute(t *testing.T) {
	taskRuntime := newFakeTaskRuntime()
	worktreeRoot := writeTaskTrackingRoot(t)
	taskService := taskrun.NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), taskRuntime)
	mux := NewMux(config.Config{
		BindAddress:     "127.0.0.1:4318",
		JobsRoot:        t.TempDir(),
		WorktreeRoot:    worktreeRoot,
		TrackingRoot:    filepath.Join(worktreeRoot, "Tracking"),
		Namespace:       "default",
		TaskQueue:       "codex-orchestration",
		TemporalAddress: "127.0.0.1:7233",
	}, controlplane.NewService(t.TempDir(), newFakeBackend()), taskService, nil)

	response := httptest.NewRecorder()
	seedWorktreeViaMux(t, mux)
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/tasks/Task-0008/dispatch-workload-failure-exercise", nil))
	if response.Code != http.StatusAccepted {
		t.Fatalf("dispatch-workload-failure-exercise status = %d, want 202", response.Code)
	}
	if len(taskRuntime.started) != 1 {
		t.Fatalf("started requests = %d, want 1", len(taskRuntime.started))
	}
	request := taskRuntime.started[0]
	if request.ExecutionDirective == nil {
		t.Fatal("expected execution directive")
	}
	if request.ExecutionDirective.FailureMode != taskrun.ExecutionFailureModeTask0008WorkloadFailureOnce {
		t.Fatalf("failure mode = %q", request.ExecutionDirective.FailureMode)
	}
}

func TestMuxExposesRetryWorkloadRoute(t *testing.T) {
	taskRuntime := newFakeTaskRuntime()
	worktreeRoot := writeTaskTrackingRoot(t)
	taskService := taskrun.NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), taskRuntime)
	mux := NewMux(config.Config{
		BindAddress:     "127.0.0.1:4318",
		JobsRoot:        t.TempDir(),
		WorktreeRoot:    worktreeRoot,
		TrackingRoot:    filepath.Join(worktreeRoot, "Tracking"),
		Namespace:       "default",
		TaskQueue:       "codex-orchestration",
		TemporalAddress: "127.0.0.1:7233",
	}, controlplane.NewService(t.TempDir(), newFakeBackend()), taskService, nil)

	dispatchResponse := httptest.NewRecorder()
	seedWorktreeViaMux(t, mux)
	mux.ServeHTTP(dispatchResponse, httptest.NewRequest(http.MethodPost, "/api/v1/tasks/Task-0008/dispatch", nil))
	if dispatchResponse.Code != http.StatusAccepted {
		t.Fatalf("dispatch status = %d, want 202", dispatchResponse.Code)
	}

	runID := taskrun.ActiveRunID("Task-0008")
	current := taskRuntime.byRunID[runID]
	originalOwnedRoot := current.RepoLane.OwnedRepoRoot
	updateBody := strings.NewReader(`{
  "state": "blocked",
  "reason_code": "workload_execution_failed",
  "state_summary": "Run could not execute the prepared workload step inside the owned lane.",
  "next_owner": "human_or_supervisor",
  "next_expected_event": "Retry the workload path with a fresh owned lane."
}`)
	updateRequest := httptest.NewRequest(http.MethodPost, "/api/v1/task-runs/"+runID+"/state", updateBody)
	updateRequest.Header.Set("Content-Type", "application/json")
	updateResponse := httptest.NewRecorder()
	mux.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusAccepted {
		t.Fatalf("state update status = %d, want 202", updateResponse.Code)
	}
	var seeded taskrun.TaskRunView
	if err := json.Unmarshal(updateResponse.Body.Bytes(), &seeded); err != nil {
		t.Fatalf("decode state update response: %v", err)
	}
	if seeded.FollowUp == nil || seeded.FollowUp.Kind != "workload_recovery" || seeded.FollowUp.Status != "pending" {
		t.Fatalf("seeded follow-up = %#v", seeded.FollowUp)
	}

	retryResponse := httptest.NewRecorder()
	mux.ServeHTTP(retryResponse, httptest.NewRequest(http.MethodPost, "/api/v1/task-runs/"+runID+"/retry-workload", nil))
	if retryResponse.Code != http.StatusAccepted {
		t.Fatalf("retry-workload status = %d, want 202", retryResponse.Code)
	}
	var retried taskrun.TaskRunView
	if err := json.Unmarshal(retryResponse.Body.Bytes(), &retried); err != nil {
		t.Fatalf("decode retry-workload response: %v", err)
	}
	if retried.StateEnvelope.ReasonCode != "workload_retry_requested" {
		t.Fatalf("reason code = %q", retried.StateEnvelope.ReasonCode)
	}
	if retried.RepoLane.OwnedRepoRoot == "" || retried.RepoLane.OwnedRepoRoot == originalOwnedRoot {
		t.Fatalf("owned repo root = %q, want fresh lane replacing %q", retried.RepoLane.OwnedRepoRoot, originalOwnedRoot)
	}
	if retried.FailureSummary != "" {
		t.Fatalf("failure summary = %q, want cleared", retried.FailureSummary)
	}
	if retried.FollowUp != nil {
		t.Fatalf("follow-up should be cleared after retry, got %#v", retried.FollowUp)
	}
	if _, err := os.Stat(originalOwnedRoot); !os.IsNotExist(err) {
		t.Fatalf("old owned lane should be removed, stat err = %v", err)
	}
}

func TestMuxExposesInterruptReviewResolutionRoute(t *testing.T) {
	taskRuntime := newFakeTaskRuntime()
	worktreeRoot := writeTaskTrackingRoot(t)
	taskService := taskrun.NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), taskRuntime)
	mux := NewMux(config.Config{
		BindAddress:     "127.0.0.1:4318",
		JobsRoot:        t.TempDir(),
		WorktreeRoot:    worktreeRoot,
		TrackingRoot:    filepath.Join(worktreeRoot, "Tracking"),
		Namespace:       "default",
		TaskQueue:       "codex-orchestration",
		TemporalAddress: "127.0.0.1:7233",
	}, controlplane.NewService(t.TempDir(), newFakeBackend()), taskService, nil)

	dispatchResponse := httptest.NewRecorder()
	seedWorktreeViaMux(t, mux)
	mux.ServeHTTP(dispatchResponse, httptest.NewRequest(http.MethodPost, "/api/v1/tasks/Task-0008/dispatch", nil))
	if dispatchResponse.Code != http.StatusAccepted {
		t.Fatalf("dispatch status = %d, want 202", dispatchResponse.Code)
	}

	runID := taskrun.ActiveRunID("Task-0008")
	interruptResponse := httptest.NewRecorder()
	mux.ServeHTTP(interruptResponse, httptest.NewRequest(http.MethodPost, "/api/v1/task-runs/"+runID+"/interrupt", nil))
	if interruptResponse.Code != http.StatusAccepted {
		t.Fatalf("interrupt status = %d, want 202", interruptResponse.Code)
	}

	taskResponse := httptest.NewRecorder()
	mux.ServeHTTP(taskResponse, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/Task-0008", nil))
	if taskResponse.Code != http.StatusOK {
		t.Fatalf("task detail status = %d, want 200", taskResponse.Code)
	}
	var taskView taskrun.TaskView
	if err := json.Unmarshal(taskResponse.Body.Bytes(), &taskView); err != nil {
		t.Fatalf("decode task detail: %v", err)
	}
	if taskView.DispatchReadiness.Ready {
		t.Fatal("dispatch should stay blocked while interrupt review is pending")
	}

	body := strings.NewReader(`{
  "decision": "redispatch_ready",
  "summary": "Human review approved another dispatch attempt.",
  "resolved_by": "human"
}`)
	resolveResponse := httptest.NewRecorder()
	resolveRequest := httptest.NewRequest(http.MethodPost, "/api/v1/task-runs/"+runID+"/resolve-interrupt-review", body)
	resolveRequest.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(resolveResponse, resolveRequest)
	if resolveResponse.Code != http.StatusAccepted {
		t.Fatalf("resolve-interrupt-review status = %d, want 202", resolveResponse.Code)
	}
	var resolved taskrun.TaskRunView
	if err := json.Unmarshal(resolveResponse.Body.Bytes(), &resolved); err != nil {
		t.Fatalf("decode resolve response: %v", err)
	}
	if resolved.Resolution == nil || resolved.Resolution.Decision != "redispatch_ready" {
		t.Fatalf("resolution = %#v", resolved.Resolution)
	}
	if resolved.FollowUp == nil || resolved.FollowUp.Status != "completed" {
		t.Fatalf("follow-up = %#v", resolved.FollowUp)
	}

	readyTaskResponse := httptest.NewRecorder()
	mux.ServeHTTP(readyTaskResponse, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/Task-0008", nil))
	if readyTaskResponse.Code != http.StatusOK {
		t.Fatalf("task detail status after resolution = %d, want 200", readyTaskResponse.Code)
	}
	if err := json.Unmarshal(readyTaskResponse.Body.Bytes(), &taskView); err != nil {
		t.Fatalf("decode task detail after resolution: %v", err)
	}
	if !taskView.DispatchReadiness.Ready {
		t.Fatal("dispatch should be ready after interrupt review resolution")
	}
}

func writeJobsRoot(t *testing.T, specs []jobs.Spec) string {
	t.Helper()
	root := t.TempDir()
	specsDir := filepath.Join(root, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	for _, spec := range specs {
		path := filepath.Join(specsDir, spec.JobID+".json")
		raw, err := json.Marshal(spec)
		if err != nil {
			t.Fatalf("marshal spec: %v", err)
		}
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatalf("write spec %s: %v", path, err)
		}
	}
	return root
}

func writeTaskTrackingRoot(t *testing.T) string {
	t.Helper()
	worktreeRoot := t.TempDir()
	taskRoot := filepath.Join(worktreeRoot, "Tracking", "Task-0008")
	if err := os.MkdirAll(taskRoot, 0o755); err != nil {
		t.Fatalf("mkdir task root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskRoot, "TASK.md"), []byte(`# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`), 0o644); err != nil {
		t.Fatalf("write TASK.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskRoot, "PLAN.md"), []byte("# plan\n"), 0o644); err != nil {
		t.Fatalf("write PLAN.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskRoot, "TASK-STATE.json"), []byte(`{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0001",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T16:44:31-04:00"
}`), 0o644); err != nil {
		t.Fatalf("write TASK-STATE.json: %v", err)
	}
	runCommand(t, worktreeRoot, "git", "init")
	runCommand(t, worktreeRoot, "git", "config", "user.email", "httpapi-tests@example.com")
	runCommand(t, worktreeRoot, "git", "config", "user.name", "HTTP API Tests")
	runCommand(t, worktreeRoot, "git", "add", ".")
	runCommand(t, worktreeRoot, "git", "commit", "-m", "initial task fixtures")
	return worktreeRoot
}

func runCommand(t *testing.T, dir string, exe string, args ...string) {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", exe, args, err, string(output))
	}
}

// seedWorktreeViaMux provisions one idle pool worktree through the create endpoint so a
// pool-draw dispatch has a worktree to draw (Task-0016: dispatch no longer provisions a
// fresh dir — it draws from the pre-created pool). The mux's worktree root must be a real
// git repo (writeTaskTrackingRoot).
func seedWorktreeViaMux(t *testing.T, mux *http.ServeMux) {
	t.Helper()
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/v1/worktrees/create",
		strings.NewReader(`{"repo":"repo"}`)))
	if resp.Code != http.StatusCreated {
		t.Fatalf("seed worktree via create endpoint status = %d, want 201: %s", resp.Code, resp.Body.String())
	}
}
