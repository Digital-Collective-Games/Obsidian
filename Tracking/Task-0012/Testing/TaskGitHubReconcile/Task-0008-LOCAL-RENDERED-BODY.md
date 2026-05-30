<!-- task-sync: repo=CodexDashboard; task_id=Task-0008; task_path=Tracking/Task-0008/TASK.md -->

# Task-0008: Build the backend task dispatch layer and durable execution-state contract.

## Source Of Truth

Local `Tracking/Task-0008/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0008:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

## Summary

If the dashboard is going to dispatch and supervise work honestly later, it needs a real execution layer behind any UI.

This task is backend-only.

The first honest operator path can be Codex or direct backend interaction against that contract while frontend work waits for later tasks.

That layer must answer questions the current repo cannot answer durably enough yet:

- what exactly was dispatched
- what contract is the agent currently expected to fulfill
- what state is the run in now
- is the agent actively working, legitimately waiting, blocked, or asleep
- when is a human or supervisor allowed to poke the run
- how does interruption work
- what task, thread, or session provenance should be returned when an operator wants deeper context
- which exclusive repository checkout the run owns for execution
- which useful commit that owned checkout can be restored to for proof or cleanup

This task owns that backend layer.

No dashboard tab, launch button, or frontend control ships in this task.

The core product promise is not just `start a task`.
It is:

- dispatch work durably
- keep the state honest
- keep the human informed
- recover when a run drifts, stalls, or goes silent

The task must build on the Temporal-backed orchestration direction delivered by [Task-0005](../Task-0005/TASK.md), not bypass it with legacy scheduler shortcuts or purely local volatile UI state.

The runtime shape is frozen for this task:

- task dispatch will be modeled as a separate backend-owned task-run workflow and API contract under `backend/orchestration/`
- task runs are not Git-tracked recurring job specs
- recurring jobs may trigger task-run creation later, but the task-run model remains a separate runtime concept
- simple task execution in this task's scope will use a backend-owned exclusive repository checkout or equivalent isolated repo lane rather than the human's shared primary worktree

## Goals

- Define a durable execution-state contract that can represent task runs honestly enough for supervision and recovery.
- Distinguish these states durably:
  - queued
  - dispatching
  - running
  - waiting_for_human
  - blocked
  - sleeping_or_stalled
  - interrupted
  - completed
  - failed
- Preserve enough state that the system can decide when a run deserves a poke rather than assuming silence is acceptable.
- Let the human interrupt task runs intentionally and see the result reflected durably.
- Preserve task, thread, and session provenance strongly enough that Codex or later clients can recover the deeper working context from backend readback.
- Give the backend an exclusive repo checkout model for simple task execution so the runtime does not rely on the human's shared primary worktree.
- Preserve enough git baseline information that a run can reset its owned checkout to a known-good useful commit during unit proof or execution cleanup.
- Build on the existing Temporal and backend foundation from [Task-0005](../Task-0005/TASK.md).
- Keep the dispatch layer separate from every client surface so the backend remains the source of truth.
- Leave behind a promotable contract for durable execution state if the task-local design proves stable.

## Acceptance Criteria

- The backend can create a durable task run through `POST /api/v1/tasks/{task_id}/dispatch`.
- The created task run persists:
  - task identity
  - run identity
  - status
  - wait contract
  - last meaningful progress
  - interrupt state
  - deep-context provenance
- For simple execution, the created task run also persists:
  - an exclusive owned checkout or equivalent isolated repo lane
  - the baseline commit captured at dispatch
  - the current commit when it changes materially
  - the useful restore commit or commits the runtime may reset to during proof or cleanup
- `GET /api/v1/task-runs/{run_id}` returns enough state to distinguish:
  - running
  - waiting_for_human
  - blocked
  - sleeping_or_stalled
  - interrupted
  - completed
  - failed
- `POST /api/v1/task-runs/{run_id}/poke` is rejected when the durable wait contract says silence is legitimate and accepted when a run is sleeping or stalled under the task rules.
- `POST /api/v1/task-runs/{run_id}/interrupt` records interrupted state durably and leaves the run recoverable for later review.
- The backend exposes context provenance strong enough for Codex or a future UI to recover the right task or thread context.
- The backend does not require ordinary simple-task execution to mutate the human's shared primary worktree.
- Unit proof and execution cleanup can reset the owned checkout to a recorded useful commit baseline without ambiguous manual cleanup steps.
- Focused automated tests exist for task-run state transitions, wait semantics, sleeping detection, poke gating, and interrupt handling.
- Real proof exists for one dispatch, one legitimate wait, one sleeping detection, one poke, and one interrupt path through direct backend interaction or Codex-driven backend calls.

## Non-Goals

- Shipping the whole `Tasks` tab UI in this task.
- Shipping any frontend dispatch controls or dashboard-side launch actions in this task.
- Implementing daily Dream generation or digest email in this task.
- Replacing task-owned markdown as the durable source of task scope and acceptance.
- Treating `no output yet` as a sufficient state model.
- Falling back to legacy Windows Scheduled Tasks for normal dispatch behavior.
- Building a generic agent framework beyond what CodexDashboard needs for honest task dispatch and supervision.
- Building a multi-tenant repo-farm product or generalized remote CI system beyond the exclusive local checkout model needed for simple task execution here.

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `8`
- Local task path: `Tracking/Task-0008/TASK.md`
- Source commit: `ed4b29411673c462f5294dabbe0be38df4e13305`
- Local task SHA-256: `7194889C63E82E15463824041B9DEC13AE59CAC8B22DA699D4BC0434A11BFC85`
- Rendered at: `2026-05-29T17:24:24.8902393-04:00`