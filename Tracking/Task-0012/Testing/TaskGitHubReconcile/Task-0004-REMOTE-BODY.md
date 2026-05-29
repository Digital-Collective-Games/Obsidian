<!-- task-sync: repo=CodexDashboard; task_id=Task-0004; task_path=Tracking/Task-0004/TASK.md -->

# Task-0004: Declarative Codex jobs registry, Windows reconciliation, and Jobs tab for CodexDashboard.

## Source Of Truth

Local `Tracking/Task-0004/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0004:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

## Summary

CodexDashboard currently succeeds at one thing: a hotkey-first token-usage cockpit. It does not yet own the related machine-state problem.

Today, Codex-related Windows state is spread across one-off mechanisms:

- the dashboard is launched at sign-in through a Startup-folder `.cmd` file
- scheduled digest jobs live in Windows Task Scheduler and call scripts under `C:\Users\gregs\.codex\scheduled-digests\`
- the app itself still models startup as a special-case preference instead of a first-class managed job

That forces the user to inspect Startup-folder entries, Task Scheduler, and local scripts by hand. This task introduces one coherent model:

- a declarative registry of Codex-related Windows jobs
- a reconciler that compares desired state to actual durable Windows state
- a dedicated `Jobs` tab in the dashboard so drift and health are visible without polluting the token-usage cockpit

## Goals

- Define a tracked, user-authored declarative registry for local Codex-related jobs.
- Support an initial bounded set of Windows job kinds:
  - Startup-folder launchers
  - Scheduled Task entries
- Reconcile desired state against actual Windows durable state idempotently and classify drift honestly.
- Preserve `Usage` as the default hotkey-summoned surface and add a separate `Jobs` tab for machine-state visibility.
- Show at-a-glance summary counts plus per-job health, desired vs observed state, and plain-language drift reasons.
- Replace the dashboard startup special case with a first managed job entry under the jobs model.
- Keep raw Windows plumbing available behind explicit details instead of on the default surface.
- Make the current manually managed Codex jobs on this machine visible to the product instead of leaving them off-screen.

## Acceptance Criteria

- The product defines a file-backed declarative registry for local Codex-related jobs in a durable tracked location under `C:\Users\gregs\.codex\`.
- The first implementation supports at least two Windows durable-state mechanisms:
  - Startup-folder launchers
  - Scheduled Tasks
- The reconciler can compare desired vs observed state for supported job kinds and classify at least:
  - `in sync`
  - `missing`
  - `drifted`
  - `disabled`
  - `unknown` or `blocked`
- Reconcile/apply behavior for supported job kinds is idempotent.
- The existing dashboard startup launcher is represented as a managed job instead of a special-case product control.
- The current scheduled digest tasks can be bootstrapped into, or otherwise brought under, the new jobs model without requiring the user to recreate them manually from scratch.
- The hotkey overlay defaults to the existing token `Usage` surface and adds a separate `Jobs` tab rather than mixing jobs into the chart area.
- The Jobs tab shows summary counts, per-job rows, last reconciliation time, and plain-language drift reasons.
- The default Jobs surface keeps raw Windows plumbing hidden unless the user explicitly opens details.
- The default Jobs surface uses plain visible labels for job semantics rather than operator acronyms such as `State (D/O)`.
- If implementation keeps the mockup's `Logs` or `Terminal` shell affordances visible, they must be clearly treated as inactive future surfaces and not implied to be part of this task's delivered scope.
- The primary `Reconcile` action has explicit UI copy that makes its scope clear.
- `unknown` and `blocked` remain first-class visible states rather than being collapsed into `missing`.
- The Jobs tab exposes bounded actions for this slice:
  - refresh state
  - reconcile/apply supported drift
  - enable or disable supported jobs
- Focused unit coverage passes for the new registry, discovery, and reconciliation behavior.
- The repo-root app-surface regression proof is updated or extended honestly to cover the new Jobs surface.

## Non-Goals

- Building a general-purpose Windows automation console.
- Supporting cross-platform job management in this task.
- Replacing the token-usage overlay as the app's primary job.
- Reworking ingest, token aggregation, normalized-metric semantics, or bucket investigation behavior.
- Building a freeform in-app command editor or Task Scheduler clone in the first pass.
- Managing unrelated non-Codex Windows tasks.
- Turning the overlay into a multi-pane admin shell.
- Adding `run now` execution controls for arbitrary jobs in the first slice.
- Treating the mockup's `Logs` and `Terminal` tabs as committed scope for this task.
- Adding decorative system footer telemetry unless it is backed by a real product signal.

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `4`
- Local task path: `Tracking/Task-0004/TASK.md`
- Source commit: `75177b6dee23399358ee66676791fb41dc01d51e`
- Local task SHA-256: `6D2D5C6ADF52E15BB927B14A3BD47A3F27F989A1BFB0147474CD7787985E0E9F`
- Rendered at: `2026-05-28T22:49:07.2855194-04:00`