package queue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// jsonStr JSON-encodes s so a Windows path's backslashes are properly escaped in
// the synthetic transcript fixtures (a raw "C:\A..." would be invalid JSON).
func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// buildAgentCommand must produce a TOP-LEVEL codex exec invocation (not a nested
// subagent) in the worktree, mirroring the jobexec precedent so the launched agent
// can spawn its own subagents (A5.1 / D3).
func TestBuildAgentCommandCodexIsTopLevelExec(t *testing.T) {
	exe, args, err := buildAgentCommand(LaunchSpec{
		Runtime:      RuntimeCodex,
		Executable:   `C:\codex.exe`,
		WorktreePath: `C:\Agent\QueueDrainTestbed\.owned\w`,
		Prompt:       "do the bounded task",
	})
	if err != nil {
		t.Fatalf("build codex command: %v", err)
	}
	if exe != `C:\codex.exe` {
		t.Fatalf("executable = %q", exe)
	}
	// It must be `codex exec ... -C <worktree> <prompt>` (a top-level process), and
	// must NOT contain any subagent/nested flag.
	if args[0] != "exec" {
		t.Fatalf("expected top-level `exec`, got args %v", args)
	}
	foundCwd := false
	for i, a := range args {
		if a == "-C" && i+1 < len(args) && args[i+1] == `C:\Agent\QueueDrainTestbed\.owned\w` {
			foundCwd = true
		}
	}
	if !foundCwd {
		t.Fatalf("command did not run in the owned worktree: %v", args)
	}
	if args[len(args)-1] != "do the bounded task" {
		t.Fatalf("prompt not last arg: %v", args)
	}
}

func TestBuildAgentCommandValidatesInputs(t *testing.T) {
	if _, _, err := buildAgentCommand(LaunchSpec{Runtime: RuntimeCodex, WorktreePath: "x"}); err == nil {
		t.Fatal("expected error on empty executable")
	}
	if _, _, err := buildAgentCommand(LaunchSpec{Runtime: RuntimeCodex, Executable: "x"}); err == nil {
		t.Fatal("expected error on empty worktree")
	}
	if _, _, err := buildAgentCommand(LaunchSpec{Runtime: "bogus", Executable: "x", WorktreePath: "y"}); err == nil {
		t.Fatal("expected error on unsupported runtime")
	}
}

