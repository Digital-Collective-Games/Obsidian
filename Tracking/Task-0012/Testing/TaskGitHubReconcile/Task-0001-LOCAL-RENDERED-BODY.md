<!-- task-sync: repo=CodexDashboard; task_id=Task-0001; task_path=Tracking/Task-0001/TASK.md -->

# Task-0001: Codex token-velocity dashboard with a hotkey-first overlay.

## Source Of Truth

Local `Tracking/Task-0001/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0001:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

## Summary

Build a Windows-first background app that watches `C:\Users\gregs\.codex` for new and modified session-history `.jsonl` files, extracts token-usage telemetry from the live append stream, and renders a compact bar-chart view of token velocity over multiple time buckets.

The primary user value is fast situational awareness:

- how many tokens were consumed recently
- whether current usage velocity is accelerating
- whether the current pace is likely to exhaust the weekly budget before reset
- whether that status can be checked with a single global hotkey instead of managing a normal foreground app window

## Goals

- detect new and modified `.jsonl` session-history files under `C:\Users\gregs\.codex`
- parse `token_count` events from those files instead of inferring usage from file counts or file sizes
- aggregate token usage into `1m`, `5m`, `15m`, `1h`, and `1d` intervals
- render those aggregates as a simple bar chart that can be read quickly
- compute and display a clear `redline` state when the current rate projects a weekly-budget overrun
- run quietly in the background with a global hotkey that summons and dismisses the dashboard overlay
- survive restarts without double-counting previously ingested events
- tolerate partially written trailing lines and files that are still being appended to by Codex

## Acceptance Criteria

- the app discovers new or modified session-history `.jsonl` files under `C:\Users\gregs\.codex` without requiring manual refresh
- the ingest pipeline extracts token usage from `token_count` events and records enough cursor state to avoid double-counting after restart
- the ingest pipeline ignores or safely retries incomplete trailing lines instead of crashing or corrupting aggregates
- the dashboard can switch between `1m`, `5m`, `15m`, `1h`, and `1d` bar views
- each interval view updates from the ingested event stream and reflects combined usage across all active session files
- the UI exposes a clearly labeled weekly-budget status with:
  - current recent velocity
  - projected weekly burn at that velocity
  - a visible `redline` state when the projection exceeds the configured weekly budget
- the first version supports a user-configured weekly token budget in absolute tokens
- when `rate_limits.secondary` metadata is available, the app stores and surfaces it as advisory context instead of pretending it is an exact local budget denominator
- a global hotkey can show and hide the overlay while the app remains backgrounded
- the overlay can be used without forcing the user to maximize or restore a conventional main window
- the app can start with Windows and keep its background footprint small enough that it is reasonable to leave running continuously

## Non-Goals

- building a full Codex session browser, transcript viewer, or orchestration console
- editing or rewriting files under `C:\Users\gregs\.codex`
- matching provider-side billing or quota calculations exactly when the local telemetry cannot prove them
- cross-platform support in the first version
- multi-user, networked, or cloud-synced telemetry storage
- a complex charting surface beyond the single token-velocity dashboard
- relying on a traditional maximize/minimize workflow as the primary UX

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `1`
- Local task path: `Tracking/Task-0001/TASK.md`
- Source commit: `75177b6dee23399358ee66676791fb41dc01d51e`
- Local task SHA-256: `9800D5DF5B67ACB2AE8C39CEE6E207A50323BB6F2313F195BEBEA3C4B7378E73`
- Rendered at: `2026-05-28T23:29:34.9541294-04:00`