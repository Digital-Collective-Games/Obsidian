# Activation Fix Proof — show/hide only, no rebuild on toggle (Task-0013, Objective 3)

Date: 2026-05-29
Scope: IMPLEMENTS the human-authorized fix from
[`../HUMAN-DIRECTIVES-FOR-WORKER.md`](../HUMAN-DIRECTIVES-FOR-WORKER.md)
("2026-05-29 — Activation fix approach: show/hide only, no rebuild on toggle").
The prior investigation
([`ACTIVATION-LATENCY-INVESTIGATION.md`](./ACTIVATION-LATENCY-INVESTIGATION.md))
found the perceived clunkiness was dominated by re-aggregating the full 7-day
window on EVERY activation. This run removes that work from the hotkey path.

## What changed (product code, Task-0013 only)

### 1. Hotkey toggles visibility only (primary, non-negotiable)

`app/codex_dashboard/ui.py`:

- `show_overlay()` now does ONLY `deiconify()` + `lift()` + `focus_force()`. The
  render call was removed from the hotkey path. The overlay is a PERSISTENT window
  built once and kept current by the background poll, so a toggle does NO
  re-aggregation, NO bucket rebuild, NO DB read, and NO full re-render — it only
  reveals the already-rendered window.
- `hide_overlay()` is unchanged (withdraw the window).
- Startup pre-render: `__init__` schedules `self.root.after(100, self._start_activation_load)`
  (was `refresh_data`, a synchronous UI-thread DB read). The initial load now runs
  OFF the UI thread and renders the withdrawn overlay via the ingest queue, so the
  FIRST hotkey press is already fast and startup itself does not block on the big
  DB read.

### 2. Background freshness is cheap (Fix B; secondary, in service of #1)

The background ingest poll keeps the persistent overlay current. To stop that poll
from re-aggregating the full ~467k-event 7-day window every cycle:

- `_load_dashboard_data()` now loads ONLY the charted window's events
  (`interval × bucket_count`, e.g. 5 hours for the default `15m`×20), not 7 days,
  so bucketing loops a few thousand events instead of hundreds of thousands.
- The rolling 7-day total is computed by an indexed SQL `SUM(...) GROUP BY source`
  (`storage.sum_total_tokens_by_source_since`) — no per-event materialization. The
  latest weekly advisory is fetched by a cheap indexed lookback
  (`storage.load_latest_weekly_advisory`).
- `_render_dashboard()` derives the displayed 7-day total by summing the SELECTED
  sources' precomputed per-source totals. The Objective-4 source filter therefore
  still adjusts the displayed total **in memory** (drop a source's precomputed
  total) with NO DB read, preserving the non-blocking contract.
- `storage.initialize_db` adds a covering index
  `idx_token_events_ts_source_total (event_timestamp, source, total_tokens)` (and a
  partial advisory index). The per-source SUM is forced onto that covering index
  (`INDEXED BY`) because the planner otherwise full-scans via the source index.

### Objective-4 source filter preserved

The source filter remains a deliberate USER action that re-renders from the
in-memory snapshot via `_toggle_source` → `_render_dashboard(latest_events, ...)`.
It performs no synchronous UI-thread DB read (unit-proven), separate from the
hotkey show/hide path.

## Measured before/after (extended end-to-end harness)

Harness: [`activation_e2e_harness.py`](./activation_e2e_harness.py), driving the
REAL Tk `DashboardApp` in-process against a task-owned synthetic SQLite DB.
Full machine-readable output: [`E2E-TIMING-RESULT.json`](./E2E-TIMING-RESULT.json).

Synthetic DB: **1,400,000 events, 423.8 MB** (well above the human's ~270 MB-class
live DB), 50/50 codex/claude across 21 days; the 7-day window holds ~189k events.
9 measured iterations after a warmup, all Tk work on the main (UI-equivalent)
thread, real paints forced with `update_idletasks()+update()`.

| Path | What it is | Median (ms) | Notes |
| --- | --- | ---: | --- |
| **BEFORE** show→painted | old render-on-show warm path | **242.8** | incl. `render_dashboard` over the full 7-day window |
| BEFORE `render_dashboard` | full-window filter + build_buckets + labels | 155.8 | the per-event work removed from the toggle path |
| **AFTER** show→painted | real `show_overlay()` (show/hide only) | **87.9** | `show_call` 12.0 + OS `show_paint` 76.6, **NO render** |
| AFTER `show_call` | deiconify + lift + focus_force | 12.0 | request only |
| AFTER `show_paint` | OS maps + composites the window | 76.6 | the dominant residual — pure OS window map/paint |
| AFTER toggle round-trip | show + paint + hide + paint | 95.7 | full hide↔show cycle |
| **POLL (Fix B)** total | `_load_dashboard_data` + one render | **64.3** | background, off the hotkey path |
| POLL `load_dashboard_data` | chart-window load + indexed 7-day SUM | 62.5 | was ~1.4 s full-window materialization before |
| POLL `render_dashboard` | render the cheap snapshot | 1.6 | cheap |

