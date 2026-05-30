package queue

import (
	"strings"
	"testing"
	"time"
)

// fakeClock is an injectable, advanceable wall clock for the watchdog tests so the
// deterministic detect/poke/ack windows are exercised with NO real time and NO real
// agent.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

// fakeTranscript is an injectable transcript whose size+mtime the watchdog stats.
// Append() grows it (simulating a fresh transcript line); the watchdog reads only
// size+mtime, never the body, exactly as LIVENESS-SIGNAL.md pins the signal.
type fakeTranscript struct {
	exists bool
	size   int64
	mtime  time.Time
}

func (f *fakeTranscript) append(now time.Time) {
	f.exists = true
	f.size += 128
	f.mtime = now
}

func (f *fakeTranscript) stat(_ string) (TranscriptSample, error) {
	return TranscriptSample{Exists: f.exists, Size: f.size, ModTime: f.mtime}, nil
}

// newTestWatchdog wires a watchdog over a fake clock, fake transcript, recording
// poker, and capture sink — so detect/poke/incident+email are all observable with
// no real agent and no real email.
func newTestWatchdog(t *testing.T, cfg WatchdogConfig) (*Watchdog, *fakeClock, *fakeTranscript, *RecordingPoker, *CaptureSink) {
	t.Helper()
	clk := &fakeClock{t: time.Date(2026, 5, 30, 8, 0, 0, 0, time.UTC)}
	tr := &fakeTranscript{}
	poker := NewRecordingPoker(nil) // nil wakeFn: honest "no wake-input mechanism" default
	sink := &CaptureSink{}
	wd := NewWatchdog(cfg, clk.now, tr.stat, poker, nil, sink)
	return wd, clk, tr, poker, sink
}

func runOf(state string) WatchedRun {
	return WatchedRun{
		RunID:          "run-1",
		TaskID:         "Task-0042",
		Repo:           "CodexDashboard",
		WorktreePath:   `C:\Agent\QueueDrainTestbed\.owned\w`,
		TranscriptPath: `C:\Users\gregs\.codex\sessions\2026\05\30\rollout-x.jsonl`,
		RunGateState:   state,
	}
}

// A5.2 / F-O5-signal: a fresh append REFRESHES the deadline so an actively-working
// run never false-stalls, and the deadline is anchored to last activity, NOT to the
// first observation / dispatch.
func TestWatchdogFreshAppendRefreshesDeadlineNoFalseStall(t *testing.T) {
	wd, clk, tr, poker, sink := newTestWatchdog(t, WatchdogConfig{StallWindow: 5 * time.Minute, PokeAckWindow: 5 * time.Minute})
	run := runOf(runGateStateRunning)

	// First observation establishes the baseline anchor at t0.
	tr.append(clk.now())
	if _, err := wd.Observe(run); err != nil {
		t.Fatalf("observe t0: %v", err)
	}
	anchor0 := wd.LastActiveSignalAt(run.RunID)

	// The agent keeps appending every 2 min for 30 min (well past the stall window
	// in absolute time) — it must NEVER stall because each append refreshes.
	for i := 0; i < 15; i++ {
		clk.advance(2 * time.Minute)
		tr.append(clk.now())
		res, err := wd.Observe(run)
		if err != nil {
			t.Fatalf("observe iter %d: %v", i, err)
		}
		if !res.SignalAdvanced {
			t.Fatalf("iter %d: expected signal advance on fresh append", i)
		}
	}
	if poker.PokeCount() != 0 {
		t.Fatalf("active run was poked %d times; must be 0 while appending", poker.PokeCount())
	}
	if len(sink.Incidents) != 0 {
		t.Fatalf("active run produced %d incidents; must be 0 while appending", len(sink.Incidents))
	}
	// The deadline anchor advanced with activity (NOT pinned to t0 / dispatch).
	if !wd.LastActiveSignalAt(run.RunID).After(anchor0) {
		t.Fatalf("deadline anchor must advance with appends (F-O5-signal); anchor0=%s now=%s",
			anchor0, wd.LastActiveSignalAt(run.RunID))
	}
}

