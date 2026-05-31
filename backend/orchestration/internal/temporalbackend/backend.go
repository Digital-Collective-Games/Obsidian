package temporalbackend

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/worker"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/config"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/controlplane"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/jobexec"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/queue"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/taskexec"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/taskrun"
)

const managedBy = "codex-orchestration"

type Backend struct {
	client client.Client
	cfg    config.Config
}

func New(cfg config.Config) (*Backend, error) {
	temporalClient, err := client.Dial(client.Options{
		HostPort:  cfg.TemporalAddress,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("dial temporal: %w", err)
	}

	return &Backend{client: temporalClient, cfg: cfg}, nil
}

func (b *Backend) Close() error {
	if b.client != nil {
		b.client.Close()
	}
	return nil
}

func (b *Backend) ListManagedSchedules(ctx context.Context) ([]controlplane.RuntimeSchedule, error) {
	iter, err := b.client.ScheduleClient().List(ctx, client.ScheduleListOptions{PageSize: 100})
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}

	schedules := make([]controlplane.RuntimeSchedule, 0)
	for iter.HasNext() {
		entry, err := iter.Next()
		if err != nil {
			return nil, fmt.Errorf("iterate schedules: %w", err)
		}
		if !strings.HasPrefix(entry.ID, "codex-job--") {
			continue
		}

		desc, err := b.client.ScheduleClient().GetHandle(ctx, entry.ID).Describe(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe schedule %s: %w", entry.ID, err)
		}

		schedule, err := scheduleFromDescription(entry.ID, desc)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, schedule)
	}

	return schedules, nil
}

func (b *Backend) CreateSchedule(ctx context.Context, desired controlplane.DesiredSchedule) error {
	_, err := b.client.ScheduleClient().Create(ctx, buildScheduleOptions(desired))
	if err != nil {
		return fmt.Errorf("create schedule %s: %w", desired.ScheduleID, err)
	}
	return nil
}

func (b *Backend) UpdateSchedule(ctx context.Context, desired controlplane.DesiredSchedule) error {
	handle := b.client.ScheduleClient().GetHandle(ctx, desired.ScheduleID)
	err := handle.Update(ctx, client.ScheduleUpdateOptions{
		DoUpdate: func(client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
			schedule := buildSchedule(desired)
			return &client.ScheduleUpdate{Schedule: &schedule}, nil
		},
	})
	if err != nil {
		return fmt.Errorf("update schedule %s: %w", desired.ScheduleID, err)
	}
	return nil
}

func (b *Backend) DeleteSchedule(ctx context.Context, scheduleID string) error {
	err := b.client.ScheduleClient().GetHandle(ctx, scheduleID).Delete(ctx)
	if err != nil {
		return fmt.Errorf("delete schedule %s: %w", scheduleID, err)
	}
	return nil
}

func (b *Backend) StartJobRun(ctx context.Context, request controlplane.JobRunRequest) (controlplane.StartedRun, error) {
	workflowID := fmt.Sprintf("job/%s/%s/%s", request.JobID, request.TriggerType, uuid.NewString())
	workflowRun, err := b.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: request.Spec.Runtime.TaskQueue,
	}, request.Spec.Runtime.WorkflowType, request)
	if err != nil {
		return controlplane.StartedRun{}, fmt.Errorf("start workflow %s: %w", workflowID, err)
	}

	return controlplane.StartedRun{
		JobID:           request.JobID,
		TriggerType:     request.TriggerType,
		TriggerPath:     request.TriggerPath,
		DesiredSpecHash: request.DesiredSpecHash,
		RequestedAt:     request.RequestedAt,
		WorkflowID:      workflowRun.GetID(),
		RunID:           workflowRun.GetRunID(),
	}, nil
}

func (b *Backend) StartWorker(cfg config.Config, taskService *taskrun.Service) (worker.Worker, error) {
	w := worker.New(b.client, cfg.TaskQueue, worker.Options{})
	jobexec.Register(w, cfg)
	taskexec.Register(w)

	// O3 (registry-driven): register the queue-drain consumer workflow next to
	// taskexec.Register (A3.4). The poll activity is wired to a live registry-driven
	// consumer that enumerates ALL registered repos from the central registry
	// (cfg.RegistryPath) and builds a per-repo provider + taskrun.Service (runtime =
	// this Backend) for each. It is wired only when a registry path is configured;
	// otherwise the workflow is registered but stays dormant.
	drainActivities, err := newQueueDrainActivities(cfg, b)
	if err != nil {
		return nil, err
	}
	queue.Register(w, drainActivities)

	if err := w.Start(); err != nil {
		return nil, fmt.Errorf("start worker: %w", err)
	}
	return w, nil
}

