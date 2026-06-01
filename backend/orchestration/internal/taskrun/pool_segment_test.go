package taskrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Task-0016 BUG-0002: the MULTI-REPO dashboard control-plane Service (no repoNamespace,
// SetPoolSegmentRegistry set) must segment a `repo`-keyed Create/Eject/Destroy/list by the
// registry repo id — the SAME segment the registry-driven consumer (SetRepoNamespace
// repo.ID) reads — so a Created worktree is visible to that repo's pool-draw. Before the
// fix the dashboard hashed its single declaredWorktreeRoot into a `repo-<hash>` segment the
// consumer never read, so a Created worktree was invisible (the REG-007 live blocker).

// newGitRoot builds a minimal real git repo (so `git worktree add` works for Create) and
// returns its root.
func newGitRoot(t *testing.T) string {
	t.Helper()
	return writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0007": {
			taskMD:    "# Task 0007\n\n## Title\n\nPool fixture.\n\n## Summary\n\nPool fixture task.\n",
			taskState: readyTaskState("Task-0007"),
			planMD:    "# approved plan\n",
		},
	})
}

// newRegistryWithRepo writes a central registry mapping one id -> a git local_root +
// task_provider slug, returning the registry path.
func newRegistryWithRepo(t *testing.T, id string, localRoot string, providerSlug string) string {
	t.Helper()
	registry := filepath.Join(t.TempDir(), "REPO-MANIFEST.json")
	writeFile(t, registry, `{
  "repos": [
    {"id": "`+id+`", "local_root": "`+filepath.ToSlash(localRoot)+`",
     "task_provider": {"kind": "github_issues", "repo": "`+providerSlug+`"}}
  ]
}`)
	return registry
}

// TestDashboardCreateLandsInRegistryIDSegmentSeenByConsumer is the BUG-0002 proof: a
// dashboard Service whose declaredWorktreeRoot is NOT the registry local_root (it would
// otherwise hash to a `repo-<hash>` segment) Creates a worktree for "RepoA" and it lands
// under the RepoA registry-id segment, AND a registry-driven consumer Service (namespace
// "RepoA", same shared owned-lane root, declaredWorktreeRoot = the registry local_root)
// SEES that worktree as idle and DRAWS it — the exact dashboard->consumer hand-off that was
// broken.
func TestDashboardCreateLandsInRegistryIDSegmentSeenByConsumer(t *testing.T) {
	// The repo's real local_root (a git repo): the consumer dispatches into this.
	repoLocalRoot := newGitRoot(t)
	registry := newRegistryWithRepo(t, "RepoA", repoLocalRoot, "Org/RepoA")

	// Shared owned-lane root both Services segment under (the production cdxow root).
	ownedLaneRoot := filepath.Join(t.TempDir(), "cdxow")

	// Dashboard Service: a DIFFERENT declared root (so its hashed fallback segment differs
	// from "RepoA"), no namespace, registry-resolved pool segment.
	dashRoot := newGitRoot(t)
	dash := NewService(dashRoot, filepath.Join(dashRoot, ".runs"), newFakeRuntime())
	dash.ownedLaneRoot = ownedLaneRoot
	dash.SetPoolSegmentRegistry(registry)

	// Pre-fix sanity: without the fix the dashboard's default segment is a hash, not RepoA.
	if dash.poolRepoSegment() == "RepoA" {
		t.Fatalf("test setup wrong: dashboard default segment must not already be RepoA")
	}

	created, err := dash.CreatePoolWorktree("RepoA")
	if err != nil {
		t.Fatalf("dashboard create for RepoA: %v", err)
	}
	if created.WorktreeID != "RepoA/wt-0001" {
		t.Fatalf("created worktree id = %q, want RepoA/wt-0001 (registry-id segment)", created.WorktreeID)
	}
	wantUnder := filepath.Join(ownedLaneRoot, "RepoA")
	if !strings.HasPrefix(created.WorktreePath, wantUnder+string(os.PathSeparator)) {
		t.Fatalf("created worktree path %q is not under the RepoA segment %q", created.WorktreePath, wantUnder)
	}

	// The consumer Service for RepoA (namespace = repo id, same owned-lane root, declared
	// root = the registry local_root) MUST see the Created worktree as idle and draw it.
	consumer := NewServiceForRepo(repoLocalRoot, filepath.Join(repoLocalRoot, ".runs"), newFakeRuntime())
	consumer.SetRepoNamespace("RepoA")
	consumer.ownedLaneRoot = ownedLaneRoot

	idle, err := consumer.IdleWorktreeCount()
	if err != nil {
		t.Fatalf("consumer idle count: %v", err)
	}
	if idle != 1 {
		t.Fatalf("consumer idle count for RepoA = %d, want 1 (the dashboard-Created worktree)", idle)
	}

	pool, err := consumer.ListPoolWorktrees()
	if err != nil {
		t.Fatalf("consumer list pool: %v", err)
	}
	if len(pool) != 1 || pool[0].WorktreeID != "RepoA/wt-0001" || pool[0].Status != poolStatusIdle {
		t.Fatalf("consumer pool = %#v, want one idle RepoA/wt-0001", pool)
	}

	// The consumer's pool-draw (the dispatch admission path) draws the dashboard-Created
	// worktree — the leg that previously failed with ErrNoIdleWorktree.
	drawn, err := consumer.drawIdlePoolWorktree("")
	if err != nil {
		t.Fatalf("consumer pool-draw of the dashboard-Created worktree: %v", err)
	}
	if drawn.record.WorktreeID != "RepoA/wt-0001" {
		t.Fatalf("consumer drew %q, want RepoA/wt-0001", drawn.record.WorktreeID)
	}
}

