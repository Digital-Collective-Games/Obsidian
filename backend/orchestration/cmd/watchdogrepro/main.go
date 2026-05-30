// Command watchdogrepro is the controlled SYNTHETIC-TRANSCRIPT repro for O5/PASS-0005.
// It runs the REAL queue.Watchdog (with the production queue.OSStatTranscript +
// queue.OSTailTranscript adapters) against a REAL on-disk transcript file whose
// mtime is FROZEN, to prove, with NO real agent and NO real email:
//
//   - A5.2 / F-O5-signal: the watchdog reads the pinned size+mtime append signal and
//     anchors the deadline to last observed activity (fresh append refreshes it).
//   - A5.3 (watchdog half): a stall past the window yields exactly ONE poke, then —
//     after the ack window with no fresh append — an incident EMAIL (captured, not
//     sent) containing BOTH the observed state AND the session transcript (tail inline
//   - full-file path).
//   - A5.5 (HARD) / F-O5-parked: a gate-parked run sits idle well past the window and
//     the watchdog stays SILENT (no poke, no email).
//
// The clock is injected/frozen so the repro is deterministic and instant. Evidence
// (JSON + the captured incident email body) is written to the path given as the
// single CLI arg. The IncidentSink is queue.CaptureSink — a real email is NEVER sent.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/queue"
)

type evidence struct {
	GeneratedAt    string          `json:"generated_at"`
	RealWatchdog   bool            `json:"real_watchdog"`
	RealEmailSent  bool            `json:"real_email_sent"`
	StallWindow    string          `json:"stall_window"`
	PokeAckWindow  string          `json:"poke_ack_window"`
	TranscriptPath string          `json:"transcript_path"`
	ActiveScenario scenarioResult  `json:"active_run_scenario"`
	ParkedScenario scenarioResult  `json:"gate_parked_scenario"`
	IncidentEmail  *queue.Incident `json:"incident_email,omitempty"`
}

type scenarioResult struct {
	Description        string `json:"description"`
	PokeCount          int    `json:"poke_count"`
	IncidentCount      int    `json:"incident_count"`
	DeadlineAnchoredTo string `json:"deadline_anchored_to"`
}

// frozenClock is an advanceable injected clock so the repro is deterministic.
type frozenClock struct{ t time.Time }

func (c *frozenClock) now() time.Time          { return c.t }
func (c *frozenClock) advance(d time.Duration) { c.t = c.t.Add(d) }

