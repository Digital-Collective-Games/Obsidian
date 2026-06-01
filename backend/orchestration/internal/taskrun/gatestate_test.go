package taskrun

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/queue"
)

// O4 PASS-0003: SetRunGateState records the parked run/gate state on the active
// owned-lane record, and a parked worktree is still surfaced by ListActiveWorktrees
// (re-proves O6 A6.4 at the code level: a parked needs-human worktree stays listed).
func TestSetRunGateStateParksAndStaysListed(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD:    "# Task 0008\n\n## Title\n\nBuild the backend task dispatch layer.\n\n## Summary\n\nDurable contract.\n",
			taskState: `{"task_id":"Task-0008","status":"in_progress","phase":"implementation","plan_approved":true,"current_pass":"PASS-0001","current_gate":"implementation","blockers":[],"updated_at":"2026-04-24T16:44:31-04:00"}`,
			planMD:    "# approved plan\n",
		},
	})
	writeFile(t, filepath.Join(worktreeRoot, "REPO-MANIFEST.json"), `{
  "repos": [
    {"id": "TestRepo", "local_root": "`+filepath.ToSlash(worktreeRoot)+`", "queue_workers": 4}
  ]
}`)

	runtime := newFakeRuntime()
	runsRoot := filepath.Join(worktreeRoot, ".runs")
	service := NewService(worktreeRoot, runsRoot, runtime)

	seedIdleWorktree(t, service)
	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	// Freshly dispatched lane is running.
	worktrees, err := service.ListActiveWorktrees()
	if err != nil {
		t.Fatalf("list active worktrees: %v", err)
	}
	if len(worktrees) != 1 || worktrees[0].RunGateState != RunGateStateRunning {
		t.Fatalf("expected one running worktree, got %#v", worktrees)
	}

	// Park it awaiting closure approval (Human Needed=Yes; perceived completion).
	updated, err := service.SetRunGateState("Task-0008", RunGateStateParkedAwaitingClosure)
	if err != nil {
		t.Fatalf("set run/gate state: %v", err)
	}
	if updated.RunGateState != RunGateStateParkedAwaitingClosure {
		t.Fatalf("returned binding run/gate state = %q, want %q", updated.RunGateState, RunGateStateParkedAwaitingClosure)
	}
	if updated.TaskID != "Task-0008" || updated.WorktreePath != run.RepoLane.OwnedRepoRoot {
		t.Fatalf("returned binding lost identity: %#v", updated)
	}

	// Parking does NOT deallocate: the worktree is still listed, now parked, and the
	// transition persisted durably (a fresh list re-reads the record from disk).
	parked, err := service.ListActiveWorktrees()
	if err != nil {
		t.Fatalf("list active worktrees after park: %v", err)
	}
	if len(parked) != 1 {
		t.Fatalf("parked worktree should remain listed (retain worktree+slot), got %d: %#v", len(parked), parked)
	}
	if parked[0].RunGateState != RunGateStateParkedAwaitingClosure {
		t.Fatalf("listed run/gate state = %q, want %q (A6.4 parked listing)", parked[0].RunGateState, RunGateStateParkedAwaitingClosure)
	}
	if parked[0].TaskID != "Task-0008" || parked[0].WorktreePath == "" || parked[0].RunID == "" {
		t.Fatalf("parked worktree must keep its binding for operator follow-up: %#v", parked[0])
	}

	// A gate park (research) is recorded as the matching state.
	if _, err := service.SetRunGateState("Task-0008", RunGateStateParkedResearch); err != nil {
		t.Fatalf("set research-gate park: %v", err)
	}
	research, err := service.ListActiveWorktrees()
	if err != nil {
		t.Fatalf("list after research park: %v", err)
	}
	if len(research) != 1 || research[0].RunGateState != RunGateStateParkedResearch {
		t.Fatalf("expected research-gate park listed, got %#v", research)
	}

	// And it can transition back to running once the gate clears (resume in place).
	if _, err := service.SetRunGateState("Task-0008", RunGateStateRunning); err != nil {
		t.Fatalf("resume to running: %v", err)
	}
}

