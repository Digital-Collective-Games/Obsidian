# PASS-0003 — Assign + dispatch-path change + queue_workers removal

Backend pass 4 of the Task-0016 backend batch (single-context implementation). This is
the model swap: the per-dispatch random-temp provision is replaced by **pool-draw**, and
the numeric `queue_workers` cap is removed (concurrency is bounded by the idle pool count
by construction).

## Objective (TASK.md §5, §9, §1; Goals 5, 9, 1; AC1, AC3, AC4, AC5; REG-007 "pool of 1")

## Changes

### Assign + pool-draw dispatch ([pool.go](../../../../backend/orchestration/internal/taskrun/pool.go), [service.go](../../../../backend/orchestration/internal/taskrun/service.go))
- `drawIdlePoolWorktree(worktreeID)` — picks an idle member (named, or any idle when empty
  for the consumer auto-assign path), resets its **existing** checkout to baseline
  (`restoreOwnedLane`), and returns a `RepoLane` pointed at the stable checkout. Empty
  pool ⇒ `ErrNoIdleWorktree`. **No fresh dir is provisioned.**
- `startRunInDrawnLane(...)` — the shared bootstrap→start tail (extracted from
  `dispatchWithDirective`) that both dispatch and Assign reuse; on a successful start it
  marks the drawn pool member **allocated** (`markPoolMemberRun` records the run id). A
  pre-start failure leaves the member **idle** (never deletes a pool worktree).
- `dispatchWithDirective` now calls `drawIdlePoolWorktree("")` (pool-draw) instead of
  `provisionOwnedLane` — the queue-drain consumer's `Dispatch` flows through this, so the
  consumer auto-assign draws **any** idle worktree; an empty pool defers.
- `AssignTaskToPoolWorktree(ctx, taskID, repo, worktreeID)` — the manual Assign: draws the
  named (or any) idle worktree, resets it, and starts the run **in it** via the same tail.
  `POST /api/v1/worktrees/assign` (202; 409 `ErrNoIdleWorktree` when none idle).
- **Release returns pool members to idle, not delete.** `releasePreviousOwnedLane` (dispatch
  supersede) and `releaseResolvedOwnedLane` (interrupt-review resolve) now call
  `returnPoolMemberToIdle` (clear `run_id`, keep folder) for a pool member, falling back to
  the legacy delete path only for a non-pool (random-temp) lane. This makes a same-task
  redispatch reuse the SAME pool worktree.
- **Pool layout fix:** the checkout leaf is the member folder name (`wt-<NNNN>`) not a shared
  `w`, so each pool worktree's `git worktree` admin name is unique per repo (a shared `w`
  leaf collided in `.git/worktrees`). The durable pool record sits in the parent member
  folder (sibling of the checkout, so reset/clean never wipes it). `poolRepoSegment` hashes
  a raw declared-root fallback to a short stable segment so the pool path never blows past
  the OS `$GIT_DIR` limit.

### queue_workers removal (the cap → pool-count migration)
- Dropped `QueueWorkers` from [`RepoEntry`](../../../../backend/orchestration/internal/queue/manifest.go)
  (a legacy key is now simply ignored), `QueueWorkersForRoot`, `DefaultQueueWorkers`,
  `EvaluateSlot`/`SlotDecision` ([slots.go](../../../../backend/orchestration/internal/queue/slots.go)
  reduced to the package doc), `EffectiveFreeConcurrency`
  ([decision.go](../../../../backend/orchestration/internal/queue/decision.go)),
  `RegistryRepo.QueueWorkers`, `Service.RepoSlotLimit` / `manifestQueueWorkers` /
  `repoSlotLimit`, and the `queueWorkers` param to `NewServiceForRepo`.
- The consumer's `SlotSizer.RepoSlotLimit()` seam became `PoolSizer.IdleWorktreeCount()`
  ([consumer.go](../../../../backend/orchestration/internal/queue/consumer.go)); admission
  is now "is there an idle pool worktree to draw?" — the idle budget is read once per poll
  and decremented per dispatch (a reclaim returns one to the budget). `Service` implements
  it via `IdleWorktreeCount`/`countIdlePoolWorktrees`. The manual dispatch gate
  (`repoSlotBlock`) blocks with `no_idle_worktree` on an empty pool.
- [queuedrain.go](../../../../backend/orchestration/internal/temporalbackend/queuedrain.go)
  drops the `repo.QueueWorkers` arg; the per-repo Service still satisfies the (now pool)
  sizer seam.

## Proof — Go unit tests (no real GitHub)

- [slots_test.go](../../../../backend/orchestration/internal/taskrun/slots_test.go) (rewritten
  for the pool model): pool-draw gate admits while idle remains / refuses `no_idle_worktree`
  on empty; refuse-then-admit-after-idle-frees; no `active_run_exists` for a sibling while
  idle remains; same-task re-dispatch stays blocked; two siblings draw distinct idle
  members (idle→0); **AC3** dispatch reuses the existing idle worktree with **no fresh dir /
  no pool growth**; **AC4** dispatch refused on empty pool.
- [consumer_test.go](../../../../backend/orchestration/internal/queue/consumer_test.go):
  `fixedSizer`→`fixedIdleSizer`; **AC1** dispatches up to the idle pool then waits;
  **REG-007 "pool of 1"** dispatches exactly one of two Ready issues; empty pool defers
  (AC5). [registry_consumer_test.go](../../../../backend/orchestration/internal/queue/registry_consumer_test.go)
  reworked to per-repo idle-pool accounting (RepoA 2 idle ⇒ both dispatch; RepoB 0 idle ⇒
  none — per-repo isolation, REG-009 shape).
- [worktrees_pool_test.go](../../../../backend/orchestration/internal/httpapi/worktrees_pool_test.go):
  endpoint Assign binds an idle worktree (202, same path/allocated) then **409** when none
  idle; assign route method-guarded.
- All other existing dispatch tests updated to **seed an idle pool worktree** before
  dispatching (the model now requires a pre-created pool worktree) via `seedIdleWorktree` /
  `seedWorktreeViaMux`; the interrupt-review/redispatch tests updated to the pool semantics
  (worktree KEPT/returned-to-idle and REUSED, not deleted+fresh).
- **AC1 grep proof:** `grep -rn 'RepoSlotLimit|EvaluateSlot|EffectiveFreeConcurrency|QueueWorkersForRoot|\.QueueWorkers' internal/ cmd/ | grep -v _test.go`
  returns only doc-comment mentions — no live admission cap anywhere.

### Commands

```
cd C:\Agent\CodexDashboard\backend\orchestration
gofmt -l internal/   # (no output — clean)
go build ./...        # exit 0
go test ./...         # all ok (full backend suite)
```

Producer testing; independent clean-context QA is coordinator-arranged after the backend
batch.

## Caveat carried forward

The consumer's terminal **reclaim-on-close** path (`ReclaimOwnedLane` → `cleanupOwnedLane`)
is intentionally **unchanged** in this pass (Non-Goals: "does not change when the consumer
reclaims") and still deletes the checkout for a closed issue. PASS-0005 introduces Eject's
keep-folder/return-idle mechanic. The pool record of a reclaimed-by-close member is
reconciled by discover-on-startup (PASS-0001) on the next backend start.
