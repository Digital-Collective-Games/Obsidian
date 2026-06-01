package taskrun

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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

// poolCheckoutDirName is the checkout subdir under a pool member folder. The folder
// itself holds poolRecordFileName; the `w` child is the git worktree checkout (the
// same `/w` convention provisionOwnedLane uses).
const poolCheckoutDirName = "w"

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
// every member (idle and allocated); allocated members additionally carry the live
// binding (task/run/gate/session) read from the per-run workflow.
type PoolWorktree struct {
	WorktreeID   string `json:"worktree_id"`
	Repo         string `json:"repo"`
	WorktreePath string `json:"worktree_path"`
	Status       string `json:"status"`
	// RunID and Binding are populated only for an allocated member (status ==
	// "allocated"); they are read live from the per-run workflow with a breadcrumb
	// fallback. An idle member leaves them zero.
	RunID   string       `json:"run_id,omitempty"`
	Binding *RepoBinding `json:"binding,omitempty"`
}

// poolRepoSegment is the stable, filesystem-safe per-repo directory segment under the
// owned-lane root that anchors a repo's pool. It prefers this Service's repoNamespace
// (the registry repo id set by NewServiceForRepo) and falls back to a sanitized
// repoIdentity() so the single-repo / manual control plane still groups its pool under
// a stable segment.
func (s *Service) poolRepoSegment() string {
	if s.repoNamespace != "" {
		return sanitizePathSegment(s.repoNamespace)
	}
	return sanitizePathSegment(s.repoIdentity())
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

// poolCheckoutPath is the `w` checkout path for a member sequence number — the stable
// worktree_path (replacing provisionOwnedLane's random os.MkdirTemp dir).
func (s *Service) poolCheckoutPath(seq int) string {
	return filepath.Join(s.poolMemberDir(seq), poolCheckoutDirName)
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
	entry := PoolWorktree{
		WorktreeID:   record.WorktreeID,
		Repo:         record.Repo,
		WorktreePath: record.WorktreePath,
		Status:       poolStatusIdle,
	}
	if record.RunID == "" {
		return entry
	}
	if s.runtime == nil {
		// No runtime to consult: trust the record's own run id + breadcrumb binding so
		// a still-bound member is not silently reclassified idle in a runtime-less test.
		entry.Status = poolStatusAllocated
		entry.RunID = record.RunID
		binding := s.poolBindingFallback(record)
		entry.Binding = &binding
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
	binding := s.poolBindingFallback(record)
	if view.RepoLane.Binding != nil {
		binding = *view.RepoLane.Binding
	}
	entry.Binding = &binding
	return entry
}

// poolBindingFallback builds a minimal binding for an allocated pool member from its
// record when the live workflow carries none yet, so an allocated member always names
// its task/worktree even before the workflow seeds the binding.
func (s *Service) poolBindingFallback(record poolRecord) RepoBinding {
	return RepoBinding{
		Repo:         record.Repo,
		WorktreePath: record.WorktreePath,
		RunGateState: RunGateStateRunning,
	}
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
