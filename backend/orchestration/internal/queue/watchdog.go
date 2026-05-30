package queue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// O5 external liveness watchdog. This is the DETERMINISTIC, fully unit-testable
// core of the "fell asleep mid-work" detector. It implements the signal pinned in
// Research/LIVENESS-SIGNAL.md: append growth (size + mtime advance) of the run's
// bound session-transcript JSONL, tracked as last_active_signal_at and REFRESHED
// on every observed append. The detection deadline is anchored to LAST OBSERVED
// ACTIVITY, never to dispatch time (F-O5-signal).
//
// Everything the watchdog touches is injected so NO real agent and NO real email
// are needed to test it (and so a real email is NEVER sent on the proof lane):
//   - Clock supplies the wall-clock now.
//   - TranscriptStat reports a transcript file's size + mtime (one stat).
//   - Poker delivers the single wake/poke (production wires it to taskrun.PokeRun;
//     a confirmed sleep additionally tries to deliver input — see WakeInput).
//   - ProcessProbe optionally reports whether the agent process is busy / has a
//     live busy child (the C2 corroboration), so a long quiet tool call (a
//     multi-minute build) is not misread as sleep.
//   - IncidentSink receives the assembled incident + email (MOCK/CAPTURE only).
//
// internal/queue stays a leaf package: the watchdog operates over these seams and
// over the parked-state strings (mirrored in decision.go), never importing taskrun.

// Clock is the injected wall clock. Production passes time.Now; tests freeze it.
type Clock func() time.Time

// TranscriptSample is the cheap OS fact the watchdog samples each poll: the bound
// transcript file's size and mtime. Exists is false when the file is not present
// yet (e.g. immediately after launch, before the agent's first append).
type TranscriptSample struct {
	Exists  bool
	Size    int64
	ModTime time.Time
}

// TranscriptStat samples the transcript at path. It must be cheap (a single stat)
// and must never parse or read the transcript body for the detection decision.
type TranscriptStat func(path string) (TranscriptSample, error)

// ProcessBusy reports whether the agent process for a run is demonstrably busy on
// a long operation (itself CPU-busy or has a live busy child such as a build), so
// the watchdog does not declare sleep during a legitimately long, transcript-quiet
// tool call (the C2 corroboration, LIVENESS-SIGNAL.md). A nil probe means "no
// corroboration available"; the watchdog then relies on the stall window alone,
// which is set comfortably above expected tool-call duration.
type ProcessBusy func(runID string) (busy bool, alive bool)

// Poker delivers the single wake/poke for a stalled run. In production Poke wires
// to taskrun.PokeRun (mutates run state + records the poke_worker_check follow-up),
// and DeliverWake attempts to actually wake the process (deliver input). Tests use
// a recorder. DeliverWake returning (false, nil) means "no wake-input mechanism is
// available" (the honest default tonight) — the watchdog then treats the poke as a
// state-only nudge and proceeds to the ack window and incident, never claiming the
// process was actually woken.
type Poker interface {
	// Poke records the one wake/poke against the run (poke_worker_check follow-up).
	Poke(runID string) error
	// DeliverWake attempts to deliver an actual wake input to the headless agent
	// process. It returns delivered=true only if an input was really delivered.
	// When no input-delivery mechanism exists it returns (false, nil) so the
	// watchdog does not pretend the process was woken.
	DeliverWake(runID string) (delivered bool, err error)
}

// IncidentSink receives the assembled incident + email on a confirmed sleep. It is
// ALWAYS a mock/capture on the proof lane and in tests (no real email tonight).
type IncidentSink interface {
	Emit(incident Incident) error
}

// WatchdogConfig holds every threshold (A5.4: configurable, not magic numbers).
type WatchdogConfig struct {
	// PollInterval is the cadence at which Observe is expected to be called
	// (informational; the watchdog decides on now - last_active_signal_at, not on
	// poll counts). Default DefaultWatchdogPoll.
	PollInterval time.Duration
	// StallWindow is the no-active-signal duration after which a non-parked run is
	// a stall candidate (~5 min). Default DefaultStallWindow.
	StallWindow time.Duration
	// PokeAckWindow is how long the watchdog waits after the single poke for a
	// fresh append before confirming the sleep and emitting the incident (~5 min).
	// Default DefaultPokeAckWindow.
	PokeAckWindow time.Duration
	// HumanEscalationAfter is the no-signal duration at which the incident attention
	// escalates to urgent (~30 min). Default DefaultHumanEscalation.
	HumanEscalationAfter time.Duration
	// TranscriptTailEvents is how many trailing transcript events the incident email
	// embeds inline (the full transcript is linked by path, not dumped inline).
	// Default DefaultTranscriptTailEvents.
	TranscriptTailEvents int
}

