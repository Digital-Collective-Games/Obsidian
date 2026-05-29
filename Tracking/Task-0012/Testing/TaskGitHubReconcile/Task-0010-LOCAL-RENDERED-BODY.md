<!-- task-sync: repo=CodexDashboard; task_id=Task-0010; task_path=Tracking/Task-0010/TASK.md -->

# Task-0010: Run Dream daily, email the results, and promote option tasks into real tasks that the dashboard can review and enqueue.

## Source Of Truth

Local `Tracking/Task-0010/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0010:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

## Summary

Dream should not stay trapped inside manual packet inspection.

The repo now has a much stronger Dream pipeline than it had before:

- first-principles burden analysis
- antagonistic solution generation
- audited winner synthesis
- richer winner-task writeups

But the output still requires too much manual harvesting.

This task turns that pipeline into a daily product bridge:

- run Dream on a schedule
- produce a digest email with enough context to review quickly
- show collapsible option-task sections in the email
- provide a `Promote to Task` action
- turn promoted options into real task skeletons
- make candidate asks visible to the `Review` tab and promoted tasks available to the `Tasks` tab for enqueue and later dispatch

This task is not the same as the intake UI, dispatch runtime, or the committed-work `Tasks` UI. It bridges Dream output into the normal task lifecycle.

The canonical promotion path is frozen for this task:

- [Task-0010](../Task-0010/TASK.md) owns the single backend promotion mechanism and provenance rules
- email `Promote to Task` links and any future `Review` tab `Promote to Task` button are both clients of that same promotion contract
- the product must not grow separate promotion semantics in email and dashboard

## Goals

- Run the Dream pipeline as a real daily product workflow.
- Preserve the output durably in the canonical intervention and Dream homes.
- Send a daily human-readable digest email with enough context to triage the proposed work.
- Use collapsible sections so the digest remains skimmable while still carrying useful context.
- Add a `Promote to Task` action for option-task proposals.
- When promoted, create a real task home with durable task artifacts rather than leaving the option only inside the packet.
- Make candidate asks visible in `Review` and promoted tasks visible and enqueueable through the future `Tasks` tab.
- Preserve provenance from promoted task back to:
  - source day packet
  - source problem
  - source winner
  - source option task

## Acceptance Criteria

- A backend-managed daily Dream job exists through the intended repo job path and produces the expected Dream packet outputs.
- A real digest email is emitted for a daily Dream run and includes collapsible candidate sections plus readable non-collapsible fallback content.
- The email's `Promote to Task` action and any dashboard-side candidate promotion from `Review` both use the same backend promotion contract and do not fork promotion semantics.
- Promoting a candidate creates a real task skeleton with:
  - `TASK.md`
  - `PLAN.md`
  - `HANDOFF.md`
  - `TASK-STATE.json`
- The generated `TASK-STATE.json` starts with:
  - `status: pending`
  - `phase: planning`
  - `plan_approved: false`
  - `current_pass: null`
  - `last_completed_pass: null`
  - `current_gate: planning`
  - `latest_audit_verdict: unknown`
- The promoted task records durable provenance back to the source packet, problem, winner, and option-task artifact.
- Candidate asks can be surfaced to the `Review` tab model, and the resulting promoted task can be surfaced to the `Tasks` tab model, without losing source lineage.
- Real proof exists for one full flow:
  - daily Dream run
  - digest emission
  - candidate promotion
  - generated task skeleton

## Non-Goals

- Replacing Dream with only email summaries.
- Letting email alone count as durable task creation.
- Dispatching promoted tasks automatically without the task lifecycle and UI controls to support that honestly.
- Reopening winner selection inside the promotion step.
- Turning every Dream option into a task automatically.

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `10`
- Local task path: `Tracking/Task-0010/TASK.md`
- Source commit: `75177b6dee23399358ee66676791fb41dc01d51e`
- Local task SHA-256: `D0CE1DA4F4E8D78AFC17E34CD1A150A83C121F88456891C371E3A4B742D8E422`
- Rendered at: `2026-05-28T23:29:43.1305490-04:00`