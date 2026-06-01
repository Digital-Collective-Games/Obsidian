package temporalbackend

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/config"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/queue"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/taskrun"
)

// taskrunDispatcher adapts the existing taskrun.Service to the queue.Dispatcher
// seam the O3 consumer drives. It REUSES the existing dispatch / park / reclaim
// paths (Dispatch, SetRunGateState, ReclaimOwnedLane) — it does not reimplement
// dispatch or slot logic. A park/reclaim for a task that has no active owned lane
// is tolerated as a no-op (ErrNoActiveOwnedLane), so a poll that observes a
// Human Needed=Yes or closed issue with no live worktree does not wedge the loop.
//
// O5 wiring: when launch.Enabled is true, after Service.Dispatch provisions an owned
// worktree (RepoLane.OwnedRepoRoot non-empty) the dispatcher (a) LAUNCHES a top-level
// claude agent in that worktree via the launcher seam, (b) BINDS the launched
// session onto the O6 owned-lane record (BindLaunchedSession), and (c) STARTS the
// external liveness watchdog supervisor for the run. When launch.Enabled is false the
// legacy dispatch-only path runs unchanged. The launcher + supervisor are injected
// seams, so unit tests use fakes (no real claude process, no real email).
type taskrunDispatcher struct {
	service dispatchBinder
	repo    string
	// repoNamespace is the registry repo id; it namespaces the active run id the SAME
	// way the per-repo Service does, so the watchdog Stop targets exactly the id the
	// launch started under (BUG-0003: empty = legacy/global id, byte-identical).
	repoNamespace string
	launch        launchConfig
	launcher      agentLauncher
	supervisor    watchdogSupervisor
}

// runID builds the active run id under this dispatcher's repo namespace — the same
// construction the per-repo Service uses for dispatch — so supervisor Start (via the
// launched view.RunID) and Stop resolve to one id and no watchdog goroutine leaks.
func (d taskrunDispatcher) runID(taskID string) string {
	return taskrun.ActiveRunIDForRepo(d.repoNamespace, taskID)
}

func (d taskrunDispatcher) Dispatch(ctx context.Context, taskID string) error {
	view, err := d.service.Dispatch(ctx, taskID)
	if err != nil {
		return err
	}
	// Legacy / launch-disabled path: dispatch only. Also guard the case where no
	// owned worktree was provisioned (nothing to launch into).
	if !d.launch.Enabled || d.launcher == nil || view.RepoLane.OwnedRepoRoot == "" {
		return nil
	}
	return d.launchAgent(ctx, taskID, view)
}

// launchAgent launches a top-level claude agent in the freshly provisioned worktree,
// binds its session onto the O6 record, and starts the watchdog supervisor for the
// run (O5). A launch/bind failure is returned so the consumer surfaces it; the
// supervisor start is best-effort (a nil supervisor simply skips supervision).
func (d taskrunDispatcher) launchAgent(ctx context.Context, taskID string, view taskrun.TaskRunView) error {
	worktree := view.RepoLane.OwnedRepoRoot
	prompt, err := queue.BuildTaskAgentPrompt(d.launch.PromptTemplate, taskID, worktree)
	if err != nil {
		return fmt.Errorf("build launch prompt for %s: %w", taskID, err)
	}
	res, err := d.launcher.Launch(ctx, queue.LaunchSpec{
		WorktreePath:   worktree,
		Prompt:         prompt,
		AllowedTools:   d.launch.AllowedTools,
		PermissionMode: d.launch.PermissionMode,
	})
	if err != nil {
		return fmt.Errorf("launch agent for %s: %w", taskID, err)
	}
	if _, err := d.service.BindLaunchedSession(taskID, res.SessionID, res.TranscriptPath, res.PID); err != nil {
		return fmt.Errorf("bind launched session for %s: %w", taskID, err)
	}
	if d.supervisor != nil {
		d.supervisor.Start(supervisedRun{
			RunID:          view.RunID,
			TaskID:         taskID,
			Repo:           d.repo,
			WorktreePath:   worktree,
			TranscriptPath: res.TranscriptPath,
			gateStateFn:    gateStateForTaskFn(d.service, taskID),
		})
	}
	return nil
}

func (d taskrunDispatcher) SetRunGateState(taskID string, state string) error {
	_, err := d.service.SetRunGateState(taskID, state)
	if errors.Is(err, taskrun.ErrNoActiveOwnedLane) {
		return nil
	}
	return err
}

