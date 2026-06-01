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

**Phase: implementation. Plan APPROVED (UPDATE 4). Backend PASS-0000…PASS-0006 DONE +
audited `ready`. FRONTEND PASS-0007 + PASS-0008 DONE; fidelity fix + in-app re-captures
DONE. LIVE PASS-0009 regression run executed this run (real GitHub web surface). Python
suite green @ 182. `TASK-STATE.json` current_pass=PASS-0009. `validation.regression =
blocked` (REG-007 closure leg blocked by a newly-found product defect — see below).**

### LIVE PASS-0009 regression run — THIS run (real GitHub web surface, isolated lanes)

Full record: [Testing/PASS-0009/REGRESSION-RUN-0001.md](./Testing/PASS-0009/REGRESSION-RUN-0001.md).
Ran on an isolated control-plane (`:24320`/`:24321`, throwaway Temporal namespaces
`reg007live`/`reg014cap` on the validation Temporal `:17233`, throwaway
`QueueDrainTestbed`/`QueueDrainTestbed2`, isolated TEMP/runs roots). Production lanes
(`:4318`, `:14318`, the real `default` namespace, the `Obsidian` repo) NEVER touched;
confirmed at teardown. Real-UI `Queue` flips driven via the human-authenticated Chrome
9222 session (`Set-IssueFieldViaUi.ps1`), each `Committed=True` / `QueueReady` /
`ApiMatches=True`.

- **REG-008 — PASS.** Park (real-UI `Human Needed=Yes`) → `/worktrees` reports
  `parked_awaiting_closure` from the workflow while the on-disk breadcrumb stays `running`
  (demotion); backend kill+restart on the same namespace reconstructs the parked lane via
  discover-on-startup and RETAINS it. ([reg008-live/](./Testing/PASS-0009/reg008-live/))
- **REG-009 — PASS.** Two repos each with `#1` → distinct repo-namespaced run ids
  (`taskrun--RepoA--Task-0001` vs `taskrun--RepoB--Task-0001`); closing RepoB's `#1`
  reclaimed ONLY RepoB's lane; RepoA's lanes survived untouched.
  ([reg009-live/](./Testing/PASS-0009/reg009-live/))
- **Live REG-015 (verifies [BUG-0001](./BUG-0001.md)) — PASS.** Dashboard Assign→Eject of a
  `Queue=Ready` throwaway task wrote **`Queue=Never` to the CORRECT repo** (registry-resolved
  RepoA→`Digital-Collective-Games/QueueDrainTestbed`), the issue stayed **OPEN**, the folder
  was **kept**, and the freed task was **NOT re-dispatched** on the next poll (while a
  genuinely-Ready `#5` WAS dispatched — the dequeue is load-bearing). BUG-0001 →
  fixed-AND-live-verified. Honest scope: Eject driven via the production HTTP endpoint (the
  code path the Tk Eject button calls), not a literal widget click; the widget-click Eject
  was proven earlier on the local-no-op lane. ([reg015-live/](./Testing/PASS-0009/reg015-live/))
- **REG-007 — PARTIAL (core PROVEN; closure leg BLOCKED).** PROVEN: real-UI Ready flip →
  consumer dispatches exactly one into a pool worktree within `<=1 min` of an idle worktree
  existing → launches a discoverable top-level **claude** agent; the SECOND Ready issue
  WAITS (empty pool defers, no auto-create). BLOCKED: seeding the consumer's pool via the
  **product Create** action and the **close/eject→reuse** leg — see the new BUG.
  ([reg007-live/](./Testing/PASS-0009/reg007-live/))
- **Clean reg014 re-capture — DONE.** Fresh-empty seed → Create `RepoX/wt-0001` → real-Tk
  Assign of `Task-9001`; the displayed `Local dir …\wt-0001\wt-0001` now matches the
  assigned worktree id (the QA-REPORT path-string artifact is resolved).
  ([reg014-assign-bound-clean/overlay.png](./Testing/PASS-0009/reg014-assign-bound-clean/overlay.png))

### NEW BLOCKER — [BUG-0002](./BUG-0002.md) (manual-pool repo-segment mismatch)

