# Task 0013

## Title

Rebrand the desktop overlay to "Obsidian", count Claude Code tokens alongside Codex, and make hotkey activation feel instant.

## Writeup Type

Concrete implementation task (burden-reduction proposal shape).

This is one task with three internally separable objectives that the human
deliberately bundled and asked to document together. Each objective has a chosen
solution shape grounded in named files and seams, its own acceptance criteria,
and its own falsifier. The merge is earned in `## Why These Three In One Task`
and the per-objective boundaries are kept explicit in `## Internal Mechanism Map`.

The three objectives are NOT to be split, broadened, narrowed, re-sequenced, or
blocked on auditor preference. See `## Scope Authority` and
`HUMAN-DIRECTIVES-FOR-WORKER.md`.

## Summary

The human runs this Tkinter desktop overlay to watch agent token velocity at a
glance, triggered by a global hotkey. Three things degrade that purpose today:

1. The app still calls itself "CodexDashboard" even though the canonical remote
   is already `Digital-Collective-Games/Obsidian`. The intended outcome is that
   the app presents itself as "Obsidian".
2. The overlay only counts Codex token usage. The human now also uses Claude
   Code, so the displayed totals undercount real spend. The intended outcome is
   that the same charts and totals reflect Codex tokens plus Claude Code tokens
   from one merged pool, without double-counting and without losing the ability
   to tell which source a number came from.
3. Activating the overlay via the global hotkey is "very slow." The intended
   outcome is that pressing the hotkey shows a populated overlay fast enough to
   feel instant, instead of stalling the UI while it reads the database.

## Who Is Affected

The single human operator who runs this overlay on their own Windows machine
(`admin@digitalcollective.games`). There are no other users. There is no remote
deployment of the desktop app.

## Burden Being Reduced

- Objective 1 (rebrand): the human reads a stale product name that no longer
  matches the repo/remote identity they have already adopted. Minor, but it is
  the explicit first directive and it removes naming drift between the app, the
  repo manifest, and the GitHub remote.
- Objective 2 (Claude merge): the human cannot trust the overlay's spend numbers
  because they silently exclude an entire agent (Claude Code). They currently
  have to mentally add "...plus whatever Claude used," which defeats a
  glance-and-trust tool.
- Objective 3 (activation speed): the human's hotkey-first, glanceable tool
  stalls on activation, so the moment they most want a fast read is the moment it
  is slowest. The exported work is waiting and re-pressing.

## Current Truth

### Rebrand

The name "CodexDashboard" / "CODEX DASHBOARD" / "codex_dashboard" appears in two
materially different kinds of place:

- Display / user-facing strings (no migration consequence):
  - `app/codex_dashboard/__init__.py:1` — `APP_NAME = "CodexDashboard"`
  - `app/codex_dashboard/ui.py:350` — root window title `"CODEX DASHBOARD"`
  - `app/codex_dashboard/ui.py:567` — overlay header brand label `"CODEX_DASHBOARD"`
  - `app/codex_dashboard/__main__.py:16` — argparse description `"CodexDashboard ingest utility"`
  - `app/codex_dashboard/jobs.py:173` — job label `"CodexDashboard overlay at sign-in"`
  - `app/codex_dashboard/investigation.py:552` — report title `"# Codex Dashboard Bucket Investigation"`
  - `README.md:1` — `# CodexDashboard`
- Identifier / path literals (renaming these has data-migration consequences):
  - `app/codex_dashboard/paths.py:43` — `app_data_root()` returns
    `%LOCALAPPDATA%\CodexDashboard`. This is the live home of `dashboard.db`
    (`paths.py:47`) and `config.json` (`paths.py:51`). It is confirmed populated
    on this machine: `config.json` exists with real settings
    (`weekly_budget_tokens: 3550000000`, `startup_enabled: true`).
  - `app/codex_dashboard/startup.py:20` — Startup folder script `CodexDashboard.cmd`
    (and `is_startup_enabled()` keys off this exact filename); `startup.py:34` —
    launcher `Start-CodexDashboard.ps1` under `%LOCALAPPDATA%\CodexDashboard\dashboard-launcher`.
    Startup is currently enabled for this user.
  - `backend/orchestration/internal/config/config.go:63,65` — Go backend runs
    root literal `...\CodexDashboard\orchestration-runs`.
  - `DATA-HANDLING.md:150-152` — documented `%LOCALAPPDATA%\CodexDashboard\...`
    runtime/runs roots and scheduled task `CodexDashboard-Orchestration-ServiceLane`.
  - `CODEX-REPO-MANIFEST.json:6` — `"id": "CodexDashboard"`.

The git remote is already `Digital-Collective-Games/Obsidian`
(`git remote -v` confirms `upstream`), and `CODEX-REPO-MANIFEST.json` already
points its providers at `Digital-Collective-Games/Obsidian`. So the remote
identity is "Obsidian"; only the local app surface and identifiers lag.

