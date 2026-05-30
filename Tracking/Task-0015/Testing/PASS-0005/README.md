# PASS-0005 — O5 evidence (top-level agent launcher + liveness watchdog)

Scope: O5 (PASS-0005), the largest/highest-risk pass. Split into the deterministic,
fully-provable half (watchdog logic + launcher/session-discovery code) and the
risky-live half (a real autonomous agent spawning its own subagent + a real
process wake-input mechanism), which is reported PENDING for supervised completion
rather than run unbounded or faked.

## What was implemented (file:line)

- Watchdog (deterministic core):
  [../../../../backend/orchestration/internal/queue/watchdog.go](../../../../backend/orchestration/internal/queue/watchdog.go)
  - signal tracking `last_active_signal_at` refreshed on every observed append
    (`Observe`, step 2), anchored to last activity, never to dispatch (F-O5-signal)
  - parked-suspension (`Observe`, step 1; `WatchedRun.IsParked`) — A5.5
  - exactly one poke (`Observe`, step 3) reusing a `Poker` seam (prod -> PokeRun)
  - long-quiet-tool-call guard via injected `ProcessBusy` (`Observe`, step 3 guard)
  - incident + email assembly (`buildIncident` / `renderIncidentBody`) — A5.3
  - configurable thresholds (`WatchdogConfig` / `withDefaults`) — A5.4
  - production adapters `OSStatTranscript` / `OSTailTranscript`
- Launcher + POST-LAUNCH session discovery:
  [../../../../backend/orchestration/internal/queue/launcher.go](../../../../backend/orchestration/internal/queue/launcher.go)
  - `buildAgentCommand` (top-level `codex exec` in the worktree; not a subagent)
  - `Launcher.Start` (distinct OS process; bounded by an injected timeout)
  - `DiscoverSession` / `discoverCodexSession` / `discoverClaudeSession` — finds the
    launched agent's OWN session id + transcript AFTER launch (correction 2)
- O6 binding update for the discovered session:
  `taskrun.Service.BindLaunchedSession`
  [../../../../backend/orchestration/internal/taskrun/service.go](../../../../backend/orchestration/internal/taskrun/service.go)

## Deterministic proof (no real agent, no real email)

Unit tests (all PASS):
[../../../../backend/orchestration/internal/queue/watchdog_test.go](../../../../backend/orchestration/internal/queue/watchdog_test.go),
[../../../../backend/orchestration/internal/queue/launcher_test.go](../../../../backend/orchestration/internal/queue/launcher_test.go),
and the BindLaunchedSession test in
[../../../../backend/orchestration/internal/taskrun/gatestate_test.go](../../../../backend/orchestration/internal/taskrun/gatestate_test.go).

Synthetic-transcript repro driving the REAL watchdog (via `OSStatTranscript`)
against a REAL on-disk transcript with a FROZEN mtime, clock injected:
- driver: [../../../../backend/orchestration/cmd/watchdogrepro/main.go](../../../../backend/orchestration/cmd/watchdogrepro/main.go)
- evidence: [watchdog-repro-evidence.json](./watchdog-repro-evidence.json)
- captured (NOT sent) incident email: [captured-incident-email.txt](./captured-incident-email.txt)

Repro result:
- active run: 1 poke, 1 incident; `deadline_anchored_to = last_observed_activity`
  (`last_active_signal_at = 08:02` = last append, NOT 08:00 baseline/dispatch);
  detected at 08:14 (within stall 5m + poke + ack 5m of last activity) — A5.2 /
  A5.3-watchdog / F-O5-signal.
- gate-parked run (`parked_research`): 0 pokes, 0 incidents across 60m idle (12x
  the window) — A5.5 / F-O5-parked.
- the captured incident email contains BOTH (a) observed state (run id, task, repo,
  worktree, bound transcript path, last_active_signal_at, thresholds, poked,
  wake_input_delivered=false) AND (b) the transcript (inline tail + full-file path).
- `real_email_sent = false` (the sink is `CaptureSink`).

`go test ./...` from `backend/orchestration`: builds + passes (AX.1).

## PENDING for supervised completion (NOT run unbounded, NOT faked)

- A5.1 (HARD — a real top-level agent spawns >=1 of its OWN subagents): PENDING.
  - A headless `codex exec` CLI IS available (v0.135.0-alpha.1, resolved from the
    VS Code extension bin) and `~/.codex/auth.json` is present.
  - BUT `codex exec --help` exposes NO subagent/delegate/spawn subcommand or flag.
    Spawning a subagent is an emergent, model-driven behavior, not a bounded,
    deterministic operation. There is no way to guarantee a trivial bounded prompt
    spawns exactly one subagent and nothing else.
  - Launching `codex exec --dangerously-bypass-approvals-and-sandbox` with auth
    present starts a real, full-shell-access, model-backed agent running unattended
    overnight — exactly the unbounded autonomous run the task forbids. Per the
    safety rule ("if a bounded-safe launch cannot be guaranteed, STOP and report
    PENDING"), A5.1 is deferred to supervised completion. The launcher + discovery
    code is implemented and unit-tested so a supervised run only needs to execute
    `Launcher.Start` against the throwaway worktree and confirm a subagent transcript
    appeared.
- Poke "actually wake the process (deliver input)" (coordinator-review correction
  3): PENDING. `codex exec` is non-interactive with no stdin injection into a
  running process; no clean input-delivery mechanism exists. The watchdog issues the
  one poke and HONESTLY reports `wake_input_delivered = false` (it never claims the
  process was woken) and proceeds to the ack window + incident. Establishing a real
  wake-input mechanism is supervised follow-up work.
