# O5 Liveness Signal — Research Artifact (Task-0015)

Status: RESEARCH COMPLETE (signal pinned). This artifact satisfies the
embedded research item the task carves out (`../TASK.md` A5.2 / falsifier
F-O5-signal). It is produced single-context by the TaskDispatch task leader
because no nested research-leader dispatch tool was available in this runtime;
the same phase discipline and durable-artifact standard was applied. See
[../HANDOFF.md](../HANDOFF.md).

## The Question (the KNOWN UNKNOWN)

> What observable signal reliably distinguishes "actively working / thinking"
> from "fell asleep / idle-waiting" for a HEADLESS coding-agent process, fast
> enough that the operator would know within ~1 min and safe enough to act on at
> ~5 min?

This is load-bearing: the entire O5 watchdog (detect → poke → incident email)
and its parked-suspension behavior depend on it. A watchdog that flips to
"asleep" on a fixed timer with no evidence-backed signal is explicitly a
disqualified proxy (`../TASK.md` "Current fallback / proxy that must not be
mistaken for success"; falsifier F-O5-signal).

## Candidate Signals Evaluated

The task named four candidates. Each is assessed against: observable for a
HEADLESS process, low false-"asleep" rate, low false-"alive" rate, sampling
cost, and cross-runtime portability (Codex AND Claude, per `REPO-MANIFEST.json`
running both).

### C1 — Session transcript / log file append growth (CHOSEN PRIMARY)

Evidence gathered on this machine (real transcripts):

- Claude writes an append-only JSONL transcript per session under
  `~/.claude/projects/<slug>/<session-id>.jsonl`, plus per-subagent transcripts
  under `.../<session-id>/subagents/agent-*.jsonl`. Each line is a JSON event
  carrying a per-line RFC3339 `timestamp`, `sessionId`, `cwd`, `requestId`, and
  a `type` (`assistant`, `user`, `queue-operation`, `file-history-snapshot`,
  ...). The last line of a live CodexDashboard session had
  `timestamp = 2026-05-29T21:25:42.001Z`, `type = assistant`.
- Codex writes an append-only JSONL "rollout" transcript per session under
  `~/.codex/sessions/YYYY/MM/DD/rollout-<ts>-<uuid>.jsonl`. Each line carries a
  per-line RFC3339 `timestamp`, a `type` (`session_meta`, `turn_context`,
  `event_msg`, `response_item`), and a typed `payload`. A live session's last
  line had `timestamp = 2026-05-29T08:06:16.176Z`, `type = event_msg`.

Why it separates the two states:

- An actively-working agent emits transcript events continuously while it
  reasons, calls tools, and reads tool results — each event appends a line and
  advances the file's size and mtime. Tool calls and model streaming produce
  events on a sub-minute cadence in normal operation.
- A "fell asleep / idle-waiting" headless agent emits NOTHING: no new line, no
  size growth, no mtime advance, and (critically per the human directive) it
  does NOT spontaneously resume. So a flat file size + stale mtime + stale
  last-line timestamp for the detection window is a high-confidence "asleep"
  signal.

Sampling method (pinned):

- Resolve the bound transcript path for the run from the owned-lane
  session binding (O6 — `repo`, `issue/Task`, worktree path, agent session id,
  transcript path). This couples O5 and O6: the watchdog reads the SAME
  transcript path the binding records.
- Sample two cheap OS facts on a fixed poll (proposed ~30 s, configurable):
  the file's `size` and `mtime` (a single `stat`). Maintain a
  `last_active_signal_at` = the wall-clock time at which size/mtime last
  advanced. This is O(1) per poll, requires no transcript parsing, and never
  touches the agent.
- Optional precision upgrade (not required for the signal to work): when the
  last line is cheaply tailable, read its `timestamp`/`type` so the incident
  email can report the last event kind and its in-transcript time. The
  size+mtime delta alone is sufficient for the detect decision.
- Detection rule: the run is "stale/asleep candidate" when
  `now - last_active_signal_at > stall_window` (proposed ~5 min, configurable)
  AND the run is NOT parked on a human gate (see Parked Suspension below). The
  deadline is anchored to LAST OBSERVED ACTIVITY, never to dispatch time — this
  is exactly what falsifier F-O5-signal forbids.

Risks / mitigations:

- Output buffering could delay flushes. Mitigation: JSONL transcripts observed
  here are line-flushed append logs (lines land as events occur); the ~5 min
  window is far larger than any per-line flush latency, and the threshold is
  configurable to absorb a slow runtime.
- A long single tool call (e.g. a multi-minute build) could look idle on the
  transcript. Mitigation #1: the conservative ~5 min window already tolerates
  most tool calls; #2 (secondary corroboration, see C2) the watchdog may also
  observe the agent process / its child build process as busy before declaring
  sleep. The PRIMARY decision remains the transcript signal; process state is
  corroboration only, so the watchdog never declares sleep purely on a timer.
- Path portability: Codex and Claude transcript locations differ, but BOTH are
  append-only JSONL with per-line timestamps and observable size/mtime, so the
  SAME size+mtime sampling works for both once the binding records the correct
  path. The binding (O6) is the abstraction that makes the signal
  runtime-agnostic.

### C2 — Process busy/idle (CHOSEN SECONDARY / CORROBORATION ONLY)

The dispatched top-level agent is a real OS process (headless `codex`/`claude`).
Its liveness (process exists; optionally CPU busy or has a live child process
such as a build/test) is observable via the OS. Useful as corroboration to
avoid a false "asleep" during a legitimately long, transcript-quiet tool call,
and to distinguish "agent crashed / exited" (process gone) from "agent idle but
alive" (process present, transcript flat). NOT used as the primary signal
because an idle-waiting agent process can still be "present" and even briefly
CPU-busy without making progress; process-present alone would under-detect the
exact "fell asleep mid-work" failure the human described.

### C3 — In-flight model request (REJECTED as primary)

An active model HTTP request is the most precise "thinking right now" signal and
is conceptually what the IDE shows. REJECTED as the watchdog primary because it
is NOT cheaply observable for a headless third-party agent process from the
Temporal side without intercepting the agent's network or instrumenting the
agent — which violates the hard requirement that supervision be EXTERNAL and
INVISIBLE to the agent (`../TASK.md` O5; D3). The transcript signal is the
externally-observable PROXY for "a turn is in flight": a turn that is making
model/tool progress appends transcript lines.

### C4 — stdout activity (REJECTED as primary)

Headless stdout is capturable, but for these runtimes the durable, structured,
per-line-timestamped record of activity is the JSONL transcript, not raw stdout;
stdout can be empty for long stretches even while the transcript grows. Using
the transcript subsumes the useful part of stdout while being structured and
already bound per-run via O6.

## Decision (PINNED)

- PRIMARY signal: append growth (size + mtime advance) of the run's bound
  session-transcript JSONL file, tracked as `last_active_signal_at`.
- SECONDARY corroboration: agent process presence / busy-or-has-live-child,
  used only to avoid a false "asleep" during a long transcript-quiet tool call
  and to tell "crashed/exited" from "idle but alive."
- The "asleep" decision = transcript flat for `> stall_window` AND not
  gate-parked AND (process not demonstrably busy on a long child operation).
- The detection deadline is refreshed by the signal (reset on every observed
  append), never anchored to dispatch time.

## How This Wires Into the Watchdog (for the implementer)

- The watchdog reuses `StateEnvelope.SuspiciousAfter` + the `staleRunUpdate`
  transition to `sleeping_or_stalled`
  (`backend/orchestration/internal/taskrun/service.go:1254-1268`) and the
  existing `PokeRun` `poke_worker_check` follow-up
  (`service.go:226-255`), but changes the meaning of `SuspiciousAfter` from
  "dispatch + 15 min" (`taskexec.go:81,165,296`) to
  "last_active_signal_at + stall_window (~5 min)", and REFRESHES it whenever the
  transcript appends. This is the concrete fix that turns the existing
  fixed-timer behavior into an evidence-backed liveness window.
- Parked suspension: when the run's GitHub issue `Human Needed=Yes` (or the
  owned-lane run/gate state shared with O4/O6 says parked), the watchdog skips
  the `staleRunUpdate` transition and the poke entirely — no stall, no poke, no
  email — for the full parked duration (`../TASK.md` A5.5 / F-O5-parked).
