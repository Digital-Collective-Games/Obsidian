package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// O5 (5a) top-level agent launcher + POST-LAUNCH session discovery.
//
// The agent dispatched into an owned worktree MUST be a TOP-LEVEL process able to
// spawn its OWN subagents (D3 / A5.1) — never a nested subagent. The only existing
// agent-launch precedent (jobexec `codex exec`) is a blocking, non-interactive run
// that captures NO session id. This launcher reuses that `codex exec` invocation
// shape but adds the missing piece the coordinator review flagged (correction 2):
// the launched agent's OWN session id + transcript path do NOT exist until AFTER
// launch and CANNOT come from the backend's dispatch-time DeepContext (that holds
// the BACKEND process's session env). So we discover them post-launch by scanning
// the agent runtime's sessions dir for the newest session file created after the
// launch instant whose recorded cwd matches the worktree.
//
// internal/queue stays a leaf package: the launcher exposes plain inputs/outputs;
// production (taskrun) maps the discovered session into the O6 binding (replacing
// the dispatch-context placeholders) without this package importing taskrun.

// AgentRuntime selects which headless agent CLI is launched and which sessions
// layout post-launch discovery scans.
type AgentRuntime string

const (
	// RuntimeCodex launches `codex exec` and discovers rollout-*.jsonl under
	// ~/.codex/sessions/YYYY/MM/DD/.
	RuntimeCodex AgentRuntime = "codex"
	// RuntimeClaude launches `claude` (headless) and discovers <session>.jsonl under
	// ~/.claude/projects/<slug>/.
	RuntimeClaude AgentRuntime = "claude"
)

// LaunchSpec is the input to launching a top-level agent in an owned worktree.
type LaunchSpec struct {
	// Runtime is the headless agent CLI to launch.
	Runtime AgentRuntime
	// Executable is the resolved agent CLI path (e.g. codex.exe). Required.
	Executable string
	// WorktreePath is the owned worktree the agent runs in (its cwd). Required.
	WorktreePath string
	// Prompt is the task instruction handed to the agent.
	Prompt string
	// SessionsRoot is the agent runtime's sessions root to scan for the launched
	// session (~/.codex/sessions for codex; ~/.claude/projects for claude). Required.
	SessionsRoot string
	// Timeout bounds the launched process. Zero means no enforced timeout (callers
	// in unattended contexts MUST set one). The launcher always returns once the
	// process exits or the timeout fires.
	Timeout time.Duration
	// StdoutPath / StderrPath capture the process streams (optional).
	StdoutPath string
	StderrPath string
}

// LaunchResult reports what the launcher started and discovered.
type LaunchResult struct {
	// PID is the launched agent process id.
	PID int
	// LaunchedAt is the instant just before the process started (the discovery
	// floor: only session files created at/after this are the launched agent's).
	LaunchedAt time.Time
	// SessionID is the launched agent's OWN session id, discovered post-launch.
	SessionID string
	// TranscriptPath is the launched agent's OWN session transcript, discovered
	// post-launch. This is the path the O5 watchdog stats and the O6 binding records
	// (replacing the dispatch-context placeholders).
	TranscriptPath string
	// Exited / ExitCode report process completion (for a bounded demo run).
	Exited   bool
	ExitCode int
}

// buildAgentCommand builds the top-level agent command. For codex it mirrors the
// jobexec `codex exec` invocation shape (non-interactive, bypassing sandbox in the
// owned worktree). It is split out so the argument shape is unit-testable without
// launching a process.
func buildAgentCommand(spec LaunchSpec) (string, []string, error) {
	if spec.Executable == "" {
		return "", nil, fmt.Errorf("launch: executable is empty")
	}
	if spec.WorktreePath == "" {
		return "", nil, fmt.Errorf("launch: worktree path is empty")
	}
	switch spec.Runtime {
	case RuntimeCodex:
		return spec.Executable, []string{
			"exec",
			"--dangerously-bypass-approvals-and-sandbox",
			"--skip-git-repo-check",
			"--json",
			"-C", spec.WorktreePath,
			spec.Prompt,
		}, nil
	case RuntimeClaude:
		return spec.Executable, []string{
			"-p", spec.Prompt,
			"--output-format", "stream-json",
			"--verbose",
		}, nil
	default:
		return "", nil, fmt.Errorf("launch: unsupported runtime %q", spec.Runtime)
	}
}

