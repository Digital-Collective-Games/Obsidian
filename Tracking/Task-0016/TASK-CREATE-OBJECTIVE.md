# Task-0016 — TaskCreate Objective

## What this task is

Redesign the **TASKS tab** of the CodexDashboard desktop app into a **Worktree
Management** surface, and add the backend endpoints that make the backend
authoritative over worktree assignment.

The desktop app is a **Python / Tkinter** app at
`C:\Agent\CodexDashboard\app\codex_dashboard\`. The new TASKS tab must be
re-implemented in that existing Tk app, reusing the styling patterns already in
`ui.py`. A Google Stitch HTML mockup exists and is to be used as a **structural
guide only** (layout, affordances, the dark cyan/navy palette), not a literal
HTML port and not a stack migration.

## Human-perceived outcome (state this first in TASK.md)

An operator opens the TASKS tab and sees, in one view, **every worktree the
backend knows about across all registered repos** — each one clearly marked as
**allocated** (bound to a task, agent running/parked) or **idle** (available).
For each worktree they see the repo, the local checkout directory (with a
one-click copy-path control), and — for allocated ones — which task it is bound
to, visually distinguished by background color. They can **filter by repo** using
a dropdown sourced from the repo registry. They can **Eject** an allocated
worktree (stop the agent, clean the checkout, return the slot to the idle pool).
They can **Assign Task** to an idle worktree via a popup that lists open tasks;
selecting one binds it to that worktree and launches work there.

The existing per-task stream / detail / actions view (dispatch / pause / poke /
retry) is **replaced** by this worktree-management view. The human has decided
that the task lifecycle now lives on the **GitHub Issues** surface (queue-drain
consumer), so the dashboard's TASKS tab is freed to be worktree management.

## Writeup type

Concrete implementation task. The solution shape (Tk redesign of the existing
tab + new authoritative backend endpoints) is already chosen by the human. The
task must name the concrete files, endpoints, request/response shapes, and the
exact UI affordances, so two implementers could not build materially different
things and both claim done.

## Bundled mechanisms (must be earned in TASK.md)

This task intentionally combines a frontend redesign and several new backend
endpoints. They belong together because the redesigned tab is non-functional
without the endpoints it consumes, and the endpoints have no other consumer.
TASK.md must keep them internally separable for implementation and proof:
the backend endpoints are independently unit-testable; the Tk tab is
independently renderable against those endpoints.

## What "done" must prove

- The TASKS tab renders the worktree list (allocated + idle), repo filter,
  copy-path, color distinction, Eject, and Assign-Task popup, driven by real
  backend data.
- The new backend endpoints exist with named routes and shapes, are unit-tested,
  and make the backend authoritative over worktree assignment and ejection.
- The old task-stream/detail/actions tab content is removed, not left dead.

## Authoritative inputs

- `Tracking/Task-0016/HUMAN-DIRECTIVES-FOR-WORKER.md` — the human's verbatim spec
  and every approved decision (Q1, Q2, D1, exclusions E1–E7).
- `Tracking/Task-0016/TASK-CREATE-CONTEXT-MANIFEST.md` — the durable files to read.
