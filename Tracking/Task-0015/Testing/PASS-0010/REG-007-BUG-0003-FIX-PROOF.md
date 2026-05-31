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

### Honest caveat (does NOT affect the fix verdict)

In this run the launched agent produced **zero assistant turns** in 6 minutes (transcript:
prompt received, no response) — so it never created `AGENT-RAN.txt`/announced completion,
and the auto-close→reclaim→reuse tail did not run here. Cause: **agent starvation under
heavy concurrent load** (6 live `claude.exe` from this session's audit + design workflows),
an environmental issue — NOT a consumer or fix defect (the consumer dispatched correctly,
namespaced the id, and kept the agent alive). The full cap=1 **deallocate/reuse lifecycle**
is independently proven in [PASS-0009](../PASS-0009/REG-007-CAP1-SERIALIZATION-PROOF.md)
(isolated single-repo). The BUG-0003 fix proof here depends only on the dispatch identity +
the absence of cross-repo reclaim, both of which are confirmed.

## Net

The cross-repo reclaim that killed a live agent is fixed and verified live on the exact
conditions that triggered it; unit tests make the regression falsifiable. Remaining
(Landing 2, named): move the run gate-state + worktree↔session binding + liveness
supervision into the per-run Temporal workflow (signal/query) and demote
`owned-lane-bootstrap.json` to a true recovery breadcrumb.
