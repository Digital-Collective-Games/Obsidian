package taskrun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/queue"
)

type Service struct {
	declaredWorktreeRoot string
	trackingRoot         string
	runArtifactsRoot     string
	ownedLaneRoot        string
	// repoNamespace disambiguates the Temporal run identity (workflow id) and the
	// runs-root artifact path across repos in the central registry. Two repos each
	// have an issue #1 -> both map to Task-0001; without a per-repo namespace they
	// collide on the workflow id + runs-root path (BUG-0003). Empty = legacy
	// behavior (single-repo control plane / manual dispatch): runID is byte-identical
	// to the historical ActiveRunID(taskID). Set to the registry repo id by
	// NewServiceForRepo at the cutover.
	repoNamespace string
	runtime       Runtime
	now           func() time.Time
	// idleWorktreeCount reports how many IDLE worktrees this repo's pool currently has
	// — the admission budget that replaced the queue_workers cap (Task-0016).
	// Concurrency is bounded by the idle pool count by construction. It is a field so
	// tests can drive admission deterministically; production wiring counts idle pool
	// members on disk (countIdlePoolWorktrees).
	idleWorktreeCount func() (int, error)
	// dequeueProvider is the task-provider WRITE seam used by DequeueTask / Eject to set
	// an issue's queue state to not-ready (Queue -> Never) THROUGH the provider, never an
	// inline gh call (Task-0016). Nil = dequeue is a safe no-op. Injected by the per-repo
	// queuedrain wiring (symmetric to the read provider) or a fake in tests.
	dequeueProvider DequeueProvider
}

type taskStateFile struct {
	TaskID       string   `json:"task_id"`
	Status       string   `json:"status"`
	Phase        string   `json:"phase"`
	PlanApproved bool     `json:"plan_approved"`
	CurrentPass  string   `json:"current_pass"`
	CurrentGate  string   `json:"current_gate"`
	Blockers     []string `json:"blockers"`
	UpdatedAt    string   `json:"updated_at"`
}

type parsedTask struct {
	state       taskStateFile
	title       string
	meaning     string
	snapshot    TaskDefinitionSnapshot
	evidenceRef []EvidenceRef
	taskRoot    string
}

type ownedLaneBootstrapRecord struct {
	TaskID               string               `json:"task_id"`
	RunID                string               `json:"run_id"`
	OwnedRepoRoot        string               `json:"owned_repo_root"`
	BaselineCommit       string               `json:"baseline_commit"`
	CurrentCommit        string               `json:"current_commit"`
	DeclaredWorktreeRoot string               `json:"declared_worktree_root"`
	DeclaredTaskRoot     string               `json:"declared_task_root"`
	DeclaredTaskRevision string               `json:"declared_task_revision"`
	DeclaredGitRevision  string               `json:"declared_git_revision,omitempty"`
	CapturedAt           time.Time            `json:"captured_at"`
	BootstrappedAt       time.Time            `json:"bootstrapped_at"`
	Files                []TaskArtifactDigest `json:"files,omitempty"`
	// Binding is the O6 worktree<->session binding persisted durably alongside the
	// rest of the bootstrap record so the GET /api/v1/worktrees endpoint can
	// enumerate active worktrees and their bound session/transcript/state.
	Binding *RepoBinding `json:"binding,omitempty"`
}

func NewService(declaredWorktreeRoot string, runsRoot string, runtime Runtime) *Service {
	s := newService(declaredWorktreeRoot, runsRoot, runtime)
	s.idleWorktreeCount = s.countIdlePoolWorktrees
	return s
}

// NewServiceForRepo builds a Service bound to a SPECIFIC repo local_root. It is the
// repo-parameterized constructor the registry-driven queue-drain consumer uses: one
// Service per registered local_root, so its idle pool worktree count is naturally
// per-repo. There is no per-repo numeric cap any more (Task-0016 removed queue_workers);
// concurrency is bounded by the count of idle pool worktrees by construction.
func NewServiceForRepo(localRoot string, runsRoot string, runtime Runtime) *Service {
	s := newService(localRoot, runsRoot, runtime)
	s.idleWorktreeCount = s.countIdlePoolWorktrees
	return s
}

// SetRepoNamespace sets the per-repo discriminator that namespaces this Service's
// Temporal run ids (and runs-root artifact paths) so the same issue number in two
// registry repos does not collide (BUG-0003). It is the single production cutover
// point (called by the registry-driven dispatch wiring with the registry repo id);
// the empty default keeps the single-repo control plane byte-identical to the legacy
// id. Set it before the Service dispatches.
func (s *Service) SetRepoNamespace(repoNamespace string) {
	s.repoNamespace = repoNamespace
}

func newService(declaredWorktreeRoot string, runsRoot string, runtime Runtime) *Service {
	return &Service{
		declaredWorktreeRoot: declaredWorktreeRoot,
		trackingRoot:         filepath.Join(declaredWorktreeRoot, "Tracking"),
		runArtifactsRoot:     filepath.Join(runsRoot, "taskruns"),
		ownedLaneRoot:        defaultOwnedLaneRoot(runsRoot),
		runtime:              runtime,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// countIdlePoolWorktrees counts this repo's IDLE pool worktrees (members with no live
// run bound) — the admission budget the queue-drain consumer and the manual dispatch
// gate draw from (Task-0016). It is the production wiring of the idleWorktreeCount seam.
func (s *Service) countIdlePoolWorktrees() (int, error) {
	pool, err := s.ListPoolWorktrees()
	if err != nil {
		return 0, err
	}
	idle := 0
	for _, wt := range pool {
		if wt.Status == poolStatusIdle {
			idle++
		}
	}
	return idle, nil
}

// IdleWorktreeCount reports how many idle pool worktrees this repo has. It is the
// queue.PoolSizer the consumer uses for admission (an empty idle pool defers a Ready
// issue) and mirrors the seam the manual dispatch gate consults.
func (s *Service) IdleWorktreeCount() (int, error) {
	if s.idleWorktreeCount == nil {
		return s.countIdlePoolWorktrees()
	}
	return s.idleWorktreeCount()
}

// countOwnedLaneWorktrees counts the live owned-lane checkouts for the repo by
// listing the declared worktree's git worktrees and keeping those rooted under
// the backend-owned lane root. Each O2 slot is one such worktree, so this is the
// durable per-repo used-slot count (siblings included, no per-task filtering).
func (s *Service) countOwnedLaneWorktrees() (int, error) {
	argv := []string{}
	if runtime.GOOS == "windows" {
		argv = append(argv, "-c", "core.longpaths=true")
	}
	argv = append(argv, "-C", s.declaredWorktreeRoot, "worktree", "list", "--porcelain")
	out, err := exec.Command("git", argv...).Output()
	if err != nil {
		return 0, fmt.Errorf("list owned-lane worktrees: %w", err)
	}
	return countOwnedLaneWorktreesFromPorcelain(out, s.ownedLaneRoot), nil
}

// countOwnedLaneWorktreesFromPorcelain counts live owned-lane slots from
// `git worktree list --porcelain` output: one per record block (blank-line
// separated, starting with `worktree <path>`) whose path is under ownedLaneRoot
// AND which is NOT prunable. A prunable entry is a stale registration whose working
// dir is gone (e.g. a worktree dir removed out-of-band); counting it wrongly pins a
// slot and blocks dispatch (the Landing-1 regression). Pure for unit-testability.
func countOwnedLaneWorktreesFromPorcelain(out []byte, ownedLaneRoot string) int {
	count := 0
	curPath := ""
	curPrunable := false
	flush := func() {
		if curPath != "" && !curPrunable && pathWithinRoot(curPath, ownedLaneRoot) {
			count++
		}
		curPath = ""
		curPrunable = false
	}
	for _, raw := range strings.Split(string(out), "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if rest, ok := strings.CutPrefix(line, "worktree "); ok {
			flush()
			curPath = strings.TrimSpace(rest)
			continue
		}
		if line == "prunable" || strings.HasPrefix(line, "prunable ") {
			curPrunable = true
		}
	}
	flush()
	return count
}

// WorktreeBinding is one active owned worktree's O6 binding as returned by the
// GET /api/v1/worktrees endpoint. It carries the run id of the bootstrap record
// it was read from alongside the durable RepoBinding. It deliberately exposes
// only the raw fields needed to CONSTRUCT a VSCodium link (worktree path, agent
// session id, transcript path) and never a vscodium:// link itself (O6 boundary).
type WorktreeBinding struct {
	RunID string `json:"run_id,omitempty"`
	RepoBinding
}

// ListActiveWorktrees enumerates the active owned-lane worktrees and returns each
// one's O6 binding. A worktree is "active" when its checkout directory still exists
// on disk (cleanupOwnedLane removes it on terminal close), so a closed/reclaimed lane
// drops out naturally. The set of live worktrees is enumerated from the durable
// owned-lane-bootstrap.json breadcrumbs (deduped by worktree path, most-recent record
// per path); each one's run/gate state + session binding is then read LIVE from the
// per-run TaskRunWorkflow (Landing 2 authority), falling back to the breadcrumb's own
// binding when the workflow is gone.
//
// A parked worktree is still active (its checkout is retained) and is listed unchanged
// with its live parked state, which is what lets the operator reach a parked agent's
// session (A6.4).
func (s *Service) ListActiveWorktrees() ([]WorktreeBinding, error) {
	// GLOBAL view (repoScoped=false): GET /api/v1/worktrees reports active lanes
	// across ALL repos. Per-repo slot/active accounting must NOT use this — it uses
	// the repo-scoped path (ActiveOwnedLaneTasks) so one repo never sees another
	// repo's lane (BUG-0003).
	byWorktree, err := s.collectActiveLaneRecords(false)
	if err != nil {
		return nil, err
	}
	return s.bindingsFromRecords(byWorktree), nil
}

// ListActiveWorktreesForRepo is the REPO-SCOPED counterpart of ListActiveWorktrees: it
// returns only THIS Service's repo's active owned-lane bindings (collectActiveLaneRecords
// repoScoped=true), each read LIVE from its per-run workflow with breadcrumb fallback. The
// queue-drain consumer uses it to RECONSTRUCT watchdog supervision on a backend restart,
// so an in-flight run is not left unwatched after the in-memory supervisor is lost.
func (s *Service) ListActiveWorktreesForRepo() ([]WorktreeBinding, error) {
	byWorktree, err := s.collectActiveLaneRecords(true)
	if err != nil {
		return nil, err
	}
	return s.bindingsFromRecords(byWorktree), nil
}

// bindingsFromRecords projects a worktree-keyed record set into the sorted WorktreeBinding
// slice both ListActiveWorktrees views return, reading each one's LIVE binding from its
// per-run workflow (liveBindingForRecord).
func (s *Service) bindingsFromRecords(byWorktree map[string]ownedLaneBootstrapRecord) []WorktreeBinding {
	bindings := make([]WorktreeBinding, 0, len(byWorktree))
	for _, record := range byWorktree {
		bindings = append(bindings, WorktreeBinding{RunID: record.RunID, RepoBinding: s.liveBindingForRecord(record)})
	}
	sort.Slice(bindings, func(i, j int) bool {
		if bindings[i].TaskID != bindings[j].TaskID {
			return bindings[i].TaskID < bindings[j].TaskID
		}
		return bindings[i].WorktreePath < bindings[j].WorktreePath
	})
	return bindings
}

// collectActiveLaneRecords scans the runs-root for the most-recent owned-lane bootstrap
// record per LIVE worktree (the worktree dir must still exist on disk), keyed by owned
// repo root. When repoScoped is true it keeps ONLY records whose DeclaredWorktreeRoot
// matches this Service's repo, so per-repo accounting never sees another repo's lane
// (BUG-0003 fix A). A record with an empty DeclaredWorktreeRoot is kept either way
// (legacy/backward-compatible: it does not assert a different repo).
func (s *Service) collectActiveLaneRecords(repoScoped bool) (map[string]ownedLaneBootstrapRecord, error) {
	taskEntries, err := os.ReadDir(s.runArtifactsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]ownedLaneBootstrapRecord{}, nil
		}
		return nil, fmt.Errorf("read task-run artifacts root: %w", err)
	}

	byWorktree := map[string]ownedLaneBootstrapRecord{}
	for _, taskEntry := range taskEntries {
		if !taskEntry.IsDir() {
			continue
		}
		taskDir := filepath.Join(s.runArtifactsRoot, taskEntry.Name())
		runEntries, err := os.ReadDir(taskDir)
		if err != nil {
			return nil, fmt.Errorf("read task-run dir %s: %w", taskDir, err)
		}
		for _, runEntry := range runEntries {
			if !runEntry.IsDir() {
				continue
			}
			recordPath := filepath.Join(taskDir, runEntry.Name(), "owned-lane-bootstrap.json")
			raw, err := os.ReadFile(recordPath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("read owned-lane bootstrap %s: %w", recordPath, err)
			}
			var record ownedLaneBootstrapRecord
			if err := json.Unmarshal(raw, &record); err != nil {
				return nil, fmt.Errorf("decode owned-lane bootstrap %s: %w", recordPath, err)
			}
			if record.OwnedRepoRoot == "" {
				continue
			}
			if repoScoped && record.DeclaredWorktreeRoot != "" && !sameRepoRoot(record.DeclaredWorktreeRoot, s.declaredWorktreeRoot) {
				// Another repo's lane: invisible to THIS repo's accounting.
				continue
			}
			if _, err := os.Stat(record.OwnedRepoRoot); err != nil {
				// Worktree directory is gone (reclaimed on terminal close) -> not active.
				continue
			}
			existing, ok := byWorktree[record.OwnedRepoRoot]
			if !ok || record.BootstrappedAt.After(existing.BootstrappedAt) {
				byWorktree[record.OwnedRepoRoot] = record
			}
		}
	}
	return byWorktree, nil
}

