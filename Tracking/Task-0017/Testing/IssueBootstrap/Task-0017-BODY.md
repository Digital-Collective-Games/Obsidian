<!-- task-sync: repo=CodexDashboard; task_id=Task-0017; task_path=Tracking/Task-0017/TASK.md -->

# Task-0017: Make ASSIGN feel instant on the WORKTREES tab — clicking ASSIGN opens the popup immediately (no ~500 ms freeze) while the open-task list loads in the background — and prove the bind→done→clear lifecycle: a worktree assigned to a task visibly returns to idle once that task is done.

## Source Of Truth

Local `Tracking/Task-0017/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0017:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

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

## Non-Goals

- **No global async refactor.** Only the Assign-popup open path changes. Other screens' fetches (the Usage source filter, the Jobs tab, the pool read on tab activation, the repo-filter read) are **not** touched. This task does not introduce an async framework or a general "every fetch off-thread" pattern; it reuses the existing worker→queue→after seam for this one path only.
- **No new backend lifecycle.** The Task-0016 backend (Create / Assign / Eject / Destroy / Dequeue + discover-on-startup, and the Eject-dequeue write) is done and is **not** modified. Part 2 is a *test* of that existing lifecycle, not new endpoints.
- **No change to human-only closure.** This task adds **no** agent self-close and **no** new autonomous deallocation path. The "done" trigger is a human action (issue close) or an operator action (Eject); the consumer's reclaim semantics ([`decision.go`](../../backend/orchestration/internal/queue/decision.go): only a closed issue deallocates; `Human Needed=Yes` parks) are unchanged.
- **No backend port / lane change.** The default backend bind stays `http://127.0.0.1:4318`; the test runs on the isolated validation lane override `http://127.0.0.1:14318` (per [TESTING.md](../../TESTING.md) / [REGRESSION.md](../../REGRESSION.md)).
- **No popup redesign.** No new filter/sort behavior, no new task-card fields, no modal grab. The only visible change is that the popup appears immediately with a momentary loading state before the list fills in.

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `17`
- Local task path: `Tracking/Task-0017/TASK.md`
- Source commit: `e2a4a82911dfb8a99458844c490787ded3c66754`
- Local task SHA-256: `91DD388E9F5815230305DD12BD40E30D414909706A4981EA3DB3E0826F3347C8`
- Rendered at: `2026-06-02T02:40:28.9086057-04:00`