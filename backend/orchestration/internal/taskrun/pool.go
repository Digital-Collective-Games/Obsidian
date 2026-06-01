package taskrun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/queue"
)

// Manual persistent worktree pool (Task-0016). The pool replaces the per-dispatch
// random-temp model (provisionOwnedLane's os.MkdirTemp dirs) with operator-managed,
// persistent worktrees that have STABLE paths and STABLE ids:
//
//	worktree_path = <ownedLaneRoot>/<repoSegment>/wt-<NNNN>/w
//	worktree_id   = <repoSegment>/wt-<NNNN>
//
// Each pool folder carries a durable poolRecord (worktree-pool.json) sibling to its
// `w` checkout, so an IDLE member (run_id == "") survives on disk with no run bound —
// unlike the owned-lane-bootstrap.json breadcrumb, which only exists once a run has
// bootstrapped the lane. Concurrency is bounded by the count of idle pool members by
// construction; there is no separate numeric cap.

// poolRecordFileName is the durable pool-membership record written next to each pool
// worktree's `w` checkout. It persists the four mandatory fields (worktree_id, stable
// worktree_path, repo, run_id-or-empty) so discover-on-startup can reconstruct the
// pool — including idle members — across a backend restart.
const poolRecordFileName = "worktree-pool.json"

// (the checkout subdir is named after the member, e.g. wt-0001, so each pool worktree's
// git worktree admin name is UNIQUE per repo — a shared `w` leaf would collide in
// .git/worktrees across members.)

// poolMemberPrefix names a pool member folder: wt-<NNNN>.
const poolMemberPrefix = "wt-"

// poolMemberIDWidth is the zero-pad width of the <NNNN> sequence in a pool member
// folder/id. It is an implementation detail (not a scope decision): 4 digits.
const poolMemberIDWidth = 4

// poolStatusIdle / poolStatusAllocated are the pool member status discriminators
// surfaced by the full-pool read. Idle = no run bound (run_id == ""); allocated =
// bound to a live run.
const (
	poolStatusIdle      = "idle"
	poolStatusAllocated = "allocated"
)

// poolRecord is the durable per-folder pool-membership record. RunID is empty for an
// idle member; a non-empty RunID marks the member allocated to that run.
type poolRecord struct {
	WorktreeID   string `json:"worktree_id"`
	WorktreePath string `json:"worktree_path"`
	Repo         string `json:"repo"`
	RunID        string `json:"run_id"`
}

// PoolWorktree is one full-pool entry returned by ListPoolWorktrees / the
// GET /api/v1/worktrees full-pool read. It carries the stable identity + status for
// every member (idle and allocated). The binding fields are FLATTENED (embedded
// RepoBinding) to match the §8 response shape: repo + worktree_path are always present;
// allocated members additionally carry the live task/run/gate/session read from the
// per-run workflow. An idle member leaves task_id/session/run_id empty and run_gate_state
// unset.
type PoolWorktree struct {
	WorktreeID string `json:"worktree_id"`
	Status     string `json:"status"`
	// RunID is the active run id for an allocated member; empty for an idle member.
	RunID string `json:"run_id,omitempty"`
	// RepoBinding is embedded (flattened) so repo, worktree_path, task_id,
	// agent_session_id, session_transcript_path, run_gate_state, and launched_pid appear
	// at the top level of each entry (§8 shape).
	RepoBinding
}

// poolRepoSegment is the stable, filesystem-safe, SHORT per-repo directory segment under
// the owned-lane root that anchors a repo's pool. It prefers this Service's repoNamespace
// (the registry repo id set by NewServiceForRepo). With no namespace it derives a stable
// segment from repoIdentity(): a manifest-resolved short id is used as-is, but a raw
// declared-root path (the no-manifest fallback) is hashed to a short, fixed-length
// segment so the pool path never blows past the OS path/`$GIT_DIR` limits.
func (s *Service) poolRepoSegment() string {
	if s.repoNamespace != "" {
		return sanitizePathSegment(s.repoNamespace)
	}
	id := s.repoIdentity()
	// repoIdentity falls back to the full declared worktree root when no manifest entry
	// matches; that path is too long to use as a directory segment. Use a short stable
	// hash of it instead. A manifest-resolved id (not equal to the raw root) is short and
	// used directly.
	if id == s.declaredWorktreeRoot {
		sum := sha256.Sum256([]byte(filepath.Clean(s.declaredWorktreeRoot)))
		return "repo-" + hex.EncodeToString(sum[:6])
	}
	return sanitizePathSegment(id)
}

