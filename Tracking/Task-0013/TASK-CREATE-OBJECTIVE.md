# Task-0013 TaskCreate Objective

Worker-safe objective for the TaskCreate writer-worker. This states WHAT the task
should accomplish and WHY. It does not prescribe the implementation; the writer
chooses the honest writeup type and the TASK.md defines scope, acceptance, and
falsifiers per `TASK-CREATE.md`.

## One-Line Objective

Document a single CodexDashboard desktop-app task that (1) rebrands the app to
"Obsidian", (2) adds Claude Code token counts to the existing Codex token totals,
and (3) fixes the slowness when the overlay is activated by the global hotkey.

## Burden Being Reduced

The human runs this overlay to watch their agent token velocity. Today it only
reflects **Codex** usage, so it undercounts their real spend now that they also
use Claude Code. The app still calls itself "CodexDashboard" even though the
canonical remote is already `Digital-Collective-Games/Obsidian`, and activating
the overlay is "very slow," which hurts the hotkey-first, glanceable purpose of
the tool.

## Sub-Objectives And Known Seams (factual, from a codebase map — not a prescribed solution)

### 1. Rebrand "CodexDashboard" → "Obsidian"

The name appears as two different kinds of occurrence, which the task must treat
differently:

- Display/user-facing strings (safe to rename): window title `ui.py:350`
  (`"CODEX DASHBOARD"`), overlay header label `ui.py:567` (`"CODEX_DASHBOARD"`),
  `APP_NAME` in `app/codex_dashboard/__init__.py:1`, CLI help `__main__.py:16`,
  job label `jobs.py:173`, investigation report title `investigation.py:552`,
  `README.md:1`.
- Identifier/path literals (renaming these has data-migration consequences):
  the `%LOCALAPPDATA%\CodexDashboard` app-data root in `paths.py:43` (holds
  `dashboard.db` and `config.json`), startup artifacts in `startup.py:20,34`
  (`CodexDashboard.cmd`, `Start-CodexDashboard.ps1`), the scheduled-task name in
  `DATA-HANDLING.md:152`, the Go backend path literal in
  `backend/orchestration/internal/config/config.go:63,65`, and the repo-manifest
  `"id"` in `CODEX-REPO-MANIFEST.json:6`.

The task must decide and state how deep the rebrand goes (display-only vs full
identifier/path rename) and, if paths change, how existing user data/config is
migrated rather than orphaned. The Python package directory name
(`app/codex_dashboard`) and the local repo path (`C:\Agent\CodexDashboard`) are
separate decisions the task should call out explicitly.

### 2. Merge Claude token counts with Codex token counts

- Today token data comes only from Codex sessions: `scanner.py:session_jsonl_files()`
  scans `{codex_root}/sessions/*.jsonl`, and `scanner.py:parse_token_event()`
  (lines 32-80) extracts token usage into the source-agnostic `token_events`
  table (`storage.py:29-45`, unique key `UNIQUE(session_path, line_offset)`).
- Aggregation/display (`aggregation.py:event_metric_tokens`, `ui.py:refresh_data`)
  is already source-agnostic and sums by timestamp, so merging is feasible once a
  second source is ingested.
- The integration seam is `scanner.py:ingest_once()` (141-194) plus the config
  (`config.py:DashboardConfig`, only has `codex_root`). The task should add a
  Claude source (e.g. a `claude_root`, plausibly `C:\Users\gregs\.claude`),
  parse Claude Code session token usage (a different on-disk format than Codex),
  and merge into the same totals without double-counting or mislabeling source.

### 3. Fix overlay-activation slowness

- Hotkey path: `hotkey.py` (Win32 `RegisterHotKey`) → `ui.py:365` registers
  `toggle_overlay` → `show_overlay()` (`ui.py:2235-2240`) → `refresh_data()`
  (`ui.py:1671-1749`) runs **synchronously on the Tk UI thread**, connecting to
  SQLite and loading events (`load_events_since`), with no visible index on
  `token_events.event_timestamp` and, in repo chart mode, loading session-context
  markers for all events.
- Note the hotkey-name mismatch: config default is `Ctrl+Alt+Space`
  (`config.py:21`), not the `Ctrl+Alt+Shift` the human named. The task must
  confirm the human's actual configured hotkey and target the real slow path
  (likely the synchronous on-UI-thread refresh and/or unindexed query), with a
  before/after performance proof rather than a guess.

## Architecture Orientation

Tkinter desktop app, Python 3.13, SQLite (`%LOCALAPPDATA%\CodexDashboard\dashboard.db`,
WAL), JSON config, entry point `python -m app.codex_dashboard`, async ingest on a
worker thread polled every few seconds, unit tests under `tests/` run with
`python -m unittest discover -s tests -p "test_*.py" -v`.

## Audit-Readiness Notes

- Keep the three objectives in one task per the human directive; do not split or
  broaden on auditor preference (see `HUMAN-DIRECTIVES-FOR-WORKER.md`).
- Give each objective its own concrete acceptance criteria and a falsifier.
- The rebrand objective must include a data/config migration story (or an explicit
  decision to keep paths/identifiers unchanged) so renaming cannot silently orphan
  the user's data.
- The performance objective needs a measurable before/after proof bar, not a vibe.