// A5.3 (watchdog half): a stall past the window yields exactly ONE poke, then —
// after the ack window with no fresh append — an incident email to the operator
// containing BOTH the observed state AND the transcript reference.
func TestWatchdogStallProducesOnePokeThenIncidentEmail(t *testing.T) {
	wd, clk, tr, poker, sink := newTestWatchdog(t, WatchdogConfig{StallWindow: 5 * time.Minute, PokeAckWindow: 5 * time.Minute})
	wd.SetTailReader(func(_ string, n int) ([]string, error) {
		return []string{`{"type":"event_msg","timestamp":"2026-05-30T07:59:00Z"}`}, nil
	})
	run := runOf(runGateStateRunning)

	// Baseline append, then the agent goes quiet.
	tr.append(clk.now())
	if _, err := wd.Observe(run); err != nil {
		t.Fatalf("baseline observe: %v", err)
	}

	// Within the stall window: no poke yet.
	clk.advance(4 * time.Minute)
	res, _ := wd.Observe(run)
	if res.Poked {
		t.Fatalf("poked before stall window elapsed")
	}

	// Past the stall window: exactly one poke.
	clk.advance(2 * time.Minute) // total quiet = 6 min > 5 min window
	res, _ = wd.Observe(run)
	if !res.Poked {
		t.Fatalf("expected a poke once the stall window elapsed")
	}
	// A second observe still inside the ack window must NOT poke again (one poke).
	clk.advance(2 * time.Minute)
	res, _ = wd.Observe(run)
	if res.Poked {
		t.Fatalf("watchdog poked more than once")
	}
	if res.IncidentEmitted {
		t.Fatalf("incident emitted before ack window elapsed")
	}

	// Ack window elapses with no fresh append -> confirmed sleep -> incident email.
	clk.advance(4 * time.Minute) // total since poke = 6 min > 5 min ack window
	res, _ = wd.Observe(run)
	if !res.IncidentEmitted {
		t.Fatalf("expected an incident after the ack window elapsed")
	}

	if poker.PokeCount() != 1 {
		t.Fatalf("expected exactly one poke, got %d", poker.PokeCount())
	}
	if poker.WakeCount() != 1 {
		t.Fatalf("expected exactly one wake-input attempt, got %d", poker.WakeCount())
	}
	if len(sink.Incidents) != 1 {
		t.Fatalf("expected exactly one incident email, got %d", len(sink.Incidents))
	}
	in := sink.Incidents[0]

	// Email targets the operator.
	if in.EmailTo != IncidentRecipient {
		t.Fatalf("incident email to %q, want %q", in.EmailTo, IncidentRecipient)
	}
	// (a) observed state present + correct.
	if in.RunID != run.RunID || in.TaskID != run.TaskID || in.WorktreePath != run.WorktreePath {
		t.Fatalf("incident missing observed state: %#v", in)
	}
	if in.LastActiveSignalAt.IsZero() || in.StallWindow == 0 || !in.Poked {
		t.Fatalf("incident missing observed thresholds/state: %#v", in)
	}
	// honest "no wake mechanism" -> wake not delivered (correction 3: not faked).
	if in.WakeDelivered {
		t.Fatalf("wake should not be reported delivered when no mechanism exists")
	}
	// (b) transcript: full-file path + an inline tail (not an impossible full dump).
	if in.TranscriptFullPath != run.TranscriptPath {
		t.Fatalf("incident missing full transcript path: %q", in.TranscriptFullPath)
	}
	if len(in.TranscriptTailLines) == 0 {
		t.Fatalf("incident missing transcript tail")
	}
	// Body renders both halves.
	for _, want := range []string{run.RunID, run.TaskID, run.WorktreePath, run.TranscriptPath, "last_active_signal_at", "Session transcript"} {
		if !strings.Contains(in.Body, want) {
			t.Fatalf("incident body missing %q:\n%s", want, in.Body)
		}
	}
}

