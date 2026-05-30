package queue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// jsonStr JSON-encodes s so a Windows path's backslashes are properly escaped in
// the synthetic transcript fixtures (a raw "C:\A..." would be invalid JSON).
func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// buildAgentCommand must produce a TOP-LEVEL CLAUDE invocation (not codex, not a
// nested subagent): `claude --session-id <id> -p <prompt> --permission-mode
// bypassPermissions --allowedTools Agent --output-format json` so the launched agent
// can spawn its own subagents via the Agent tool (A5.1 / D3; dispatch-claude-only).
func TestBuildAgentCommandIsTopLevelClaude(t *testing.T) {
	exe, args, err := buildAgentCommand(LaunchSpec{
		Executable:   `C:\claude.exe`,
		WorktreePath: `C:\Agent\QueueDrainTestbed`,
		SessionID:    "11111111-2222-4333-8444-555555555555",
		Prompt:       "do the bounded task",
	})
	if err != nil {
		t.Fatalf("build claude command: %v", err)
	}
	if exe != `C:\claude.exe` {
		t.Fatalf("executable = %q", exe)
	}
	// It must NOT be codex (no `exec` subcommand, no codex-only flags).
	for _, bad := range []string{"exec", "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check", "-C"} {
		for _, a := range args {
			if a == bad {
				t.Fatalf("claude command must not contain codex flag %q: %v", bad, args)
			}
		}
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--session-id 11111111-2222-4333-8444-555555555555",
		"-p do the bounded task",
		"--permission-mode bypassPermissions",
		"--allowedTools Agent",
		"--output-format json",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("claude command missing %q: %v", want, args)
		}
	}
	// --allowedTools "Agent" (subagent tool), NOT a nested-subagent launch flag.
	foundAllowed := false
	for i, a := range args {
		if a == "--allowedTools" && i+1 < len(args) && args[i+1] == "Agent" {
			foundAllowed = true
		}
	}
	if !foundAllowed {
		t.Fatalf("expected --allowedTools Agent (top-level able to spawn subagents): %v", args)
	}
}

func TestBuildAgentCommandValidatesInputs(t *testing.T) {
	if _, _, err := buildAgentCommand(LaunchSpec{WorktreePath: "x", SessionID: "s"}); err == nil {
		t.Fatal("expected error on empty executable")
	}
	if _, _, err := buildAgentCommand(LaunchSpec{Executable: "x", SessionID: "s"}); err == nil {
		t.Fatal("expected error on empty worktree")
	}
	if _, _, err := buildAgentCommand(LaunchSpec{Executable: "x", WorktreePath: "y"}); err == nil {
		t.Fatal("expected error on empty session id")
	}
}

// claudeProjectSlug / ClaudeTranscriptPath compute the DETERMINISTIC transcript path
// from (worktree, session-id): the drive letter is lowercased and ':','\\','/' become
// '-'. Both examples were verified on this machine.
func TestClaudeProjectSlugVerifiedExamples(t *testing.T) {
	cases := []struct {
		worktree string
		want     string
	}{
		{`C:\Agent\QueueDrainTestbed`, "c--Agent-QueueDrainTestbed"},
		{`C:\Agent\CodexDashboard`, "c--Agent-CodexDashboard"},
	}
	for _, c := range cases {
		if got := claudeProjectSlug(c.worktree); got != c.want {
			t.Fatalf("slug(%q) = %q, want %q", c.worktree, got, c.want)
		}
	}
}

func TestClaudeTranscriptPathIsDeterministic(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home: %v", err)
	}
	got, err := ClaudeTranscriptPath(`C:\Agent\QueueDrainTestbed`, "session-xyz")
	if err != nil {
		t.Fatalf("transcript path: %v", err)
	}
	want := filepath.Join(home, ".claude", "projects", "c--Agent-QueueDrainTestbed", "session-xyz.jsonl")
	if got != want {
		t.Fatalf("transcript path = %q, want %q", got, want)
	}
}

