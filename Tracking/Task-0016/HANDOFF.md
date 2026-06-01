# Task-0016 Handoff

## What this task is

**One testable chunk** delivering BOTH halves, closed as one unit (per
[HUMAN-DIRECTIVES-FOR-WORKER.md](./HUMAN-DIRECTIVES-FOR-WORKER.md) **UPDATE 3**, the
latest authoritative directive that **reverses** the earlier backend-only split):

- **[BE] Go backend** — manual persistent worktree pool (Create / Assign / Eject /
  Destroy / Dequeue + discover-on-startup; `queue_workers` removed; Eject dequeues via
  the task provider), and
- **[FE] Tkinter desktop UI** — the `TASKS` tab renamed to **`WORKTREES`** and its
  content **replaced** (D1=a) with a worktree-management surface that consumes the [BE]
  endpoints and actually works against live backend data.

**"Done" requires the working in-app human surface** (Human-Facing Outcome Rule):
backend endpoints + unit/server-smoke do NOT satisfy closure on their own — the
`WORKTREES` tab must work in-app and the new named in-app cases **REG-010…REG-016** must
pass on the isolated validation lane. Full scope + acceptance live in
[TASK.md](./TASK.md); the manual-pool MODEL (DESIGN PIVOT) and the Eject-dequeue
(UPDATE 2) still stand. **There is no Task-0017** — the UI is not deferred.

## Current resume point

**Phase: implementation (backend batch). Plan APPROVED (UPDATE 4). Backend passes
PASS-0000…PASS-0006 are in progress in this single-context run; UI passes
PASS-0007…PASS-0009 are deferred to a later coordinator-arranged run after a
clean-context QA verdict on the backend.**

Backend batch progress:

- **PASS-0000 — Pool record + stable identity — DONE.** New
  [pool.go](../../backend/orchestration/internal/taskrun/pool.go) + idle-persistence
  unit tests ([pool_test.go](../../backend/orchestration/internal/taskrun/pool_test.go)).
  Proof: [Testing/PASS-0000/PASS-0000-NOTES.md](./Testing/PASS-0000/PASS-0000-NOTES.md),
  [Testing/PASS-0000-CHECKLIST.json](./Testing/PASS-0000-CHECKLIST.json). `go build ./...`
  + `go test ./internal/taskrun/...` green.
- **PASS-0001 — Discover-on-startup — DONE.** `ListPoolWorktrees()` + `DiscoverPool()`
  in [pool.go](../../backend/orchestration/internal/taskrun/pool.go) (reconstructs
  idle-vs-allocated from disk + the live workflow; persists reclassified-idle; subsumes
  the prune hygiene); the queuedrain startup wiring
  ([queuedrain.go](../../backend/orchestration/internal/temporalbackend/queuedrain.go)
  L229) now calls `DiscoverPool()` instead of the prune-only `ReconcileOwnedLanes()`.
  Proof: [Testing/PASS-0001/PASS-0001-NOTES.md](./Testing/PASS-0001/PASS-0001-NOTES.md),
  [Testing/PASS-0001-CHECKLIST.json](./Testing/PASS-0001-CHECKLIST.json). Full
  `go test ./...` green.
- **PASS-0002 — Create + Destroy + full-pool/repos reads + route guards — DONE.**
  `CreatePoolWorktree` / `DestroyPoolWorktree` / `ListRepos` / `ListFullPool` +
  `PoolWorktree` flattened to the §8 shape
  ([pool.go](../../backend/orchestration/internal/taskrun/pool.go)); `GET /api/v1/repos`,
  `GET /api/v1/worktrees` (full pool), and the method/path-guarded
  `POST /api/v1/worktrees/{create,destroy}` sub-router
  ([mux.go](../../backend/orchestration/internal/httpapi/mux.go)). The existing REG-008
  parked-lane `/worktrees` read stays green via the pool+active merge. Proof:
  [Testing/PASS-0002/PASS-0002-NOTES.md](./Testing/PASS-0002/PASS-0002-NOTES.md),
  [Testing/PASS-0002-CHECKLIST.json](./Testing/PASS-0002-CHECKLIST.json). Full
  `go test ./...` green.
