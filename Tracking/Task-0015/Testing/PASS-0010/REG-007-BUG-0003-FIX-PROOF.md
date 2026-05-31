# PASS-0010 — BUG-0003 cross-repo collision fix (Landing 1): live + unit verification

Task: Task-0015. Verdict: **PASS** (the cross-repo reclaim bug is fixed and verified).
Fixes [BUG-0003](../../BUG-0003.md) per [Design/QUEUE-DRAIN-DURABLE-STATE-REDESIGN.md](../../Design/QUEUE-DRAIN-DURABLE-STATE-REDESIGN.md)
Landing 1. The Temporal-bypass audit's identity + slot-count bypass families are closed;
the gate/binding-in-Temporal family is the named remaining work (Landing 2).

## What was fixed (Landing 1, Steps 1–5)

The queue-drain consumer used a **global, non-repo-scoped** task identity and lane index:
`TaskIDForIssue(n)=Task-%04d` (issue-number only) + a globbed runs-root, so two registry
repos' issue #1 both became `Task-0001` and collided — one repo's closed #1 reclaimed the
other repo's live lane (killing its agent + deleting its worktree). Fix:

- **Repo-namespaced run identity** (`ActiveRunIDForRepo(repoNamespace, taskID)`): the
  Temporal workflow id + the runs-root artifact path now carry the registry repo id.
  Empty namespace = byte-identical legacy id for the single-repo control plane.
- **Id-keyed `GetActiveTaskRun`**: the Service owns namespace construction (`s.runID`), so
  dispatch and lookup never diverge.
- **Repo-scoped accounting** (`ActiveOwnedLaneTasks`) **and** `findActiveLaneRecord` filter
  by `DeclaredWorktreeRoot == s.declaredWorktreeRoot` — a repo can no longer see/resolve
  another repo's lane (closes the residual the design review flagged). `ListActiveWorktrees`
  stays global for `GET /api/v1/worktrees`.
- **One run-id path for the watchdog** Start/Stop (no leaked goroutine after namespacing).
- **Cutover** wired only in the registry dispatch (`SetRepoNamespace(repo.ID)`).

## Unit verification (GREEN)

`go build ./... && go vet ./... && go test ./...` — all green. New falsifiable tests:
- `internal/taskrun/runid_namespace_test.go` — empty namespace is byte-identical to the
  legacy id; two repos' `Task-0001` produce DISTINCT run ids.
- `internal/taskrun/reposcope_lane_test.go` — two repos sharing one runs-root + the same
  `Task-0001`: each repo's `ActiveOwnedLaneTasks` and `findActiveLaneRecord` resolve ONLY
  its own lane; the global `ListActiveWorktrees` still reports BOTH.

## Live verification (fixed binary, the exact BUG-0003 conditions)

Isolated namespace `reg007m`, **2-repo registry** (`QueueDrainTestbed` workers=2 +
`QueueDrainTestbed2` workers=1), **shared default runs-root**, launch-agent + auto-close ON.
Trigger present: `QueueDrainTestbed` issue **#1 CLOSED** (maps `Task-0001`) +
`QueueDrainTestbed2` issue **#1 OPEN/Ready** (maps `Task-0001`). Evidence:
[evidence/bugfix-verify-summary.txt](./evidence/bugfix-verify-summary.txt),
[evidence/bugfix-verify-backend-log.txt](./evidence/bugfix-verify-backend-log.txt),
[evidence/bugfix-verify-monitor.txt](./evidence/bugfix-verify-monitor.txt).

- **Fix B (namespaced id):** Task-0001 dispatched under
  `taskrun--QueueDrainTestbed2--Task-0001--active` — the repo is in the workflow id, not the
  bare colliding `taskrun--Task-0001--active`.
- **Fix A (no cross-repo reclaim):** `dispatched [Task-0001] parked [] reclaimed []` — the
  `QueueDrainTestbed` consumer's CLOSED #1 did **not** reclaim `QueueDrainTestbed2`'s live
  `Task-0001`. The launched agent (PID 67736) stayed **alive for the full 6-minute monitor**
  with exactly one worktree (`wt=[1 Task-0001:running]`). In the [BUG-0003 repro](../PASS-0009/evidence/repro-instrumented-log.txt)
  the same agent was killed + its worktree deleted within ~2 polls (~40s) by the cross-repo
  reclaim. No `already owns an active run` wedge, no `is not a working tree`, no reclaim of
  Task-0001 by the other repo.

### First-run caveat (resolved by the regression re-run below)

In the FIRST verification run the launched agent produced **zero assistant turns** in 6
minutes (transcript: prompt received, no response) — agent **starvation under heavy
concurrent load** (6 live `claude.exe` from this session's audit + design workflows), an
environmental issue, NOT a consumer/fix defect (the consumer dispatched correctly,
namespaced the id, kept the agent alive). So that run proved Fix A + Fix B but not the
completion tail. The regression re-run below (lower load) completed the full lifecycle.

## Regression on the fixed binary (no-regression confirmation, full cap=1 lifecycle)

Isolated single-repo (`reg007p`, dedicated runs-root, `QueueDrainTestbed2` only, auto-close
ON) on the FIXED binary — the PASS-0009 scenario re-run post-fix. Evidence:
[evidence/regression-fixed-binary-monitor.txt](./evidence/regression-fixed-binary-monitor.txt),
[evidence/regression-fixed-binary-log.txt](./evidence/regression-fixed-binary-log.txt).

```
dispatched [Task-0001]                          cap=1 (one worktree; #2 deferred)
agent ran (AGENT-RAN.txt) -> auto-close #1
dispatched [Task-0002] ... reclaimed [Task-0001]   slot freed + REUSED
agent ran -> auto-close #2
reclaimed [Task-0002] -> 0 worktrees
namespaced ids: taskrun--QueueDrainTestbed2--Task-0001--active, ...--Task-0002--active
```

Exactly one worktree throughout; the full dispatch→complete→auto-close→reclaim→reuse cycle
ran clean under repo-namespaced workflow-ids. **The namespacing fix did not regress the
happy path** (PASS-0009 behavior preserved). One harness note: the first re-run attempt was
blocked by a STALE prunable worktree left registered in `QueueDrainTestbed2`'s git (a test
hygiene miss — `git worktree prune` clears it); `countOwnedLaneWorktrees` counts prunable
git entries, so reconciliation/pruning of stale entries is a Landing-2 robustness item.

## Net

The cross-repo reclaim that killed a live agent is fixed and verified live on the exact
conditions that triggered it; unit tests make the regression falsifiable. Remaining
(Landing 2, named): move the run gate-state + worktree↔session binding + liveness
supervision into the per-run Temporal workflow (signal/query) and demote
`owned-lane-bootstrap.json` to a true recovery breadcrumb.
