# Task-0016 — Human Directives (authoritative)

These are the human's explicit decisions for this task. They outrank any
writeup-standard preference. Do not narrow or re-sequence the scope below.

## UPDATE 2026-05-31 — DESIGN PIVOT (authoritative; supersedes the capacity model and E1 below)

After the first draft, the human pivoted the model and split the scope. The
following is now governing; where it conflicts with anything later in this file
(the "capacity model", "no idle directories", or exclusion **E1**), THIS section
wins.

**Scope split — Task-0016 is now BACKEND-ONLY.** Q1 = a (split). Task-0016 is the
**manual worktree-pool lifecycle** in the Go backend, proven green against
REG-007/008/009. The Tkinter UI redesign (the original frontend work, D1=replace)
moves to a **future Task-0017** — do NOT draft Task-0017 and do NOT touch
`app/codex_dashboard/*` in Task-0016. Task-0016 may add the HTTP endpoints that
expose the lifecycle (so it is drivable/testable headlessly and the future UI has
a seam), but no Tk code.

**Manual persistent worktree pool (replaces the capacity model).** The human
confirmed that today the backend does NOT pre-create worktrees and `queue_workers`
is only a runtime concurrency cap. New model:

- **Remove `queue_workers`** from `REPO-MANIFEST.json` (`RepoEntry`) and from the
  queue-drain consumer admission logic (`internal/queue/decision.go`
  effectiveSlotLimit / `consumer.go` SlotSizer). Concurrency is now bounded
  **by the number of idle worktrees in the pool, by construction** — no separate
  cap. (REG-007 "cap=1" becomes "a pool of 1": the second Ready issue waits
  because there is no idle worktree.)
- **Worktrees are a manually-managed, persistent pool per repo.** Idle worktrees
  are real folders on disk with stable paths and stable identifiers (NOT the
  current per-dispatch random temp dirs). The pivot REVERSES E1: **manual worktree
  creation is now IN scope.**
- **Discover-on-startup, not create-to-a-number.** Replace/extend the prune-only
  `ReconcileOwnedLanes()` so startup ENUMERATES the worktrees that exist on disk
  for the repo and reconstructs each one's allocated (bound to a live run) vs idle
  state from durable records / the live workflow. No auto-seeding to a count.