- **PASS-0003 — Assign + dispatch-path change + `queue_workers` removal — DONE.** The model
  swap: `dispatchWithDirective` + the manual `POST /api/v1/worktrees/assign` now **pool-draw**
  (`drawIdlePoolWorktree` → reset → `startRunInDrawnLane`, no fresh dir), the consumer admits
  via the new `PoolSizer.IdleWorktreeCount()` seam, and `queue_workers` / `EvaluateSlot` /
  `EffectiveFreeConcurrency` / `RepoSlotLimit` are removed (idle pool count is the cap).
  Release/resolve return a pool member to idle (kept) instead of delete; reclaim-on-close is
  intentionally unchanged (PASS-0005 owns Eject's keep-folder). Pool checkout leaf made
  unique (`wt-<NNNN>`) to avoid a `git worktree` admin-name collision. Proof:
  [Testing/PASS-0003/PASS-0003-NOTES.md](./Testing/PASS-0003/PASS-0003-NOTES.md),
  [Testing/PASS-0003-CHECKLIST.json](./Testing/PASS-0003-CHECKLIST.json). Full
  `go test ./...` green; AC1 grep proof clean (no live cap). REG-007 "pool of 1" unit-proven.
- **Next:** PASS-0004 Provider dequeue write + standalone dequeue endpoint.

### Original planning resume point (superseded by the approval above)

**Phase: planning. Gate: awaiting explicit human PLAN approval. No implementation has
started.**

- [TASK.md](./TASK.md) is **revised** to the full UPDATE 3 scope: [BE] criteria 1–15 kept,
  [FE] criteria 16–22 added (falsifiable, in-app), the Task-0017 deferral / backend-only
  framing removed, the Tk exclusion reversed (E2–E7 still excluded; E1 reversed), and
  "done = the working human surface" made explicit.
- [REGRESSION.md](../../REGRESSION.md) gained **seven new named in-app desktop-app-surface
  cases**, one per new `WORKTREES`-tab human surface:
  - **REG-010** pool view (allocated/idle color + repo + path + rename/replace)
  - **REG-011** copy-path control
  - **REG-012** registry-sourced repo filter
  - **REG-013** Create
  - **REG-014** Assign popup → bind
  - **REG-015** Eject (returns idle + dequeues)
  - **REG-016** Destroy (idle-only + allocated rejection) + standalone Dequeue
  - REG-007's cap=1 sub-scenario also got a **"pool of 1"** reinterpretation note under
    the new model (no new lane; same case under the manual pool).
- [PLAN.md](./PLAN.md) is **revised** to nine passes: backend PASS-0000…PASS-0006
  (endpoints the UI consumes), UI PASS-0007 (rename/replace + read-only pool view +
  copy-path + registry repo filter) and PASS-0008 (Create / Assign popup / Eject /
  Destroy / Dequeue controls), and PASS-0009 (in-app regression run of REG-010…REG-016
  plus the REG-007/008/009 re-run under the new model). The acceptance-criteria→pass map
  covers all 22 criteria + every new/updated regression case.
- GitHub issue **#16** is bound ([TASK-META.json](./TASK-META.json)).
- [TASK-STATE.json](./TASK-STATE.json): `phase=planning`, `current_gate=planning`,
  `plan_approved=false`.
- The plan review package is at
  [Testing/PLAN-APPROVAL/REVIEW-PACKAGE.md](./Testing/PLAN-APPROVAL/REVIEW-PACKAGE.md).

**Next step (blocked on the human):** the TaskDispatch coordinator relays the
PLAN-APPROVAL question (see the review package). On approval, set `plan_approved=true`
and begin PASS-0000. On revision, edit `PLAN.md` and re-present.

## Verified baseline

- Backend: `go build ./...` exit 0; `go test ./internal/queue/... ./internal/taskrun/...
  ./internal/httpapi/...` all `ok` (pre-implementation). Every load-bearing [BE] symbol
  cited in `TASK.md` was confirmed present (see [PLAN.md "Verified code baseline"](./PLAN.md)).
- Frontend (this re-plan): the [FE]-target structure exists — the nav-tab tuple at
  [ui.py L671](../../app/codex_dashboard/ui.py#L671), `select_tab`/`_render_active_tab`
  dispatch, the `_configure_styles` palette, the `tasks_backend.py` `urllib` HTTP-client
  pattern (incl. `fetch_tasks_snapshot` for the Assign popup), and the `tasks_tab.py`
  pure-helper pattern. The Stitch mockup folder exists (structural guide only).

## Process gaps (flagged to the TaskDispatch coordinator)

These are runtime/process limitations, **not** product task failures:

1. **No nested sub-agent / delegated-leader dispatch tool** is exposed to this worker.
   Per the worker doc's **Single-Context Fallback**, planning was performed
   single-context by the task leader and labeled as such. If the coordinator wants the
   specialized-leader lanes (IMPLEMENTATION-LEADER per pass, etc.), it must arrange them.
2. **No clean-context QA lane** can be created from this worker. The implementation passes
   — especially the UI passes (PASS-0007/0008) — require QA verdicts from a
   **QA-designated clean-context** subagent for the in-app surface; producer self-review
   must **not** stand in for QA. The coordinator owns arranging that lane.
3. **[FE] In-app regression needs a runnable desktop Tk surface.** REG-010…REG-016 must be
   exercised in the running app on the validation lane (app-surface capture), not by unit
   tests or endpoint smoke alone. If the environment cannot drive the real Tk surface,
   that is a blocker to surface, not grounds to downgrade the closure bar.
4. **PASS-0009 in-app REG-007 re-run needs a human-authenticated Chrome debug session**
   for the real-UI `Queue=Ready` flip (human authenticates once; agent drives the UI).
   Human prerequisite to surface when PASS-0009 is reached.
5. **D1=replace retires the old `TASKS`-tab behavior** — the implementer must reconcile
   with the coordinator how REG-003/REG-004 (the old `Tasks` surface cases) are
   superseded so REGRESSION.md stays internally consistent (the task lifecycle now lives
   on the GitHub Issues queue surface).

## Guardrails for whoever implements

- Two homes: the **Go backend** ([`backend/orchestration`](../../backend/orchestration/))
  and the **desktop app** ([`app/codex_dashboard/*`](../../app/codex_dashboard/)). The UI
  acts only through the [BE] HTTP endpoints (the backend is authoritative).
- The UI is **re-implemented in the existing Python/Tkinter app** (Q1=a), reusing its
  `ttk` styles + palette; the Stitch HTML is a structural guide, not a port/web migration.
  Mockup exclusions **E2–E7** are out of scope; **E1 reversed** (Create/Destroy in scope).
- All proof runs on the **isolated validation / `reg007` lanes**, task-owned config +
  isolated SQLite, and throwaway testbed repos per [TESTING.md](../../TESTING.md) — never
  the human's service lane, production repo, human dashboard config/database, or live
  Codex data. Do **not** publish a new pinned dashboard release to the human lane as part
  of this task's closure (separate human-gated publish + restart).
- Provider/queue tests use a **fake provider / injectable `run`** and must **not** hit
  real GitHub.
- Eject keeps the folder (Destroy is the only delete); dequeue sets `Queue=Never` and
  **never** closes the issue (human-only closure preserved; the agent never self-closes —
  including for Task-0016's own closure).
- Before any change to task/GitHub provider sync, `TASK-META.json`, issue fields,
  bootstrap, or reconcile, use `skills/obsidian-operator/SKILL.md` (repo AGENTS.md
  guardrail).