### Claude token merge

Token data is Codex-only today:

- `scanner.py:session_jsonl_files()` (lines 20-24) scans only
  `{config.codex_root}/sessions/*.jsonl`.
- `scanner.py:parse_token_event()` (lines 32-80) accepts only the Codex shape:
  `payload.type == "event_msg"`, `payload.payload.type == "token_count"`, reading
  `info.last_token_usage` (the incremental per-event usage) and
  `info.total_token_usage`. Fields: `total_tokens`, `input_tokens`,
  `cached_input_tokens`, `output_tokens`, `reasoning_output_tokens`.
- `config.py:DashboardConfig` (lines 14-28) only has `codex_root`; there is no
  Claude source.
- The events land in a source-agnostic table `token_events`
  (`storage.py:29-45`) keyed `UNIQUE(session_path, line_offset)`, and aggregation
  / display (`aggregation.py:event_metric_tokens` 46-58, `build_buckets` 61-93,
  `ui.py:refresh_data` 1671-1749) sum purely by `event_timestamp`. So the
  downstream pipeline is already source-agnostic; merging is feasible once a
  second source is ingested correctly.

Claude Code's on-disk format is different (confirmed by reading
`C:\Users\gregs\.claude\projects\<project-slug>\*.jsonl` on this machine):

- Token usage lives on lines with `type == "assistant"`, under `message.usage`,
  with fields `input_tokens`, `cache_creation_input_tokens`,
  `cache_read_input_tokens`, `output_tokens` (plus nested `cache_creation`,
  `iterations`, `server_tool_use`). There is no single `total_tokens` field and
  no `reasoning_output_tokens`; a total must be derived.
- Files live under `~/.claude/projects/<encoded-cwd>/*.jsonl`, NOT under a
  `sessions/*.jsonl` tree. (`~/.claude/sessions` also exists but the per-message
  usage is in the `projects` transcripts.)
- Timestamp is top-level `timestamp` (ISO-8601 with `Z`), like Codex.
- Double-counting is a confirmed real risk, not hypothetical: in one sampled
  transcript there were 115 `type == "assistant"` lines but only 32 distinct
  `requestId` values. Claude streams multiple assistant events per request whose
  `usage` blocks overlap, so naive per-line summation over-counts severely. A
  correct parser must de-duplicate per request: record exactly one usage record
  per `requestId`, using the usage from the LAST assistant event seen for that
  `requestId` (the final cumulative usage block), not a sum across the request's
  assistant lines.

### Activation speed

- The hotkey path is: `hotkey.py:GlobalHotkey` (Win32 `RegisterHotKey`,
  72-157) → polled by `ui.py:_poll_hotkey` (1611-1614) → `toggle_overlay`
  (2227-2233) → `show_overlay` (2235-2240) → `refresh_data` (1671-1749).
- `show_overlay()` calls `self.refresh_data()` **synchronously, before** it
  deiconifies the window. `refresh_data()` runs on the Tk UI thread and does:
  `connect()` to SQLite + `initialize_db()`, then `load_events_since()`
  (`storage.py:156-200`) which runs `SELECT * FROM token_events WHERE
  event_timestamp >= ? ORDER BY event_timestamp ASC`, then in repo chart mode
  also `load_session_context_markers()` for every event's session.
- There is **no index** on `token_events.event_timestamp`. The schema
  (`storage.py:29-45`) declares only the implicit primary key and the
  `UNIQUE(session_path, line_offset)` constraint. So `load_events_since` is a
  full-table scan that grows with DB size, executed on the UI thread at the exact
  moment of activation. Adding Claude data (Objective 2) will increase row count
  and make this worse.
- The human reported the hotkey as `Ctrl+Alt+Shift`. The shipped default is
  `Ctrl+Alt+Space` (`config.py:21`). The actual bound hotkey was confirmed by
  reading the live config on this machine: `%LOCALAPPDATA%\CodexDashboard\config.json`
  has `"hotkey": "Ctrl+Alt+Space"`, and the human confirmed `Ctrl+Alt+Shift` was a
  typo. So the slow path is real and is the synchronous-refresh-plus-unindexed-scan
  path; the `Ctrl+Alt+Shift` label was a recall slip and should not drive the
  design. The task targets the actual slow path, not the misremembered hotkey name.

## Target Truth

### Rebrand

- The app presents itself as "Obsidian" in every user-facing string listed above.
- The `%LOCALAPPDATA%` data/config/DB location and the OS-registered identifiers
  (Startup script filename, launcher filename, scheduled-task name, Go runs root)
  are handled by an explicit, stated decision (see `## Proposed Changes` →
  Objective 1), such that the human's existing `dashboard.db`, `config.json`, and
  enabled startup entry are NOT orphaned.

### Claude token merge

