# Task-0017 — TaskCreate Objective (worker-safe)

This is the worker-safe objective for the blind TaskCreate writer-worker. It
contains the human's request verbatim, the concrete code grounding the
coordinator already located, and the hard constraints. It contains no hidden
coordinator reasoning and no expected audit findings.

## Human request (verbatim)

> There's a delay when i cliuck ASSIGN on a worktree - can you TaskCreate that
> and send me the gh link when done. Its maybe 500 ms delay, i guess its a
> synchronous call. But we should fix that for just this screen. As a test of
> binding a task to a worktree, just want to try something simple like that, and
> see the worktree cleared when task is done. I'll do it, just create the task
> pls.

The human will implement this task themselves. The deliverable of THIS run is a
created, provider-bound task (GitHub issue + `TASK.md` + `TASK-META.json`), not
implementation.

## What the human is asking for (two parts — keep both; do not narrow)

1. **Fix the perceptible delay when clicking ASSIGN on a worktree, scoped to that
   one screen.** When the operator clicks ASSIGN on a worktree card in the
   WORKTREES tab, the whole overlay visibly stalls (~500 ms) before the Assign
   popup appears. The cause is a synchronous, blocking HTTP fetch on the Tk UI
   thread. The fix is scoped to the Assign-popup open path only — not a global
   refactor of every fetch in the app.

2. **A simple end-to-end test of the bind→done→clear lifecycle.** The human wants
   to try something simple: bind a task to a worktree, then see the worktree
   cleared (returned to idle) when the task is done. This is the human's own
   acceptance exercise; the task should pin down exactly what "done" means here
   and give a concrete, runnable procedure on the isolated test lane.

## Concrete code grounding (already located by the coordinator)

The delay (part 1):

- The Assign popup is opened by `DashboardApp.open_assign_popup` in
  `app/codex_dashboard/ui.py` (around line 1586).
- The first thing it does, **synchronously on the Tk UI thread**, is
  `tasks_snapshot = fetch_tasks_snapshot(self.worktrees_backend_url)`
  (`app/codex_dashboard/ui.py` ~line 1590) before any popup window is built.
  `fetch_tasks_snapshot` is an HTTP call defined in
  `app/codex_dashboard/tasks_backend.py` (line 29). That blocking call on the UI
  thread is the ~500 ms stall: nothing is drawn until it returns.
- The app already has an established **non-blocking** pattern: run the network/IO
  work on a `threading.Thread` worker, post the result onto `self.ingest_queue`,
  and apply it back on the UI thread via the `_poll_ingest_results` /
  `self.root.after(...)` drain. See `_start_activation_load` /`schedule_ingest`
  (`app/codex_dashboard/ui.py` ~lines 2290–2331) for the worker→queue→after
  pattern, and the ingest-queue poll wiring (`self.root.after(100, self._poll_ingest_results)`).
- The popup must NOT use `grab_set()` (a prior bug: a modal grab on this
  borderless topmost popup froze every overlay click — see the comment at
  `app/codex_dashboard/ui.py` ~lines 1618–1625). Preserve the non-modal +
  Escape-to-close behavior.

The bind / clear lifecycle (part 2):

- Binding: `_confirm_assign` (`app/codex_dashboard/ui.py` ~line 1983) calls
  `assign_worktree(task_id, repo, worktree_id, self.worktrees_backend_url)`
  (`app/codex_dashboard/worktrees_backend.py` line 51). After assign the worktree
  shows `allocated` with the bound task's GitHub-issue link on its card.
- Clearing: `eject_worktree_action` (`app/codex_dashboard/ui.py` ~line 1551) calls
  `eject_worktree(...)` (`app/codex_dashboard/worktrees_backend.py` line 64),
  which returns the folder to `idle`.
- Important lifecycle truth from Task-0016 / the `obsidian-operator` skill: the
  agent NEVER self-closes; **only a human-CLOSED issue deallocates** a worktree
  (the consumer reclaims it). `Human Needed=Yes` parks in place (worktree
  retained). Eject is the operator-initiated clear that returns a folder to idle
  without deleting it. So "the worktree cleared when the task is done" has a
  precise meaning the task must state: either (a) the operator closes the bound
  issue (the human-only closure path), after which discover/reconcile returns the
  worktree to idle, or (b) the operator Ejects. The writer must pick the simplest
  honest "done" trigger for the simple test and name it explicitly, and note the
  alternative.

The backend worktree lifecycle (Create / Assign / Eject / Destroy / Dequeue +
discover-on-startup, and the `Eject dequeues via the provider` write) was built
in Task-0016 (issue #16) — read `Tracking/Task-0016/TASK.md` for the endpoint
contract. Part 2 is a TEST of that existing lifecycle through the human surface,
not new backend lifecycle work.

## Hard constraints (do not violate)

- **Scope part 1 to the Assign-popup open path only.** Do not globally refactor
  other screens' fetches. The acceptance must prove ASSIGN no longer blocks the
  UI thread, and must not require touching unrelated screens.
- **Human-only closure is preserved.** Nothing in this task may add an
  agent-side self-close or a new autonomous deallocation path. If the simple test
  uses issue-closure as the "done" trigger, that close is a human action.
- **All testing is on the isolated validation lane / a throwaway testbed —
  never production.** Never the live human service lane (`:4318`), never the real
  `Digital-Collective-Games/Obsidian` repo as a test target, never the live
  CodexDashboard, never the real `default` Temporal namespace. The isolated lane
  binds at `http://127.0.0.1:14318`. The proof plan must state the isolated lane.
- **Simplicity first / surgical changes.** Minimum code that removes the stall.
  No speculative async framework. Match the app's existing worker→queue→after
  pattern and existing button/popup style. Touch only the Assign-popup open path
  and whatever the simple test needs.
- **Do not broaden** into a general async-everything refactor, a worktree-pool
  redesign, or new backend endpoints. The Task-0016 backend lifecycle is done;
  this is a UI responsiveness fix + a simple lifecycle test.

## Writeup type

Default to **concrete implementation** (the main mechanism — moving the Assign
fetch off the UI thread — is already chosen and the files are named). If you find
that part 2's "done" trigger is genuinely undecided in a way that changes the
acceptance bar, surface that as an explicit Open Question rather than guessing;
do not downgrade the whole task to research without saying why.

## Provider binding (coordinator handles this — informational only)

- Task id / dir: **Task-0017**, `Tracking/Task-0017/`. The matching GitHub issue
  is **#17** in `Digital-Collective-Games/Obsidian` (issue-first binding gate;
  local task number must equal the issue number).
- The coordinator creates the issue + `TASK-META.json` after reviewing your
  draft, via the `obsidian-operator` scripts. You only write
  `Tracking/Task-0017/TASK.md` (and optionally a short writer note).
- The issue will be created **without** `Queue=Ready` (the human implements it
  manually; it must not be auto-dispatched by the queue-drain consumer).

## Required output

Write `Tracking/Task-0017/TASK.md` per `TASK-CREATE.md`. Include the base
sections plus `Proposed Changes`, `Expected Resolution` (this touches a
human-facing surface), `What Does Not Count`, and a `Proof Plan` that names the
isolated lane and the bind→done→clear procedure. Optionally write a short writer
note. Do not write an audit verdict.