// bindingFromRecord returns the binding persisted on the record, reconstructing a
// minimal binding from the record's own fields for legacy records written before
// O6 added the binding (so a pre-O6 worktree still enumerates with its task id and
// worktree path rather than being dropped).
func bindingFromRecord(record ownedLaneBootstrapRecord) RepoBinding {
	if record.Binding != nil {
		return *record.Binding
	}
	return RepoBinding{
		TaskID:       record.TaskID,
		WorktreePath: record.OwnedRepoRoot,
		RunGateState: RunGateStateRunning,
	}
}

// worktreeBindingFromView projects a TaskRunView's live RepoLane binding (the Landing-2
// workflow authority) into the WorktreeBinding shape Set/Bind return. A view with no
// binding yields an empty RepoBinding (the workflow seeds one on the first signal).
func worktreeBindingFromView(view TaskRunView) WorktreeBinding {
	binding := RepoBinding{}
	if view.RepoLane.Binding != nil {
		binding = *view.RepoLane.Binding
	}
	return WorktreeBinding{RunID: view.RunID, RepoBinding: binding}
}

// liveBindingForRecord returns the LIVE run/gate + session binding for an owned-lane
// record. Landing 2 makes the per-run TaskRunWorkflow the durable authority for the
// binding, so this queries it by the record's run id. The breadcrumb is a recovery
// fallback: when the workflow is gone (ErrRunNotFound), unreachable, or carries no
// binding yet, the record's own persisted binding is returned (its gate ossifies at the
// last value; its launched PID is kept faithful by BindLaunchedSession for reclaim).
func (s *Service) liveBindingForRecord(record ownedLaneBootstrapRecord) RepoBinding {
	if s.runtime == nil {
		return bindingFromRecord(record)
	}
	view, err := s.runtime.GetActiveTaskRun(context.Background(), record.RunID)
	if err != nil || view.RepoLane.Binding == nil {
		return bindingFromRecord(record)
	}
	return *view.RepoLane.Binding
}

// ErrNoActiveOwnedLane is returned by SetRunGateState when the task has no active
// owned-lane record (no worktree to record a park/running state on).
var ErrNoActiveOwnedLane = errors.New("no active owned lane for task")

// SetRunGateState records the run/gate state for a task's active owned lane by
// SIGNALING the per-run TaskRunWorkflow, which is the durable, sole live writer of the
// run/gate label (Landing 2: no JSON side-store). It is the clean transition API the O3
// consumer (PASS-0004) calls when it observes a GitHub issue parked Human Needed=Yes
// (one of the parked states) or back to running. A run that is gone (ErrRunNotFound)
// maps to ErrNoActiveOwnedLane so the consumer tolerates it as an already-reclaimed no-op.
//
// SetRunGateState never deallocates: parking RETAINS the worktree and slot (D2).
// Only the human-approved CLOSE path (cleanupOwnedLane) frees a slot.
func (s *Service) SetRunGateState(taskID string, state string) (WorktreeBinding, error) {
	switch state {
	case RunGateStateRunning,
		RunGateStateParkedAwaitingClosure,
		RunGateStateParkedResearch,
		RunGateStateParkedPlan,
		RunGateStateParkedRegression:
	default:
		return WorktreeBinding{}, fmt.Errorf("unknown run/gate state %q", state)
	}
	if s.runtime == nil {
		return WorktreeBinding{}, fmt.Errorf("set run/gate state for %s: task runtime not configured", taskID)
	}
	view, err := s.runtime.SetRunGateState(context.Background(), s.runID(taskID), state)
	if err != nil {
		if errors.Is(err, ErrRunNotFound) {
			return WorktreeBinding{}, fmt.Errorf("%w: %s", ErrNoActiveOwnedLane, taskID)
		}
		return WorktreeBinding{}, fmt.Errorf("signal run/gate state for %s: %w", taskID, err)
	}
	return worktreeBindingFromView(view), nil
}

// BindLaunchedSession records the POST-LAUNCH-discovered agent session id, transcript
// path, and OS pid on the task's active owned-lane binding (O5/O6, coordinator-review
// correction 2). At dispatch the binding's session fields are placeholders sourced from
// the BACKEND process's env (bindingForLane); they cannot name the launched agent's own
// session, which does not exist until after launch. After the launcher discovers the
// agent's session, the consumer calls this to replace those placeholders.
//
// Landing 2: the per-run TaskRunWorkflow is the durable, sole live writer of the binding
// (signaled here). The owned-lane breadcrumb is demoted to a recovery breadcrumb, but it
// RETAINS a faithful launched PID (R2/BUG-0002): reclaim's recovery path terminates the
// agent before removing the worktree, and must find the real PID even if the workflow is
// already gone. It never changes the run/gate state and never deallocates.
func (s *Service) BindLaunchedSession(taskID string, sessionID string, transcriptPath string, pid int) (WorktreeBinding, error) {
	if s.runtime == nil {
		return WorktreeBinding{}, fmt.Errorf("bind launched session for %s: task runtime not configured", taskID)
	}
	view, err := s.runtime.BindLaunchedSession(context.Background(), s.runID(taskID), sessionID, transcriptPath, pid)
	if err != nil {
		if errors.Is(err, ErrRunNotFound) {
			return WorktreeBinding{}, fmt.Errorf("%w: %s", ErrNoActiveOwnedLane, taskID)
		}
		return WorktreeBinding{}, fmt.Errorf("signal launched-session binding for %s: %w", taskID, err)
	}
	// R2 (BUG-0002): keep a faithful launched PID on the recovery breadcrumb so reclaim
	// can terminate-before-remove even when the workflow is gone. Only the PID is written
	// (the live binding lives in the workflow); a task with no breadcrumb is tolerated.
	if activePath, activeRecord, ferr := s.findActiveLaneRecord(taskID); ferr == nil {
		binding := bindingFromRecord(activeRecord)
		binding.LaunchedPID = pid
		activeRecord.Binding = &binding
		if werr := writeJSONFile(activePath, activeRecord); werr != nil {
			return WorktreeBinding{}, fmt.Errorf("persist launched PID on owned-lane breadcrumb: %w", werr)
		}
	} else if !errors.Is(ferr, ErrNoActiveOwnedLane) {
		return WorktreeBinding{}, ferr
	}
	return worktreeBindingFromView(view), nil
}

// findActiveLaneRecord returns the bootstrap record path and decoded record for a
// task's active owned lane (the most recently bootstrapped record whose worktree
// still exists on disk). It returns ErrNoActiveOwnedLane when the task has no
// active lane. Shared by SetRunGateState (O4 park) and ReclaimOwnedLane (O4 close).
func (s *Service) findActiveLaneRecord(taskID string) (string, ownedLaneBootstrapRecord, error) {
	taskDir := filepath.Join(s.runArtifactsRoot, sanitizePathSegment(taskID))
	runEntries, err := os.ReadDir(taskDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ownedLaneBootstrapRecord{}, fmt.Errorf("%w: %s", ErrNoActiveOwnedLane, taskID)
		}
		return "", ownedLaneBootstrapRecord{}, fmt.Errorf("read task-run dir %s: %w", taskDir, err)
	}

	var activePath string
	var activeRecord ownedLaneBootstrapRecord
	for _, runEntry := range runEntries {
		if !runEntry.IsDir() {
			continue
		}
		recordPath := filepath.Join(taskDir, runEntry.Name(), "owned-lane-bootstrap.json")
		raw, err := os.ReadFile(recordPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", ownedLaneBootstrapRecord{}, fmt.Errorf("read owned-lane bootstrap %s: %w", recordPath, err)
		}
		var record ownedLaneBootstrapRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return "", ownedLaneBootstrapRecord{}, fmt.Errorf("decode owned-lane bootstrap %s: %w", recordPath, err)
		}
		if record.OwnedRepoRoot == "" {
			continue
		}
		if record.DeclaredWorktreeRoot != "" && !sameRepoRoot(record.DeclaredWorktreeRoot, s.declaredWorktreeRoot) {
			// Another repo's lane sharing the Task-NNNN runs-root dir: never resolve
			// it as THIS repo's active lane (BUG-0003 residual: Set/Bind/Reclaim/
			// ClosureRequested must be repo-scoped too, not just slot accounting).
			continue
		}
		if _, err := os.Stat(record.OwnedRepoRoot); err != nil {
			// Worktree gone (reclaimed on close) -> not an active lane.
			continue
		}
		if activePath == "" || record.BootstrappedAt.After(activeRecord.BootstrappedAt) {
			activePath = recordPath
			activeRecord = record
		}
	}
	if activePath == "" {
		return "", ownedLaneBootstrapRecord{}, fmt.Errorf("%w: %s", ErrNoActiveOwnedLane, taskID)
	}
	return activePath, activeRecord, nil
}

// ReclaimOwnedLane performs the O4 terminal-close handling for a task: it reclaims
// the task's active owned-lane worktree (cleanupOwnedLane) so the per-repo slot
// frees and the next Ready issue can dequeue. It is the ONLY deallocating action
// and is invoked by the consumer ONLY when the GitHub issue is CLOSED — never on a
// park (Human Needed=Yes retains the worktree+slot). A task with no active owned
// lane returns ErrNoActiveOwnedLane, which the consumer treats as already-reclaimed.
func (s *Service) ReclaimOwnedLane(taskID string) error {
	_, activeRecord, err := s.findActiveLaneRecord(taskID)
	if err != nil {
		return err
	}
	// BUG-0002: terminate the launched agent BEFORE removing the worktree, so its
	// open handle on the checkout does not make git worktree remove --force
	// partially fail and leave a residual directory. Best-effort: a missing PID or
	// a kill failure never blocks the reclaim (cleanupOwnedLane is self-healing).
	if activeRecord.Binding != nil && activeRecord.Binding.LaunchedPID > 0 {
		terminateAgentProcess(activeRecord.Binding.LaunchedPID)
	}
	return s.cleanupOwnedLane(RepoLane{OwnedRepoRoot: activeRecord.OwnedRepoRoot})
}

// terminateAgentProcess best-effort terminates the launched agent process (and its
// tree) before the owned worktree is removed (BUG-0002). It NEVER returns a fatal
// error that could block reclaim: cleanupOwnedLane self-heals if the process is
// still mid-exit. On Windows it first verifies the PID's image is the claude
// executable (tasklist) so a reused PID is never killed, then taskkill /T /F. On
// other platforms it signals the process directly. An already-exited / not-found
// process is ignored.
func terminateAgentProcess(pid int) {
	if pid <= 0 {
		return
	}
	if runtime.GOOS == "windows" {
		// Guard against killing an innocent reused PID: only proceed when the PID's
		// current image is claude. A lookup failure or a non-claude image is treated
		// as "nothing to kill".
		out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
		if err != nil || !strings.Contains(strings.ToLower(string(out)), "claude") {
			return
		}
		_ = exec.Command("taskkill", "/PID", fmt.Sprintf("%d", pid), "/T", "/F").Run()
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Kill()
}

// ClosureRequested reports whether the task's dispatched agent has ANNOUNCED
// completion by setting current_gate to "closure" in its OWNED worktree's
// Tracking/<taskID>/TASK-STATE.json. It is read by the TEST-ONLY auto-close path
// (OBSIDIAN_AUTO_CLOSE_QUEUED) so the consumer can simulate a human closing the
// issue. A task with no active owned lane (ErrNoActiveOwnedLane) or a missing state
// file reports false, nil — not an error — so a poll never wedges on an absent file.
func (s *Service) ClosureRequested(taskID string) (bool, error) {
	_, activeRecord, err := s.findActiveLaneRecord(taskID)
	if err != nil {
		if errors.Is(err, ErrNoActiveOwnedLane) {
			return false, nil
		}
		return false, err
	}
	return closureRequestedAtRoot(activeRecord.OwnedRepoRoot, taskID)
}

// closureRequestedAtRoot reads <ownedRepoRoot>/Tracking/<taskID>/TASK-STATE.json and
// reports whether its current_gate is "closure". A missing file reports false, nil
// (the agent has not announced completion yet). It reuses the taskStateFile shape
// the loadTask read uses so the gate field decodes identically.
func closureRequestedAtRoot(ownedRepoRoot string, taskID string) (bool, error) {
	statePath := filepath.Join(ownedRepoRoot, "Tracking", taskID, "TASK-STATE.json")
	raw, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read %s: %w", statePath, err)
	}
	var state taskStateFile
	if err := json.Unmarshal(raw, &state); err != nil {
		return false, fmt.Errorf("decode %s: %w", statePath, err)
	}
	return state.CurrentGate == "closure", nil
}