// Launcher launches a top-level agent and discovers its session post-launch. now is
// injected so the discovery floor (LaunchedAt) is testable without real wall time.
type Launcher struct {
	now func() time.Time
}

// NewLauncher builds a launcher. A nil clock uses time.Now.
func NewLauncher(now func() time.Time) *Launcher {
	if now == nil {
		now = time.Now
	}
	return &Launcher{now: now}
}

// Start launches the agent as a top-level process in its worktree and (optionally,
// when wait is true) waits for it to exit within the spec's timeout, then discovers
// the launched session. It does NOT launch the agent as a nested subagent: it is a
// distinct OS process via exec, exactly like jobexec's codex exec, so the agent is
// free to spawn its own subagents (A5.1).
//
// SAFETY: the caller is responsible for bounding what the agent may do (a trivial
// prompt + a hard timeout + a throwaway worktree). Start never pushes and never acts
// outside the worktree it is given.
func (l *Launcher) Start(ctx context.Context, spec LaunchSpec, wait bool) (LaunchResult, error) {
	executable, args, err := buildAgentCommand(spec)
	if err != nil {
		return LaunchResult{}, err
	}

	launchedAt := l.now()

	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = spec.WorktreePath
	if spec.StdoutPath != "" {
		f, err := os.Create(spec.StdoutPath)
		if err != nil {
			return LaunchResult{}, fmt.Errorf("create stdout capture: %w", err)
		}
		defer f.Close()
		cmd.Stdout = f
	}
	if spec.StderrPath != "" {
		f, err := os.Create(spec.StderrPath)
		if err != nil {
			return LaunchResult{}, fmt.Errorf("create stderr capture: %w", err)
		}
		defer f.Close()
		cmd.Stderr = f
	}

	if err := cmd.Start(); err != nil {
		return LaunchResult{}, fmt.Errorf("start agent: %w", err)
	}
	result := LaunchResult{PID: cmd.Process.Pid, LaunchedAt: launchedAt}

	if wait {
		waitErr := waitWithTimeout(cmd, spec.Timeout)
		result.Exited = true
		result.ExitCode = exitCodeOf(cmd)
		// A timeout/non-zero exit is reported to the caller; discovery still runs so
		// even a bounded demo can confirm a session file appeared.
		if disc, derr := DiscoverSession(spec.Runtime, spec.SessionsRoot, spec.WorktreePath, launchedAt); derr == nil {
			result.SessionID = disc.SessionID
			result.TranscriptPath = disc.TranscriptPath
		}
		if waitErr != nil {
			return result, waitErr
		}
		return result, nil
	}

	return result, nil
}

// waitWithTimeout waits for cmd, killing it if timeout elapses (timeout<=0 waits
// indefinitely — callers in unattended contexts must set one).
func waitWithTimeout(cmd *exec.Cmd, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	if timeout <= 0 {
		return <-done
	}
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		<-done
		return fmt.Errorf("agent exceeded timeout %s and was terminated", timeout)
	}
}

func exitCodeOf(cmd *exec.Cmd) int {
	if cmd.ProcessState == nil {
		return -1
	}
	return cmd.ProcessState.ExitCode()
}

// DiscoveredSession is the result of post-launch session discovery.
type DiscoveredSession struct {
	SessionID      string
	TranscriptPath string
}

// DiscoverSession finds the launched agent's OWN session file post-launch. It scans
// the runtime's sessions layout for the newest session file created at/after
// launchedAt whose recorded cwd matches the worktree. This is the missing piece the
// dispatch-time DeepContext cannot supply (correction 2): the launched agent's
// session id/transcript do not exist until after launch.
//
//   - codex: rollout-*.jsonl under sessionsRoot/YYYY/MM/DD/; the first line is a
//     session_meta event carrying payload.id (session id) and payload.cwd.
//   - claude: <session>.jsonl under sessionsRoot/<slug>/; each line carries cwd and
//     sessionId. The slug for a cwd is the path with separators replaced by '-'.
//
// It returns an error when no matching session file appears (so the caller does not
// silently bind an empty/placeholder session).
func DiscoverSession(runtime AgentRuntime, sessionsRoot, worktreePath string, launchedAt time.Time) (DiscoveredSession, error) {
	switch runtime {
	case RuntimeCodex:
		return discoverCodexSession(sessionsRoot, worktreePath, launchedAt)
	case RuntimeClaude:
		return discoverClaudeSession(sessionsRoot, worktreePath, launchedAt)
	default:
		return DiscoveredSession{}, fmt.Errorf("discover: unsupported runtime %q", runtime)
	}
}

