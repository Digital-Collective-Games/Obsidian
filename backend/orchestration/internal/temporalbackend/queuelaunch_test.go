package temporalbackend

import (
	"context"
	"strings"
	"testing"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/queue"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/taskrun"
)

// fakeBinder is a deterministic in-memory dispatchBinder. It records every call so
// the O5 wiring tests can assert that Dispatch dispatched via the service, then bound
// the launched session — WITHOUT a real git worktree, claude process, or Temporal.
type fakeBinder struct {
	dispatched   []string
	bound        []boundSession
	ownedRoot    string // the worktree Dispatch reports provisioned ("" => none)
	gateState    string // what ListActiveWorktrees reports for the dispatched task
	dispatchTask string
}

type boundSession struct {
	taskID, sessionID, transcriptPath string
	pid                               int
}

func (f *fakeBinder) Dispatch(_ context.Context, taskID string) (taskrun.TaskRunView, error) {
	f.dispatched = append(f.dispatched, taskID)
	f.dispatchTask = taskID
	return taskrun.TaskRunView{
		RunID:    taskrun.ActiveRunID(taskID),
		TaskID:   taskID,
		RepoLane: taskrun.RepoLane{OwnedRepoRoot: f.ownedRoot},
	}, nil
}

func (f *fakeBinder) BindLaunchedSession(taskID, sessionID, transcriptPath string, pid int) (taskrun.WorktreeBinding, error) {
	f.bound = append(f.bound, boundSession{taskID, sessionID, transcriptPath, pid})
	return taskrun.WorktreeBinding{}, nil
}

func (f *fakeBinder) SetRunGateState(string, string) (taskrun.WorktreeBinding, error) {
	return taskrun.WorktreeBinding{}, nil
}

func (f *fakeBinder) ReclaimOwnedLane(string) error { return nil }

func (f *fakeBinder) ActiveOwnedLaneTasks() ([]string, error) { return nil, nil }

func (f *fakeBinder) ListActiveWorktrees() ([]taskrun.WorktreeBinding, error) {
	if f.dispatchTask == "" {
		return nil, nil
	}
	return []taskrun.WorktreeBinding{{
		RepoBinding: taskrun.RepoBinding{TaskID: f.dispatchTask, RunGateState: f.gateState},
	}}, nil
}

func (f *fakeBinder) ClosureRequested(string) (bool, error) { return false, nil }

// fakeLauncher records the launch spec and returns a fixed session id + transcript
// path, so the wiring test proves the launcher seam was invoked WITHOUT launching a
// real claude process.
type fakeLauncher struct {
	calls []queue.LaunchSpec
	res   queue.LaunchResult
}

func (l *fakeLauncher) Launch(_ context.Context, spec queue.LaunchSpec) (queue.LaunchResult, error) {
	l.calls = append(l.calls, spec)
	return l.res, nil
}

// fakeSupervisor records which runs were supervised (started/stopped), so the wiring
// test proves the watchdog supervisor was started for the launched run.
type fakeSupervisor struct {
	started []supervisedRun
	stopped []string
}

func (s *fakeSupervisor) Start(run supervisedRun) { s.started = append(s.started, run) }
func (s *fakeSupervisor) Stop(runID string)       { s.stopped = append(s.stopped, runID) }

