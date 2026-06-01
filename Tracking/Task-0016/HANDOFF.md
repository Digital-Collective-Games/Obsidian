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

**Phase: implementation. Plan APPROVED (UPDATE 4). Backend passes PASS-0000…PASS-0006 DONE
+ clean-context audit (PASS-0006-AUDIT.json `ready`). FRONTEND passes PASS-0007 + PASS-0008
DONE (implemented, Python unit suite green @ 178, in-app REG-010…016 captured on the
isolated validation lane, committed/pushed on `master`). REG-003/REG-004 retired in
repo-root `REGRESSION.md` (UPDATE 4). `TASK-STATE.json` current_pass=PASS-0009.**

Remaining: **PASS-0009** — the consolidated in-app regression re-run (REG-010…016 + the
REG-007/008/009 re-run under the new model), which needs a coordinator-arranged
clean-context QA verdict on the in-app surface and the human-authenticated Chrome debug
session for the real-UI `Queue=Ready` flip. The agent never closes issue #16.

### Frontend batch (PASS-0007 + PASS-0008) — DONE this run

- **PASS-0007 — rename `TASKS`→`WORKTREES` + replace + read-only pool view — DONE.** New
  [worktrees_backend.py](../../app/codex_dashboard/worktrees_backend.py) HTTP client +
  [worktrees_tab.py](../../app/codex_dashboard/worktrees_tab.py) pure helpers; `ui.py` nav
  rename + D1=replace of the old task-stream/detail/dispatch surface with the
  worktree-management surface (allocated/idle background color, repo + local dir + id,
  per-row copy-path, registry-sourced repo filter, backend-unavailable message). Proof:
  [Testing/PASS-0007/PASS-0007-NOTES.md](./Testing/PASS-0007/PASS-0007-NOTES.md),
  [Testing/PASS-0007-CHECKLIST.json](./Testing/PASS-0007-CHECKLIST.json). In-app REG-010
  (pool view + color + rename + backend-unavailable), REG-011 (copy-path, clipboard ==
  backend path), REG-012 (registry repo filter narrows) captured under
  [Testing/PASS-0007/](./Testing/PASS-0007/).
- **PASS-0008 — mutating controls (Create / Assign popup → bind / Eject / Destroy /
  Dequeue) — DONE.** Controls wired to the [BE] endpoints (Assign popup lists open tasks
  from `GET /api/v1/tasks`, id+title+state, no progress bars; Destroy idle-only with the
  allocated 409 surfaced; standalone Dequeue leaves the worktree allocated). Proof:
  [Testing/PASS-0008/PASS-0008-NOTES.md](./Testing/PASS-0008/PASS-0008-NOTES.md),
  [Testing/PASS-0008-CHECKLIST.json](./Testing/PASS-0008-CHECKLIST.json). In-app REG-013
  (Create), REG-014 (Assign popup + bind→allocated), REG-015 (Eject→idle, folder kept),
  REG-016 (Destroy idle removes / allocated rejected + standalone Dequeue) captured under
  [Testing/PASS-0008/](./Testing/PASS-0008/). Caveat: on this single-Service control plane
  the dequeue provider is unwired, so the in-app dequeue is a safe no-op — the
  `Queue=Never` write + no-bounce-back are unit-proven in PASS-0004/0005 and re-exercised
  live in PASS-0009.
- **REG-003 / REG-004 retired** in [REGRESSION.md](../../REGRESSION.md): both marked
  SUPERSEDED by Task-0016 D1=replace (the `Tasks` surface is removed; the lifecycle lives
  on the GitHub Issues queue surface; the `WORKTREES` tab is covered by REG-010…016). Case
  history retained.

### Backend batch progress (PASS-0000…PASS-0006) — DONE earlier

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
- **PASS-0004 — Provider dequeue write + standalone dequeue endpoint — DONE.** Added
  `QueueProvider.DequeueIssue` (gh-CLI `Queue→Never` write, idempotent, never closes) +
  the `Service.DequeueProvider` seam + `Service.DequeueTask` + `POST
  /api/v1/worktrees/dequeue`; wired the gh provider as the Service's dequeue provider in
  queuedrain. Proof: [Testing/PASS-0004/PASS-0004-NOTES.md](./Testing/PASS-0004/PASS-0004-NOTES.md),
  [Testing/PASS-0004-CHECKLIST.json](./Testing/PASS-0004-CHECKLIST.json). Provider test
  fatals on any `issue close` (AC14); dequeue resolves Task-0007→#7 through the provider
  (AC12); endpoint 200 + guard (AC15). Full `go test ./...` green.