// poolRepoRoot is the directory that holds all of this repo's pool member folders:
// <ownedLaneRoot>/<repoSegment>.
func (s *Service) poolRepoRoot() string {
	return filepath.Join(s.ownedLaneRoot, s.poolRepoSegment())
}

// poolWorktreeID builds the stable worktree id for a member sequence number:
// <repoSegment>/wt-<NNNN>. It is path-like but uses a forward slash so it reads the
// same on every platform and never collides with a filesystem separator in the id.
func (s *Service) poolWorktreeID(seq int) string {
	return s.poolRepoSegment() + "/" + poolMemberFolder(seq)
}

// poolMemberFolder is the wt-<NNNN> folder/id leaf for a sequence number.
func poolMemberFolder(seq int) string {
	return fmt.Sprintf("%s%0*d", poolMemberPrefix, poolMemberIDWidth, seq)
}

// poolMemberDir is the absolute folder for a member sequence number (holds the pool
// record + the `w` checkout): <ownedLaneRoot>/<repoSegment>/wt-<NNNN>.
func (s *Service) poolMemberDir(seq int) string {
	return filepath.Join(s.poolRepoRoot(), poolMemberFolder(seq))
}

// poolCheckoutPath is the stable checkout path for a member sequence number — the stable
// worktree_path (replacing provisionOwnedLane's random os.MkdirTemp dir). The checkout
// leaf is the member folder name (wt-<NNNN>) so the git worktree admin name is unique per
// repo; the durable pool record sits in the parent member folder (sibling of the
// checkout, so reset/clean never wipes it).
func (s *Service) poolCheckoutPath(seq int) string {
	return filepath.Join(s.poolMemberDir(seq), poolMemberFolder(seq))
}

// poolRecordPath is the worktree-pool.json path for a member sequence number.
func (s *Service) poolRecordPath(seq int) string {
	return filepath.Join(s.poolMemberDir(seq), poolRecordFileName)
}

// poolMemberDirForID resolves the absolute member folder for a worktree id of the
// form <repoSegment>/wt-<NNNN>. It returns an error for an id that does not belong to
// this Service's repo segment or is not a wt-<NNNN> leaf, so a caller can never act on
// another repo's pool member or an arbitrary path.
func (s *Service) poolMemberDirForID(worktreeID string) (string, error) {
	seg, leaf, ok := strings.Cut(worktreeID, "/")
	if !ok || leaf == "" {
		return "", fmt.Errorf("worktree id %q is not in <repo>/wt-<NNNN> form", worktreeID)
	}
	if seg != s.poolRepoSegment() {
		return "", fmt.Errorf("worktree id %q does not belong to repo %q", worktreeID, s.poolRepoSegment())
	}
	if _, err := poolMemberSeq(leaf); err != nil {
		return "", err
	}
	return filepath.Join(s.poolRepoRoot(), leaf), nil
}

// poolMemberSeq parses the <NNNN> sequence number from a wt-<NNNN> folder leaf. It
// errors for any leaf that is not a pool member folder, so enumeration ignores stray
// dirs and id resolution rejects malformed ids.
func poolMemberSeq(leaf string) (int, error) {
	rest, ok := strings.CutPrefix(leaf, poolMemberPrefix)
	if !ok || rest == "" {
		return 0, fmt.Errorf("folder %q is not a wt-<NNNN> pool member", leaf)
	}
	seq, err := strconv.Atoi(rest)
	if err != nil {
		return 0, fmt.Errorf("folder %q has a non-numeric sequence: %w", leaf, err)
	}
	return seq, nil
}

// nextPoolMemberSeq returns the next free wt-<NNNN> sequence number for this repo's
// pool, scanning existing member folders for the highest in-use sequence and adding
// one. A missing pool root means an empty pool, so the first member is wt-0001.
func (s *Service) nextPoolMemberSeq() (int, error) {
	entries, err := os.ReadDir(s.poolRepoRoot())
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, fmt.Errorf("read pool root %s: %w", s.poolRepoRoot(), err)
	}
	highest := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		seq, err := poolMemberSeq(entry.Name())
		if err != nil {
			continue
		}
		if seq > highest {
			highest = seq
		}
	}
	return highest + 1, nil
}