- The overlay's totals, projections, and charts reflect Codex tokens plus Claude
  Code tokens from one merged pool, summed by timestamp like today.
- Each ingested Claude usage record is counted once (de-duplicated per Claude
  request), with a Claude-derived `total_tokens` mapped into the existing
  `token_events` columns.
- The source of any event is still recoverable (a `source` column or equivalent
  stored discriminator), so a future view can split Codex vs Claude and so the
  Codex-specific advisory logic (`weekly_used_percent`, etc., which Claude does
  not provide) is not mislabeled as applying to Claude.

### Activation speed

- Pressing the configured hotkey shows a populated overlay quickly. Concretely:
  overlay activation must not block the Tk UI thread on a synchronous database
  read, and the time from hotkey press to a visible, populated overlay must be at
  or below a stated budget (see `## Acceptance Criteria` → Objective 3) measured
  on a database at least as large as the current one with Claude data merged in.

## Why These Three In One Task

Per the human directive (`HUMAN-DIRECTIVES-FOR-WORKER.md`, "document this" as one
task), these three ship together against the same desktop app. They are kept in
one task because:

- They share one implementation home (the `app/codex_dashboard` desktop app) and
  one human (the overlay operator), so one review and one enqueue is efficient.
- Objectives 2 and 3 interact: adding Claude data grows the table that Objective 3
  must keep fast, so designing them together avoids shipping a speed fix that the
  data merge immediately regresses.
- The rebrand is the human's stated framing for the whole change ("rebrand as
  Obsidian, and...").

They remain internally separable: each has its own files, its own acceptance
criteria, and its own falsifier. A reviewer can accept or reject each objective
independently even though they enqueue together.

## Internal Mechanism Map

| # | Mechanism | Where it acts | What it reduces | Proven done by |
|---|-----------|---------------|-----------------|----------------|
| 1 | Rebrand display + decide identifier/path strategy with migration | `__init__.py`, `ui.py`, `__main__.py`, `jobs.py`, `investigation.py`, `README.md`; plus a stated decision (and migration if renamed) for `paths.py`, `startup.py`, `config.go`, `DATA-HANDLING.md`, `CODEX-REPO-MANIFEST.json` | Stale name; identity drift | Objective 1 acceptance + falsifier |
| 2 | Add a Claude source: config root, Claude parser, dedup, source tag, merge | `config.py`, `scanner.py`, `storage.py`/`models.py` (source column), tests | Under-counted spend | Objective 2 acceptance + falsifier |
| 3 | Make activation non-blocking + index the time query | `ui.py` (`show_overlay`/`refresh_data` path), `storage.py` (index on `event_timestamp`) | Slow glance | Objective 3 acceptance + falsifier |
| 4 | Source filter over aggregation (Codex/Claude checkboxes) | `ui.py` (filter control + filtered render from snapshot), aggregation/render path (filter by `source`), tests, `REGRESSION.md` | Cannot isolate one source's spend; enables iterating on the distinction | Objective 4 acceptance + falsifier |

## Implementation Home

Primary home: the Python desktop app package `app/codex_dashboard/` (Tkinter UI,
SQLite storage, scanner/ingest, config). This is correct because all three
objectives are properties of that app: its displayed name, the data it ingests,
and how it activates.

Secondary, rebrand-only touches outside the package are limited to surfaces that
literally embed the old name and would otherwise contradict the rebrand:
`README.md`, `CODEX-REPO-MANIFEST.json`, `DATA-HANDLING.md`, and
`backend/orchestration/internal/config/config.go`. These are in scope only to the
extent the rebrand strictly requires (string/identifier consistency), not as
backend feature work.

Out of home (not changed here): backend orchestration behavior beyond the rebrand
path literal, Temporal/service-lane logic, the drain-queue consumer, the Python
package directory rename, the local repo folder rename, and any cross-repo work.
See `## Non-Goals` and `## Not Solved Here`.

## Constraints And Baseline

- Python 3.13, Tkinter, SQLite (WAL) at
  `%LOCALAPPDATA%\CodexDashboard\dashboard.db`, JSON config at
  `...\CodexDashboard\config.json`. Entry point `python -m app.codex_dashboard`.
- Ingest already runs async on a worker thread (`ui.py:schedule_ingest`
  1641-1669) and posts results to a queue polled by `_poll_ingest_results`
  (1616-1639). Overlay activation, however, calls `refresh_data` synchronously.
- The live machine has startup enabled and a populated DB/config, so any
  identifier/path rename has real migration stakes.
