package taskrun

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// twoSiblingFixture builds a git tracking root with two distinct, dispatch-ready
// sibling tasks so the per-repo slot gate can be exercised with N>1 lanes.
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

// TestDispatchGateAdmitsSiblingsWhileSlotsRemainAndRefusesWhenFull table-drives
// the relaxed per-repo gate over a stub slot count (A2.2/A2.3): with a free slot,
// no active_run_exists / repo_slots_exhausted block is emitted; once the repo's
// queue_workers slots are all used, dispatch is refused with repo_slots_exhausted.
func TestDispatchGateAdmitsSiblingsWhileSlotsRemainAndRefusesWhenFull(t *testing.T) {
	worktreeRoot := twoSiblingFixture(t)

	cases := []struct {
		name             string
		limit            int
		used             int
		wantReady        bool
		wantBlockCode    string
		forbidBlockCodes []string
	}{
		{name: "empty repo admits first lane", limit: 4, used: 0, wantReady: true, forbidBlockCodes: []string{"active_run_exists", "repo_slots_exhausted"}},
		{name: "one sibling active still admits second", limit: 4, used: 1, wantReady: true, forbidBlockCodes: []string{"active_run_exists", "repo_slots_exhausted"}},
		{name: "three siblings active still admits fourth", limit: 4, used: 3, wantReady: true, forbidBlockCodes: []string{"active_run_exists", "repo_slots_exhausted"}},
		{name: "fifth dispatch refused when four slots full", limit: 4, used: 4, wantReady: false, wantBlockCode: "repo_slots_exhausted"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime := newFakeRuntime()
			service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
			limit := tc.limit
			used := tc.used
			service.repoSlotLimit = func() int { return limit }
			service.countActiveOwnedLanes = func() (int, error) { return used, nil }

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
					t.Fatalf("block %q should not be emitted while a free slot remains, blockers = %#v", forbidden, task.DispatchReadiness.BlockReasons)
				}
			}
		})
	}
}

// TestDispatchGateRefusesFifthThenAdmitsAfterSlotFrees proves the A2.2 sequence:
// a dispatch is refused while slots are full, then admitted once a slot frees.
func TestDispatchGateRefusesFifthThenAdmitsAfterSlotFrees(t *testing.T) {
	worktreeRoot := twoSiblingFixture(t)
	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)

	used := 4
	service.repoSlotLimit = func() int { return 4 }
	service.countActiveOwnedLanes = func() (int, error) { return used, nil }

	full, err := service.Task(context.Background(), "Task-0002")
	if err != nil {
		t.Fatalf("task detail (full): %v", err)
	}
	if full.DispatchReadiness.Ready {
		t.Fatalf("dispatch should be refused while all slots are full, blockers = %#v", full.DispatchReadiness.BlockReasons)
	}
	if !hasBlockReason(full.DispatchReadiness.BlockReasons, "repo_slots_exhausted") {
		t.Fatalf("expected repo_slots_exhausted, blockers = %#v", full.DispatchReadiness.BlockReasons)
	}

	used = 3 // a terminal close freed one slot
	freed, err := service.Task(context.Background(), "Task-0002")
	if err != nil {
		t.Fatalf("task detail (freed): %v", err)
	}
	if !freed.DispatchReadiness.Ready {
		t.Fatalf("dispatch should be admitted after a slot frees, blockers = %#v", freed.DispatchReadiness.BlockReasons)
	}
	if hasBlockReason(freed.DispatchReadiness.BlockReasons, "repo_slots_exhausted") {
		t.Fatalf("repo_slots_exhausted should clear after a slot frees, blockers = %#v", freed.DispatchReadiness.BlockReasons)
	}
}

// TestDispatchGateNeverEmitsActiveRunExistsForSiblingWhileSlotFree locks A2.3:
// the legacy per-task active_run_exists block reason is gone for a same-repo
// dispatch while a free slot remains.
func TestDispatchGateNeverEmitsActiveRunExistsForSiblingWhileSlotFree(t *testing.T) {
	worktreeRoot := twoSiblingFixture(t)
	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)

	// Task-0001 is dispatched and owns a live story (occupying one slot).
	if _, err := service.Dispatch(context.Background(), "Task-0001"); err != nil {
		t.Fatalf("dispatch Task-0001: %v", err)
	}

	// Task-0002 reads with one sibling slot used and three free.
	service.repoSlotLimit = func() int { return 4 }
	service.countActiveOwnedLanes = func() (int, error) { return 1, nil }

	sibling, err := service.Task(context.Background(), "Task-0002")
	if err != nil {
		t.Fatalf("sibling task detail: %v", err)
	}
	if !sibling.DispatchReadiness.Ready {
		t.Fatalf("sibling dispatch should be ready while a slot is free, blockers = %#v", sibling.DispatchReadiness.BlockReasons)
	}
	if hasBlockReason(sibling.DispatchReadiness.BlockReasons, "active_run_exists") {
		t.Fatalf("active_run_exists must not block a same-repo sibling while a slot is free, blockers = %#v", sibling.DispatchReadiness.BlockReasons)
	}
	if !sibling.Actions[ActionDispatch].Allowed {
		t.Fatal("sibling dispatch action should be allowed while a slot is free")
	}
}