// ActiveOwnedLaneTasks returns the task ids that currently hold an active owned-lane
// worktree (one per allocated pool worktree), deduped and sorted. The consumer uses it
// to know which issues already own a worktree to park or reclaim rather than redispatch.
func (s *Service) ActiveOwnedLaneTasks() ([]string, error) {
	// REPO-SCOPED (repoScoped=true): the consumer's slot/active accounting must only
	// see THIS repo's lanes, never another registry repo's (BUG-0003: a global view
	// let repo A's closed #1 reclaim repo B's live Task-0001).
	byWorktree, err := s.collectActiveLaneRecords(true)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	tasks := make([]string, 0, len(byWorktree))
	for _, record := range byWorktree {
		taskID := bindingFromRecord(record).TaskID
		if taskID == "" || seen[taskID] {
			continue
		}
		seen[taskID] = true
		tasks = append(tasks, taskID)
	}
	sort.Strings(tasks)
	return tasks, nil
}

func (s *Service) ListTasks(ctx context.Context) ([]TaskView, error) {
	entries, err := os.ReadDir(s.trackingRoot)
	if err != nil {
		return nil, fmt.Errorf("read tracking root: %w", err)
	}

	tasks := make([]TaskView, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "Task-") {
			continue
		}
		task, err := s.readTask(ctx, filepath.Join(s.trackingRoot, entry.Name()))
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].TaskID < tasks[j].TaskID
	})
	return tasks, nil
}

func (s *Service) Task(ctx context.Context, taskID string) (TaskView, error) {
	taskRoot := filepath.Join(s.trackingRoot, taskID)
	if _, err := os.Stat(taskRoot); err != nil {
		if os.IsNotExist(err) {
			return TaskView{}, fmt.Errorf("task %q not found", taskID)
		}
		return TaskView{}, err
	}
	return s.readTask(ctx, taskRoot)
}

func (s *Service) Dispatch(ctx context.Context, taskID string) (TaskRunView, error) {
	return s.dispatchWithDirective(ctx, taskID, nil)
}

func (s *Service) DispatchWorkloadFailureExercise(ctx context.Context, taskID string) (TaskRunView, error) {
	if taskID != "Task-0008" {
		return TaskRunView{}, fmt.Errorf("workload failure exercise is only implemented for Task-0008")
	}
	return s.dispatchWithDirective(ctx, taskID, &ExecutionDirective{
		FailureMode: ExecutionFailureModeTask0008WorkloadFailureOnce,
	})
}

func (s *Service) dispatchWithDirective(ctx context.Context, taskID string, directive *ExecutionDirective) (TaskRunView, error) {
	if s.runtime == nil {
		return TaskRunView{}, fmt.Errorf("task runtime backend is not configured")
	}

	task, err := s.Task(ctx, taskID)
	if err != nil {
		return TaskRunView{}, err
	}
	if !task.DispatchReadiness.Ready {
		return TaskRunView{}, fmt.Errorf("dispatch blocked: %s", summarizeBlockReasons(task.DispatchReadiness.BlockReasons))
	}
	if err := s.releasePreviousOwnedLane(ctx, task.TaskID); err != nil {
		return TaskRunView{}, err
	}

	// Pool-draw (Task-0016): draw an IDLE pool worktree and reset it to baseline instead
	// of provisioning a fresh os.MkdirTemp dir. An empty pool defers the dispatch (no
	// auto-create — capacity is operator-owned).
	drawn, err := s.drawIdlePoolWorktree("")
	if err != nil {
		return TaskRunView{}, err
	}
	return s.startRunInDrawnLane(ctx, task, directive, drawn)
}

// startRunInDrawnLane is the shared bootstrap->start tail used by both the queue-drain
// dispatch path and the manual Assign action: it bootstraps the already-reset drawn pool
// worktree and starts the run IN IT (no fresh dir), then marks the pool member allocated
// (records the run id). On a pre-start failure the pool member is left IDLE (never
// deleted — it is a persistent pool worktree, not a per-dispatch temp dir).
func (s *Service) startRunInDrawnLane(ctx context.Context, task TaskView, directive *ExecutionDirective, drawn drawnLane) (TaskRunView, error) {
	request := StartTaskRunRequest{
		RunID:          s.runID(task.TaskID),
		TaskID:         task.TaskID,
		Title:          task.Title,
		MeaningSummary: task.MeaningSummary,
		CapturedTaskSnapshot: TaskDefinitionSnapshot{
			DeclaredWorktreeRoot: task.DeclaredWorktreeRoot,
			DeclaredTaskRoot:     task.DeclaredTaskRoot,
			DeclaredTaskRevision: task.DeclaredTaskRevision,
			DeclaredGitRevision:  task.DeclaredGitRevision,
			CapturedAt:           s.now(),
			Files:                nil,
		},
		ExecutionDirective:  directive,
		ContextSnapshot:     captureDispatchContext(),
		RepoLane:            drawn.repoLane,
		DispatchRequestedAt: s.now(),
	}

	metadata, err := s.loadTask(taskRootForID(s.trackingRoot, task.TaskID))
	if err != nil {
		return TaskRunView{}, err
	}
	request.CapturedTaskSnapshot = metadata.snapshot
	request.RepoLane, err = s.bootstrapOwnedLane(request.TaskID, request.RunID, request.CapturedTaskSnapshot, request.RepoLane, request.ContextSnapshot)
	if err != nil {
		return TaskRunView{}, err
	}

	run, err := s.runtime.StartTaskRun(ctx, request)
	if err != nil {
		return TaskRunView{}, err
	}
	// Mark the drawn pool member allocated to the started run (idle -> allocated).
	if err := s.markPoolMemberRun(drawn, request.RunID); err != nil {
		return TaskRunView{}, fmt.Errorf("record pool worktree run binding: %w", err)
	}
	run.DeepContext = runDeepContext(run)
	return run, nil
}

// AssignTaskToPoolWorktree binds a chosen open task onto an IDLE pool worktree and starts
// the run IN THAT EXISTING worktree (Task-0016 manual Assign). With worktreeID set it
// draws that specific idle member; with worktreeID empty (the consumer auto-assign path)
// it draws any idle worktree in the repo. It resets the existing checkout to baseline and
// reuses the dispatch bootstrap->start tail WITHOUT provisioning a fresh dir; an empty
// pool yields ErrNoIdleWorktree (no run started). The repo argument is accepted for
// symmetry with the request shape; the worktree is always this Service's repo pool.
func (s *Service) AssignTaskToPoolWorktree(ctx context.Context, taskID string, repo string, worktreeID string) (TaskRunView, error) {
	if s.runtime == nil {
		return TaskRunView{}, fmt.Errorf("task runtime backend is not configured")
	}
	task, err := s.Task(ctx, taskID)
	if err != nil {
		return TaskRunView{}, err
	}
	if err := s.releasePreviousOwnedLane(ctx, task.TaskID); err != nil {
		return TaskRunView{}, err
	}
	drawn, err := s.drawIdlePoolWorktree(worktreeID)
	if err != nil {
		return TaskRunView{}, err
	}
	return s.startRunInDrawnLane(ctx, task, nil, drawn)
}

func (s *Service) Run(ctx context.Context, runID string) (TaskRunView, error) {
	if s.runtime == nil {
		return TaskRunView{}, fmt.Errorf("task runtime backend is not configured")
	}
	run, err := s.runtime.GetTaskRun(ctx, runID)
	if err != nil {
		return TaskRunView{}, err
	}
	run, err = s.refreshRun(ctx, run)
	if err != nil {
		return TaskRunView{}, err
	}
	run.DeepContext = runDeepContext(run)
	return run, nil
}

func (s *Service) UpdateRun(ctx context.Context, runID string, update TaskRunUpdate) (TaskRunView, error) {
	if s.runtime == nil {
		return TaskRunView{}, fmt.Errorf("task runtime backend is not configured")
	}
	current, err := s.runtime.GetTaskRun(ctx, runID)
	if err != nil {
		return TaskRunView{}, err
	}
	now := s.now()
	if update.FollowUp == nil {
		update.FollowUp = derivedFollowUp(current, update, now)
	}
	projected := projectRun(current, update, now)
	if update.Actions == nil {
		update.Actions = actionsForRun(projected, now)
	}
	if update.Attention == nil {
		attention := attentionForRun(projected, now)
		update.Attention = &attention
	}
	return s.runtime.UpdateTaskRun(ctx, runID, update)
}

func (s *Service) PokeRun(ctx context.Context, runID string) (TaskRunView, error) {
	run, err := s.Run(ctx, runID)
	if err != nil {
		return TaskRunView{}, err
	}
	availability := run.Actions[ActionPoke]
	if !availability.Allowed {
		return TaskRunView{}, fmt.Errorf("poke blocked: %s", summarizeBlockReasons(availability.BlockReasons))
	}

	now := s.now()
	update := TaskRunUpdate{
		State:               StateSleepingOrStalled,
		ReasonCode:          "poke_requested",
		StateSummary:        "Run was poked and is waiting for a fresh backend progress signal.",
		NextOwner:           "backend",
		NextExpectedEvent:   "Execution worker records a fresh progress update or explicit wait reason.",
		SuspiciousAfter:     now.Add(10 * time.Minute),
		LastProgressSummary: "Backend requested a fresh status update for the stalled run.",
		FollowUp: &RunFollowUp{
			Kind:        "poke_worker_check",
			Owner:       "backend_worker",
			Status:      "pending",
			Summary:     "Execution worker should acknowledge the poke with fresh progress or an explicit wait reason.",
			RequestedAt: now,
			DueAt:       now.Add(5 * time.Minute),
		},
	}
	return s.UpdateRun(ctx, runID, update)
}

func (s *Service) InterruptRun(ctx context.Context, runID string) (TaskRunView, error) {
	run, err := s.Run(ctx, runID)
	if err != nil {
		return TaskRunView{}, err
	}
	availability := run.Actions[ActionInterrupt]
	if !availability.Allowed {
		return TaskRunView{}, fmt.Errorf("interrupt blocked: %s", summarizeBlockReasons(availability.BlockReasons))
	}

	repoLane, resetErr := s.restoreOwnedLane(run.RepoLane)
	if resetErr != nil {
		update := TaskRunUpdate{
			State:               StateBlocked,
			ReasonCode:          "interrupt_cleanup_blocked",
			StateSummary:        "Run interrupt could not restore the owned checkout.",
			NextOwner:           "human_or_supervisor",
			NextExpectedEvent:   "Review cleanup failure and resolve the owned checkout manually.",
			LastProgressSummary: "Interrupt cleanup failed and the owned checkout needs manual review.",
			FollowUp: &RunFollowUp{
				Kind:        "cleanup_repair",
				Owner:       "human_or_supervisor",
				Status:      "pending",
				Summary:     "Repair the cleanup-blocked owned checkout before attempting another interrupt or redispatch.",
				RequestedAt: s.now(),
				DueAt:       s.now().Add(24 * time.Hour),
			},
			RepoLane:       &repoLane,
			FailureSummary: resetErr.Error(),
		}
		return s.UpdateRun(ctx, runID, update)
	}

	now := s.now()
	update := TaskRunUpdate{
		State:               StateInterrupted,
		ReasonCode:          "interrupt_requested",
		StateSummary:        "Run was interrupted and the owned checkout was restored.",
		NextOwner:           "human_or_supervisor",
		NextExpectedEvent:   "Review the interrupted run and decide whether to dispatch again.",
		SuspiciousAfter:     now,
		LastProgressSummary: "Interrupt restored the owned checkout to its recorded restore commit.",
		FollowUp: &RunFollowUp{
			Kind:        "interrupt_review",
			Owner:       "human_or_supervisor",
			Status:      "pending",
			Summary:     "Review the interrupted run and decide whether to redispatch, revise the task docs, or close the attempt.",
			RequestedAt: now,
			DueAt:       now.Add(24 * time.Hour),
		},
		RepoLane:    &repoLane,
		CompletedAt: now,
	}
	return s.UpdateRun(ctx, runID, update)
}