func (b *Backend) StartTaskRun(ctx context.Context, request taskrun.StartTaskRunRequest) (taskrun.TaskRunView, error) {
	workflowRun, err := b.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       request.RunID,
		TaskQueue:                b.cfg.TaskQueue,
		WorkflowExecutionTimeout: 30 * 24 * time.Hour,
		WorkflowIDReusePolicy:    enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	}, taskexec.TaskRunWorkflowName, request)
	if err != nil {
		return taskrun.TaskRunView{}, fmt.Errorf("start task run %s: %w", request.RunID, err)
	}

	return taskexec.InitialView(request, workflowRun.GetID(), workflowRun.GetRunID()), nil
}

func (b *Backend) GetTaskRun(ctx context.Context, runID string) (taskrun.TaskRunView, error) {
	response, err := b.client.QueryWorkflow(ctx, runID, "", taskexec.TaskRunStateQueryName)
	if err != nil {
		if isTemporalNotFound(err) {
			return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
		}
		return taskrun.TaskRunView{}, fmt.Errorf("query task run %s: %w", runID, err)
	}

	var view taskrun.TaskRunView
	if err := response.Get(&view); err != nil {
		return taskrun.TaskRunView{}, fmt.Errorf("decode task run %s query result: %w", runID, err)
	}
	return view, nil
}

func (b *Backend) GetActiveTaskRun(ctx context.Context, runID string) (taskrun.TaskRunView, error) {
	return b.GetTaskRun(ctx, runID)
}

// SetRunGateState signals the per-run workflow to set its durable run/gate label
// (Landing 2: the workflow is the sole live writer, not the JSON side-store) and waits
// until the query reflects it. ErrRunNotFound when the run is gone (so the Service can
// map it to ErrNoActiveOwnedLane, matching the legacy file path's tolerance).
func (b *Backend) SetRunGateState(ctx context.Context, runID string, state string) (taskrun.TaskRunView, error) {
	if err := b.client.SignalWorkflow(ctx, runID, "", taskexec.SetGateStateSignalName, taskrun.SetGateStateRequest{State: state}); err != nil {
		if isTemporalNotFound(err) {
			return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
		}
		return taskrun.TaskRunView{}, fmt.Errorf("signal set-gate for task run %s: %w", runID, err)
	}
	return waitForTaskRunCondition(ctx, 10, 50*time.Millisecond, func() (taskrun.TaskRunView, error) {
		return b.GetTaskRun(ctx, runID)
	}, func(view taskrun.TaskRunView) bool {
		return view.RepoLane.Binding != nil && view.RepoLane.Binding.RunGateState == state
	})
}

// BindLaunchedSession signals the per-run workflow to record the launched agent's
// discovered session/transcript/PID onto its durable binding (Landing 2) and waits until
// the query reflects it. ErrRunNotFound when the run is gone.
func (b *Backend) BindLaunchedSession(ctx context.Context, runID string, sessionID string, transcriptPath string, pid int) (taskrun.TaskRunView, error) {
	if err := b.client.SignalWorkflow(ctx, runID, "", taskexec.BindSessionSignalName, taskrun.BindSessionRequest{
		AgentSessionID:        sessionID,
		SessionTranscriptPath: transcriptPath,
		LaunchedPID:           pid,
	}); err != nil {
		if isTemporalNotFound(err) {
			return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
		}
		return taskrun.TaskRunView{}, fmt.Errorf("signal bind-session for task run %s: %w", runID, err)
	}
	return waitForTaskRunCondition(ctx, 10, 50*time.Millisecond, func() (taskrun.TaskRunView, error) {
		return b.GetTaskRun(ctx, runID)
	}, func(view taskrun.TaskRunView) bool {
		bnd := view.RepoLane.Binding
		return bnd != nil && bnd.AgentSessionID == sessionID && bnd.SessionTranscriptPath == transcriptPath && bnd.LaunchedPID == pid
	})
}

func (b *Backend) ReconcileTaskSnapshot(ctx context.Context, runID string, snapshot taskrun.TaskDefinitionSnapshot) (taskrun.TaskRunView, error) {
	if err := b.client.SignalWorkflow(ctx, runID, "", taskexec.ReconcileSnapshotSignalName, snapshot); err != nil {
		if isTemporalNotFound(err) {
			return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
		}
		return taskrun.TaskRunView{}, fmt.Errorf("signal reconcile for task run %s: %w", runID, err)
	}
	return waitForTaskRunCondition(ctx, 10, 50*time.Millisecond, func() (taskrun.TaskRunView, error) {
		return b.GetTaskRun(ctx, runID)
	}, func(view taskrun.TaskRunView) bool {
		return view.CapturedTaskSnapshot.DeclaredTaskRevision == snapshot.DeclaredTaskRevision &&
			view.DocRuntimeDivergenceStatus == "reconciled"
	})
}

