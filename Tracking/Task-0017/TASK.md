# Task 0017

## Title

Make ASSIGN feel instant on the WORKTREES tab — clicking ASSIGN opens the popup immediately (no ~500 ms freeze) while the open-task list loads in the background — and prove the bind→done→clear lifecycle: a worktree assigned to a task visibly returns to idle once that task is done.

## Writeup Type

Concrete implementation task.

The main mechanism is already chosen and the files are named: move the one synchronous, UI-thread `fetch_tasks_snapshot(...)` call out of [`open_assign_popup`](../../app/codex_dashboard/ui.py#L1586) and onto a worker thread, reusing the app's existing worker → `ingest_queue` → `root.after(...)` pattern (see [`_start_activation_load` / `schedule_ingest`](../../app/codex_dashboard/ui.py#L2281) and [`_poll_ingest_results`](../../app/codex_dashboard/ui.py#L2244)). Part 2 is a *test* of the already-shipped Task-0016 worktree lifecycle through the human surface, not new backend work. There is one honest open decision — exactly which "done" trigger the simple lifecycle test uses — which this draft resolves to a named default (issue closure, the human-only-closure path) and records the alternative (Eject) under [Open Questions](#open-questions); that choice changes the test procedure, not the writeup type, so the task stays concrete implementation.

## Summary

The human operates the desktop dashboard's **WORKTREES** tab to manage a pool of reusable git worktrees. Two things this task fixes/proves, in human terms first:

1. **A snappy ASSIGN.** Today, when the operator clicks **ASSIGN** on a worktree card, the whole overlay visibly freezes for roughly half a second before the Assign popup appears. Nothing is drawn during that stall. After this task, clicking ASSIGN opens the popup **immediately** with a brief loading state, and the open-task list fills in a moment later when the network read returns — the overlay never freezes on that click.

2. **A visible bind → done → clear lifecycle.** The human wants to run one simple, honest end-to-end exercise: assign a task to an idle worktree (the worktree flips to **allocated** and shows the bound task's GitHub-issue link on its card), then mark that task "done" and watch the worktree **return to idle**. This task pins down exactly what "done" means here (see below), names the simplest honest trigger, and gives a concrete runnable procedure on the isolated validation lane.

**The technical seam (part 1).** [`open_assign_popup`](../../app/codex_dashboard/ui.py#L1586) runs `tasks_snapshot = fetch_tasks_snapshot(self.worktrees_backend_url)` synchronously on the Tk UI thread at [ui.py L1590](../../app/codex_dashboard/ui.py#L1590) **before any popup window is built**. [`fetch_tasks_snapshot`](../../app/codex_dashboard/tasks_backend.py#L29) is a blocking HTTP GET (`/api/v1/tasks`, [10 s timeout](../../app/codex_dashboard/tasks_backend.py#L15)). Because it runs on the UI thread before `tk.Toplevel(...)` is created, the event loop is blocked and nothing is drawn until the HTTP call returns — that round trip is the ~500 ms stall the operator perceives. The app already has the right pattern for this: do the network read on a `threading.Thread`, post the result onto `self.ingest_queue`, and apply it back on the UI thread via the [`_poll_ingest_results`](../../app/codex_dashboard/ui.py#L2244) drain that `self.root.after(100, ...)` already runs.

**The lifecycle truth that makes "done"/"cleared" precise (part 2).** Per Task-0016 and the obsidian-operator [Queue Done-Contract](../../skills/obsidian-operator/SKILL.md), the agent **never self-closes**, and **only a CLOSED issue deallocates** a worktree (the consumer reclaims it). `Human Needed=Yes` (including awaiting-closure) **parks in place** — the worktree is retained, not freed. **Eject** is the operator-initiated clear that returns a folder to idle (folder kept, run terminated, task dequeued) without deleting it. So "the worktree cleared when the task is done" has two honest meanings, and this task names one as the default:

- **(default) Human closes the bound issue** — the human-only-closure path. After a human CLOSES the issue (via the operator close path, e.g. obsidian-operator's reconcile close), the consumer / discover-on-startup reclaims the worktree and it returns to **idle**. This is the truest "task is done → worktree clears" story and it preserves human-only closure exactly.
- **(alternative) Operator Ejects** — the operator clicks **Eject** on the allocated worktree; it returns to idle immediately (folder kept, run terminated, task dequeued to `Queue=Never`, issue stays open). This is faster to demonstrate but is "operator cleared it," not "the task reached done," so it is recorded as the alternative, not the default.

Both are human actions; neither adds an agent self-close. The default is chosen because the human's words were "see the worktree cleared **when task is done**" — closure is the event that means "done" in this system. The Eject alternative is named so the human can pick the faster demonstration if they prefer.

**Current truth that must not masquerade as success.** The ASSIGN stall is real and on the UI thread; a fix that only *speeds up* the fetch (shorter timeout, caching) but still calls it on the UI thread before drawing the popup is **not** this task — the popup must appear without waiting on the network. For part 2, a backend-only `/worktrees` JSON transition, or a unit test of the lifecycle helpers, does **not** satisfy the "visible bind→clear" exercise — the allocated→idle transition must be observed in the running app's WORKTREES tab.

## Goals

1. **[FE] Open the Assign popup without blocking the UI thread.** Restructure [`open_assign_popup`](../../app/codex_dashboard/ui.py#L1586) so that clicking ASSIGN builds and shows the popup **immediately** with a transient loading state (e.g. a "Loading open tasks…" line in the list area), then runs `fetch_tasks_snapshot(...)` on a `threading.Thread` worker, posts the result onto `self.ingest_queue`, and populates the popup's task list back on the UI thread via the existing [`_poll_ingest_results`](../../app/codex_dashboard/ui.py#L2244) → `self.root.after(...)` drain. No `fetch_tasks_snapshot(...)` (or any blocking HTTP/IO call) remains on the synchronous path of `open_assign_popup` before the popup window is created.
2. **[FE] Preserve the existing popup contract.** Keep the popup **non-modal** and **Escape-to-close** (and the close X / CANCEL). Do **not** reintroduce `grab_set()` (the [comment at ui.py L1618–1625](../../app/codex_dashboard/ui.py#L1618) records that a modal grab on this borderless topmost popup froze every overlay click — that bug must not return). Keep the popup centered on the overlay, the cyan top border, the pinned footer with ASSIGN/CANCEL, the filter/sort toolbar, and the scrollable task list (the REG-019 / BUG-0007 layout is unchanged). Keep the existing backend-error handling: if the background fetch fails, the open popup shows a clear human-facing message instead of a crash (today's `except` branch sets a worktrees status message; preserve an equivalent human-facing failure surface from the worker path).
3. **[Test] Run and document the bind→done→clear lifecycle exercise on the isolated lane.** Using the WORKTREES tab on the isolated validation lane (`http://127.0.0.1:14318`) against a throwaway testbed task (never production), perform and capture: (a) **bind** — Assign an open task onto an idle worktree; the worktree flips to **allocated** and its card shows the bound task's clickable GitHub-issue link ([`worktree_issue_url`](../../app/codex_dashboard/worktrees_tab.py#L125) → the link rendered at [ui.py L1340–1352](../../app/codex_dashboard/ui.py#L1340)); (b) **done** — trigger the chosen "done" event (default: a human CLOSES the bound issue via the operator close path); (c) **clear** — the worktree returns to **idle** in the WORKTREES tab view (idle background, no bound task). Record the exact trigger used and confirm no agent self-close occurred.

## Non-Goals

- **No global async refactor.** Only the Assign-popup open path changes. Other screens' fetches (the Usage source filter, the Jobs tab, the pool read on tab activation, the repo-filter read) are **not** touched. This task does not introduce an async framework or a general "every fetch off-thread" pattern; it reuses the existing worker→queue→after seam for this one path only.
- **No new backend lifecycle.** The Task-0016 backend (Create / Assign / Eject / Destroy / Dequeue + discover-on-startup, and the Eject-dequeue write) is done and is **not** modified. Part 2 is a *test* of that existing lifecycle, not new endpoints.
- **No change to human-only closure.** This task adds **no** agent self-close and **no** new autonomous deallocation path. The "done" trigger is a human action (issue close) or an operator action (Eject); the consumer's reclaim semantics ([`decision.go`](../../backend/orchestration/internal/queue/decision.go): only a closed issue deallocates; `Human Needed=Yes` parks) are unchanged.
- **No backend port / lane change.** The default backend bind stays `http://127.0.0.1:4318`; the test runs on the isolated validation lane override `http://127.0.0.1:14318` (per [TESTING.md](../../TESTING.md) / [REGRESSION.md](../../REGRESSION.md)).
- **No popup redesign.** No new filter/sort behavior, no new task-card fields, no modal grab. The only visible change is that the popup appears immediately with a momentary loading state before the list fills in.

## Implementation Home

One code home for the fix: the Python/Tkinter desktop app, specifically [`app/codex_dashboard/ui.py`](../../app/codex_dashboard/ui.py). The blocking call, the popup construction, and the existing worker→queue→after machinery all live in this one file, so the fix is local to it.

- **The fix lives in [`open_assign_popup`](../../app/codex_dashboard/ui.py#L1586)** (and a small worker + an ingest-queue event branch in [`_poll_ingest_results`](../../app/codex_dashboard/ui.py#L2244)) because that is the exact method that today calls `fetch_tasks_snapshot` synchronously on the UI thread at [L1590](../../app/codex_dashboard/ui.py#L1590). The right pattern is already in the same class: [`_start_activation_load`](../../app/codex_dashboard/ui.py#L2281) spawns a `threading.Thread`, the worker puts a tagged tuple on `self.ingest_queue`, and [`_poll_ingest_results`](../../app/codex_dashboard/ui.py#L2244) — already pumped by `self.root.after(100, self._poll_ingest_results)` — applies it on the UI thread. The Assign-popup fetch should use the same seam, not a new mechanism.
- **The HTTP client is unchanged.** [`fetch_tasks_snapshot`](../../app/codex_dashboard/tasks_backend.py#L29) stays as-is; only its *call site* moves off the UI thread. No change to [`worktrees_backend.py`](../../app/codex_dashboard/worktrees_backend.py) or [`worktrees_tab.py`](../../app/codex_dashboard/worktrees_tab.py).
- **No backend home.** Part 2 exercises the existing Task-0016 backend through HTTP; it does not edit Go code.

## Proposed Changes

### 1. Move the Assign-popup open-task fetch off the UI thread (the only code change)

In [`open_assign_popup`](../../app/codex_dashboard/ui.py#L1586), reorder so the popup is built and shown first, then the fetch runs on a worker:

- **Remove** the synchronous `tasks_snapshot = fetch_tasks_snapshot(self.worktrees_backend_url)` at [L1590](../../app/codex_dashboard/ui.py#L1590) from the top of the method.
- **Build and show the popup immediately** (the `tk.Toplevel`, cyan border, header, pinned footer, toolbar, and the scrollable `list_shell`/`canvas`/`inner` are constructed exactly as today — keep the non-modal + Escape + lift/focus behavior and the no-`grab_set()` decision). Render a transient **loading state** in the list area (e.g. a single "Loading open tasks…" label where the empty/`No open tasks` message renders today at [L1671](../../app/codex_dashboard/ui.py#L1671)).
- **Spawn a worker thread** that calls `fetch_tasks_snapshot(self.worktrees_backend_url)` and posts a tagged result onto `self.ingest_queue` — success and failure both, mirroring the `("dashboard_data", …)` / `("dashboard_data_error", …)` shape in [`_start_activation_load`](../../app/codex_dashboard/ui.py#L2281). The tag carries enough to find the still-open popup (e.g. the popup reference / a per-open token) so a stale result for a closed popup is dropped.
- **Add a branch in [`_poll_ingest_results`](../../app/codex_dashboard/ui.py#L2244)** for that tag: when the result arrives, if the popup is still open, compute `options = open_task_options(tasks_snapshot)` and populate the list (the existing `render()` / `_build_assign_task_card` logic and the `first_assignable_task_id` default selection — i.e. the body that today runs from [L1594](../../app/codex_dashboard/ui.py#L1594) and [L1671–1778](../../app/codex_dashboard/ui.py#L1671) onward), replacing the loading label. On the error tag, show a clear human-facing message in the open popup (equivalent to today's `self._set_worktrees_status(f"Could not load open tasks: {exc}")` at [L1592](../../app/codex_dashboard/ui.py#L1592), surfaced so the operator sees why the list is empty).
- The selection state (`selection = tk.StringVar(...)`), filter/sort toolbar, and the **ASSIGN** binding to [`_confirm_assign`](../../app/codex_dashboard/ui.py#L1983) are unchanged; only *when* the list is populated changes (after the async result, not before the popup is drawn).

Keep the change surgical: same widget tree, same styles, same popup geometry/centering, same Escape/close/CANCEL, same scroll behavior. The diff should read as "the fetch and list-population moved from synchronous-before-draw to async-after-draw," nothing more.

### 2. The lifecycle test exercise (no product code; a documented procedure + captured proof)

This is the human's own acceptance exercise, run through the WORKTREES tab on the isolated lane. It uses only the existing controls (Assign, the card's issue link, Eject) and the existing operator close path. The concrete trigger and steps are in [Proof Plan](#proof-plan).

## Expected Resolution

After this task:

- Clicking **ASSIGN** on a worktree card opens the Assign popup **right away** — the operator sees the popup window (cyan border, header naming the target worktree, ASSIGN/CANCEL footer) with a brief "Loading open tasks…" state, and the open-task list fills in a moment later. The overlay never freezes on the ASSIGN click. The popup is still non-modal, Escape-closes, and centers on the overlay; ASSIGN/CANCEL stay visible and the list scrolls (REG-019 behavior preserved).
- The operator can run one simple end-to-end exercise on the isolated lane and *see it*: Assign a task → the worktree flips to **allocated** with the bound issue link on its card → the bound issue is CLOSED by the human (the chosen "done" trigger) → the worktree returns to **idle** in the WORKTREES view, with no bound task and no agent having self-closed anything.

## Acceptance Criteria

Each criterion is pass/fail.

**Part 1 — ASSIGN no longer blocks the UI thread:**

1. After the change, [`open_assign_popup`](../../app/codex_dashboard/ui.py#L1586) contains **no** call to `fetch_tasks_snapshot(...)` (nor any other blocking HTTP/IO call) on its synchronous path before the `tk.Toplevel` popup is created. (Grep/read the method: the fetch is gone from the top; it now runs only inside a `threading.Thread` worker.) — **falsifier:** the method still calls `fetch_tasks_snapshot` before building the popup.
2. The Assign popup **window appears immediately** on the ASSIGN click — within a small bound (e.g. it is visible well under ~100 ms, independent of backend latency), while the task list shows a loading state, then fills in when the worker result arrives. Demonstrated by a deliberately slow backend: with the open-task read artificially delayed (e.g. a stub/slow lane adding ~1 s to `GET /api/v1/tasks`), the popup still opens immediately and the overlay stays responsive (Escape closes it during the load) — **falsifier:** the overlay freezes / the popup does not draw until the read returns.
3. The popup remains **non-modal** with **Escape-to-close** and the close X / CANCEL working; no `grab_set()` is present. (REG-019 layout — buttons always visible, list scrolls, centered — still holds.) — **falsifier:** a modal grab is reintroduced, or Escape/close stops working, or the buttons/scroll regress.
4. When the open-task list populates after the async load, selecting a task and clicking **ASSIGN** still binds it via [`_confirm_assign`](../../app/codex_dashboard/ui.py#L1983) → `assign_worktree(...)` exactly as before (the assign behavior is unchanged; only list population timing changed). — **falsifier:** assigning a task from the popup no longer works, or binds the wrong task.
5. If the background open-task fetch **fails**, the open popup shows a clear human-facing message (equivalent to today's "Could not load open tasks: …") rather than crashing or hanging. — **falsifier:** a fetch error crashes the app, leaves a spinner forever, or shows no message.

**Part 2 — visible bind → done → clear (observed in the WORKTREES tab on the isolated lane):**

6. **Bind:** Assigning a throwaway testbed task to an idle worktree flips that worktree to **allocated** in the WORKTREES view (allocated background, distinct from idle — [`worktree_status_background`](../../app/codex_dashboard/worktrees_tab.py#L37)), bound to that task, with the task's clickable GitHub-issue link on its card ([ui.py L1340–1352](../../app/codex_dashboard/ui.py#L1340)). — **falsifier:** the worktree does not flip to allocated, or no bound issue link appears.
7. **Done → clear (default trigger):** After the human CLOSES the bound issue (operator close path), the worktree **returns to idle** in the WORKTREES view (idle background, no bound task) on the next pool read — i.e. closure deallocates exactly as Task-0016 / the done-contract specify. — **falsifier:** the worktree stays allocated after the issue is closed, or it clears without the issue being closed (which would mean an autonomous deallocation crept in).
8. **Human-only closure preserved:** No agent self-close occurred at any point in the exercise; the worktree did **not** deallocate on `Human Needed=Yes` (park-in-place) — only on the human CLOSE. (If the alternative Eject trigger is used instead, the equivalent check is: Eject returned the worktree to idle, terminated the run, set the freed task `Queue=Never`, and left the issue **open**.) — **falsifier:** the agent closed its own issue, or a park (`Human Needed=Yes`) cleared the worktree.

## What Does Not Count

- **Speeding up the fetch instead of moving it.** Shortening the HTTP timeout, caching the task snapshot, or otherwise making the synchronous call *faster* while it still runs on the UI thread before the popup is drawn does **not** satisfy part 1. The popup must appear without waiting on the network.
- **A backend-only or unit-only proof for part 2.** A `GET /api/v1/worktrees` JSON showing idle, or a passing `worktrees_tab.py` unit test, does **not** satisfy the "visible bind→clear" exercise. The allocated→idle transition must be observed in the running WORKTREES tab.
- **Reintroducing a modal grab.** Any `grab_set()` (or other modal capture) on the popup fails — it reintroduces the frozen-overlay bug the [ui.py L1618–1625](../../app/codex_dashboard/ui.py#L1618) comment records.
- **A new autonomous deallocation path.** Anything that clears the worktree without a human close (default trigger) or operator Eject (alternative) — e.g. an agent self-close, or auto-deallocation on park — fails part 2 and violates a Non-Goal.
- **Running any of this against production.** Proof produced against `:4318`, the real `Digital-Collective-Games/Obsidian` repo, the live CodexDashboard, or the real `default` Temporal namespace does **not** count (see Proof Plan).
- **A global async refactor.** Moving other screens' fetches off-thread "while we're here" is out of scope; only the Assign-popup open path may change.

## Proof Plan

**Lane discipline (mandatory).** ALL testing runs on the **isolated validation lane** — backend `http://127.0.0.1:14318` (Temporal `127.0.0.1:17233`, Postgres `15432`, runtime root `%LOCALAPPDATA%\CodexDashboard\orchestration-validation-lane`), with the app pointed at it via the `CODEX_DASHBOARD_*_BACKEND_URL` override, against a **throwaway testbed** task/repo. NEVER the live service lane (`:4318`), NEVER the real `Digital-Collective-Games/Obsidian` repo as a test target, NEVER the live CodexDashboard, NEVER the real `default` Temporal namespace. (Per [REGRESSION.md](../../REGRESSION.md) Canonical Rule + [TESTING.md](../../TESTING.md).)

**Part 1 — ASSIGN responsiveness (the new behavior):**

1. Launch the real app on the isolated lane (task-owned config + SQLite per [TESTING.md](../../TESTING.md)); point worktrees/tasks readback at `http://127.0.0.1:14318`.
2. Open the WORKTREES tab with at least one idle worktree and at least one open task on the lane.
3. **Slow-read check:** introduce a deliberate delay on the lane's `GET /api/v1/tasks` (a slow stub / proxy adding ~1 s). Click **ASSIGN** on an idle worktree and confirm the popup **window appears immediately** (with a "Loading open tasks…" state) while the overlay stays responsive (Escape closes the popup mid-load). Then confirm the list **fills in** once the read returns and a task can be selected + bound. Capture an app-surface artifact of the immediate popup with the loading state, and of the populated list afterward.
4. **Code check:** confirm [`open_assign_popup`](../../app/codex_dashboard/ui.py#L1586) no longer calls `fetch_tasks_snapshot` before building the popup (the fetch is in a worker thread; population is via the [`_poll_ingest_results`](../../app/codex_dashboard/ui.py#L2244) drain).
5. **Error check:** point the lane's `/api/v1/tasks` at a failing response and confirm the open popup shows a clear human-facing message, no crash.

This part overlaps the existing [REG-014](../../REGRESSION.md) (Assign popup binds an open task) and [REG-019](../../REGRESSION.md) (Assign popup layout: buttons visible, list scrolls, centered) human-surface cases — those cases must still pass after the change (the popup must still bind and must still keep its layout/scroll/centering). Cite REG-014 and REG-019 as the existing in-app cases this change must not regress; do not redefine them.

**Part 2 — bind → done → clear (the lifecycle exercise):**

1. On the isolated lane with a throwaway testbed task whose issue is on the lane's task provider (never a production issue), open the WORKTREES tab with at least one idle worktree.
2. **Bind:** Click **ASSIGN**, select the testbed task, confirm. Verify the worktree flips to **allocated** with the bound task's GitHub-issue link on its card. Capture the allocated card.
3. **Done (default trigger):** The **human CLOSES the bound issue** via the operator close path (the obsidian-operator human-gated close, e.g. via `Reconcile-TaskGitHubState.ps1`). This is a human action — no agent self-close.
4. **Clear:** Refresh the pool view (or let discover/reconcile run) and verify the worktree **returns to idle** (idle background, no bound task). Capture the idle card.
5. Confirm: the worktree did **not** clear on a mere park (`Human Needed=Yes`) and only cleared on the human CLOSE; no agent self-close occurred.
6. **(Alternative trigger, if the human prefers the faster demo):** instead of closing the issue, click **Eject** on the allocated worktree and verify it returns to idle, the run is terminated, the task is dequeued (`Queue=Never`), and the issue stays **open** — this exercises the existing [REG-015](../../REGRESSION.md) Eject behavior. If used, record that the demonstration used Eject (operator-cleared), not issue closure (task-done).

Part 2's bind/clear behavior is the existing [REG-014](../../REGRESSION.md) (bind) and the close/deallocate sub-scenario of [REG-007](../../REGRESSION.md) (only a closed issue deallocates) / [REG-015](../../REGRESSION.md) (Eject returns idle and dequeues). Cite those case ids; this task does not redefine what counts as a regression lane.

## Open Questions

1. **Which "done" trigger does the human want for the simple exercise?** This draft defaults to **issue closure** (the human-only-closure path — closure is what "task is done" means in this system) and records **Eject** as the faster operator-initiated alternative. If the human wants the quicker demonstration, use Eject and label it "operator-cleared" rather than "task-done." Either choice is honest; it changes only the part-2 procedure (Acceptance criteria 7/8 and Proof Plan step 3/6), not the writeup type, the home, or part 1. No other blocking ambiguity is open.

## References

- Async pattern to mirror: [`_start_activation_load` / `schedule_ingest`](../../app/codex_dashboard/ui.py#L2281), [`_poll_ingest_results`](../../app/codex_dashboard/ui.py#L2244).
- The stall: [`open_assign_popup`](../../app/codex_dashboard/ui.py#L1586) calling [`fetch_tasks_snapshot`](../../app/codex_dashboard/tasks_backend.py#L29) at [ui.py L1590](../../app/codex_dashboard/ui.py#L1590); the no-`grab_set()` note at [ui.py L1618–1625](../../app/codex_dashboard/ui.py#L1618).
- Bind / clear actions: [`_confirm_assign`](../../app/codex_dashboard/ui.py#L1983) → [`assign_worktree`](../../app/codex_dashboard/worktrees_backend.py#L51); [`eject_worktree_action`](../../app/codex_dashboard/ui.py#L1551) → [`eject_worktree`](../../app/codex_dashboard/worktrees_backend.py#L64); allocated card issue link [ui.py L1340–1352](../../app/codex_dashboard/ui.py#L1340) / [`worktree_issue_url`](../../app/codex_dashboard/worktrees_tab.py#L125); allocated/idle render [`worktree_status_background`](../../app/codex_dashboard/worktrees_tab.py#L37).
- Lifecycle contract: [Tracking/Task-0016/TASK.md](../Task-0016/TASK.md); done-contract [skills/obsidian-operator/SKILL.md](../../skills/obsidian-operator/SKILL.md) (Queue Done-Contract: agent never self-closes; only a closed issue deallocates; `Human Needed=Yes` parks).
- Regression cases (existing; not redefined here): [REG-014](../../REGRESSION.md), [REG-015](../../REGRESSION.md), [REG-019](../../REGRESSION.md), and the close/deallocate sub-scenario of [REG-007](../../REGRESSION.md).
- Lane: [REGRESSION.md](../../REGRESSION.md) Canonical Rule; [TESTING.md](../../TESTING.md) (isolated validation lane `http://127.0.0.1:14318`).
