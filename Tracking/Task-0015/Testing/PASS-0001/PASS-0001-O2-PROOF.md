# PASS-0001 (O2) — Real N>1 per-repo concurrency: proof

Pass scope: O2 from [PLAN.md](../../PLAN.md) "PASS-0001". Acceptance: A2.1 (HARD),
A2.2, A2.3 from [TASK.md](../../TASK.md). Implemented by the context-blind
PASS-0001 worker; proof captured on the isolated validation lane against the
throwaway test repo `C:\Agent\QueueDrainTestbed` (never the live lane / live
CodexDashboard repo). The agent committed only inside the throwaway test repo.

## What was built

- New `backend/orchestration/internal/queue` package:
  - `slots.go` — `EvaluateSlot(limit, used)` per-repo admit/refuse decision
    (admit while `used < limit`, refuse with a reason once full); `DefaultQueueWorkers = 4`.
  - `manifest.go` — reads `REPO-MANIFEST.json` and resolves `queue_workers` for
    the repo whose `local_root` matches the declared worktree root.
- `backend/orchestration/internal/taskrun/service.go`:
  - `Service` gained `repoSlotLimit` + `countActiveOwnedLanes` hooks (production
    wiring reads the manifest and counts live `git worktree list` owned lanes
    under the backend-owned lane root).
  - The dispatch gate in `deriveDispatchReadiness` no longer emits the legacy
    per-task `active_run_exists` block. `repoSlotBlock` admits a same-repo
    dispatch while fewer than `queue_workers` owned lanes are active and refuses
    with `repo_slots_exhausted` only when full. A task that already owns its own
    active run is still blocked from a duplicate dispatch via a distinct
    `task_already_running` reason (not the per-repo one).
  - `releasePreviousOwnedLane` is unchanged: it acts only on the SAME task's
    superseded run (`GetActiveTaskRun(taskID)`), so a same-repo sibling lane is
    never torn down.

## TIER 1 — Go unit tests (REQUIRED): PASS

`go test ./...` from `backend/orchestration` builds and passes — see
[evidence/AX.1-go-test-all.txt](./evidence/AX.1-go-test-all.txt).

New tests:
- `internal/queue/slots_test.go` — table-driven slot decisions + manifest
  resolution (separator/case/trailing-slash, fallback to default).
- `internal/taskrun/slots_test.go` —
  - relaxed gate admits siblings while slots remain, refuses the 5th when full,
    re-admits after a slot frees (A2.2/A2.3);
  - no `active_run_exists` for a same-repo sibling while a slot is free (A2.3);
  - same-task re-dispatch still blocked while owning the live story;
  - **two sibling tasks hold concurrent owned lanes in one repo** with a live
    worktree count of 2 (unit-level A2.1);
  - `releasePreviousOwnedLane` for one task does not remove a sibling's lane
    (F-O2 guard).

## TIER 2 — Live proof on the isolated validation lane: PASS (A2.1, A2.2, A2.3)

Infra: backend bound `127.0.0.1:14318`, validation Temporal `127.0.0.1:17233`,
namespace `default`, `CODEX_ORCHESTRATION_WORKTREE_ROOT=C:\Agent\QueueDrainTestbed`,
isolated runs root under the validation runtime root. `/healthz` returned `ok`.
Test repo carries `Tracking/Task-1`, `Task-2`, `Task-3` and a `REPO-MANIFEST.json`
entry (`queue_workers: 4`).

- **A2.3** — before any dispatch, both `Task-1` and `Task-2` read
  `dispatch_readiness.ready = true` with ZERO block reasons: no `active_run_exists`
  while a free slot remains.
- **A2.1 (HARD)** — `POST /api/v1/tasks/Task-1/dispatch` and
  `.../Task-2/dispatch` BOTH succeeded into state `running`, each with a distinct
  owned worktree. `git worktree list --porcelain` showed **two concurrent owned
  lanes** under `/cdxow/` for the one repo, plus both live run views in state
  `running`. See [evidence/A2.1-git-worktree-list.txt](./evidence/A2.1-git-worktree-list.txt),
  [evidence/A2.1-run-Task-1.json](./evidence/A2.1-run-Task-1.json),
  [evidence/A2.1-run-Task-2.json](./evidence/A2.1-run-Task-2.json).
- **A2.2** — with 2 lanes active and the cap lowered to 2 (repo full), the
  non-owning `Task-3` dispatch readiness was **refused** with `repo_slots_exhausted`;
  raising the cap back to 4 made `Task-3` `ready=true` again; dispatching it
  yielded a **third concurrent owned lane**. See
  [evidence/A2.2-refused-when-full.txt](./evidence/A2.2-refused-when-full.txt) and
  [evidence/A2.2-three-concurrent-worktrees.txt](./evidence/A2.2-three-concurrent-worktrees.txt).

## Teardown

The validation backend launched for this proof was stopped (`:14318` freed) and
the three owned-lane worktrees were removed (`git worktree remove --force` +
prune); the test repo is back to a single main worktree.