// O5/O6 (coordinator-review correction 2): BindLaunchedSession replaces the
// dispatch-time placeholder session fields on the active lane binding with the
// POST-LAUNCH-discovered agent session id + transcript path, without changing the
// run/gate state or deallocating. This is what the consumer calls after the
// launcher discovers the launched agent's OWN session.
func TestBindLaunchedSessionReplacesPlaceholders(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD:    "# Task 0008\n\n## Title\n\nBuild the backend task dispatch layer.\n\n## Summary\n\nDurable contract.\n",
			taskState: `{"task_id":"Task-0008","status":"in_progress","phase":"implementation","plan_approved":true,"current_pass":"PASS-0001","current_gate":"implementation","blockers":[],"updated_at":"2026-04-24T16:44:31-04:00"}`,
			planMD:    "# approved plan\n",
		},
	})
	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	seedIdleWorktree(t, service)
	if _, err := service.Dispatch(context.Background(), "Task-0008"); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	const wantSession = "019e771f-94db-7c22-bd3a-cf13a11df3ff"
	const wantTranscript = `C:\Users\gregs\.codex\sessions\2026\05\30\rollout-019e771f.jsonl`
	const wantPID = 4242
	updated, err := service.BindLaunchedSession("Task-0008", wantSession, wantTranscript, wantPID)
	if err != nil {
		t.Fatalf("bind launched session: %v", err)
	}
	if updated.AgentSessionID != wantSession || updated.SessionTranscriptPath != wantTranscript {
		t.Fatalf("binding not updated with launched session: %#v", updated)
	}
	// BUG-0002: the launched PID is persisted so reclaim can terminate the agent.
	if updated.LaunchedPID != wantPID {
		t.Fatalf("binding LaunchedPID = %d, want %d", updated.LaunchedPID, wantPID)
	}
	// Run/gate state is untouched (still running) and the worktree is not deallocated.
	if updated.RunGateState != RunGateStateRunning {
		t.Fatalf("BindLaunchedSession changed run/gate state to %q", updated.RunGateState)
	}
	// The update persisted durably (a fresh list re-reads from disk).
	worktrees, err := service.ListActiveWorktrees()
	if err != nil {
		t.Fatalf("list after bind: %v", err)
	}
	if len(worktrees) != 1 || worktrees[0].AgentSessionID != wantSession || worktrees[0].SessionTranscriptPath != wantTranscript {
		t.Fatalf("launched-session binding did not persist: %#v", worktrees)
	}
	if worktrees[0].LaunchedPID != wantPID {
		t.Fatalf("launched-session binding LaunchedPID did not persist: got %d, want %d", worktrees[0].LaunchedPID, wantPID)
	}
}

func TestSetRunGateStateRejectsUnknownState(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD:    "# Task 0008\n\n## Title\n\nBuild the backend task dispatch layer.\n\n## Summary\n\nDurable contract.\n",
			taskState: `{"task_id":"Task-0008","status":"in_progress","phase":"implementation","plan_approved":true,"current_pass":"PASS-0001","current_gate":"implementation","blockers":[],"updated_at":"2026-04-24T16:44:31-04:00"}`,
			planMD:    "# approved plan\n",
		},
	})
	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	seedIdleWorktree(t, service)
	if _, err := service.Dispatch(context.Background(), "Task-0008"); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if _, err := service.SetRunGateState("Task-0008", "not_a_state"); err == nil {
		t.Fatal("expected unknown run/gate state to be rejected")
	}
}

func TestSetRunGateStateErrorsWithoutActiveLane(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD:    "# Task 0008\n\n## Title\n\nBuild the backend task dispatch layer.\n\n## Summary\n\nDurable contract.\n",
			taskState: `{"task_id":"Task-0008","status":"in_progress","phase":"implementation","plan_approved":true,"current_pass":"PASS-0001","current_gate":"implementation","blockers":[],"updated_at":"2026-04-24T16:44:31-04:00"}`,
			planMD:    "# approved plan\n",
		},
	})
	runtime := newFakeRuntime()
	service := NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), runtime)
	if _, err := service.SetRunGateState("Task-0008", RunGateStateParkedPlan); err == nil {
		t.Fatal("expected error when no active owned lane exists")
	}
}

// The internal/queue decision function records park states as plain strings (it is
// a leaf package and must not import taskrun). This guard asserts those strings
// match the taskrun run/gate constants so the two never drift, and that each maps
// to a recognized parked state.
func TestQueueParkStatesMatchTaskrunConstants(t *testing.T) {
	pairs := []struct {
		hint queue.GateHint
		want string
	}{
		{queue.GateHintAwaitingClosure, RunGateStateParkedAwaitingClosure},
		{queue.GateHintResearch, RunGateStateParkedResearch},
		{queue.GateHintPlan, RunGateStateParkedPlan},
		{queue.GateHintRegression, RunGateStateParkedRegression},
		{queue.GateHintNone, RunGateStateParkedAwaitingClosure},
	}
	for _, p := range pairs {
		got := queue.DecideQueueAction(queue.IssueState{HumanNeeded: true, GateHint: p.hint}).ParkState
		if got != p.want {
			t.Fatalf("queue park state for hint %q = %q, want taskrun constant %q", p.hint, got, p.want)
		}
		if !IsParkedRunGateState(got) {
			t.Fatalf("queue park state %q is not a recognized taskrun parked state", got)
		}
	}
}