// O5 wiring (launch ENABLED): taskrunDispatcher.Dispatch (a) dispatches via the
// taskrun service seam, (b) invokes the launcher with a built prompt + configured
// tools/permission-mode, (c) binds the launched session via BindLaunchedSession, and
// (d) starts the watchdog supervisor for the run.
func TestDispatchLaunchEnabledLaunchesBindsAndSupervises(t *testing.T) {
	binder := &fakeBinder{ownedRoot: `C:\Agent\QueueDrainTestbed`}
	launcher := &fakeLauncher{res: queue.LaunchResult{
		SessionID:      "11111111-2222-4333-8444-555555555555",
		TranscriptPath: `C:\Users\gregs\.claude\projects\c--Agent-QueueDrainTestbed\sess.jsonl`,
		PID:            4242,
	}}
	supervisor := &fakeSupervisor{}
	d := taskrunDispatcher{
		service: binder,
		repo:    "Digital-Collective-Games/QueueDrainTestbed",
		launch: launchConfig{
			Enabled:        true,
			AllowedTools:   "Read,Edit,Bash,Agent",
			PermissionMode: "bypassPermissions",
		},
		launcher:   launcher,
		supervisor: supervisor,
	}

	if err := d.Dispatch(context.Background(), "Task-7001"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	// (a) dispatched via the service.
	if len(binder.dispatched) != 1 || binder.dispatched[0] != "Task-7001" {
		t.Fatalf("dispatched = %v, want [Task-7001]", binder.dispatched)
	}
	// (b) launcher invoked with the worktree, configured tools/mode, and a prompt
	// naming the task + the done-contract.
	if len(launcher.calls) != 1 {
		t.Fatalf("launcher calls = %d, want 1", len(launcher.calls))
	}
	spec := launcher.calls[0]
	if spec.WorktreePath != `C:\Agent\QueueDrainTestbed` {
		t.Fatalf("launch worktree = %q", spec.WorktreePath)
	}
	if spec.AllowedTools != "Read,Edit,Bash,Agent" || spec.PermissionMode != "bypassPermissions" {
		t.Fatalf("launch tools/mode = %q / %q", spec.AllowedTools, spec.PermissionMode)
	}
	for _, want := range []string{"Task-7001", "NEVER close the GitHub issue", "Human Needed=Yes"} {
		if !strings.Contains(spec.Prompt, want) {
			t.Fatalf("launch prompt missing %q:\n%s", want, spec.Prompt)
		}
	}
	// (c) bound the launched session with the launcher's returned values.
	if len(binder.bound) != 1 {
		t.Fatalf("bound calls = %d, want 1", len(binder.bound))
	}
	b := binder.bound[0]
	if b.taskID != "Task-7001" || b.sessionID != launcher.res.SessionID || b.transcriptPath != launcher.res.TranscriptPath {
		t.Fatalf("bound = %+v, want the launcher's session/transcript for Task-7001", b)
	}
	// BUG-0002: the launched PID is persisted so reclaim can terminate the agent.
	if b.pid != launcher.res.PID {
		t.Fatalf("bound pid = %d, want the launcher's pid %d", b.pid, launcher.res.PID)
	}
	// (d) supervisor started for the run (run id == active run id of the task).
	if len(supervisor.started) != 1 {
		t.Fatalf("supervisor started = %d, want 1", len(supervisor.started))
	}
	run := supervisor.started[0]
	if run.RunID != taskrun.ActiveRunID("Task-7001") {
		t.Fatalf("supervised run id = %q, want %q", run.RunID, taskrun.ActiveRunID("Task-7001"))
	}
	if run.TranscriptPath != launcher.res.TranscriptPath || run.WorktreePath != `C:\Agent\QueueDrainTestbed` {
		t.Fatalf("supervised run = %+v, want bound transcript + worktree", run)
	}
}

// O5 wiring (launch DISABLED): the legacy path dispatches via the service and does
// NOT launch, bind, or supervise — non-queue/legacy dispatch is unaffected.
func TestDispatchLaunchDisabledDispatchesOnly(t *testing.T) {
	binder := &fakeBinder{ownedRoot: `C:\Agent\QueueDrainTestbed`}
	launcher := &fakeLauncher{}
	supervisor := &fakeSupervisor{}
	d := taskrunDispatcher{
		service:    binder,
		launch:     launchConfig{Enabled: false},
		launcher:   launcher,
		supervisor: supervisor,
	}

	if err := d.Dispatch(context.Background(), "Task-7002"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(binder.dispatched) != 1 {
		t.Fatalf("dispatched = %v, want one dispatch (legacy path intact)", binder.dispatched)
	}
	if len(launcher.calls) != 0 {
		t.Fatalf("launcher calls = %d, want 0 (launch disabled)", len(launcher.calls))
	}
	if len(binder.bound) != 0 {
		t.Fatalf("bound calls = %d, want 0 (launch disabled)", len(binder.bound))
	}
	if len(supervisor.started) != 0 {
		t.Fatalf("supervisor started = %d, want 0 (launch disabled)", len(supervisor.started))
	}
}

// With launch enabled but no worktree provisioned (OwnedRepoRoot empty), the
// dispatcher dispatches but does NOT launch — there is no worktree to launch into.
func TestDispatchLaunchEnabledNoWorktreeDoesNotLaunch(t *testing.T) {
	binder := &fakeBinder{ownedRoot: ""}
	launcher := &fakeLauncher{}
	supervisor := &fakeSupervisor{}
	d := taskrunDispatcher{
		service:    binder,
		launch:     launchConfig{Enabled: true},
		launcher:   launcher,
		supervisor: supervisor,
	}

	if err := d.Dispatch(context.Background(), "Task-7003"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(binder.dispatched) != 1 {
		t.Fatalf("dispatched = %v, want one dispatch", binder.dispatched)
	}
	if len(launcher.calls) != 0 || len(binder.bound) != 0 || len(supervisor.started) != 0 {
		t.Fatalf("no worktree => must not launch/bind/supervise: launches=%d bound=%d started=%d",
			len(launcher.calls), len(binder.bound), len(supervisor.started))
	}
}

// On terminal close (Reclaim) the dispatcher stops the run's watchdog supervisor so a
// reclaimed slot does not keep a goroutine polling a removed transcript.
func TestReclaimStopsWatchdogSupervisor(t *testing.T) {
	binder := &fakeBinder{}
	supervisor := &fakeSupervisor{}
	d := taskrunDispatcher{service: binder, supervisor: supervisor}

	if err := d.Reclaim(context.Background(), "Task-7004"); err != nil {
		t.Fatalf("Reclaim: %v", err)
	}
	if len(supervisor.stopped) != 1 || supervisor.stopped[0] != taskrun.ActiveRunID("Task-7004") {
		t.Fatalf("supervisor.stopped = %v, want [%s]", supervisor.stopped, taskrun.ActiveRunID("Task-7004"))
	}
}

// The live launcher liveLauncher and the goroutine supervisor must satisfy the
// dispatcher's seams (compile-time assertion). *taskrun.Service must satisfy
// dispatchBinder so production wires it directly.
var (
	_ agentLauncher      = liveLauncher{}
	_ watchdogSupervisor = (*goroutineSupervisor)(nil)
	_ dispatchBinder     = (*taskrun.Service)(nil)
)
