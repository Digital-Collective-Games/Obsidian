# Task-0016 — Plan Approval Review Package

- **Gate:** PLAN approval (planning → implementation)
- **Task:** Task-0016 — Manual persistent worktree pool (Go backend) **+ the desktop
  `WORKTREES` tab that drives it** (one testable chunk; UPDATE 3 scope reversal)
- **Status:** awaiting explicit human approval; **no implementation has started**

## Exact approval question

> Approve [PLAN.md](../../PLAN.md) (PASS-0000 → PASS-0009) as safe and specific enough to
> execute — i.e. begin implementation of the manual worktree-pool model swap **AND** the
> desktop `WORKTREES`-tab UI that consumes it, as sequenced (backend endpoints first, then
> the UI, then the in-app regression run), with all proof on the isolated validation /
> `reg007` lanes only, and with "done" gated on the **working in-app `WORKTREES` surface**
> plus the named in-app cases (REG-010…REG-016, and REG-007/008/009 green under the new
> model)?

Answer terse: **approve**, **approve with changes** (say which pass/section), or
**reject** (say why).

## What changed since the prior (backend-only) review package

The human's **UPDATE 3** ([HUMAN-DIRECTIVES-FOR-WORKER.md](../../HUMAN-DIRECTIVES-FOR-WORKER.md))
**reversed** the earlier backend-only split: the Tk UI is **back in scope** and "done"
requires the **working in-app human surface**. The artifacts were revised accordingly:

- **[TASK.md](../../TASK.md)** — kept [BE] criteria 1–15; **added [FE] criteria 16–22**
  (falsifiable, in-app); removed the Task-0017 deferral / backend-only framing; reversed
  the Tk exclusion (E2–E7 still excluded, E1 reversed); made "done = the working human
  surface" explicit; renamed the title/scope to cover both halves.
- **[REGRESSION.md](../../REGRESSION.md)** — **added 7 new named in-app
  desktop-app-surface cases** (REG-010…REG-016), one per new `WORKTREES`-tab surface; added
  a "pool of 1" reinterpretation note to REG-007's cap=1 sub-scenario under the new model.
- **[PLAN.md](../../PLAN.md)** — re-sequenced into **nine passes** (backend
  PASS-0000…PASS-0006, UI PASS-0007/0008, in-app regression PASS-0009); the
  acceptance-criteria→pass map now covers all 22 criteria + every new/updated regression
  case; "What approval authorizes" updated to include the UI passes and the human-surface
  closure bar.

## What approval authorizes (and what it does NOT)

**Authorizes:** implementing PASS-0000 → PASS-0009 as scoped in [PLAN.md](../../PLAN.md)
— the Go backend manual worktree-pool lifecycle **and** the desktop `WORKTREES`-tab UI —
with all proof on the isolated validation (`http://127.0.0.1:14318`) / `reg007` lanes,
task-owned config + isolated SQLite, and throwaway testbed repos.

**Does NOT authorize:** any work against the human's production repo, service lane, live
Codex data, or human dashboard config/database; publishing/rolling out a new pinned
dashboard release to the human lane (a separate human-gated publish + restart); or closing
GitHub issue **#16** (closure is a separate human gate — the agent never self-closes).

## The plan, in one screen

One **model swap + its one human surface** (not a scope split) sequenced into 9 passes:

| Pass | What it builds | Key acceptance criteria |
| --- | --- | --- |
| [PASS-0000](../../PLAN.md) | Stable pool record + stable `worktree_id` (idle = `run_id` null) | AC2 foundation |
| [PASS-0001](../../PLAN.md) | Discover-on-startup (enumerate pool, reclassify idle/allocated) | AC8; REG-008 |
| [PASS-0002](../../PLAN.md) | Create + Destroy + full-pool/repos reads + route guards | AC2, 7, 9, 10, 11 |
| [PASS-0003](../../PLAN.md) | Assign (pool-draw reuse) + dispatch-path change + `queue_workers` removal | AC1, 3, 4, 5; REG-007 "pool of 1" |
| [PASS-0004](../../PLAN.md) | Provider dequeue write (`Queue→Never`) + standalone dequeue endpoint | AC12, 14, 15 |
| [PASS-0005](../../PLAN.md) | Eject (keep folder + return idle + dequeue) + no-bounce-back seam | AC6, 13, 14 |
| [PASS-0006](../../PLAN.md) | Backend cross-cut: full `go test ./...` + server-only smoke | backend AC green |
| [PASS-0007](../../PLAN.md) | **[FE]** Rename `TASKS`→`WORKTREES` + replace; read-only pool view (allocated/idle color), copy-path, registry repo filter | AC16–19; REG-010/011/012 |
| [PASS-0008](../../PLAN.md) | **[FE]** Create / Assign popup→bind / Eject / Destroy / Dequeue controls | AC20–22; REG-013/014/015/016 |
| [PASS-0009](../../PLAN.md) | Closure: in-app run of REG-010…REG-016 + REG-007/008/009 re-run under the new model | all in-app cases pass |

Full pass detail, dependency rationale, and the complete
acceptance-criteria→pass map are in [PLAN.md](../../PLAN.md).

## New in-app regression cases authored (must pass for closure)

