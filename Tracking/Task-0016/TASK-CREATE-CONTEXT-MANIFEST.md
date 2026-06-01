# Task-0016 — Context Manifest (worker-safe)

Read these durable files to draft `TASK.md`. All paths absolute. Quote concrete
routes, structs, and line references in the draft so a cold reader can verify.

## The mockup (structural guide ONLY — see approved exclusions in HUMAN-DIRECTIVES)

- `C:\Users\gregs\Downloads\stitch_codex_token_velocity_overlay (4)\code.html`
  — the HTML/Tailwind mockup of the redesigned tab (titled "Worktree Management").
- `C:\Users\gregs\Downloads\stitch_codex_token_velocity_overlay (4)\screen.png`
  — rendered screenshot.
- `C:\Users\gregs\Downloads\stitch_codex_token_velocity_overlay (4)\DESIGN.md`
  — the mockup's design-system notes (palette, typography, "no lines" rule).

## Frontend (Python / Tkinter) — implementation home for the UI

- `C:\Agent\CodexDashboard\app\codex_dashboard\ui.py`
  - `_build_tasks_lane()` (~L1024) builds the CURRENT TASKS tab (summary cards +
    task stream canvas + detail panel). This is the content being **replaced**.
  - nav/tab definition (~L671): tabs are `("usage","Usage"),("jobs","Jobs"),("tasks","Tasks")`.
  - `ttk.Style` configuration block (~L493) defines the reusable styles
    (Card.TFrame, Shell.TFrame, ChartTitle.TLabel, MetricValue.TLabel, etc.) and
    the dark palette. Reuse these.
- `C:\Agent\CodexDashboard\app\codex_dashboard\tasks_tab.py`
  - task grouping / detail-section helpers and the task-state color map (~L23).
- `C:\Agent\CodexDashboard\app\codex_dashboard\tasks_backend.py`
  - backend base URL (default `http://127.0.0.1:4318`, env
    `CODEX_DASHBOARD_TASKS_BACKEND_URL`); `fetch_tasks_snapshot`, `dispatch_task`,
    `poke_task_run`, `pause_task_run`, `retry_task_run_workload`,
    `map_backend_tasks_snapshot` (~L78). New endpoints get client functions here.
- `C:\Agent\CodexDashboard\tests\test_tasks_backend.py` — pattern for
  frontend-mapping unit tests.

## Backend (Go) — implementation home for the new endpoints

- `C:\Agent\CodexDashboard\backend\orchestration\internal\httpapi\mux.go`
  - `NewMux()` (~L25): the full route registration block — see what exists.
  - `handleWorktreesList()` (~L130): `GET /api/v1/worktrees` (currently
    ACTIVE-only).
  - `handleTasksList()` (~L106): `GET /api/v1/tasks`.
  - `handleTaskAPIRoute()` (~L202): `POST /api/v1/tasks/{taskID}/dispatch` (~L214).
  - `handleTaskRunDetail()` (~L288): includes
    `POST /api/v1/task-runs/{runID}/resolve-interrupt-review` (~L429), the only
    existing path that reclaims a worktree (and only when a run is parked).
- `C:\Agent\CodexDashboard\backend\orchestration\internal\taskrun\service.go`
  - `ListActiveWorktrees()` (~L223) and `ListActiveWorktreesForRepo()`
    (repo-scoped) — current worktree enumeration (active lanes only).
  - `ListTasks()` (~L623), `Dispatch()` (~L658) — dispatch auto-provisions a
    fresh worktree today (no "assign to an existing idle worktree" path).
  - `cleanupOwnedLane()` (~L1640) — PID kill + `git worktree remove --force`;
    the mechanics Eject must reuse.
  - `ReconcileOwnedLanes()` — prune-only reconcile (Task-0015 Landing 2).
- `C:\Agent\CodexDashboard\backend\orchestration\internal\taskrun\types.go`
  - `RepoBinding` / `WorktreeBinding` (fields: Repo, TaskID, WorktreePath,
    AgentSessionID, SessionTranscriptPath, RunGateState, LaunchedPID), `TaskView`,
    `BindLaunchedSession` (~L300). The worktree-list response shape extends from here.
- `C:\Agent\CodexDashboard\backend\orchestration\internal\queue\manifest.go`
  - `RepoManifest` / `RepoEntry` (~L18): `id`, `local_root`, `queue_workers`,
    `task_provider{kind,repo,...}`. `LoadManifest()` (~L61) / `LoadRegistry()`
    (~L70). Registry path env: `OBSIDIAN_REGISTRY_PATH`; file `REPO-MANIFEST.json`.
    This feeds the repo filter dropdown and the per-repo idle-slot capacity.
- `C:\Agent\CodexDashboard\backend\orchestration\internal\queue\registry_consumer.go`
  - `RegistryRepos()` (~L33): extracts dispatch-ready repos from the registry.
- `C:\Agent\CodexDashboard\backend\orchestration\internal\httpapi\worktrees_test.go`
  and `mux_test.go`, plus
  `C:\Agent\CodexDashboard\backend\orchestration\internal\taskrun\service_test.go`
  — patterns for endpoint/service unit tests.

## Capabilities the redesign needs but the backend/frontend lack today (MISSING)

- `GET /api/v1/repos` — list registered repos for the filter dropdown. MISSING.
- `GET /api/v1/worktrees` that **includes idle/unallocated slots** (today it
  returns active lanes only). MISSING.
- An **Assign** path: bind a chosen open task onto a chosen idle worktree.
  MISSING (today dispatch always provisions a fresh worktree).
- An operator **Eject** endpoint: stop agent + clean checkout + return slot to
  idle, callable regardless of parked state (today only parked runs reclaim via
  resolve-interrupt-review). MISSING.

## Relevant durable task / shared context

- `C:\Agent\CodexDashboard\Tracking\Task-0015\Design\QUEUE-DRAIN-DURABLE-STATE-REDESIGN.md`
  — Landing 2: gate/binding live in the per-run Temporal workflow; worktree↔session
  binding authority; reclaim is the consumer's GitHub-issue-closed path.
- `C:\Users\gregs\.codex\Orchestration\WORKTREES.md` — shared worktree slot
  naming / allocation / binding rules.
- `C:\Agent\CodexDashboard\REGRESSION.md` — repo regression lanes (REG-007/008/009
  are the queue-drain/worktree/durable-state cases; the new tab should not break them).

## Drafting standard

Follow `C:\Users\gregs\.codex\Orchestration\Processes\TASK-CREATE.md` exactly:
concrete-implementation sections, human-facing-outcome-first wording, named files
+ endpoints + shapes, falsifiable acceptance criteria, `What Does Not Count`, and
a proof plan. Honor the approved exclusions and the worktree model in
`HUMAN-DIRECTIVES-FOR-WORKER.md` without narrowing scope.