// TestSameTaskReDispatchStaysBlockedWhileOwningLiveStory ensures the relaxation
// did not let a single task be dispatched twice: a task that already owns the
// live story is still blocked (with task_already_running, not active_run_exists).
func TestSameTaskReDispatchStaysBlockedWhileOwningLiveStory(t *testing.T) {
	worktreeRoot := twoSiblingFixture(t)
	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	service.repoSlotLimit = func() int { return 4 }
	service.countActiveOwnedLanes = func() (int, error) { return 1, nil }

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

// TestTwoSiblingsHoldConcurrentOwnedLanesInOneRepo is the unit-level A2.1: two
// distinct sibling tasks in ONE repo each provision their own owned worktree and
// BOTH checkouts exist on disk simultaneously, with the live worktree count = 2.
func TestTwoSiblingsHoldConcurrentOwnedLanesInOneRepo(t *testing.T) {
	worktreeRoot := twoSiblingFixture(t)
	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)

	run1, err := service.Dispatch(context.Background(), "Task-0001")
	if err != nil {
		t.Fatalf("dispatch Task-0001: %v", err)
	}
	run2, err := service.Dispatch(context.Background(), "Task-0002")
	if err != nil {
		t.Fatalf("dispatch Task-0002: %v", err)
	}

	if run1.RepoLane.OwnedRepoRoot == "" || run2.RepoLane.OwnedRepoRoot == "" {
		t.Fatalf("both siblings must own a worktree, got %q and %q", run1.RepoLane.OwnedRepoRoot, run2.RepoLane.OwnedRepoRoot)
	}
	if run1.RepoLane.OwnedRepoRoot == run2.RepoLane.OwnedRepoRoot {
		t.Fatalf("siblings must own distinct worktrees, both = %q", run1.RepoLane.OwnedRepoRoot)
	}
	if _, err := os.Stat(run1.RepoLane.OwnedRepoRoot); err != nil {
		t.Fatalf("Task-0001 owned checkout missing: %v", err)
	}
	if _, err := os.Stat(run2.RepoLane.OwnedRepoRoot); err != nil {
		t.Fatalf("Task-0002 owned checkout missing: %v", err)
	}

	count, err := service.countOwnedLaneWorktrees()
	if err != nil {
		t.Fatalf("count owned-lane worktrees: %v", err)
	}
	if count != 2 {
		t.Fatalf("live owned-lane worktree count = %d, want 2", count)
	}
}

// TestReleasePreviousOwnedLaneDoesNotTearDownSiblingLane is the F-O2 guard:
// dispatching/re-dispatching one task must never remove a same-repo sibling's
// worktree. releasePreviousOwnedLane only ever acts on the SAME task's superseded
// run, so the sibling's checkout survives.
func TestReleasePreviousOwnedLaneDoesNotTearDownSiblingLane(t *testing.T) {
	worktreeRoot := twoSiblingFixture(t)
	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)

	run1, err := service.Dispatch(context.Background(), "Task-0001")
	if err != nil {
		t.Fatalf("dispatch Task-0001: %v", err)
	}
	siblingRoot := run1.RepoLane.OwnedRepoRoot

	// Dispatching Task-0002 runs releasePreviousOwnedLane(Task-0002) first.
	run2, err := service.Dispatch(context.Background(), "Task-0002")
	if err != nil {
		t.Fatalf("dispatch Task-0002: %v", err)
	}

	if _, err := os.Stat(siblingRoot); err != nil {
		t.Fatalf("sibling Task-0001 worktree must survive Task-0002 dispatch, stat err = %v", err)
	}
	if _, err := os.Stat(run2.RepoLane.OwnedRepoRoot); err != nil {
		t.Fatalf("Task-0002 worktree missing: %v", err)
	}

	// Directly invoking the per-task release for Task-0002 must also leave the
	// Task-0001 sibling lane untouched.
	if err := service.releasePreviousOwnedLane(context.Background(), "Task-0002"); err != nil {
		t.Fatalf("releasePreviousOwnedLane(Task-0002): %v", err)
	}
	if _, err := os.Stat(siblingRoot); err != nil {
		t.Fatalf("sibling Task-0001 worktree must survive a Task-0002 release, stat err = %v", err)
	}
}