// TestDashboardEjectAndDestroyResolveRegistryIDSegment proves the worktree_id-keyed ops
// (Eject, Destroy) on the multi-repo dashboard resolve the SAME RepoA segment a Create
// landed in — the dashboard addresses a "RepoA/wt-NNNN" worktree even though its own default
// segment is a hash. (Eject of an idle member is the idempotent clean-and-return; Destroy of
// an idle member removes it.)
func TestDashboardEjectAndDestroyResolveRegistryIDSegment(t *testing.T) {
	repoLocalRoot := newGitRoot(t)
	registry := newRegistryWithRepo(t, "RepoA", repoLocalRoot, "Org/RepoA")
	ownedLaneRoot := filepath.Join(t.TempDir(), "cdxow")

	dashRoot := newGitRoot(t)
	dash := NewService(dashRoot, filepath.Join(dashRoot, ".runs"), newFakeRuntime())
	dash.ownedLaneRoot = ownedLaneRoot
	dash.SetPoolSegmentRegistry(registry)

	created, err := dash.CreatePoolWorktree("RepoA")
	if err != nil {
		t.Fatalf("dashboard create: %v", err)
	}

	// Eject (idempotent on an idle member): resolves the RepoA segment, cleans + returns
	// idle, keeps the folder.
	ejected, err := dash.EjectWorktree(context.Background(), "", created.WorktreeID)
	if err != nil {
		t.Fatalf("dashboard eject RepoA/wt-0001: %v", err)
	}
	if ejected.WorktreeID != "RepoA/wt-0001" || ejected.Status != poolStatusIdle {
		t.Fatalf("ejected = %#v, want idle RepoA/wt-0001", ejected)
	}

	// Destroy: resolves the RepoA segment and removes the member; the dashboard list no
	// longer carries it.
	if err := dash.DestroyPoolWorktree(created.WorktreeID); err != nil {
		t.Fatalf("dashboard destroy RepoA/wt-0001: %v", err)
	}
	pool, err := dash.ListFullPool()
	if err != nil {
		t.Fatalf("dashboard list after destroy: %v", err)
	}
	if len(pool) != 0 {
		t.Fatalf("dashboard pool after destroy = %#v, want empty", pool)
	}
}