func (d taskrunDispatcher) Reclaim(_ context.Context, taskID string) error {
	err := d.service.ReclaimOwnedLane(taskID)
	if errors.Is(err, taskrun.ErrNoActiveOwnedLane) {
		return nil
	}
	// On terminal close the run is no longer supervised: stop its watchdog so a
	// reclaimed slot does not keep a goroutine polling a removed transcript.
	if err == nil && d.supervisor != nil {
		d.supervisor.Stop(d.runID(taskID))
	}
	return err
}

func (d taskrunDispatcher) ActiveOwnedLaneTasks() ([]string, error) {
	return d.service.ActiveOwnedLaneTasks()
}

// ClosureRequested delegates to the taskrun service's read of the task's owned
// worktree TASK-STATE.json current_gate (the TEST-ONLY auto-close closure signal).
func (d taskrunDispatcher) ClosureRequested(taskID string) (bool, error) {
	return d.service.ClosureRequested(taskID)
}

// reconstructSupervision re-establishes watchdog supervision for a repo's already-active
// owned lanes. It runs once per backend lifetime per repo (on the first poll that builds
// the repo's cached supervisor), so a backend restart that loses the in-memory supervisor
// does not silently leave an in-flight run unwatched. Each active lane with a bound
// transcript is re-supervised; a lane without one (never launched/bound) has nothing to
// watch and is skipped. Best-effort: an enumeration failure leaves the repo unsupervised
// this cycle rather than aborting the dispatch build.
func reconstructSupervision(sup watchdogSupervisor, service *taskrun.Service, providerRepo string) {
	bindings, err := service.ListActiveWorktreesForRepo()
	if err != nil {
		return
	}
	for _, b := range bindings {
		if b.SessionTranscriptPath == "" {
			continue
		}
		sup.Start(supervisedRun{
			RunID:          b.RunID,
			TaskID:         b.TaskID,
			Repo:           providerRepo,
			WorktreePath:   b.WorktreePath,
			TranscriptPath: b.SessionTranscriptPath,
			gateStateFn:    gateStateForTaskFn(service, b.TaskID),
		})
	}
}