// Watchdog default thresholds. All overridable via WatchdogConfig (A5.4).
const (
	DefaultWatchdogPoll         = 30 * time.Second
	DefaultStallWindow          = 5 * time.Minute
	DefaultPokeAckWindow        = 5 * time.Minute
	DefaultHumanEscalation      = 30 * time.Minute
	DefaultTranscriptTailEvents = 40
)

func (c WatchdogConfig) withDefaults() WatchdogConfig {
	if c.PollInterval <= 0 {
		c.PollInterval = DefaultWatchdogPoll
	}
	if c.StallWindow <= 0 {
		c.StallWindow = DefaultStallWindow
	}
	if c.PokeAckWindow <= 0 {
		c.PokeAckWindow = DefaultPokeAckWindow
	}
	if c.HumanEscalationAfter <= 0 {
		c.HumanEscalationAfter = DefaultHumanEscalation
	}
	if c.TranscriptTailEvents <= 0 {
		c.TranscriptTailEvents = DefaultTranscriptTailEvents
	}
	return c
}

// WatchedRun is the per-run input the watchdog observes each poll. It is built from
// the O6 worktree<->session binding (the watchdog reads the SAME transcript path
// the binding records) plus the run's current parked/running gate state (O4). The
// watchdog NEVER injects a supervision-aware signal into the agent — it only reads
// these externally observable facts (D3: supervision is external + invisible).
type WatchedRun struct {
	// RunID is the task-run id (used for poke + incident correlation).
	RunID string
	// TaskID is the issue #N / Task-N (incident context).
	TaskID string
	// Repo is the declared repo id (incident context).
	Repo string
	// WorktreePath is the owned worktree checkout (incident context).
	WorktreePath string
	// TranscriptPath is the bound session-transcript JSONL the watchdog stats.
	TranscriptPath string
	// RunGateState is the current run/gate state (one of the runGateStateParked*
	// strings while parked, or "" / running while active). A parked run SUSPENDS
	// the watchdog entirely (A5.5 / F-O5-parked).
	RunGateState string
}

// IsParked reports whether the run is parked on a human gate (Human Needed=Yes).
// While parked the watchdog produces NO stall transition, NO poke, and NO email.
func (r WatchedRun) IsParked() bool {
	switch r.RunGateState {
	case runGateStateParkedAwaitingClosure,
		runGateStateParkedResearch,
		runGateStateParkedPlan,
		runGateStateParkedRegression:
		return true
	default:
		return false
	}
}

// runWatch is the watchdog's per-run mutable tracking state.
type runWatch struct {
	lastSize           int64
	haveSample         bool
	lastActiveSignalAt time.Time // refreshed on every observed append (the deadline anchor)
	poked              bool
	pokeAt             time.Time
	wakeDelivered      bool
	incidentEmitted    bool
}

// Incident is the assembled stall incident. The IncidentSink turns it into the log
// entry + email to admin@digitalcollective.games (A5.3 / F-O5-detect+email). It
// carries BOTH (a) the observed state and (b) the agent's session transcript
// (a tail inline + the full-file path/link — never an impossible multi-MB dump).
type Incident struct {
	// EmailTo is the incident email recipient.
	EmailTo string
	// Subject / Body render the human-facing incident email.
	Subject string
	Body    string
	// Observed state (also rendered into Body; kept structured for tests/proof).
	RunID              string
	TaskID             string
	Repo               string
	WorktreePath       string
	TranscriptPath     string
	LastActiveSignalAt time.Time
	DetectedAt         time.Time
	StallWindow        time.Duration
	PokeAckWindow      time.Duration
	Poked              bool
	WakeDelivered      bool
	ProcessAlive       bool
	Escalated          bool // true once no-signal duration passes HumanEscalationAfter
	// Transcript content: a tail of the last events inline + the full-file path.
	TranscriptTailLines []string
	TranscriptFullPath  string
}