- Pinned-release deployment (critical for closure): the overlay the human
  actually runs is NOT the repo source. `scripts\Publish-DashboardRelease.ps1`
  snapshots `app/` into `%LOCALAPPDATA%\CodexDashboard\dashboard-releases\<id>`,
  writes a hash manifest, and PINS it via `dashboard-current-release.json`. The
  launcher `Start-CodexDashboard.ps1` (installed under `dashboard-launcher\`, run
  at login by the Startup `CodexDashboard.cmd`) reads the pinned manifest, sets
  `PYTHONPATH` to the release root, and launches `python -m app.codex_dashboard`
  from that pinned copy. Editing repo source does NOT change the running overlay:
  a new release must be published (re-pinned via `Publish-DashboardRelease.ps1`)
  and the overlay restarted. So this task's human-facing outcomes (the app reads
  "Obsidian", totals include Claude, activation is instant+populated, the source
  filter works) appear ONLY after publishing a new pinned release and restarting —
  source edits plus passing unit tests do not, by themselves, deliver them. Task
  closure proof must include publishing a release and verifying on the running
  pinned instance.
- Tests run with `python -m unittest discover -s tests -p "test_*.py" -v`.
  Existing relevant tests: `tests/test_ingest_core.py`, `tests/test_token_time.py`.

## Proposed Changes

### Objective 1 — Rebrand to "Obsidian"

Chosen solution shape: rename all display strings to "Obsidian" now, and make an
explicit, documented decision about identifiers/paths with a safe default that
avoids orphaning data.

- Display strings (rename to "Obsidian" / appropriate cased form):
  `__init__.py:1` `APP_NAME`; `ui.py:350` window title; `ui.py:567` brand label;
  `__main__.py:16` argparse description; `jobs.py:173` job label;
  `investigation.py:552` report title; `README.md:1` title.
- Identifier / path decision (the task REQUIRES one of these two be chosen and
  written down in the implementation, not left open):
  - Decision A (default, lowest risk): keep the `%LOCALAPPDATA%\CodexDashboard`
    data root, the `CodexDashboard.cmd` / `Start-CodexDashboard.ps1` startup
    artifact names, the scheduled-task name, the Go runs-root literal, and
    `CODEX-REPO-MANIFEST.json` `"id"` UNCHANGED, and add a one-line code comment
    plus a `DATA-HANDLING.md` note stating that these identifiers stay
    "CodexDashboard" for data-continuity reasons while the product name is
    "Obsidian". No migration needed; nothing is orphaned.
  - Decision B (full rename): rename the data root to
    `%LOCALAPPDATA%\Obsidian` and the OS identifiers, AND ship a one-time
    migration that, on startup, detects an existing `%LOCALAPPDATA%\CodexDashboard`
    with `dashboard.db`/`config.json`, moves or copies them to the new location
    (idempotently, only if the new location does not already exist), re-points the
    Startup script and any scheduled task, and leaves the old location safe to
    delete. The migration must be covered by a test and must be a no-op on a fresh
    install.
  The task does not pre-pick A vs B for the implementer beyond requiring that
  whichever is chosen, existing data is preserved. If B is chosen, the migration
  is mandatory.
- Python package directory (`app/codex_dashboard`) rename and local repo path
  (`C:\Agent\CodexDashboard`) rename are explicitly OUT (see Non-Goals) to keep
  import paths, the manifest `local_root`, and tooling stable.

### Objective 2 — Merge Claude Code tokens

Chosen solution shape: add a second ingest source that parses Claude Code
transcripts into the existing `token_events` pool, de-duplicated per request and
tagged with its source.

- `config.py:DashboardConfig`: add `claude_root: str` (default
  `str(Path.home() / ".claude")`), threaded through `defaults()`, `load_config`,
  and `save_config` like `codex_root`. Treat a missing/empty `claude_root` as
  "no Claude source" (skip cleanly).
- `scanner.py`: add a Claude file enumerator (scan
  `{claude_root}/projects/**/*.jsonl`) and a `parse_claude_token_event()` that
  accepts `type == "assistant"` lines, reads `message.usage`, and maps:
  - `input_tokens` ← `usage.input_tokens + usage.cache_creation_input_tokens`
    (cache-creation tokens are real, billed input cost, so they count as input)
  - `cached_input_tokens` ← `usage.cache_read_input_tokens`
  - `output_tokens` ← `usage.output_tokens`
  - `reasoning_output_tokens` ← 0 (Claude has no such field)
  - `total_tokens` ← `usage.input_tokens + usage.cache_creation_input_tokens +
    usage.cache_read_input_tokens + usage.output_tokens` (the single canonical
    formula; a test must assert it)
  - Codex-only advisory fields (`weekly_used_percent`, `weekly_window_minutes`,
    `weekly_resets_at`) ← None for Claude events.
  De-duplicate per Claude request: record exactly one usage record per
  `requestId`, using the usage from the last assistant event for that `requestId`
  (the final cumulative usage block), NOT one per assistant line, to avoid the
  confirmed 115-lines/32-requests over-count.
- `ingest_once` (`scanner.py:141-194`): iterate Codex sources then Claude
  sources into the same `token_events` table, preserving the incremental
  cursor/offset behavior so re-ingest is idempotent.
- `storage.py` + `models.py`: add a `source` discriminator column to
  `token_events` (e.g. `source TEXT NOT NULL DEFAULT 'codex'`) and to the
  `TokenEvent` dataclass, set on insert and read back in `load_events_since`. Make
  cross-scan idempotency concrete with a per-source event identity: add a
  `source_event_id TEXT` column and a `UNIQUE(source, source_event_id)`
  constraint, where `source_event_id` is the Claude `requestId` for Claude events
  and `"<session_path>:<line_offset>"` for Codex events. Insert with
  `INSERT OR IGNORE` on that key so re-scanning the same Claude transcript cannot
  create duplicate rows even though the byte cursor advances. (The existing
  `UNIQUE(session_path, line_offset)` may remain for Codex compatibility, but the
  per-source key is the authoritative idempotency guarantee.)
- Aggregation/display stay source-agnostic (no change needed to
  `aggregation.py:event_metric_tokens` for totals); the `source` column exists so
  source can be recovered and so Codex advisory logic is not applied to Claude
  events.

### Objective 3 — Fast hotkey activation

Chosen solution shape: stop blocking the UI thread on activation, and index the
time-window query.

- `storage.py:initialize_db`: add `CREATE INDEX IF NOT EXISTS
  idx_token_events_event_timestamp ON token_events(event_timestamp)` so
  `load_events_since` (`WHERE event_timestamp >= ? ORDER BY event_timestamp ASC`)
  uses an index instead of a full-table scan. Idempotent for existing DBs.
- `ui.py:show_overlay` (2235-2240): show the overlay immediately (deiconify /
  lift / focus) and do the data read off the UI thread, OR render from the most
  recent already-loaded snapshot (`self.latest_events`, populated every
  `polling_seconds` by the background ingest) and let the next scheduled refresh
  update it. The hard requirement is that activation does not run a synchronous
  SQLite read on the Tk UI thread. If the snapshot path is used and the snapshot
  is empty/unset (cold start before the first background refresh), activation must
  kick off an off-thread load and populate the overlay when it returns, rather
  than showing an empty overlay. Reuse the existing worker-thread + queue pattern
  (`schedule_ingest` / `_poll_ingest_results`) rather than inventing a new
  concurrency mechanism.
- Target the actual configured hotkey (`Ctrl+Alt+Space` per live config), not the
  misremembered `Ctrl+Alt+Shift`.

### Objective 4 — Source filter (Codex/Claude)

Added by explicit human direction 2026-05-29 (see `HUMAN-DIRECTIVES-FOR-WORKER.md`).
Overrides the prior per-source-UI non-goal. Depends on Objective 2's `source`
column and must not regress Objective 3's non-blocking activation.

Chosen solution shape: a source filter control in the overlay (a dropdown / popup
containing a checkbox per source: Codex and Claude) that selects which source(s)
are included in the displayed token aggregation (totals, projections, charts).

- The filter reads the already-stored `source` column. It filters the in-memory
  loaded snapshot (`self.latest_events`) and re-renders; it must NOT add a
  synchronous SQLite read on the Tk UI thread (preserve Objective 3).
- Filtering happens at aggregation/render time only — no re-ingest, no schema
  change beyond Objective 2's `source` column.
- Default = all sources checked (today's merged behavior). Selecting a subset
  restricts totals/buckets/charts to those sources. Unchecking all sources shows
  a clear empty/zero state, not a crash.
- The control matches the overlay's existing visual style. Place it near the
  existing interval/usage controls.
- Persist the selection at least for the lifetime of the running app so toggling
  the overlay does not reset it (config-level persistence is acceptable but not
  required).
- This is new human-facing functionality, so repo-root `REGRESSION.md` gets a
  named case (or an extended existing case) covering the filter interaction.

## Expected Resolution (human-visible)

- The window title bar, overlay header, README, and CLI help read "Obsidian".
- The "last 7d" total and the chart go UP after the change in a way that
  corresponds to the human's Claude Code usage, and a Claude-only day still shows
  non-zero tokens.
- Pressing the hotkey pops a populated overlay essentially immediately; the UI no
  longer hangs at the moment of activation.
- A filter control with Codex and Claude checkboxes lets the human include or
  exclude a source; toggling a checkbox updates the totals and chart immediately.

## Goals

- Rename all listed display strings to "Obsidian".
- Make an explicit, data-safe decision (and migration if renaming) for the
  `%LOCALAPPDATA%` paths and OS identifiers.
- Ingest Claude Code token usage into the existing merged `token_events` pool,
  de-duplicated per request, tagged with its source, mapped onto existing columns.
- Make hotkey activation non-blocking on the UI thread and index the time-window
  query, with a measured before/after.
- Add a source filter (Codex/Claude checkboxes) over the displayed token
  aggregation, operating on the stored `source` column without blocking the UI
  thread (Objective 4).

## Non-Goals

- Renaming the Python package directory `app/codex_dashboard` or its import path.
- Renaming the local repo folder `C:\Agent\CodexDashboard` or the manifest
  `local_root`.
- Backend orchestration, Temporal, or service-lane feature work beyond the single
  rebrand path literal in `config.go` and the `DATA-HANDLING.md` text.
- Drain-queue consumer work or any cross-repo work.
- A separate side-by-side per-source *breakdown chart* (Codex bars next to Claude
  bars). Note: a per-source *filter* (Codex/Claude checkboxes that include/exclude
  a source from the existing merged aggregation) IS in scope as Objective 4, added
  by human direction 2026-05-29. The breakdown-chart visualization remains out
  unless the human asks for it.
- Changing the default hotkey binding.

## Acceptance Criteria

### Objective 1 — Rebrand (each is pass/fail)

- `app/codex_dashboard/__init__.py` defines the app name as "Obsidian" (not
  "CodexDashboard").
- `ui.py:350` window title and `ui.py:567` overlay brand label render "Obsidian"
  (cased as appropriate), confirmed by launching `python -m app.codex_dashboard`
  or by a string assertion test.
- `__main__.py`, `jobs.py`, `investigation.py`, and `README.md:1` no longer
  contain the user-facing "CodexDashboard"/"Codex Dashboard" product name and use
  "Obsidian" instead.
- The implementation contains an explicit recorded decision (Decision A or
  Decision B above) for the `%LOCALAPPDATA%` data root and OS identifiers. If
  Decision B, a startup migration exists, is covered by a test, moves/copies an
  existing `dashboard.db` + `config.json` to the new location idempotently, and is
  a no-op on a fresh install.

### Objective 2 — Claude merge (each is pass/fail)

- `DashboardConfig` has a `claude_root` field that round-trips through
  `load_config`/`save_config`; absent/empty `claude_root` ingests cleanly with no
  Claude events and no error.
- A new test feeds a fixture Claude `projects/.../*.jsonl` (mirroring the real
  shape: `type:"assistant"`, `message.usage`, repeated `requestId`s) and asserts
  the resulting summed tokens equal the per-request de-duplicated total — NOT the
  per-assistant-line sum — proving no double-count.
- After ingest, `token_events` rows carry a `source` value distinguishing
  `claude` from `codex`, and Claude rows have null Codex advisory fields.
- Re-running ingest over the same Claude files does not create duplicate
  `token_events` rows (idempotent re-scan).
- With both sources present, `load_events_since` / `build_buckets` produce totals
  equal to Codex-total + Claude-total for the same window.

### Objective 3 — Activation speed (each is pass/fail)

- `initialize_db` creates an index on `token_events(event_timestamp)`; verified by
  querying `sqlite_master` (or `PRAGMA index_list`) in a test.
- Overlay activation does not perform a synchronous SQLite read on the Tk UI
  thread: `show_overlay` either renders from a pre-loaded snapshot or dispatches
  the read to a worker thread. Verified by code inspection plus a test that the
  activation entry point returns without calling the blocking DB read inline.
- Activation presents non-empty, current-enough data, not an "instant but blank"
  overlay. On cold start (snapshot empty/unset before the first background
  refresh), activation triggers an off-thread load and then populates. The
  "instant but blank" path is an explicit fail.
- A measured before/after exists on a database at least as large as the current
  live DB with Claude data merged in: the time from activation call to a visible
  populated overlay (or the UI-thread blocking time during activation) is recorded
  before and after, the after value is at or below a stated budget, and the budget
  is stated in the proof artifact. Suggested budget: UI-thread blocking time
  during activation under ~50 ms (a number the implementer may tighten with
  evidence, but must state and measure).

### Objective 4 — Source filter (each is pass/fail)

- The overlay presents a visible filter control with separate Codex and Claude
  checkboxes (a dropdown/popup with checkboxes), styled consistently with the
  overlay.
- With both checked, displayed totals/charts equal the merged Codex+Claude
  aggregation (unchanged from Objective 2's merged behavior).
- Unchecking Claude shows Codex-only totals/charts; unchecking Codex shows
  Claude-only; unchecking both shows a clear empty/zero state and does not crash.
- The filter reads the stored `source` column and filters the in-memory snapshot;
  applying the filter does NOT perform a synchronous SQLite read on the Tk UI
  thread (verified by code inspection plus a test that the filter path renders
  from the snapshot without the blocking DB read).
- A unit test asserts the source-filter aggregation: given mixed-source events,
  the filtered total equals the sum of only the selected source(s).
- Repo-root `REGRESSION.md` has a named case (or an extended case) covering the
  filter interaction.

## What Does Not Count

- Renaming only some display strings while others still say "CodexDashboard" to
  the user. The user-facing rename must be complete.
- Renaming the data/config paths or OS identifiers WITHOUT a migration, so the
  human's existing `dashboard.db`, `config.json`, or enabled startup entry is
  orphaned or silently recreated empty. This is an explicit fail, not a
  preference. (Either keep them, or migrate them.)
- Summing every Claude `type:"assistant"` line's `usage` (the 115-line path)
  instead of de-duplicating per request — that inflates totals and does not count.
- Counting Claude tokens but storing them with no recoverable source, or
  mislabeling Claude events with Codex advisory fields.
- "Feels faster" with no measurement. Objective 3 requires a recorded before/after
  against a realistically sized DB.
- Moving the blocking read to a worker thread but leaving the unindexed full scan,
  or adding the index but leaving the read synchronous on the UI thread. Both
  changes are required.
- Targeting `Ctrl+Alt+Shift` (the misremembered name) instead of the actual bound
  hotkey.

## Proof Plan

- Rebrand: string-assertion test over the renamed display surfaces and/or a
  launch screenshot showing "Obsidian"; for Decision B, a migration unit test.
- Claude merge: unit test with a fixture Claude transcript containing repeated
  `requestId`s, asserting de-duplicated totals and idempotent re-scan; a unit test
  asserting merged totals = Codex + Claude over a window; a test asserting the
  `source` column and null Codex-advisory fields on Claude rows.
- Speed: a timing harness (script or test) that builds/loads a DB at least as
  large as the current live one (with Claude data), measures activation
  UI-thread blocking time before and after the change, and records both numbers
  plus the stated budget in a task-owned proof artifact under
  `Tracking/Task-0013/Testing/`.
- Source filter: a unit test asserting source-filtered aggregation (filtered
  total = sum of selected sources only) over mixed-source events; a test that the
  filter path renders from the in-memory snapshot without a synchronous UI-thread
  DB read; and a regression artifact showing the filter control plus a
  before/after of toggling a source (subject to GUI-capture availability, with an
  honest caveat if blocked).
- All `tests/test_*.py` continue to pass via
  `python -m unittest discover -s tests -p "test_*.py" -v`.

## Falsifiers

- Objective 1 falsifier: after the change, any user-facing surface in the listed
  set still presents "CodexDashboard"/"Codex Dashboard" as the product name; OR
  paths/identifiers were renamed and the human's pre-existing `dashboard.db` or
  `config.json` is no longer read (settings reset to defaults, startup broken).
- Objective 2 falsifier: a day with known Claude-only activity shows zero tokens;
  OR merged totals exceed Codex+Claude because assistant lines were summed without
  per-request dedup; OR a second ingest run multiplies row counts; OR source is
  unrecoverable / Claude rows carry Codex advisory values.
- Objective 3 falsifier: activation still runs a synchronous SQLite read on the Tk
  UI thread; OR no index on `event_timestamp` exists after init; OR no recorded
  before/after measurement exists, or the after value exceeds the stated budget on
  a realistically sized DB; OR activation shows an empty overlay on cold start
  instead of populating real data.
- Objective 4 falsifier: no visible per-source filter control with Codex and
  Claude checkboxes; OR toggling a checkbox does not change the displayed
  totals/charts; OR the displayed value for a selected subset does not equal the
  sum of only the selected source(s); OR applying the filter triggers a
  synchronous SQLite read on the Tk UI thread; OR unchecking a source still
  includes it or crashes; OR no test covers source-filtered aggregation; OR the
  per-event `source` distinction is missing at ingestion (the floor).

## Causal Claim

- Spend looks low because an entire agent's tokens are excluded at the ingest
  boundary (`scanner.py` only reads Codex), not because aggregation is wrong
  (aggregation is already source-agnostic). Fixing the ingest source fixes the
  count.
- Activation is slow because `show_overlay` runs `refresh_data` synchronously on
  the UI thread and `load_events_since` is an unindexed full scan that grows with
  the DB. Removing the synchronous read and indexing the query removes the stall.

## Rival Explanations Considered

- "Aggregation already handles multiple sources, so nothing is needed" — false for
  counting: the data never arrives because the scanner only reads Codex sessions.
- "The slowness is the Win32 hotkey/message loop" — `hotkey.py` only polls and
  posts a callback; the heavy work is the synchronous DB read in `show_overlay`,
  which is the clear, code-visible cost. If profiling during proof shows the
  bottleneck is elsewhere (e.g. font/Tk layout), that finding belongs in the proof
  artifact and would refine, not block, the task.

## Rival Mechanisms Considered

- Claude merge via a separate table + a union at query time: rejected because
  `token_events` + `aggregation` are already source-agnostic; a `source` column on
  the existing table is the smaller change and keeps one merge path.
- Speed via caching query results without an index: rejected as half a fix; the
  unindexed scan still bites on cache miss and as the DB grows with Claude data.
- Full path rename without migration: rejected — it orphans live data (explicit
  human correctness constraint).

## Tradeoffs

- Decision A (keep identifiers) trades naming purity for zero migration risk;
  Decision B (full rename) trades migration complexity for a fully consistent
  identity. The task requires whichever is chosen to preserve existing data.
- Adding a `source` column is a schema change; it is additive with a default so
  existing rows/DBs remain valid.

## Shared Substrate

- `token_events` table and `aggregation.py` are shared by both Codex and Claude
  ingest; the `source` column and the timestamp index live there and benefit any
  future source. Naming this does not widen the task's claimed outcome.

## Not Solved Here

- A separate per-source (Codex vs Claude) *breakdown chart* (side-by-side bars).
  The source *filter* (Codex/Claude checkboxes over the merged aggregation) IS
  built here as Objective 4; a distinct comparison/breakdown visualization is not.
- Renaming the Python package directory or local repo folder.
- Backend/Temporal/service-lane behavior beyond the rebrand string/path literal.
- Claude rate-limit/advisory equivalents (Claude transcripts do not expose the
  Codex `weekly_used_percent` shape).

## Human Relief If Successful

The human sees the app as "Obsidian", trusts the token totals because they now
include Claude Code, and gets an overlay that pops instantly when they hit the
hotkey — restoring the glance-and-trust purpose of the tool.

## Remaining Uncertainty

- (Resolved by coordinator review) `cache_creation_input_tokens` counts as input
  and is included in the canonical `total_tokens` formula in Objective 2; the
  implementation must add a test asserting that formula.
- The exact activation budget (a starting suggestion of ~50 ms UI-thread block is
  given; the implementer must state and measure the real number).
- Whether the human prefers Decision A or Decision B for the rebrand depth; the
  task allows either as long as data is preserved. This is a human-gate preference
  note, not a blocker (default to A if unstated).

## Scope Authority

Authoritative per `HUMAN-DIRECTIVES-FOR-WORKER.md`: this stays ONE task with
FOUR objectives — the original three (rebrand, Claude+Codex merge, activation
speed) plus Objective 4 (source filter), which the human added by explicit
direction on 2026-05-29 (overriding the earlier per-source-UI non-goal). An agent
auditor must not split, broaden, narrow, re-sequence, or block this task on
preference. The human said: "don't let the auditor dissuade
you, just override if it says something stupid." Preference-only auditor findings
are optional strengthening or human-gate notes, not blockers.

Two findings are explicitly NOT preference and are addressed above as hard
requirements: (a) any path/identifier rename must preserve existing data via
migration or an explicit keep-decision (`What Does Not Count`, Objective 1
falsifier); (b) the Claude merge must parse Claude's distinct format and
de-duplicate per request without mislabeling source (`What Does Not Count`,
Objective 2 falsifier).

## References

- `HUMAN-DIRECTIVES-FOR-WORKER.md`, `TASK-CREATE-OBJECTIVE.md`,
  `TASK-CREATE-CONTEXT-MANIFEST.md` (this task folder).
- Rebrand: `app/codex_dashboard/__init__.py:1`, `ui.py:350`, `ui.py:567`,
  `__main__.py:16`, `jobs.py:173`, `investigation.py:552`, `paths.py:42-51`,
  `startup.py:10-34`, `backend/orchestration/internal/config/config.go:61-66`,
  `CODEX-REPO-MANIFEST.json:6`, `README.md:1`, `DATA-HANDLING.md:143-152`.
- Token merge: `scanner.py:20-194`, `storage.py:17-200`, `models.py:7-21`,
  `aggregation.py:46-93`, `config.py:14-76`; Claude data
  `C:\Users\gregs\.claude\projects\<slug>\*.jsonl` (`type:"assistant"` →
  `message.usage`).
- Speed: `ui.py:_poll_hotkey` 1611-1614, `toggle_overlay` 2227-2233,
  `show_overlay` 2235-2240, `refresh_data` 1671-1749, `schedule_ingest` 1641-1669;
  `storage.py:load_events_since` 156-200 (unindexed `event_timestamp` scan).
- Live config confirming hotkey: `%LOCALAPPDATA%\CodexDashboard\config.json`
  (`"hotkey": "Ctrl+Alt+Space"`).

## Audit Status

Coordinator-reviewed for concreteness, awaiting human approval. Under the
TaskCreate model adopted 2026-05-29 there is no separate agent-auditor lane; the
coordinator (keeper of common sense and human intention) reviews the draft and
increases concreteness without narrowing scope. This review pinned the previously
ambiguous mechanisms — Objective 2's per-source idempotency key
(`UNIQUE(source, source_event_id)`), the single canonical per-request usage rule
(last assistant event per `requestId`), and the canonical Claude `total_tokens`
formula (cache-creation counted as input) — and Objective 3's cold-start
"populated, not blank" requirement. All three objectives were preserved; nothing
was narrowed. See [TASK-CREATE-COORDINATOR-NOTES.md](./TASK-CREATE-COORDINATOR-NOTES.md).
