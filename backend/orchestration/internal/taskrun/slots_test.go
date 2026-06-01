package taskrun

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// twoSiblingFixture builds a git tracking root with two distinct, dispatch-ready
// sibling tasks so the pool-draw dispatch path can be exercised with N>1 lanes. Its
// owned-lane root is isolated under the repo so the pool layout is inspectable.
func twoSiblingFixture(t *testing.T) string {
	t.Helper()
	return writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0001": {
			taskMD:    "# Task 0001\n\n## Title\n\nFirst sibling.\n\n## Summary\n\nFirst sibling queue lane.\n",
			taskState: readyTaskState("Task-0001"),
			planMD:    "# approved plan\n",
		},
		"Task-0002": {
			taskMD:    "# Task 0002\n\n## Title\n\nSecond sibling.\n\n## Summary\n\nSecond sibling queue lane.\n",
			taskState: readyTaskState("Task-0002"),
			planMD:    "# approved plan\n",
		},
	})
}

func readyTaskState(taskID string) string {
	return `{
  "task_id": "` + taskID + `",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0001",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-05-29T12:00:00-04:00"
}`
}

// newSiblingPoolService builds a Service over the two-sibling git fixture with an
// isolated owned-lane root so CreatePoolWorktree provisions stable pool members the
// pool-draw dispatch path can draw from.
func newSiblingPoolService(t *testing.T) *Service {
	t.Helper()
	worktreeRoot := twoSiblingFixture(t)
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), newFakeRuntime())
	service.ownedLaneRoot = filepath.Join(worktreeRoot, "owned-lanes")
	return service
}

// TestNewServiceForRepoBindsRoot proves the repo-parameterized constructor binds the
// Service to the passed local_root. There is no per-repo numeric cap any more
// (Task-0016 removed queue_workers); concurrency is bounded by the idle pool count.
func TestNewServiceForRepoBindsRoot(t *testing.T) {
	worktreeRoot := twoSiblingFixture(t)
	service := NewServiceForRepo(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), newFakeRuntime())
	if service.declaredWorktreeRoot != worktreeRoot {
		t.Fatalf("declaredWorktreeRoot = %q, want %q (bound to the registry local_root)", service.declaredWorktreeRoot, worktreeRoot)
	}
}

// TestDispatchGateAdmitsWhileIdleWorktreeRemainsAndRefusesWhenEmpty table-drives the
// Task-0016 pool-draw gate over a stubbed idle worktree count: with an idle worktree, no
// task_already_running / no_idle_worktree block is emitted; with an empty pool, dispatch
// is refused with no_idle_worktree.
func TestDispatchGateAdmitsWhileIdleWorktreeRemainsAndRefusesWhenEmpty(t *testing.T) {
	worktreeRoot := twoSiblingFixture(t)

	cases := []struct {
		name             string
		idle             int
		wantReady        bool
		wantBlockCode    string
		forbidBlockCodes []string
	}{
		{name: "one idle worktree admits", idle: 1, wantReady: true, forbidBlockCodes: []string{"task_already_running", "no_idle_worktree"}},
		{name: "several idle worktrees admit", idle: 3, wantReady: true, forbidBlockCodes: []string{"task_already_running", "no_idle_worktree"}},
		{name: "empty pool refuses", idle: 0, wantReady: false, wantBlockCode: "no_idle_worktree"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), newFakeRuntime())
			idle := tc.idle
			service.idleWorktreeCount = func() (int, error) { return idle, nil }

			task, err := service.Task(context.Background(), "Task-0002")
			if err != nil {
				t.Fatalf("task detail: %v", err)
			}
			if task.DispatchReadiness.Ready != tc.wantReady {
				t.Fatalf("ready = %v, want %v (blockers = %#v)", task.DispatchReadiness.Ready, tc.wantReady, task.DispatchReadiness.BlockReasons)
			}
			if tc.wantBlockCode != "" && !hasBlockReason(task.DispatchReadiness.BlockReasons, tc.wantBlockCode) {
				t.Fatalf("missing expected block %q, blockers = %#v", tc.wantBlockCode, task.DispatchReadiness.BlockReasons)
			}
			for _, forbidden := range tc.forbidBlockCodes {
				if hasBlockReason(task.DispatchReadiness.BlockReasons, forbidden) {
					t.Fatalf("block %q should not be emitted while an idle worktree remains, blockers = %#v", forbidden, task.DispatchReadiness.BlockReasons)
				}
			}
		})
	}
}