// IncidentRecipient is the operator the incident email targets (D4 / A5.3).
const IncidentRecipient = "admin@digitalcollective.games"

// ObserveResult summarizes what a single Observe step did (for proof/logging).
type ObserveResult struct {
	Suspended       bool // run parked -> watchdog silent
	SignalAdvanced  bool // transcript appended since last poll -> deadline refreshed
	Poked           bool // single wake/poke issued this step
	IncidentEmitted bool
}

// Watchdog is the stateful external liveness watchdog. Call Observe once per run
// per poll cycle. It is concurrency-naive on purpose (driven from one poll loop /
// one Temporal monitor activity at a time, like the queue consumer).
type Watchdog struct {
	cfg     WatchdogConfig
	clock   Clock
	stat    TranscriptStat
	poker   Poker
	probe   ProcessBusy
	sink    IncidentSink
	watches map[string]*runWatch
	// tail reads up to n trailing transcript lines for the incident email. Optional:
	// nil leaves the tail empty (size+mtime alone drive detection). Injected so the
	// email-content path is testable without a real transcript reader.
	tail func(path string, n int) ([]string, error)
}

// NewWatchdog builds a watchdog with injected dependencies. clock and stat are
// required; poker/probe/sink/tail may be nil (a nil sink means a confirmed sleep is
// tracked but not emitted — used only in narrow tests; production always injects a
// capture/mock sink).
func NewWatchdog(cfg WatchdogConfig, clock Clock, stat TranscriptStat, poker Poker, probe ProcessBusy, sink IncidentSink) *Watchdog {
	return &Watchdog{
		cfg:     cfg.withDefaults(),
		clock:   clock,
		stat:    stat,
		poker:   poker,
		probe:   probe,
		sink:    sink,
		watches: map[string]*runWatch{},
	}
}

// SetTailReader injects the optional transcript-tail reader used for incident email
// content (the precision upgrade in LIVENESS-SIGNAL.md). Detection never depends on
// it; only the email body's inline tail does.
func (w *Watchdog) SetTailReader(tail func(path string, n int) ([]string, error)) {
	w.tail = tail
}

// Forget drops a run's tracking state (e.g. after it is reclaimed/closed).
func (w *Watchdog) Forget(runID string) {
	delete(w.watches, runID)
}

// LastActiveSignalAt returns the last-observed-append time tracked for a run (the
// deadline anchor). Zero when the run has not been observed yet. Exposed for proof.
func (w *Watchdog) LastActiveSignalAt(runID string) time.Time {
	if rw, ok := w.watches[runID]; ok {
		return rw.lastActiveSignalAt
	}
	return time.Time{}
}

