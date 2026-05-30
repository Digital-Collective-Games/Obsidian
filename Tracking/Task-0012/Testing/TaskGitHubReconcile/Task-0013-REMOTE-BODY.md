<!-- task-sync: repo=CodexDashboard; task_id=Task-0013; task_path=Tracking/Task-0013/TASK.md -->

# Task-0013: Rebrand the desktop overlay to Obsidian, count Claude Code tokens alongside Codex, and make hotkey activation feel instant.

## Source Of Truth

Local `Tracking/Task-0013/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0013:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

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

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `13`
- Local task path: `Tracking/Task-0013/TASK.md`
- Source commit: `8fb77523df224eabf2d833a91a8b4bd230a0796f`
- Local task SHA-256: `3B2CC5E05EDB8D9351E100DB525B2D9C0664DEF8B625C8CEDF0A3B4DE1A221F9`
- Rendered at: `2026-05-29T13:36:49.8984068-04:00`