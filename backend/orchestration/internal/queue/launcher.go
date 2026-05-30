package queue

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// O5 (5a) top-level CLAUDE agent launcher + deterministic session binding.
//
// Per the human directive "dispatch CLAUDE only, not codex" (2026-05-30, refines
// D3): the dispatched queue agent is `claude` ONLY. The agent dispatched into an
// owned worktree MUST be a TOP-LEVEL process able to spawn its OWN subagents via the
// Agent tool (A5.1) — never a nested subagent, and never codex.
//
// The launched agent's session id is now known UP FRONT: we generate a UUID v4 and
// pass it as `--session-id`, so claude writes its transcript at a DETERMINISTIC path
// computed from (worktree, session-id) via the slug rule (no post-launch scan needed
// to learn the id). DiscoverSession is retained ONLY as a verification fallback to
// confirm the transcript file appeared at the computed path shortly after launch.
//
// internal/queue stays a leaf package: the launcher exposes plain inputs/outputs;
// production (taskrun) maps the up-front session id + computed transcript path into
// the O6 binding without this package importing taskrun.

// LaunchSpec is the input to launching a top-level claude agent in an owned worktree.
type LaunchSpec struct {
	// Executable is the resolved claude CLI path. Optional: empty resolves via
	// ResolveClaudeExecutable (configurable path/env, else the newest installed
	// claude-code extension binary).
	Executable string
	// WorktreePath is the owned worktree the agent runs in (its cwd). Required.
	WorktreePath string
	// Prompt is the task instruction handed to the agent.
	Prompt string
	// SessionID is the agent's session id. Optional: empty generates a UUID v4. It is
	// passed to claude as --session-id, so it (and the transcript path) are known
	// before launch.
	SessionID string
	// SessionsRoot is claude's projects root (~/.claude/projects) used by the
	// DiscoverSession verification fallback. Optional.
	SessionsRoot string
	// AllowedTools is the comma-separated --allowedTools value. Optional: empty
	// uses DefaultAllowedTools ("Agent") so the existing trivial-proof invocation is
	// unchanged. The queue dispatcher overrides it with the real tool set a working
	// task agent needs (Read/Edit/Bash/Agent), kept configurable.
	AllowedTools string
	// PermissionMode is the --permission-mode value. Optional: empty uses
	// DefaultPermissionMode ("bypassPermissions").
	PermissionMode string
	// Timeout bounds the launched process. Zero means no enforced timeout (callers
	// in unattended contexts MUST set one). The launcher always returns once the
	// process exits or the timeout fires.
	Timeout time.Duration
	// StdoutPath / StderrPath capture the process streams (optional).
	StdoutPath string
	StderrPath string
}

// LaunchResult reports what the launcher started.
type LaunchResult struct {
	// PID is the launched agent process id.
	PID int
	// LaunchedAt is the instant just before the process started.
	LaunchedAt time.Time
	// SessionID is the launched agent's session id (the one passed to --session-id;
	// known UP FRONT, not discovered).
	SessionID string
	// TranscriptPath is the launched agent's session transcript, computed
	// deterministically from (worktree, session-id). This is the path the O5 watchdog
	// stats and the O6 binding records.
	TranscriptPath string
	// Command is the resolved executable + args (for proof/logging).
	Command []string
	// Exited / ExitCode report process completion (for a bounded demo run).
	Exited   bool
	ExitCode int
}

// Defaults for the configurable launch knobs. Empty LaunchSpec fields fall back to
// these, so the validated trivial-proof invocation (--allowedTools Agent,
// --permission-mode bypassPermissions) is unchanged unless a caller overrides it.
const (
	// DefaultAllowedTools is the trivial-proof --allowedTools value.
	DefaultAllowedTools = "Agent"
	// DefaultPermissionMode is the --permission-mode value.
	DefaultPermissionMode = "bypassPermissions"
)

// buildAgentCommand builds the top-level CLAUDE command (the VALIDATED invocation
// proven on this machine to run headlessly and spawn its own subagent):
//
//	claude --session-id <UUID> -p "<prompt>" --permission-mode <mode> \
//	       --allowedTools "<tools>" --output-format json
//
// run with cwd = the owned worktree. It is a top-level OS process (NOT a nested
// subagent), so the agent is free to spawn its own subagents via the Agent tool
// (A5.1 / D3). --allowedTools and --permission-mode are configurable (LaunchSpec);
// empty fields fall back to the validated defaults. It is split out so the argument
// shape is unit-testable without launching a process.
func buildAgentCommand(spec LaunchSpec) (string, []string, error) {
	if spec.Executable == "" {
		return "", nil, fmt.Errorf("launch: executable is empty")
	}
	if spec.WorktreePath == "" {
		return "", nil, fmt.Errorf("launch: worktree path is empty")
	}
	if spec.SessionID == "" {
		return "", nil, fmt.Errorf("launch: session id is empty")
	}
	allowedTools := spec.AllowedTools
	if allowedTools == "" {
		allowedTools = DefaultAllowedTools
	}
	permissionMode := spec.PermissionMode
	if permissionMode == "" {
		permissionMode = DefaultPermissionMode
	}
	return spec.Executable, []string{
		"--session-id", spec.SessionID,
		"-p", spec.Prompt,
		"--permission-mode", permissionMode,
		"--allowedTools", allowedTools,
		"--output-format", "json",
	}, nil
}