// A5.5 (HARD) / F-O5-parked: a gate-parked run sits idle WELL past the stall window
// and the watchdog stays completely silent — no stall, no poke, no email.
func TestWatchdogGateParkedRunStaysSilentPastStallWindow(t *testing.T) {
	for _, parked := range []string{
		runGateStateParkedAwaitingClosure,
		runGateStateParkedResearch,
		runGateStateParkedPlan,
		runGateStateParkedRegression,
	} {
		t.Run(parked, func(t *testing.T) {
			wd, clk, tr, poker, sink := newTestWatchdog(t, WatchdogConfig{StallWindow: 5 * time.Minute, PokeAckWindow: 5 * time.Minute})
			run := runOf(parked)

			// Establish a baseline append, then never append again.
			tr.append(clk.now())
			if _, err := wd.Observe(run); err != nil {
				t.Fatalf("baseline: %v", err)
			}
			// Sit idle for an hour (12x the stall window) while parked.
			for i := 0; i < 12; i++ {
				clk.advance(5 * time.Minute)
				res, err := wd.Observe(run)
				if err != nil {
					t.Fatalf("parked observe iter %d: %v", i, err)
				}
				if !res.Suspended {
					t.Fatalf("iter %d: parked run must report Suspended", i)
				}
				if res.Poked || res.IncidentEmitted {
					t.Fatalf("iter %d: parked run must never poke or emit", i)
				}
			}
			if poker.PokeCount() != 0 {
				t.Fatalf("parked run was poked %d times; must be 0 (A5.5)", poker.PokeCount())
			}
			if len(sink.Incidents) != 0 {
				t.Fatalf("parked run produced %d incidents; must be 0 (A5.5)", len(sink.Incidents))
			}
		})
	}
}

// A run un-parked after sitting idle must NOT instantly false-stall: the deadline
// re-anchors to "now" on resume (a parked run is not "asleep").
func TestWatchdogResumeAfterParkDoesNotInstantlyStall(t *testing.T) {
	wd, clk, tr, poker, _ := newTestWatchdog(t, WatchdogConfig{StallWindow: 5 * time.Minute, PokeAckWindow: 5 * time.Minute})
	run := runOf(runGateStateRunning)

	tr.append(clk.now())
	wd.Observe(run)

	// Park and sit idle for 30 min.
	parked := run
	parked.RunGateState = runGateStateParkedResearch
	for i := 0; i < 6; i++ {
		clk.advance(5 * time.Minute)
		wd.Observe(parked)
	}
	// Un-park; the very next observe (no new append yet) must not poke — the
	// deadline re-anchored on resume.
	res, _ := wd.Observe(run)
	if res.Poked {
		t.Fatalf("resume immediately stalled; deadline did not re-anchor on un-park")
	}
	if poker.PokeCount() != 0 {
		t.Fatalf("poked %d times right after un-park; want 0", poker.PokeCount())
	}
}

// A5.4: thresholds are configurable, not hard-coded. A tight 1-min stall window
// trips faster than the 5-min default would.
func TestWatchdogThresholdsAreConfigurable(t *testing.T) {
	wd, clk, tr, poker, _ := newTestWatchdog(t, WatchdogConfig{StallWindow: 1 * time.Minute, PokeAckWindow: 1 * time.Minute})
	run := runOf(runGateStateRunning)

	tr.append(clk.now())
	wd.Observe(run)

	// At 90s quiet (> 1 min configured window, < 5 min default) it must poke.
	clk.advance(90 * time.Second)
	res, _ := wd.Observe(run)
	if !res.Poked {
		t.Fatalf("configured 1-min window did not trip at 90s quiet")
	}
	if poker.PokeCount() != 1 {
		t.Fatalf("expected one poke under the tight window, got %d", poker.PokeCount())
	}
}

// withDefaults must fill every zero threshold from the documented defaults (A5.4
// safety: a misconfigured/empty config never pins a window to zero).
func TestWatchdogConfigDefaults(t *testing.T) {
	got := WatchdogConfig{}.withDefaults()
	if got.StallWindow != DefaultStallWindow ||
		got.PokeAckWindow != DefaultPokeAckWindow ||
		got.HumanEscalationAfter != DefaultHumanEscalation ||
		got.PollInterval != DefaultWatchdogPoll ||
		got.TranscriptTailEvents != DefaultTranscriptTailEvents {
		t.Fatalf("withDefaults did not fill defaults: %#v", got)
	}
}

