package temporalbackend

import (
	"context"
	"sync"
	"time"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/queue"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/taskrun"
)

// O5 wiring seams. The queue-drain consumer dispatches a task into an owned
// worktree, and (when launch is enabled) launches a TOP-LEVEL claude agent in that
// worktree, binds its session onto the O6 owned-lane record, and starts an external
// liveness watchdog supervisor for the run. The launcher and supervisor are behind
// interfaces so unit tests inject FAKES — no real claude process and no real email
// run in tests or any default code path.

// dispatchBinder is the slice of taskrun.Service the queue dispatcher drives:
// dispatch (provisions the worktree), bind the launched session (O6), record the
// parked gate state / reclaim on close (O4), and enumerate active lanes (slot
// accounting + the watchdog's live gate-state read). *taskrun.Service satisfies it;
// unit tests inject a fake so the launch/bind/watchdog wiring is provable WITHOUT a
// real git worktree, claude process, or Temporal runtime.
type dispatchBinder interface {
	Dispatch(ctx context.Context, taskID string) (taskrun.TaskRunView, error)
	BindLaunchedSession(taskID, sessionID, transcriptPath string, pid int) (taskrun.WorktreeBinding, error)
	SetRunGateState(taskID, state string) (taskrun.WorktreeBinding, error)
	ReclaimOwnedLane(taskID string) error
	ActiveOwnedLaneTasks() ([]string, error)
	ListActiveWorktrees() ([]taskrun.WorktreeBinding, error)
	ClosureRequested(taskID string) (bool, error)
}

// agentLauncher launches a top-level claude agent in an owned worktree. Production
// wires it to queue.Launcher.Start; tests inject a fake that returns a session id +
// transcript path without launching a process.
type agentLauncher interface {
	// Launch starts the agent and returns the bound session id + transcript path the
	// O6 binding records and the O5 watchdog stats.
	Launch(ctx context.Context, spec queue.LaunchSpec) (queue.LaunchResult, error)
}

// watchdogSupervisor starts/stops the external liveness watchdog for a launched run.
// Production wires it to a goroutine that polls the bound transcript through a
// queue.Watchdog; tests inject a recorder. A run is supervised from launch until it
// reaches a terminal/close (Stop), so a silently-stalled agent does not hold a slot
// forever (O5).
type watchdogSupervisor interface {
	// Start begins supervising the launched run. It must be non-blocking.
	Start(run supervisedRun)
	// Stop stops supervising the run (called on terminal/close).
	Stop(runID string)
}

// supervisedRun is the per-run input a supervisor needs to observe a launched agent:
// the run/task identity, the owned worktree, and the bound transcript the watchdog
// stats. The run/gate state is read live from the binding each poll (so the watchdog
// suspends while parked), via gateStateFn.
type supervisedRun struct {
	RunID          string
	TaskID         string
	Repo           string
	WorktreePath   string
	TranscriptPath string
	// gateStateFn returns the run's current run/gate state (one of the taskrun
	// RunGateState* values). The supervisor reads it each poll so a parked run
	// suspends the watchdog (A5.5). A nil fn is treated as "running".
	gateStateFn func() string
}

// liveLauncher adapts queue.Launcher to the agentLauncher seam.
type liveLauncher struct {
	launcher *queue.Launcher
}

func (l liveLauncher) Launch(ctx context.Context, spec queue.LaunchSpec) (queue.LaunchResult, error) {
	// wait=false: the queue agent runs unattended and is supervised externally by the
	// watchdog; the launcher returns once the process has started.
	//
	// BUG-0001: the launched process MUST outlive the dispatch activity. The consumer
	// dispatches inside the Temporal queue.drain.poll activity, so passing that ctx to
	// exec.CommandContext would kill the agent the instant the poll activity returns
	// (binding recorded, but the process dies before it appends a transcript). The
	// agent's lifecycle is owned by the watchdog + worktree reclaim, not the activity,
	// so launch under a detached context. (The activity ctx is intentionally ignored.)
	_ = ctx
	return l.launcher.Start(context.Background(), spec, false)
}

// goroutineSupervisor is the production watchdogSupervisor: it runs one goroutine per
// launched run that calls queue.Watchdog.Observe on a ticker, feeding the run's live
// run/gate state so the watchdog suspends while parked. The watchdog itself owns the
// transcript stat, poke, and incident-sink seams (all injectable; the sink defaults
// to a capture/no-op so NO real email is ever sent here).
type goroutineSupervisor struct {
	watchdog *queue.Watchdog
	interval time.Duration

	mu    sync.Mutex
	stops map[string]chan struct{}
}

// newGoroutineSupervisor builds a supervisor over a watchdog. A non-positive interval
// uses queue.DefaultWatchdogPoll.
func newGoroutineSupervisor(watchdog *queue.Watchdog, interval time.Duration) *goroutineSupervisor {
	if interval <= 0 {
		interval = queue.DefaultWatchdogPoll
	}
	return &goroutineSupervisor{watchdog: watchdog, interval: interval, stops: map[string]chan struct{}{}}
}

func (s *goroutineSupervisor) Start(run supervisedRun) {
	if s.watchdog == nil {
		return
	}
	stop := make(chan struct{})
	s.mu.Lock()
	if existing, ok := s.stops[run.RunID]; ok {
		close(existing) // replace an older supervisor for the same run id
	}
	s.stops[run.RunID] = stop
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				gate := ""
				if run.gateStateFn != nil {
					gate = run.gateStateFn()
				}
				// Observe errors (e.g. a transient stat failure) are non-fatal to the
				// loop; the next tick retries. The watchdog records its own state.
				_, _ = s.watchdog.Observe(queue.WatchedRun{
					RunID:          run.RunID,
					TaskID:         run.TaskID,
					Repo:           run.Repo,
					WorktreePath:   run.WorktreePath,
					TranscriptPath: run.TranscriptPath,
					RunGateState:   gate,
				})
			}
		}
	}()
}

func (s *goroutineSupervisor) Stop(runID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if stop, ok := s.stops[runID]; ok {
		close(stop)
		delete(s.stops, runID)
	}
	if s.watchdog != nil {
		s.watchdog.Forget(runID)
	}
}

// launchConfig holds the O5 launch wiring for the dispatcher: the enable toggle and
// the configurable prompt/tools/permission-mode (safe defaults applied at build
// time). When Enabled is false the dispatcher uses the legacy dispatch-only path.
type launchConfig struct {
	Enabled        bool
	PromptTemplate string
	AllowedTools   string
	PermissionMode string
}

// gateStateForTaskFn returns a function that reads a task's current run/gate state
// from its active owned-lane binding (so the watchdog supervisor suspends while the
// run is parked). A task with no active lane (e.g. reclaimed) is reported as running
// so the absence of a binding never masquerades as a park.
func gateStateForTaskFn(service dispatchBinder, taskID string) func() string {
	return func() string {
		bindings, err := service.ListActiveWorktrees()
		if err != nil {
			return taskrun.RunGateStateRunning
		}
		for _, b := range bindings {
			if b.TaskID == taskID {
				if b.RunGateState != "" {
					return b.RunGateState
				}
				return taskrun.RunGateStateRunning
			}
		}
		return taskrun.RunGateStateRunning
	}
}