// writeTranscript writes a synthetic codex rollout transcript and freezes its mtime
// to the given instant, so the watchdog's OSStatTranscript sees a real but frozen
// size+mtime — exactly the "agent went quiet" condition.
func writeTranscript(path string, lines int, mtime time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := ""
	for i := 0; i < lines; i++ {
		body += fmt.Sprintf(`{"timestamp":"2026-05-30T08:0%d:00Z","type":"event_msg","payload":{"n":%d}}`+"\n", i%10, i)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return err
	}
	return os.Chtimes(path, mtime, mtime)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: watchdogrepro <evidence-dir>")
		os.Exit(2)
	}
	evidenceDir := os.Args[1]
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	scratch, err := os.MkdirTemp("", "watchdogrepro-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(scratch)
	transcriptPath := filepath.Join(scratch, "2026", "05", "30", "rollout-repro.jsonl")

	cfg := queue.WatchdogConfig{StallWindow: 5 * time.Minute, PokeAckWindow: 5 * time.Minute}
	t0 := time.Date(2026, 5, 30, 8, 0, 0, 0, time.UTC)

	ev := evidence{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		RealWatchdog:   true,
		RealEmailSent:  false, // CaptureSink only — no real email tonight
		StallWindow:    cfg.StallWindow.String(),
		PokeAckWindow:  cfg.PokeAckWindow.String(),
		TranscriptPath: transcriptPath,
	}

	// ---- Scenario 1: an ACTIVE run that goes quiet -> detect -> one poke -> incident.
	{
		clk := &frozenClock{t: t0}
		poker := queue.NewRecordingPoker(nil) // honest: no real wake-input mechanism
		sink := &queue.CaptureSink{}
		wd := queue.NewWatchdog(cfg, clk.now, queue.OSStatTranscript, poker, nil, sink)
		wd.SetTailReader(queue.OSTailTranscript)

		run := queue.WatchedRun{
			RunID:          "repro-run-active",
			TaskID:         "Task-0042",
			Repo:           "QueueDrainTestbed",
			WorktreePath:   `C:\Agent\QueueDrainTestbed\.owned\w`,
			TranscriptPath: transcriptPath,
			RunGateState:   "running",
		}

		// Initial transcript with mtime at t0 (the agent's last append).
		if err := writeTranscript(transcriptPath, 5, t0); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		// Baseline observation establishes the deadline anchor at last activity (t0).
		mustObserve(wd, run)
		anchorAtBaseline := wd.LastActiveSignalAt(run.RunID)

		// The agent appends once more at t0+2m (deadline must refresh to last activity).
		clk.advance(2 * time.Minute)
		if err := writeTranscript(transcriptPath, 6, clk.now()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		mustObserve(wd, run)
		anchorAfterAppend := wd.LastActiveSignalAt(run.RunID)

		// Now the agent goes quiet (transcript frozen). Cross the stall window -> poke.
		clk.advance(6 * time.Minute)
		mustObserve(wd, run)
		// Ack window elapses with no fresh append -> incident email (captured).
		clk.advance(6 * time.Minute)
		mustObserve(wd, run)

		anchored := "last_observed_activity"
		if anchorAfterAppend.Equal(anchorAtBaseline) {
			anchored = "NOT_refreshed_on_append (would be F-O5-signal failure)"
		}
		ev.ActiveScenario = scenarioResult{
			Description:        "active run goes quiet past the 5m stall window; one poke; ack window elapses; incident email captured",
			PokeCount:          poker.PokeCount(),
			IncidentCount:      len(sink.Incidents),
			DeadlineAnchoredTo: anchored,
		}
		if len(sink.Incidents) > 0 {
			in := sink.Incidents[0]
			ev.IncidentEmail = &in
		}
	}

	// ---- Scenario 2: a GATE-PARKED run sits idle past the window -> SILENT (A5.5).
	{
		clk := &frozenClock{t: t0}
		poker := queue.NewRecordingPoker(nil)
		sink := &queue.CaptureSink{}
		wd := queue.NewWatchdog(cfg, clk.now, queue.OSStatTranscript, poker, nil, sink)
		wd.SetTailReader(queue.OSTailTranscript)

		run := queue.WatchedRun{
			RunID:          "repro-run-parked",
			TaskID:         "Task-0043",
			Repo:           "QueueDrainTestbed",
			WorktreePath:   `C:\Agent\QueueDrainTestbed\.owned\w2`,
			TranscriptPath: transcriptPath, // same frozen-quiet transcript
			RunGateState:   "parked_research",
		}
		// Sit idle for an hour (12x the stall window) while parked.
		for i := 0; i < 12; i++ {
			mustObserve(wd, run)
			clk.advance(5 * time.Minute)
		}
		ev.ParkedScenario = scenarioResult{
			Description:        "gate-parked (parked_research) run sits idle 60m (12x the 5m window); watchdog stays silent",
			PokeCount:          poker.PokeCount(),
			IncidentCount:      len(sink.Incidents),
			DeadlineAnchoredTo: "n/a (suspended while parked)",
		}
	}

	// ---- Write evidence: JSON + the captured incident email body.
	jsonPath := filepath.Join(evidenceDir, "watchdog-repro-evidence.json")
	jsonBytes, _ := json.MarshalIndent(ev, "", "  ")
	if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if ev.IncidentEmail != nil {
		emailPath := filepath.Join(evidenceDir, "captured-incident-email.txt")
		header := fmt.Sprintf("To: %s\nSubject: %s\n\n", ev.IncidentEmail.EmailTo, ev.IncidentEmail.Subject)
		if err := os.WriteFile(emailPath, []byte(header+ev.IncidentEmail.Body), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("wrote", emailPath)
	}
	fmt.Println("wrote", jsonPath)

	// Self-check the falsifiers so a regression in the repro itself is loud.
	fail := false
	if ev.ActiveScenario.PokeCount != 1 {
		fmt.Fprintf(os.Stderr, "FAIL A5.3: active poke count = %d, want 1\n", ev.ActiveScenario.PokeCount)
		fail = true
	}
	if ev.ActiveScenario.IncidentCount != 1 {
		fmt.Fprintf(os.Stderr, "FAIL A5.3: active incident count = %d, want 1\n", ev.ActiveScenario.IncidentCount)
		fail = true
	}
	if ev.ParkedScenario.PokeCount != 0 || ev.ParkedScenario.IncidentCount != 0 {
		fmt.Fprintf(os.Stderr, "FAIL A5.5: parked run was not silent (pokes=%d incidents=%d)\n",
			ev.ParkedScenario.PokeCount, ev.ParkedScenario.IncidentCount)
		fail = true
	}
	if ev.ActiveScenario.DeadlineAnchoredTo != "last_observed_activity" {
		fmt.Fprintf(os.Stderr, "FAIL F-O5-signal: deadline anchor = %q\n", ev.ActiveScenario.DeadlineAnchoredTo)
		fail = true
	}
	if fail {
		os.Exit(1)
	}
	fmt.Println("REPRO OK: A5.2/A5.3-watchdog/A5.5 + F-O5-signal/parked all held (no real agent, no real email)")
}

func mustObserve(wd *queue.Watchdog, run queue.WatchedRun) {
	if _, err := wd.Observe(run); err != nil {
		fmt.Fprintln(os.Stderr, "observe:", err)
		os.Exit(1)
	}
}