// TestDispatchGateRefusesOnEmptyPoolThenAdmitsAfterIdleFrees proves the sequence: a
// dispatch is refused while the pool is empty, then admitted once an idle worktree
// frees (created or ejected back to idle).
func TestDispatchGateRefusesOnEmptyPoolThenAdmitsAfterIdleFrees(t *testing.T) {
	worktreeRoot := twoSiblingFixture(t)
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), newFakeRuntime())

	idle := 0
	service.idleWorktreeCount = func() (int, error) { return idle, nil }

	empty, err := service.Task(context.Background(), "Task-0002")
	if err != nil {
		t.Fatalf("task detail (empty pool): %v", err)
	}
	if empty.DispatchReadiness.Ready {
		t.Fatalf("dispatch should be refused while the pool is empty, blockers = %#v", empty.DispatchReadiness.BlockReasons)
	}
	if !hasBlockReason(empty.DispatchReadiness.BlockReasons, "no_idle_worktree") {
		t.Fatalf("expected no_idle_worktree, blockers = %#v", empty.DispatchReadiness.BlockReasons)
	}

	idle = 1 // a Create / eject freed an idle worktree
	freed, err := service.Task(context.Background(), "Task-0002")
	if err != nil {
		t.Fatalf("task detail (idle freed): %v", err)
	}
	if !freed.DispatchReadiness.Ready {
		t.Fatalf("dispatch should be admitted after an idle worktree frees, blockers = %#v", freed.DispatchReadiness.BlockReasons)
	}
	if hasBlockReason(freed.DispatchReadiness.BlockReasons, "no_idle_worktree") {
		t.Fatalf("no_idle_worktree should clear after an idle worktree frees, blockers = %#v", freed.DispatchReadiness.BlockReasons)
	}
}

// TestDispatchGateNeverEmitsActiveRunExistsForSiblingWhileIdleFree locks that the legacy
// per-task active_run_exists block is gone for a same-repo sibling dispatch while an idle
// worktree remains: Task-0001 owns a lane (drawn from the pool) and Task-0002 still
// dispatch-ready against the remaining idle worktree.
func TestDispatchGateNeverEmitsActiveRunExistsForSiblingWhileIdleFree(t *testing.T) {
	service := newSiblingPoolService(t)
	// Seed two idle pool worktrees so both siblings can draw one.
	if _, err := service.CreatePoolWorktree("repo"); err != nil {
		t.Fatalf("create worktree #1: %v", err)
	}
	if _, err := service.CreatePoolWorktree("repo"); err != nil {
		t.Fatalf("create worktree #2: %v", err)
	}

	if _, err := service.Dispatch(context.Background(), "Task-0001"); err != nil {
		t.Fatalf("dispatch Task-0001: %v", err)
	}

	sibling, err := service.Task(context.Background(), "Task-0002")
	if err != nil {
		t.Fatalf("sibling task detail: %v", err)
	}
	if !sibling.DispatchReadiness.Ready {
		t.Fatalf("sibling dispatch should be ready while an idle worktree remains, blockers = %#v", sibling.DispatchReadiness.BlockReasons)
	}
	if hasBlockReason(sibling.DispatchReadiness.BlockReasons, "active_run_exists") {
		t.Fatalf("active_run_exists must not block a same-repo sibling while an idle worktree remains, blockers = %#v", sibling.DispatchReadiness.BlockReasons)
	}
	if !sibling.Actions[ActionDispatch].Allowed {
		t.Fatal("sibling dispatch action should be allowed while an idle worktree remains")
	}
}

// TestSameTaskReDispatchStaysBlockedWhileOwningLiveStory ensures a single task that
// already owns the live story is still blocked from a duplicate dispatch
// (task_already_running, not active_run_exists).
func TestSameTaskReDispatchStaysBlockedWhileOwningLiveStory(t *testing.T) {
	service := newSiblingPoolService(t)
	if _, err := service.CreatePoolWorktree("repo"); err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	if _, err := service.Dispatch(context.Background(), "Task-0001"); err != nil {
		t.Fatalf("dispatch Task-0001: %v", err)
	}

	same, err := service.Task(context.Background(), "Task-0001")
	if err != nil {
		t.Fatalf("task detail: %v", err)
	}
	if same.Actions[ActionDispatch].Allowed {
		t.Fatal("a task already owning the live story must not be dispatchable again")
	}
	if !hasBlockReason(same.LatestRun.StateEnvelope.ActionBlockReasons[ActionDispatch], "active_run_exists") &&
		!hasBlockReason(same.DispatchReadiness.BlockReasons, "task_already_running") {
		t.Fatalf("expected the owning task's dispatch to stay blocked, blockers = %#v / %#v",
			same.DispatchReadiness.BlockReasons, same.LatestRun.StateEnvelope.ActionBlockReasons[ActionDispatch])
	}
}