// readPoolRecord reads the durable pool record for a member folder. A missing record
// reports ok=false (the folder is not a pool member), not an error, so enumeration can
// skip non-pool folders cheaply.
func readPoolRecord(memberDir string) (poolRecord, bool, error) {
	raw, err := os.ReadFile(filepath.Join(memberDir, poolRecordFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return poolRecord{}, false, nil
		}
		return poolRecord{}, false, fmt.Errorf("read pool record %s: %w", memberDir, err)
	}
	var record poolRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return poolRecord{}, false, fmt.Errorf("decode pool record %s: %w", memberDir, err)
	}
	return record, true, nil
}

// writePoolRecord persists a member's pool record (worktree-pool.json) inside its
// member folder. The folder must already exist (Create makes it before the worktree
// is added).
func (s *Service) writePoolRecord(seq int, record poolRecord) error {
	return writeJSONFile(s.poolRecordPath(seq), record)
}

// enumeratePoolRecords scans this repo's pool root for member folders that carry a
// durable pool record, keyed by worktree id. It surfaces idle members (a folder that
// exists with run_id == "") rather than dropping them, which is what lets the pool
// persist when no run is bound. A missing pool root yields an empty map. The boolean
// reports, per id, whether the member's `w` checkout still exists on disk.
func (s *Service) enumeratePoolRecords() (map[string]poolRecord, error) {
	out := map[string]poolRecord{}
	entries, err := os.ReadDir(s.poolRepoRoot())
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("read pool root %s: %w", s.poolRepoRoot(), err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := poolMemberSeq(entry.Name()); err != nil {
			continue
		}
		memberDir := filepath.Join(s.poolRepoRoot(), entry.Name())
		record, ok, err := readPoolRecord(memberDir)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		out[record.WorktreeID] = record
	}
	return out, nil
}

// sortedPoolWorktrees returns pool entries sorted by worktree id for deterministic
// reads and proof.
func sortedPoolWorktrees(in []PoolWorktree) []PoolWorktree {
	sort.Slice(in, func(i, j int) bool { return in[i].WorktreeID < in[j].WorktreeID })
	return in
}

// classifyPoolMember derives a member's live status from its durable record. A record
// with an empty run_id is idle. A record with a run_id is allocated ONLY while its
// per-run workflow is still LIVE (a live binding is read from it, Task-0015 Landing-2
// authority); a run that has ended (ErrRunNotFound / no live binding) reclassifies the
// member to idle. This is what reconstructs allocated-vs-idle across a backend restart
// without losing bound state for a still-live run.
func (s *Service) classifyPoolMember(record poolRecord) PoolWorktree {
	// Idle baseline: repo + worktree_path always present, no task/session/gate bound.
	entry := PoolWorktree{
		WorktreeID: record.WorktreeID,
		Status:     poolStatusIdle,
		RepoBinding: RepoBinding{
			Repo:         record.Repo,
			WorktreePath: record.WorktreePath,
		},
	}
	if record.RunID == "" {
		return entry
	}
	if s.runtime == nil {
		// No runtime to consult: trust the record's own run id + a minimal allocated
		// binding so a still-bound member is not silently reclassified idle in a
		// runtime-less test.
		entry.Status = poolStatusAllocated
		entry.RunID = record.RunID
		entry.RunGateState = RunGateStateRunning
		return entry
	}
	view, err := s.runtime.GetActiveTaskRun(context.Background(), record.RunID)
	if err != nil || !runOwnsLiveStory(view) {
		// The bound run has ended (closed/failed/interrupted) or is gone: the member is
		// no longer allocated. It returns to idle (its folder is kept).
		return entry
	}
	entry.Status = poolStatusAllocated
	entry.RunID = record.RunID
	entry.RunGateState = RunGateStateRunning
	if view.RepoLane.Binding != nil {
		// Carry the live task/run/gate/session binding, but keep the stable pool path +
		// repo from the record (the binding's worktree path mirrors it).
		binding := *view.RepoLane.Binding
		if binding.WorktreePath == "" {
			binding.WorktreePath = record.WorktreePath
		}
		if binding.Repo == "" {
			binding.Repo = record.Repo
		}
		entry.RepoBinding = binding
	}
	return entry
}

