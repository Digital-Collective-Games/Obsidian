package taskrun

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// BUG-0003 Step 3 (Fix A): per-repo accounting must be repo-scoped. Two repos' lanes
// share one runs-root and the same Task-0001 id; repo A's Service must NOT see repo B's
// lane as active (that was the path that reclaimed/killed the wrong agent), while the
// global GET /api/v1/worktrees view (ListActiveWorktrees) must still report BOTH.
func TestActiveOwnedLaneAccountingIsRepoScoped(t *testing.T) {
	base := t.TempDir()
	runsRoot := filepath.Join(base, "runs")
	repoA := filepath.Join(base, "RepoA")
	repoB := filepath.Join(base, "RepoB")
	// Real on-disk worktree dirs so the os.Stat liveness check passes for both.
	wtA := filepath.Join(base, "cdxow", "Task-0001-a", "w")
	wtB := filepath.Join(base, "cdxow", "Task-0001-b", "w")
	for _, d := range []string{repoA, repoB, wtA, wtB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	writeLaneRecord(t, runsRoot, "Task-0001", "taskrun--RepoA--Task-0001--active", repoA, wtA, time.Unix(100, 0))
	writeLaneRecord(t, runsRoot, "Task-0001", "taskrun--RepoB--Task-0001--active", repoB, wtB, time.Unix(200, 0))

	svcA := NewServiceForRepo(repoA, runsRoot, nil)
	svcB := NewServiceForRepo(repoB, runsRoot, nil)

	tasksA, err := svcA.ActiveOwnedLaneTasks()
	if err != nil {
		t.Fatalf("repoA ActiveOwnedLaneTasks: %v", err)
	}
	tasksB, err := svcB.ActiveOwnedLaneTasks()
	if err != nil {
		t.Fatalf("repoB ActiveOwnedLaneTasks: %v", err)
	}
	if len(tasksA) != 1 || tasksA[0] != "Task-0001" {
		t.Fatalf("repoA active tasks = %v, want exactly [Task-0001] (its own lane only)", tasksA)
	}
	if len(tasksB) != 1 || tasksB[0] != "Task-0001" {
		t.Fatalf("repoB active tasks = %v, want exactly [Task-0001] (its own lane only)", tasksB)
	}

	// findActiveLaneRecord (Set/Bind/Reclaim/ClosureRequested) must also be repo-scoped:
	// repoA must resolve ITS worktree, not repoB's.
	_, recA, err := svcA.findActiveLaneRecord("Task-0001")
	if err != nil {
		t.Fatalf("repoA findActiveLaneRecord: %v", err)
	}
	if !sameRepoRoot(recA.DeclaredWorktreeRoot, repoA) || recA.OwnedRepoRoot != wtA {
		t.Fatalf("repoA resolved the WRONG repo's lane: declaredRoot=%s owned=%s (want repoA/%s)", recA.DeclaredWorktreeRoot, recA.OwnedRepoRoot, wtA)
	}
	_, recB, err := svcB.findActiveLaneRecord("Task-0001")
	if err != nil {
		t.Fatalf("repoB findActiveLaneRecord: %v", err)
	}
	if !sameRepoRoot(recB.DeclaredWorktreeRoot, repoB) || recB.OwnedRepoRoot != wtB {
		t.Fatalf("repoB resolved the WRONG repo's lane: declaredRoot=%s owned=%s (want repoB/%s)", recB.DeclaredWorktreeRoot, recB.OwnedRepoRoot, wtB)
	}

	// The GLOBAL endpoint view must still report BOTH repos' lanes.
	all, err := svcA.ListActiveWorktrees()
	if err != nil {
		t.Fatalf("ListActiveWorktrees: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("global ListActiveWorktrees = %d lanes, want 2 (endpoint stays global)", len(all))
	}
}

func writeLaneRecord(t *testing.T, runsRoot, taskID, runID, declaredRoot, ownedRoot string, bootstrappedAt time.Time) {
	t.Helper()
	dir := filepath.Join(runsRoot, "taskruns", sanitizePathSegment(taskID), sanitizePathSegment(runID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir record dir: %v", err)
	}
	rec := ownedLaneBootstrapRecord{
		TaskID:               taskID,
		RunID:                runID,
		OwnedRepoRoot:        ownedRoot,
		DeclaredWorktreeRoot: declaredRoot,
		BootstrappedAt:       bootstrappedAt,
		Binding: &RepoBinding{
			Repo:         declaredRoot,
			TaskID:       taskID,
			WorktreePath: ownedRoot,
			RunGateState: RunGateStateRunning,
		},
	}
	if err := writeJSONFile(filepath.Join(dir, "owned-lane-bootstrap.json"), rec); err != nil {
		t.Fatalf("write lane record: %v", err)
	}
}
