# Task-0013 TaskCreate Coordinator Notes

## Process model

TaskCreate as of 2026-05-29: coordinator + blind writer-worker, with a
**coordinator review** in place of a separate blind agent-auditor lane. The
coordinator is the keeper of common sense and human intention and reviews the
draft to **increase concreteness without narrowing scope**. See
`C:\Users\gregs\.codex\Orchestration\Processes\TaskCreate\README.md`.

## Writer draft

The writer-worker produced [TASK.md](./TASK.md) (one task, three objectives:
Obsidian rebrand, Claude+Codex token merge, hotkey-activation speed), grounded in
named files and live machine state (real hotkey `Ctrl+Alt+Space`, populated
`%LOCALAPPDATA%\CodexDashboard`, Claude `~/.claude/projects` usage shape).

## Coordinator review — concreteness changes made (scope preserved)

A one-time clean-context audit had run before the model changed; its
**concreteness** findings (not scope-narrowing) were folded into the draft by the
coordinator:

1. **Objective 2 idempotency key (mechanism blur → pinned).** Named a concrete
   per-source identity: add `source_event_id` + `UNIQUE(source, source_event_id)`
   (`requestId` for Claude, `"<session_path>:<line_offset>"` for Codex), inserted
   with `INSERT OR IGNORE`, so re-scanning a Claude transcript can't duplicate
   rows as the byte cursor advances.
2. **Objective 2 per-request usage rule (ambiguous → single rule).** "final/maximal
   usage" collapsed to one canonical rule: the usage from the **last** assistant
   event for each `requestId`.
3. **Objective 2 `total_tokens` formula (open choice → pinned).** `cache_creation_input_tokens`
   counts as input; canonical total = `input + cache_creation + cache_read + output`,
   with a required test. Moved out of "Remaining Uncertainty".
4. **Objective 3 cold-start (proxy closure → blocked).** Activation must show
   non-empty, current data; "instant but blank" on cold start is an explicit fail
   (added to acceptance, What-Does-Not-Count, and the falsifier).

## Scope integrity

No objective was split, narrowed, re-sequenced, or removed. All three remain.
The rebrand's keep-vs-migrate decision and the activation budget remain
human-gate preferences, not narrowings.

## Status

Coordinator-reviewed for concreteness, awaiting direct human approval.