// ListPoolWorktrees returns the FULL pool for this Service's repo — every member,
// idle and allocated — each with its stable identity + status; allocated members carry
// the live binding (task/run/gate/session). It is the single read both discover
// assertions and the GET /api/v1/worktrees full-pool read use. A repo with no pool root
// yet returns an empty slice.
func (s *Service) ListPoolWorktrees() ([]PoolWorktree, error) {
	records, err := s.enumeratePoolRecords()
	if err != nil {
		return nil, err
	}
	out := make([]PoolWorktree, 0, len(records))
	for _, record := range records {
		out = append(out, s.classifyPoolMember(record))
	}
	return sortedPoolWorktrees(out), nil
}

// ListFullPool returns the full-pool view the GET /api/v1/worktrees read serves: every
// pool member (idle + allocated) from ListPoolWorktrees, MERGED with any active
// owned-lane worktree that is not itself a pool member. The merge keeps the endpoint
// correct across the dispatch-path transition: before PASS-0003 a dispatch still
// provisions a non-pool random-temp lane (surfaced here as an allocated entry from its
// live binding), and after PASS-0003 a dispatched lane IS a pool member (surfaced via
// the pool path) — either way it appears, and REG-008's parked-lane read keeps working.
// Entries are deduped by worktree path (a pool member wins over a legacy lane at the
// same path) and sorted by worktree id.
func (s *Service) ListFullPool() ([]PoolWorktree, error) {
	members, err := s.ListPoolWorktrees()
	if err != nil {
		return nil, err
	}
	seenPath := map[string]bool{}
	out := make([]PoolWorktree, 0, len(members))
	for _, m := range members {
		out = append(out, m)
		if m.WorktreePath != "" {
			seenPath[m.WorktreePath] = true
		}
	}
	// Merge active owned-lane worktrees that are NOT pool members (legacy random-temp
	// dispatch lanes). They are allocated by definition (an active worktree is bound to
	// a live run). Their worktree id falls back to the worktree path (no stable pool id).
	active, err := s.ListActiveWorktrees()
	if err != nil {
		return nil, err
	}
	for _, b := range active {
		if b.WorktreePath != "" && seenPath[b.WorktreePath] {
			continue
		}
		out = append(out, PoolWorktree{
			WorktreeID:  b.WorktreePath,
			Status:      poolStatusAllocated,
			RunID:       b.RunID,
			RepoBinding: b.RepoBinding,
		})
	}
	return sortedPoolWorktrees(out), nil
}

// DiscoverPool performs startup discovery for this repo's pool: it ENUMERATES the pool
// member folders on disk, reconstructs each one's idle-vs-allocated state from its
// durable record + the live per-run workflow, and persists the corrected run_id back
// onto a record whose bound run has ended (so the durable record reflects idle once the
// run is gone). It does NOT create or destroy any folder, and it preserves the existing
// `git worktree prune` hygiene (ReconcileOwnedLanes) for genuinely stale git metadata.
// It is invoked at the same wiring point the prune-only reconcile was.
//
// Crucially it never reclassifies a STILL-LIVE allocated member as idle (that is the
// load-bearing restart-survival guarantee, REG-008): classifyPoolMember marks a member
// idle only when its bound run has actually ended.
func (s *Service) DiscoverPool() error {
	// Keep the prune-only hygiene first so a crashed/partial owned-lane removal does not
	// linger. The prune is best-effort: a prune failure (e.g. a non-git declared root in
	// a focused test) must not abort pool reconstruction — it never reclaims a live
	// worktree, exactly as the wiring already treats it.
	_ = s.ReconcileOwnedLanes()
	records, err := s.enumeratePoolRecords()
	if err != nil {
		return err
	}
	for _, record := range records {
		if record.RunID == "" {
			continue
		}
		entry := s.classifyPoolMember(record)
		if entry.Status == poolStatusIdle {
			// The bound run ended: persist the member back to idle so the durable record
			// is consistent with the reconstructed state (the folder is kept).
			seq, err := s.poolSeqForID(record.WorktreeID)
			if err != nil {
				continue
			}
			record.RunID = ""
			if err := s.writePoolRecord(seq, record); err != nil {
				return fmt.Errorf("persist reclassified idle pool member %s: %w", record.WorktreeID, err)
			}
		}
	}
	return nil
}

