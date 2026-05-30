# PASS-0004 (O3) — GitHub queue-drain consumer: proof

Implements O3 (Tracking/Task-0015 TASK.md "### O3", acceptance A3.1–A3.4 + the
consumer-side O4 A4.3/A4.4-in-loop). Reuses the already-built O2 `EvaluateSlot`,
O4 `DecideQueueAction`/`EffectiveFreeConcurrency`, and the O6 binding/endpoint;
does NOT reimplement them. See [../../PLAN.md](../../PLAN.md) PASS-0004.

## What was built

- Provider-read abstraction: `QueueProvider.ListReadyIssues(repo)` +
  read-only `gh`-backed production impl
  (`backend/orchestration/internal/queue/provider.go`).
- Pure, unit-testable consumer core `Consumer.DrainOnce` that applies
  `DecideQueueAction` (O4) + `EvaluateSlot` (O2), maps issue `#N` → `Task-N`
  exactly, and dispatches via the existing `taskrun` dispatch path with NO manual
  call (`backend/orchestration/internal/queue/consumer.go`).
- Temporal queue-drain workflow `QueueDrainWorkflow` (sibling to
  `TaskRunWorkflow`), polling on a configurable interval (default ~2 min),
  stoppable by signal, registered via `queue.Register` in `StartWorker` next to
  `taskexec.Register` (`backend/orchestration/internal/queue/workflow.go`,
  `backend/orchestration/internal/temporalbackend/backend.go`).
- Start/stop HTTP endpoint `POST /api/v1/queue/consumer/{start,stop}` following the
  dispatch-route pattern (`backend/orchestration/internal/httpapi/mux.go`).
- Dispatcher adapter `taskrunDispatcher` wiring the consumer to the existing
  `taskrun.Service` Dispatch / SetRunGateState / ReclaimOwnedLane (reuse, not
  reimplement) — `backend/orchestration/internal/temporalbackend/queuedrain.go`.

## TIER 1 — deterministic (fake provider). PROVEN.

`go-test-all.txt` — full `go test ./...` from `backend/orchestration` builds +
passes (AX.1). `tier1-o3-tests.txt` — verbose O3 tests:

- A3.1 `TestDrainOnceReadyIssueDispatchesWithoutManualCall` — Queue=Ready ⇒
  dispatch through the dispatch seam, no manual `/dispatch` call.
- A3.2 `TestDrainOnceNeverOrUnsetIsNotDispatched` — Queue=Never/unset ⇒ not
  dispatched.
- A3.3 `TestTaskIDForIssueMapsExactly` + `TestDrainOnceDispatchesExactTaskIDForIssueNumber`
  — `#N` → `Task-N` exact (no mapping layer).
- A3.4 `TestQueueConsumerStartStopEndpoint` (endpoint starts/stops the consumer) +
  `TestRegisterRegistersWorkflowAndActivity` (workflow + poll activity registered
  in StartWorker's `queue.Register`) + `TestQueueDrainWorkflowPollsThenDispatchesAndStops`
  (workflow polls + dispatches + stops on signal).
- A4.3/A4.4-in-loop `TestDrainOnceClosedReclaimsAndHumanNeededParks` +
  `TestDrainOnceParkedTaskIsNotRedispatchedAndRetainsSlot` — closed ⇒ terminal
  reclaim path invoked; Human Needed=Yes ⇒ park (SetRunGateState), NOT
  redispatched, slot retained.
- Slot cap `TestDrainOnceRespectsPerRepoSlotCap` — refuses a dispatch past
  queue_workers, re-picks on a later poll.

## TIER 2 — live GitHub smoke (honest). PROVEN (read path).

`tier2-live-smoke.txt` — opt-in `TestLiveGitHubQueueDrainSmoke` (gated by
`QUEUE_DRAIN_SMOKE_REPO`; never runs in normal `go test`) drove the REAL
`gh`-backed provider + the real `Consumer.DrainOnce` against the throwaway repo
`Digital-Collective-Games/QueueDrainTestbed` (local `C:\Agent\QueueDrainTestbed`):

- Setup writes (authorized on the throwaway repo only, via the established
  issue-field-values path): issue #1 `Queue=Ready` (type Task); issue #2
  `Queue=Never`.
- Result: `dispatched=[Task-0001] skipped=[Task-0002]` — the consumer READ live
  GitHub state and decided to dispatch the Ready issue (no manual call) and ignore
  the Never issue. This caught and fixed a real parse bug (the `issue-field-values`
  `value` is the numeric option id; the name is in `single_select_option`).

This proves the production gh-read + consumer decision path against LIVE GitHub
state. It does NOT spin up the full isolated-lane backend to provision a real
worktree, and it is NOT the no-proxy real-GitHub-UI end-to-end regression: that
(the human's Chrome-debug `Queue=Ready` flip → backend pickup) is **PASS-0006**.
The consumer is READ-ONLY against GitHub; the `Queue` flips above were
authorized setup writes on the throwaway repo, not the agent's queue write path.

PROVEN: deterministic A3.1–A3.4 + A4.3/A4.4-in-loop (TIER 1); live gh-read
dispatch/ignore decision (TIER 2). DEFERRED to PASS-0006: the no-proxy real
GitHub-UI A3.1 end-to-end.