- Poke = one `poke_worker_check` with an ack window (~5 min, configurable) that
  attempts to actually wake the process (deliver input) and asks it to write a
  durable stop update (set `Human Needed=Yes` or request closure) or resume.
- Confirmed sleep (ack window elapsed with no fresh append) → incident logged +
  email to `admin@digitalcollective.games` via the `gmail-digest-email` skill /
  local gmail MCP, containing BOTH (a) the watchdog's observed state
  (run id, task/issue, worktree path, bound transcript path,
  `last_active_signal_at`, last event kind/time, process state, thresholds) and
  (b) the agent's session transcript (`../TASK.md` A5.3 / F-O5-detect+email).

## Open Implementation Choices (do NOT change the chosen signal)

- Exact poll cadence (proposed ~30 s) and stall window (~5 min) — both
  configurable, fixed by acceptance behavior not by a magic number (A5.4).
- Whether the watchdog runs as a Temporal monitor activity/child or a heartbeat
  timer inside the queue package — implementation detail; the signal is the
  same either way.
- Whether to read the last transcript line for richer email content (precision
  upgrade) vs size+mtime only (sufficient for detection).

## Provenance

- Real transcript shapes inspected on this machine on 2026-05-29:
  - Claude: `~/.claude/projects/c--Agent-CodexDashboard/<session>.jsonl`
    (+ `subagents/agent-*.jsonl`), per-line `timestamp`, typed events.
  - Codex: `~/.codex/sessions/2026/05/29/rollout-*.jsonl`, per-line
    `timestamp`, typed `payload` events.
- Existing reusable backend primitives: `service.go:226-255` (`PokeRun`),
  `service.go:1254-1268` (`staleRunUpdate`), `service.go:1214-1222`
  (overdue follow-up escalation), `taskexec.go:81,165,296` (the +15 min
  `SuspiciousAfter` this task narrows to last-activity-anchored ~5 min),
  `types.go:47,186` (`SuspiciousAfter`, `LastProgressAt`).
- Human directive D4 + the O5 revision (suspended-while-parked):
  [../HUMAN-DIRECTIVES-FOR-WORKER.md](../HUMAN-DIRECTIVES-FOR-WORKER.md).
