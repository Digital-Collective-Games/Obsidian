# Task-0013 Handoff

## Current Baseline (2026-05-29)

Task-0013 ("Rebrand to Obsidian, merge Claude Code tokens, fast hotkey
activation, source filter") is implemented and proven on an isolated lane. All
four objectives are complete; the unit suite and the task-level regression run
pass. The task is at the closure gate.

### Drift reconciled at start

The working tree already contained Objectives 1-3 implementation
(uncommitted), while `TASK-STATE.json` still said `phase: planning`. This worker
reconciled from disk: it audited the existing code against the acceptance
criteria, found Objective 4 (source filter), the Objective-2 Claude unit tests,
the Objective-3 timing proof, and the app-surface regression all missing, then
completed them. `TASK-STATE.json` now reflects the true `closure` state.

## What changed (Task-0013 files only)

Source:
- `app/codex_dashboard/__init__.py`, `ui.py`, `__main__.py`, `jobs.py`,
  `investigation.py`, `README.md` — rebrand display strings to "Obsidian".
- `app/codex_dashboard/paths.py` — `default_claude_root()` + Decision A comment.
- `app/codex_dashboard/config.py` — `claude_root` field.
- `app/codex_dashboard/models.py` — `source` + `source_event_id` on `TokenEvent`.
- `app/codex_dashboard/storage.py` — source columns, migration, source-event
  unique index, `event_timestamp` index, `upsert_claude_event`.
- `app/codex_dashboard/scanner.py` — Claude enumerator + per-request parser +
  Claude ingest into the merged pool.
- `app/codex_dashboard/aggregation.py` — `KNOWN_SOURCES`, `SOURCE_LABELS`,
  `filter_events_by_source` (Objective 4).
- `app/codex_dashboard/ui.py` — non-blocking `show_overlay` / off-thread
  cold-start load (Objective 3) and the Source filter dropdown (Objective 4).

Tests:
- `tests/test_task0013_obsidian.py` (new) — Objectives 1, 2, 3 index, 4.
- `tests/test_desktop_support.py`, `tests/test_jobs.py`,
  `tests/test_investigation.py` — updated rebrand label + Objective-3 activation
  tests.

Repo docs (task-required):
- `REGRESSION.md` — new `REG-005 Usage Source Filter (Codex/Claude)`.
- `DATA-HANDLING.md` — Decision A note (product "Obsidian", data identifiers stay
  "CodexDashboard").

Durable artifacts:
- `Tracking/Task-0013/PLAN.md` (Objective 4 added), `TASK-STATE.json`,
  `Testing/PROOF.md`, `Testing/REGRESSION-RUN-0001.md`, timing/filter harnesses
  and result JSON, `Testing/smoke-usage/` app-surface capture.

## Closure Preflight

- Required repo-root lane: `REG-001 Desktop Overlay Launch And Data Smoke`
  (canonical app-surface lane) plus the new `REG-005` source-filter case.
- Satisfying artifact: `Testing/REGRESSION-RUN-0001.md` (both `passed`), backed by
  `Testing/smoke-usage/overlay.png` + `overlay-summary.txt`, `Testing/PROOF.md`,
  `Testing/TIMING-RESULT.json`, `Testing/SOURCE-FILTER-RESULT.json`.
- Why it counts: real Tk app launched via the real hotkey/`show_overlay` path on a
  task-owned isolated lane (fixtures + isolated SQLite); the captured overlay
  shows OBSIDIAN, the merged 138.5M total, and the Source filter control.
- What it does not prove: it does not exercise the backend Jobs/Tasks lanes
  (unchanged by this task), and the source-filter per-checkbox UI before/after is
  shown at render/aggregation level (`SOURCE-FILTER-RESULT.json`) plus the live
  control in the screenshot rather than four separate GUI screenshots.
- Human-facing outcome: app reads "Obsidian"; totals include Claude
  (Codex + Claude = merged); activation does zero UI-thread DB work
  (340 ms -> ~5 ms); a Source filter lets the human include/exclude a source.
- Data safety: Decision A keeps the live `%LOCALAPPDATA%\CodexDashboard` data
  root and OS identifiers unchanged — no migration, nothing orphaned.

## Pinned-release caveat (not yet done; human-gated)

`TASK.md` notes the overlay the human actually runs is a PINNED RELEASE, not the
repo checkout. Source edits + passing tests do NOT change the running overlay
until a new release is published (`scripts\Publish-DashboardRelease.ps1`) and the
overlay restarted. Publishing to the human's live release lane and restarting the
human's running overlay touches the human lane, so it is left for explicit human
authorization rather than taken autonomously. The code, tests, and isolated-lane
proof are complete; the live publish/restart is the remaining human-lane step to
make the human SEE these changes on their pinned instance.

## Git

Only Task-0013 files were committed. The pre-existing unrelated working-tree
changes (other tasks' tracking edits; `app/codex_dashboard/token_time.py`,
`scripts/add_total_time_to_token_usage_csvs.py`, `tests/test_token_time.py`, and
the `Tracking/Task-0009`/`Task-0012` edits) were left untouched per
`HUMAN-DIRECTIVES-FOR-WORKER.md`.
