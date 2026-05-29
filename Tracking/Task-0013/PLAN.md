# Task-0013 Plan

Auto-approved per `HUMAN-DIRECTIVES-FOR-WORKER.md` (2026-05-29 dispatch directive:
"Auto-approve all gates; coordinator you are responsible for quality"). The
coordinator owns quality review of this plan; this worker proceeds without a
separate human approval click.

This is one task with three internally separable objectives (see [TASK.md](./TASK.md)).
The chosen solution shapes are already pinned in `TASK.md`; this plan sequences the
implementation, proof, and closure.

## Lifecycle Position

- Research: complete. `TASK.md` is a concrete implementation task with named seams,
  chosen solution shapes, acceptance criteria, and falsifiers. The two non-trivial
  external facts were re-verified on disk by this worker:
  - Live hotkey is `Ctrl+Alt+Space` (per `TASK.md`; target the real slow path, not
    the misremembered `Ctrl+Alt+Shift`).
  - Claude transcript shape confirmed: `type:"assistant"` lines with top-level
    `requestId` + `timestamp` and `message.usage` (`input_tokens`,
    `cache_creation_input_tokens`, `cache_read_input_tokens`, `output_tokens`).
    One sampled transcript had 219 assistant lines across only 56 distinct
    `requestId`s (every request spans multiple assistant lines), confirming the
    per-request dedup requirement is real.
- Planning: this document.
- Implementation: PASS-0001 (below).
- Regression: repo-root `REGRESSION.md` REG-001 desktop overlay smoke on an
  isolated lane + the Objective-3 activation timing proof.
- Closure: durable artifacts, unit suite, regression, commit/push, cleanup.

## Single Pass: PASS-0001

All three objectives ship in one pass against the same desktop app. They are kept
together because Objective 2 (more rows) interacts with Objective 3 (keep
activation fast). Each objective keeps its own proof.

### Objective 1 — Rebrand to "Obsidian" (display only; Decision A)

Decision A (default, lowest risk) is chosen: keep the
`%LOCALAPPDATA%\CodexDashboard` data root, the `CodexDashboard.cmd` /
`Start-CodexDashboard.ps1` startup artifact names, the scheduled-task name, the Go
runs-root literal, and `CODEX-REPO-MANIFEST.json` `"id"` UNCHANGED for
data-continuity. Rationale: the live machine has a populated `dashboard.db` +
`config.json` and enabled startup; renaming identifiers without flawless migration
would orphan the human's data (an explicit `What Does Not Count` failure). Decision A
preserves data with zero migration risk and still satisfies the human-visible
outcome (the app reads "Obsidian").

Display-string renames to "Obsidian":
- `app/codex_dashboard/__init__.py` `APP_NAME` -> `"Obsidian"`
- `app/codex_dashboard/ui.py:350` window title `"CODEX DASHBOARD"` -> `"OBSIDIAN"`
- `app/codex_dashboard/ui.py:567` brand label `"CODEX_DASHBOARD"` -> `"OBSIDIAN"`
- `app/codex_dashboard/__main__.py:16` argparse description -> `"Obsidian ingest utility"`
- `app/codex_dashboard/jobs.py:173` job label -> `"Obsidian overlay at sign-in"`
- `app/codex_dashboard/investigation.py:552` report title -> `"# Obsidian Bucket Investigation"`
- `README.md:1` title -> `# Obsidian`

Decision A record: a one-line code comment on `paths.py:app_data_root()` and a
`DATA-HANDLING.md` note stating the `%LOCALAPPDATA%\CodexDashboard` identifiers
stay "CodexDashboard" for data continuity while the product name is "Obsidian".

Out (Non-Goals): package dir rename, repo folder rename, `config.go` path literal
change, manifest `"id"` change, scheduled-task name change.

Proof: string-assertion test over the renamed display surfaces.

### Objective 2 — Merge Claude Code tokens

- `config.py`: add `claude_root: str` (default `str(Path.home() / ".claude")`),
  thread through `defaults()`, `load_config`, `save_config`. Empty/missing
  `claude_root` => no Claude source, no error.
- `models.py`: add `source: str` and `source_event_id: str` to `TokenEvent`.
- `storage.py`: add `source TEXT NOT NULL DEFAULT 'codex'` and
  `source_event_id TEXT` columns + `UNIQUE(source, source_event_id)` to
  `token_events`; migrate existing DBs additively (ALTER TABLE ADD COLUMN if
  missing, backfill `source_event_id` for legacy Codex rows). Insert with
  `INSERT OR IGNORE` keyed on `(source, source_event_id)`. Read `source` back in
  `load_events_since`.
