<!-- task-sync: repo=CodexDashboard; task_id=Task-0003; task_path=Tracking/Task-0003/TASK.md -->

# Task-0003: Add a Total/Norm metric toggle for CodexDashboard chart analysis.

## Source Of Truth

Local `Tracking/Task-0003/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0003:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

## Summary

This task adds a top-level metric-mode toggle so the overlay can switch between raw total tokens and a cost-weighted normalized proxy for chart analysis.

The goal is to make burst inspection more useful without replacing the existing total-token budget model or the current dashboard operator workflow.

## Goals

- Add a `Total` / `Norm` toggle in the overlay header.
- Drive velocity and repo charts from the selected metric mode.
- Keep hover values aligned with the selected metric mode.
- Keep the right-click investigation flow honest by preserving raw bucket context for investigation prompts.
- Avoid misleading raw budget-line comparisons when the chart is in normalized mode.

## Acceptance Criteria

- The overlay header includes a visible `Total` / `Norm` toggle.
- In `Total` mode, the chart behaves as it does today with raw token totals.
- In `Norm` mode, the chart and hover values use a normalized weighted metric derived from uncached input, cached input, output, and reasoning output.
- Repo mode uses the selected metric mode consistently for stacked bar totals.
- Right-click investigation still analyzes the real raw bucket range rather than a misleading normalized “total tokens” value.
- The unit test suite passes.
- The repo-root desktop overlay regression lane is rerun honestly after the change.

## Non-Goals

- Replacing the existing weekly budget model with a new billing model.
- Claiming the normalized metric is an exact OpenAI billing number.
- Reworking persistence or scanner ingestion.
- Expanding the overlay into a new multi-pane analytics product.

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `3`
- Local task path: `Tracking/Task-0003/TASK.md`
- Source commit: `ed4b29411673c462f5294dabbe0be38df4e13305`
- Local task SHA-256: `D0178312B7ECCF790DBB5F3A996471C455A8AC09E18AF7F8D7955D0922C113FA`
- Rendered at: `2026-05-29T17:24:19.9461985-04:00`