func (s *Service) RetryCleanupRun(ctx context.Context, runID string) (TaskRunView, error) {
	run, err := s.Run(ctx, runID)
	if err != nil {
		return TaskRunView{}, err
	}
	if run.StateEnvelope.State != StateBlocked || run.StateEnvelope.ReasonCode != "interrupt_cleanup_blocked" {
		return TaskRunView{}, fmt.Errorf("cleanup retry blocked: run is not waiting on cleanup repair")
	}

	repoLane, resetErr := s.restoreOwnedLane(run.RepoLane)
	now := s.now()
	if resetErr != nil {
		update := TaskRunUpdate{
			State:               StateBlocked,
			ReasonCode:          "interrupt_cleanup_blocked",
			StateSummary:        "Cleanup retry could not restore the owned checkout.",
			NextOwner:           "human_or_supervisor",
			NextExpectedEvent:   "Repair the owned checkout or retry cleanup again.",
			LastProgressSummary: "Backend cleanup retry failed and the owned checkout still needs repair.",
			FollowUp: &RunFollowUp{
				Kind:        "cleanup_repair",
				Owner:       "human_or_supervisor",
				Status:      "pending",
				Summary:     "Repair the cleanup-blocked owned checkout or retry cleanup again after the restore target is valid.",
				RequestedAt: now,
				DueAt:       now.Add(24 * time.Hour),
			},
			RepoLane:       &repoLane,
			FailureSummary: resetErr.Error(),
		}
		return s.UpdateRun(ctx, runID, update)
	}

	update := TaskRunUpdate{
		State:               StateInterrupted,
		ReasonCode:          "interrupt_cleanup_repaired",
		StateSummary:        "Cleanup retry restored the owned checkout and the run now needs interrupt review.",
		NextOwner:           "human_or_supervisor",
		NextExpectedEvent:   "Review the repaired interrupted run and decide whether to dispatch again.",
		SuspiciousAfter:     now,
		LastProgressSummary: "Cleanup retry restored the owned checkout to its recorded restore commit.",
		FollowUp: &RunFollowUp{
			Kind:        "interrupt_review",
			Owner:       "human_or_supervisor",
			Status:      "pending",
			Summary:     "Cleanup repair completed; review the interrupted run and decide whether to redispatch, revise the task docs, or close the attempt.",
			RequestedAt: now,
			DueAt:       now.Add(24 * time.Hour),
		},
		RepoLane:    &repoLane,
		CompletedAt: now,
	}
	return s.UpdateRun(ctx, runID, update)
}

func (s *Service) RetryWorkloadRun(ctx context.Context, runID string) (TaskRunView, error) {
	run, err := s.Run(ctx, runID)
	if err != nil {
		return TaskRunView{}, err
	}
	if run.StateEnvelope.State != StateBlocked || run.StateEnvelope.ReasonCode != "workload_execution_failed" {
		return TaskRunView{}, fmt.Errorf("workload retry blocked: run is not waiting on workload execution recovery")
	}

	if err := s.cleanupOwnedLane(run.RepoLane); err != nil {
		return TaskRunView{}, fmt.Errorf("workload retry blocked: %w", err)
	}

	metadata, err := s.loadTask(taskRootForID(s.trackingRoot, run.TaskID))
	if err != nil {
		return TaskRunView{}, err
	}

	repoLane, err := s.provisionOwnedLane(run.TaskID)
	if err != nil {
		return TaskRunView{}, err
	}
	repoLane, err = s.bootstrapOwnedLane(run.TaskID, run.RunID, metadata.snapshot, repoLane, run.DeepContext)
	if err != nil {
		_ = s.cleanupOwnedLane(repoLane)
		return TaskRunView{}, err
	}

	retried, err := s.runtime.RetryTaskRunWorkload(ctx, runID, WorkloadRetryRequest{
		CapturedTaskSnapshot: metadata.snapshot,
		RepoLane:             repoLane,
		RetryRequestedAt:     s.now(),
	})
	if err != nil {
		_ = s.cleanupOwnedLane(repoLane)
		return TaskRunView{}, err
	}
	return retried, nil
}

func (s *Service) ResolveInterruptReview(ctx context.Context, runID string, resolution InterruptReviewResolution) (TaskRunView, error) {
	run, err := s.Run(ctx, runID)
	if err != nil {
		return TaskRunView{}, err
	}
	if !hasPendingInterruptReview(run) {
		return TaskRunView{}, fmt.Errorf("interrupt review resolution blocked: run is not waiting on interrupt review")
	}

	now := s.now()
	resolvedBy := strings.TrimSpace(resolution.ResolvedBy)
	if resolvedBy == "" {
		resolvedBy = "human_or_supervisor"
	}

	decision := strings.TrimSpace(resolution.Decision)
	switch decision {
	case "redispatch_ready":
		summary := strings.TrimSpace(resolution.Summary)
		if summary == "" {
			summary = "Interrupt review approved the run for a later redispatch."
		}
		repoLane, progressSummary, err := s.releaseResolvedOwnedLane(run.RepoLane, summary)
		if err != nil {
			return TaskRunView{}, err
		}
		return s.UpdateRun(ctx, runID, TaskRunUpdate{
			State:               StateInterrupted,
			ReasonCode:          "interrupt_review_resolved_redispatch_ready",
			StateSummary:        "Interrupt review approved the run for redispatch and backend released the prior owned lane.",
			NextOwner:           "backend",
			NextExpectedEvent:   "Dispatch a new run when the task is ready.",
			LastProgressSummary: progressSummary,
			FollowUp: &RunFollowUp{
				Kind:        "interrupt_review",
				Owner:       "human_or_supervisor",
				Status:      "completed",
				Summary:     summary,
				RequestedAt: run.FollowUp.RequestedAt,
				DueAt:       run.FollowUp.DueAt,
				CompletedAt: now,
			},
			Resolution: &RunResolution{
				Kind:       "interrupt_review",
				Decision:   decision,
				Summary:    summary,
				ResolvedBy: resolvedBy,
				ResolvedAt: now,
			},
			RepoLane: &repoLane,
		})
	case "keep_closed":
		summary := strings.TrimSpace(resolution.Summary)
		if summary == "" {
			summary = "Interrupt review closed this interrupted attempt without redispatch."
		}
		repoLane, progressSummary, err := s.releaseResolvedOwnedLane(run.RepoLane, summary)
		if err != nil {
			return TaskRunView{}, err
		}
		return s.UpdateRun(ctx, runID, TaskRunUpdate{
			State:               StateInterrupted,
			ReasonCode:          "interrupt_review_resolved_keep_closed",
			StateSummary:        "Interrupt review closed this interrupted attempt and backend released the prior owned lane.",
			NextOwner:           "none",
			NextExpectedEvent:   "No further action is required for this run.",
			LastProgressSummary: progressSummary,
			FollowUp: &RunFollowUp{
				Kind:        "interrupt_review",
				Owner:       "human_or_supervisor",
				Status:      "completed",
				Summary:     summary,
				RequestedAt: run.FollowUp.RequestedAt,
				DueAt:       run.FollowUp.DueAt,
				CompletedAt: now,
			},
			Resolution: &RunResolution{
				Kind:       "interrupt_review",
				Decision:   decision,
				Summary:    summary,
				ResolvedBy: resolvedBy,
				ResolvedAt: now,
			},
			RepoLane: &repoLane,
		})
	default:
		return TaskRunView{}, fmt.Errorf("interrupt review resolution blocked: unsupported decision %q", decision)
	}
}

func (s *Service) releaseResolvedOwnedLane(repoLane RepoLane, summary string) (RepoLane, string, error) {
	if repoLane.OwnedRepoRoot == "" {
		return repoLane, summary, nil
	}

	restoreCommit := repoLane.ApprovedRestoreCommit
	if restoreCommit == "" {
		restoreCommit = repoLane.BaselineCommit
	}
	// Pool model (Task-0016): if the resolved run occupied a POOL worktree, return it to
	// idle (folder kept, run_id cleared) for reuse rather than deleting it. A legacy
	// non-pool (random-temp) lane still uses the delete path.
	isPoolMember, err := s.returnPoolMemberToIdle(repoLane.OwnedRepoRoot)
	if err != nil {
		return repoLane, "", fmt.Errorf("release resolved owned lane: %w", err)
	}
	if !isPoolMember {
		if err := s.cleanupOwnedLane(repoLane); err != nil {
			return repoLane, "", fmt.Errorf("release resolved owned lane: %w", err)
		}
	}

	repoLane.OwnedRepoRoot = ""
	repoLane.ResetStatus = "released"
	repoLane.ResetFailureSummary = ""
	repoLane.LastResetTargetCommit = restoreCommit
	repoLane.LastResetAt = s.now()

	progressSummary := strings.TrimSpace(summary)
	if progressSummary == "" {
		progressSummary = "Interrupt review resolved and backend released the prior owned lane."
	} else {
		progressSummary += " Backend released the prior owned lane."
	}
	return repoLane, progressSummary, nil
}

func (s *Service) readTask(ctx context.Context, taskRoot string) (TaskView, error) {
	metadata, err := s.loadTask(taskRoot)
	if err != nil {
		return TaskView{}, err
	}

	view := TaskView{
		TaskID:               metadata.state.TaskID,
		Title:                metadata.title,
		MeaningSummary:       metadata.meaning,
		DeclaredWorktreeRoot: metadata.snapshot.DeclaredWorktreeRoot,
		DeclaredTaskRoot:     metadata.snapshot.DeclaredTaskRoot,
		DeclaredTaskRevision: metadata.snapshot.DeclaredTaskRevision,
		DeclaredGitRevision:  metadata.snapshot.DeclaredGitRevision,
		CurrentStory: StoryOwnership{
			Status: "no_active_run",
			Reason: "No task run currently owns the live story.",
		},
		CurrentGate:  metadata.state.CurrentGate,
		CurrentPass:  metadata.state.CurrentPass,
		Phase:        metadata.state.Phase,
		PlanApproved: metadata.state.PlanApproved,
		Blockers:     append([]string(nil), metadata.state.Blockers...),
		UpdatedAt:    metadata.state.UpdatedAt,
		DeepContext:  taskDeepContext(metadata),
	}

	view.StateEnvelope = s.deriveStateEnvelope(metadata)
	view.DispatchReadiness = s.deriveDispatchReadiness(metadata, false)
	view.Attention = s.deriveAttention(view.StateEnvelope.State, view.DispatchReadiness.Ready)
	view.Actions = defaultActions(view.DispatchReadiness)
	view.StateEnvelope.ActionBlockReasons = collectActionBlockReasons(view.Actions)

	if s.runtime == nil || isTerminalTaskState(metadata.state.Status) {
		return view, nil
	}

	run, err := s.runtime.GetActiveTaskRun(ctx, s.runID(metadata.state.TaskID))
	if err != nil {
		if errors.Is(err, ErrRunNotFound) {
			return view, nil
		}
		return TaskView{}, err
	}

	if run.CapturedTaskSnapshot.DeclaredTaskRevision != metadata.snapshot.DeclaredTaskRevision {
		reconciled, reconcileErr := s.runtime.ReconcileTaskSnapshot(ctx, run.RunID, metadata.snapshot)
		if reconcileErr == nil {
			run = reconciled
		}
	}
	run, err = s.refreshRun(ctx, run)
	if err != nil {
		return TaskView{}, err
	}
	run.DeepContext = runDeepContext(run)

	view.LatestRun = &run
	if runOwnsLiveStory(run) {
		view.CurrentStory = StoryOwnership{
			OwnerRunID: run.RunID,
			Status:     "active_run",
			Reason:     "An active task run owns the current live story.",
		}
		view.StateEnvelope = run.StateEnvelope
		view.Attention = run.Attention
		view.Actions = run.Actions
		view.DispatchReadiness = s.deriveDispatchReadiness(metadata, true)
		view.StateEnvelope.ActionBlockReasons = collectActionBlockReasons(view.Actions)
		view.DeepContext = run.DeepContext
	} else {
		view.CurrentStory = StoryOwnership{
			Status: "no_active_run",
			Reason: "The latest task run is terminal and no run currently owns the live story.",
		}
		if hasPendingInterruptReview(run) {
			view.StateEnvelope = StateEnvelope{
				State:             StateWaitingForHuman,
				ReasonCode:        "interrupt_review_pending",
				StateSummary:      "Task is waiting for interrupt review before redispatch.",
				EvidenceRefs:      metadata.evidenceRef,
				NextOwner:         "human_or_supervisor",
				NextExpectedEvent: "Resolve the interrupted run review decision.",
				SuspiciousAfter:   run.FollowUp.DueAt,
			}
			view.DispatchReadiness = DispatchReadiness{
				Ready: false,
				BlockReasons: []ActionBlockReason{{
					Code:    "interrupt_review_pending",
					Summary: "Dispatch stays blocked until the interrupted run review is resolved.",
				}},
			}
			view.Attention = attentionForRun(run, s.now())
			view.Actions = defaultActions(view.DispatchReadiness)
			view.StateEnvelope.ActionBlockReasons = collectActionBlockReasons(view.Actions)
		} else {
			view.DispatchReadiness = s.deriveDispatchReadiness(metadata, false)
			view.Actions = defaultActions(view.DispatchReadiness)
			view.StateEnvelope.ActionBlockReasons = collectActionBlockReasons(view.Actions)
		}
	}

	return view, nil
}

