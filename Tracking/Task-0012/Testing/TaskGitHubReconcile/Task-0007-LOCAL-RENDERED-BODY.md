<!-- task-sync: repo=CodexDashboard; task_id=Task-0007; task_path=Tracking/Task-0007/TASK.md -->

# Task-0007: Bootstrap a task-owned home for Jarvis intervention analysis and conversation capture.

## Source Of Truth

Local `Tracking/Task-0007/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0007:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

## Summary

This task exists to hold analysis and conversation artifacts for a proposed `Jarvis` layer that treats direct human input as evidence that the current system failed to infer something it should have inferred.

The recurring question behind that model is:

- `How could the system have inferred the need for this input?`

The immediate need is not implementation. The immediate need is to preserve the framing, tensions, open questions, and repo-fit implications in durable task artifacts instead of letting them disappear into chat history.

## Goals

- Create a durable task-owned home for ongoing Jarvis analysis and conversation notes.
- Preserve the current framing that human intervention is a system-insufficiency signal worth analyzing.
- Keep the recurring inference question explicit in the task definition.
- Define the research space for daily per-repo reports and task proposals.
- Leave explicit room for a future `HUMAN-DESIRE.md` contract.
- Keep local human constraints in scope instead of flattening all repos into one global policy.
- Keep the desired value targets explicit:
  - truthfulness
  - compassion
  - tolerance
- Require later work to separate explicit statements, strong implication, and speculation.
- Keep the promotion boundary explicit so shared orchestration rules are only promoted after the local analysis is stable.

## Acceptance Criteria

- `Tracking/Task-0007/` exists with `TASK.md`, `PLAN.md`, `HANDOFF.md`, and a valid `TASK-STATE.json`.
- `Tracking/Task-0007/Research/` contains explicit homes for conversation capture and analysis synthesis.
- The task definition preserves the core Jarvis framing and the recurring inference question.
- The task definition keeps local human constraints and the target humane values explicit.
- The task definition explicitly separates explicit, strongly implied, and speculative inference.
- The task definition keeps implementation of Jarvis and recurring automation out of scope for now.
- The next honest phase after task creation is research and analysis rather than immediate system changes.

## Non-Goals

- Implementing `Jarvis`, autonomous critics, or daily job execution in this task.
- Claiming the system can infer every new or novel human request.
- Treating every human message as equally inferable or equally blameworthy.
- Finalizing a cross-repo workflow contract before the analysis is grounded.
- Recreating [Task-0006](/c:/Agent/CodexDashboard/Tracking/Task-0006/TASK.md) as a separate active incident-capture lane. `Task-0006` is now superseded historical context.

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `7`
- Local task path: `Tracking/Task-0007/TASK.md`
- Source commit: `75177b6dee23399358ee66676791fb41dc01d51e`
- Local task SHA-256: `D4FCB16684A7EDE6A6D4B4D694AB5DB88E85148735919080DCEFE7A51B33414C`
- Rendered at: `2026-05-28T23:29:40.5237330-04:00`