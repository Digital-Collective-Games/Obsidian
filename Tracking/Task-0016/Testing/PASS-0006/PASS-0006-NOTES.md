# PASS-0006 — Backend cross-cut (full Go suite + server-only smoke)

Backend pass 7 (final backend pass) of the Task-0016 backend batch (single-context).

## Objective

Prove the backend model swap holds together as a whole before the UI consumes it:
full `go test ./...` green under `backend/orchestration`, plus a **server-only smoke**
(supporting proof, NOT regression) of the new endpoints on the isolated validation lane
against a **throwaway** registry/repo.

## Full Go suite

```
cd C:\Agent\CodexDashboard\backend\orchestration
gofmt -l internal/   # (no output — clean)
go build ./...        # exit 0
go test ./...         # all packages ok
```

All packages green: `controlplane`, `httpapi`, `jobexec`, `queue`, `taskexec`,
`taskrun`, `temporalbackend`.

## Server-only smoke (NOT regression)

Lane: the isolated **validation lane** at `http://127.0.0.1:14318` with its own Temporal
(`127.0.0.1:17233`) — started via `scripts/Start-ValidationLane.ps1`. The validation
lane's default `WorktreeRoot`/`RegistryPath` point at the real repo, so the lane's
supervised control plane was stopped and the **validation lane binary was re-run with
`CODEX_ORCHESTRATION_WORKTREE_ROOT` / `CODEX_ORCHESTRATION_TRACKING_ROOT` /
`CODEX_ORCHESTRATION_RUNS_ROOT` / `OBSIDIAN_REGISTRY_PATH` overridden to a THROWAWAY
testbed git repo** (`Tracking/Task-0016/Testing/Runtime/smoke-testbed`, since removed),
against the validation lane's Temporal. NEVER the human's production repo / service lane /
live Codex data / real `default` namespace. Compose (Temporal/Postgres) was already up and
left up; the supervised validation lane was restored and confirmed healthy afterward, and
the throwaway testbed + the temp pool checkouts (`%TEMP%\cdxow\repo-<hash>`) were deleted.

Full captured transcript: [SERVER-ONLY-SMOKE.txt](./SERVER-ONLY-SMOKE.txt). Sequence and
results:

| Step | Call | Result |
| --- | --- | --- |
| 1 | `GET /api/v1/repos` | 200 — `{id, local_root, task_provider_repo}`, **no `queue_workers`** (after correcting the throwaway registry's JSON; the malformed-JSON attempt correctly surfaced a clean 502 error, not a crash) |
| 2 | `GET /api/v1/worktrees` (empty) | 200 — `{"worktrees":[]}` |
| 3,4 | `POST /api/v1/worktrees/create {repo:"smoke"}` ×2 | 201 — two **idle** members at stable paths `…/wt-0001/wt-0001`, `…/wt-0002/wt-0002` |
| 5 | `GET /api/v1/worktrees` | 200 — two `status:"idle"` |
| 6 | `POST /api/v1/worktrees/assign {task_id:"Task-0007", worktree_id:"…/wt-0001"}` | 202 — run started; **REUSE-CHECK: the run's `owned_repo_root` == the pre-created wt-0001 path (no fresh dir)** ✓ |
| 7 | `GET /api/v1/worktrees` | 200 — wt-0001 **allocated** (bound Task-0007, `run_gate_state:"running"`), wt-0002 idle |
| 8 | `POST /api/v1/worktrees/eject {run_id}` | 200 — wt-0001 back to **idle**; **EJECT-KEEPS-FOLDER: the checkout still exists on disk** ✓ |
| 9 | `GET /api/v1/worktrees` | 200 — both idle |
| 10 | `POST /api/v1/worktrees/dequeue {repo, task_id}` | 200 — `{"status":"dequeued"}` (standalone, no eject) |
| 11 | `POST /api/v1/worktrees/destroy {worktree_id:"…/wt-0001"}` | 200 — `{"status":"destroyed"}`; **DESTROY-REMOVES-FOLDER: the checkout is gone** ✓ |
| 12 | `GET /api/v1/worktrees` | 200 — wt-0001 gone, wt-0002 remains |

Key load-bearing assertions confirmed live: Create at a **stable** path; Assign **reuses
the existing idle worktree** (no fresh dir); Eject **keeps the folder** and returns it
idle; Destroy **removes** the idle folder; the standalone Dequeue runs without ejecting.
This is the exact JSON shape the UI client (`worktrees_backend.py`) will consume in
PASS-0007/0008.

## Notes / caveats

- The smoke exercised the GitHub-issue dequeue (Eject + standalone) against the throwaway
  testbed's `task_provider.repo` only via the Service `DequeueTask` path; because the
  testbed's run id had no real GitHub issue and the lane's provider write targets the
  throwaway repo, the dequeue is the backend-owned provider write under test, never the
  human's production queue.
- On Windows the pool lives under the OS temp owned-lane root
  (`%TEMP%\cdxow\<repoSegment>`) per the inherited `defaultOwnedLaneRoot` behavior
  (unchanged by this task). The smoke's temp pool dirs were cleaned up.
- This is a **server-only smoke** (supporting proof), not a regression. The in-app
  `WORKTREES`-tab regression cases (REG-010…REG-016) and the REG-007/008/009 in-app re-run
  under the new model are the UI-pass closure bar (PASS-0007…PASS-0009), arranged by the
  coordinator with a clean-context QA verdict.

## Backend batch status

PASS-0000…PASS-0006 implemented, unit-tested green, server-smoke green, committed and
pushed. The independent clean-context QA verdict on this backend batch is
coordinator-arranged (not claimed here). Remaining: the UI passes PASS-0007…PASS-0009.