// poolSeqForID parses the wt-<NNNN> sequence number from a <repo>/wt-<NNNN> worktree id
// that belongs to this Service's repo segment.
func (s *Service) poolSeqForID(worktreeID string) (int, error) {
	if _, err := s.poolMemberDirForID(worktreeID); err != nil {
		return 0, err
	}
	_, leaf, _ := strings.Cut(worktreeID, "/")
	return poolMemberSeq(leaf)
}

// RepoView is one registered repo projected by GET /api/v1/repos: its id, the
// arbitrary absolute local_root, and the task-provider repo it polls. queue_workers is
// DELIBERATELY not projected — the pool's idle count is the only concurrency bound now
// (Task-0016 removes queue_workers as an admission cap in PASS-0003).
type RepoView struct {
	ID               string `json:"id"`
	LocalRoot        string `json:"local_root"`
	TaskProviderRepo string `json:"task_provider_repo,omitempty"`
}

// ListRepos reads the central repo registry at the given explicit path
// (OBSIDIAN_REGISTRY_PATH) and projects one RepoView per registered repo — id +
// local_root (+ task_provider_repo), with NO queue_workers in the response. It is the
// registry-sourced repo list the UI's repo filter dropdown consumes.
func ListRepos(registryPath string) ([]RepoView, error) {
	manifest, err := queue.LoadRegistry(registryPath)
	if err != nil {
		return nil, err
	}
	repos := make([]RepoView, 0, len(manifest.Repos))
	for _, entry := range manifest.Repos {
		if entry.ID == "" || entry.LocalRoot == "" {
			continue
		}
		view := RepoView{ID: entry.ID, LocalRoot: entry.LocalRoot}
		if entry.TaskProvider != nil {
			view.TaskProviderRepo = entry.TaskProvider.Repo
		}
		repos = append(repos, view)
	}
	sort.Slice(repos, func(i, j int) bool { return repos[i].ID < repos[j].ID })
	return repos, nil
}

// ErrPoolWorktreeAllocated is returned by DestroyPoolWorktree when the target member is
// allocated (bound to a live run). The operator must Eject it first; Destroy only
// removes an idle member.
var ErrPoolWorktreeAllocated = errors.New("worktree is allocated; eject it before destroy")

// ErrPoolWorktreeNotFound is returned when a worktree id names no pool member.
var ErrPoolWorktreeNotFound = errors.New("pool worktree not found")

// ErrNoIdleWorktree is returned by the pool-draw path when no idle worktree is available
// to draw for a dispatch/assign (an empty pool defers — no auto-create).
var ErrNoIdleWorktree = errors.New("no idle worktree available in the repo pool")

// DequeueProvider is the task-provider WRITE seam the Service uses to take a task out of
// the queue (Task-0016): set the issue's queue state to not-ready. It is a small interface
// on the Service so Eject and the standalone dequeue endpoint call THROUGH the provider
// (never an inline gh call), and so tests inject a fake. Production wires it to the
// queue.QueueProvider built in the queuedrain wiring (symmetric to the read provider). A
// nil provider makes a dequeue a safe no-op (e.g. the manual single-repo control plane
// with no provider configured).
type DequeueProvider interface {
	DequeueIssue(repo string, number int) error
}

// SetDequeueProvider injects the task-provider write capability used by DequeueTask and
// Eject. It is set by the per-repo queuedrain wiring (the same place the read provider is
// built). Left nil, dequeue is a safe no-op.
func (s *Service) SetDequeueProvider(p DequeueProvider) {
	s.dequeueProvider = p
}

// DequeueTask takes a task out of the queue THROUGH the task provider (Queue -> Never)
// WITHOUT ejecting: it does not stop the agent, clean the checkout, or unbind the run.
// It resolves the issue number from the task id and calls the provider dequeue; a task
// with no parseable issue number (or no provider configured) is a safe no-op. It never
// closes the issue (the issue stays open). It is the standalone POST
// /api/v1/worktrees/dequeue operation and is also reused by Eject (PASS-0005).
func (s *Service) DequeueTask(repo string, taskID string) error {
	if s.dequeueProvider == nil {
		return nil
	}
	number, err := queue.IssueNumberFromTaskID(taskID)
	if err != nil {
		// No provider-backed issue number (e.g. a non-issue task): safe no-op.
		return nil
	}
	return s.dequeueProvider.DequeueIssue(repo, number)
}

