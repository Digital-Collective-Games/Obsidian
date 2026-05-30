package queue

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLiveClaudeLaunchA51Smoke is an OPT-IN bounded live re-proof of A5.1 VIA THE
// LAUNCHER CODE. It runs ONLY when CLAUDE_LAUNCH_SMOKE_WORKTREE is set (e.g.
// C:\Agent\QueueDrainTestbed — a THROWAWAY worktree), so the normal `go test ./...`
// never starts a real claude process or touches the network.
//
// It drives the REAL queue.Launcher.Start to actually launch a top-level claude in
// the worktree with a TRIVIAL bounded prompt that asks the agent to spawn exactly one
// subagent, then confirms:
//   - the run exited is_error=false (parsed from claude --output-format json),
//   - the transcript exists at the deterministic ClaudeTranscriptPath,
//   - a per-subagent transcript appeared under <session>/subagents/,
//   - DeliverWake (claude --resume) appends to the SAME transcript (size grows).
//
// SAFETY: trivial prompt, hard timeout, --allowedTools Agent only, throwaway
// worktree, no push. Evidence is written under the dir named by
// CLAUDE_LAUNCH_SMOKE_EVIDENCE (if set).
func TestLiveClaudeLaunchA51Smoke(t *testing.T) {
	worktree := os.Getenv("CLAUDE_LAUNCH_SMOKE_WORKTREE")
	if worktree == "" {
		t.Skip("set CLAUDE_LAUNCH_SMOKE_WORKTREE (a throwaway worktree) to run the bounded live claude launch smoke")
	}
	evidenceDir := os.Getenv("CLAUDE_LAUNCH_SMOKE_EVIDENCE")

	const prompt = "Use the Agent tool to launch exactly one general-purpose subagent that replies SUBAGENT-OK, then reply DONE and stop. Do not modify files."

	stdoutPath := filepath.Join(t.TempDir(), "launch-stdout.json")
	stderrPath := filepath.Join(t.TempDir(), "launch-stderr.txt")
	if evidenceDir != "" {
		_ = os.MkdirAll(evidenceDir, 0o755)
		stdoutPath = filepath.Join(evidenceDir, "launch-stdout.json")
		stderrPath = filepath.Join(evidenceDir, "launch-stderr.txt")
	}

	launcher := NewLauncher(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	res, err := launcher.Start(ctx, LaunchSpec{
		WorktreePath: worktree,
		Prompt:       prompt,
		Timeout:      7 * time.Minute,
		StdoutPath:   stdoutPath,
		StderrPath:   stderrPath,
	}, true)
	t.Logf("launch: pid=%d session=%s transcript=%s exit=%d cmd=%v", res.PID, res.SessionID, res.TranscriptPath, res.ExitCode, res.Command)
	if err != nil {
		t.Fatalf("launch claude: %v (see %s / %s)", err, stdoutPath, stderrPath)
	}

	// (a) is_error=false from claude --output-format json.
	rawOut, rerr := os.ReadFile(stdoutPath)
	if rerr != nil {
		t.Fatalf("read launch stdout: %v", rerr)
	}
	var out struct {
		SessionID string `json:"session_id"`
		IsError   bool   `json:"is_error"`
		Result    string `json:"result"`
	}
	if jerr := json.Unmarshal([]byte(strings.TrimSpace(string(rawOut))), &out); jerr != nil {
		t.Fatalf("parse claude json output (%q): %v", string(rawOut), jerr)
	}
	t.Logf("claude json: session_id=%s is_error=%v result=%q", out.SessionID, out.IsError, out.Result)
	if out.IsError {
		t.Fatalf("claude run reported is_error=true: %q", out.Result)
	}
	if out.SessionID != res.SessionID {
		t.Fatalf("claude reported session_id=%q but we set --session-id=%q", out.SessionID, res.SessionID)
	}

	// (b) transcript exists at the DETERMINISTIC path.
	info, serr := os.Stat(res.TranscriptPath)
	if serr != nil {
		t.Fatalf("deterministic transcript not found at %s: %v", res.TranscriptPath, serr)
	}
	sizeAfterLaunch := info.Size()
	t.Logf("transcript at deterministic path: %s (%d bytes)", res.TranscriptPath, sizeAfterLaunch)

	// (c) a per-subagent transcript appeared under <session>/subagents/.
	subagentsDir := filepath.Join(filepath.Dir(res.TranscriptPath), res.SessionID, "subagents")
	subEntries, derr := os.ReadDir(subagentsDir)
	if derr != nil || len(subEntries) == 0 {
		t.Fatalf("no subagent transcript under %s (err=%v entries=%d) — the top-level agent did not spawn a subagent", subagentsDir, derr, len(subEntries))
	}
	var subPaths []string
	for _, e := range subEntries {
		subPaths = append(subPaths, filepath.Join(subagentsDir, e.Name()))
	}
	t.Logf("subagent transcript(s): %v", subPaths)

	// (d) DeliverWake (claude --resume) appends to the SAME transcript (size grows).
	poker := NewClaudeResumePoker(
		func(string) (WakeTarget, error) {
			exe, rerr := ResolveClaudeExecutable()
			if rerr != nil {
				return WakeTarget{}, rerr
			}
			return WakeTarget{Executable: exe, SessionID: res.SessionID, WorktreePath: worktree}, nil
		},
		nil,
		RunClaudeWake,
	)
	wctx, wcancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer wcancel()
	poker.ctx = wctx

	delivered, werr := poker.DeliverWake("live-run")
	if werr != nil {
		t.Fatalf("deliver wake (claude --resume): %v", werr)
	}
	if !delivered {
		t.Fatal("wake (claude --resume) did not exit 0 — wake input not delivered")
	}
	infoAfterWake, serr2 := os.Stat(res.TranscriptPath)
	if serr2 != nil {
		t.Fatalf("stat transcript after wake: %v", serr2)
	}
	t.Logf("transcript size: after_launch=%d after_wake=%d", sizeAfterLaunch, infoAfterWake.Size())
	if infoAfterWake.Size() <= sizeAfterLaunch {
		t.Fatalf("wake did not append to the same transcript: size %d -> %d", sizeAfterLaunch, infoAfterWake.Size())
	}

	// Write a small evidence summary if requested.
	if evidenceDir != "" {
		summary := map[string]any{
			"session_id":                    res.SessionID,
			"transcript_path":               res.TranscriptPath,
			"subagent_transcripts":          subPaths,
			"is_error":                      out.IsError,
			"result":                        out.Result,
			"transcript_after_launch_bytes": sizeAfterLaunch,
			"transcript_after_wake_bytes":   infoAfterWake.Size(),
			"launch_command":                res.Command,
			"wake_delivered":                delivered,
			"recorded_at":                   time.Now().UTC().Format(time.RFC3339),
		}
		if b, merr := json.MarshalIndent(summary, "", "  "); merr == nil {
			_ = os.WriteFile(filepath.Join(evidenceDir, "PROOF-SUMMARY.json"), b, 0o644)
		}
	}
}