func (b *Backend) UpdateTaskRun(ctx context.Context, runID string, update taskrun.TaskRunUpdate) (taskrun.TaskRunView, error) {
	current, err := b.GetTaskRun(ctx, runID)
	if err != nil {
		return taskrun.TaskRunView{}, err
	}

	if err := b.client.SignalWorkflow(ctx, runID, "", taskexec.UpdateRunSignalName, update); err != nil {
		if isTemporalNotFound(err) {
			return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
		}
		return taskrun.TaskRunView{}, fmt.Errorf("signal update for task run %s: %w", runID, err)
	}
	return readUpdatedTaskRun(func() (taskrun.TaskRunView, error) {
		return b.GetTaskRun(ctx, runID)
	}, func() (taskrun.TaskRunView, error) {
		if current.TemporalExecutionRunID == "" {
			return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
		}
		return b.getClosedTaskRunResult(ctx, runID, current.TemporalExecutionRunID)
	})
}

func (b *Backend) RetryTaskRunWorkload(ctx context.Context, runID string, request taskrun.WorkloadRetryRequest) (taskrun.TaskRunView, error) {
	current, err := b.GetTaskRun(ctx, runID)
	if err != nil {
		return taskrun.TaskRunView{}, err
	}

	if err := b.client.SignalWorkflow(ctx, runID, "", taskexec.RetryWorkloadSignalName, request); err != nil {
		if isTemporalNotFound(err) {
			return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
		}
		return taskrun.TaskRunView{}, fmt.Errorf("signal workload retry for task run %s: %w", runID, err)
	}
	return readUpdatedTaskRun(func() (taskrun.TaskRunView, error) {
		return b.GetTaskRun(ctx, runID)
	}, func() (taskrun.TaskRunView, error) {
		if current.TemporalExecutionRunID == "" {
			return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
		}
		return b.getClosedTaskRunResult(ctx, runID, current.TemporalExecutionRunID)
	})
}

func (b *Backend) getClosedTaskRunResult(ctx context.Context, workflowID string, executionRunID string) (taskrun.TaskRunView, error) {
	workflowRun := b.client.GetWorkflow(ctx, workflowID, executionRunID)

	var view taskrun.TaskRunView
	if err := workflowRun.Get(ctx, &view); err != nil {
		if isTemporalNotFound(err) {
			return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
		}
		return taskrun.TaskRunView{}, fmt.Errorf("get closed task run %s/%s result: %w", workflowID, executionRunID, err)
	}
	return view, nil
}

func readUpdatedTaskRun(
	queryCurrent func() (taskrun.TaskRunView, error),
	queryClosed func() (taskrun.TaskRunView, error),
) (taskrun.TaskRunView, error) {
	view, err := queryCurrent()
	if err == nil {
		return view, nil
	}
	if !errors.Is(err, taskrun.ErrRunNotFound) {
		return taskrun.TaskRunView{}, err
	}
	return queryClosed()
}

func waitForTaskRunCondition(
	ctx context.Context,
	maxAttempts int,
	delay time.Duration,
	query func() (taskrun.TaskRunView, error),
	ready func(taskrun.TaskRunView) bool,
) (taskrun.TaskRunView, error) {
	var last taskrun.TaskRunView
	for attempt := 0; attempt < maxAttempts; attempt++ {
		view, err := query()
		if err != nil {
			return taskrun.TaskRunView{}, err
		}
		last = view
		if ready(view) {
			return view, nil
		}
		if attempt == maxAttempts-1 {
			break
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return last, ctx.Err()
		case <-timer.C:
		}
	}
	return last, nil
}

func buildScheduleOptions(desired controlplane.DesiredSchedule) client.ScheduleOptions {
	return client.ScheduleOptions{
		ID:            desired.ScheduleID,
		Spec:          client.ScheduleSpec{CronExpressions: []string{desired.Cron}, TimeZoneName: desired.Timezone},
		Action:        buildAction(desired),
		Overlap:       enumspb.SCHEDULE_OVERLAP_POLICY_SKIP,
		CatchupWindow: desired.CatchupWindow,
		Note:          fmt.Sprintf("Managed by %s", managedBy),
		Paused:        false,
	}
}

func buildSchedule(desired controlplane.DesiredSchedule) client.Schedule {
	return client.Schedule{
		Spec:   &client.ScheduleSpec{CronExpressions: []string{desired.Cron}, TimeZoneName: desired.Timezone},
		Action: buildAction(desired),
		Policy: &client.SchedulePolicies{
			Overlap:       enumspb.SCHEDULE_OVERLAP_POLICY_SKIP,
			CatchupWindow: desired.CatchupWindow,
		},
		State: &client.ScheduleState{Paused: false, Note: fmt.Sprintf("Managed by %s", managedBy)},
	}
}

