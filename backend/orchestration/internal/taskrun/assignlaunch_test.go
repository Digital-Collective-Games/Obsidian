package taskrun

import (
	"context"
	"strings"
	"testing"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/queue"
)

// fakeAgentLauncher records the launch spec and returns a fixed session WITHOUT spawning a
// real claude process, so the manual-Assign launch wiring is provable without a process.
type fakeAgentLauncher struct {
	calls   []queue.LaunchSpec
	session string
	path    string
	pid     int
	err     error
}

func (f *fakeAgentLauncher) Start(_ context.Context, spec queue.LaunchSpec, _ bool) (queue.LaunchResult, error) {
	f.calls = append(f.calls, spec)
	if f.err != nil {
		return queue.LaunchResult{}, f.err
	}
	return queue.LaunchResult{SessionID: f.session, TranscriptPath: f.path, PID: f.pid}, nil
}

// TestAssignLaunchesClaudeAgentAndBindsSession proves the manual Assign path launches a
// top-level claude agent IN the bound worktree and records its session on the binding, so
// the WORKTREES tab can open the running session in the editor (the launcher is a FAKE — no
// real process / token spend in the test).
func TestAssignLaunchesClaudeAgentAndBindsSession(t *testing.T) {
	service := newSiblingPoolService(t)
	launcher := &fakeAgentLauncher{
		session: "019e771f-94db-7c22-bd3a-cf13a11df3ff",
		path:    `C:\Users\gregs\.claude\projects\slug\019e771f-94db-7c22-bd3a-cf13a11df3ff.jsonl`,
		pid:     4242,
	}
	service.SetAgentLauncher(launcher, AgentLaunchConfig{
		Enabled:        true,
		AllowedTools:   "Read,Edit,Bash,Agent",
		PermissionMode: "bypassPermissions",
	})

	created, err := service.CreatePoolWorktree("repo")
	if err != nil {
		t.Fatalf("create worktree: %v", err)
	}

	run, err := service.AssignTaskToPoolWorktree(context.Background(), "Task-0001", "repo", created.WorktreeID)
	if err != nil {
		t.Fatalf("assign: %v", err)
	}

	// The agent was launched EXACTLY once, in the bound worktree, with the configured tools.
	if len(launcher.calls) != 1 {
		t.Fatalf("launcher called %d times, want exactly 1", len(launcher.calls))
	}
	spec := launcher.calls[0]
	if spec.WorktreePath != created.WorktreePath {
		t.Fatalf("launched in %q, want the bound worktree %q", spec.WorktreePath, created.WorktreePath)
	}
	if !strings.Contains(spec.Prompt, "Task-0001") {
		t.Fatalf("launch prompt does not mention the task: %q", spec.Prompt)
	}
	if spec.AllowedTools != "Read,Edit,Bash,Agent" || spec.PermissionMode != "bypassPermissions" {
		t.Fatalf("launch spec tools/mode = %q / %q", spec.AllowedTools, spec.PermissionMode)
	}

	// The launched session was bound onto the worktree, so GET /worktrees exposes it and the
	// dashboard can open vscodium://anthropic.claude-code/open?session=<id>.
	worktrees, err := service.ListActiveWorktrees()
	if err != nil {
		t.Fatalf("list active worktrees: %v", err)
	}
	if len(worktrees) != 1 || worktrees[0].AgentSessionID != launcher.session || worktrees[0].SessionTranscriptPath != launcher.path {
		t.Fatalf("launched session not bound onto the worktree: %#v", worktrees)
	}
	if run.RepoLane.OwnedRepoRoot != created.WorktreePath {
		t.Fatalf("assigned run worktree = %q, want %q", run.RepoLane.OwnedRepoRoot, created.WorktreePath)
	}
}

// TestAssignDoesNotLaunchWhenDisabled proves the legacy bind-only Assign is preserved:
// with launch disabled, Assign starts the run but launches NO agent (enabling launch is
// opt-in, and a Service with no launcher never spends tokens).
func TestAssignDoesNotLaunchWhenDisabled(t *testing.T) {
	service := newSiblingPoolService(t)
	launcher := &fakeAgentLauncher{session: "should-not-be-used"}
	service.SetAgentLauncher(launcher, AgentLaunchConfig{Enabled: false})

	created, err := service.CreatePoolWorktree("repo")
	if err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	if _, err := service.AssignTaskToPoolWorktree(context.Background(), "Task-0001", "repo", created.WorktreeID); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if len(launcher.calls) != 0 {
		t.Fatalf("launcher must NOT be called when disabled, got %d calls", len(launcher.calls))
	}
}