The dashboard control-plane `taskService.CreatePoolWorktree` segments the pool by a hash
of its single `WorktreeRoot`, while the registry-driven consumer segments by the registry
repo id — so a worktree Created via the WORKTREES tab / `POST /api/v1/worktrees/create` is
**invisible to the queue-drain consumer's pool-draw**. This breaks the REG-007 "pool of 1"
documented flow ("seed the pool via Create first") and the close/eject→reuse leg.
Structurally identical to BUG-0001 (multi-repo dashboard Service not repo-resolving a
per-repo op). Proposed fix: registry-resolve the pool segment from the `repo` arg (mirror
the BUG-0001 dequeue-provider pattern). **REG-007 is not fully green and the task is not
closure-ready until BUG-0002 is fixed and REG-007 (Create-seed + reuse) is re-verified live.**

### PASS-0009 QA route-back re-capture (REG-014/015/016) — DONE this run

The [PASS-0009 QA](./Testing/PASS-0009/QA-REPORT.md) PASSED the UI fidelity re-check and the
REG-010…REG-013 functional QA but FAILED three ACTION proofs on **evidence quality only** (a
capture-harness defect, not a product defect): the REG-014/REG-015 post-action PNGs captured
the wrong foreground window, and the REG-016 rejection/confirmation messages were not rendered
in-frame. Both are now fixed and the three captures re-shot on the same isolated throwaway lane
(`:24319`, namespace `reg016fix`, throwaway `RepoX/RepoY` with NO `task_provider` → Eject/Dequeue
are safe local no-ops; never the human service `:4318`, validation `:14318`, the human dashboard
config/db, or live data; no production queue touched):

- **Wrong-window root cause** — the task-owned capture harness (gitignored
  `Tracking/Task-0016/Testing/Runtime/capture_filtered.py`, NOT product code) screenshotted the
  overlay's *screen rectangle* via `CopyFromScreen`, so when the borderless `overrideredirect`
  overlay was not the top-most painted window, the Claude Code IDE in the same region was grabbed.
  Fixed by capturing the overlay's OWN content **by HWND via `PrintWindow(PW_RENDERFULLCONTENT)`**
  (immune to occlusion/foreground), plus a blank-frame guard that pumps Tk/DWM repaints and
  retries until the grab is non-black, and by neutralizing the app's auto smoke-capture (which had
  re-introduced the `CopyFromScreen` path + `os._exit`) so the harness fully owns each shot.
- **REG-016 message-not-in-frame — minimal real UI fix.** The action status (`worktrees_status_message`)
  was only pushed to the global `status_label`, which is **never laid out on the WORKTREES tab**, so
  Eject/Destroy-reject/Dequeue outcomes were invisible. Added a visible
  `worktrees_action_status_label` to the WORKTREES header (same `Status.TLabel` style) and wired
  `_set_worktrees_status` to update it; the pool re-render does not clear it. Two-line surgical
  change in [ui.py](../../app/codex_dashboard/ui.py); Python unit suite green @ 182.
- **Re-captured, opened, and verified** (each PNG confirmed to show the WORKTREES tab with the
  claimed state/message, not another window):
  - [reg014-assign-bound/overlay.png](./Testing/PASS-0009/reg014-assign-bound/overlay.png) —
    `RepoX/wt-0002` ALLOCATED-RUNNING bound to `Task-9001` + the "Assigned …; it is now allocated."
    status in-frame, idle `RepoX/wt-0001` below.
  - [reg015-eject/overlay.png](./Testing/PASS-0009/reg015-eject/overlay.png) — `RepoX/wt-0001`
    returned IDLE (folder kept) + the "Ejected RepoX/wt-0001; it is idle and the task is dequeued."
    status in-frame.
  - [reg016-destroy-allocated-reject/overlay.png](./Testing/PASS-0009/reg016-destroy-allocated-reject/overlay.png)
    — the HTTP 409 "worktree is allocated; eject it before destroy" rejection rendered in-frame,
    `wt-0001` still allocated.
  - [reg016-dequeue/overlay.png](./Testing/PASS-0009/reg016-dequeue/overlay.png) — the "Dequeued
    Task-9001 (Queue=Never); the issue stays open." confirmation rendered in-frame, worktree still
    allocated.

REG-010/011/012/013 and REG-016 destroy-idle were unaffected (no rework). The LIVE GitHub
`Queue=Never` provider write + no-re-dispatch consequence remain a separate human-Chrome step.

