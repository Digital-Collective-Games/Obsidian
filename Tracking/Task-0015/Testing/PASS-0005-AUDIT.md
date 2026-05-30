# PASS-0005 Audit — O5: Liveness watchdog + top-level agent launch (PARTIAL)

Task: Task-0015. Pass: PASS-0005 (O5). Verdict: **PARTIAL — deterministic scope
PASS (committed); A5.1 (HARD) + poke-wake PENDING supervised completion.**

This is the largest, highest-risk pass. Per the overnight safety rule, the
deterministic watchdog + launcher/discovery CODE was implemented + independently
verified, but the genuinely risky/under-specified pieces (launching a real
autonomous agent; delivering wake input to a running headless agent) were NOT run
unattended and are honestly deferred — not faked.

## Independence

Implementer and QA were SEPARATE clean-context subagents. QA re-ran `go build`/`go
test`, wrote its OWN harness driving the real watchdog against a real on-disk
transcript with a frozen mtime, and re-ran the implementer's repro. No real agent
launched; no real email sent.

## PROVEN now (deterministic, independently re-derived)

- **A5.2 (HARD) PASS** — the pinned signal (transcript size+mtime →
  `last_active_signal_at`) is read by the watchdog via `OSStatTranscript`, not a
  fixed timer. **F-O5-signal NOT triggered**: QA confirmed the deadline anchors to
  the last observed append (e.g. dispatch+3m), never dispatch time, and refreshes
  on every append (a continuously-appending run never false-stalls).
- **A5.5 (HARD) PASS** — a run in any `parked_*` gate state is FULLY suspended (0
  pokes, 0 incidents, 0 emails) past the window, and re-anchors on un-park.
  **F-O5-parked NOT triggered.**
- **A5.3 watchdog-half PASS** — detect within window → exactly ONE poke (injected
  `Poker` seam) → after the ack window → ONE incident emailed to
  `admin@digitalcollective.games` via an INJECTED capture sink (mock, never a real
  send), body carrying BOTH observed state AND the transcript (inline tail +
  full-file path).
- **A5.4 PASS** — all thresholds are `WatchdogConfig` fields with defaults;
  configurability test trips a 1-min window at 90s.
- **Long-build guard PASS** — a `ProcessBusy` guard suppresses a false stall while
  the agent process is busy on a long child (the C2 corroboration).
- **AX.1 PASS** — `go build ./...` + `go test -count=1 ./...` clean; `go vet` +
  `gofmt` clean. The 11 watchdog + 5 launcher/discovery tests are substantive.

## Code implemented + unit-tested (activation deferred)

- Launcher (`internal/queue/launcher.go`): `buildAgentCommand` builds a TOP-LEVEL
  `codex exec -C <worktree>` (not nested); `DiscoverSession`/`discoverCodex`/
  `discoverClaude` implement the POST-LAUNCH session discovery
  (coordinator-review correction 2) — newest session/rollout file created at/after
  the launch instant, matched by cwd. `taskrun.Service.BindLaunchedSession` writes
  the discovered session id + transcript into the O6 binding, replacing the
  dispatch-context placeholders. Unit-tested with synthetic sessions dirs.
- Headless CLI availability: `codex.exe` present + authed; `claude` not on PATH.

## PENDING supervised completion (honestly deferred — NOT faked)

- **A5.1 (HARD)** — a real top-level agent spawning ≥1 of its OWN subagents.
  `codex exec` exposes no bounded subagent-spawn op; spawning is emergent
  model-driven behavior, and launching `--dangerously-bypass-approvals-and-sandbox`
  unattended is the unbounded autonomous run the task forbids. The launcher +
  discovery code is ready; a SUPERVISED run executes `Launcher.Start` against the
  throwaway `C:\Agent\QueueDrainTestbed` worktree and confirms a subagent transcript
  appeared. (F-O5-toplevel + the real-agent half of A5.3/F-O5-detect+email ride on
  this.)
- **Poke "actually wake" input delivery** (coordinator-review correction 3) —
  `codex exec` is non-interactive with no stdin injection into a running process;
  no clean mechanism exists. `DeliverWake` honestly returns `false`
  (`wake_input_delivered=false`); the wake-input mechanism is a design item for the
  supervised session.

## Isolation / churn

No real email, no real agent, no live lane, no production repo touched. Diffs to
existing files are purely additive (decision.go +1 const; service.go +24
`BindLaunchedSession`; gatestate_test.go +42), gofmt/vet clean. Leaf-package
discipline preserved; the `TestQueueParkStatesMatchTaskrunConstants` drift guard
passes (the queue mirror strings match `taskrun/types.go` exactly).

## What remains for the supervised O5 finish

1. Launch a real bounded top-level agent and capture it spawning a subagent (A5.1).
2. Choose/implement a real wake-input delivery so `DeliverWake` can return true.
3. Wire launcher → `DiscoverSession` → `BindLaunchedSession` and the watchdog → a
   real gmail incident sink into the live consumer, then run the real-agent stall
   repro (the real-agent half of A5.3).
