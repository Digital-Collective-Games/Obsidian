# Task 0016

## Title

Manual persistent worktree pool + the desktop WORKTREES tab that drives it (Go backend lifecycle: Create / Assign / Eject / Destroy / Dequeue + discover-on-startup; Eject dequeues via the task provider — AND the Tkinter WORKTREES-tab UI that consumes it; done = the working in-app human surface)

## Scope (UPDATE 3 — the Tk UI is back in; "done" is the working human surface)

This is **ONE testable chunk** delivering **both** halves and closed as one unit
(per [HUMAN-DIRECTIVES-FOR-WORKER.md](./HUMAN-DIRECTIVES-FOR-WORKER.md) UPDATE 3,
the latest authoritative directive):

- **[BE] the Go backend** manual worktree-pool lifecycle (Create / Assign / Eject /
  Destroy / Dequeue + discover-on-startup; `queue_workers` removed; Eject dequeues
  via the task provider), and
- **[FE] the Tkinter desktop UI** — the `TASKS` tab is **renamed to `WORKTREES`**
  and its content is **replaced** (D1 = replace) with a worktree-management surface
  that **consumes the [BE] endpoints** and actually works against real backend data.

**"Done" requires the human surface to be WORKING** (Human-Facing Outcome Rule): the
operator opens the desktop app's `WORKTREES` tab and the worktree-management surface
actually performs each interaction against the live backend. Backend endpoints plus
unit/server-smoke proof do **not** satisfy "done" on their own — the working in-app
surface plus its **named in-app regression cases passing** do. The Stitch HTML mockup
(`C:\Users\gregs\Downloads\stitch_codex_token_velocity_overlay (4)`) is a **structural
guide only** (Q1 = a): re-implement in the existing Python/Tkinter app, reusing the
app's existing `ttk` styles and dark cyan/navy palette — **not** a web migration.

Mockup exclusions **E2–E7** still stand (drag-to-bind / Task Browser pane, Register
New Task, agent-model chip, animated transitional states, decorative progress bars,
top-nav chrome changes). **E1 is reversed** — manual Create/Destroy are in scope as
the visible Create / Destroy controls. The UI also surfaces the standalone **Dequeue**
and reflects the **Eject-dequeue** behavior (UPDATE 2).

## Summary