// TestDashboardCreateFallsBackWhenRepoUnknown proves the safe fallback: a `repo` arg the
// registry does not know does NOT crash and lands in the Service's hashed default segment
// (legacy behavior), not under a bogus segment.
func TestDashboardCreateFallsBackWhenRepoUnknown(t *testing.T) {
	repoLocalRoot := newGitRoot(t)
	registry := newRegistryWithRepo(t, "RepoA", repoLocalRoot, "Org/RepoA")
	ownedLaneRoot := filepath.Join(t.TempDir(), "cdxow")

	dashRoot := newGitRoot(t)
	dash := NewService(dashRoot, filepath.Join(dashRoot, ".runs"), newFakeRuntime())
	dash.ownedLaneRoot = ownedLaneRoot
	dash.SetPoolSegmentRegistry(registry)

	created, err := dash.CreatePoolWorktree("NotInRegistry")
	if err != nil {
		t.Fatalf("dashboard create for unknown repo: %v", err)
	}
	wantSeg := dash.poolRepoSegment() // the hashed default segment
	if created.WorktreeID != wantSeg+"/wt-0001" {
		t.Fatalf("unknown-repo create id = %q, want fallback %q/wt-0001", created.WorktreeID, wantSeg)
	}
}

// TestReclaimReturnsPoolMemberToIdleKeepingCheckout proves the reclaim/run_id fix: a
// close->reclaim of a pool worktree returns it to IDLE (run_id nulled, folder + checkout
// KEPT) so the freed member is reused, instead of deleting the checkout and leaving a stale
// run_id on the record. This is the close->reclaim->reuse leg of REG-007.
func TestReclaimReturnsPoolMemberToIdleKeepingCheckout(t *testing.T) {
	service := newGitPoolTestService(t, "RepoA")

	created, err := service.CreatePoolWorktree("RepoA")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Mark the member allocated to a live run + persist a bootstrap record so
	// findActiveLaneRecord (used by ReclaimOwnedLane) resolves the active lane.
	runID := ActiveRunIDForRepo("RepoA", "Task-0007")
	service.runtime = poolDiscoverRuntime{liveRunID: runID, liveTaskID: "Task-0007"}
	record, _, err := readPoolRecord(service.poolMemberDir(1))
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	record.RunID = runID
	if err := service.writePoolRecord(1, record); err != nil {
		t.Fatalf("mark allocated: %v", err)
	}
	writeActiveLaneBootstrap(t, service, "Task-0007", runID, created.WorktreePath)

	// The run has now ended (terminal close): the runtime no longer reports it live.
	service.runtime = poolDiscoverRuntime{}

	if err := service.ReclaimOwnedLane("Task-0007"); err != nil {
		t.Fatalf("reclaim owned lane: %v", err)
	}

	// The checkout + member folder are KEPT (reused), not deleted.
	if _, statErr := os.Stat(created.WorktreePath); statErr != nil {
		t.Fatalf("reclaimed pool checkout must be kept for reuse: %v", statErr)
	}
	// The durable record's run_id is NULLED (no stale run_id), so it reads idle.
	after, _, err := readPoolRecord(service.poolMemberDir(1))
	if err != nil {
		t.Fatalf("read record after reclaim: %v", err)
	}
	if after.RunID != "" {
		t.Fatalf("reclaimed pool record run_id = %q, want empty (nulled on reclaim)", after.RunID)
	}
	idle, err := service.IdleWorktreeCount()
	if err != nil {
		t.Fatalf("idle count after reclaim: %v", err)
	}
	if idle != 1 {
		t.Fatalf("idle count after reclaim = %d, want 1 (the freed member is reusable)", idle)
	}
}

// writeActiveLaneBootstrap writes the owned-lane bootstrap record findActiveLaneRecord
// reads, scoped to this Service's declared worktree root + run-artifacts root.
func writeActiveLaneBootstrap(t *testing.T, s *Service, taskID, runID, ownedRepoRoot string) {
	t.Helper()
	dir := filepath.Join(s.runArtifactsRoot, sanitizePathSegment(taskID), sanitizePathSegment(runID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir bootstrap dir: %v", err)
	}
	rec := ownedLaneBootstrapRecord{
		TaskID:               taskID,
		RunID:                runID,
		OwnedRepoRoot:        ownedRepoRoot,
		DeclaredWorktreeRoot: s.declaredWorktreeRoot,
		BootstrappedAt:       s.now(),
		Binding: &RepoBinding{
			Repo:         "RepoA",
			TaskID:       taskID,
			WorktreePath: ownedRepoRoot,
			RunGateState: RunGateStateRunning,
		},
	}
	if err := writeJSONFile(filepath.Join(dir, "owned-lane-bootstrap.json"), rec); err != nil {
		t.Fatalf("write bootstrap record: %v", err)
	}
}
