<!-- task-sync: repo=CodexDashboard; task_id=Task-0005; task_path=Tracking/Task-0005/TASK.md -->

# Task-0005: Build a Git-backed Go + Temporal jobs control plane for CodexDashboard.

## Source Of Truth

Local `Tracking/Task-0005/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0005:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

## Summary

CodexDashboard currently has a local Python/Tk overlay and a partially explored dashboard-first jobs surface from `Task-0004`.

That is not the right foundation for the durable scheduler and orchestration direction the product now wants. The next honest move is a separate long-running backend:

- Git-tracked JSON job specs as desired state
- a Go control-plane service that reconciles desired state into Temporal
- a local Temporal + Postgres runtime for durable schedules and executions
- dashboard integration that shows desired-vs-runtime correctness without owning scheduling logic

For this task, `long-running backend` means more than a proved binary or a cleanly documented stack. The human-facing outcome is a service lane that remains up on the local computer for real scheduled jobs, plus a separate disposable validation lane so unit, smoke, and regression work do not disturb the always-on scheduler.

The v1 closure bar is a jobs system with `schedule`, `manual`, and `webhook` triggers. Future agent-oriented primitives should only be included now when they materially improve that v1 shape instead of widening the task.

## Goals

- Introduce a separate Go service for jobs and orchestration control.
- Self-host Temporal plus Postgres as the runtime durability layer for local-first development.
- Leave the local machine with a usable always-on service lane rather than a proof-only backend that has been torn down.
- Separate the always-on service lane from the disposable validation lane used for tests and regression.
- Keep Git-tracked JSON under `C:\Users\gregs\.codex\Orchestration\Jobs\` as the source of truth for desired job specs.
- Reconcile desired job state into Temporal schedules and workflows on service startup.
- Provide a non-restart sync path so Git changes can be reconciled explicitly while the service is already running.
- Support v1 trigger types:
  - `schedule`
  - `manual`
  - `webhook`
- Support at least one real executor path for Codex-oriented work, starting with `codex exec`.
- Expose backend APIs for job list/detail, recent runs, manual trigger, webhook intake, sync, and health.
- Integrate CodexDashboard so it can display desired state, runtime state, drift, recent runs, and last error for correctness checking.
- Let the dashboard invoke bounded backend actions such as `Sync now` and `Run now`.
- Shape the runtime so future agent orchestration can grow from it without making full agent primitives part of this task's closure bar.

## Acceptance Criteria

- The repo defines a Git-tracked JSON schema and job layout under `C:\Users\gregs\.codex\Orchestration\Jobs\` for v1 jobs and triggers.
- The first supported trigger types are:
  - `schedule`
  - `manual`
  - `webhook`
- A local self-hosted Temporal plus Postgres dev stack is documented and runnable for this repo's workflow.
- The repo defines a concrete service-lane workflow that leaves the local scheduler running after task closure.
- The repo defines a separate validation-lane workflow so test and regression work do not disturb the service lane.
- A Go service can load and validate desired job specs and reconcile them into Temporal schedules or workflows on startup.
- The same service exposes an explicit sync path so Git changes do not require a service restart to reconcile.
- Schedule, manual, and webhook trigger paths can each start durable runs through Temporal.
- At least one real executor path is integrated end to end for Codex-oriented work, starting with `codex exec`.
- The backend records enough run identity to audit correctness, including job id plus desired-state version or Git revision and Temporal runtime identifiers.
- The dashboard can query the backend and show desired-vs-runtime status, recent runs, and last error for real job data.
- The dashboard can invoke at least one bounded control path against the backend, such as explicit sync or manual run.
- The system does not require the dashboard process to be running for scheduled, manual, or webhook jobs to execute.
- Focused automated proof exists for spec validation, reconciliation behavior, and at least one real trigger path.
- Repo-root regression coverage is updated or extended honestly for any new or materially changed dashboard job surface required to exercise this backend-backed correctness flow.

## Non-Goals

- Building a full agent graph engine, autonomous planner, self-improvement loop, or self-healing system in v1.
- Embedding scheduler or clock ownership inside the Tk dashboard.
- Rewriting the dashboard in Go or replacing Tk in this task.
- Multi-node HA, managed cloud deployment, or a production-grade public webhook edge in the first slice.
- Building a full in-dashboard authoring, PR, or Git-conflict workflow for job specs in v1.
- Reworking token ingest, chart math, or other existing dashboard analytics except where the new correctness surface needs integration.
- Finishing the superseded `Task-0004` dashboard-first Jobs implementation path.

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `5`
- Local task path: `Tracking/Task-0005/TASK.md`
- Source commit: `75177b6dee23399358ee66676791fb41dc01d51e`
- Local task SHA-256: `0626C9F61EF36454B03DFF873BD316A02178D44D5C39CC1D4875E0736F026D5C`
- Rendered at: `2026-05-28T17:20:40.5899250-04:00`