- **Lifecycle actions (Q2, confirmed):**
  - **Create** — provision one new IDLE worktree into the pool (stable path/id;
    persists; no task). This is the manual allocation the human wants.
  - **Assign** — bind a chosen open task onto a chosen IDLE pool worktree: reset
    the checkout to baseline, then bind + start the run **in that existing
    worktree** (do NOT provision a fresh dir). Both the manual Assign endpoint AND
    the queue-drain consumer use this pool-draw path.
  - **Eject** — stop the launched agent + clean the checkout back to baseline
    (`git reset --hard` / `git clean -fdx`) + unbind/terminate the run, but **KEEP
    the folder and return it to idle**. Must NOT delete the folder. Works
    regardless of parked state (operator's explicit "give me this slot back").
  - **Destroy** — remove an IDLE worktree from the pool (the current
    `removeOwnedLaneWorktree` delete mechanics). Reject if the worktree is
    allocated (operator must Eject first).
- **Dispatch path change.** The current "provision a fresh temp worktree per
  dispatch" ([provisionOwnedLane](../../backend/orchestration/internal/taskrun/service.go#L1507))
  is replaced by **pool-draw** (pick an idle pool worktree → reset → bind →
  start). Creating a worktree happens ONLY via the manual **Create** action. With
  the cap gone, the queue-drain consumer can only dispatch into idle pool
  worktrees; an **empty pool ⇒ Ready issues wait** (no auto-create). The human
  explicitly accepts this operator-owned-capacity behavior.

**Concreteness the writer must pin** (without narrowing scope):
- the stable worktree **identity + durable pool record** (worktree_id ↔ path ↔
  repo ↔ current run_id-or-null), and how **discover** reconstructs idle vs
  allocated across a backend restart (must keep REG-008 durable-state survival);
- that **Eject keeps the folder** (clean-and-return) while **Destroy** is the only
  delete, and Destroy rejects an allocated worktree;
- that the **queue-drain consumer draws from the same shared pool** (REG-009
  cross-repo isolation preserved: each repo's pool is independent).

Everything below remains true EXCEPT: the scope is now backend-only (the Tk UI is
Task-0017), the model is the manual pool above (not the capacity model), and E1 is
reversed (manual Create/Destroy are in scope). E2–E7 still stand.

---

## Verbatim original request

> I'd like to redo the UI for the desktop app, TASKS tab.
>
> - Shows all worktrees (allocated or not) and a display which shows if they're
>   allocated to a particular task or not. Shows repo, plus local dir (copy icon
>   to copy local directory path).
> - Allocated worktrees show up with different background color to distinguish
>   them.
> - Has a filter for repo, drop-down should use repo registry to select repo
>   filter for worktrees.
> - Has an eject button which will stop the agent, clean up the worktree, and
>   return it to the pool.
> - Has an assign task popup which queries the open tasks, and if selected then
>   binds the task to that worktree.
>
> See mockup here: `C:\Users\gregs\Downloads\stitch_codex_token_velocity_overlay (4)`

These five bullets are the authoritative functional spec.

## Decisions made with the human

- **Q1 (stack) = a.** Re-implement the TASKS tab in the **existing Python /
  Tkinter app** (`app/codex_dashboard/`). Use the Stitch HTML mockup as a
  **structural guide only**. "A lot of the style is already captured in existing
  patterns" — reuse the existing `ttk` styles and the dark cyan/navy palette in
  `ui.py`; do not port Tailwind tokens literally; this is **not** a migration to
  a web frontend.
- Human directive, verbatim: *"dont implement anything that doesn't make sense
  like add worktree, just call out any exclusions you're making to stitch."*
  The exclusions below were called out and approved.
- **Q2 (backend) = yes, new endpoints.** Verbatim: *"backend needs a concept of
  worktrees since its authoritative over their assignment, yes this is new
  endpoints."* The backend is the authority over worktree allocation, assignment,
  and ejection. The frontend reads/acts through new HTTP endpoints.
- **D1 (existing task view) = a (replace).** The worktree-management view
  **replaces** the current task-stream / detail / actions content of the TASKS
  tab. Verbatim justification: *"now we have github issues as a surface, that's
  much more appropriate"* — the task lifecycle (dispatch/park/close) now lives on
  the GitHub Issues queue surface (the queue-drain consumer), so the dashboard's
  TASKS tab is freed to become worktree management. Removing the old
  dispatch/pause/poke UI from this tab is intended, not a regression.

## Worktree model (confirmed with the human)

The backend is authoritative. A worktree **slot** belongs to a repo (registry
`queue_workers` capacity). **Allocated** = bound to a task with a running or
parked agent. **Idle** = a slot with no task. **Assign Task** binds a chosen
open task onto a chosen idle worktree (provision checkout + launch agent).
**Eject** stops the agent, cleans the checkout, and returns the slot to the idle
pool (the slot is not destroyed; it becomes idle and re-assignable).

## Approved exclusions from the Stitch mockup (do NOT implement)

- **E1 — "Spawn Worktree" / add-worktree button.** Slots come from registry
  capacity; Eject returns a slot to idle. Ad-hoc spawn is meaningless here.
- **E2 — Drag-to-bind interaction, the persistent left "Task Browser" pane, and
  the "Drag task to worktree to bind" banner.** Replaced by the explicit
  **Assign Task popup** the human specified.
- **E3 — "Register New Task" button.** Task creation is a separate workflow
  (TaskCreate / obsidian-operator), not this tab.
- **E4 — Agent-model chip** ("Claude-3.5-Sonnet" / "Codex-Agent-09"). The backend
  binding has no model name (only session id / transcript path / launched PID).
  Excluded to avoid fabricated data. (Permitted cheap substitute if trivially
  available: the launch-agent kind, e.g. "claude". Otherwise omit.)
- **E5 — Animated transitional states** ("Initializing Claude Agent…",
  "Binding…", "Release to Bind…", pulsing drop-zones). Drag-only; a static status
  chip (running / parked / idle) is sufficient.
- **E6 — Per-task progress bars and file-ref metadata lines in task cards.**
  Decorative; the backend has no per-task progress metric. The Assign popup lists
  task id + title + state instead.
- **E7 — Top-nav chrome changes** (search box, a "Review" tab, settings /
  terminal / notifications icons). Global chrome, outside a TASKS-tab redesign.

## Open implementation detail for the writer to pin

The Assign-Task popup "queries the open tasks." Given GitHub Issues is now the
task surface, the writer must pin **which source** feeds that popup and state it
concretely: either the existing `GET /api/v1/tasks` (local committed tasks bound
to issues) or an open-GitHub-issues-derived list. Choose one, name the endpoint,
and make it consistent with "GitHub Issues is the task surface." This is a
concreteness decision, not a scope change.

## Style note

The mockup palette/typography (Space Grotesk display, Inter body, #0a0e14 /
#1c2026 surfaces, #00e5ff / #c3f5ff cyan accents, 0px radius, no dividers) mostly
already exists in the app's `ttk` styles. Match the existing app look; use the
mockup only to decide layout and which controls appear.

## UPDATE 2 — 2026-05-31 — Eject dequeues through the task provider (authoritative; in Task-0016 scope)

Decision on the Eject behavioral consequence. The human confirmed: a freed task
whose issue is still `Queue=Ready` would otherwise be **re-dispatched** by the
consumer ("throw itself back in"). The fix is that **Eject must take the task out
of the queue, and that dequeue must go through the task provider** — not internal
state. New scope for Task-0016 (the provider is read-only today; this adds a
provider WRITE):

- Add a **task-provider "dequeue" write** capability through the `TaskProvider`
  abstraction: set the task's provider queue state to **not-ready**. For the
  `github_issues` provider this means setting the issue's **Queue single-select to
  `Never`** (the same field the consumer polls for `Ready`). Implement it on the
  provider, not as a one-off hardcoded GitHub call inside Eject.
- **Eject now dequeues:** after stopping the agent + cleaning the checkout +
  returning the worktree to idle, Eject calls the provider dequeue so the freed
  task is **not** re-dispatched on the next poll. Make it idempotent (already
  not-ready ⇒ no-op) and a safe no-op for a worktree with no provider-backed task.
- Expose a standalone **dequeue** operation too (so the operator can dequeue
  without ejecting, and Task-0017's UI has the seam). The writer pins the exact
  route.

**Hard constraint — dequeue is NOT close.** Dequeue only sets `Queue=Never`; the
issue stays **open**. This preserves human-only closure (the agent never
self-closes; only a human-closed issue deallocates). Eject/dequeue are
operator-initiated, so they are human actions, not autonomous self-close. To
**restart** a dequeued task the operator sets `Queue=Ready` again (the GitHub
surface).

**Testing nuance to record (not a conflict):** the product's dequeue is a
backend-owned provider WRITE (`Queue → Never`). It is distinct from the REG-007
*testing* rule that the **Ready** flip must be driven at the real GitHub web UI
(that rule governs how the regression sets `Ready` to prove the real surface; it
does not forbid the product from writing `Queue=Never` on an operator dequeue).
