package taskrun

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// O6 PASS-0002: the owned-lane record carries the worktree<->session binding
// after dispatch, and ListActiveWorktrees enumerates it.

func TestDispatchPopulatesOwnedLaneBinding(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD: `# Task 0008

## Title

Build the backend task dispatch layer.

## Summary

Create the durable backend task-run contract so later clients do not guess state.
`,
			taskState: `{
  "task_id": "Task-0008",
  "status": "in_progress",
  "phase": "implementation",
  "plan_approved": true,
  "current_pass": "PASS-0001",
  "current_gate": "implementation",
  "blockers": [],
  "updated_at": "2026-04-24T16:44:31-04:00"
}`,
			planMD: "# approved plan\n",
		},
	})

	// A manifest entry matching the declared worktree root lets repoIdentity
	// resolve a stable repo id for the binding.
	writeFile(t, filepath.Join(worktreeRoot, "REPO-MANIFEST.json"), `{
  "repos": [
    {"id": "TestRepo", "local_root": "`+filepath.ToSlash(worktreeRoot)+`", "queue_workers": 4}
  ]
}`)

	// captureDispatchContext reads these env vars; on this pass they are the
	// best-available (backend dispatch-context) session/transcript placeholders.
	t.Setenv("CODEX_SESSION_ID", "session-abc123")
	t.Setenv("CODEX_TRANSCRIPT_PATH", filepath.Join(worktreeRoot, "transcript.jsonl"))

	runtime := newFakeRuntime()
	runsRoot := filepath.Join(worktreeRoot, ".runs")
	service := NewService(worktreeRoot, runsRoot, runtime)

	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	// The returned RepoLane carries the binding.
	binding := run.RepoLane.Binding
	if binding == nil {
		t.Fatal("expected RepoLane.Binding to be populated after dispatch")
	}
	if binding.Repo != "TestRepo" {
		t.Fatalf("binding repo = %q, want TestRepo", binding.Repo)
	}
	if binding.TaskID != "Task-0008" {
		t.Fatalf("binding task id = %q, want Task-0008", binding.TaskID)
	}
	if binding.WorktreePath != run.RepoLane.OwnedRepoRoot {
		t.Fatalf("binding worktree path = %q, want owned repo root %q", binding.WorktreePath, run.RepoLane.OwnedRepoRoot)
	}
	if binding.AgentSessionID != "session-abc123" {
		t.Fatalf("binding agent session id = %q, want session-abc123", binding.AgentSessionID)
	}
	if binding.SessionTranscriptPath != filepath.Join(worktreeRoot, "transcript.jsonl") {
		t.Fatalf("binding transcript path = %q", binding.SessionTranscriptPath)
	}
	if binding.RunGateState != RunGateStateRunning {
		t.Fatalf("binding run/gate state = %q, want %q", binding.RunGateState, RunGateStateRunning)
	}

	// The durable owned-lane-bootstrap.json record carries the same binding.
	rawBootstrap, err := os.ReadFile(run.RepoLane.BootstrapArtifactPath)
	if err != nil {
		t.Fatalf("read bootstrap artifact: %v", err)
	}
	var record ownedLaneBootstrapRecord
	if err := json.Unmarshal(rawBootstrap, &record); err != nil {
		t.Fatalf("decode bootstrap artifact: %v", err)
	}
	if record.Binding == nil {
		t.Fatal("expected durable bootstrap record to carry the binding")
	}
	if record.Binding.TaskID != "Task-0008" || record.Binding.WorktreePath != run.RepoLane.OwnedRepoRoot {
		t.Fatalf("durable binding = %#v", record.Binding)
	}
	if record.Binding.AgentSessionID != "session-abc123" {
		t.Fatalf("durable binding session id = %q", record.Binding.AgentSessionID)
	}
}

func TestListActiveWorktreesReturnsBinding(t *testing.T) {
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
	t.Setenv("CODEX_SESSION_ID", "session-xyz789")
	t.Setenv("CODEX_TRANSCRIPT_PATH", filepath.Join(worktreeRoot, "session.jsonl"))

	runtime := newFakeRuntime()
	runsRoot := filepath.Join(worktreeRoot, ".runs")
	service := NewService(worktreeRoot, runsRoot, runtime)

	if _, err := service.Dispatch(context.Background(), "Task-0008"); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	worktrees, err := service.ListActiveWorktrees()
	if err != nil {
		t.Fatalf("list active worktrees: %v", err)
	}
	if len(worktrees) != 1 {
		t.Fatalf("active worktrees = %d, want 1: %#v", len(worktrees), worktrees)
	}
	wt := worktrees[0]
	if wt.TaskID != "Task-0008" {
		t.Fatalf("worktree task id = %q, want Task-0008", wt.TaskID)
	}
	if wt.Repo != "TestRepo" {
		t.Fatalf("worktree repo = %q, want TestRepo", wt.Repo)
	}
	if wt.AgentSessionID != "session-xyz789" {
		t.Fatalf("worktree session id = %q, want session-xyz789", wt.AgentSessionID)
	}
	if wt.SessionTranscriptPath != filepath.Join(worktreeRoot, "session.jsonl") {
		t.Fatalf("worktree transcript path = %q", wt.SessionTranscriptPath)
	}
	if wt.RunGateState != RunGateStateRunning {
		t.Fatalf("worktree run/gate state = %q, want %q", wt.RunGateState, RunGateStateRunning)
	}
	if wt.WorktreePath == "" {
		t.Fatal("worktree path should be set for VSCodium link construction")
	}
	if wt.RunID == "" {
		t.Fatal("worktree should expose its run id")
	}
}

// A reclaimed (cleaned-up) worktree drops out of the active listing because its
// checkout directory no longer exists.
func TestListActiveWorktreesSkipsReclaimedWorktree(t *testing.T) {
	worktreeRoot := writeGitTaskTrackingRoot(t, map[string]taskFixture{
		"Task-0008": {
			taskMD:    "# Task 0008\n\n## Title\n\nBuild the backend task dispatch layer.\n\n## Summary\n\nDurable contract.\n",
			taskState: `{"task_id":"Task-0008","status":"in_progress","phase":"implementation","plan_approved":true,"current_pass":"PASS-0001","current_gate":"implementation","blockers":[],"updated_at":"2026-04-24T16:44:31-04:00"}`,
			planMD:    "# approved plan\n",
		},
	})

	runtime := newFakeRuntime()
	runsRoot := filepath.Join(worktreeRoot, ".runs")
	service := NewService(worktreeRoot, runsRoot, runtime)

	run, err := service.Dispatch(context.Background(), "Task-0008")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	worktrees, err := service.ListActiveWorktrees()
	if err != nil {
		t.Fatalf("list active worktrees: %v", err)
	}
	if len(worktrees) != 1 {
		t.Fatalf("active worktrees before cleanup = %d, want 1", len(worktrees))
	}

	if err := service.cleanupOwnedLane(run.RepoLane); err != nil {
		t.Fatalf("cleanup owned lane: %v", err)
	}

	worktrees, err = service.ListActiveWorktrees()
	if err != nil {
		t.Fatalf("list active worktrees after cleanup: %v", err)
	}
	if len(worktrees) != 0 {
		t.Fatalf("active worktrees after cleanup = %d, want 0: %#v", len(worktrees), worktrees)
	}
}