// WakeMessage is the fixed nudge delivered to a stalled agent on the one poke (D4):
// either write a durable stop update or resume work.
const WakeMessage = "Write a durable stop update — set Human Needed=Yes or request closure — or get back to work."

// buildWakeCommand builds the VALIDATED claude resume-wake invocation that actually
// delivers input to a stalled headless agent and appends to its SAME session
// transcript (proven on this machine):
//
//	claude --resume <session-id> -p "<wake message>"
//
// run with cwd = the owned worktree and the same env overrides as launch
// (claudeChildEnv). It is split out so the wake argument shape is unit-testable
// without launching a process.
func buildWakeCommand(executable, sessionID, message string) (string, []string, error) {
	if executable == "" {
		return "", nil, fmt.Errorf("wake: executable is empty")
	}
	if sessionID == "" {
		return "", nil, fmt.Errorf("wake: session id is empty")
	}
	return executable, []string{
		"--resume", sessionID,
		"-p", message,
	}, nil
}

// claudeChildEnv returns the environment for a dispatched claude child: the parent's
// environment with the critical overrides the validated invocation requires —
//   - CLAUDE_CODE_ENABLE_TASKS=1 (the host sets =0, which disables the Agent/subagent
//     tool the dispatched agent MUST be able to use; A5.1),
//   - CLAUDE_CODE_SESSION_ID and CLAUDE_CODE_ENTRYPOINT UNSET (so the child does not
//     inherit the parent backend's session/entrypoint),
//
// and the rest of the environment passed through. It is split out so the override
// rule is unit-testable without launching a process.
func claudeChildEnv(parent []string) []string {
	out := make([]string, 0, len(parent)+1)
	for _, kv := range parent {
		key := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			key = kv[:i]
		}
		switch key {
		case "CLAUDE_CODE_ENABLE_TASKS", "CLAUDE_CODE_SESSION_ID", "CLAUDE_CODE_ENTRYPOINT":
			// Dropped here; ENABLE_TASKS is re-added below, the others stay unset.
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "CLAUDE_CODE_ENABLE_TASKS=1")
	return out
}

// ResolveClaudeExecutable resolves the claude CLI path. Resolution order:
//  1. the configured CODEX_ORCHESTRATION_CLAUDE_BIN env var (explicit override),
//  2. `claude` on PATH,
//  3. the NEWEST installed `anthropic.claude-code-*/resources/native-binary/claude.exe`
//     found under the VS Code / VS Code-OSS / Cursor extensions dirs.
//
// It errors clearly if none is found (it never silently returns a bare "claude" that
// would then fail at launch). The versioned extension dir is resolved, NOT hardcoded.
func ResolveClaudeExecutable() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("CODEX_ORCHESTRATION_CLAUDE_BIN")); configured != "" {
		return configured, nil
	}
	if path, err := exec.LookPath("claude"); err == nil {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve claude: user home: %w", err)
	}
	var candidates []string
	extDirs := []string{
		filepath.Join(home, ".vscode-oss", "extensions"),
		filepath.Join(home, ".vscode", "extensions"),
		filepath.Join(home, ".cursor", "extensions"),
	}
	for _, ext := range extDirs {
		matches, _ := filepath.Glob(filepath.Join(ext, "anthropic.claude-code-*", "resources", "native-binary", "claude.exe"))
		candidates = append(candidates, matches...)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("resolve claude: no claude executable found (set CODEX_ORCHESTRATION_CLAUDE_BIN, put claude on PATH, or install the claude-code extension)")
	}
	// Lexical sort puts the newest version dir last (e.g. ...-2.1.158-... wins).
	sort.Strings(candidates)
	return candidates[len(candidates)-1], nil
}