// candidate is a discovered session file with the facts used to rank it.
type candidate struct {
	path      string
	sessionID string
	createdAt time.Time // file mtime (creation proxy for an append-only just-written file)
}

func discoverCodexSession(sessionsRoot, worktreePath string, launchedAt time.Time) (DiscoveredSession, error) {
	var candidates []candidate
	walkErr := filepath.WalkDir(sessionsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // tolerate unreadable subtrees
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(name, ".jsonl") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		// Only files touched at/after the launch instant are the launched session.
		if info.ModTime().Before(launchedAt) {
			return nil
		}
		id, cwd, ok := readCodexSessionMeta(path)
		if !ok {
			return nil
		}
		if !sameRoot(cwd, worktreePath) {
			return nil
		}
		candidates = append(candidates, candidate{path: path, sessionID: id, createdAt: info.ModTime()})
		return nil
	})
	if walkErr != nil {
		return DiscoveredSession{}, fmt.Errorf("scan codex sessions %s: %w", sessionsRoot, walkErr)
	}
	return pickNewest(candidates, "codex", worktreePath)
}

// readCodexSessionMeta reads the first line of a rollout file and extracts the
// session id (payload.id) and cwd (payload.cwd) from the session_meta event.
func readCodexSessionMeta(path string) (id, cwd string, ok bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", false
	}
	line := raw
	if idx := indexByte(raw, '\n'); idx >= 0 {
		line = raw[:idx]
	}
	var head struct {
		Type    string `json:"type"`
		Payload struct {
			ID  string `json:"id"`
			Cwd string `json:"cwd"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(line, &head); err != nil {
		return "", "", false
	}
	if head.Type != "session_meta" || head.Payload.ID == "" {
		return "", "", false
	}
	return head.Payload.ID, head.Payload.Cwd, true
}

func discoverClaudeSession(sessionsRoot, worktreePath string, launchedAt time.Time) (DiscoveredSession, error) {
	var candidates []candidate
	walkErr := filepath.WalkDir(sessionsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		// Skip per-subagent transcripts under .../<session>/subagents/.
		if strings.Contains(filepath.ToSlash(path), "/subagents/") {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.ModTime().Before(launchedAt) {
			return nil
		}
		id, cwd, ok := readClaudeSessionMeta(path)
		if !ok || !sameRoot(cwd, worktreePath) {
			return nil
		}
		candidates = append(candidates, candidate{path: path, sessionID: id, createdAt: info.ModTime()})
		return nil
	})
	if walkErr != nil {
		return DiscoveredSession{}, fmt.Errorf("scan claude sessions %s: %w", sessionsRoot, walkErr)
	}
	return pickNewest(candidates, "claude", worktreePath)
}

// readClaudeSessionMeta reads the first decodable line of a claude transcript and
// extracts its sessionId + cwd (every line carries both).
func readClaudeSessionMeta(path string) (id, cwd string, ok bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", false
	}
	for _, line := range splitLines(raw) {
		if len(line) == 0 {
			continue
		}
		var head struct {
			SessionID string `json:"sessionId"`
			Cwd       string `json:"cwd"`
		}
		if err := json.Unmarshal(line, &head); err != nil {
			continue
		}
		if head.SessionID != "" {
			return head.SessionID, head.Cwd, true
		}
	}
	return "", "", false
}

// pickNewest returns the newest matching candidate (the launched session) or an
// error naming what was scanned when none matched.
func pickNewest(candidates []candidate, runtime, worktreePath string) (DiscoveredSession, error) {
	if len(candidates) == 0 {
		return DiscoveredSession{}, fmt.Errorf("no %s session file created after launch matched worktree %s", runtime, worktreePath)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].createdAt.After(candidates[j].createdAt) })
	best := candidates[0]
	return DiscoveredSession{SessionID: best.sessionID, TranscriptPath: best.path}, nil
}

// sameRoot compares two paths for equality up to separator/case/trailing-slash
// differences (reusing the manifest's normalizeRoot rules).
func sameRoot(a, b string) bool {
	return normalizeRoot(a) == normalizeRoot(b)
}

func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}

func splitLines(b []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := range b {
		if b[i] == '\n' {
			lines = append(lines, b[start:i])
			start = i + 1
		}
	}
	if start < len(b) {
		lines = append(lines, b[start:])
	}
	return lines
}