In [REGRESSION.md](../../REGRESSION.md), one per new `WORKTREES`-tab human surface:

- **REG-010** — pool view (allocated/idle background color + repo + local dir + id;
  rename/replace; read-only tab switch; backend-unavailable message)
- **REG-011** — copy-path control (clipboard == backend `worktree_path`)
- **REG-012** — registry-sourced repo filter (options from `GET /api/v1/repos`, not hardcoded)
- **REG-013** — Create (new idle worktree appears in the view)
- **REG-014** — Assign popup → bind (open tasks from `GET /api/v1/tasks`; worktree flips allocated)
- **REG-015** — Eject (returns idle, folder kept, task dequeued, no bounce-back)
- **REG-016** — Destroy (idle-only; allocated rejection) + standalone Dequeue (not close)

The queue-drain dispatch behavior change stays covered by the updated **REG-007 "pool of 1"**.

## Documents to inspect (primary review surface)

- **[PLAN.md](../../PLAN.md)** — the plan under approval. Read in order: "Scope
  Discipline", "Pass Order And Dependencies", PASS-0000…PASS-0009, the
  "Acceptance-criteria → pass map", and "Risks / caveats carried into implementation".
- **[TASK.md](../../TASK.md)** — the authoritative scope, 22 acceptance criteria ([BE]
  1–15, [FE] 16–22), and "What Does Not Count" falsifiers the plan maps onto.
- **[REGRESSION.md](../../REGRESSION.md)** — REG-007/008/009 (must stay green) + the new
  REG-010…REG-016 in-app cases (must pass for closure).
- **[HUMAN-DIRECTIVES-FOR-WORKER.md](../../HUMAN-DIRECTIVES-FOR-WORKER.md)** — the
  governing human decisions (**UPDATE 3** at the top wins; the DESIGN PIVOT model + UPDATE
  2 dequeue still stand).
- **[HANDOFF.md](../../HANDOFF.md)** — current resume point + the flagged process gaps.
- **[TASK-STATE.json](../../TASK-STATE.json)** — machine state (`phase=planning`,
  `plan_approved=false`).

## Change notes for this gate

This gate produced **planning artifacts only** — no product code changed:

- **Revised** [TASK.md](../../TASK.md), [PLAN.md](../../PLAN.md),
  [HANDOFF.md](../../HANDOFF.md), [TASK-STATE.json](../../TASK-STATE.json), and this review
  package to the UPDATE 3 reversed scope.
- **Edited** repo-root [REGRESSION.md](../../REGRESSION.md) to author the new in-app cases
  REG-010…REG-016 and the REG-007 "pool of 1" note.
- **Did not change** any file under
  [backend/orchestration](../../../../backend/orchestration/) or
  [app/codex_dashboard](../../../../app/codex_dashboard/). `TASK-META.json` and the
  TaskCreate artifacts were read, not edited.

## Validation / proof status at this gate

- **Build:** `go build ./...` exit 0 (pre-implementation baseline).
- **Unit tests:** Go `internal/queue|taskrun|httpapi` all `ok` (pre-implementation); the
  Python suite is the existing baseline.
- **Regression:** `not_run` — REG-010…REG-016 (and the REG-007/008/009 re-run) are PASS-0009
  work, after approval and after the UI passes.
- **Code/structure verification:** every load-bearing [BE] symbol cited in `TASK.md` was
  confirmed present; the [FE]-target desktop structure (nav tuple, tab dispatch, palette,
  HTTP-client + pure-helper patterns) was confirmed present (see
  [PLAN.md "Verified code baseline"](../../PLAN.md)).

There are no proof images at this gate (planning gate, no visual surface yet).

## Caveats / residual risk (see PLAN.md "Risks" for detail)

1. **Stable-path migration** of pre-existing random-temp lanes — discover must treat a
   non-pool-layout folder as not-a-pool-member (pinned in PASS-0001).
2. **`-fdx` (Eject) vs `-fd` (Assign reset)** — `restoreOwnedLane` parameterized / a sibling
   used so Eject gets a true baseline without weakening Assign.
3. **No clean-context QA lane / nested implementers** from this runtime — the coordinator
   must arrange QA as a QA-designated clean-context lane (no producer self-review),
   especially for the in-app UI passes.
4. **[FE] in-app proof needs a runnable Tk surface** — REG-010…REG-016 run in the running
   app, not via unit tests / endpoint smoke; if the env cannot drive Tk, that is a blocker,
   not grounds to downgrade the closure bar.
5. **REG-007 re-run** needs the human-authenticated Chrome debug session for the real-UI
   Ready flip (human authenticates once; agent drives) — a human prerequisite at PASS-0009.
6. **D1=replace retires the old `TASKS`-tab behavior** — reconcile with the coordinator how
   REG-003/REG-004 (old `Tasks` surface) are superseded so REGRESSION.md stays consistent.

## Consequence of rejection / requested revision

If rejected or sent back, the task stays in **planning**; [PLAN.md](../../PLAN.md) (and, if
the scope shape changes, [TASK.md](../../TASK.md) / [REGRESSION.md](../../REGRESSION.md)) is
edited to the requested shape and re-presented through the coordinator. No implementation
begins until the plan is approved.