Add the ~25 ms average hotkey poll latency to BOTH paths for perceived latency:
BEFORE ≈ **268 ms**, AFTER ≈ **113 ms** (speedup ~2.76x on this run).

### Why this is the right proof

- The directive's target was "toggle latency dominated only by OS window map/paint
  + hotkey detection, no per-event work (~tens of ms vs the measured ~350 ms warm
  path)." The AFTER toggle is exactly that: `show_call` ~12 ms + OS `show_paint`
  ~77 ms, with **zero** `_render_dashboard` on the toggle path. The residual is the
  unavoidable OS window compositing, not per-event aggregation.
- The earlier investigation measured the old warm path at ~350 ms (render ~230 ms);
  this BEFORE column (~243 ms, render ~156 ms) reproduces the same structure. The
  absolute render number varies with machine contention, but the structural change
  — no render on toggle — is what removes the clunkiness, and it is also unit-proven
  (see below) independent of timing.
- Fix B cut the background poll's DB work from ~1.4 s (full-window materialization)
  to ~63 ms (chart-window load + an indexed covering-index SUM), so keeping the
  overlay fresh in the background is cheap and does not undermine the show/hide win.

## Unit proof (timing-independent)

`python -m unittest discover -s tests -p "test_*.py" -v` → **140 tests, OK**.

Activation-fix-specific cases:

- `tests/test_desktop_support.py`
  - `test_show_overlay_only_reveals_persistent_window_no_rebuild`: the hotkey
    `show_overlay` does NOT call `_render_dashboard`, `_load_dashboard_data`,
    `refresh_data`, or `_start_activation_load`; it only `deiconify/lift/focus`.
  - `test_show_overlay_cold_snapshot_still_only_reveals_no_load`: even with an empty
    snapshot the toggle only reveals (no load/render dispatched on the toggle path).
- `tests/test_task0013_obsidian.py`
  - `ActivationFixStorageTests`: the per-source 7-day `SUM` excludes events before
    the window, lets the filter subtract a source in memory, uses a covering-index
    RANGE scan (not a full-table scan), and the advisory lookback returns the most
    recent advisory / `None` when absent.
  - `ActivationFixRenderPathTests`: `_load_dashboard_data` loads only the chart
    window (5 h) while summing 7 days separately; `_render_dashboard` derives the
    7-day total from precomputed per-source totals and the source filter excludes a
    source's total in memory (no DB read).
  - `SourceFilterRenderPathTests.test_toggle_source_renders_from_snapshot_without_db_read`
    (existing): the source filter re-renders from the snapshot, never calling
    `_load_dashboard_data`/`refresh_data` (Objective 4 preserved).

## Data-handling / lane compliance

- All measurement used a **task-owned synthetic SQLite DB** under
  `Tracking/Task-0013/Testing/Runtime/` (gitignored; not staged). The human's live
  `dashboard.db`, live `config.json`, `C:\Users\gregs\.codex`, and `~/.claude` were
  NOT opened, read, or sized. The synthetic DB is sized above the ~270 MB live class
  per `HUMAN-DIRECTIVES-FOR-WORKER.md` and repo `REGRESSION.md`/`DATA-HANDLING.md`.
- The harness registers a distinct hotkey (`Ctrl+Alt+Shift+F24`) so it does not
  contend with the human's `Ctrl+Alt+Space` overlay.
- This run did NOT publish or restart the human's live overlay (that live-lane step
  is gated separately).

## Real app-surface smoke (isolated lane)

The real Tk `DashboardApp` was launched via `python -m app.codex_dashboard
--smoke-artifact-dir ... --smoke-tab usage` against the task-owned 423.8 MB
synthetic DB, on an isolated config with a distinct hotkey (`Ctrl+Alt+Shift+F24`,
no contention with the human's `Ctrl+Alt+Space`). The overlay launched, the
`show_overlay()` (show/hide-only) path revealed it, and the captured surface shows
OBSIDIAN, the `Source: All` filter control, and `7d_total=662.3M` derived from the
Fix-B precomputed per-source totals (codex 331.5M + claude 331.4M). This confirms
the show/hide path + Fix-B render correctly on the real app surface.

- [activation-fix-smoke/overlay.png](./activation-fix-smoke/overlay.png)
- [activation-fix-smoke/overlay-summary.txt](./activation-fix-smoke/overlay-summary.txt)

This is supporting app-surface evidence; the full REG-001/REG-005 closure run and
the live publish/restart remain the gated next step.

## Artifacts

- This proof: `ACTIVATION-FIX-PROOF.md`
- Investigation (root cause + postulated fixes): [`ACTIVATION-LATENCY-INVESTIGATION.md`](./ACTIVATION-LATENCY-INVESTIGATION.md)
- Extended harness: [`activation_e2e_harness.py`](./activation_e2e_harness.py)
- Measured result (423.8 MB synthetic DB): [`E2E-TIMING-RESULT.json`](./E2E-TIMING-RESULT.json)