### PASS-0007 INTERFACE-DESIGNER fidelity fix + final re-capture — DONE this run

The clean-context [INTERFACE-DESIGNER review](./Testing/PASS-0007/INTERFACE-DESIGNER-REVIEW.md)
raised two blocking fidelity findings; both are fixed (fix commit `4967cf9`, captures
`e18be01`, both pushed on `master`):

- **B1 (short repo heading)** — the panel HEADING now uses `worktree_heading_repo()` (the
  stable worktree-id repo segment, identical for idle and allocated) in BOTH states instead
  of the raw `repo` field (which for an allocated worktree is the bound checkout PATH and
  overflowed the heading). The full bound path stays in the DETAILS reveal. Applied to the
  row panel and the Details popup.
- **B2 (chip state mark)** — the status chip now carries a leading state MARK (a small
  0px-radius Canvas swatch in the cyan/green family — filled for allocated, hollow/outlined
  for idle, no emoji), not text only. New `_build_worktree_status_chip()` reused by the row
  and the Details popup; `worktree_chip_mark_filled()` helper.
- **Empty-pool message** — a reachable-backend zero-worktrees state now reads "No worktrees
  in this repo yet - use CREATE WORKTREE to add one." instead of the generic refresh line.
- **N1 (copy glyph)** intentionally skipped (a Tk text glyph risks reading as a placeholder,
  which UPDATE 5 forbids; the all-caps COPY PATH text is the approved Tk affordance).
- **REGRESSION.md REG-010** tightened (process gap §7a): the in-app step + expected result
  now require the heading to be the short repo id in BOTH idle and allocated states.
- **Tests** — `tests/test_worktrees_tab.py` adds heading-uses-segment + chip-mark tests;
  full suite green @ 182.
- **B3 re-capture** — the full REG-010…REG-016 set was re-taken on the FINAL post-fix Tk
  surface and confirms B1+B2 fixed (short `RepoX` headings + filled/hollow chip marks):
  [Testing/PASS-0009/PASS-0009-NOTES.md](./Testing/PASS-0009/PASS-0009-NOTES.md). Isolated
  throwaway lane on a fresh port `:24319`, namespace `reg016fix`, a throwaway registry
  (RepoX/RepoY) with **NO `task_provider`** so the post-fix Eject/Dequeue is a guaranteed
  safe local no-op; never the human service/validation lanes or live data; no production
  queue touched.

Remaining: **PASS-0009 closure** — a coordinator-arranged clean-context QA verdict on the
in-app surface (a fresh INTERFACE-DESIGNER re-check on this final surface) and the
human-authenticated Chrome debug session for the real-UI `Queue=Ready` flip + the LIVE
provider `Queue=Never` dequeue write against a throwaway GitHub testbed (NOT proven by this
run's no-provider captures). The agent never closes issue #16.

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
  [Testing/PASS-0008/](./Testing/PASS-0008/). Caveat (PASS-0008): on the single-Service
  control plane the dequeue provider was unwired, so the in-app dequeue was a safe no-op.
  **This is now FIXED — see [BUG-0001](./BUG-0001.md) (fixed-pending-live-verification):**
  a registry-backed multi-repo `DequeueProvider`
  ([`temporalbackend.NewControlPlaneDequeueProvider`](../../backend/orchestration/internal/temporalbackend/queuedrain.go))
  is now injected into the control-plane `taskService` in
  [`cmd/controlplane/main.go`](../../backend/orchestration/cmd/controlplane/main.go), and
  Eject routes `Queue→Never` to the ejected worktree's own repo. Unit/integration-proven
  (no real GitHub); `go build/test ./...` green; gofmt clean. The LIVE in-app dequeue
  (`Queue=Never` + no-bounce-back) is REG-015 in PASS-0009.
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
  exercised against the throwaway testbed. The control-plane dequeue provider is now WIRED
  (see [BUG-0001](./BUG-0001.md), fixed-pending-live-verification), so the in-app dequeue is
  no longer a no-op; PASS-0009 REG-015 is the LIVE confirmation (in-app Eject of a
  `Queue=Ready` testbed task → `Queue=Never` → no re-dispatch).

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