// newSessionID generates a random UUID v4 string for --session-id.
func newSessionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// claudeProjectSlug computes claude's project-dir slug for a worktree cwd: the
// absolute path with the drive letter lowercased and every ':' , '\\' and '/'
// replaced by '-'. Verified on this machine:
//
//	C:\Agent\QueueDrainTestbed -> c--Agent-QueueDrainTestbed
//	C:\Agent\CodexDashboard    -> c--Agent-CodexDashboard
func claudeProjectSlug(worktreePath string) string {
	p := strings.TrimSpace(worktreePath)
	// Lowercase the drive letter (e.g. "C:" -> "c:") without touching the rest.
	if len(p) >= 2 && p[1] == ':' {
		p = strings.ToLower(p[:1]) + p[1:]
	}
	repl := strings.NewReplacer(":", "-", "\\", "-", "/", "-")
	return repl.Replace(p)
}

// ClaudeTranscriptPath computes the DETERMINISTIC transcript path for a claude run
// from its worktree cwd + session id:
//
//	~/.claude/projects/<slug>/<session-id>.jsonl
//
// where <slug> = claudeProjectSlug(worktreePath). This is the PRIMARY binding source
// (the watchdog stats it; the O6 binding records it) — known before launch because
// the session id is set up front.
func ClaudeTranscriptPath(worktreePath, sessionID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("claude transcript path: user home: %w", err)
	}
	return filepath.Join(home, ".claude", "projects", claudeProjectSlug(worktreePath), sessionID+".jsonl"), nil
}

// Launcher launches a top-level claude agent. now is injected so LaunchedAt is
// testable without real wall time.
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

// Start launches claude as a top-level process in its worktree and (optionally, when
// wait is true) waits for it to exit within the spec's timeout. The session id is
// generated up front (if not supplied) and the transcript path is computed
// deterministically, so the binding is known before the process even appends.
//
// It does NOT launch claude as a nested subagent: it is a distinct OS process via
// exec, so the agent is free to spawn its OWN subagents via the Agent tool (A5.1).
// The child env applies the validated overrides (claudeChildEnv).
//
// SAFETY: the caller is responsible for bounding what the agent may do (a trivial
// prompt + a hard timeout + a throwaway worktree). Start never pushes and never acts
// outside the worktree it is given.
func (l *Launcher) Start(ctx context.Context, spec LaunchSpec, wait bool) (LaunchResult, error) {
	if spec.Executable == "" {
		resolved, err := ResolveClaudeExecutable()
		if err != nil {
			return LaunchResult{}, err
		}
		spec.Executable = resolved
	}
	if spec.SessionID == "" {
		id, err := newSessionID()
		if err != nil {
			return LaunchResult{}, err
		}
		spec.SessionID = id
	}

	executable, args, err := buildAgentCommand(spec)
	if err != nil {
		return LaunchResult{}, err
	}

	transcriptPath, err := ClaudeTranscriptPath(spec.WorktreePath, spec.SessionID)
	if err != nil {
		return LaunchResult{}, err
	}

	launchedAt := l.now()

	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = spec.WorktreePath
	cmd.Env = claudeChildEnv(os.Environ())
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
	result := LaunchResult{
		PID:            cmd.Process.Pid,
		LaunchedAt:     launchedAt,
		SessionID:      spec.SessionID,
		TranscriptPath: transcriptPath,
		Command:        append([]string{executable}, args...),
	}

	if wait {
		waitErr := waitWithTimeout(cmd, spec.Timeout)
		result.Exited = true
		result.ExitCode = exitCodeOf(cmd)
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

// DiscoveredSession is the result of the post-launch session verification fallback.
type DiscoveredSession struct {
	SessionID      string
	TranscriptPath string
}

// DiscoverSession is a VERIFICATION FALLBACK: it confirms claude wrote a transcript
// for the launched run by scanning ~/.claude/projects/<slug>/ for the newest
// <session>.jsonl created at/after launchedAt whose recorded cwd matches the worktree
// (skipping per-subagent transcripts under .../<session>/subagents/). The PRIMARY
// binding source is now the up-front session id + ClaudeTranscriptPath; this exists
// only to confirm the file appeared at the computed path shortly after launch.
//
// It returns an error when no matching session file appears (so the caller can
// surface a launch that produced no transcript).
func DiscoverSession(sessionsRoot, worktreePath string, launchedAt time.Time) (DiscoveredSession, error) {
	var candidates []candidate
	walkErr := filepath.WalkDir(sessionsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // tolerate unreadable subtrees
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
	return pickNewest(candidates, worktreePath)
}

// candidate is a discovered session file with the facts used to rank it.
type candidate struct {
	path      string
	sessionID string
	createdAt time.Time // file mtime (creation proxy for an append-only just-written file)
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
func pickNewest(candidates []candidate, worktreePath string) (DiscoveredSession, error) {
	if len(candidates) == 0 {
		return DiscoveredSession{}, fmt.Errorf("no claude session file created after launch matched worktree %s", worktreePath)
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
