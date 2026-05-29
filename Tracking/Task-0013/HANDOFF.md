# Task-0013 Handoff

## Current Baseline (2026-05-29)

Task-0013 ("Rebrand to Obsidian, merge Claude Code tokens, fast hotkey
activation, source filter") is implemented, committed/pushed, and now PUBLISHED +
RESTARTED on the human's pinned dashboard release. All four objectives are
complete; the unit suite and the task-level regression run pass; the
human-authorized publish + restart deploy step is done. The task is **complete**.

## Publish + Restart deploy (2026-05-29 — DONE)

The human authorized "Publish + restart now." This is complete:
- Published pinned release **20260529T143554Z-e99ac895ee61** from the committed
  Git tree (`source_mode=git_commit`, `source_dirty=false`, commit `e99ac895`);
  the dirty working tree did NOT ship.
- Restarted the human's overlay onto it (old PID 36756 -> new PID 67656); the
  running process points at the pinned release id + release root and
  `Test-DashboardRelease.ps1` reports `current_release_error=null`,
  `startup_uses_pinned_launcher=true`, `running_process_count=1`.
- Decision A data preserved: `config.json` byte-identical, startup `.cmd`
  content-identical and still points at the runtime launcher, `dashboard.db` is
  the same file at the same path (additive idempotent migration; not reset).
- Human-surface proof captured from the pinned release code: OBSIDIAN brand,
  merged 138.5M total, expanded Codex/Claude source filter, and a before/after
  toggle (All 138.5M -> Codex-only 137.7M).
- Full evidence: [Testing/ReleaseProof/RELEASE-DEPLOY-PROOF.md](./Testing/ReleaseProof/RELEASE-DEPLOY-PROOF.md).

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

## Pinned-release step (RESOLVED 2026-05-29)

`TASK.md` noted the overlay the human runs is a PINNED RELEASE, not the repo
checkout, so source edits + passing tests did not change the running overlay until
a release was published and the overlay restarted. The human authorized
"Publish + restart now," and this worker executed it (see "Publish + Restart
deploy" above and [Testing/ReleaseProof/RELEASE-DEPLOY-PROOF.md](./Testing/ReleaseProof/RELEASE-DEPLOY-PROOF.md)).
The human's running overlay is now pinned to release
`20260529T143554Z-e99ac895ee61` (commit `e99ac895`), so the Obsidian rebrand,
merged Codex+Claude totals, fast activation, and source filter are now what the
human sees. No remaining human-lane step.

### Known limitation (tooling, not product)

The expanded-source-filter screenshot composites the released popup over the
agent's editor (the overrideredirect topmost overlay cannot be forced
un-occluded for a screen-region grab from a background process in this contended
desktop). The released checkbox control is fully legible, and the OBSIDIAN body +
before/after totals are proven by the `smoke-*/overlay.png` captures and the
deterministic `release-capture-summary.json`. Recorded in
[Testing/ReleaseProof/RELEASE-DEPLOY-PROOF.md](./Testing/ReleaseProof/RELEASE-DEPLOY-PROOF.md).

## Git

Only Task-0013 files were committed. The pre-existing unrelated working-tree
changes (other tasks' tracking edits; `app/codex_dashboard/token_time.py`,
`scripts/add_total_time_to_token_usage_csvs.py`, `tests/test_token_time.py`, and
the `Tracking/Task-0009`/`Task-0012` edits) were left untouched per
`HUMAN-DIRECTIVES-FOR-WORKER.md`.