func buildAction(desired controlplane.DesiredSchedule) *client.ScheduleWorkflowAction {
	return &client.ScheduleWorkflowAction{
		ID:        desired.WorkflowID,
		Workflow:  desired.WorkflowType,
		Args:      []interface{}{scheduleRunRequest(desired)},
		TaskQueue: desired.TaskQueue,
		Memo: map[string]interface{}{
			"managed_by":        managedBy,
			"job_id":            desired.JobID,
			"trigger_index":     strconv.Itoa(desired.TriggerIndex),
			"desired_cron":      desired.Cron,
			"desired_timezone":  desired.Timezone,
			"desired_spec_hash": desired.SpecHash,
			"workflow_type":     desired.WorkflowType,
			"task_queue":        desired.TaskQueue,
		},
	}
}

func scheduleFromDescription(scheduleID string, desc *client.ScheduleDescription) (controlplane.RuntimeSchedule, error) {
	runtime := controlplane.RuntimeSchedule{
		ScheduleID: scheduleID,
	}

	if parsedJobID, parsedIndex, ok := controlplane.ParseManagedScheduleID(scheduleID); ok {
		runtime.JobID = parsedJobID
		runtime.TriggerIndex = parsedIndex
	}

	if desc.Schedule.Spec != nil {
		runtime.CronExpressions = append([]string(nil), desc.Schedule.Spec.CronExpressions...)
		runtime.TimeZoneName = desc.Schedule.Spec.TimeZoneName
	}
	if desc.Schedule.State != nil {
		runtime.Note = desc.Schedule.State.Note
		runtime.Paused = desc.Schedule.State.Paused
	}
	if desc.Schedule.Policy != nil {
		runtime.CatchupWindow = desc.Schedule.Policy.CatchupWindow
	}
	runtime.NextActionTimes = append([]time.Time(nil), desc.Info.NextActionTimes...)
	runtime.RecentRuns = recentRuns(scheduleID, desc.Info.RecentActions)

	action, ok := desc.Schedule.Action.(*client.ScheduleWorkflowAction)
	if !ok {
		return controlplane.RuntimeSchedule{}, fmt.Errorf("schedule %s uses an unsupported action type", scheduleID)
	}

	if workflowName, ok := action.Workflow.(string); ok {
		runtime.WorkflowType = workflowName
	}
	runtime.TaskQueue = action.TaskQueue

	runtime.ManagedCron = decodeMemoString(action.Memo, "desired_cron")
	runtime.ManagedTimezone = decodeMemoString(action.Memo, "desired_timezone")
	runtime.ManagedSpecHash = decodeMemoString(action.Memo, "desired_spec_hash")

	if managedJobID := decodeMemoString(action.Memo, "job_id"); managedJobID != "" {
		runtime.JobID = managedJobID
	}
	if rawTriggerIndex := decodeMemoString(action.Memo, "trigger_index"); rawTriggerIndex != "" {
		if parsedIndex, err := strconv.Atoi(rawTriggerIndex); err == nil {
			runtime.TriggerIndex = parsedIndex
		}
	}
	if workflowType := decodeMemoString(action.Memo, "workflow_type"); workflowType != "" {
		runtime.WorkflowType = workflowType
	}
	if taskQueue := decodeMemoString(action.Memo, "task_queue"); taskQueue != "" {
		runtime.TaskQueue = taskQueue
	}

	return runtime, nil
}

func recentRuns(scheduleID string, actions []client.ScheduleActionResult) []controlplane.RunRecord {
	runs := make([]controlplane.RunRecord, 0, len(actions))
	for _, action := range actions {
		run := controlplane.RunRecord{
			ScheduleID:   scheduleID,
			ScheduleTime: action.ScheduleTime,
			ActualTime:   action.ActualTime,
		}
		if action.StartWorkflowResult != nil {
			run.WorkflowID = action.StartWorkflowResult.WorkflowID
			run.FirstExecutionRunID = action.StartWorkflowResult.FirstExecutionRunID
		}
		runs = append(runs, run)
	}
	return runs
}

func decodeMemoString(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	raw, exists := values[key]
	if !exists {
		return ""
	}
	payload, ok := raw.(*commonpb.Payload)
	if !ok {
		if stringValue, ok := raw.(string); ok {
			return stringValue
		}
		return ""
	}
	var decoded string
	if err := converter.GetDefaultDataConverter().FromPayload(payload, &decoded); err != nil {
		return ""
	}
	return decoded
}

func scheduleRunRequest(desired controlplane.DesiredSchedule) controlplane.JobRunRequest {
	return controlplane.JobRunRequest{
		JobID:           desired.JobID,
		TriggerType:     "schedule",
		TriggerIndex:    desired.TriggerIndex,
		DesiredSpecHash: desired.SpecHash,
		RequestedAt:     time.Now().UTC(),
		Spec:            desired.Spec,
	}
}

func isTemporalNotFound(err error) bool {
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		return true
	}
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found")
}
