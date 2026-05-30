# PASS-0006 Audit — Cross-cutting regression (REG-007) + BUG-0001 fix

Task: Task-0015. Pass: PASS-0006. Verdict: **READY** (REG-007 end-to-end PASS;
BUG-0001 fixed; committed).

## Independence

A separate clean-context QA worker re-ran `go build`/`go vet`/`go test ./...`,
**re-ran the `github-operator` skill live against the real GitHub UI**, reviewed
the BUG-0001 fix diff, the TESTING.md / REGRESSION.md / SKILL.md docs, and the
REG-007 evidence — cross-checking it against live state.

## Verdict (independent)

- **REG-007 end-to-end PASS** — agent-driven real GitHub web-UI `Queue=Ready` flip
  (CDP) → consumer `poll acted ... dispatched [Task-0005]` (≤30s, isolated `reg007b`
  namespace) → worktree + O6 binding (session `bcd2a9b2…`) → the launched top-level
  claude agent **persisted and ran** (transcript at the bound path, AGENT-RAN.txt =
  "reg-007 agent launched ok"). QA cross-checked the live transcript: it grew
  46KB→76KB, corroborating that the agent outlived the dispatch activity (the fix
  works). Evidence: [PASS-0006/REG-007-PROOF.md](./PASS-0006/REG-007-PROOF.md) +
  `PASS-0006/evidence/`.
- **BUG-0001 fix correct** — `liveLauncher.Launch` now launches under
  `context.Background()` (detached) instead of the Temporal `queue.drain.poll`
  activity ctx, which had been killing the agent the instant the activity returned
  (binding written, no transcript). Orphan-lifecycle + launch-logging are documented
  O5 follow-ups, not blockers. See [../BUG-0001.md](../BUG-0001.md).
- **github-operator skill works (live, real UI)** — QA re-ran
  `Set-IssueFieldViaUi` → Ready (button `QueueReady`, API `Ready`) and Never
  (`QueueNever`/`Never`); `Get-IssueQueueState` → `Queue -> Never`. The flip is a
  real CDP browser-UI click of the field control + option (not an API write).
- **Cadence** — `DefaultPollInterval = 30s` (≤1 min); no test pinned the old 2-min.
- **AX.1** — `go build`/`go vet`/`go test -count=1 ./...` all green.
- **Docs** — TESTING.md "Issue-Provider Integration Testing (GitHub web surface)"
  codifies the standard: exercised at the real GitHub web UI via the Chrome debug
  session (`github-operator`); human only authenticates; agent drives end-to-end;
  **no pass/excuse to skip end-to-end** (API/proxy does not satisfy). REGRESSION.md
  REG-007 matches the case format; SKILL.md accurate, ASCII-clean.

## Isolation

All on the throwaway `QueueDrainTestbed` repo + an isolated `reg007*` Temporal
namespace (the real cron `default` namespace was never used — per the human
directive not to touch scheduled jobs). No real email. #5 left at `Never`.

## Remaining (follow-ups, not blockers)

- O5 hardening: kill the launched agent PID on owned-lane reclaim/shutdown (orphan
  lifecycle) + add launcher launch/error logging (BUG-0001 follow-up).
- O5 A5.3: a full REAL-agent stall→detect→resume-poke→incident-email repro
  (the watchdog logic + the resume-poke + the launch are each proven; the integrated
  real-agent stall scenario was not separately reproduced).
- Task CLOSURE: a distinct, final human gate (the agent never self-closes).