// Observe performs one watchdog step for a single run. The decision flow:
//  1. Parked -> SUSPEND: no stall, no poke, no email (A5.5). Tracking is left so
//     the deadline re-anchors to "now" when the run un-parks (no false instant
//     stall on resume).
//  2. Sample size+mtime. First sample (or an append since last sample) REFRESHES
//     last_active_signal_at — the deadline is anchored to last activity, refreshed
//     on append, NEVER to dispatch time (F-O5-signal).
//  3. No append for > StallWindow AND not process-busy on a long child -> stall
//     candidate. Issue exactly ONE poke (Poke + DeliverWake attempt).
//  4. After the poke, if no fresh append within PokeAckWindow -> confirmed sleep:
//     assemble + emit the incident (log + email with state + transcript). Emitted
//     exactly once.
func (w *Watchdog) Observe(run WatchedRun) (ObserveResult, error) {
	now := w.clock()
	rw := w.watches[run.RunID]
	if rw == nil {
		rw = &runWatch{}
		w.watches[run.RunID] = rw
	}

	// (1) Parked -> suspend entirely. Re-anchor the deadline to now so the run does
	// not instantly trip the stall window the moment the human clears the gate.
	if run.IsParked() {
		rw.lastActiveSignalAt = now
		rw.poked = false
		rw.pokeAt = time.Time{}
		rw.wakeDelivered = false
		rw.incidentEmitted = false
		return ObserveResult{Suspended: true}, nil
	}

	// (2) Sample the transcript and refresh the deadline on any append.
	sample, err := w.stat(run.TranscriptPath)
	if err != nil {
		return ObserveResult{}, fmt.Errorf("stat transcript %s: %w", run.TranscriptPath, err)
	}
	result := ObserveResult{}
	advanced := false
	if !rw.haveSample {
		// First observation establishes the baseline. If the file does not exist
		// yet (agent has not appended), anchor to now and wait for the first append.
		rw.haveSample = true
		rw.lastSize = sample.Size
		rw.lastActiveSignalAt = now
		advanced = true
	} else if sample.Exists && (sample.Size != rw.lastSize) {
		rw.lastSize = sample.Size
		rw.lastActiveSignalAt = now
		advanced = true
	}
	if advanced {
		// A fresh append clears any in-flight stall: reset the poke/ack/incident.
		rw.poked = false
		rw.pokeAt = time.Time{}
		rw.wakeDelivered = false
		rw.incidentEmitted = false
		result.SignalAdvanced = true
		return result, nil
	}

	quietFor := now.Sub(rw.lastActiveSignalAt)

	// (3) Not yet past the stall window -> nothing to do.
	if quietFor <= w.cfg.StallWindow {
		return result, nil
	}

	// Long-quiet-tool-call guard (C2): if the agent process is demonstrably busy on
	// a long operation, do NOT declare sleep — treat it as still alive and refresh
	// the activity anchor so a multi-minute build is not misread as a stall.
	busy, alive := false, true
	if w.probe != nil {
		busy, alive = w.probe(run.RunID)
		if busy {
			rw.lastActiveSignalAt = now
			rw.poked = false
			rw.pokeAt = time.Time{}
			rw.incidentEmitted = false
			result.SignalAdvanced = true
			return result, nil
		}
	}

	// (3) Stall candidate: issue exactly ONE poke (Poke + a wake-input attempt).
	if !rw.poked {
		if w.poker != nil {
			if err := w.poker.Poke(run.RunID); err != nil {
				return result, fmt.Errorf("poke run %s: %w", run.RunID, err)
			}
			delivered, derr := w.poker.DeliverWake(run.RunID)
			if derr != nil {
				return result, fmt.Errorf("deliver wake to run %s: %w", run.RunID, derr)
			}
			rw.wakeDelivered = delivered
		}
		rw.poked = true
		rw.pokeAt = now
		result.Poked = true
		return result, nil
	}

	// (4) Poke already issued: wait for the ack window. A fresh append would have
	// been caught at step (2) and reset rw.poked. If the ack window elapses with no
	// fresh append, confirm the sleep and emit the incident exactly once.
	if rw.incidentEmitted {
		return result, nil
	}
	if now.Sub(rw.pokeAt) < w.cfg.PokeAckWindow {
		return result, nil
	}

	incident := w.buildIncident(run, rw, now, alive, quietFor)
	if w.sink != nil {
		if err := w.sink.Emit(incident); err != nil {
			return result, fmt.Errorf("emit incident for run %s: %w", run.RunID, err)
		}
	}
	rw.incidentEmitted = true
	result.IncidentEmitted = true
	return result, nil
}

// buildIncident assembles the incident + email: (a) the observed state and (b) the
// agent's session transcript (a tail inline + the full-file path). The full
// transcript is referenced by path, never dumped inline (coordinator-review caveat).
func (w *Watchdog) buildIncident(run WatchedRun, rw *runWatch, now time.Time, alive bool, quietFor time.Duration) Incident {
	escalated := quietFor >= w.cfg.HumanEscalationAfter

	var tail []string
	if w.tail != nil && run.TranscriptPath != "" {
		if lines, err := w.tail(run.TranscriptPath, w.cfg.TranscriptTailEvents); err == nil {
			tail = lines
		}
	}

	incident := Incident{
		EmailTo:             IncidentRecipient,
		RunID:               run.RunID,
		TaskID:              run.TaskID,
		Repo:                run.Repo,
		WorktreePath:        run.WorktreePath,
		TranscriptPath:      run.TranscriptPath,
		LastActiveSignalAt:  rw.lastActiveSignalAt,
		DetectedAt:          now,
		StallWindow:         w.cfg.StallWindow,
		PokeAckWindow:       w.cfg.PokeAckWindow,
		Poked:               rw.poked,
		WakeDelivered:       rw.wakeDelivered,
		ProcessAlive:        alive,
		Escalated:           escalated,
		TranscriptTailLines: tail,
		TranscriptFullPath:  run.TranscriptPath,
	}
	incident.Subject = fmt.Sprintf("[codex watchdog] agent stalled: %s (%s)", run.TaskID, run.RunID)
	incident.Body = renderIncidentBody(incident)
	return incident
}

