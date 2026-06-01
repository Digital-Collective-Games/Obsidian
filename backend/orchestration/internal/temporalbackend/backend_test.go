package temporalbackend

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.temporal.io/sdk/temporal"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/controlplane"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/jobs"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/taskrun"
)

func TestBuildScheduleOptionsUseManagedCatchupWindow(t *testing.T) {
	desired := testDesiredSchedule()

	options := buildScheduleOptions(desired)

	if options.CatchupWindow != controlplane.ManagedScheduleCatchupWindow {
		t.Fatalf("catchup window = %v, want %v", options.CatchupWindow, controlplane.ManagedScheduleCatchupWindow)
	}
}

func TestBuildScheduleUseManagedCatchupWindow(t *testing.T) {
	desired := testDesiredSchedule()

	schedule := buildSchedule(desired)

	if schedule.Policy == nil {
		t.Fatal("expected schedule policy")
	}
	if schedule.Policy.CatchupWindow != controlplane.ManagedScheduleCatchupWindow {
		t.Fatalf("catchup window = %v, want %v", schedule.Policy.CatchupWindow, controlplane.ManagedScheduleCatchupWindow)
	}
}

func TestReadUpdatedTaskRunReturnsOpenViewWhenQuerySucceeds(t *testing.T) {
	want := taskrun.TaskRunView{RunID: "taskrun--Task-0008--active", Status: "active"}

	got, err := readUpdatedTaskRun(func() (taskrun.TaskRunView, error) {
		return want, nil
	}, func() (taskrun.TaskRunView, error) {
		t.Fatal("closed fallback should not run when current query succeeds")
		return taskrun.TaskRunView{}, nil
	})
	if err != nil {
		t.Fatalf("readUpdatedTaskRun returned error: %v", err)
	}
	if got.RunID != want.RunID || got.Status != want.Status {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestReadUpdatedTaskRunFallsBackToClosedResultOnNotFound(t *testing.T) {
	want := taskrun.TaskRunView{
		RunID:      "taskrun--Task-0008--active",
		Status:     "interrupted",
		Resolution: &taskrun.RunResolution{Kind: "interrupt_review", Decision: "redispatch_ready"},
	}

	got, err := readUpdatedTaskRun(func() (taskrun.TaskRunView, error) {
		return taskrun.TaskRunView{}, taskrun.ErrRunNotFound
	}, func() (taskrun.TaskRunView, error) {
		return want, nil
	})
	if err != nil {
		t.Fatalf("readUpdatedTaskRun returned error: %v", err)
	}
	if got.RunID != want.RunID || got.Status != want.Status {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if got.Resolution == nil || got.Resolution.Decision != "redispatch_ready" {
		t.Fatalf("resolution = %#v", got.Resolution)
	}
}

func TestReadUpdatedTaskRunPreservesNonNotFoundError(t *testing.T) {
	wantErr := errors.New("query failed")

	_, err := readUpdatedTaskRun(func() (taskrun.TaskRunView, error) {
		return taskrun.TaskRunView{}, wantErr
	}, func() (taskrun.TaskRunView, error) {
		t.Fatal("closed fallback should not run on non-not-found errors")
		return taskrun.TaskRunView{}, nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestIsTemporalNotFoundTreatsTerminatedWorkflowAsGone(t *testing.T) {
	// BUG-0006: after an Eject terminates a run's workflow (BUG-0005), the closed-result read
	// (getClosedTaskRunResult -> workflowRun.Get) returns a *temporal.TerminatedError whose
	// message lacks "not found". It must map to ErrRunNotFound so the run reads as gone — the
	// direct read /api/v1/task-runs/<id> returns 404 (not 502) and readTask tolerates it.
	terminated := error(&temporal.TerminatedError{})
	if !isTemporalNotFound(terminated) {
		t.Fatalf("isTemporalNotFound(TerminatedError) = false, want true (terminated workflow must read as gone)")
	}
	if isTemporalNotFound(errors.New("some other transient failure")) {
		t.Fatal("isTemporalNotFound must not treat an unrelated error as gone")
	}
}

func TestWaitForTaskRunConditionReturnsWhenReady(t *testing.T) {
	attempts := 0
	got, err := waitForTaskRunCondition(context.Background(), 3, time.Nanosecond, func() (taskrun.TaskRunView, error) {
		attempts++
		if attempts == 1 {
			return taskrun.TaskRunView{
				CapturedTaskSnapshot:       taskrun.TaskDefinitionSnapshot{DeclaredTaskRevision: "old"},
				DocRuntimeDivergenceStatus: "in_sync",
			}, nil
		}
		return taskrun.TaskRunView{
			CapturedTaskSnapshot:       taskrun.TaskDefinitionSnapshot{DeclaredTaskRevision: "new"},
			DocRuntimeDivergenceStatus: "reconciled",
		}, nil
	}, func(view taskrun.TaskRunView) bool {
		return view.CapturedTaskSnapshot.DeclaredTaskRevision == "new" && view.DocRuntimeDivergenceStatus == "reconciled"
	})
	if err != nil {
		t.Fatalf("waitForTaskRunCondition returned error: %v", err)
	}
	if got.CapturedTaskSnapshot.DeclaredTaskRevision != "new" {
		t.Fatalf("revision = %q, want new", got.CapturedTaskSnapshot.DeclaredTaskRevision)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestWaitForTaskRunConditionReturnsLastViewWhenNotReady(t *testing.T) {
	got, err := waitForTaskRunCondition(context.Background(), 2, time.Nanosecond, func() (taskrun.TaskRunView, error) {
		return taskrun.TaskRunView{
			CapturedTaskSnapshot:       taskrun.TaskDefinitionSnapshot{DeclaredTaskRevision: "old"},
			DocRuntimeDivergenceStatus: "in_sync",
		}, nil
	}, func(view taskrun.TaskRunView) bool {
		return view.DocRuntimeDivergenceStatus == "reconciled"
	})
	if err != nil {
		t.Fatalf("waitForTaskRunCondition returned error: %v", err)
	}
	if got.DocRuntimeDivergenceStatus != "in_sync" {
		t.Fatalf("status = %q, want in_sync", got.DocRuntimeDivergenceStatus)
	}
}

func testDesiredSchedule() controlplane.DesiredSchedule {
	return controlplane.DesiredSchedule{
		ScheduleID:    "codex-job--codex-daily-agentic-swe-digest--00",
		JobID:         "codex-daily-agentic-swe-digest",
		TriggerIndex:  0,
		Enabled:       true,
		Cron:          "0 4 * * *",
		Timezone:      "America/Toronto",
		CatchupWindow: controlplane.ManagedScheduleCatchupWindow,
		WorkflowType:  "codex.exec.job",
		TaskQueue:     "codex-orchestration",
		WorkflowID:    "codex-daily-agentic-swe-digest/schedule/00",
		SpecHash:      "spec-hash",
		Spec: jobs.Spec{
			APIVersion:   jobs.APIVersion,
			JobID:        "codex-daily-agentic-swe-digest",
			Label:        "Codex Daily Agentic SWE Digest",
			DesiredState: jobs.DesiredStateEnabled,
			Triggers: []jobs.Trigger{
				{Type: jobs.TriggerTypeSchedule, Cron: "0 4 * * *", Timezone: "America/Toronto"},
				{Type: jobs.TriggerTypeManual},
			},
			Executor: jobs.Executor{
				Type:       jobs.ExecutorTypeCodexExec,
				Cwd:        `C:\Users\gregs\.codex`,
				Entrypoint: "agentic-swe-digest",
			},
			Runtime: jobs.RuntimeConfig{
				WorkflowType: "codex.exec.job",
				TaskQueue:    "codex-orchestration",
			},
		},
	}
}