// TestTwoSiblingsDrawDistinctIdlePoolWorktrees is the pool-model A2.1: two distinct
// sibling tasks each DRAW their own idle pool worktree (no fresh dir) and BOTH checkouts
// are bound simultaneously, leaving zero idle worktrees.
func TestTwoSiblingsDrawDistinctIdlePoolWorktrees(t *testing.T) {
	service := newSiblingPoolService(t)
	wt1, err := service.CreatePoolWorktree("repo")
	if err != nil {
		t.Fatalf("create worktree #1: %v", err)
	}
	wt2, err := service.CreatePoolWorktree("repo")
	if err != nil {
		t.Fatalf("create worktree #2: %v", err)
	}

	run1, err := service.Dispatch(context.Background(), "Task-0001")
	if err != nil {
		t.Fatalf("dispatch Task-0001: %v", err)
	}
	run2, err := service.Dispatch(context.Background(), "Task-0002")
	if err != nil {
		t.Fatalf("dispatch Task-0002: %v", err)
	}

	// Each run reused one of the EXISTING idle pool checkouts (no fresh dir).
	drawn := map[string]bool{wt1.WorktreePath: false, wt2.WorktreePath: false}
	if _, ok := drawn[run1.RepoLane.OwnedRepoRoot]; !ok {
		t.Fatalf("Task-0001 owned root %q is not one of the pre-created pool worktrees", run1.RepoLane.OwnedRepoRoot)
	}
	if _, ok := drawn[run2.RepoLane.OwnedRepoRoot]; !ok {
		t.Fatalf("Task-0002 owned root %q is not one of the pre-created pool worktrees", run2.RepoLane.OwnedRepoRoot)
	}
	if run1.RepoLane.OwnedRepoRoot == run2.RepoLane.OwnedRepoRoot {
		t.Fatalf("siblings must draw distinct worktrees, both = %q", run1.RepoLane.OwnedRepoRoot)
	}
	if _, err := os.Stat(run1.RepoLane.OwnedRepoRoot); err != nil {
		t.Fatalf("Task-0001 drawn checkout missing: %v", err)
	}
	if _, err := os.Stat(run2.RepoLane.OwnedRepoRoot); err != nil {
		t.Fatalf("Task-0002 drawn checkout missing: %v", err)
	}

	// Both pool worktrees are now allocated -> zero idle (the cap by construction).
	idle, err := service.countIdlePoolWorktrees()
	if err != nil {
		t.Fatalf("count idle pool worktrees: %v", err)
	}
	if idle != 0 {
		t.Fatalf("idle worktree count = %d, want 0 (both drawn)", idle)
	}
}

// TestDispatchDrawsExistingIdleWorktreeNoFreshDir is the AC3 falsifier guard: Assign /
// dispatch reuses an EXISTING idle pool worktree and never provisions a fresh dir or
// grows the pool count.
func TestDispatchDrawsExistingIdleWorktreeNoFreshDir(t *testing.T) {
	service := newSiblingPoolService(t)
	created, err := service.CreatePoolWorktree("repo")
	if err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	poolBefore, _ := service.ListPoolWorktrees()

	run, err := service.Dispatch(context.Background(), "Task-0001")
	if err != nil {
		t.Fatalf("dispatch Task-0001: %v", err)
	}
	if run.RepoLane.OwnedRepoRoot != created.WorktreePath {
		t.Fatalf("dispatch drew %q, want the pre-created idle worktree %q (no fresh dir)", run.RepoLane.OwnedRepoRoot, created.WorktreePath)
	}
	poolAfter, _ := service.ListPoolWorktrees()
	if len(poolAfter) != len(poolBefore) {
		t.Fatalf("pool count grew from %d to %d on dispatch (must reuse, not provision)", len(poolBefore), len(poolAfter))
	}
	if len(poolAfter) != 1 || poolAfter[0].Status != poolStatusAllocated {
		t.Fatalf("the drawn worktree should be allocated, pool = %#v", poolAfter)
	}
}