func (s *Service) loadTask(taskRoot string) (parsedTask, error) {
	taskMDPath := filepath.Join(taskRoot, "TASK.md")
	taskRaw, err := os.ReadFile(taskMDPath)
	if err != nil {
		return parsedTask{}, fmt.Errorf("read %s: %w", taskMDPath, err)
	}

	taskStatePath := filepath.Join(taskRoot, "TASK-STATE.json")
	taskStateRaw, err := os.ReadFile(taskStatePath)
	if err != nil {
		return parsedTask{}, fmt.Errorf("read %s: %w", taskStatePath, err)
	}

	var state taskStateFile
	if err := json.Unmarshal(taskStateRaw, &state); err != nil {
		return parsedTask{}, fmt.Errorf("decode %s: %w", taskStatePath, err)
	}

	title := extractMarkdownSection(string(taskRaw), "Title")
	if title == "" {
		title = strings.TrimSpace(state.TaskID)
	}
	meaning := firstParagraph(extractMarkdownSection(string(taskRaw), "Summary"))
	if meaning == "" {
		meaning = title
	}

	snapshot, err := s.captureSnapshot(taskRoot)
	if err != nil {
		return parsedTask{}, err
	}

	evidenceRefs := []EvidenceRef{
		taskArtifactRef("TASK.md", taskMDPath),
		taskArtifactRef("TASK-STATE.json", taskStatePath),
	}
	if _, err := os.Stat(filepath.Join(taskRoot, "PLAN.md")); err == nil {
		evidenceRefs = append(evidenceRefs, taskArtifactRef("PLAN.md", filepath.Join(taskRoot, "PLAN.md")))
	}
	if _, err := os.Stat(filepath.Join(taskRoot, "HANDOFF.md")); err == nil {
		evidenceRefs = append(evidenceRefs, taskArtifactRef("HANDOFF.md", filepath.Join(taskRoot, "HANDOFF.md")))
	}
	if _, err := os.Stat(filepath.Join(taskRoot, "CONSTRAINTS.md")); err == nil {
		evidenceRefs = append(evidenceRefs, taskArtifactRef("CONSTRAINTS.md", filepath.Join(taskRoot, "CONSTRAINTS.md")))
	}

	return parsedTask{
		state:       state,
		title:       title,
		meaning:     meaning,
		snapshot:    snapshot,
		evidenceRef: evidenceRefs,
		taskRoot:    taskRoot,
	}, nil
}

func (s *Service) deriveStateEnvelope(metadata parsedTask) StateEnvelope {
	now := s.now()
	envelope := StateEnvelope{
		EvidenceRefs: metadata.evidenceRef,
		NextOwner:    "backend",
	}

	switch {
	case isCompletedTaskStatus(metadata.state.Status):
		envelope.State = StateCompleted
		envelope.ReasonCode = "task_complete"
		envelope.StateSummary = "Task is complete."
		envelope.NextOwner = "none"
		envelope.NextExpectedEvent = "No further action is required."
	case isCancelledTaskStatus(metadata.state.Status):
		envelope.State = StateCancelled
		envelope.ReasonCode = "task_cancelled"
		envelope.StateSummary = "Task is cancelled."
		envelope.NextOwner = "none"
		envelope.NextExpectedEvent = "No further action is required."
	case len(metadata.state.Blockers) > 0:
		envelope.State = StateBlocked
		envelope.ReasonCode = "task_blocked"
		envelope.StateSummary = "Task is blocked on recorded constraints."
		envelope.NextOwner = "human_or_supervisor"
		envelope.NextExpectedEvent = "Resolve blockers and reassess dispatch readiness."
		envelope.SuspiciousAfter = now.Add(24 * time.Hour)
	case !metadata.state.PlanApproved:
		envelope.State = StateWaitingForHuman
		envelope.ReasonCode = "plan_approval_required"
		envelope.StateSummary = "Task is waiting for plan approval."
		envelope.NextOwner = "human"
		envelope.NextExpectedEvent = "Approve PLAN.md."
		envelope.SuspiciousAfter = now.Add(72 * time.Hour)
	case metadata.state.Phase == "implementation" && metadata.state.CurrentGate == "implementation":
		envelope.State = StateReady
		envelope.ReasonCode = "ready_for_dispatch"
		envelope.StateSummary = "Task is ready for backend dispatch."
		envelope.NextExpectedEvent = "Dispatch a backend task run."
		envelope.SuspiciousAfter = now.Add(12 * time.Hour)
	default:
		envelope.State = StateBlocked
		envelope.ReasonCode = "task_state_unmapped"
		envelope.StateSummary = "Task needs backend review before dispatch."
		envelope.NextOwner = "backend"
		envelope.NextExpectedEvent = "Review task docs and runtime contract."
		envelope.SuspiciousAfter = now.Add(24 * time.Hour)
	}

	if envelope.State == StateWaitingForHuman && envelope.ReasonCode == "plan_approval_required" {
		envelope.EvidenceRefs = append(envelope.EvidenceRefs, EvidenceRef{
			Type:  "task_artifact",
			Label: "PLAN approval target",
			URI:   fileURI(filepath.Join(metadata.taskRoot, "PLAN.md")),
		})
	}

	return envelope
}

func (s *Service) deriveDispatchReadiness(metadata parsedTask, activeRunExists bool) DispatchReadiness {
	readiness := DispatchReadiness{
		Ready:        false,
		BlockReasons: []ActionBlockReason{},
	}

	if s.runtime == nil {
		readiness.BlockReasons = append(readiness.BlockReasons, ActionBlockReason{
			Code:    "dispatch_runtime_not_implemented",
			Summary: "The durable dispatch lane is not implemented yet.",
		})
	}
	if !metadata.state.PlanApproved {
		readiness.BlockReasons = append(readiness.BlockReasons, ActionBlockReason{
			Code:    "plan_not_approved",
			Summary: "Dispatch requires an approved plan.",
		})
	}
	if len(metadata.state.Blockers) > 0 {
		readiness.BlockReasons = append(readiness.BlockReasons, ActionBlockReason{
			Code:    "task_blockers_present",
			Summary: "Dispatch is blocked until the recorded task blockers are cleared.",
		})
	}
	if isTerminalTaskState(metadata.state.Status) {
		readiness.BlockReasons = append(readiness.BlockReasons, ActionBlockReason{
			Code:    "task_terminal",
			Summary: "Dispatch is unavailable because the task is already terminal.",
		})
	}
	if blocked, reason := s.repoSlotBlock(activeRunExists); blocked {
		readiness.BlockReasons = append(readiness.BlockReasons, reason)
	}
	if _, err := os.Stat(filepath.Join(metadata.taskRoot, "PLAN.md")); err != nil {
		readiness.BlockReasons = append(readiness.BlockReasons, ActionBlockReason{
			Code:    "plan_missing",
			Summary: "Dispatch requires PLAN.md to be present in the declared task root.",
		})
	}
	if _, err := os.Stat(filepath.Join(metadata.taskRoot, "TASK.md")); err != nil {
		readiness.BlockReasons = append(readiness.BlockReasons, ActionBlockReason{
			Code:    "task_missing",
			Summary: "Dispatch requires TASK.md to be present in the declared task root.",
		})
	}
	if metadata.snapshot.DeclaredGitRevision == "" {
		readiness.BlockReasons = append(readiness.BlockReasons, ActionBlockReason{
			Code:    "baseline_commit_unavailable",
			Summary: "Dispatch requires a resolvable git baseline for the declared worktree.",
		})
	}

	if len(readiness.BlockReasons) == 0 {
		readiness.Ready = true
		readiness.ExpectedFirstSignal = "Create a durable backend task run with an owned checkout and captured baseline commit."
		readiness.FirstSuspiciousAfter = s.now().Add(15 * time.Minute)
	}

	return readiness
}

// repoSlotBlock applies the Task-0016 pool-draw dispatch gate. A task that already
// owns the live story is blocked from a duplicate dispatch (task_already_running).
// Otherwise dispatch requires an IDLE pool worktree to draw: with an empty pool the
// dispatch is refused (no_idle_worktree) because the consumer/manual dispatch can only
// draw from existing idle worktrees (no auto-create — capacity is operator-owned). The
// numeric queue_workers cap is gone; the idle pool count is the bound, by construction.
func (s *Service) repoSlotBlock(activeRunExists bool) (bool, ActionBlockReason) {
	if activeRunExists {
		return true, ActionBlockReason{
			Code:    "task_already_running",
			Summary: "Dispatch is blocked while this task already owns an active run.",
		}
	}
	if s.idleWorktreeCount == nil {
		return false, ActionBlockReason{}
	}
	idle, err := s.idleWorktreeCount()
	if err != nil {
		// Admission accounting is best-effort; if the idle count is unavailable, fall
		// back to allowing dispatch rather than wedging the queue on a transient error.
		return false, ActionBlockReason{}
	}
	if idle > 0 {
		return false, ActionBlockReason{}
	}
	return true, ActionBlockReason{
		Code:    "no_idle_worktree",
		Summary: "Dispatch is blocked: the repo worktree pool has no idle worktree to draw (create one or eject an allocated one).",
	}
}

func (s *Service) deriveAttention(state string, dispatchReady bool) AttentionPriority {
	switch {
	case state == StateWaitingForHuman:
		return AttentionPriority{Level: AttentionNeedsAttention, Reason: "Task needs an explicit human action.", SortKey: "20-waiting_for_human"}
	case state == StateBlocked:
		return AttentionPriority{Level: AttentionNeedsAttention, Reason: "Task is blocked and needs review.", SortKey: "30-blocked"}
	case dispatchReady:
		return AttentionPriority{Level: AttentionNeedsAttention, Reason: "Task is ready for dispatch.", SortKey: "40-ready"}
	case state == StateCompleted:
		return AttentionPriority{Level: AttentionNone, Reason: "Task is complete.", SortKey: "90-complete"}
	case state == StateCancelled:
		return AttentionPriority{Level: AttentionNone, Reason: "Task is cancelled.", SortKey: "91-cancelled"}
	default:
		return AttentionPriority{Level: AttentionWatch, Reason: "Task should remain visible for backend follow-up.", SortKey: "50-watch"}
	}
}

func isTerminalTaskState(status string) bool {
	return isCompletedTaskStatus(status) || isCancelledTaskStatus(status)
}

func isCompletedTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "complete", "completed", "done", "closed":
		return true
	default:
		return false
	}
}

func isCancelledTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func (s *Service) captureSnapshot(taskRoot string) (TaskDefinitionSnapshot, error) {
	paths := []string{
		filepath.Join(taskRoot, "TASK.md"),
		filepath.Join(taskRoot, "PLAN.md"),
		filepath.Join(taskRoot, "HANDOFF.md"),
		filepath.Join(taskRoot, "TASK-STATE.json"),
		filepath.Join(taskRoot, "CONSTRAINTS.md"),
	}

	digests := make([]TaskArtifactDigest, 0, len(paths))
	hash := sha256.New()
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return TaskDefinitionSnapshot{}, fmt.Errorf("read snapshot file %s: %w", path, err)
		}
		relativePath, err := filepath.Rel(taskRoot, path)
		if err != nil {
			return TaskDefinitionSnapshot{}, fmt.Errorf("rel snapshot path %s: %w", path, err)
		}
		fileHash := sha256.Sum256(raw)
		digests = append(digests, TaskArtifactDigest{
			RelativePath: filepath.ToSlash(relativePath),
			SHA256:       hex.EncodeToString(fileHash[:]),
		})
		hash.Write([]byte(filepath.ToSlash(relativePath)))
		hash.Write(fileHash[:])
	}

	return TaskDefinitionSnapshot{
		DeclaredWorktreeRoot: s.declaredWorktreeRoot,
		DeclaredTaskRoot:     taskRoot,
		DeclaredTaskRevision: hex.EncodeToString(hash.Sum(nil)),
		DeclaredGitRevision:  gitRevision(s.declaredWorktreeRoot),
		CapturedAt:           s.now(),
		Files:                digests,
	}, nil
}

func (s *Service) refreshRun(ctx context.Context, run TaskRunView) (TaskRunView, error) {
	if s.runtime == nil {
		return run, nil
	}
	update := s.derivedRunUpdate(run)
	if update == nil {
		return run, nil
	}
	return s.runtime.UpdateTaskRun(ctx, run.RunID, *update)
}

func (s *Service) derivedRunUpdate(run TaskRunView) *TaskRunUpdate {
	now := s.now()
	desiredActions := actionsForRun(run, now)
	desiredAttention := attentionForRun(run, now)
	desiredFollowUp := desiredFollowUp(run, now)

	var update TaskRunUpdate
	changed := false

	if staleUpdate, ok := staleRunUpdate(run, now, desiredActions, desiredAttention); ok {
		return &staleUpdate
	}

	if !reflect.DeepEqual(run.Actions, desiredActions) {
		update.Actions = desiredActions
		changed = true
	}
	if run.Attention != desiredAttention {
		update.Attention = &desiredAttention
		changed = true
	}
	if !reflect.DeepEqual(run.FollowUp, desiredFollowUp) {
		update.FollowUp = desiredFollowUp
		changed = true
	}
	if !changed {
		return nil
	}
	return &update
}

