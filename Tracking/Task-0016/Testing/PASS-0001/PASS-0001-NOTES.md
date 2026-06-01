# PASS-0001 — Discover-on-startup (replaces prune-only reconcile)

Backend pass 2 of the Task-0016 backend batch (single-context implementation).

## Objective (TASK.md §3, Goal 3; AC8; REG-008)

Startup **enumerates** the pool member folders on disk per repo and reconstructs each
one's **idle vs allocated** state; allocated = bound to a **live run** (read from the
per-run `TaskRunWorkflow`, Task-0015 Landing-2 authority). No auto-seeding to a count;
no folder created or destroyed by discovery. **Survives a backend restart with no bound
state lost.**

## Changes

- [pool.go](../../../../backend/orchestration/internal/taskrun/pool.go):
  - `classifyPoolMember(record)` — derives status from the durable record: empty
    `run_id` ⇒ idle; a `run_id` ⇒ allocated **only while its per-run workflow is still
    live** (`runtime.GetActiveTaskRun` + `runOwnsLiveStory`); a run that has ended
    reclassifies the member to **idle** (folder kept). Never reclassifies a still-live
    allocated member as idle — the load-bearing restart-survival guarantee.
  - `ListPoolWorktrees()` — the full-pool read (idle + allocated) both discover
    assertions and the §8 endpoint (PASS-0002) use.
  - `DiscoverPool()` — enumerates pool folders, reconstructs status from disk + the live
    workflow, and **persists the corrected `run_id == ""`** back onto a record whose
    bound run has ended (so the durable record matches the reconstructed state); creates
    / destroys no folder; subsumes the existing `git worktree prune` hygiene
    (`ReconcileOwnedLanes`, kept intact) as a **best-effort** first step so a prune
    failure never aborts reconstruction.
- [queuedrain.go](../../../../backend/orchestration/internal/temporalbackend/queuedrain.go)
  L222–229 — the per-repo startup wiring now calls `service.DiscoverPool()` in place of
  the prune-only `service.ReconcileOwnedLanes()` (DiscoverPool subsumes the prune). One
  call-site change; `ReconcileOwnedLanes` is unchanged and still callable.

## Proof — Go unit tests

[pool_test.go](../../../../backend/orchestration/internal/taskrun/pool_test.go):

- `TestDiscoverPoolReconstructsAllocatedAndIdleAcrossRestart` (AC8 / REG-008) — lays an
  on-disk pool (one member bound to a live run, one idle) with one Service, then builds
  a **fresh** Service over the same roots (restart) whose runtime still reports the live
  run; asserts `ListPoolWorktrees()` reports the allocated member as `allocated` with its
  bound `run_id`/`task_id` and the idle member as `idle`, **with no bound state lost**.
  (Falsifier: a discover that reclassifies the live-allocated member as idle, or drops
  it, fails.)
- `TestDiscoverPoolReclassifiesEndedRunToIdleAndPersists` — a member whose bound run has
  ended is reconstructed as `idle`, `DiscoverPool` persists `run_id == ""` back onto the
  record, and the folder is **kept** on disk.

### Commands

```
cd C:\Agent\CodexDashboard\backend\orchestration
go build ./...        # exit 0
go test ./...         # all ok (full backend suite)
```

Producer testing; independent clean-context QA is coordinator-arranged after the
backend batch.
