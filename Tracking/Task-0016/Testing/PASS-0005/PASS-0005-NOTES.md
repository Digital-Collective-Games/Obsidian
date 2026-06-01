# PASS-0005 — Eject (keep folder + return idle + dequeue) + no-bounce-back seam

Backend pass 6 of the Task-0016 backend batch (single-context implementation).

## Objective (TASK.md §6; Goal 6; AC6, AC13, AC14)

Eject stops the agent, cleans the checkout to a true baseline, unbinds the run, **keeps
the folder** (returns it idle), and **dequeues the freed task** so the still-`Ready` task
is **not** re-dispatched. Works regardless of parked state. Never deletes the folder,
never closes the issue.

## Changes

- [pool.go](../../../../backend/orchestration/internal/taskrun/pool.go)
  `EjectWorktree(ctx, runID, worktreeID)`:
  - resolves the target pool member by `run_id` (the record whose `RunID == runID`) or by
    `worktree_id` (accepted alternate);
  - terminates the launched agent (`terminateAgentProcess` PID kill, the same BUG-0002
    terminate-before-touch `ReclaimOwnedLane` performs), reading the live PID from the
    bound run's binding when available;
  - cleans the checkout to a TRUE baseline via the new `restoreOwnedLaneFull` (`reset
    --hard` + `git clean -fdx` — drops ignored files), **keeping the folder**;
  - sets the pool record `run_id=""` (idle) — it does **not** call
    `removeOwnedLaneWorktree` / delete the folder;
  - then **dequeues** the freed task via `DequeueTask` (PASS-0004 provider write,
    `Queue→Never`), resolving the task id from the live binding or the run id; a safe no-op
    when there is no provider-backed task; it never closes the issue;
  - returns the now-idle `PoolWorktree`.
  - Works on a `running` AND a `parked_*` lane (it does not require a parked run, unlike
    `resolve-interrupt-review`).
- [service.go](../../../../backend/orchestration/internal/taskrun/service.go): split the
  reset clean into `restoreOwnedLaneWithClean(repoLane, cleanFlags)`; `restoreOwnedLane`
  keeps `-fd` (Assign/dispatch — **not weakened**), `restoreOwnedLaneFull` uses `-fdx`
  (Eject).
- [mux.go](../../../../backend/orchestration/internal/httpapi/mux.go): `POST
  /api/v1/worktrees/eject {run_id|worktree_id}` on the method/path-guarded worktree
  sub-router (200; returns the now-idle worktree; 404 unknown).

## Proof — Go unit tests (no real GitHub)

- [slots_test.go](../../../../backend/orchestration/internal/taskrun/slots_test.go):
  - `TestEjectKeepsFolderReturnsIdleAndDequeues` (AC6/AC14) — Create → Dispatch (allocate)
    → Eject: the **folder still exists** on disk, the worktree reads `idle`, the pool
    record `run_id` is empty, and the fake dequeue provider was called for issue #1
    (Task-0001) — never closed. (A test that finds the folder deleted, or the dequeue not
    called, fails.)
  - `TestEjectWorksWhileParked` — Eject works on a `parked_*` lane too (folder kept, idle).
- [consumer_test.go](../../../../backend/orchestration/internal/queue/consumer_test.go)
  `TestEjectThenNoBounceBackOnNextPoll` (AC13, consumer+service seam) — first poll
  dispatches the Ready issue; an Eject frees the lane AND dequeues through the fake
  provider (`Queue→Never`); the **next poll does NOT re-dispatch** (no bounce-back). The
  **load-bearing variant** (Eject that SKIPS the dequeue, leaving the issue `Ready`) shows
  the task **is** re-dispatched — proving the dequeue is what prevents the bounce-back.
- [provider_test.go](../../../../backend/orchestration/internal/queue/provider_test.go)
  (from PASS-0004) proves the dequeue write never closes (AC14).
- [worktrees_pool_test.go](../../../../backend/orchestration/internal/httpapi/worktrees_pool_test.go)
  `TestWorktreeEjectReturnsIdleAndDequeues` (AC6/AC13 endpoint) — Create → Assign(Task-0008)
  → Eject: the worktree returns to idle in the pool view and the freed task is dequeued
  (issue #8) via the injected provider; eject route method-guarded.

### Falsifier guards covered
- An Eject that **deletes** the folder fails `TestEjectKeepsFolderReturnsIdleAndDequeues`
  (the folder must still exist) — AC6.
- An Eject that leaves the issue `Queue=Ready` so the consumer re-dispatches it fails
  `TestEjectThenNoBounceBackOnNextPoll` — AC13.
- An Eject that closes the issue fails the PASS-0004 provider test (it fatals on
  `issue close`) — AC14.

### Commands

```
cd C:\Agent\CodexDashboard\backend\orchestration
gofmt -l internal/   # (no output — clean)
go build ./...        # exit 0
go test ./...         # all ok
```

Producer testing; independent clean-context QA is coordinator-arranged after the backend
batch (PASS-0006 is the backend cross-cut + server-only smoke).