// writeCodexRollout writes a synthetic codex rollout-*.jsonl with the given session
// id + cwd in its session_meta header line, and sets its mtime.
func writeCodexRollout(t *testing.T, dir, name, sessionID, cwd string, mtime time.Time) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	body := `{"timestamp":"2026-05-30T04:23:40.147Z","type":"session_meta","payload":{"id":` + jsonStr(sessionID) + `,"cwd":` + jsonStr(cwd) + `,"originator":"codex_exec","source":"exec"}}
{"timestamp":"2026-05-30T04:23:41.000Z","type":"event_msg","payload":{}}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return path
}

// POST-LAUNCH discovery (correction 2): the launched agent's OWN session id +
// transcript path are discovered from the sessions dir AFTER launch — the newest
// rollout created at/after the launch instant whose cwd matches the worktree. This
// is unit-tested against a SYNTHETIC sessions dir (no real agent).
func TestDiscoverCodexSessionPicksNewestAfterLaunchMatchingWorktree(t *testing.T) {
	root := t.TempDir()
	worktree := `C:\Agent\QueueDrainTestbed\.owned\w`
	launchedAt := time.Date(2026, 5, 30, 8, 0, 0, 0, time.UTC)

	day := filepath.Join(root, "2026", "05", "30")

	// (a) An OLD session in the same worktree, created BEFORE launch -> ignored.
	writeCodexRollout(t, day, "rollout-old.jsonl", "old-session", worktree, launchedAt.Add(-time.Hour))
	// (b) A session for a DIFFERENT cwd created after launch -> ignored.
	writeCodexRollout(t, day, "rollout-other.jsonl", "other-session", `C:\Some\Other\Dir`, launchedAt.Add(2*time.Minute))
	// (c) The launched agent's session: same worktree, created AFTER launch.
	wantPath := writeCodexRollout(t, day, "rollout-new.jsonl", "new-session-id", worktree, launchedAt.Add(1*time.Minute))
	// (d) An even-newer session in the same worktree (re-launch); newest wins.
	wantPath = writeCodexRollout(t, day, "rollout-newest.jsonl", "newest-session-id", worktree, launchedAt.Add(3*time.Minute))

	got, err := DiscoverSession(RuntimeCodex, root, worktree, launchedAt)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if got.SessionID != "newest-session-id" {
		t.Fatalf("session id = %q, want newest-session-id", got.SessionID)
	}
	if got.TranscriptPath != wantPath {
		t.Fatalf("transcript path = %q, want %q", got.TranscriptPath, wantPath)
	}
}

// When no session file appears after launch, discovery returns an error so the
// caller never binds an empty/placeholder session (it must surface the failure).
func TestDiscoverCodexSessionErrorsWhenNoneMatch(t *testing.T) {
	root := t.TempDir()
	worktree := `C:\Agent\QueueDrainTestbed\.owned\w`
	launchedAt := time.Date(2026, 5, 30, 8, 0, 0, 0, time.UTC)

	// Only a pre-launch session exists.
	writeCodexRollout(t, filepath.Join(root, "2026", "05", "30"), "rollout-old.jsonl", "old", worktree, launchedAt.Add(-time.Minute))

	if _, err := DiscoverSession(RuntimeCodex, root, worktree, launchedAt); err == nil {
		t.Fatal("expected an error when no post-launch session matched")
	}
}

// writeClaudeTranscript writes a synthetic claude <session>.jsonl with sessionId +
// cwd on every line, under a project-slug dir, and sets its mtime.
func writeClaudeTranscript(t *testing.T, slugDir, name, sessionID, cwd string, mtime time.Time) string {
	t.Helper()
	if err := os.MkdirAll(slugDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(slugDir, name)
	body := `{"type":"user","sessionId":` + jsonStr(sessionID) + `,"cwd":` + jsonStr(cwd) + `,"timestamp":"2026-05-30T08:01:00Z"}
{"type":"assistant","sessionId":` + jsonStr(sessionID) + `,"cwd":` + jsonStr(cwd) + `,"timestamp":"2026-05-30T08:01:30Z"}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return path
}

// The same discovery works for the claude sessions layout (cross-runtime, per
// LIVENESS-SIGNAL.md), and per-subagent transcripts are skipped so the top-level
// session is bound (not a child agent's transcript).
func TestDiscoverClaudeSessionPicksTopLevelAfterLaunch(t *testing.T) {
	root := t.TempDir()
	worktree := `C:\Agent\QueueDrainTestbed\.owned\w`
	launchedAt := time.Date(2026, 5, 30, 8, 0, 0, 0, time.UTC)
	slugDir := filepath.Join(root, "c--Agent-QueueDrainTestbed--owned-w")

	wantPath := writeClaudeTranscript(t, slugDir, "session-abc.jsonl", "session-abc", worktree, launchedAt.Add(1*time.Minute))
	// A per-subagent transcript under subagents/ must NOT be picked as the top-level.
	writeClaudeTranscript(t, filepath.Join(slugDir, "session-abc", "subagents"), "agent-1.jsonl", "subagent-1", worktree, launchedAt.Add(2*time.Minute))

	got, err := DiscoverSession(RuntimeClaude, root, worktree, launchedAt)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if got.SessionID != "session-abc" {
		t.Fatalf("session id = %q, want session-abc (the top-level, not the subagent)", got.SessionID)
	}
	if got.TranscriptPath != wantPath {
		t.Fatalf("transcript path = %q, want %q", got.TranscriptPath, wantPath)
	}
}