func (s *Service) provisionOwnedLane(taskID string) (RepoLane, error) {
	baselineCommit := gitRevision(s.declaredWorktreeRoot)
	if baselineCommit == "" {
		return RepoLane{}, fmt.Errorf("resolve baseline commit for %s", taskID)
	}

	if err := os.MkdirAll(s.ownedLaneRoot, 0o755); err != nil {
		return RepoLane{}, fmt.Errorf("create owned lane root: %w", err)
	}
	laneDir, err := os.MkdirTemp(s.ownedLaneRoot, shortTaskSegment(taskID)+"-")
	if err != nil {
		return RepoLane{}, fmt.Errorf("create owned lane temp dir: %w", err)
	}
	ownedRepoRoot := filepath.Join(laneDir, "w")

	args := []string{"-C", s.declaredWorktreeRoot}
	if runtime.GOOS == "windows" {
		args = append([]string{"-c", "core.longpaths=true"}, args...)
	}
	args = append(args, "worktree", "add", "--detach", ownedRepoRoot, baselineCommit)
	cmd := exec.Command("git", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return RepoLane{}, fmt.Errorf("create owned worktree: %w: %s", err, strings.TrimSpace(string(output)))
	}

	return RepoLane{
		OwnedRepoRoot:         ownedRepoRoot,
		CheckoutMode:          "git_worktree_detached",
		BaselineCommit:        baselineCommit,
		ApprovedRestoreCommit: baselineCommit,
		ResetStatus:           "not_run",
	}, nil
}

func (s *Service) releasePreviousOwnedLane(ctx context.Context, taskID string) error {
	if s.runtime == nil {
		return nil
	}
	previousRun, err := s.runtime.GetActiveTaskRun(ctx, s.runID(taskID))
	if err != nil {
		if errors.Is(err, ErrRunNotFound) {
			return nil
		}
		return err
	}
	if runOwnsLiveStory(previousRun) || previousRun.RepoLane.OwnedRepoRoot == "" {
		return nil
	}
	// Pool model (Task-0016): a superseded same-task run that occupied a POOL worktree
	// returns that worktree to idle (folder kept) for reuse, rather than deleting it. A
	// legacy non-pool (random-temp) lane still uses the delete path.
	isPoolMember, err := s.returnPoolMemberToIdle(previousRun.RepoLane.OwnedRepoRoot)
	if err != nil {
		return fmt.Errorf("release previous owned lane for %s: %w", taskID, err)
	}
	if isPoolMember {
		return nil
	}
	if err := s.cleanupOwnedLane(previousRun.RepoLane); err != nil {
		return fmt.Errorf("release previous owned lane for %s: %w", taskID, err)
	}
	return nil
}

func (s *Service) bootstrapOwnedLane(taskID string, runID string, snapshot TaskDefinitionSnapshot, repoLane RepoLane, dispatchContext *DeepContext) (RepoLane, error) {
	if repoLane.OwnedRepoRoot == "" {
		return RepoLane{}, fmt.Errorf("bootstrap owned lane for %s: owned repo root is missing", taskID)
	}

	currentCommit := gitRevision(repoLane.OwnedRepoRoot)
	if currentCommit == "" {
		return RepoLane{}, fmt.Errorf("bootstrap owned lane for %s: resolve current commit", taskID)
	}

	artifactRoot := filepath.Join(s.runArtifactsRoot, sanitizePathSegment(taskID), sanitizePathSegment(runID))
	if err := os.MkdirAll(artifactRoot, 0o755); err != nil {
		return RepoLane{}, fmt.Errorf("create task-run artifact root: %w", err)
	}

	binding := s.bindingForLane(taskID, repoLane.OwnedRepoRoot, dispatchContext)

	bootstrapPath := filepath.Join(artifactRoot, "owned-lane-bootstrap.json")
	record := ownedLaneBootstrapRecord{
		TaskID:               taskID,
		RunID:                runID,
		OwnedRepoRoot:        repoLane.OwnedRepoRoot,
		BaselineCommit:       repoLane.BaselineCommit,
		CurrentCommit:        currentCommit,
		DeclaredWorktreeRoot: snapshot.DeclaredWorktreeRoot,
		DeclaredTaskRoot:     snapshot.DeclaredTaskRoot,
		DeclaredTaskRevision: snapshot.DeclaredTaskRevision,
		DeclaredGitRevision:  snapshot.DeclaredGitRevision,
		CapturedAt:           snapshot.CapturedAt,
		BootstrappedAt:       s.now(),
		Files:                append([]TaskArtifactDigest(nil), snapshot.Files...),
		Binding:              binding,
	}
	if err := writeJSONFile(bootstrapPath, record); err != nil {
		return RepoLane{}, fmt.Errorf("write owned-lane bootstrap artifact: %w", err)
	}

	repoLane.CurrentCommit = currentCommit
	repoLane.RunArtifactRoot = artifactRoot
	repoLane.BootstrapArtifactPath = bootstrapPath
	repoLane.Binding = binding
	return repoLane, nil
}

// bindingForLane builds the O6 worktree<->session binding from what is genuinely
// available at dispatch: the task id, the owned worktree path, the repo id from
// the manifest, and the dispatch context's session id/transcript path. The
// session id/transcript path are the BACKEND dispatch process's values
// (best-available placeholders); real launched-agent session capture is PASS-0005.
// run/gate state defaults to RunGateStateRunning; the parked-needs-human enum is
// O4/PASS-0003. No values are invented — placeholder fields are left empty when
// the dispatch context does not carry them.
func (s *Service) bindingForLane(taskID string, ownedRepoRoot string, dispatchContext *DeepContext) *RepoBinding {
	binding := &RepoBinding{
		Repo:         s.repoIdentity(),
		TaskID:       taskID,
		WorktreePath: ownedRepoRoot,
		RunGateState: RunGateStateRunning,
	}
	if dispatchContext != nil {
		binding.AgentSessionID = dispatchContext.SessionID
		binding.SessionTranscriptPath = dispatchContext.TranscriptPath
	}
	return binding
}

// repoIdentity resolves the repo id for the declared worktree root from
// REPO-MANIFEST.json, falling back to the declared worktree root itself when the
// manifest is missing or has no matching entry (so the binding still names a
// stable repo identifier rather than an empty string).
func (s *Service) repoIdentity() string {
	if manifest, err := queue.LoadManifest(s.declaredWorktreeRoot); err == nil {
		if id := manifest.RepoIDForRoot(s.declaredWorktreeRoot); id != "" {
			return id
		}
	}
	return s.declaredWorktreeRoot
}

func (s *Service) cleanupOwnedLane(repoLane RepoLane) error {
	if repoLane.OwnedRepoRoot == "" {
		return nil
	}
	if !pathWithinRoot(repoLane.OwnedRepoRoot, s.ownedLaneRoot) {
		return fmt.Errorf("owned repo root %q is outside the backend-owned lane root", repoLane.OwnedRepoRoot)
	}
	return removeOwnedLaneWorktree(s.declaredWorktreeRoot, s.ownedLaneRoot, repoLane.OwnedRepoRoot)
}

// ReconcileOwnedLanes performs startup reconciliation of the irreducible on-disk facts
// for THIS Service's repo: it runs `git worktree prune` to clear administrative metadata
// for owned-lane worktrees whose checkout directory is already gone (a crashed or partial
// removal), complementing countOwnedLaneWorktrees' prunable-exclusion so a stale entry can
// never wedge slot accounting (the earlier "all per-repo slots are occupied" failure).
//
// It DELIBERATELY does NOT autonomously reclaim a worktree that still EXISTS on disk.
// Under the human-only-closure / park-in-place contract (HUMAN-DIRECTIVES O4: the agent
// NEVER self-closes), neither an absent/expired Temporal workflow NOR a terminal run
// status is sufficient evidence that a HUMAN approved closure — a lane parked awaiting
// closure can legitimately outlive its workflow history, and reclaiming it would be a
// self-close. The authoritative reclaim stays the consumer's GitHub-issue-closed path,
// the only signal that proves a human closed the work.
func (s *Service) ReconcileOwnedLanes() error {
	argv := []string{}
	if runtime.GOOS == "windows" {
		argv = append(argv, "-c", "core.longpaths=true")
	}
	argv = append(argv, "-C", s.declaredWorktreeRoot, "worktree", "prune")
	if output, err := exec.Command("git", argv...).CombinedOutput(); err != nil {
		return fmt.Errorf("prune owned-lane worktrees for %s: %w: %s", s.declaredWorktreeRoot, err, strings.TrimSpace(string(output)))
	}
	return nil
}