Today the orchestration backend has **no concept of an idle, reusable worktree**.
Every dispatch provisions a *fresh, random* temp checkout
([`provisionOwnedLane()`](../../backend/orchestration/internal/taskrun/service.go#L1507)
uses `os.MkdirTemp(s.ownedLaneRoot, ...)` then `git worktree add --detach`), and
the only way a checkout disappears is the human-approved close path, which
**deletes** it ([`cleanupOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1640)
→ [`removeOwnedLaneWorktree`](../../backend/orchestration/internal/taskrun/service.go#L1688)).
Per-repo concurrency is gated by a separate runtime cap, `queue_workers`
([`RepoEntry.QueueWorkers`](../../backend/orchestration/internal/queue/manifest.go#L32)),
read through [`RepoSlotLimit()`](../../backend/orchestration/internal/taskrun/service.go#L616)
and enforced by [`EvaluateSlot`](../../backend/orchestration/internal/queue/slots.go#L39)
inside the queue-drain consumer ([`consumer.go` L141/L205](../../backend/orchestration/internal/queue/consumer.go#L141)).

This task replaces that model with a **manually-managed, persistent worktree pool
per repo** in the **Go backend**, and **surfaces that pool in the desktop app's
`WORKTREES` tab** so a human operates it directly. An operator pre-creates worktrees
as real on-disk folders with **stable paths and stable ids**; those folders persist
as **idle** pool members until assigned, and an Eject cleans an allocated worktree
back to idle **without deleting the folder**. Concurrency is then bounded **by the
number of idle worktrees in the pool, by construction** — `queue_workers` is
removed entirely. Both the manual **Assign** action and the autonomous queue-drain
consumer **draw from the same shared pool**; an empty pool means Ready issues wait
(no auto-create).

**Human-perceived outcome (state this first — this is the "done" bar):** an operator
opens the desktop app, switches to the **`WORKTREES`** tab (renamed from `TASKS`), and
the tab actually works against the live backend:

- it **shows the whole pool** — every worktree across all registered repos — with a
  **visibly different background color** for allocated vs idle, plus the **repo**, the
  **local dir** (with a **copy-path control**), and (for allocated ones) the bound
  task / run / status;
- a **repo filter dropdown** (sourced from the **repo registry**, not hardcoded) filters
  the pool view to one repo;
- a **Create** control provisions a new idle worktree into the selected repo's pool and
  it appears in the view;
- an **Assign Task** popup queries the open tasks, and selecting one **binds** it onto a
  chosen idle worktree (which flips to allocated);
- an **Eject** control stops the agent, cleans the worktree back to idle (folder kept),
  and **dequeues** the freed task so it is not re-dispatched;
- a **Destroy** control removes an idle worktree (rejected, with a clear message, if it
  is allocated);
- a **Dequeue** control takes a task out of the queue without ejecting.

The backend is authoritative over worktree allocation/assignment/ejection; the UI
reads and acts **through the HTTP endpoints**. Under the hood, the same operator can
also drive every action via headless HTTP calls against the backend
(`http://127.0.0.1:4318`):

- **pre-create** one or more idle worktrees for a repo (`POST /api/v1/worktrees/create`),
- **see the whole pool** — every worktree the backend owns across all registered
  repos, each with a real `worktree_path`, a stable `worktree_id`, and a `status`
  of `idle` or `allocated` (allocated ones carry the bound task/run/gate)
  (`GET /api/v1/worktrees`),
- **manually assign** an open task onto a chosen idle worktree, which resets that
  *existing* checkout to baseline and launches the run **in it** (no fresh dir)
  (`POST /api/v1/worktrees/assign`),
- **eject** an allocated worktree — stop the agent, clean the checkout to baseline,
  unbind the run — and get the **same folder back as idle, re-assignable**
  (`POST /api/v1/worktrees/eject`),
- **destroy** an idle worktree to shrink the pool, rejected if it is allocated
  (`POST /api/v1/worktrees/destroy`),
- and after a **backend restart** still see the pool reconstructed: which folders
  are idle and which are allocated to a live run (discover-on-startup).

The autonomous queue-drain consumer keeps working: when an issue flips
`Queue=Ready` it now **draws an idle worktree from that repo's pool** instead of
provisioning a fresh one, and if the pool is empty the issue simply waits.

**Eject must also dequeue the freed task (UPDATE 2 — 2026-05-31).** Today the
queue-drain consumer is **read-only** against the task provider — it only ever
*reads* the `Queue` single-select for `Ready`
([`ghQueueProvider.ListReadyIssues`](../../backend/orchestration/internal/queue/provider.go#L119)).
If Eject frees a worktree but leaves the task's issue at `Queue=Ready`, the very
next consumer poll **re-dispatches it** ("throws itself back in"). So this task
adds the **first task-provider queue WRITE**: a `TaskProvider`-abstraction
**dequeue** that sets the task's provider queue state to **not-ready**
(`github_issues`: set the issue's `Queue` single-select to `Never`, the same field
the consumer polls). Eject calls it; a standalone dequeue endpoint exposes it
alone. **Dequeue is not close** — it only sets `Queue=Never`; the issue stays
**open**, preserving human-only closure (only a human-closed issue deallocates; the
agent never self-closes). To restart a dequeued task the operator sets
`Queue=Ready` again.

**Current truth that must not masquerade as success:** the backend can enumerate
only *active* worktrees today ([`ListActiveWorktrees()`](../../backend/orchestration/internal/taskrun/service.go#L223),
served at [`GET /api/v1/worktrees`](../../backend/orchestration/internal/httpapi/mux.go#L130)),
where "active" means *a dispatch happened and the random temp checkout still exists
on disk* — there is no notion of a pre-created idle folder, no stable id, and
[`ReconcileOwnedLanes()`](../../backend/orchestration/internal/taskrun/service.go#L1663)
is prune-only (it removes stale git metadata; it does **not** enumerate folders or
classify them idle-vs-allocated). A change that keeps the per-dispatch random-temp
model, or that only renames `queue_workers`, or whose "discover" loses bound state
across a restart, would look adjacent but is **not** this task.

## Why these mechanisms belong in one task (earned merge)

This task bundles the backend mechanisms and the UI that consumes them: (a) removing
`queue_workers`, (b) the durable pool record + stable identity, (c)
discover-on-startup, (d) the four lifecycle operations (Create / Assign / Eject /
Destroy) with their endpoints, (e) the dispatch-path change from per-dispatch-provision
to pool-draw, (f) the task-provider **dequeue write** (set `Queue=Never`) that Eject and
a standalone endpoint call, and (g) the **desktop `WORKTREES` tab** that renders the pool
and drives every operation. They belong in one task because they are **one model swap**
with **one human surface for it**: the pool record is the thing discover reconstructs,
the thing the four operations mutate, the thing the dispatch path draws from, and the
thing the `WORKTREES` tab displays and acts on — and removing `queue_workers` is only
safe *because* the pool's idle count is now the cap. The dequeue write belongs in the
same task because **Eject is only correct if it dequeues**: without it, an
Ejected-but-still-`Ready` task is re-dispatched on the next pool-draw poll, so the
pool-draw model and the dequeue write are two halves of one freed-slot contract. The UI
belongs in the same task because the human directive (UPDATE 3) makes the **working
in-app surface** the definition of done — backend endpoints with no working tab would
fail the "done" bar. Splitting them would leave a contradictory half-state (a pool with
no cap-replacement, operations with no durable record to act on, an Eject that frees a
slot only for the consumer to immediately re-take it, or a backend lifecycle no human
can actually drive).

They stay internally separable for implementation and proof:

- **`queue_workers` removal** — manifest/consumer change; proven by the consumer
  no longer reading a cap and by updated `decision.go`/`consumer.go`/`slots.go`
  tests.
- **Pool record + identity** — a durable record type; proven by a Go unit test on
  its read/write and id stability.
- **Discover-on-startup** — `ReconcileOwnedLanes()` replacement; proven by a
  restart-survival Go unit test (re-enumerate, reclassify) and by REG-008.
- **Create / Assign / Eject / Destroy** — four service methods + four HTTP routes;
  each independently unit-tested with the fake runtime as in
  [`service_test.go`](../../backend/orchestration/internal/taskrun/service_test.go).
- **Dispatch-path change** — `dispatchWithDirective` draws from the pool; proven by
  a Go test that asserts no fresh dir is created and an empty pool defers.
- **Provider dequeue write** — a new write method on the `QueueProvider`
  abstraction (symmetric to the read/poll and to `CloseIssue`); proven by a Go unit
  test against a **fake/mock provider** that asserts the write call set the queue
  state to not-ready (and never closed the issue) — no real GitHub access. Eject's
  use of it and the eject-then-no-redispatch behavior are proven on the
  consumer+service seam with the same fake provider.
- **Desktop `WORKTREES` tab** — the `TASKS` tab renamed and its content replaced
  ([ui.py L671](../../app/codex_dashboard/ui.py#L671) nav tuple; the
  `select_tab`/`_render_active_tab` dispatch); a small worktrees HTTP client mirroring
  [`tasks_backend.py`](../../app/codex_dashboard/tasks_backend.py); pure render/format
  helpers (mirroring [`tasks_tab.py`](../../app/codex_dashboard/tasks_tab.py)) unit-tested;
  and the human-facing interactions proven by **named in-app regression cases**
  (REG-010…REG-016 below), run on the isolated validation lane against the live backend.

Acceptance criteria are tagged **[BE]** (backend) or **[FE]** (the desktop
`WORKTREES` tab). Both halves ship in this one task; **"done" is gated on the
working [FE] surface plus its in-app regression cases**, not on [BE] endpoints alone.

## Goals

1. **[BE] Remove `queue_workers`.** Delete the `QueueWorkers` field from
   [`RepoEntry`](../../backend/orchestration/internal/queue/manifest.go#L29) and
   from `REPO-MANIFEST.json` documentation, and remove its use from admission:
   [`RepoSlotLimit()`](../../backend/orchestration/internal/taskrun/service.go#L616),
   the [`SlotSizer`](../../backend/orchestration/internal/queue/consumer.go#L46)
   path / [`EvaluateSlot`](../../backend/orchestration/internal/queue/slots.go#L39),
   [`EffectiveFreeConcurrency`](../../backend/orchestration/internal/queue/decision.go#L144),
   and the `queueWorkers` argument threaded into
   [`NewServiceForRepo`](../../backend/orchestration/internal/taskrun/service.go#L102)
   from [`queuedrain.go` L196](../../backend/orchestration/internal/temporalbackend/queuedrain.go#L196).
   Concurrency is bounded by the count of idle pool worktrees, by construction.
2. **[BE] Make worktrees a manually-managed persistent pool per repo.** Idle
   worktrees are real folders with **stable paths and stable ids**, not the
   `os.MkdirTemp` random dirs of
   [`provisionOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1507).
   Pin a durable **pool record** (`worktree_id` ↔ stable path ↔ repo ↔ current
   `run_id`-or-null), grounded in the existing
   [`owned-lane-bootstrap.json`](../../backend/orchestration/internal/taskrun/service.go#L68)
   record + [`bindingsFromRecords`](../../backend/orchestration/internal/taskrun/service.go#L251)
   enumeration.
3. **[BE] Discover-on-startup.** Replace/extend prune-only
   [`ReconcileOwnedLanes()`](../../backend/orchestration/internal/taskrun/service.go#L1663)
   so startup **enumerates the worktrees that exist on disk** for the repo and
   reconstructs each one's **idle** vs **allocated** state (allocated = bound to a
   live run, read from the per-run workflow per Task-0015 Landing-2). Survives a
   backend restart (REG-008). No auto-seeding to a number.
4. **[BE] Create** — `POST /api/v1/worktrees/create` provisions one new **idle**
   pool worktree (stable path + id; `git worktree add --detach`; persists; no
   task) and returns it.
5. **[BE] Assign** — `POST /api/v1/worktrees/assign` resets a chosen **existing
   idle** worktree to baseline, then binds + starts the run **in that worktree**
   (reuse the bootstrap→start tail of
   [`dispatchWithDirective`](../../backend/orchestration/internal/taskrun/service.go#L671)),
   without provisioning a fresh dir. The queue-drain consumer's auto-assign draws
   **any** idle worktree in the repo. Reject when no idle worktree is available.
6. **[BE] Eject** — `POST /api/v1/worktrees/eject` stops the launched agent,
   cleans the checkout to baseline (`git reset --hard` + `git clean -fdx`),
   unbinds/terminates the run, **keeps the folder, marking it idle**, and then
   **dequeues the freed task through the task provider** (Goal 10) so the
   still-`Queue=Ready` task is **not** re-dispatched on the next poll — this is what
   prevents the bounce-back. Must **not** delete the folder and must **not** close
   the issue. Works regardless of parked state.
7. **[BE] Destroy** — `POST /api/v1/worktrees/destroy` removes an **idle** worktree
   from the pool (reuse
   [`removeOwnedLaneWorktree`](../../backend/orchestration/internal/taskrun/service.go#L1688)
   delete mechanics). **Reject if allocated** (operator must Eject first).
8. **[BE] Repos + full-pool reads.** `GET /api/v1/repos` returns repo id +
   `local_root` (with `queue_workers` **removed** from the response).
   `GET /api/v1/worktrees` returns the **full pool** — allocated + idle — each with
   a real `worktree_path`, stable `worktree_id`, and `status`; allocated ones carry
   the bound task/run/gate. The existing REG-008 read of `/worktrees` (parked lane
   reported from the workflow) keeps working.
9. **[BE] Dispatch-path change.** Replace the per-dispatch
   [`provisionOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1507)
   call in
   [`dispatchWithDirective`](../../backend/orchestration/internal/taskrun/service.go#L671)
   with **pool-draw** (pick an idle pool worktree → reset → bind → start). Worktree
   **creation** happens only via the manual Create action. An empty pool ⇒ Ready
   issues wait (no auto-create). Each repo's pool is independent (REG-009).
10. **[BE] Task-provider dequeue write.** Add a **queue WRITE** to the
    [`QueueProvider`](../../backend/orchestration/internal/queue/provider.go#L22)
    abstraction — `DequeueIssue(repo, number)` (or `SetQueueState(repo, number, QueueNever)`)
    — that sets the task's provider queue state to **not-ready**. For
    `github_issues` ([`ghQueueProvider`](../../backend/orchestration/internal/queue/provider.go#L49))
    this sets the issue's **`Queue` single-select to `Never`**
    ([`QueueNever`](../../backend/orchestration/internal/queue/decision.go#L23)),
    the same field [`ListReadyIssues`](../../backend/orchestration/internal/queue/provider.go#L119)
    polls for `Ready`. It must be implemented **on the provider** (symmetric to the
    read/poll and to the existing write [`CloseIssue`](../../backend/orchestration/internal/queue/provider.go#L166)),
    **not** a one-off hardcoded `gh`/GitHub call inside Eject. It is **idempotent**
    (already `Never` ⇒ no-op) and a **safe no-op** when the worktree has no
    provider-backed task. It must **not** close the issue (the issue stays open).
    Concrete home: the new method on
    [`provider.go`](../../backend/orchestration/internal/queue/provider.go) (interface
    + `ghQueueProvider` gh-CLI implementation), wired into the per-repo Service the
    same way the read provider is built in
    [`queuedrain.go` L189](../../backend/orchestration/internal/temporalbackend/queuedrain.go#L189).
    Eject (Goal 6) and the standalone dequeue endpoint (Goal 11) both call it.
11. **[BE] Standalone dequeue endpoint.** `POST /api/v1/worktrees/dequeue` invokes
    the provider dequeue (Goal 10) **without** ejecting, so the operator can take a
    task out of the queue while leaving the run alone, and the `WORKTREES` tab's
    Dequeue control (Goal 18) has the seam. Method-guarded like the other handlers.
    Does **not** close the issue.

### Frontend goals — the desktop `WORKTREES` tab (the "done" surface)

All [FE] goals are in the existing Python/Tkinter app
([`app/codex_dashboard/`](../../app/codex_dashboard/)), reuse the existing `ttk`
styles + dark cyan/navy palette, and consume the [BE] HTTP endpoints above. The
Stitch mockup is a **structural guide only** (Q1 = a; D1 = replace). The tab **must
actually work against the live backend** — backend-shaped fixtures are fine for unit
tests, but the "done" bar is the live in-app surface (REG-010…REG-016).

12. **[FE] Rename `TASKS` → `WORKTREES` and replace its content.** Rename the nav tab
    label at [ui.py L671](../../app/codex_dashboard/ui.py#L671) (the
    `(tab_id, label)` tuple) from `Tasks` to `Worktrees` and rewire the
    `select_tab`/`_render_active_tab` dispatch so the tab renders the worktree-management
    surface (D1 = replace: the old task-stream / detail / dispatch-pause-poke content of
    the `TASKS` tab is **removed** from this tab — that lifecycle now lives on the GitHub
    Issues queue surface). Switching tabs stays read-only (no backend mutation on tab
    switch, like REG-003/REG-006). The `Usage` and `Jobs` tabs are unchanged.
13. **[FE] Full-pool view with allocated/idle distinction.** The tab shows **every
    worktree** the backend owns (from `GET /api/v1/worktrees`), each row showing the
    **repo**, the **local dir** (`worktree_path`), a stable identifier, and — for
    allocated ones — the bound `task_id` / `run_id` / `run_gate_state`. **Allocated
    rows have a visibly different background color** from idle rows (reuse the existing
    palette; the allocated/idle distinction must be perceivable, not just a text label).
    A backend-unavailable state shows a clear human-facing message (mirroring
    [`tasks_backend_error_snapshot`](../../app/codex_dashboard/tasks_backend.py#L58)).
14. **[FE] Copy-path control.** Each worktree row exposes a **copy control** that copies
    that worktree's local directory path to the clipboard (the mockup's copy icon). It
    copies the exact `worktree_path` string.
15. **[FE] Repo filter dropdown sourced from the registry.** A dropdown lists the
    registered repos from `GET /api/v1/repos` (the **repo registry**, not a hardcoded
    list) plus an "All repos" option; selecting a repo filters the pool view to that
    repo. The dropdown options reload when the repo list reloads.
16. **[FE] Create control.** A visible control provisions a new idle worktree into the
    currently-selected repo's pool via `POST /api/v1/worktrees/create`; after it
    succeeds the new idle worktree appears in the view (a refresh of the pool read). If
    no specific repo is selected, the control prompts for / requires choosing one.
17. **[FE] Assign-Task popup → bind.** An **Assign** control on an **idle** worktree
    opens a popup that **queries the open tasks** and lists each task's id + title +
    state (mockup exclusion **E6**: no progress bars / file-ref lines). Selecting a task
    and confirming calls `POST /api/v1/worktrees/assign` with that `task_id` + the
    chosen idle `worktree_id`; on success the worktree flips to **allocated** bound to
    that task. **The open-tasks source is pinned to the existing
    `GET /api/v1/tasks`** (local committed tasks bound to issues; consistent with
    "GitHub Issues is the task surface") via the existing
    [`tasks_backend.fetch_tasks_snapshot`](../../app/codex_dashboard/tasks_backend.py#L29)
    client — resolving the [Open Questions](#open-questions) Assign-source decision.
18. **[FE] Eject, Destroy, and standalone Dequeue controls.**
    - **Eject** (on an **allocated** worktree) calls `POST /api/v1/worktrees/eject`;
      on success the worktree returns to **idle** in the view (same row, idle color)
      and the freed task is dequeued (UPDATE 2 behavior — reflected because a
      subsequent pool read shows it idle and the task no longer `Ready`).
    - **Destroy** (on an **idle** worktree) calls `POST /api/v1/worktrees/destroy`; on
      success the worktree disappears from the view. Attempting Destroy on an
      **allocated** worktree surfaces the backend's rejection as a clear human-facing
      message (it is not silently dropped, and the worktree is **not** removed).
    - **Dequeue** — a standalone control takes a task out of the queue (via
      `POST /api/v1/worktrees/dequeue`) **without** ejecting (the run keeps going / the
      worktree stays allocated); it does **not** close the issue.

## Non-Goals

- **No auto-create / auto-seed.** The backend never grows the pool on its own; an
  empty pool defers Ready issues. Capacity is operator-owned (the human explicitly
  accepts this).
- **No change to the done-contract / park-in-place closure rules or human-only
  closure** ([`decision.go`](../../backend/orchestration/internal/queue/decision.go):
  only a closed issue deallocates; `Human Needed=Yes` parks in place; the agent
  never self-closes). Eject is an operator action that returns a folder to idle; it
  is **not** a new autonomous deallocation path and does not change when the
  consumer reclaims. The new dequeue write only sets `Queue=Never`; it **never
  closes** the issue, so human-only closure is preserved (Eject/dequeue are
  operator-initiated human actions, not an agent self-close).
- **No change to the queue-drain consumer's poll/decision semantics** beyond the
  cap removal and the pool-draw dispatch path. The consumer's
  [`DecideQueueAction`](../../backend/orchestration/internal/queue/decision.go#L102)
  logic is unchanged; the new dequeue is a Service/provider **write** invoked by
  Eject and the dequeue endpoint, not a new consumer decision branch. The consumer
  remains read-only in its own poll path (it still only *reads* `Queue`).
- **No change to the Usage or Jobs tabs**, or to any other endpoint. Only the
  `TASKS` tab is renamed/replaced (becomes `WORKTREES`); `Usage` and `Jobs` are
  untouched.
- The original Stitch-mockup UI exclusions **E2–E7** still stand and are **out of
  scope** for the `WORKTREES` tab: **E2** drag-to-bind / the persistent left Task
  Browser pane / the drag banner (replaced by the explicit Assign-Task popup);
  **E3** Register New Task; **E4** the agent-model chip (the backend binding has no
  model name — omit, or at most show the cheap launch-agent kind if trivially
  available); **E5** animated transitional states / pulsing drop-zones (a static
  running/parked/idle status chip is sufficient); **E6** per-task progress bars and
  file-ref metadata lines in task cards (the Assign popup lists task id + title +
  state instead); **E7** top-nav chrome changes (search box, a Review tab, settings /
  terminal / notifications icons). **E1 (manual worktree create/destroy) is reversed:
  it is now in scope** as the visible Create / Destroy controls.
- **No new product backend that the UI bypasses.** The `WORKTREES` tab acts only
  through the [BE] HTTP endpoints in this task (the backend is authoritative); it
  does not reach into git or the Temporal workflow directly.

## Implementation Home

Two homes: the **Go backend** (the worktree-pool authority + endpoints) and the
**Python/Tkinter desktop app** (the `WORKTREES` tab that consumes them).

### Backend (Go)

- **Routes:**
  [`backend/orchestration/internal/httpapi/mux.go`](../../backend/orchestration/internal/httpapi/mux.go)
  — register the new routes in `NewMux()` ([L25](../../backend/orchestration/internal/httpapi/mux.go#L25))
  and add a method-guarded `handleWorktreeAPIRoute` mirroring the existing
  [`handleTaskAPIRoute`](../../backend/orchestration/internal/httpapi/mux.go#L202)
  pattern; extend [`handleWorktreesList`](../../backend/orchestration/internal/httpapi/mux.go#L130)
  for the full-pool read and add `handleReposList`.
- **Service:**
  [`backend/orchestration/internal/taskrun/service.go`](../../backend/orchestration/internal/taskrun/service.go)
  owns worktree authority (provision, bootstrap, reset, cleanup, enumerate). The
  new pool methods, the discover replacement of
  [`ReconcileOwnedLanes()`](../../backend/orchestration/internal/taskrun/service.go#L1663),
  and the dispatch-path change all live here. `GET /api/v1/repos` reads the
  registry via [`queue.LoadRegistry`](../../backend/orchestration/internal/queue/manifest.go#L70)
  / [`RegistryRepos()`](../../backend/orchestration/internal/queue/registry_consumer.go#L33).
- **Manifest/consumer:**
  [`queue/manifest.go`](../../backend/orchestration/internal/queue/manifest.go) (drop
  `QueueWorkers`), [`queue/decision.go`](../../backend/orchestration/internal/queue/decision.go)
  (drop `EffectiveFreeConcurrency`'s cap math),
  [`queue/consumer.go`](../../backend/orchestration/internal/queue/consumer.go) /
  [`queue/slots.go`](../../backend/orchestration/internal/queue/slots.go) (the
  `SlotSizer`/`EvaluateSlot` cap path), and
  [`temporalbackend/queuedrain.go`](../../backend/orchestration/internal/temporalbackend/queuedrain.go)
  (drop the `queueWorkers` argument into `NewServiceForRepo`; switch the consumer
  to pool-draw).
- **Provider dequeue write:**
  [`queue/provider.go`](../../backend/orchestration/internal/queue/provider.go) is
  the right home for the dequeue write — it already owns the `QueueProvider`
  interface, the gh-CLI `ghQueueProvider`, the `Queue`-field read in
  [`ListReadyIssues`](../../backend/orchestration/internal/queue/provider.go#L119)
  (`fieldIDMap` resolves the org field-id↔name map; `fieldValues` reads each issue's
  field values), and the existing provider WRITE
  [`CloseIssue`](../../backend/orchestration/internal/queue/provider.go#L166). The
  new dequeue write is the symmetric sibling of that read and that write. The
  per-repo Service that runs Eject is given the write capability the same way the
  consumer is given the read provider — built via
  [`NewGitHubQueueProvider`](../../backend/orchestration/internal/temporalbackend/queuedrain.go#L189)
  in the queuedrain wiring and injected into the Service (the Service holds no
  provider today, so a small write-provider seam is added on the Service for Eject /
  the dequeue endpoint to call). Implement the write on the provider, never inline in
  Eject.

### Frontend (Python / Tkinter desktop app)

- **Tab nav + dispatch:**
  [`app/codex_dashboard/ui.py`](../../app/codex_dashboard/ui.py) — rename the
  `("tasks", "Tasks")` entry in the nav tuple at
  [L671](../../app/codex_dashboard/ui.py#L671) to `("worktrees", "Worktrees")` (the
  human-visible label becomes `WORKTREES`; keep one `tab_id` consistently), and rewire
  `select_tab` / `_render_active_tab` (around [L1152](../../app/codex_dashboard/ui.py#L1152)/[L1164](../../app/codex_dashboard/ui.py#L1164))
  so the renamed tab builds and renders the worktree-management surface instead of the
  committed-task stream/detail. The old task-stream / detail / dispatch-pause-poke
  widgets and their render methods are removed from this tab (D1 = replace). Reuse the
  existing styles defined in `_configure_styles` and the palette constants
  (`#0a0e14` / `#1c2026` surfaces, `#00e5ff` / `#c3f5ff` cyan accents,
  `TAB_ACTIVE_FOREGROUND` etc.). The allocated/idle background distinction (Goal 13)
  reuses that palette (an allocated-row background style alongside the idle one).
- **Worktrees HTTP client:** a new module
  [`app/codex_dashboard/worktrees_backend.py`](../../app/codex_dashboard/worktrees_backend.py)
  (new file) mirroring [`tasks_backend.py`](../../app/codex_dashboard/tasks_backend.py):
  `configured_*_backend_url()`, `fetch_pool_snapshot()` (GET `/api/v1/worktrees`),
  `fetch_repos()` (GET `/api/v1/repos`), and `create_worktree` / `assign_worktree` /
  `eject_worktree` / `destroy_worktree` / `dequeue_task` POST helpers, with the same
  `urllib`-based `_request_json` and the same backend-unavailable error snapshot shape.
  The Assign popup's open-task list reuses the existing
  [`tasks_backend.fetch_tasks_snapshot`](../../app/codex_dashboard/tasks_backend.py#L29)
  (`GET /api/v1/tasks`) — no new task-source backend.
- **Pure render/format helpers:** a new module
  [`app/codex_dashboard/worktrees_tab.py`](../../app/codex_dashboard/worktrees_tab.py)
  (new file) mirroring [`tasks_tab.py`](../../app/codex_dashboard/tasks_tab.py) for the
  unit-testable pure logic: group/sort worktrees, allocated-vs-idle color selection,
  per-row detail formatting, and the Assign-popup open-task projection. These are the
  unit-test target ([FE] unit proof), distinct from the in-app regression cases.

The backend home is right because the backend is authoritative over worktree
assignment; the dequeue write lives on the provider because that is where the symmetric
`Queue`-field read and the existing `CloseIssue` write already live, and keeping it
there (not inline in Eject) keeps the provider the single task-provider surface. The
frontend home is right because the human directive (UPDATE 3) reinstates the original
desktop-app `TASKS`-tab redesign (Q1 = a: reuse the existing Python/Tkinter app; D1 = a:
replace) — and "done" is the working in-app surface, so the UI must live in the shipped
desktop app, not a separate prototype.

## Proposed Changes

All routes are JSON over `http://127.0.0.1:4318`, method-guarded (405 on the wrong
method, 404 on unknown sub-paths) to match the existing handlers.

### 1. Remove `queue_workers` (the cap → pool-count migration)

- Delete `QueueWorkers int \`json:"queue_workers"\`` from
  [`RepoEntry`](../../backend/orchestration/internal/queue/manifest.go#L29); remove
  `QueueWorkersForRoot` and the `DefaultQueueWorkers` cap constant where it only
  served admission.
- Remove [`RepoSlotLimit()`](../../backend/orchestration/internal/taskrun/service.go#L616),
  the `repoSlotLimit` field + `manifestQueueWorkers`, the `SlotSizer` interface
  consumption, [`EvaluateSlot`](../../backend/orchestration/internal/queue/slots.go#L39),
  and [`EffectiveFreeConcurrency`](../../backend/orchestration/internal/queue/decision.go#L144)'s
  cap arithmetic from the consumer's admission. The consumer no longer computes a
  numeric cap; admission is "is there an idle pool worktree for this repo?".
- Drop the `queueWorkers` parameter from
  [`NewServiceForRepo`](../../backend/orchestration/internal/taskrun/service.go#L102)
  and its call site
  ([`queuedrain.go` L196](../../backend/orchestration/internal/temporalbackend/queuedrain.go#L196)).
- **Migration:** the field is removed (unknown JSON keys are already ignored, so an
  old `REPO-MANIFEST.json` carrying `queue_workers` still loads — the value is just
  no longer consulted). Tests that set `queue_workers` / a fixed `SlotSizer` (e.g.
  `fixedSizer` in [`consumer_test.go`](../../backend/orchestration/internal/queue/consumer_test.go#L95),
  `slots_test.go`) are updated to drive admission from a pool of idle worktrees
  instead of a numeric cap.

### 2. Stable pool record + identity

- Each pool worktree gets a **stable path** under the backend-owned lane root —
  e.g. `<ownedLaneRoot>/<repoID>/wt-<NNNN>/w` — replacing
  `os.MkdirTemp(...)`-derived random dirs, and a **stable `worktree_id`** derived
  from that path (e.g. `<repoID>/wt-<NNNN>`).
- Extend the durable
  [`ownedLaneBootstrapRecord`](../../backend/orchestration/internal/taskrun/service.go#L68)
  (or add a sibling `worktree-pool.json` per pool folder) to persist the pool
  record: `worktree_id`, stable `worktree_path`, `repo`, and the **current
  `run_id` or null** (null = idle). Idle worktrees must persist with `run_id=null`
  so they survive when no run is bound.
- The enumeration that
  [`bindingsFromRecords`](../../backend/orchestration/internal/taskrun/service.go#L251)
  /
  [`collectActiveLaneRecords`](../../backend/orchestration/internal/taskrun/service.go#L271)
  perform is extended to also surface idle pool members (today it drops a record
  whose checkout dir is gone; now an idle folder that *exists* must be surfaced as
  idle).

### 3. Discover-on-startup (replaces prune-only reconcile)

Replace the body of
[`ReconcileOwnedLanes()`](../../backend/orchestration/internal/taskrun/service.go#L1663)
(or add `DiscoverPool()` invoked at the same wiring point in
[`queuedrain.go` L226](../../backend/orchestration/internal/temporalbackend/queuedrain.go#L226))
so that, per repo, startup:

- enumerates the pool folders that exist on disk (the stable-path layout above),
- for each, reads its current binding **live from the per-run TaskRunWorkflow**
  (Task-0015 Landing-2 authority, as
  [`liveBindingForRecord`](../../backend/orchestration/internal/taskrun/service.go#L358)
  does) to classify **allocated** (bound to a live run) vs **idle**,
- keeps the existing `git worktree prune` hygiene for genuinely stale metadata.

This must reconstruct allocated-vs-idle across a backend restart without losing
bound state (REG-008). No folder is created or destroyed by discovery.

### 4. Create — `POST /api/v1/worktrees/create`

Request: `{ "repo": "obsidian" }`. New `Service.CreatePoolWorktree(repo)` that
provisions one new **idle** worktree at the next stable path (`git worktree add
--detach <stablePath> <baselineCommit>`, the
[`provisionOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1507)
git mechanics but at a stable, non-temp path and with **no task bound**), writes
its pool record with `run_id=null`, and returns the new idle worktree
(`worktree_id`, `worktree_path`, `repo`, `status:"idle"`). HTTP 201/200.

### 5. Assign — `POST /api/v1/worktrees/assign`

Request: `{ "task_id": "Task-0007", "repo": "obsidian", "worktree_id": "obsidian/wt-0001" }`.
New `Service.AssignTaskToPoolWorktree(ctx, taskID, repo, worktreeID)`:

- resolve the target idle worktree: the named `worktree_id` if given, otherwise
  (the consumer auto-assign path) **any** idle worktree in the repo; reject (409,
  no run started) if none is idle/available;
- **reset that existing checkout to baseline** by reusing
  [`restoreOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1726)
  (`git reset --hard` + `git clean -fd`),
- then **bind + start the run in that worktree** by reusing the bootstrap→start
  tail of
  [`dispatchWithDirective`](../../backend/orchestration/internal/taskrun/service.go#L671)
  ([`bootstrapOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1561)
  → `runtime.StartTaskRun`) — **without** calling
  [`provisionOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1507)
  (no fresh dir),
- update the pool record's `run_id` to the started run.

Response: the started
[`TaskRunView`](../../backend/orchestration/internal/taskrun/types.go#L242)
(HTTP 202), like dispatch. The acceptance bar is **reusing an existing idle folder**,
not provisioning a new one.

The **queue-drain consumer** ([`consumer.go` `ActionDispatch`](../../backend/orchestration/internal/queue/consumer.go#L199))
calls this same pool-draw path (`worktree_id` omitted → any idle worktree in the
repo) in place of today's `Dispatch`/`provisionOwnedLane`. When no idle worktree is
available the consumer skips the issue (it is re-picked on a later poll once an
Eject/close frees one), exactly as it skips a full repo today.

### 6. Eject — `POST /api/v1/worktrees/eject`

Request: `{ "run_id": "taskrun--obsidian--Task-0007--active" }` (default key;
`worktree_id` accepted as an alternate — see Open Questions). New
`Service.EjectWorktree(ctx, runID)`:

- terminate the launched agent (the
  [`terminateAgentProcess`](../../backend/orchestration/internal/taskrun/service.go#L528)
  PID kill that
  [`ReclaimOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L506)
  already performs) and unbind/terminate the run,
- **clean the checkout to baseline** (`git reset --hard` + `git clean -fdx` —
  reuse [`restoreOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1726),
  with `-fdx` to also drop ignored files for a true baseline),
- **keep the folder** and set its pool record `run_id=null` (idle). It must **not**
  call [`removeOwnedLaneWorktree`](../../backend/orchestration/internal/taskrun/service.go#L1688) /
  delete the folder.
- **then dequeue the freed task through the task provider** (section 10): call the
  provider dequeue for the freed task's issue (`Queue → Never`) so the still-`Ready`
  task is **not** re-dispatched on the next consumer poll. **This is what prevents
  the bounce-back.** Idempotent (already `Never` ⇒ no-op); a safe no-op when the
  ejected worktree had no provider-backed task (e.g. a manual-Assign of a
  non-issue-backed task). It must **not** close the issue — the issue stays open so
  human-only closure is preserved.
- **Works regardless of parked state** — unlike `resolve-interrupt-review`
  ([`mux.go` L429](../../backend/orchestration/internal/httpapi/mux.go#L429)), which
  only reclaims a parked run. Eject is the operator's explicit "give me this slot
  back."

Response: the now-idle worktree (HTTP 200). After Eject, the next
`GET /api/v1/worktrees` shows that `worktree_id` as `idle` and the task as no
longer bound; the freed issue reads `Queue=Never` (still open), so the next
consumer poll does **not** re-dispatch it.

### 7. Destroy — `POST /api/v1/worktrees/destroy`

Request: `{ "worktree_id": "obsidian/wt-0001" }`. New
`Service.DestroyPoolWorktree(worktreeID)`:

- **reject (409, nothing removed) if the worktree is allocated** (its pool record
  `run_id` is non-null / a live run is bound) — the operator must Eject first;
- otherwise remove the idle folder from the pool by reusing
  [`cleanupOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1640)
  → [`removeOwnedLaneWorktree`](../../backend/orchestration/internal/taskrun/service.go#L1688)
  (the existing PID-safe / self-healing `git worktree remove --force` mechanics
  that [BUG-0002](../Task-0015/BUG-0002.md) hardened), and delete its pool record.

Response: HTTP 200/204.

### 8. Repos + full-pool reads

- **`GET /api/v1/repos`** — new `handleReposList` + `Service.ListRepos()` reading
  [`queue.LoadRegistry`](../../backend/orchestration/internal/queue/manifest.go#L70)
  at the configured `OBSIDIAN_REGISTRY_PATH` and projecting
  [`RegistryRepos()`](../../backend/orchestration/internal/queue/registry_consumer.go#L33).
  Response (note: **no `queue_workers`**):
  ```json
  { "repos": [ { "id": "obsidian", "local_root": "C:\\Agent\\CodexDashboard", "task_provider_repo": "gregsemple2003/obsidian" } ] }
  ```
- **`GET /api/v1/worktrees`** — extend
  [`handleWorktreesList`](../../backend/orchestration/internal/httpapi/mux.go#L130)
  + add `Service.ListPoolWorktrees()` returning the **full pool** (allocated +
  idle), per registered repo. Each entry adds a `status` discriminator and a stable
  `worktree_id`; allocated entries carry the bound task/run/gate from the live
  binding. Shape:
  ```json
  {
    "worktrees": [
      { "status": "allocated", "worktree_id": "obsidian/wt-0001", "repo": "obsidian",
        "worktree_path": "C:\\...\\owned-lanes\\obsidian\\wt-0001\\w",
        "task_id": "Task-0007", "run_id": "taskrun--obsidian--Task-0007--active",
        "run_gate_state": "running", "agent_session_id": "...",
        "session_transcript_path": "...", "launched_pid": 12345 },
      { "status": "idle", "worktree_id": "obsidian/wt-0002", "repo": "obsidian",
        "worktree_path": "C:\\...\\owned-lanes\\obsidian\\wt-0002\\w" }
    ]
  }
  ```
  `run_gate_state` reuses the existing enum
  (`running` / `parked_*`, [types.go L147–160](../../backend/orchestration/internal/taskrun/types.go#L147)).
  REG-008 reads `/worktrees` for the parked lane reported from the workflow; that
  allocated entry (gate read live from the per-run workflow) must remain present
  and correct.

### 9. Dispatch-path change

In [`dispatchWithDirective`](../../backend/orchestration/internal/taskrun/service.go#L671),
replace the `provisionOwnedLane(task.TaskID)` call (L687) with a **pool-draw**: pick
an idle pool worktree for the repo, reset it
([`restoreOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1726)),
and proceed into the existing bootstrap→start tail. If no idle worktree is
available, dispatch is refused (the consumer skips and re-picks later). Worktree
creation is no longer a side effect of dispatch — it only happens via Create.

### 10. Task-provider dequeue write (`Queue → Never`)

Today the provider is **read-only** for the queue field: the consumer only reads
`Queue` for `Ready`
([`ListReadyIssues`](../../backend/orchestration/internal/queue/provider.go#L119)
resolves the org field-id↔name map with `fieldIDMap`, reads each issue's
`issue-field-values` with `fieldValues`, and maps the `Queue` single-select option
name onto [`IssueState.Queue`](../../backend/orchestration/internal/queue/decision.go#L48)).
The only existing provider WRITE is
[`CloseIssue`](../../backend/orchestration/internal/queue/provider.go#L166) (a thin
`gh issue close`), and it is the model for the new write.

- Add a dequeue write to the
  [`QueueProvider`](../../backend/orchestration/internal/queue/provider.go#L22)
  interface, e.g. `DequeueIssue(repo string, number int) error` (equivalently
  `SetQueueState(repo, number, QueueNever)`), with a one-line doc that it is a
  provider WRITE that sets the issue's queue state to not-ready and **never** closes
  the issue.
- Implement it on
  [`ghQueueProvider`](../../backend/orchestration/internal/queue/provider.go#L49)
  as the symmetric sibling of the `Queue` read: resolve the `Queue` field id via the
  existing `fieldIDMap()` (org `issue-fields`), resolve the `Never` option id for
  that single-select field, and `gh api` the issue-field-value for the `Queue` field
  to the `Never` option (the write counterpart of the `/repos/<repo>/issues/<n>/issue-field-values`
  read), through the injectable `run` func so a test never touches real GitHub.
  Reuse the existing [`QueueNever`](../../backend/orchestration/internal/queue/decision.go#L23)
  constant for the option name. Idempotent: setting `Never` when it is already
  `Never` is a no-op.
- Wire the write capability into the per-repo Service that runs Eject. The provider
  is built in the queuedrain wiring with
  [`NewGitHubQueueProvider`](../../backend/orchestration/internal/temporalbackend/queuedrain.go#L189);
  the Service holds no provider today, so add a small write-provider seam on the
  Service (a `DequeueProvider`-style field/interface, fake-able in tests like the
  existing fake runtime) that Eject and the dequeue endpoint call. Eject resolves
  the freed task's issue number with the existing
  [`IssueNumberFromTaskID`](../../backend/orchestration/internal/queue/consumer.go#L93)
  (Task-`Task-N` ⇒ issue `#N`) and calls the dequeue. A worktree with no
  provider-backed task (no parseable issue number) ⇒ safe no-op.
- The write is **never** inlined as a hardcoded `gh`/GitHub call inside Eject; it
  lives on the provider so the provider stays the single task-provider surface.

### 11. Standalone dequeue endpoint — `POST /api/v1/worktrees/dequeue`

Request: `{ "repo": "obsidian", "task_id": "Task-0007" }` (or `{ "worktree_id": ... }`
resolving to its bound task). New `Service.DequeueTask(repo, taskID)` that calls the
provider dequeue (section 10) **without** stopping the agent, cleaning the checkout,
or unbinding the run — so an operator can take a task out of the queue while leaving
the run alone, and the `WORKTREES` tab's Dequeue control has the seam. It does **not**
close the issue. Registered as a method-guarded sub-path on `handleWorktreeAPIRoute`
alongside create/assign/eject/destroy (405 on the wrong method, 404 on unknown
sub-paths). Response: HTTP 200.

The route is on the `/api/v1/worktrees/*` surface (not `/api/v1/tasks/{id}/dequeue`)
because dequeue is part of the same worktree-management lane as create/assign/eject/
destroy — the operator surface the `WORKTREES` tab drives — and it shares the worktree
handler's method/path guards; keeping it under `/worktrees/*` keeps all five pool
operations on one consistent, method-guarded sub-router rather than splitting one
operation onto the separate task router.

### 12. Desktop `WORKTREES` tab (replaces the `TASKS` tab content)

The renamed tab consumes the endpoints above. All against the configured backend URL
(default `http://127.0.0.1:4318`; the regression lane overrides it to
`http://127.0.0.1:14318`).

- **Rename + replace.** [ui.py L671](../../app/codex_dashboard/ui.py#L671) nav tuple
  `("tasks", "Tasks")` → `("worktrees", "Worktrees")`; `select_tab` /
  `_render_active_tab` build the worktree surface; the old committed-task
  stream/detail/dispatch widgets are removed from this tab (D1 = replace). Tab switch
  stays read-only (no backend write on switch).
- **Pool view.** On tab activation (and on a refresh control), `GET /api/v1/worktrees`
  → render one row per worktree with repo, `worktree_path`, identifier, and (allocated)
  `task_id` / `run_gate_state`. **Allocated rows use a distinct background** vs idle
  rows (reuse the palette). Backend-unavailable → a clear message, no crash.
- **Copy-path.** Per-row copy control writes `worktree_path` to the clipboard.
- **Repo filter.** `GET /api/v1/repos` populates a dropdown (registry-sourced) + "All
  repos"; selecting filters the view.
- **Create.** A control → `POST /api/v1/worktrees/create {repo}` for the selected repo
  → refresh → the new idle worktree appears.
- **Assign popup.** On an idle worktree, a popup lists open tasks
  (`tasks_backend.fetch_tasks_snapshot` → `GET /api/v1/tasks`) as id + title + state
  (no progress bars — E6); confirm → `POST /api/v1/worktrees/assign {task_id, repo,
  worktree_id}` → the worktree flips to allocated.
- **Eject.** On an allocated worktree → `POST /api/v1/worktrees/eject {run_id}` → the
  worktree returns to idle in the view; the freed task is dequeued (reflected by the
  refreshed read).
- **Destroy.** On an idle worktree → `POST /api/v1/worktrees/destroy {worktree_id}` →
  it disappears. On an allocated worktree the backend's 409 rejection is shown as a
  clear message; nothing is removed.
- **Dequeue.** A standalone control → `POST /api/v1/worktrees/dequeue {repo, task_id}`
  without ejecting; does not close the issue.

The pure helpers (grouping, color selection, row/popup formatting) live in
`worktrees_tab.py` (unit-tested); the HTTP calls live in `worktrees_backend.py`
(mirroring `tasks_backend.py`); the Tk widget wiring lives in `ui.py`.

## Expected Resolution

After this task, the operator opens the desktop app, clicks the **`WORKTREES`** tab,
and the surface works: the pool view shows every worktree (allocated rows visibly
distinct from idle), a registry-sourced repo filter narrows it, the copy control puts a
worktree's path on the clipboard, **Create** adds an idle worktree, the **Assign** popup
binds an open task onto an idle one (it flips allocated), **Eject** returns an allocated
worktree to idle (and dequeues its task), **Destroy** removes an idle one (and refuses an
allocated one with a clear message), and **Dequeue** takes a task out of the queue
without ejecting. The named in-app cases REG-010…REG-016 pass on the validation lane.

Equivalently, against a backend with a two-repo registry (`obsidian`, `demo`),
an operator working purely over HTTP (the same endpoints the tab calls) can:

1. `POST /api/v1/worktrees/create {repo:"obsidian"}` twice → two idle worktrees
   `obsidian/wt-0001`, `obsidian/wt-0002` appear in `GET /api/v1/worktrees` as
   `status:"idle"` with real paths.
2. `POST /api/v1/worktrees/assign {task_id:"Task-0007", repo:"obsidian", worktree_id:"obsidian/wt-0001"}`
   → `wt-0001` flips to `allocated` bound to `Task-0007` (`run_gate_state:"running"`);
   the *same* folder is reused (no new dir created).
3. Flip a GitHub issue `Queue=Ready` (REG-007 surface) → the consumer draws
   `obsidian/wt-0002` (the remaining idle one) and dispatches into it; with both
   now allocated, a *third* Ready issue **waits** (empty pool ⇒ defer).
4. `POST /api/v1/worktrees/eject {run_id:"...Task-0007..."}` → the agent stops, the
   checkout is reset to baseline, `wt-0001` returns to `status:"idle"` with the
   **same folder still on disk** and no task bound, and `Task-0007`'s issue is
   **dequeued** (`Queue → Never`, still open). On the next consumer poll the freed
   task is **not** re-dispatched (it bounced back before this change). To restart it
   the operator flips `Queue=Ready` again on GitHub.
5. `POST /api/v1/worktrees/destroy {worktree_id:"obsidian/wt-0001"}` (now idle) →
   it is removed from the pool; attempting Destroy on the still-allocated `wt-0002`
   is **rejected** until it is Ejected.
6. Kill and restart the backend → `GET /api/v1/worktrees` still shows the pool with
   each worktree's idle-vs-allocated state reconstructed from disk + the live
   workflows.

`demo`'s pool is independent of `obsidian`'s (REG-009): a Create/Assign/Eject on
one repo never touches the other.

## Acceptance Criteria

Each criterion is pass/fail. Criteria 1–15 are **[BE]** (backend); 16–22 are **[FE]**
(the desktop `WORKTREES` tab). **"Done" is gated on the [FE] criteria plus the named
in-app regression cases (REG-010…REG-016) passing — not on the [BE] criteria alone.**

1. **queue_workers removed.** `QueueWorkers` no longer exists on
   [`RepoEntry`](../../backend/orchestration/internal/queue/manifest.go#L29), and
   the queue-drain consumer admits a dispatch based on idle-pool availability, not
   a numeric cap: with a pool of 1 idle worktree and two Ready issues, the consumer
   dispatches exactly one and skips the second (it waits). Asserted by an updated
   [`consumer_test.go`](../../backend/orchestration/internal/queue/consumer_test.go)
   that drives admission from a pool (no `fixedSizer`/`EvaluateSlot` cap). A grep
   proof shows `queue_workers` / `RepoSlotLimit` / `EvaluateSlot` are no longer a
   live admission cap anywhere.
2. **Create.** `POST /api/v1/worktrees/create {repo}` provisions one new idle
   worktree at a **stable** path (not `os.MkdirTemp`), persists its pool record with
   `run_id=null`, and a follow-up `GET /api/v1/worktrees` lists it `status:"idle"`
   with that `worktree_id` + `worktree_path`. Go unit test (fake runtime as in
   [`service_test.go`](../../backend/orchestration/internal/taskrun/service_test.go)).
3. **Assign reuses an idle worktree.** `POST /api/v1/worktrees/assign {task_id, repo, worktree_id}`
   resets the chosen **existing** idle worktree and starts the run in it; a Go unit
   test asserts (a) `StartTaskRun` was called for that task, (b) the run is bound to
   the **same** `worktree_path` that was idle (no fresh dir provisioned), and (c)
   the pool count did not grow.
4. **Assign rejects when none idle.** `POST /api/v1/worktrees/assign` returns 409
   and starts no run when the repo has no idle worktree. Go unit test.
5. **Auto-assign draws from the pool.** With `worktree_id` omitted (the consumer
   path), Assign picks any idle worktree in the repo; the consumer dispatches into a
   pool worktree and **defers** (skips) when the pool is empty. Go unit test on the
   consumer + service seam.
6. **Eject keeps the folder + returns idle + dequeues.** `POST /api/v1/worktrees/eject {run_id}`
   stops the agent, resets the checkout to baseline, unbinds the run, and the folder
   **still exists on disk** afterward, listed `status:"idle"`. A Go unit test
   asserts the directory still exists and the pool record `run_id` is null —
   covering **both** a `running` lane and a `parked_*` lane (works regardless of
   parked state). A test that finds the folder deleted **fails**. The same test (or a
   sibling) asserts that Eject called the provider dequeue for the freed task's issue
   (`Queue → Never`) via a **fake provider**, and that it did **not** call the
   provider close path. A test where Eject leaves the issue `Queue=Ready` **fails**.
7. **Destroy rejects allocated; removes idle.** `POST /api/v1/worktrees/destroy {worktree_id}`
   returns 409 and removes nothing when the worktree is allocated; when idle it
   removes the folder (via
   [`removeOwnedLaneWorktree`](../../backend/orchestration/internal/taskrun/service.go#L1688))
   and drops the pool record. Go unit tests for both cases.
8. **Discover-on-startup across restart.** Given pool folders on disk — some bound
   to a live run, some idle — a fresh `Service` (simulating restart) reconstructs
   each worktree's allocated-vs-idle state from disk + the live workflow, with no
   bound state lost. Go unit test that builds the on-disk pool, constructs a new
   service, and asserts `ListPoolWorktrees()` reports the correct `status` and bound
   `task_id`/`run_id` per worktree.
9. **Full-pool read.** `GET /api/v1/worktrees` returns allocated + idle entries,
   each with `worktree_id`, `repo`, `worktree_path`, `status`; allocated entries add
   `task_id`, `run_id`, `run_gate_state` from the live binding. Asserted by a Go
   unit test on the handler/service.
10. **Repos read without queue_workers.** `GET /api/v1/repos` returns one entry per
    registered repo with `id` + `local_root` and **no** `queue_workers`, matching
    [`RegistryRepos()`](../../backend/orchestration/internal/queue/registry_consumer.go#L33)
    for a fixture registry. Go unit test.
11. **Method/path guards.** The new routes return 405 on the wrong method and 404
    on unknown sub-paths, matching the existing handler guards. Asserted in
    `mux_test.go`.
12. **Provider dequeue write sets not-ready.** The new
    [`QueueProvider`](../../backend/orchestration/internal/queue/provider.go#L22)
    dequeue write sets the task's queue state to not-ready (`github_issues`:
    `Queue → Never`) **through the provider**. A Go unit test against a **fake/mock
    provider** asserts the dequeue write call was made with the correct repo + issue
    number and the not-ready value. The test must **not** hit real GitHub (it uses
    the fake provider / injectable `run` func, exactly as the existing provider tests
    do). A dequeue implemented as a hardcoded inline `gh`/GitHub call inside Eject
    instead of on the provider **fails** this criterion.
13. **Eject does not re-dispatch (no bounce-back).** After Eject of a task whose
    issue is `Queue=Ready`, a subsequent consumer poll does **not** re-dispatch that
    task (because Eject dequeued it). Asserted by a Go test on the consumer + service
    seam with a **fake provider**: Eject sets the fake's `Queue` to `Never`, the next
    `DrainOnce` observes `Never`, and the task is **not** in `Dispatched`. A variant
    where Eject skips the dequeue (issue stays `Ready`) shows the task **is**
    re-dispatched — proving the dequeue is load-bearing.
14. **Dequeue leaves the issue open (not closed).** The provider dequeue write and
    the Eject dequeue **only** set `Queue=Never`; they do **not** call any close
    path. A Go unit test asserts the fake provider's `CloseIssue` was **never**
    invoked by Eject / dequeue (the issue stays open). A dequeue that closes the
    issue **fails**.
15. **Standalone dequeue endpoint.** `POST /api/v1/worktrees/dequeue` invokes the
    provider dequeue for the named task **without** ejecting (the run is not stopped
    and the worktree is not returned to idle) and does **not** close the issue;
    method/path guards return 405 on the wrong method and 404 on unknown sub-paths.
    Asserted in `mux_test.go` + a service test with a fake provider.

### Frontend acceptance criteria (the desktop `WORKTREES` tab)

Each [FE] criterion is pass/fail and is proven **in the running app** against the live
backend on the validation lane (the named regression case in parentheses), with pure
helpers also covered by Python unit tests. A criterion satisfied only by a unit test,
backend endpoint, or screenshot of a non-functional widget does **not** pass.

16. **[FE] Tab renamed + replaced.** The desktop nav shows a **`WORKTREES`** tab where
    `TASKS` used to be; clicking it renders the worktree-management surface (not the old
    task stream/detail/dispatch widgets), and the switch performs no backend mutation.
    `Usage` and `Jobs` are unchanged. (REG-010)
17. **[FE] Full-pool view with allocated/idle color.** The tab lists every worktree from
    `GET /api/v1/worktrees`, each showing repo + local dir + identifier (+ bound
    task/run/status for allocated), and **allocated rows are a visibly different
    background color** from idle rows. A view that shows allocated vs idle only as
    identical-looking text **fails**. (REG-010)
18. **[FE] Copy-path control.** Clicking a row's copy control places that worktree's
    exact `worktree_path` on the system clipboard. (REG-011)
19. **[FE] Registry-sourced repo filter.** The repo filter dropdown is populated from
    `GET /api/v1/repos` (the registry) — not a hardcoded list — and selecting a repo
    narrows the pool view to that repo; "All repos" restores the full view. A hardcoded
    repo list **fails**. (REG-012)
20. **[FE] Create from the UI.** The Create control calls
    `POST /api/v1/worktrees/create` for the selected repo and, after success, the new
    idle worktree appears in the view. (REG-013)
21. **[FE] Assign popup binds an open task.** The Assign control on an idle worktree
    opens a popup listing open tasks (from `GET /api/v1/tasks`, id + title + state);
    confirming a selection calls `POST /api/v1/worktrees/assign` and the worktree flips
    to **allocated** bound to that task in the view. A popup whose list is empty/hardcoded
    or that does not actually bind **fails**. (REG-014)
22. **[FE] Eject, Destroy, and Dequeue from the UI.** From the tab: **Eject** on an
    allocated worktree returns it to **idle** in the view (folder kept; the task is
    dequeued); **Destroy** on an idle worktree removes it from the view, while Destroy on
    an allocated worktree surfaces the backend's rejection as a clear message and removes
    nothing; **Dequeue** takes a task out of the queue without ejecting (the worktree
    stays allocated, the issue stays open). (REG-015 Eject; REG-016 Destroy + Dequeue)

**Regression [must not break]:** `REG-007`, `REG-008`, `REG-009` in
[`REGRESSION.md`](../../REGRESSION.md) re-run **green under the new model**,
with the pool **seeded (via Create) before the drain can dispatch**:

- **REG-007** — "cap=1" is now "**a pool of 1**": with one idle worktree, the
  consumer dispatches exactly one Ready issue and the second **waits for an idle
  worktree**; on close/eject the freed worktree is reused. (The REG-007 cap=1
  sub-scenario is reinterpreted: one idle pool worktree, not `queue_workers=1`.)
- **REG-008** — durable state survives a backend restart: the parked lane is still
  reported from the workflow, and the pool's allocated-vs-idle classification is
  reconstructed by **discover-on-startup**.
- **REG-009** — each repo's pool is independent: a Create/Assign/Eject/Destroy or a
  close on repo A never touches repo B's pool worktrees.

**New in-app regression [must pass for closure]:** the new desktop `WORKTREES`-tab
cases **REG-010 … REG-016** in [`REGRESSION.md`](../../REGRESSION.md) — one per new
human surface (pool view + allocated/idle color + repo + path + copy; repo filter;
Create; Assign popup → bind; Eject; Destroy; Dequeue) — must **pass in-app on the
isolated validation lane** against the live backend before closure.

## What Does Not Count

- An **Eject that deletes the folder** (it must keep the folder and return it to
  idle). Deleting is **only** Destroy, and only for an idle worktree.
- An **Eject that leaves the issue `Queue=Ready`** so the consumer re-dispatches it
  on the next poll (the bounce-back). Eject must dequeue (`Queue → Never`).
- A **dequeue that bypasses the `TaskProvider` abstraction** — a hardcoded inline
  `gh`/GitHub call inside Eject (or anywhere outside the provider) instead of the
  provider write method symmetric to the `Queue` read and to `CloseIssue`.
- A **dequeue that closes the issue** instead of only setting `Queue=Never`. Dequeue
  must leave the issue **open** (human-only closure is preserved; the agent never
  self-closes).
- An **Assign that provisions a fresh dir** (calls
  [`provisionOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1507)
  / `os.MkdirTemp`) instead of reusing an existing idle pool worktree, or that grows
  the pool count.
- Leaving **`queue_workers` as a live cap** anywhere — in the manifest as a consulted
  field, in `RepoSlotLimit`/`SlotSizer`/`EvaluateSlot`/`EffectiveFreeConcurrency`, or
  threaded into `NewServiceForRepo`. Renaming it is not removing it.
- A **discover that loses bound state across a restart** (e.g. reclassifies a
  live-allocated worktree as idle, or drops it).
- A **Destroy that deletes an allocated worktree** (it must reject and require Eject
  first).
- **Auto-creating** a worktree to satisfy a Ready issue instead of deferring on an
  empty pool.
- A backend that emits a `vscodium://` link itself rather than supplying the raw
  fields (the O6 orchestrator boundary in
  [types.go L177–L208](../../backend/orchestration/internal/taskrun/types.go#L177)).
- **[FE] Backend-only "done."** Treating the task as done because the endpoints exist
  and pass unit/server-smoke, while the desktop `WORKTREES` tab does not actually work
  in-app. The working in-app surface (REG-010…REG-016) is the closure bar.
- **[FE] A non-functional or fake UI.** A tab that renders rows but whose controls do
  not call the real endpoints; a repo filter from a **hardcoded** list instead of
  `GET /api/v1/repos`; an Assign popup with an empty/hardcoded task list or that does
  not actually bind; allocated vs idle shown only as identical-looking text with **no
  visible color distinction**; or a copy control that copies the wrong/no path.
- **[FE] A web migration / new frontend stack.** Porting to HTML/Tailwind or a new
  framework instead of re-implementing in the existing Python/Tkinter app reusing its
  styles (Q1 = a). The Stitch HTML is a structural guide, not a port target.
- **[FE] Implementing an excluded mockup element** (E2–E7): drag-to-bind / a persistent
  Task Browser pane, Register New Task, a fabricated agent-model chip, animated
  transitional states, decorative per-task progress bars, or top-nav chrome changes.
- **[FE] Keeping the old `TASKS` surface.** Leaving the old task stream/detail/
  dispatch-pause-poke content on this tab (D1 = replace requires removing it), or not
  renaming the tab to `WORKTREES`.

## Proof Plan

- **Go unit tests (no app):** add/extend
  [`service_test.go`](../../backend/orchestration/internal/taskrun/service_test.go)
  (Create / Assign-reuses-idle / Assign-rejects-when-none / Eject-keeps-folder for
  running + parked / Eject-dequeues-via-fake-provider / Eject-does-not-close /
  Destroy-rejects-allocated / discover-across-restart / full-pool read /
  standalone-dequeue), the
  [`worktrees_test.go`](../../backend/orchestration/internal/httpapi/worktrees_test.go)
  + `mux_test.go` (route shapes incl. `/worktrees/dequeue`, repos read without
  `queue_workers`, method/path guards), and
  [`provider_test.go`](../../backend/orchestration/internal/queue/provider_test.go)
  / [`consumer_test.go`](../../backend/orchestration/internal/queue/consumer_test.go)
  / `slots_test.go` (admission from pool, not cap; the new dequeue write against the
  injectable `run` func / a fake provider asserting `Queue → Never` and **no**
  close; the eject-then-no-redispatch consumer+service seam test). All
  provider-related tests use a **fake/mock provider** or the injectable `run` and
  **must not hit real GitHub**. Run `go test ./...` under
  [`backend/orchestration`](../../backend/orchestration/).
- **Server-only smoke for the new endpoints:** start the backend against a
  throwaway registry and exercise Create → list → Assign → list → Eject → list →
  Destroy → list with `curl`/PowerShell, asserting the JSON shapes and that the
  Ejected folder persists on disk and the Destroyed one is gone. (This is a
  `server-only smoke`, supporting proof, not a regression.)
- **[FE] Python unit tests (no app):** add `tests/test_worktrees_tab.py` (mirroring the
  existing tab tests) for the pure `worktrees_tab.py` helpers — worktree grouping/sort,
  allocated-vs-idle color selection, per-row + Assign-popup formatting — and
  `worktrees_backend.py` mapping/error-snapshot logic against backend-shaped fixtures.
  Run via `python -m unittest discover -s tests -p "test_*.py"` ([TESTING.md](../../TESTING.md)).
  This supports but does **not** replace the in-app cases.
- **[FE] In-app regression (the closure bar):** run **REG-010 … REG-016** in the
  **running desktop app** on the isolated validation lane (`CODEX_DASHBOARD_*_BACKEND_URL`
  → `http://127.0.0.1:14318`, task-owned config + SQLite per [TESTING.md](../../TESTING.md)),
  against the live backend, seeding the pool via Create. Capture an app-surface artifact
  per case under `Tracking/Task-0016/Testing/`. These are **real in-app cases** (the UI is
  in scope), not server-only smoke.
- **REG-007 / REG-008 / REG-009 re-run (in-app):** on the isolated `reg007` lane
  against the throwaway testbed repos, drive the GitHub web surface + consumer as
  the existing cases require, but with the **pool seeded via Create** first
  (REG-007 "pool of 1"; REG-008 restart reconstructs the pool via discover; REG-009
  per-repo pool independence). Capture proof under `Tracking/Task-0016/Testing/`.
- **Testing nuance — dequeue write vs the REG-007 Ready-flip rule (record, not a
  conflict):** the product dequeue is a **backend-owned provider WRITE** (`Queue →
  Never`) against a **throwaway reg007 testbed repo**, distinct from the REG-007
  *testing* rule that the **Ready** flip must be driven at the real GitHub web UI.
  That rule governs how the regression *sets* `Ready` to prove the real surface; it
  does **not** forbid the product from writing `Queue=Never` on an operator dequeue.
  Any in-app re-run that exercises Eject/dequeue does so against the isolated reg007
  testbeds, so the product write never touches a production-owned queue.

## Open Questions

None are decisive enough to change the writeup type, home, scope, solution shape,
enforcement boundary, or acceptance bar. Non-blocking defaults the implementer may
keep without re-litigating scope:

- **Eject key — `run_id` vs `worktree_id`.** Default: **`run_id`** (matches the
  existing `resolve-interrupt-review` keying and is unambiguous for a live run).
  Accept `worktree_id` as an alternate request key if the caller has it instead;
  both resolve to the same allocated worktree.
- **Pool record location — extend `owned-lane-bootstrap.json` vs a sibling
  `worktree-pool.json` per folder.** Default: extend the existing breadcrumb so the
  discover path reuses
  [`collectActiveLaneRecords`](../../backend/orchestration/internal/taskrun/service.go#L271);
  a sibling per-folder file is acceptable if it makes idle (`run_id=null`)
  persistence cleaner. Either way the record must carry `worktree_id`, stable path,
  repo, and `run_id`-or-null.
- **Stable id/path scheme.** Default: `<ownedLaneRoot>/<repoID>/wt-<NNNN>/w` with
  `worktree_id = <repoID>/wt-<NNNN>`; the exact zero-pad width is an implementation
  detail, not a scope decision.
- **Standalone dequeue route — `/api/v1/worktrees/dequeue` vs
  `/api/v1/tasks/{taskID}/dequeue`.** Chosen: **`POST /api/v1/worktrees/dequeue`**.
  Rationale: dequeue is part of the same worktree-management operator lane as
  create/assign/eject/destroy (the surface the `WORKTREES` tab drives), it shares the
  worktree handler's method/path guards, and keeping all five pool operations on one
  `/worktrees/*` sub-router is more consistent than splitting one onto the separate
  task router. This is a route-placement choice, not a scope decision; the dequeue
  *behavior* (provider write `Queue → Never`, never close) is fixed.
- **Dequeue write method name/signature on the provider.** Default:
  `DequeueIssue(repo string, number int) error` on
  [`QueueProvider`](../../backend/orchestration/internal/queue/provider.go#L22)
  (equivalently `SetQueueState(repo, number, QueueNever)`); the exact name is an
  implementation detail, not a scope decision. Either way it is a provider WRITE that
  sets `Queue=Never` and never closes the issue.
- **[FE] Assign-popup open-tasks source — RESOLVED.** The popup queries the existing
  **`GET /api/v1/tasks`** (local committed tasks bound to issues) via the existing
  [`tasks_backend.fetch_tasks_snapshot`](../../app/codex_dashboard/tasks_backend.py#L29)
  client, consistent with "GitHub Issues is the task surface." This resolves the
  original [Open implementation detail](#) the writer was asked to pin; it is not an
  open question anymore.
- **[FE] `worktree_id` vs `run_id` from the UI.** For Eject the tab has the allocated
  worktree's `run_id` from the pool read, so it keys Eject on `run_id` (the [BE]
  default); for Destroy/Assign it uses `worktree_id`. Non-blocking; both are accepted
  request keys per the [BE] handlers.

## References

- Human directives (authoritative; the 2026-05-31 design pivot governs):
  [`HUMAN-DIRECTIVES-FOR-WORKER.md`](./HUMAN-DIRECTIVES-FOR-WORKER.md)
- TaskCreate objective: [`TASK-CREATE-OBJECTIVE.md`](./TASK-CREATE-OBJECTIVE.md)
- Context manifest: [`TASK-CREATE-CONTEXT-MANIFEST.md`](./TASK-CREATE-CONTEXT-MANIFEST.md)
- Backend service + worktree authority:
  [`taskrun/service.go`](../../backend/orchestration/internal/taskrun/service.go)
  (provisionOwnedLane L1507, restoreOwnedLane L1726, cleanupOwnedLane L1640,
  removeOwnedLaneWorktree L1688, ReconcileOwnedLanes L1663, ListActiveWorktrees L223,
  dispatchWithDirective L671, NewServiceForRepo L102, RepoSlotLimit L616),
  [`taskrun/types.go`](../../backend/orchestration/internal/taskrun/types.go)
  (RepoBinding L182, RunGateState enum L147, TaskRunView L242)
- Routes:
  [`httpapi/mux.go`](../../backend/orchestration/internal/httpapi/mux.go)
  (NewMux L25, handleWorktreesList L130, handleTaskAPIRoute L202,
  resolve-interrupt-review L429)
- Manifest + consumer (queue_workers removal + pool-draw):
  [`queue/manifest.go`](../../backend/orchestration/internal/queue/manifest.go)
  (RepoEntry.QueueWorkers L32),
  [`queue/decision.go`](../../backend/orchestration/internal/queue/decision.go)
  (EffectiveFreeConcurrency L144, QueueNever L23, IssueState.Queue L48),
  [`queue/consumer.go`](../../backend/orchestration/internal/queue/consumer.go)
  (SlotSizer L46, DrainOnce admission L141/L205, IssueNumberFromTaskID L93),
  [`queue/slots.go`](../../backend/orchestration/internal/queue/slots.go)
  (EvaluateSlot L39),
  [`temporalbackend/queuedrain.go`](../../backend/orchestration/internal/temporalbackend/queuedrain.go)
  (NewServiceForRepo wiring L196, reconcile call L226)
- Task-provider abstraction + the dequeue WRITE home (UPDATE 2):
  [`queue/provider.go`](../../backend/orchestration/internal/queue/provider.go)
  (QueueProvider interface L22, ghQueueProvider L49, ListReadyIssues Queue read
  L119/L151-153, fieldIDMap L174, fieldValues L202, CloseIssue write precedent L166),
  built into the per-repo Service via
  [`NewGitHubQueueProvider`](../../backend/orchestration/internal/temporalbackend/queuedrain.go#L189)
- Worktree model + Task-0015 durable-state authority:
  [`WORKTREES.md`](../../../../Users/gregs/.codex/Orchestration/WORKTREES.md),
  [`QUEUE-DRAIN-DURABLE-STATE-REDESIGN.md`](../Task-0015/Design/QUEUE-DRAIN-DURABLE-STATE-REDESIGN.md)
- Regression cases that must not break, and the new in-app cases authored for this
  task: [`REGRESSION.md`](../../REGRESSION.md) (REG-007/008/009 must stay green;
  REG-010…REG-016 are the new `WORKTREES`-tab cases that must pass for closure).
- Frontend home (the desktop `WORKTREES` tab, in scope per UPDATE 3):
  [`app/codex_dashboard/ui.py`](../../app/codex_dashboard/ui.py)
  (nav tuple L671, `select_tab` L1152, `_render_active_tab` L1164, `_configure_styles`
  + palette constants L100-102),
  [`app/codex_dashboard/tasks_backend.py`](../../app/codex_dashboard/tasks_backend.py)
  (HTTP-client pattern to mirror; `fetch_tasks_snapshot` L29 reused by the Assign popup),
  [`app/codex_dashboard/tasks_tab.py`](../../app/codex_dashboard/tasks_tab.py)
  (pure-helper pattern to mirror). New files:
  `app/codex_dashboard/worktrees_backend.py`, `app/codex_dashboard/worktrees_tab.py`,
  `tests/test_worktrees_tab.py`.
- UI mockup (structural guide only, Q1 = a):
  `C:\Users\gregs\Downloads\stitch_codex_token_velocity_overlay (4)`.

## Audit status

Revised for the **UPDATE 3 scope reversal** (the Tk `WORKTREES` tab is back in scope;
"done" = the working in-app surface). Local draft awaiting coordinator review and the
human PLAN-approval gate. Not yet human-audited or agent-audited. `TASK-META.json` is
already bound to GitHub issue #16 (TaskCreate provider-binding gate done earlier).