- `scanner.py`: add `claude_jsonl_files()` (scan `{claude_root}/projects/**/*.jsonl`)
  and `parse_claude_token_events(file)` that groups assistant lines by `requestId`,
  keeps the LAST assistant event's `usage` per `requestId`, and maps:
  - `input_tokens` <- `input_tokens + cache_creation_input_tokens`
  - `cached_input_tokens` <- `cache_read_input_tokens`
  - `output_tokens` <- `output_tokens`
  - `reasoning_output_tokens` <- 0
  - `total_tokens` <- `input + cache_creation + cache_read + output` (canonical; tested)
  - Codex advisory fields <- None
  - `source` = `"claude"`, `source_event_id` = `requestId`
  - Codex events get `source` = `"codex"`, `source_event_id` = `"<session_path>:<line_offset>"`
- `ingest_once`: iterate Codex sources then Claude sources into the same table,
  idempotent re-scan.
- Aggregation/display unchanged (already source-agnostic).

Proof (unit tests): per-request dedup total equals last-event-per-request sum (not
219-line sum); canonical `total_tokens` formula; `source` column + null advisory on
Claude rows; idempotent re-scan; merged window total = Codex + Claude.

### Objective 3 — Fast hotkey activation

- `storage.py:initialize_db`: add
  `CREATE INDEX IF NOT EXISTS idx_token_events_event_timestamp ON token_events(event_timestamp)`.
- `ui.py`: split `refresh_data()` into:
  - `_load_dashboard_data()` -> reads DB off the implied path, returns
    (events, session_context_markers). Safe to call on a worker thread.
  - `_render_dashboard(events, markers)` -> pure Tk render from a snapshot, on UI thread.
  - `refresh_data()` keeps current behavior (load + render) for the background
    ingest poll path, but is no longer called inline by `show_overlay`.
  - `show_overlay()`: deiconify/lift/focus IMMEDIATELY (no synchronous DB read),
    then render from the current snapshot (`self.latest_events`). On cold start
    (snapshot empty), dispatch an off-thread load via the existing worker+queue
    pattern and render when it returns (not "instant but blank").
- Target the actual configured hotkey (`Ctrl+Alt+Space`); do not change the binding.

Proof: index existence test (PRAGMA index_list / sqlite_master); test that the
activation entry point does not perform the inline blocking DB read; cold-start
populates; and a timing harness measuring UI-thread blocking time during activation
before/after on a task-owned DB at least as large as the live DB with Claude data
merged in, recorded with a stated budget (UI-thread block <= 50 ms target).

### Objective 4 — Source filter (Codex/Claude)

Added by explicit human direction 2026-05-29 (see
`HUMAN-DIRECTIVES-FOR-WORKER.md`). Depends on Objective 2's `source` column and
must not regress Objective 3's non-blocking activation.

- `aggregation.py`: add `KNOWN_SOURCES`, `SOURCE_LABELS`, and a pure
  `filter_events_by_source(events, selected_sources)` helper that filters the
  in-memory snapshot by the stored `source` column (no DB read; `None` = all,
  empty set = none).
- `ui.py`: add `self.selected_sources` (default all known sources). Add a
  "Source" dropdown (`ttk.Menubutton` + per-source `tk.Menu` checkbuttons) next
  to the metric controls via `_build_source_filter_control`. `_toggle_source`
  mutates the selection and re-renders from `self.latest_events`
  (`_render_dashboard`), never calling `refresh_data()` / a synchronous DB read.
  `_render_dashboard` applies `filter_events_by_source` to the snapshot before
  aggregating so totals/projections/charts reflect the selection.

Proof (unit tests + lane): source-filtered aggregation (both = merged, codex-only,
claude-only, none = zero/no-crash, legacy null source = codex, filtered buckets);
toggle re-renders from snapshot without a DB read; lane demo
(`SOURCE-FILTER-RESULT.json`) showing All = Codex + Claude and None = 0; live
control captured in `Testing/smoke-usage/overlay.png`. Repo-root `REGRESSION.md`
gains `REG-005 Usage Source Filter (Codex/Claude)`.

## Data Handling / Lanes (binding)

- Repo-root `REGRESSION.md` and `DATA-HANDLING.md` bind. All validation, regression,
  ingest, and timing run on an ISOLATED lane with task-owned fixtures and a
  task-owned SQLite DB under `Tracking/Task-0013/Testing/Runtime/`.
- Do NOT touch the human's live dashboard config, live
  `%LOCALAPPDATA%\CodexDashboard\dashboard.db`, or live `C:\Users\gregs\.codex`
  Codex data. The "DB at least as large as the live DB" requirement is met with a
  task-owned SYNTHETIC DB seeded to a documented realistic size; the live DB is not
  opened to size it.
- Claude parser unit tests use task-owned fixture transcripts under
  `tests/fixtures/`, not the human's live `~/.claude` data.

## Git Hygiene (binding)

Stage and commit ONLY Task-0013 files (the source files this task changes plus
`Tracking/Task-0013/**` and new test files). Leave the pre-existing unrelated
working-tree changes (other tasks' tracking edits; `token_time.py`,
`add_total_time_to_token_usage_csvs.py`, `test_token_time.py`) untouched.

## Unit Suite

`python -m unittest discover -s tests -p "test_*.py" -v` must stay green.
