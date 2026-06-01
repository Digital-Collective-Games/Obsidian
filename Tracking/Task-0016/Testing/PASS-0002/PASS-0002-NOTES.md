# PASS-0002 — Create + Destroy + full-pool/repos reads + route guards

Backend pass 3 of the Task-0016 backend batch (single-context implementation).

## Objective (TASK.md §4, §7, §8; Goals 4, 7, 8; AC2, AC7, AC9, AC10, AC11)

The operator can pre-create idle pool members, destroy idle ones (reject allocated),
and read the full pool + the repo list — all method/path-guarded.

## Changes

- [pool.go](../../../../backend/orchestration/internal/taskrun/pool.go):
  - `CreatePoolWorktree(repo)` — provisions one new **idle** worktree at the next stable
    path (`git worktree add --detach <stablePath> <baselineCommit>` — the
    `provisionOwnedLane` git mechanics, but at a **stable, non-temp** path and with **no
    task bound**), writes the pool record `run_id == ""`, returns the idle `PoolWorktree`.
    Creation happens **only** here (never an `os.MkdirTemp` random dir). Best-effort
    cleanup of a partially-created member dir on git failure.
  - `DestroyPoolWorktree(worktreeID)` — **rejects** (`ErrPoolWorktreeAllocated`, removing
    nothing) an allocated member (classified live); else removes the checkout via the
    BUG-0002-hardened `removeOwnedLaneWorktree` and deletes the member folder + its
    durable pool record. Unknown id ⇒ `ErrPoolWorktreeNotFound`. Path-guarded
    (`pathWithinRoot` the owned-lane root).
  - `ListRepos(registryPath)` + `RepoView` — projects `id` + `local_root`
    (+ `task_provider_repo`) from the registry, **no `queue_workers`** in the response,
    sorted by id.
  - `ListFullPool()` — the GET `/api/v1/worktrees` read: pool members (idle + allocated)
    **merged** with active non-pool owned-lane worktrees, deduped by worktree path. The
    merge keeps the endpoint correct across the dispatch-path transition (before PASS-0003
    a dispatch still provisions a non-pool lane; after it, a dispatched lane IS a pool
    member) and **preserves the existing REG-008 parked-lane read** (the existing
    httpapi worktrees tests stay green unchanged).
  - `PoolWorktree` redesigned to **flatten** the binding (embed `RepoBinding`) to match
    the §8 response shape: `worktree_id` + `status` + `run_id` at top level, with
    `repo` / `worktree_path` / `task_id` / `agent_session_id` /
    `session_transcript_path` / `run_gate_state` / `launched_pid` from the embedded
    binding.
- [mux.go](../../../../backend/orchestration/internal/httpapi/mux.go):
  - `handleWorktreesList` now serves the **full pool** (`ListFullPool()`).
  - New `handleReposList` (`GET /api/v1/repos`, reads `cfg.RegistryPath`).
  - New method/path-guarded `handleWorktreeAPIRoute` on `/api/v1/worktrees/*` mirroring
    `handleTaskAPIRoute` (405 wrong method, 404 unknown sub-path), hosting **create** (201)
    + **destroy** (200; 409 on allocated via `ErrPoolWorktreeAllocated`; 404 on unknown).
    Routes registered in `NewMux`. `decodeJSONBody` helper for lenient optional bodies.

## Proof — Go unit tests

- [pool_test.go](../../../../backend/orchestration/internal/taskrun/pool_test.go):
  `TestCreatePoolWorktreeProvisionsIdleAtStablePath` (AC2 — idle at the exact stable
  `<ownedLaneRoot>/obsidian/wt-0001/w`, real `.git`, durable record, follow-up list shows
  idle), `TestCreatePoolWorktreeAllocatesNextStableID` (wt-0002), `TestDestroyPoolWorktreeRemovesIdle`
  (AC7 — folder+record gone, not listed), `TestDestroyPoolWorktreeRejectsAllocated`
  (AC7 falsifier — `ErrPoolWorktreeAllocated`, removes nothing),
  `TestListReposProjectsRegistryWithoutQueueWorkers` (AC10 — sorted, no `queue_workers`
  in the marshaled response).
- [worktrees_pool_test.go](../../../../backend/orchestration/internal/httpapi/worktrees_pool_test.go):
  `TestWorktreeCreateThenFullPoolRead` (AC2/AC9 — 201 + full-pool read shape with
  `status`/`worktree_id`), `TestWorktreeDestroyIdle` (AC7 — 200, gone from list),
  `TestReposEndpointReadsRegistryWithoutQueueWorkers` (AC10 — no `queue_workers`),
  `TestWorktreePoolRouteGuards` (AC11 — 405 wrong method on create/destroy/repos, 404 on
  unknown sub-path).
- The existing `TestWorktreesEndpointReturnsBindingAfterDispatch` /
  `TestWorktreesEndpointListsParkedWorktree` (REG-008 parked-lane read) stay green
  **unchanged** through the full-pool merge.

### Commands

```
cd C:\Agent\CodexDashboard\backend\orchestration
gofmt -l internal/   # (no output — clean)
go build ./...        # exit 0
go test ./...         # all ok
```

Producer testing; independent clean-context QA is coordinator-arranged after the
backend batch.
