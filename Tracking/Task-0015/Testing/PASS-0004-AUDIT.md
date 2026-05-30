# PASS-0004 Audit — O3: GitHub queue-drain consumer

Task: Task-0015. Pass: PASS-0004 (O3). Verdict: **READY** (O3 PASS, committed).

## Independence

Implementer and QA were SEPARATE clean-context subagents. QA re-ran `go build`/`go
test` itself and **independently re-derived the live gh-read smoke** by creating its
OWN fresh throwaway issues (#3 Ready, #4 Never) on `QueueDrainTestbed`, running the
real gh-backed provider + real `Consumer.DrainOnce`, then closing them.

## Per-criterion verdict (independent)

- **A3.1 PASS** — `Queue=Ready` dispatches via the `Dispatcher.Dispatch` seam
  (→ `taskrun.Service.Dispatch`), NO manual `/dispatch` call; also end-to-end through
  the Temporal testsuite (`QueueDrainWorkflow` polls→dispatches→stops).
- **A3.2 PASS** — `Queue=Never` and unset are skipped.
- **A3.3 PASS** — `#N → Task-%04d` exact (#12→Task-0012), consistent with the on-disk
  `Tracking/Task-NNNN` convention.
- **A3.4 PASS** — `POST /api/v1/queue/consumer/{start,stop}` (with repo +
  `poll_interval_seconds` override); `queue.Register` wires the workflow + poll
  activity in `StartWorker` next to `taskexec.Register`; default interval 2 min,
  configurable.
- **A4.3/A4.4-in-loop PASS** — closed ⇒ `Reclaim` (cleanupOwnedLane) invoked; Human
  Needed=Yes ⇒ park via `SetRunGateState`, never redispatched (even when Queue still
  reads Ready), slot retained.
- **A4.6 PASS** — the gh provider issues ONLY reads (`gh api /orgs/<owner>/issue-fields`,
  `gh issue list`, `gh api /repos/.../issues/<n>/issue-field-values`); no
  `-X POST/PATCH/PUT/DELETE`, no `issue edit/close`. The sole GitHub-write path stays
  in the obsidian-operator skill.
- **AX.1 PASS** — `go build ./...` exit 0; `go test -count=1 ./...` all `ok`; gofmt clean.
- **F-O3 NOT triggered** — Ready dispatches, Never does not, no manual call required.

## Live gh-read smoke (QA re-derived)

QA's fresh #3(Ready)→Task-0003 DISPATCHED (decision), #4(Never)→Task-0004 SKIPPED,
correct ids, against live GitHub via the real provider + `DrainOnce` (recording
dispatcher). Confirms A3.1/A3.2 at the live gh-read/decide level. The live smoke is
an opt-in test gated by `QUEUE_DRAIN_SMOKE_REPO` (never runs in normal `go test`).

The implementer's live attempt also caught + fixed a real bug: `issue-field-values`
`value` is a numeric option id (the name lives in `single_select_option`). QA
verified the decode against the live read-back (`value:74648226` +
`single_select_option.name:"Ready"`) and confirmed it matches the reconcile script.

## Genuineness / reuse / isolation

- `Consumer.DrainOnce` REUSES O4 `DecideQueueAction` + O2 `EvaluateSlot` (not
  reimplemented); `taskrunDispatcher` delegates to existing `taskrun.Service` methods
  (`Dispatch`, `SetRunGateState`, `ReclaimOwnedLane`, `ActiveOwnedLaneTasks`).
- No production Obsidian repo touched (throwaway `QueueDrainTestbed` only); no live
  lane started (`:14318` free; smoke ran as `go test` with a fake dispatcher, so no
  worktree provisioned); QA cleaned up its issues. Churn surgical, gofmt-clean.

## Honest deferral

The no-proxy real-GitHub-**UI** end-to-end A3.1 (human Chrome-debug `Queue=Ready`
flip → backend pickup with a provisioned worktree) is DEFERRED to **PASS-0006**
(per PLAN + HUMAN-DIRECTIVES). PASS-0004 proves the gh-read/decide path + the
deterministic unit behavior, not the full worktree spin-up. Verified honest.

Carry-forward: the agent-driven A4.1 (abandon ⇒ Human Needed=Yes, open) and A4.7
(agent never self-closes) land in PASS-0005 (O5), where the launched agent calls
`Set-TaskDoneContract.ps1`.

## Minor note (non-blocking)

`internal/queue/decision.go:88` comment says a unit test asserts the park strings
match the `taskrun` constants; the O4 pass added such a guard
(`TestQueueParkStatesMatchTaskrunConstants`) — QA verified the strings match
regardless, so this is at worst a stale/imprecise comment, not a functional gap.
