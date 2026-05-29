# Task-0013 Handoff

## Current Baseline (2026-05-29, updated — show/hide fix DEPLOYED LIVE)

Task-0013 ("Rebrand to Obsidian, merge Claude Code tokens, fast hotkey
activation, source filter") shipped all four objectives. After deploy the human
reported the hotkey still felt clunky (~350-500 ms perceived). The investigation
([Testing/ACTIVATION-LATENCY-INVESTIGATION.md](./Testing/ACTIVATION-LATENCY-INVESTIGATION.md))
found the cause and the human authorized the fix (show/hide only, no rebuild on
toggle). **PASS-0002 implemented that fix in the repo source (commit `247f2973`),
with unit proof and a measured before/after.** The committed source no longer
renders on the hotkey path.

**The show/hide fix is now LIVE.** The human authorized "Publish + restart now"
for commit `247f297`, and this run published pinned release
**`20260529T161636Z-247f2973b2a4`** from the committed tree and restarted the
human's overlay onto it (new PID 64592, on the LIVE config, data preserved). See
"Publish + Restart deploy (show/hide fix)" below and
[Testing/ReleaseProof/RELEASE-DEPLOY-PROOF.md](./Testing/ReleaseProof/RELEASE-DEPLOY-PROOF.md).
The only remaining step is the human-acceptance gate.

## Publish + Restart deploy (show/hide fix, 2026-05-29 — DONE)

The human authorized "Publish + restart now" for the show/hide fix (commit
`247f297`). This run executed the canonical dashboard release-isolation flow
(repo `TESTING.md` SMOKE-003, `DATA-HANDLING.md`):

- Release-script unit coverage: `python -m unittest
  tests.test_dashboard_release_scripts tests.test_desktop_support -v` -> 44 OK.
- Published pinned release **`20260529T161636Z-247f2973b2a4`** from the committed
  Git tree (`source_mode=git_commit`, `source_dirty=false`, commit `247f2973`);
  21 `app/` files, the unrelated untracked `token_time.py` and other dirty files
  did NOT ship (`contains_token_time=false`).
- Stopped only the prior-release overlay (PID 67656, release
  `20260529T143554Z-e99ac895ee61`) and restarted via `Start-DashboardRelease.ps1`
  onto the new release (new PID **64592**).
- `Test-DashboardRelease.ps1`: `current_release_error=null`, all 21 hashes
  validate, `startup_uses_pinned_launcher=true`, `running_process_count=1`, the
  running process points at the new release id + release root.
- LIVE config (no isolated-config override on the running process command line),
  `config.json` byte-identical (sha `D1BF7990…`), startup `.cmd` content-identical
  (sha `BA8F5401…`), `dashboard.db` the same file at the same path (creation
  `2026-04-03` unchanged; not migrated/reset/overwritten).
- Human-surface proof from the new pinned release root (isolated fixtures, no live
  data read): OBSIDIAN brand, merged 138.5M total, Source filter All->Codex
  (138.5M -> 137.7M), expanded Codex/Claude checkboxes. Toggle latency itself is
  already proven by the committed e2e harness and was not re-measured live.
- Full evidence: [Testing/ReleaseProof/RELEASE-DEPLOY-PROOF.md](./Testing/ReleaseProof/RELEASE-DEPLOY-PROOF.md).

## Activation fix (PASS-0002, 2026-05-29 — IMPLEMENTED + PROVEN, DEPLOYED LIVE)

What changed (Task-0013 source only):
- `app/codex_dashboard/ui.py`: `show_overlay()` toggles VISIBILITY ONLY
  (deiconify/lift/focus); the render was removed from the hotkey path. Startup
  pre-render moved off the UI thread (`__init__` → `_start_activation_load`). The
  background poll keeps the persistent overlay current.
- Fix B (cheap background freshness): `_load_dashboard_data()` loads only the
  charted window + an indexed per-source 7-day `SUM` + an indexed advisory
  lookback; `_render_dashboard()` derives the 7-day total from precomputed
  per-source totals so the Objective-4 source filter still adjusts it in memory
  with no DB read.
- `app/codex_dashboard/storage.py`: `sum_total_tokens_by_source_since`,
  `load_latest_weekly_advisory`, a covering index
  `idx_token_events_ts_source_total`, and a partial advisory index.

Proof:
- Unit suite: `python -m unittest discover -s tests -p "test_*.py" -v` → 140 OK.
  Toggle-does-no-aggregation/DB/render and Fix-B coverage in
  `tests/test_desktop_support.py` + `tests/test_task0013_obsidian.py`.
- Measured before/after on a 423.8 MB / 1.4M-row task-owned synthetic DB
  ([Testing/E2E-TIMING-RESULT.json](./Testing/E2E-TIMING-RESULT.json),
  [Testing/ACTIVATION-FIX-PROOF.md](./Testing/ACTIVATION-FIX-PROOF.md)):
  BEFORE show→painted ~243 ms (render ~156 ms) vs AFTER ~88 ms (deiconify/focus
  ~12 ms + OS paint ~77 ms, NO render). Fix-B background poll ~64 ms (was ~1.4 s).

Remaining for full closure: this was the live-lane publish/restart gate, and it is
now DONE (see "Publish + Restart deploy (show/hide fix)" above). The human's
overlay runs the show/hide fix on release `20260529T161636Z-247f2973b2a4`. The
only open item is the human-acceptance gate (the coordinator owns presenting it).

## Prior baseline (Objectives 1-4 shipped + first deploy, 2026-05-29)

The four objectives were implemented, committed/pushed, and PUBLISHED + RESTARTED
on the human's pinned dashboard release (see the deploy note below). That deploy
predates the PASS-0002 activation fix above; the live overlay still runs the older
render-on-show activation until a new release is published.

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