// Long-quiet-tool-call guard (coordinator-review caveat): when the injected probe
// reports the agent process busy on a long child operation, the watchdog must NOT
// declare sleep even past the stall window.
func TestWatchdogProcessBusyGuardSuppressesStall(t *testing.T) {
	clk := &fakeClock{t: time.Date(2026, 5, 30, 8, 0, 0, 0, time.UTC)}
	tr := &fakeTranscript{}
	poker := NewRecordingPoker(nil)
	sink := &CaptureSink{}
	busy := func(_ string) (bool, bool) { return true, true } // busy on a long child
	wd := NewWatchdog(WatchdogConfig{StallWindow: 5 * time.Minute, PokeAckWindow: 5 * time.Minute},
		clk.now, tr.stat, poker, busy, sink)
	run := runOf(runGateStateRunning)

	tr.append(clk.now())
	wd.Observe(run)

	// Transcript quiet for 20 min, but the process is busy on a build the whole time.
	for i := 0; i < 4; i++ {
		clk.advance(5 * time.Minute)
		if _, err := wd.Observe(run); err != nil {
			t.Fatalf("busy observe iter %d: %v", i, err)
		}
	}
	if poker.PokeCount() != 0 {
		t.Fatalf("busy-on-long-child run was poked %d times; the guard must suppress it", poker.PokeCount())
	}
	if len(sink.Incidents) != 0 {
		t.Fatalf("busy-on-long-child run produced %d incidents; the guard must suppress them", len(sink.Incidents))
	}
}

// A fresh append DURING the ack window cancels the in-flight incident: the agent
// woke on its own, so no incident email is sent (avoids a false stall report).
func TestWatchdogFreshAppendDuringAckWindowCancelsIncident(t *testing.T) {
	wd, clk, tr, poker, sink := newTestWatchdog(t, WatchdogConfig{StallWindow: 5 * time.Minute, PokeAckWindow: 5 * time.Minute})
	run := runOf(runGateStateRunning)

	tr.append(clk.now())
	wd.Observe(run)

	clk.advance(6 * time.Minute) // stall -> poke
	if res, _ := wd.Observe(run); !res.Poked {
		t.Fatalf("expected poke after stall window")
	}
	// The agent appends again during the ack window -> recovered (the in-flight
	// stall is cancelled before the ack window could confirm a sleep).
	clk.advance(2 * time.Minute)
	tr.append(clk.now())
	res, _ := wd.Observe(run)
	if !res.SignalAdvanced {
		t.Fatalf("fresh append during ack window should advance the signal")
	}
	// The recovered run keeps working: no incident is ever emitted from the first
	// stall, and the single poke is not repeated while it stays active.
	for i := 0; i < 6; i++ {
		clk.advance(2 * time.Minute)
		tr.append(clk.now())
		wd.Observe(run)
	}
	if len(sink.Incidents) != 0 {
		t.Fatalf("recovered run produced %d incidents; the first stall must be cancelled", len(sink.Incidents))
	}
	if poker.PokeCount() != 1 {
		t.Fatalf("expected exactly one poke (the cancelled first stall), got %d", poker.PokeCount())
	}
}

// The duplicated parked-state strings the watchdog branches on must match the
// taskrun constants (mirrored via decision.go's runGateStateParked* — the package
// already has a drift guard test in taskrun; this asserts the watchdog uses them).
func TestWatchedRunIsParkedCoversAllParkedStates(t *testing.T) {
	for _, s := range []string{
		runGateStateParkedAwaitingClosure,
		runGateStateParkedResearch,
		runGateStateParkedPlan,
		runGateStateParkedRegression,
	} {
		if !(WatchedRun{RunGateState: s}).IsParked() {
			t.Fatalf("state %q should be parked", s)
		}
	}
	for _, s := range []string{"", runGateStateRunning, "bogus"} {
		if (WatchedRun{RunGateState: s}).IsParked() {
			t.Fatalf("state %q should NOT be parked", s)
		}
	}
}