- **PASS-0005 — Eject (keep folder + return idle + dequeue) + no-bounce-back seam — DONE.**
  `Service.EjectWorktree` (terminate agent + `reset --hard`/`clean -fdx` via the new
  `restoreOwnedLaneFull` + keep folder + clear pool record run_id + dequeue the freed task;
  works running AND parked; never deletes, never closes) + `POST /api/v1/worktrees/eject`.
  Proof: [Testing/PASS-0005/PASS-0005-NOTES.md](./Testing/PASS-0005/PASS-0005-NOTES.md),
  [Testing/PASS-0005-CHECKLIST.json](./Testing/PASS-0005-CHECKLIST.json). Eject keeps the
  folder + returns idle + dequeues #1 (AC6/AC14, running & parked); the consumer+service
  no-bounce-back seam dequeues so the next poll does NOT re-dispatch, with a load-bearing
  variant that DOES bounce when the dequeue is skipped (AC13); endpoint Eject returns idle +
  dequeues #8. Full `go test ./...` green.
- **PASS-0006 — Backend cross-cut (full Go suite + server-only smoke) — DONE.** Full
  `go test ./...` green under `backend/orchestration` (gofmt clean); a server-only smoke
  ran Create → list → Assign → list → Eject → list → Dequeue → Destroy → list + repos read
  on the isolated validation lane (`http://127.0.0.1:14318`) against a **throwaway** testbed
  repo/registry (never the human's repo/data). Proof:
  [Testing/PASS-0006/SERVER-ONLY-SMOKE.txt](./Testing/PASS-0006/SERVER-ONLY-SMOKE.txt),
  [Testing/PASS-0006/PASS-0006-NOTES.md](./Testing/PASS-0006/PASS-0006-NOTES.md),
  [Testing/PASS-0006-CHECKLIST.json](./Testing/PASS-0006-CHECKLIST.json). Live-confirmed:
  Assign reuses the existing idle worktree (no fresh dir), Eject keeps the folder + returns
  idle, Destroy removes the idle folder, repos read carries no `queue_workers`.

## Frontend batch checkpoint (PASS-0007 + PASS-0008 complete)

The backend half (PASS-0000…PASS-0006) was implemented, unit/server-smoke proven, and
independently audited `ready` earlier. The frontend half (PASS-0007 + PASS-0008) is now
implemented, the Python unit suite is green (178), the in-app REG-010…016 cases are
captured on the isolated validation lane against the live backend, and both passes are
committed/pushed on `master`. The independent in-app QA verdict is NOT claimed by this
producer — it is coordinator-arranged.

**Remaining (PASS-0009, a later coordinator-arranged run):**

- **PASS-0009** — Cross-cutting closure: the consolidated in-app `WORKTREES`-tab regression
  run (REG-010…REG-016) + the REG-007/008/009 in-app re-run under the new model, on the
  isolated validation / reg007 lanes. Needs a clean-context QA verdict (coordinator-
  arranged) and the human-authenticated Chrome debug session for the real-UI REG-007
  `Queue=Ready` flip. This is also where the live provider `Queue=Never` dequeue write is
  exercised against the throwaway testbed (the in-app dequeue is a safe no-op on the
  single-Service control plane; see [PASS-0008 NOTES](./Testing/PASS-0008/PASS-0008-NOTES.md)).

The new [BE] endpoints the UI consumes (verified live in the PASS-0006 smoke and the
PASS-0007/0008 in-app captures): `GET /api/v1/repos`, `GET /api/v1/worktrees` (full pool,
§8 flat shape), `POST /api/v1/worktrees/{create,assign,eject,destroy,dequeue}`.

### Backend batch checkpoint (PASS-0000…PASS-0006 complete)

The backend half of Task-0016 is implemented, unit-tested green, server-smoke green, and
committed/pushed on `master`, and passed a clean-context audit. The independent QA verdict
is NOT claimed by this producer.

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