// drawnLane is one idle pool worktree drawn for a dispatch/assign: its sequence number
// (to update the record's run_id after the run starts) and a RepoLane pointed at its
// stable checkout, already reset to baseline.
type drawnLane struct {
	seq      int
	record   poolRecord
	repoLane RepoLane
}

// drawIdlePoolWorktree picks an IDLE pool worktree to dispatch into and resets its
// existing checkout to baseline — the pool-draw that replaced provisionOwnedLane's
// fresh os.MkdirTemp dir. With worktreeID set it draws that specific idle member
// (rejecting it if not idle); with worktreeID empty (the consumer auto-assign path) it
// draws the lowest-sequence idle member. An empty pool yields ErrNoIdleWorktree. It
// provisions NO new directory: the acceptance bar is reusing an existing idle folder.
func (s *Service) drawIdlePoolWorktree(worktreeID string) (drawnLane, error) {
	records, err := s.enumeratePoolRecords()
	if err != nil {
		return drawnLane{}, err
	}

	var chosenID string
	if strings.TrimSpace(worktreeID) != "" {
		record, ok := records[worktreeID]
		if !ok {
			return drawnLane{}, fmt.Errorf("%w: %s", ErrPoolWorktreeNotFound, worktreeID)
		}
		if s.classifyPoolMember(record).Status != poolStatusIdle {
			return drawnLane{}, fmt.Errorf("%w: %s is allocated", ErrNoIdleWorktree, worktreeID)
		}
		chosenID = worktreeID
	} else {
		// Lowest-id idle member, for deterministic draw order.
		ids := make([]string, 0, len(records))
		for id, record := range records {
			if s.classifyPoolMember(record).Status == poolStatusIdle {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			return drawnLane{}, ErrNoIdleWorktree
		}
		sort.Strings(ids)
		chosenID = ids[0]
	}

	record := records[chosenID]
	seq, err := s.poolSeqForID(chosenID)
	if err != nil {
		return drawnLane{}, err
	}

	baselineCommit := gitRevision(s.declaredWorktreeRoot)
	if baselineCommit == "" {
		return drawnLane{}, fmt.Errorf("resolve baseline commit for %s", s.declaredWorktreeRoot)
	}
	repoLane := RepoLane{
		OwnedRepoRoot:         record.WorktreePath,
		CheckoutMode:          "git_worktree_detached",
		BaselineCommit:        baselineCommit,
		ApprovedRestoreCommit: baselineCommit,
		ResetStatus:           "not_run",
	}
	// Reset the EXISTING checkout to baseline (no fresh dir provisioned).
	repoLane, err = s.restoreOwnedLane(repoLane)
	if err != nil {
		return drawnLane{}, fmt.Errorf("reset idle pool worktree %s to baseline: %w", chosenID, err)
	}
	return drawnLane{seq: seq, record: record, repoLane: repoLane}, nil
}

// markPoolMemberRun records the started run id on a drawn pool member's durable record
// (idle -> allocated). A failure to persist after the run has started is returned so the
// caller can surface it, but the run itself is already live.
func (s *Service) markPoolMemberRun(drawn drawnLane, runID string) error {
	record := drawn.record
	record.RunID = runID
	return s.writePoolRecord(drawn.seq, record)
}

// returnPoolMemberToIdle finds the pool member whose checkout is ownedRepoRoot and, if it
// is a pool member, clears its run_id (allocated -> idle) WITHOUT deleting the folder, so
// a superseded same-task run frees its pool worktree for reuse. It reports whether the
// checkout was a pool member. A non-pool (legacy random-temp) checkout reports false so
// the caller can fall back to the delete path.
func (s *Service) returnPoolMemberToIdle(ownedRepoRoot string) (bool, error) {
	records, err := s.enumeratePoolRecords()
	if err != nil {
		return false, err
	}
	for id, record := range records {
		if !sameRepoRoot(record.WorktreePath, ownedRepoRoot) {
			continue
		}
		seq, err := s.poolSeqForID(id)
		if err != nil {
			return false, err
		}
		if record.RunID == "" {
			return true, nil // already idle
		}
		record.RunID = ""
		if err := s.writePoolRecord(seq, record); err != nil {
			return false, fmt.Errorf("return pool member %s to idle: %w", id, err)
		}
		return true, nil
	}
	return false, nil
}

// CreatePoolWorktree provisions one new IDLE pool worktree into this Service's repo
// pool at the next stable path (git worktree add --detach <stablePath> <baselineCommit>
// — the provisionOwnedLane git mechanics, but at a STABLE, non-temp path and with NO
// task bound), writes its durable pool record with run_id == "", and returns it as an
// idle PoolWorktree. The repo argument is the operator-facing repo id; it is recorded on
// the member but the worktree is always created under THIS Service's bound repo (the
// backend Service is per-repo). Worktree CREATION happens only here (and never as a
// dispatch side effect, after PASS-0003).
func (s *Service) CreatePoolWorktree(repo string) (PoolWorktree, error) {
	baselineCommit := gitRevision(s.declaredWorktreeRoot)
	if baselineCommit == "" {
		return PoolWorktree{}, fmt.Errorf("resolve baseline commit for repo %q", s.declaredWorktreeRoot)
	}
	seq, err := s.nextPoolMemberSeq()
	if err != nil {
		return PoolWorktree{}, err
	}
	checkout := s.poolCheckoutPath(seq)
	if err := os.MkdirAll(s.poolMemberDir(seq), 0o755); err != nil {
		return PoolWorktree{}, fmt.Errorf("create pool member dir: %w", err)
	}

	args := []string{"-C", s.declaredWorktreeRoot}
	if runtime.GOOS == "windows" {
		args = append([]string{"-c", "core.longpaths=true"}, args...)
	}
	args = append(args, "worktree", "add", "--detach", checkout, baselineCommit)
	if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		// Best-effort cleanup of the partially-created member dir so a failed Create does
		// not leave a stray folder that nextPoolMemberSeq would skip over.
		_ = os.RemoveAll(s.poolMemberDir(seq))
		return PoolWorktree{}, fmt.Errorf("create pool worktree: %w: %s", err, strings.TrimSpace(string(output)))
	}

	recordRepo := strings.TrimSpace(repo)
	if recordRepo == "" {
		recordRepo = s.poolRepoSegment()
	}
	record := poolRecord{
		WorktreeID:   s.poolWorktreeID(seq),
		WorktreePath: checkout,
		Repo:         recordRepo,
		RunID:        "",
	}
	if err := s.writePoolRecord(seq, record); err != nil {
		return PoolWorktree{}, fmt.Errorf("write pool record: %w", err)
	}
	return PoolWorktree{
		WorktreeID: record.WorktreeID,
		Status:     poolStatusIdle,
		RepoBinding: RepoBinding{
			Repo:         record.Repo,
			WorktreePath: record.WorktreePath,
		},
	}, nil
}