// removeOwnedLaneWorktree removes an owned-lane worktree idempotently and
// self-heals a residual checkout (BUG-0002). It first runs git worktree remove
// --force. On success it also best-effort removes any residual directory left by a
// Windows handle and returns nil. On failure (e.g. a partial removal already
// unregistered the worktree, so a retry reports "is not a working tree") it does
// NOT immediately fail: it prunes stale worktree metadata, then retries
// os.RemoveAll on the checkout to tolerate a closing Windows handle. It returns nil
// once the directory no longer exists on disk (the lane IS reclaimed), and only
// returns an error when the directory still exists after every attempt.
//
// declaredWorktreeRoot is the repo the worktree was added under (git -C target);
// ownedLaneRoot anchors the longpaths/prune invocation; ownedRepoRoot is the
// checkout to remove. The caller is responsible for the pathWithinRoot guard.
func removeOwnedLaneWorktree(declaredWorktreeRoot string, ownedLaneRoot string, ownedRepoRoot string) error {
	argv := []string{}
	if runtime.GOOS == "windows" {
		argv = append(argv, "-c", "core.longpaths=true")
	}
	argv = append(argv, "-C", declaredWorktreeRoot, "worktree", "remove", "--force", ownedRepoRoot)
	output, err := exec.Command("git", argv...).CombinedOutput()
	if err == nil {
		// Clear any residual the forced removal left behind (best-effort).
		_ = os.RemoveAll(ownedRepoRoot)
		return nil
	}
	gitErr := fmt.Errorf("remove owned worktree: %w: %s", err, strings.TrimSpace(string(output)))

	// Self-heal: prune stale worktree metadata, then retry RemoveAll to tolerate a
	// Windows handle that is still closing. The directory becoming absent means the
	// lane is reclaimed even though the forced removal reported an error.
	pruneArgv := []string{}
	if runtime.GOOS == "windows" {
		pruneArgv = append(pruneArgv, "-c", "core.longpaths=true")
	}
	pruneArgv = append(pruneArgv, "-C", declaredWorktreeRoot, "worktree", "prune")
	_ = exec.Command("git", pruneArgv...).Run()

	var removeAllErr error
	for attempt := 0; attempt < 5; attempt++ {
		removeAllErr = os.RemoveAll(ownedRepoRoot)
		if _, statErr := os.Stat(ownedRepoRoot); os.IsNotExist(statErr) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	if _, statErr := os.Stat(ownedRepoRoot); os.IsNotExist(statErr) {
		return nil
	}
	return fmt.Errorf("%w; residual checkout remained after prune+removeall: %v", gitErr, removeAllErr)
}

// restoreOwnedLane resets the owned checkout to baseline with `git clean -fd` — the
// Assign/dispatch reset (preserves ignored files). Eject uses restoreOwnedLaneFull
// (`-fdx`) for a true baseline.
func (s *Service) restoreOwnedLane(repoLane RepoLane) (RepoLane, error) {
	return s.restoreOwnedLaneWithClean(repoLane, "-fd")
}

// restoreOwnedLaneFull resets the owned checkout to baseline with `git clean -fdx`,
// dropping ignored files too for a TRUE baseline (Eject's "give me a clean slot back").
func (s *Service) restoreOwnedLaneFull(repoLane RepoLane) (RepoLane, error) {
	return s.restoreOwnedLaneWithClean(repoLane, "-fdx")
}

// restoreOwnedLaneWithClean resets the owned checkout to baseline (reset --hard) then runs
// `git clean <cleanFlags>`. cleanFlags is "-fd" for the Assign/dispatch reset and "-fdx"
// for an Eject true-baseline; nothing else differs, so Assign's reset is never weakened.
func (s *Service) restoreOwnedLaneWithClean(repoLane RepoLane, cleanFlags string) (RepoLane, error) {
	now := s.now()
	repoLane.LastResetAt = now
	restoreCommit := repoLane.ApprovedRestoreCommit
	if restoreCommit == "" {
		restoreCommit = repoLane.BaselineCommit
	}
	repoLane.LastResetTargetCommit = restoreCommit
	repoLane.ResetFailureSummary = ""
	if repoLane.OwnedRepoRoot == "" {
		repoLane.ResetStatus = "cleanup_blocked"
		repoLane.ResetFailureSummary = "Owned repo root is missing."
		return repoLane, fmt.Errorf("owned repo root is missing")
	}
	if restoreCommit == "" {
		repoLane.ResetStatus = "cleanup_blocked"
		repoLane.ResetFailureSummary = "Restore commit is missing."
		return repoLane, fmt.Errorf("restore commit is missing")
	}
	if !pathWithinRoot(repoLane.OwnedRepoRoot, s.ownedLaneRoot) {
		repoLane.ResetStatus = "cleanup_blocked"
		repoLane.ResetFailureSummary = fmt.Sprintf("Owned repo root %q is outside the backend-owned lane root.", repoLane.OwnedRepoRoot)
		return repoLane, fmt.Errorf("owned repo root %q is outside the backend-owned lane root", repoLane.OwnedRepoRoot)
	}
	if err := gitInWorktree(repoLane.OwnedRepoRoot, "reset", "--hard", restoreCommit); err != nil {
		repoLane.ResetStatus = "cleanup_blocked"
		repoLane.ResetFailureSummary = fmt.Sprintf("Reset to %s failed.", restoreCommit)
		return repoLane, fmt.Errorf("reset owned lane to %s: %w", restoreCommit, err)
	}
	if err := gitInWorktree(repoLane.OwnedRepoRoot, "clean", cleanFlags); err != nil {
		repoLane.ResetStatus = "cleanup_blocked"
		repoLane.ResetFailureSummary = "Git clean failed while restoring the owned checkout."
		return repoLane, fmt.Errorf("clean owned lane: %w", err)
	}
	repoLane.ResetStatus = "restored"
	repoLane.ApprovedRestoreCommit = restoreCommit
	repoLane.ResetFailureSummary = ""
	return repoLane, nil
}

func defaultActions(readiness DispatchReadiness) map[string]ActionAvailability {
	return map[string]ActionAvailability{
		ActionDispatch: {
			Allowed:      readiness.Ready,
			BlockReasons: append([]ActionBlockReason(nil), readiness.BlockReasons...),
		},
		ActionPoke: {
			Allowed: false,
			BlockReasons: []ActionBlockReason{{
				Code:    "no_active_run",
				Summary: "Poke is unavailable until a task run exists.",
			}},
		},
		ActionInterrupt: {
			Allowed: false,
			BlockReasons: []ActionBlockReason{{
				Code:    "no_active_run",
				Summary: "Interrupt is unavailable until a task run exists.",
			}},
		},
	}
}

func actionsForRunState(state string) map[string]ActionAvailability {
	dispatchBlocked := []ActionBlockReason{{
		Code:    "active_run_exists",
		Summary: "Dispatch is blocked while this run owns the current live story.",
	}}
	pokeUnavailable := []ActionBlockReason{{Code: "poke_not_allowed_for_state", Summary: "Poke is not allowed in the current run state."}}
	interruptAllowed := ActionAvailability{Allowed: true}
	interruptUnavailable := []ActionBlockReason{{Code: "interrupt_not_allowed_for_state", Summary: "Interrupt is not allowed in the current run state."}}

	switch state {
	case StateRunning, StateDispatching:
		return map[string]ActionAvailability{
			ActionDispatch:  {Allowed: false, BlockReasons: dispatchBlocked},
			ActionPoke:      {Allowed: false, BlockReasons: pokeUnavailable},
			ActionInterrupt: interruptAllowed,
		}
	case StateSleepingOrStalled:
		return map[string]ActionAvailability{
			ActionDispatch:  {Allowed: false, BlockReasons: dispatchBlocked},
			ActionPoke:      {Allowed: true},
			ActionInterrupt: interruptAllowed,
		}
	case StateWaitingForHuman, StateBlocked:
		return map[string]ActionAvailability{
			ActionDispatch:  {Allowed: false, BlockReasons: dispatchBlocked},
			ActionPoke:      {Allowed: false, BlockReasons: pokeUnavailable},
			ActionInterrupt: interruptAllowed,
		}
	case StateCompleted, StateCancelled, StateFailed, StateInterrupted:
		return map[string]ActionAvailability{
			ActionDispatch: {Allowed: false, BlockReasons: dispatchBlocked},
			ActionPoke: {Allowed: false, BlockReasons: []ActionBlockReason{{
				Code:    "run_terminal",
				Summary: "Poke is not allowed after the run has already ended.",
			}}},
			ActionInterrupt: {Allowed: false, BlockReasons: []ActionBlockReason{{
				Code:    "run_terminal",
				Summary: "Interrupt is not allowed after the run has already ended.",
			}}},
		}
	default:
		return map[string]ActionAvailability{
			ActionDispatch:  {Allowed: false, BlockReasons: dispatchBlocked},
			ActionPoke:      {Allowed: false, BlockReasons: pokeUnavailable},
			ActionInterrupt: {Allowed: false, BlockReasons: interruptUnavailable},
		}
	}
}

func actionsForRun(run TaskRunView, now time.Time) map[string]ActionAvailability {
	actions := actionsForRunState(run.StateEnvelope.State)
	if run.FollowUp != nil && run.FollowUp.Status == "pending" && run.FollowUp.Kind == "poke_worker_check" {
		actions[ActionPoke] = ActionAvailability{
			Allowed: false,
			BlockReasons: []ActionBlockReason{{
				Code:    "follow_up_pending",
				Summary: "Poke is already waiting on a backend-worker follow-up.",
			}},
		}
	}
	if run.StateEnvelope.State == StateRunning || run.StateEnvelope.State == StateDispatching {
		if !run.StateEnvelope.SuspiciousAfter.IsZero() && now.After(run.StateEnvelope.SuspiciousAfter) {
			actions[ActionPoke] = ActionAvailability{Allowed: true}
		} else {
			actions[ActionPoke] = ActionAvailability{
				Allowed: false,
				BlockReasons: []ActionBlockReason{{
					Code:    "run_not_suspicious_yet",
					Summary: "Poke stays blocked until the run misses its next expected progress deadline.",
				}},
			}
		}
	}
	if run.StateEnvelope.State == StateWaitingForHuman && run.WaitContract != nil && !run.WaitContract.StaleAfter.IsZero() && now.After(run.WaitContract.StaleAfter) {
		actions[ActionPoke] = ActionAvailability{
			Allowed: false,
			BlockReasons: []ActionBlockReason{{
				Code:    "waiting_for_human_stale",
				Summary: "Poke does not replace the required human action on a stale human wait.",
			}},
		}
	}
	return actions
}

func attentionForRunState(state string) AttentionPriority {
	switch state {
	case StateWaitingForHuman:
		return AttentionPriority{Level: AttentionNeedsAttention, Reason: "Run is waiting on a human action.", SortKey: "20-waiting_for_human"}
	case StateBlocked:
		return AttentionPriority{Level: AttentionNeedsAttention, Reason: "Run is blocked and needs review.", SortKey: "30-blocked"}
	case StateSleepingOrStalled:
		return AttentionPriority{Level: AttentionUrgent, Reason: "Run appears stalled.", SortKey: "10-stalled"}
	case StateCompleted:
		return AttentionPriority{Level: AttentionNone, Reason: "Run is complete.", SortKey: "90-complete"}
	case StateFailed:
		return AttentionPriority{Level: AttentionUrgent, Reason: "Run failed.", SortKey: "15-failed"}
	case StateInterrupted:
		return AttentionPriority{Level: AttentionWatch, Reason: "Run was interrupted.", SortKey: "60-interrupted"}
	default:
		return AttentionPriority{Level: AttentionWatch, Reason: "Run is active.", SortKey: "50-active"}
	}
}

func attentionForRun(run TaskRunView, now time.Time) AttentionPriority {
	if run.FollowUp != nil && run.FollowUp.Status == "overdue" {
		switch run.FollowUp.Owner {
		case "backend_worker":
			return AttentionPriority{Level: AttentionUrgent, Reason: "A backend-worker follow-up is overdue.", SortKey: "11-follow_up_overdue"}
		default:
			return AttentionPriority{Level: AttentionUrgent, Reason: "A required follow-up is overdue.", SortKey: "13-follow_up_overdue"}
		}
	}
	if run.StateEnvelope.State == StateWaitingForHuman && run.WaitContract != nil && !run.WaitContract.StaleAfter.IsZero() && now.After(run.WaitContract.StaleAfter) {
		return AttentionPriority{Level: AttentionUrgent, Reason: "Run is still waiting on a human action past its stale deadline.", SortKey: "12-waiting_stale"}
	}
	if hasPendingInterruptReview(run) {
		return AttentionPriority{Level: AttentionNeedsAttention, Reason: "Interrupted run is still waiting on review resolution.", SortKey: "25-interrupt_review_pending"}
	}
	if run.StateEnvelope.State == StateInterrupted && run.Resolution != nil {
		return AttentionPriority{Level: AttentionNone, Reason: "Interrupted run review is resolved.", SortKey: "85-interrupt_review_resolved"}
	}
	return attentionForRunState(run.StateEnvelope.State)
}

func desiredFollowUp(run TaskRunView, now time.Time) *RunFollowUp {
	if run.FollowUp == nil {
		return nil
	}
	followUp := *run.FollowUp
	if followUp.Status == "pending" && !followUp.DueAt.IsZero() && now.After(followUp.DueAt) {
		followUp.Status = "overdue"
		return &followUp
	}
	return run.FollowUp
}

func hasPendingInterruptReview(run TaskRunView) bool {
	return run.StateEnvelope.State == StateInterrupted &&
		run.FollowUp != nil &&
		run.FollowUp.Kind == "interrupt_review" &&
		(run.FollowUp.Status == "pending" || run.FollowUp.Status == "overdue")
}

func staleRunUpdate(run TaskRunView, now time.Time, actions map[string]ActionAvailability, attention AttentionPriority) (TaskRunUpdate, bool) {
	if (run.StateEnvelope.State == StateRunning || run.StateEnvelope.State == StateDispatching) &&
		!run.StateEnvelope.SuspiciousAfter.IsZero() &&
		now.After(run.StateEnvelope.SuspiciousAfter) {
		return TaskRunUpdate{
			State:               StateSleepingOrStalled,
			ReasonCode:          "progress_stale",
			StateSummary:        "Run has gone quiet past its expected progress window.",
			NextOwner:           "backend",
			NextExpectedEvent:   "Poke or interrupt the run.",
			SuspiciousAfter:     run.StateEnvelope.SuspiciousAfter,
			LastProgressSummary: "Supervision marked the run as sleeping or stalled.",
			Attention:           &attention,
			Actions:             actionsForRunState(StateSleepingOrStalled),
		}, true
	}
	if run.StateEnvelope.State == StateWaitingForHuman &&
		run.WaitContract != nil &&
		!run.WaitContract.StaleAfter.IsZero() &&
		now.After(run.WaitContract.StaleAfter) &&
		run.StateEnvelope.ReasonCode != "human_wait_stale" {
		return TaskRunUpdate{
			State:               StateWaitingForHuman,
			ReasonCode:          "human_wait_stale",
			StateSummary:        "Run is still waiting for human input and the wait has gone stale.",
			NextOwner:           "human_or_supervisor",
			NextExpectedEvent:   "Review the stale human wait or interrupt the run.",
			SuspiciousAfter:     run.WaitContract.StaleAfter,
			LastProgressSummary: "Supervision marked the human wait as stale.",
			Attention:           &attention,
			Actions:             actions,
		}, true
	}
	return TaskRunUpdate{}, false
}

func projectRun(current TaskRunView, update TaskRunUpdate, now time.Time) TaskRunView {
	projected := current
	if update.State != "" {
		projected.StateEnvelope.State = update.State
	}
	if update.ReasonCode != "" {
		projected.StateEnvelope.ReasonCode = update.ReasonCode
	}
	if update.StateSummary != "" {
		projected.StateEnvelope.StateSummary = update.StateSummary
	}
	if update.NextOwner != "" {
		projected.StateEnvelope.NextOwner = update.NextOwner
	}
	if update.NextExpectedEvent != "" {
		projected.StateEnvelope.NextExpectedEvent = update.NextExpectedEvent
	}
	if !update.SuspiciousAfter.IsZero() {
		projected.StateEnvelope.SuspiciousAfter = update.SuspiciousAfter
	}
	if update.WaitContract != nil {
		projected.WaitContract = update.WaitContract
	} else if update.State != "" && update.State != StateWaitingForHuman {
		projected.WaitContract = nil
	}
	if update.RepoLane != nil {
		projected.RepoLane = *update.RepoLane
	}
	if update.FollowUp != nil {
		if isEmptyRunFollowUp(update.FollowUp) {
			projected.FollowUp = nil
		} else {
			projected.FollowUp = update.FollowUp
		}
	}
	if update.Resolution != nil {
		projected.Resolution = update.Resolution
	}
	if update.LastProgressSummary != "" {
		projected.LastProgressSummary = update.LastProgressSummary
		projected.LastProgressAt = now
	}
	if update.FailureSummary != "" {
		projected.FailureSummary = update.FailureSummary
	} else if update.State != "" && update.State != StateBlocked && update.State != StateFailed {
		projected.FailureSummary = ""
	}
	if !update.CompletedAt.IsZero() {
		projected.LastProgressAt = update.CompletedAt
	}
	switch projected.StateEnvelope.State {
	case StateCompleted:
		projected.Status = "completed"
	case StateFailed:
		projected.Status = "failed"
	case StateInterrupted:
		projected.Status = "interrupted"
	default:
		projected.Status = "active"
	}
	return projected
}

func derivedFollowUp(current TaskRunView, update TaskRunUpdate, now time.Time) *RunFollowUp {
	if update.FollowUp != nil {
		return update.FollowUp
	}
	effectiveState := current.StateEnvelope.State
	if update.State != "" {
		effectiveState = update.State
	}
	effectiveReason := current.StateEnvelope.ReasonCode
	if update.ReasonCode != "" {
		effectiveReason = update.ReasonCode
	}
	if effectiveState == StateBlocked && effectiveReason == "workload_execution_failed" && current.FollowUp == nil {
		return &RunFollowUp{
			Kind:        "workload_recovery",
			Owner:       "human_or_supervisor",
			Status:      "pending",
			Summary:     "Retry the workload with a fresh owned lane or inspect the failure artifacts before retrying.",
			RequestedAt: now,
			DueAt:       now.Add(24 * time.Hour),
		}
	}
	if current.FollowUp == nil {
		return nil
	}
	if current.FollowUp.Kind == "workload_recovery" && (effectiveState != StateBlocked || effectiveReason != "workload_execution_failed") {
		return &RunFollowUp{}
	}
	if current.FollowUp.Owner == "backend_worker" && current.FollowUp.Status != "completed" {
		if update.LastProgressSummary != "" && update.ReasonCode != "poke_requested" && effectiveState != StateSleepingOrStalled {
			completed := *current.FollowUp
			completed.Status = "completed"
			completed.CompletedAt = now
			completed.Summary = "Backend worker follow-up completed with a fresh runtime update."
			return &completed
		}
	}
	return current.FollowUp
}

func isEmptyRunFollowUp(followUp *RunFollowUp) bool {
	return followUp != nil &&
		followUp.Kind == "" &&
		followUp.Owner == "" &&
		followUp.Status == "" &&
		followUp.Summary == "" &&
		followUp.RequestedAt.IsZero() &&
		followUp.DueAt.IsZero() &&
		followUp.CompletedAt.IsZero()
}

func collectActionBlockReasons(actions map[string]ActionAvailability) map[string][]ActionBlockReason {
	blockReasons := map[string][]ActionBlockReason{}
	for action, availability := range actions {
		blockReasons[action] = append([]ActionBlockReason(nil), availability.BlockReasons...)
	}
	return blockReasons
}

func summarizeBlockReasons(reasons []ActionBlockReason) string {
	if len(reasons) == 0 {
		return "unknown reason"
	}
	summaries := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		summaries = append(summaries, reason.Summary)
	}
	return strings.Join(summaries, "; ")
}