// claudeChildEnv must FORCE CLAUDE_CODE_ENABLE_TASKS=1 (so the dispatched agent keeps
// the Agent/subagent tool the host disables with =0), UNSET CLAUDE_CODE_SESSION_ID and
// CLAUDE_CODE_ENTRYPOINT (so the child does not inherit the parent's), and pass the
// rest of the environment through.
func TestClaudeChildEnvOverrides(t *testing.T) {
	parent := []string{
		"PATH=C:\\bin",
		"CLAUDE_CODE_ENABLE_TASKS=0",
		"CLAUDE_CODE_SESSION_ID=parent-session",
		"CLAUDE_CODE_ENTRYPOINT=cli",
		"SOME_OTHER=keepme",
	}
	got := claudeChildEnv(parent)

	enableTasks := 0
	for _, kv := range got {
		switch {
		case strings.HasPrefix(kv, "CLAUDE_CODE_ENABLE_TASKS="):
			enableTasks++
			if kv != "CLAUDE_CODE_ENABLE_TASKS=1" {
				t.Fatalf("CLAUDE_CODE_ENABLE_TASKS = %q, want =1", kv)
			}
		case strings.HasPrefix(kv, "CLAUDE_CODE_SESSION_ID="):
			t.Fatalf("CLAUDE_CODE_SESSION_ID must be unset, found %q", kv)
		case strings.HasPrefix(kv, "CLAUDE_CODE_ENTRYPOINT="):
			t.Fatalf("CLAUDE_CODE_ENTRYPOINT must be unset, found %q", kv)
		}
	}
	if enableTasks != 1 {
		t.Fatalf("expected exactly one CLAUDE_CODE_ENABLE_TASKS=1, got %d occurrences", enableTasks)
	}
	// Pass-through of unrelated vars.
	if !containsEnv(got, "PATH=C:\\bin") || !containsEnv(got, "SOME_OTHER=keepme") {
		t.Fatalf("child env dropped a pass-through var: %v", got)
	}
}

func containsEnv(env []string, kv string) bool {
	for _, e := range env {
		if e == kv {
			return true
		}
	}
	return false
}

// buildWakeCommand must produce the `claude --resume <id> -p <message>` invocation
// (the real wake-input delivery; resumes the same session/transcript), not codex.
func TestBuildWakeCommandIsClaudeResume(t *testing.T) {
	exe, args, err := buildWakeCommand(`C:\claude.exe`, "session-abc", WakeMessage)
	if err != nil {
		t.Fatalf("build wake command: %v", err)
	}
	if exe != `C:\claude.exe` {
		t.Fatalf("executable = %q", exe)
	}
	if len(args) != 4 || args[0] != "--resume" || args[1] != "session-abc" || args[2] != "-p" || args[3] != WakeMessage {
		t.Fatalf("wake args = %v, want [--resume session-abc -p <WakeMessage>]", args)
	}
	if _, _, err := buildWakeCommand("", "s", "m"); err == nil {
		t.Fatal("expected error on empty executable")
	}
	if _, _, err := buildWakeCommand("x", "", "m"); err == nil {
		t.Fatal("expected error on empty session id")
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

// DiscoverSession is the VERIFICATION FALLBACK: it confirms claude wrote a transcript
// for the launched run (newest <session>.jsonl created at/after launch whose cwd
// matches the worktree), skipping per-subagent transcripts under subagents/.
func TestDiscoverClaudeSessionPicksTopLevelAfterLaunch(t *testing.T) {
	root := t.TempDir()
	worktree := `C:\Agent\QueueDrainTestbed\.owned\w`
	launchedAt := time.Date(2026, 5, 30, 8, 0, 0, 0, time.UTC)
	slugDir := filepath.Join(root, "c--Agent-QueueDrainTestbed--owned-w")

	wantPath := writeClaudeTranscript(t, slugDir, "session-abc.jsonl", "session-abc", worktree, launchedAt.Add(1*time.Minute))
	// A per-subagent transcript under subagents/ must NOT be picked as the top-level.
	writeClaudeTranscript(t, filepath.Join(slugDir, "session-abc", "subagents"), "agent-1.jsonl", "subagent-1", worktree, launchedAt.Add(2*time.Minute))

	got, err := DiscoverSession(root, worktree, launchedAt)
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

// When no session file appears after launch, the verification fallback returns an
// error so the caller can surface a launch that produced no transcript.
func TestDiscoverClaudeSessionErrorsWhenNoneMatch(t *testing.T) {
	root := t.TempDir()
	worktree := `C:\Agent\QueueDrainTestbed`
	launchedAt := time.Date(2026, 5, 30, 8, 0, 0, 0, time.UTC)

	// Only a pre-launch session exists.
	writeClaudeTranscript(t, filepath.Join(root, "c--Agent-QueueDrainTestbed"), "old.jsonl", "old", worktree, launchedAt.Add(-time.Minute))

	if _, err := DiscoverSession(root, worktree, launchedAt); err == nil {
		t.Fatal("expected an error when no post-launch session matched")
	}
}