// DestroyPoolWorktree removes an IDLE pool worktree from the pool: it rejects
// (ErrPoolWorktreeAllocated, removing nothing) a member that is allocated to a live run,
// otherwise it removes the checkout via the BUG-0002-hardened removeOwnedLaneWorktree
// mechanics and deletes the member folder + its durable pool record. An unknown id
// reports ErrPoolWorktreeNotFound.
func (s *Service) DestroyPoolWorktree(worktreeID string) error {
	memberDir, err := s.poolMemberDirForID(worktreeID)
	if err != nil {
		return err
	}
	record, ok, err := readPoolRecord(memberDir)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s", ErrPoolWorktreeNotFound, worktreeID)
	}
	// Classify live: an allocated member (its bound run is still live) must be ejected
	// first; Destroy never tears down a live run's checkout.
	if s.classifyPoolMember(record).Status == poolStatusAllocated {
		return fmt.Errorf("%w: %s", ErrPoolWorktreeAllocated, worktreeID)
	}
	checkout := record.WorktreePath
	if checkout == "" {
		checkout = filepath.Join(memberDir, filepath.Base(memberDir))
	}
	if !pathWithinRoot(checkout, s.ownedLaneRoot) {
		return fmt.Errorf("pool worktree %q is outside the backend-owned lane root", checkout)
	}
	if err := removeOwnedLaneWorktree(s.declaredWorktreeRoot, s.ownedLaneRoot, checkout); err != nil {
		return err
	}
	// Drop the whole member folder (checkout + the durable pool record).
	if err := os.RemoveAll(memberDir); err != nil {
		return fmt.Errorf("remove pool member dir %s: %w", memberDir, err)
	}
	return nil
}