// TestDispatchRefusedWhenPoolEmpty is the AC4/AC5 guard at the service level: with no
// idle worktree, Dispatch is refused (ErrNoIdleWorktree) and no run starts.
func TestDispatchRefusedWhenPoolEmpty(t *testing.T) {
	service := newSiblingPoolService(t)
	// No CreatePoolWorktree: the pool is empty.
	_, err := service.Dispatch(context.Background(), "Task-0001")
	if err == nil {
		t.Fatal("dispatch into an empty pool must be refused, got nil error")
	}
}

// TestEjectKeepsFolderReturnsIdleAndDequeues is the AC6/AC14 service proof: Eject of an
// allocated worktree KEEPS the folder, returns it idle (run_id cleared), and DEQUEUES the
// freed task through the provider (Queue -> Never) WITHOUT closing the issue. A test that
// finds the folder deleted, or that finds the dequeue not called, or that finds the issue
// closed, fails.
func TestEjectKeepsFolderReturnsIdleAndDequeues(t *testing.T) {
	service := newSiblingPoolService(t)
	dequeue := &fakeDequeueProvider{}
	service.SetDequeueProvider(dequeue)

	created := seedThroughCreate(t, service)
	run, err := service.Dispatch(context.Background(), "Task-0001")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if _, err := os.Stat(created.WorktreePath); err != nil {
		t.Fatalf("allocated checkout should exist: %v", err)
	}

	ejected, err := service.EjectWorktree(context.Background(), run.RunID, "")
	if err != nil {
		t.Fatalf("eject: %v", err)
	}
	// Folder is KEPT (NOT deleted) and now idle.
	if _, err := os.Stat(created.WorktreePath); err != nil {
		t.Fatalf("Eject must KEEP the folder, but the checkout is gone: %v", err)
	}
	if ejected.Status != poolStatusIdle {
		t.Fatalf("ejected worktree status = %q, want idle", ejected.Status)
	}
	rec, ok, err := readPoolRecord(service.poolMemberDir(1))
	if err != nil || !ok {
		t.Fatalf("read pool record after eject: ok=%v err=%v", ok, err)
	}
	if rec.RunID != "" {
		t.Fatalf("ejected pool record run_id = %q, want empty (idle)", rec.RunID)
	}
	// Dequeued the freed task (Task-0001 -> issue #1) through the provider; never closed.
	if len(dequeue.calls) != 1 || dequeue.calls[0].number != 1 {
		t.Fatalf("dequeue calls = %#v, want one dequeue for issue #1 (Task-0001)", dequeue.calls)
	}
}

// TestEjectWorksWhileParked proves Eject works regardless of parked state (it does not
// require a parked run, unlike resolve-interrupt-review which only reclaims parked).
func TestEjectWorksWhileParked(t *testing.T) {
	service := newSiblingPoolService(t)
	service.SetDequeueProvider(&fakeDequeueProvider{})
	created := seedThroughCreate(t, service)
	run, err := service.Dispatch(context.Background(), "Task-0001")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	// Park the lane (Human Needed=Yes), as the consumer would.
	if _, err := service.SetRunGateState("Task-0001", RunGateStateParkedAwaitingClosure); err != nil {
		t.Fatalf("park lane: %v", err)
	}
	ejected, err := service.EjectWorktree(context.Background(), run.RunID, "")
	if err != nil {
		t.Fatalf("eject parked lane: %v", err)
	}
	if ejected.Status != poolStatusIdle {
		t.Fatalf("ejected parked worktree status = %q, want idle", ejected.Status)
	}
	if _, err := os.Stat(created.WorktreePath); err != nil {
		t.Fatalf("Eject of a parked lane must KEEP the folder: %v", err)
	}
}

// seedThroughCreate provisions one idle pool worktree via the real CreatePoolWorktree on a
// sibling pool service (declared root is a real git repo, owned-lane root isolated).
func seedThroughCreate(t *testing.T, service *Service) PoolWorktree {
	t.Helper()
	wt, err := service.CreatePoolWorktree("repo")
	if err != nil {
		t.Fatalf("create pool worktree: %v", err)
	}
	return wt
}