// renderIncidentBody renders the operator-facing incident email body containing
// BOTH the observed state and the transcript (tail inline + full-file link).
func renderIncidentBody(in Incident) string {
	var b strings.Builder
	fmt.Fprintf(&b, "A dispatched agent stopped emitting transcript activity and did not\n")
	fmt.Fprintf(&b, "respond to one wake/poke within the ack window. Observed state:\n\n")
	fmt.Fprintf(&b, "  run id:               %s\n", in.RunID)
	fmt.Fprintf(&b, "  task / issue:         %s\n", in.TaskID)
	fmt.Fprintf(&b, "  repo:                 %s\n", in.Repo)
	fmt.Fprintf(&b, "  worktree:             %s\n", in.WorktreePath)
	fmt.Fprintf(&b, "  bound transcript:     %s\n", in.TranscriptPath)
	fmt.Fprintf(&b, "  last_active_signal_at:%s\n", in.LastActiveSignalAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "  detected_at:          %s\n", in.DetectedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "  stall_window:         %s\n", in.StallWindow)
	fmt.Fprintf(&b, "  poke_ack_window:      %s\n", in.PokeAckWindow)
	fmt.Fprintf(&b, "  poked:                %t\n", in.Poked)
	fmt.Fprintf(&b, "  wake_input_delivered: %t\n", in.WakeDelivered)
	fmt.Fprintf(&b, "  process_alive:        %t\n", in.ProcessAlive)
	fmt.Fprintf(&b, "  escalated_urgent:     %t\n", in.Escalated)
	fmt.Fprintf(&b, "\nSession transcript (full file):\n  %s\n", in.TranscriptFullPath)
	if len(in.TranscriptTailLines) > 0 {
		fmt.Fprintf(&b, "\nLast %d transcript events (inline tail):\n", len(in.TranscriptTailLines))
		for _, line := range in.TranscriptTailLines {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	} else {
		fmt.Fprintf(&b, "\n(Transcript tail not inlined; open the full file above.)\n")
	}
	return b.String()
}

// CaptureSink is a mock IncidentSink that records emitted incidents instead of
// sending real email. Production proof and all tests use it (no real email tonight).
type CaptureSink struct {
	Incidents []Incident
}

// Emit records the incident.
func (c *CaptureSink) Emit(incident Incident) error {
	c.Incidents = append(c.Incidents, incident)
	return nil
}

// RecordingPoker is a mock Poker that records poke + wake-input attempts. wakeFn
// controls whether DeliverWake reports a delivered input (nil wakeFn -> not
// delivered, the honest default when no input-delivery mechanism exists).
type RecordingPoker struct {
	Pokes  []string
	Wakes  []string
	wakeFn func(runID string) (bool, error)
}

// NewRecordingPoker builds a recording poker. Pass nil wakeFn for the
// no-wake-mechanism default (DeliverWake -> false, nil).
func NewRecordingPoker(wakeFn func(runID string) (bool, error)) *RecordingPoker {
	return &RecordingPoker{wakeFn: wakeFn}
}

func (p *RecordingPoker) Poke(runID string) error {
	p.Pokes = append(p.Pokes, runID)
	return nil
}

func (p *RecordingPoker) DeliverWake(runID string) (bool, error) {
	p.Wakes = append(p.Wakes, runID)
	if p.wakeFn == nil {
		return false, nil
	}
	return p.wakeFn(runID)
}

// PokeCount / WakeCount are convenience accessors for tests/proof.
func (p *RecordingPoker) PokeCount() int { return len(p.Pokes) }
func (p *RecordingPoker) WakeCount() int { return len(p.Wakes) }

// WakeTarget is what a run's poke needs to wake the headless claude agent: the
// resolved claude executable, the agent's session id, and the owned worktree cwd.
// The production poker resolves this from the O6 binding (run id -> bound session +
// worktree); the watchdog itself never holds it (D3: external, invisible).
type WakeTarget struct {
	Executable   string
	SessionID    string
	WorktreePath string
}

// WakeRunner runs a wake command (executable, args, cwd) and reports whether it
// exited 0. It is injected so DeliverWake is testable with a fake exit-0 runner (no
// real claude process). The production runner execs `claude --resume`.
type WakeRunner func(ctx context.Context, executable string, args []string, cwd string) (exitCode int, err error)

// ClaudeResumePoker is the production Poker: Poke records the one wake/poke against
// the run (via the injected record func, wired in production to taskrun.PokeRun), and
// DeliverWake actually wakes the agent by running `claude --resume <session-id> -p
// "<WakeMessage>"` in the worktree with the same env overrides as launch — the REAL
// wake-input delivery (it resumes the same session and appends to the same
// transcript). DeliverWake returns delivered=true only on exit 0.
type ClaudeResumePoker struct {
	// resolve maps a run id to its wake target (executable + session id + worktree).
	resolve func(runID string) (WakeTarget, error)
	// record records the one poke against the run (production: taskrun.PokeRun). May
	// be nil (the wake is still delivered).
	record func(runID string) error
	// run executes the wake command. Required.
	run WakeRunner
	// ctx bounds each wake run. Nil uses context.Background.
	ctx context.Context
}

// NewClaudeResumePoker builds the production poker. run is required; resolve maps the
// run id to its wake target; record (optional) records the poke against run state.
func NewClaudeResumePoker(resolve func(runID string) (WakeTarget, error), record func(runID string) error, run WakeRunner) *ClaudeResumePoker {
	return &ClaudeResumePoker{resolve: resolve, record: record, run: run}
}

// Poke records the one wake/poke against the run (poke_worker_check follow-up). The
// actual wake-input delivery happens in DeliverWake.
func (p *ClaudeResumePoker) Poke(runID string) error {
	if p.record == nil {
		return nil
	}
	return p.record(runID)
}

// DeliverWake runs `claude --resume <session-id> -p "<WakeMessage>"` in the run's
// worktree to actually wake the headless agent and append to its session transcript.
// It returns delivered=true only when the resume exits 0.
func (p *ClaudeResumePoker) DeliverWake(runID string) (bool, error) {
	if p.resolve == nil || p.run == nil {
		return false, nil
	}
	target, err := p.resolve(runID)
	if err != nil {
		return false, fmt.Errorf("resolve wake target for run %s: %w", runID, err)
	}
	exe, args, err := buildWakeCommand(target.Executable, target.SessionID, WakeMessage)
	if err != nil {
		return false, err
	}
	ctx := p.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	code, err := p.run(ctx, exe, args, target.WorktreePath)
	if err != nil {
		return false, fmt.Errorf("deliver wake to run %s: %w", runID, err)
	}
	return code == 0, nil
}

// RunClaudeWake is the production WakeRunner: it execs the claude resume-wake command
// in cwd with the same env overrides as launch (claudeChildEnv) and returns its exit
// code. Stdout/stderr are discarded (the wake's only durable effect is the transcript
// append, which the watchdog detects on its next poll).
func RunClaudeWake(ctx context.Context, executable string, args []string, cwd string) (int, error) {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = cwd
	cmd.Env = claudeChildEnv(os.Environ())
	err := cmd.Run()
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// A non-zero exit is reported via the code, not as a runner error, so the
			// poker can report delivered=false without treating it as a fault.
			return code, nil
		}
		return code, err
	}
	return code, nil
}

// OSStatTranscript is the production TranscriptStat: a single os.Stat returning the
// transcript file's size + mtime (the cheap O(1) sample LIVENESS-SIGNAL.md pins). A
// missing file is reported as Exists=false (not an error) so a just-launched run
// whose first append has not landed yet does not fault the watchdog.
func OSStatTranscript(path string) (TranscriptSample, error) {
	if path == "" {
		return TranscriptSample{}, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return TranscriptSample{Exists: false}, nil
		}
		return TranscriptSample{}, err
	}
	return TranscriptSample{Exists: true, Size: info.Size(), ModTime: info.ModTime()}, nil
}

// OSTailTranscript reads up to n trailing lines of a transcript for the incident
// email's inline tail (the precision upgrade; detection never depends on it). It
// reads the whole file and keeps the last n lines — fine for the email path, which
// runs only on a confirmed stall, not every poll.
func OSTailTranscript(path string, n int) ([]string, error) {
	if path == "" || n <= 0 {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

// SortedRunIDs returns the run ids the watchdog is tracking (proof helper).
func (w *Watchdog) SortedRunIDs() []string {
	ids := make([]string, 0, len(w.watches))
	for id := range w.watches {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