// ActiveRunID is the historical (repo-unaware) active run id for a task. It is the
// empty-namespace case of ActiveRunIDForRepo and is retained as a shim so the
// single-repo control plane and existing call sites/tests stay byte-identical.
func ActiveRunID(taskID string) string {
	return ActiveRunIDForRepo("", taskID)
}

// ActiveRunIDForRepo builds the active Temporal run id (workflow id) for a task,
// namespaced by repo so the same issue number in two different registry repos does
// not collide on the workflow id (BUG-0003). An empty repoNamespace returns the
// historical id verbatim ("taskrun--<taskID>--active"); a non-empty namespace yields
// "taskrun--<repo>--<taskID>--active". The result is also the per-run runs-root
// segment, so the same namespacing separates the on-disk artifact paths.
func ActiveRunIDForRepo(repoNamespace string, taskID string) string {
	if repoNamespace == "" {
		return "taskrun--" + sanitizePathSegment(taskID) + "--active"
	}
	return "taskrun--" + sanitizePathSegment(repoNamespace) + "--" + sanitizePathSegment(taskID) + "--active"
}

// runID is the Service's single construction path for a task's active run id,
// applying this Service's repoNamespace. Every Service-internal start/read of a run
// goes through this so dispatch and lookup never diverge on the id.
func (s *Service) runID(taskID string) string {
	return ActiveRunIDForRepo(s.repoNamespace, taskID)
}

func taskRootForID(trackingRoot string, taskID string) string {
	return filepath.Join(trackingRoot, taskID)
}

func extractMarkdownSection(markdown string, heading string) string {
	lines := strings.Split(markdown, "\n")
	header := "## " + heading
	capture := false
	section := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == header {
			capture = true
			continue
		}
		if capture && strings.HasPrefix(trimmed, "## ") {
			break
		}
		if capture {
			section = append(section, line)
		}
	}
	return strings.TrimSpace(strings.Join(section, "\n"))
}

func firstParagraph(section string) string {
	lines := strings.Split(section, "\n")
	paragraph := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(paragraph) > 0 {
				break
			}
			continue
		}
		paragraph = append(paragraph, trimmed)
	}
	return strings.Join(paragraph, " ")
}

func taskArtifactRef(label string, path string) EvidenceRef {
	return EvidenceRef{
		Type:  "task_artifact",
		Label: label,
		URI:   fileURI(path),
	}
}

func fileURI(path string) string {
	value := filepath.ToSlash(path)
	return (&url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(value, "/")}).String()
}

func apiResourceURI(path string) string {
	return "api://" + strings.TrimPrefix(path, "/")
}

func captureDispatchContext() *DeepContext {
	sessionID := firstNonEmptyEnv("CODEX_SESSION_ID", "CODEX_THREAD_ID", "CODEX_CONVERSATION_ID")
	transcriptPath := firstNonEmptyEnv("CODEX_TRANSCRIPT_PATH", "CODEX_SESSION_TRANSCRIPT_PATH")
	if sessionID == "" && transcriptPath == "" {
		return nil
	}
	return &DeepContext{
		SessionID:      sessionID,
		TranscriptPath: transcriptPath,
	}
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func taskDeepContext(metadata parsedTask) *DeepContext {
	taskPath := filepath.Join(metadata.taskRoot, "TASK.md")
	targets := []LaunchTarget{{
		Kind:      "task_artifact",
		Label:     "Task",
		URI:       fileURI(taskPath),
		Command:   []string{"code", taskPath},
		Preferred: true,
	}}
	if hasSnapshotFile(metadata.snapshot, "HANDOFF.md") {
		handoffPath := filepath.Join(metadata.taskRoot, "HANDOFF.md")
		targets = append(targets, LaunchTarget{
			Kind:    "task_artifact",
			Label:   "Task handoff",
			URI:     fileURI(handoffPath),
			Command: []string{"code", handoffPath},
		})
	}
	if hasSnapshotFile(metadata.snapshot, "PLAN.md") {
		planPath := filepath.Join(metadata.taskRoot, "PLAN.md")
		targets = append(targets, LaunchTarget{
			Kind:    "task_artifact",
			Label:   "Task plan",
			URI:     fileURI(planPath),
			Command: []string{"code", planPath},
		})
	}
	preferred := targets[0]
	return &DeepContext{
		PreferredLaunchTarget: &preferred,
		LaunchTargets:         targets,
	}
}

func runDeepContext(run TaskRunView) *DeepContext {
	base := &DeepContext{}
	if run.DeepContext != nil {
		base.SessionID = run.DeepContext.SessionID
		base.TranscriptPath = run.DeepContext.TranscriptPath
	}
	targets := make([]LaunchTarget, 0, 6)
	if base.TranscriptPath != "" {
		targets = append(targets, LaunchTarget{
			Kind:      "transcript",
			Label:     "Session transcript",
			URI:       fileURI(base.TranscriptPath),
			Command:   []string{"code", base.TranscriptPath},
			Preferred: true,
		})
	}
	if run.CapturedTaskSnapshot.DeclaredTaskRoot != "" {
		taskPath := filepath.Join(run.CapturedTaskSnapshot.DeclaredTaskRoot, "TASK.md")
		if hasSnapshotFile(run.CapturedTaskSnapshot, "TASK.md") {
			targets = append(targets, LaunchTarget{
				Kind:      "task_artifact",
				Label:     "Task",
				URI:       fileURI(taskPath),
				Command:   []string{"code", taskPath},
				Preferred: len(targets) == 0,
			})
		} else {
			targets = append(targets, LaunchTarget{
				Kind:      "task_artifact",
				Label:     "Task folder",
				URI:       fileURI(run.CapturedTaskSnapshot.DeclaredTaskRoot),
				Command:   []string{"code", run.CapturedTaskSnapshot.DeclaredTaskRoot},
				Preferred: len(targets) == 0,
			})
		}
		if hasSnapshotFile(run.CapturedTaskSnapshot, "HANDOFF.md") {
			handoffPath := filepath.Join(run.CapturedTaskSnapshot.DeclaredTaskRoot, "HANDOFF.md")
			targets = append(targets, LaunchTarget{
				Kind:    "task_artifact",
				Label:   "Task handoff",
				URI:     fileURI(handoffPath),
				Command: []string{"code", handoffPath},
			})
		}
		if hasSnapshotFile(run.CapturedTaskSnapshot, "PLAN.md") {
			planPath := filepath.Join(run.CapturedTaskSnapshot.DeclaredTaskRoot, "PLAN.md")
			targets = append(targets, LaunchTarget{
				Kind:    "task_artifact",
				Label:   "Task plan",
				URI:     fileURI(planPath),
				Command: []string{"code", planPath},
			})
		}
	}
	if run.RepoLane.OwnedRepoRoot != "" {
		targets = append(targets, LaunchTarget{
			Kind:      "owned_checkout",
			Label:     "Owned checkout",
			URI:       fileURI(run.RepoLane.OwnedRepoRoot),
			Command:   []string{"code", run.RepoLane.OwnedRepoRoot},
			Preferred: len(targets) == 0,
		})
	}
	if run.RepoLane.RunArtifactRoot != "" {
		targets = append(targets, LaunchTarget{
			Kind:    "run_artifact",
			Label:   "Run artifacts",
			URI:     fileURI(run.RepoLane.RunArtifactRoot),
			Command: []string{"code", run.RepoLane.RunArtifactRoot},
		})
	}
	if run.RunID != "" {
		targets = append(targets, LaunchTarget{
			Kind:  "api_resource",
			Label: "Active run API resource",
			URI:   apiResourceURI("/api/v1/task-runs/" + run.RunID),
		})
	}
	if len(targets) == 0 && base.SessionID == "" && base.TranscriptPath == "" {
		return nil
	}
	preferredIndex := 0
	for i := range targets {
		if targets[i].Preferred {
			preferredIndex = i
			break
		}
	}
	if len(targets) > 0 {
		targets[preferredIndex].Preferred = true
		preferred := targets[preferredIndex]
		base.PreferredLaunchTarget = &preferred
	}
	base.LaunchTargets = targets
	return base
}

func hasSnapshotFile(snapshot TaskDefinitionSnapshot, relativePath string) bool {
	for _, file := range snapshot.Files {
		if filepath.ToSlash(file.RelativePath) == filepath.ToSlash(relativePath) {
			return true
		}
	}
	return false
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func gitRevision(worktreeRoot string) string {
	out, err := exec.Command("git", "-C", worktreeRoot, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func sanitizePathSegment(value string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", " ", "_")
	return replacer.Replace(value)
}

func defaultOwnedLaneRoot(runsRoot string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.TempDir(), "cdxow")
	}
	return filepath.Join(runsRoot, "task-owned-checkouts")
}

func shortTaskSegment(taskID string) string {
	hash := sha256.Sum256([]byte(taskID))
	return sanitizePathSegment(taskID) + "-" + hex.EncodeToString(hash[:4])
}

func runOwnsLiveStory(run TaskRunView) bool {
	return run.Status != "completed" && run.Status != "failed" && run.Status != "interrupted"
}

// sameRepoRoot reports whether two declared worktree roots name the same repo
// checkout, normalized to absolute + Clean and case-insensitive on Windows. It
// repo-scopes owned-lane accounting (BUG-0003) so one registry repo's consumer never
// counts/resolves another repo's lane that happens to share a Task-NNNN id.
func sameRepoRoot(a string, b string) bool {
	na, err := filepath.Abs(a)
	if err != nil {
		return false
	}
	nb, err := filepath.Abs(b)
	if err != nil {
		return false
	}
	na = filepath.Clean(na)
	nb = filepath.Clean(nb)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(na, nb)
	}
	return na == nb
}

func pathWithinRoot(path string, root string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func gitInWorktree(worktreeRoot string, args ...string) error {
	argv := []string{}
	if runtime.GOOS == "windows" {
		argv = append(argv, "-c", "core.longpaths=true")
	}
	argv = append(argv, "-C", worktreeRoot)
	argv = append(argv, args...)
	cmd := exec.Command("git", argv...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