// newQueueDrainActivities builds the REGISTRY-DRIVEN consumer for the worker's poll
// activity. The consumer reads the central registry (cfg.RegistryPath, the explicit
// OBSIDIAN_REGISTRY_PATH) each poll and iterates ALL registered repos: for each
// repo it builds a gh provider polling that entry's task_provider.repo and a
// taskrun.Service bound to that entry's local_root (per-repo slot cap = the entry's
// queue_workers). The single CODEX_ORCHESTRATION_QUEUE_DRAIN_REPO is NO LONGER the
// provider source — global awareness comes from iterating the registry.
//
// An empty registry path yields nil activities so the workflow can still be
// registered while the consumer stays dormant.
//
// O5: when cfg.LaunchQueueAgent is enabled each per-repo dispatcher is wired with a
// live claude launcher + a goroutine watchdog supervisor (default capture incident
// sink — NEVER a real email), so a queue dispatch launches a top-level agent in its
// worktree, binds the session, and supervises it. When disabled the dispatcher uses
// the legacy dispatch-only path.
func newQueueDrainActivities(cfg config.Config, runtime taskrun.Runtime) (*queue.QueueDrainActivities, error) {
	if cfg.RegistryPath == "" || runtime == nil {
		return nil, nil
	}
	registryPath := cfg.RegistryPath
	loadRegistry := func() (queue.RepoManifest, error) {
		return queue.LoadRegistry(registryPath)
	}
	// supervisors caches one durable watchdog supervisor per repo id ACROSS polls. The
	// dispatch binding (provider + Service) is rebuilt each poll so a registry edit is
	// picked up, but the supervisor must persist: a run's Start (the dispatching poll) and
	// Stop (a later poll's Reclaim) have to hit the SAME supervisor, or each poll's fresh
	// supervisor leaks the prior poll's watchdog goroutine and Stop becomes a no-op.
	supervisors := map[string]*goroutineSupervisor{}
	var supervisorsMu sync.Mutex
	dispatchFor := func(repo queue.RegistryRepo) (queue.RepoDispatch, error) {
		provider, err := queue.NewGitHubQueueProvider(repo.ProviderRepo, 0)
		if err != nil {
			return queue.RepoDispatch{}, fmt.Errorf("build queue provider: %w", err)
		}
		// One Service per registry local_root: its idle pool worktree count
		// is naturally per-repo and is the dispatch admission budget (Task-0016 removed
		// the queue_workers cap; concurrency is bounded by the idle pool count).
		service := taskrun.NewServiceForRepo(repo.LocalRoot, cfg.RunsRoot, runtime)
		// CUTOVER (BUG-0003): namespace this repo's run ids + artifact paths by the
		// registry repo id so two repos' identical issue #N can never collide on the
		// Temporal workflow id or the runs-root path.
		service.SetRepoNamespace(repo.ID)
		dispatcher := taskrunDispatcher{service: service, repo: repo.ProviderRepo, repoNamespace: repo.ID}
		if cfg.LaunchQueueAgent {
			dispatcher.launch = launchConfig{
				Enabled:        true,
				AllowedTools:   cfg.QueueAgentAllowedTools,
				PermissionMode: cfg.QueueAgentPermissionMode,
			}
			dispatcher.launcher = liveLauncher{launcher: queue.NewLauncher(nil)}
			// Get-or-create the repo's durable supervisor. On FIRST build (e.g. after a
			// backend restart, when the in-memory map is empty but worktrees + their runs
			// are still live) reconstruct supervision for already-active lanes so an
			// in-flight run is not left unwatched.
			supervisorsMu.Lock()
			sup, ok := supervisors[repo.ID]
			if !ok {
				// The watchdog's incident sink defaults to a capture sink, so a confirmed
				// stall is recorded but NO real email is ever sent from this default wiring.
				watchdog := queue.NewWatchdog(queue.WatchdogConfig{}, time.Now, queue.OSStatTranscript, nil, nil, &queue.CaptureSink{})
				watchdog.SetTailReader(queue.OSTailTranscript)
				sup = newGoroutineSupervisor(watchdog, 0)
				supervisors[repo.ID] = sup
				// Startup reconciliation (once per repo): discover-on-startup (Task-0016)
				// reconstructs the worktree pool's idle-vs-allocated state from disk + the
				// live per-run workflow and subsumes the prune-only hygiene: it prunes stale
				// git worktree metadata BEFORE reconstructing supervision, so a crashed/partial removal
				// does not linger. Best-effort hygiene — a prune failure must not abort the
				// dispatch build (it never reclaims a still-live worktree; see DiscoverPool / ReconcileOwnedLanes).
				_ = service.DiscoverPool()
				reconstructSupervision(sup, service, repo.ProviderRepo)
			}
			supervisorsMu.Unlock()
			dispatcher.supervisor = sup
		}
		return queue.RepoDispatch{Provider: provider, Dispatcher: dispatcher, Sizer: service}, nil
	}
	consumer := queue.NewRegistryConsumer(loadRegistry, dispatchFor)
	// TEST-ONLY: when OBSIDIAN_AUTO_CLOSE_QUEUED is set, each per-repo consumer closes
	// the issue of a task that announced completion (simulated human closure). Default
	// false keeps the consumer read-only against GitHub.
	consumer.SetAutoCloseEnabled(cfg.AutoCloseQueued)
	return queue.NewQueueDrainActivities(consumer), nil
}

// StartQueueDrainConsumer starts the singleton queue-drain consumer workflow
// (A3.4). A duplicate start (the consumer is already running) is reported as a
// non-fatal AlreadyStarted so the start/stop endpoint is idempotent.
func (b *Backend) StartQueueDrainConsumer(ctx context.Context, config queue.QueueDrainConfig) (string, error) {
	run, err := b.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                    queue.QueueDrainWorkflowID,
		TaskQueue:             b.cfg.TaskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	}, queue.QueueDrainWorkflowName, config)
	if err != nil {
		return "", fmt.Errorf("start queue-drain consumer: %w", err)
	}
	return run.GetID(), nil
}

// StopQueueDrainConsumer signals the running consumer to stop draining and exit.
// A not-found consumer (already stopped) is tolerated so stop is idempotent.
func (b *Backend) StopQueueDrainConsumer(ctx context.Context) error {
	if err := b.client.SignalWorkflow(ctx, queue.QueueDrainWorkflowID, "", queue.QueueDrainStopSignalName, nil); err != nil {
		if isTemporalNotFound(err) {
			return nil
		}
		return fmt.Errorf("stop queue-drain consumer: %w", err)
	}
	return nil
}
