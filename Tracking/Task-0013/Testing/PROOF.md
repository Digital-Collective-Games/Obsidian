# Task-0013 Proof

Evidence for the four objectives in [../TASK.md](../TASK.md). All runtime proof
used a task-owned isolated lane under `Testing/Runtime/` with task-owned fixtures
and an isolated SQLite database. No live `%LOCALAPPDATA%\CodexDashboard`
`dashboard.db`/`config.json`, no `C:\Users\gregs\.codex`, and no `~/.claude` data
were read (per repo-root `REGRESSION.md` / `DATA-HANDLING.md` and
[../HUMAN-DIRECTIVES-FOR-WORKER.md](../HUMAN-DIRECTIVES-FOR-WORKER.md)).

## Unit Suite

`python -m unittest discover -s tests -p "test_*.py" -v` — **133 tests, OK**.
Task-0013 coverage lives in `tests/test_task0013_obsidian.py` (26 tests) plus the
Objective-3 activation tests added to `tests/test_desktop_support.py`.

## Objective 1 — Rebrand to "Obsidian" (Decision A)

- Display strings renamed: `__init__.py` `APP_NAME="Obsidian"`, `ui.py` window
  title `"OBSIDIAN"` and brand label `"OBSIDIAN"`, `__main__.py` argparse
  description `"Obsidian ingest utility"`, `jobs.py` job label
  `"Obsidian overlay at sign-in"`, `investigation.py` report title, `README.md`
  `# Obsidian`.
- Decision A recorded: `paths.py:app_data_root()` carries a comment, and
  `DATA-HANDLING.md` has a "Product Name vs. Data Identifiers (Task-0013
  Decision A)" section. The `%LOCALAPPDATA%\CodexDashboard` data root and OS
  identifiers are kept unchanged for data continuity (no migration, nothing
  orphaned).
- Tests: `RebrandTests` (app name, no product "CodexDashboard" string in
  user-facing surfaces, window title + brand label, README title).
- App-surface: `smoke-usage/overlay.png` header reads **OBSIDIAN**.

## Objective 2 — Merge Claude Code tokens

- `config.py` adds `claude_root` (default `~/.claude`), round-trips through
  load/save; empty `claude_root` ingests cleanly with zero Claude events.
- `scanner.py:parse_claude_token_events` de-duplicates per `requestId` using the
  LAST assistant event; canonical total = input + cache_creation + cache_read +
  output (cache-creation folded into the input column); Codex advisory fields are
  null for Claude.
- `storage.py` adds `source` + `source_event_id` columns, a
  `UNIQUE(source, source_event_id)` index, an additive migration for legacy DBs,
  and `upsert_claude_event` so a later re-parse updates an in-progress request
  instead of duplicating it.
- Tests: `ClaudeParserTests`, `ClaudeIngestIntegrationTests`,
  `ConfigClaudeRootTests` — per-request dedup (last event, not line sum),
  canonical formula, null advisory, merged window total = Codex + Claude,
  idempotent re-scan, streaming-request update-not-duplicate.
- Lane proof: ingest over the fixture produced 60 Codex + 40 Claude rows
  (80 Claude assistant lines de-duplicated to 40 requests); merged 7d total
  138,539,000 = Codex 137,700,000 + Claude 839,000; 0 Claude rows with non-null
  advisory. See `SOURCE-FILTER-RESULT.json` and `smoke-usage/overlay-summary.txt`
  (`7d_total=138.5M`).

## Objective 3 — Fast hotkey activation

- `storage.py:initialize_db` creates
  `idx_token_events_event_timestamp` (idempotent). Verified by
  `TimestampIndexTests` (sqlite_master + EXPLAIN QUERY PLAN).
- `ui.py:show_overlay` no longer calls `refresh_data()` synchronously: it shows
  the overlay immediately and renders from the in-memory snapshot
  (`self.latest_events`); on cold start it dispatches an off-thread load
  (`_start_activation_load`) and renders when it returns (not "instant but
  blank"). Verified by `test_desktop_support.py`
  (`test_show_overlay_renders_from_snapshot_without_blocking_load`,
  `test_show_overlay_cold_start_dispatches_offthread_load`,
  `test_start_activation_load_runs_load_off_thread`).
- **Measured before/after** (`TIMING-RESULT.json`) on a task-owned synthetic DB
  of 250,000 rows / 49.2 MB (larger than a realistic live DB; live DB not
  opened):
  - BEFORE (unindexed synchronous UI-thread read): **340.98 ms**
  - BEFORE (indexed synchronous UI-thread read): **340.04 ms**
  - AFTER (snapshot render, no UI-thread DB read): **4.78 ms**
  - Budget: **50 ms UI-thread block** — AFTER is under budget.
  - Honest caveat: the index barely changes THIS specific full-7d-window read
    because the read returns a large in-window result set that dominates cost;
    the index is still required by the acceptance criteria and benefits sparser
    queries and the ORDER BY. The decisive activation win is removing the
    synchronous read from the UI thread entirely (340 ms -> ~5 ms).
- App-surface: the smoke run activated via the real hotkey path
  (`hotkey_triggered=True`, `overlay_fallback=False`) and rendered populated data.

## Objective 4 — Source filter (Codex/Claude)

- `aggregation.py:filter_events_by_source` filters the in-memory snapshot by the
  stored `source` column (no DB read). `ui.py` adds a "Source" dropdown
  (Menubutton + per-source checkbuttons) next to the metric controls;
  `_toggle_source` re-renders from `self.latest_events` (no synchronous DB read,
  preserving Objective 3). Default = all checked.
- Tests: `SourceFilterTests` (both = merged, codex-only, claude-only,
  none = zero/no crash, legacy null source = codex, filtered buckets),
  `SourceFilterRenderPathTests` (toggle re-renders from snapshot, no DB read).
- Lane proof (`SOURCE-FILTER-RESULT.json`, single snapshot read):
  - All: 138,539,000 ; Codex only: 137,700,000 ; Claude only: 839,000 ;
    None: 0
  - merged_equals_sum_of_parts = true ; none_is_zero = true
- App-surface: `smoke-usage/overlay.png` shows the **"Source: All"** control in
  the toolbar, styled with the overlay.
- Regression: repo-root `REGRESSION.md` gains **REG-005 Usage Source Filter
  (Codex/Claude)**.

## Artifacts

- `TIMING-RESULT.json` — Objective 3 before/after timing.
- `SOURCE-FILTER-RESULT.json` — Objective 4 per-selection totals.
- `smoke-usage/overlay.png`, `smoke-usage/overlay-summary.txt` — app-surface
  capture (OBSIDIAN brand, merged total, source filter control).
- `activation_timing_harness.py`, `build_regression_fixture.py`,
  `source_filter_demo.py` — task-owned harnesses (reproducible; read no live data).
- `Runtime/` — disposable lane (fixtures + isolated DB). Not durable product
  state; safe to delete.
