# Activation Latency Follow-Up Investigation (Task-0013, Objective 3)

Date: 2026-05-29
Scope: the human reports that pressing the global hotkey still feels "clunky"
and that the time from key press to a fully rendered window is roughly ~500 ms
(revised up from >250 ms). The prior Objective-3 proof measured only the
synchronous UI-thread DB read (~5 ms after the fix), which is NOT the
end-to-end perceived latency. This report explains the full pipeline, measures
the end-to-end wall clock by phase on a large task-owned synthetic DB, explains
why the prior proof missed it, and postulates concrete fixes. It does NOT
implement a fix (the human asked to investigate + postulate + report first).

## TL;DR — Root cause and postulated fix

**Root cause (measured):** activation perceived latency is dominated by
**re-reading and re-aggregating a full 7-day window of token events on every
activation**, even though the chart only displays 20 buckets (5 hours at the
default `15m` interval). On a ~270 MB-class DB (1.4M rows, ~467k rows inside the
7-day window) the two per-event passes cost, by phase median:

- **cold activation** (no snapshot yet): ~1.2 s SQLite read + materialization,
  then ~0.23 s re-aggregation render → ~1.5 s end to end.
- **warm activation** (background poll already loaded the snapshot — the common
  steady-state case): no DB read, but `_render_dashboard` still re-runs
  `filter_events_by_source` + `build_buckets` over the full ~467k-event 7-day
  snapshot → **~0.33 s of work after the keypress**, plus ~0.08 s for the OS to
  actually map/paint the window and ~0.025 s average hotkey-poll latency →
  **~0.43 s perceived**, which matches the human's ~500 ms report.

The prior ~5 ms proof was correct but narrow: it measured only the UI-thread
*DB block* removed by Objective 3. It did not measure the **render
re-aggregation**, the **window map/first paint**, or the **50 ms hotkey poll
interval** — the three things that actually make activation feel clunky now.

**Postulated primary fix:** stop re-aggregating the whole 7-day window on every
activation. Aggregate incrementally in the background poll and cache the
display-ready buckets + summary, so activation does only `deiconify` + paint +
label set (tens of ms). Secondary: tighten the hotkey poll interval (50 → ~10
ms) to shave the average ~25 ms detection lag. Details and tradeoffs in
[§4](#4-postulated-fixes).

## 1. What the activation pipeline actually does (plain terms + real code)

From key press to a visible, painted window:

1. **Hotkey is registered on a dedicated Win32 thread.** At startup the app
   calls `GlobalHotkey.register()`
   ([`app/codex_dashboard/ui.py:374-377`](../../../app/codex_dashboard/ui.py)),
   which spawns a thread running a `GetMessageW` loop
   ([`app/codex_dashboard/hotkey.py:104-138`](../../../app/codex_dashboard/hotkey.py)).
   When Windows delivers `WM_HOTKEY`, that thread immediately pushes a token
   onto a `SimpleQueue` ([`hotkey.py:128-129`](../../../app/codex_dashboard/hotkey.py)).
   It does **not** call the toggle callback directly.

2. **The Tk UI thread polls that queue every 50 ms.** `_poll_hotkey`
   ([`ui.py:1622-1625`](../../../app/codex_dashboard/ui.py)) calls
   `self.hotkey.poll()` and then re-arms itself with
   `self.root.after(50, self._poll_hotkey)`. `poll()` drains the queue and
   invokes the callback ([`hotkey.py:140-146`](../../../app/codex_dashboard/hotkey.py)).
   **Consequence:** a keypress that arrives just after a poll tick waits up to
   the full 50 ms (≈25 ms on average) before `toggle_overlay` even runs. This is
   pure added latency that no DB or render optimization can remove.

3. **`toggle_overlay` → `show_overlay`.**
   `toggle_overlay` ([`ui.py:2370-2376`](../../../app/codex_dashboard/ui.py))
   calls `show_overlay` ([`ui.py:2378-2394`](../../../app/codex_dashboard/ui.py)).

4. **`show_overlay` makes the window visible, then renders.** It calls
   `self.overlay.deiconify()`, `self.overlay.lift()`,
   `self.overlay.focus_force()` ([`ui.py:2384-2387`](../../../app/codex_dashboard/ui.py)),
   then:
   - **warm path:** if `self.latest_events` is populated (the background poll
     keeps it current), it calls `_render_dashboard(self.latest_events, ...)`
     immediately on the UI thread ([`ui.py:2388-2392`](../../../app/codex_dashboard/ui.py)).
   - **cold path:** if there is no snapshot yet, it calls
     `_start_activation_load()` ([`ui.py:2394`](../../../app/codex_dashboard/ui.py)),
     which runs the SQLite read on a worker thread
     ([`ui.py:1664-1683`](../../../app/codex_dashboard/ui.py)) and renders when
     the result arrives via `_poll_ingest_results`
     ([`ui.py:1648-1654`](../../../app/codex_dashboard/ui.py)).

5. **The window is actually mapped and painted.** `deiconify` only *requests*
   the state change; the window is not on screen until Tk processes its event
   queue and the OS maps and composites it. In normal operation that happens on
   the next Tk idle cycle, after `show_overlay` returns. This OS map/first-paint
   step is real wall-clock time and was never measured before.

6. **`_render_dashboard` does the heavy per-event work**
   ([`ui.py:1747-1879`](../../../app/codex_dashboard/ui.py)):
   - `filter_events_by_source(events, self.selected_sources)` — iterates **every
     event in the snapshot** ([`ui.py:1767`](../../../app/codex_dashboard/ui.py),
     `aggregation.py:32-49`).
   - `build_buckets(events, ...)` — iterates **every event** again, computing
     `_align_bucket_start(event.event_timestamp.astimezone(tz))` per event
     ([`ui.py:1769-1776`](../../../app/codex_dashboard/ui.py),
     `aggregation.py:107-109`). Called a second time if `metric_mode != "total"`.
   - `total_7d = sum(event.total_tokens for event in events if ...)` — another
     full pass ([`ui.py:1788-1790`](../../../app/codex_dashboard/ui.py)).
   - label/`configure` updates and `draw_chart` — `draw_chart` only draws the
     ~20 chart bars and is cheap ([`ui.py:1900-2025+`](../../../app/codex_dashboard/ui.py)).
   The expensive part scales with **events in the window (~467k)**, not with the
   ~20 bars shown.

7. **Why the window loads a full 7-day window for a 5-hour chart.** The cold
   load and the snapshot both come from `_load_dashboard_data`
   ([`ui.py:1715-1741`](../../../app/codex_dashboard/ui.py)) using
   `usage_history_lookback` ([`ui.py:208-213`](../../../app/codex_dashboard/ui.py)),
   which is `max(7 days, interval × bucket_count)`. At the default `15m`
   interval (set at [`ui.py:307`](../../../app/codex_dashboard/ui.py)) with 20
   buckets that is `max(7d, 5h)` = **7 days**. So the activation materializes and
   loops ~467k events to draw 20 bars covering only the most recent 5 hours.

## 2. Measured end-to-end wall clock by phase

Instrumentation: a new task-owned harness,
[`activation_e2e_harness.py`](./activation_e2e_harness.py), drives the **real Tk
`DashboardApp` in-process** against a task-owned synthetic SQLite DB, times each
production phase with `time.perf_counter`, and forces the OS to actually paint
with `update_idletasks()` + `update()`. All Tk work runs on the main
(UI-equivalent) thread, exactly like production. The harness does **not** open
the human's live `dashboard.db`, live config, `C:\Users\gregs\.codex`, or
`~/.claude`; it seeds and measures its own DB and uses a distinct hotkey so it
does not contend with the human's running overlay. Full machine-readable output:
[`E2E-TIMING-RESULT.json`](./E2E-TIMING-RESULT.json).

Synthetic DB: **1,400,000 events, 294.2 MB** (≈ the human's ~270 MB-class live
DB), 50/50 codex/claude across 21 days, so the 7-day activation window holds
~467k events (older 2/3 sit outside the window, like a long-running real DB).
7 measured iterations after a warmup. Phase medians (ms):

| Phase | What it is | Median (ms) | Notes |
| --- | --- | ---: | --- |
| hotkey poll latency | wait for next 50 ms `_poll_hotkey` tick | ~25 | 0–50 ms; not in-process measurable, derived from the 50 ms `root.after` interval |
| `db_read_ms` | `_load_dashboard_data` (SQLite read + build TokenEvents) | **1202** | **cold path only**; warm path skips this |
| `deiconify_lift_focus_ms` | `deiconify` + `lift` + `focus_force` calls | 11 | request only |
| `first_paint_ms` | OS maps + composites the just-shown window | 76 | real, previously unmeasured |
| `render_dashboard_ms` | `filter_events_by_source` + `build_buckets` + labels + `draw_chart` | **230** | **both paths**; scales with ~467k events |
| `render_paint_ms` | flush the chart redraw to screen | 3 | cheap |
| `show_to_painted_ms` | everything after the DB read (warm-path cost) | **326** | deiconify + first paint + render + render paint |
| `end_to_end_ms` | full cold activation incl. DB read | **1503** | cold path |

### Perceived latency the human feels

- **Warm activation (common steady state — background poll has a snapshot):**
  `show_to_painted_ms` ≈ **326 ms** + ~25 ms average poll latency ≈ **~350 ms**.
  The dominant contributor is `render_dashboard_ms` (~230 ms), then
  `first_paint_ms` (~76 ms).
- **Cold activation (first hotkey after launch, before any background poll
  completes):** `end_to_end_ms` ≈ **1.5 s** + ~25 ms poll latency. Dominated by
  `db_read_ms` (~1.2 s).

The human's "~500 ms, clunky" is fully consistent with the warm path on a large
DB (≈350–400 ms here, and the human's machine/DB state can push it past 500 ms,
e.g. cold WAL, contended desktop, or a slightly larger window). The clunkiness is
**not** the ~5 ms UI-thread DB block that the prior proof removed — it is the
**render re-aggregation + window first-paint + poll lag** that the prior proof
never looked at.

For comparison the prior narrow harness ([`TIMING-RESULT.json`](./TIMING-RESULT.json),
250k events / 49 MB) measured the in-memory snapshot filter at ~5 ms — but that
harness deliberately timed only `filter_events_by_source` + a `sum`, **not**
`build_buckets` or the paint, so it did not surface the ~230 ms render cost that
this end-to-end harness exposes on the real render path.

## 3. Why the prior ~5 ms proof did not capture this

The prior Objective-3 budget and proof targeted exactly one thing: removing the
**synchronous full-table SQLite read from the Tk UI thread**. That fix is real
and correct — `show_overlay` no longer blocks the UI thread on
`load_events_since` (340 ms → off-thread). But the ~5 ms number is a **proxy**,
not the perceived latency, for four reasons:

1. **It measured a different operation.** The prior harness
   ([`activation_timing_harness.py`](./activation_timing_harness.py),
   `time_snapshot_render`) timed only `filter_events_by_source` + a `sum`
   comprehension — *not* `build_buckets`, which is the ~230 ms per-event pass
   that `_render_dashboard` actually runs on the UI thread on every activation.
2. **It ignored the OS window map/first paint.** `deiconify`/`lift`/`focus` plus
   the OS compositing the `overrideredirect` topmost window is ~85 ms here and
   was never in the budget.
3. **It ignored hotkey poll latency.** The 50 ms `_poll_hotkey` interval adds
   0–50 ms (~25 ms average) between the physical keypress and `toggle_overlay`.
   A UI-thread-block measurement cannot see this — it starts the clock after the
   callback already fired.
4. **It ignored the cold path.** On the first activation after launch (no
   snapshot yet), `show_overlay` takes the `_start_activation_load` branch and
   the user waits ~1.2 s for the off-thread DB read before any data paints.
   Off-thread means the UI thread is not *blocked*, but the **window still has no
   data until the read finishes**, so the perceived "rendered window" latency is
   the full ~1.5 s, not 5 ms.

In short: the prior proof correctly closed its written budget (UI-thread block),
but the written budget was the wrong target for the human-facing outcome
("feels instant"). The human is measuring keypress → fully rendered window; the
prior proof measured one slice in the middle.

## 4. Postulated fixes

Not implemented in this run. Targeted at the dominant phases identified above.

### Fix A (primary) — precompute display buckets in the background; stop re-aggregating on activation

**Idea:** move the per-event aggregation out of the activation path. The
background ingest poll (`schedule_ingest` → `_poll_ingest_results` →
`refresh_data`, [`ui.py:1685-1745`](../../../app/codex_dashboard/ui.py)) already
runs off the UI thread on a timer. Have it compute and cache the display-ready
artifacts for the current interval/metric/source selection: the `raw_buckets`,
`display_buckets`, `total_7d`, projection inputs, and repo stacks. Then
`_render_dashboard` on activation becomes: read the cached buckets, set labels,
`draw_chart` (~20 bars). That removes essentially all of the ~230 ms
`render_dashboard_ms` per-event work from the keypress path.

- **Expected improvement:** warm-path `render_dashboard_ms` ~230 ms → low tens of
  ms (label set + 20-bar draw). Warm perceived latency ~350 ms → **~100–130 ms**
  (deiconify + first paint + cheap render + poll).
- **Tradeoffs / risks:** the cache must be invalidated/recomputed when the user
  changes interval, metric mode, chart mode, or the **source filter**. Objective
  4 requires the source filter to re-render from the in-memory snapshot without a
  DB read; if buckets are precomputed only for "all sources", a filter toggle
  must either recompute buckets on the UI thread (back to ~230 ms) or precompute
  per-source bucket sets and combine them (cheap). Recommended: precompute
  per-source bucket arrays in the background and sum the selected subset at
  render (combining ~20-bucket arrays is trivial). Memory cost is negligible
  (2 sources × ~20 buckets).

### Fix B (primary, complements A) — only load/aggregate the window the chart shows

**Idea:** the activation loads a full 7-day window
(`usage_history_lookback = max(7d, interval×buckets)`,
[`ui.py:208-213`](../../../app/codex_dashboard/ui.py)) but the default `15m`
chart shows only the last 5 hours. The 7-day span is needed for the `total_7d`
summary and the weekly-budget projection, but **not** for the chart buckets.
Split the two: keep a cheap rolling 7-day **aggregate** (sum + latest advisory)
updated incrementally by the poll, and load only the chart window's events
(5 hours ≈ a few thousand rows) for bucketing.

- **Expected improvement:** cold `db_read_ms` ~1.2 s → tens of ms (a 5-hour
  indexed range vs. a 467k-row 7-day scan), and `render_dashboard_ms` per-event
  work shrinks proportionally because `build_buckets` then loops thousands of
  events, not ~467k.
- **Tradeoffs / risks:** requires maintaining the 7-day rolling total separately
  (incremental sum in the ingest poll, or a cheap indexed `SUM()` query). The
  `total_tokens >= summary_since` pass ([`ui.py:1788-1790`](../../../app/codex_dashboard/ui.py))
  and the advisory lookup must move to that aggregate. Care needed so the
  7-day total stays correct across the WAL/poll boundary. Combine with Fix A:
  the poll computes both the rolling total and the per-source chart buckets.

### Fix C (secondary) — tighten the hotkey poll interval

**Idea:** change `_poll_hotkey`'s `root.after(50, ...)`
([`ui.py:1625`](../../../app/codex_dashboard/ui.py)) to ~10 ms, or deliver the
callback without polling (e.g. have the Win32 thread post a Tk event /
`event_generate` so the toggle runs on the next idle cycle instead of waiting for
a poll tick).

- **Expected improvement:** average detection latency ~25 ms → ~5 ms (10 ms
  poll) or near-zero (event-driven). Small but it is pure, always-present lag.
- **Tradeoffs / risks:** a 10 ms `after` loop wakes the UI thread 100×/s even
  when idle (minor CPU/battery cost). The event-driven approach is cleaner but
  needs a thread-safe Tk handoff (Tk is not thread-safe; `event_generate` from a
  foreign thread works on Win32 but should be validated). Low risk, modest gain;
  do it after A/B.

### Fix D (secondary) — make the empty window appear instantly, fill in async (cold path)

**Idea:** on the cold path, paint the window chrome immediately (it already
shows a "No token data yet." state, [`ui.py:1932-1940`](../../../app/codex_dashboard/ui.py))
and let the off-thread load fill the chart when ready, with a visible "loading"
affordance. This does not make the data appear faster but makes the **window**
appear instantly so it does not feel frozen for ~1.2 s on first activation.

- **Expected improvement:** cold-path *window* latency ~1.5 s → ~100 ms; data
  still arrives at ~1.2 s but on a window that is already up.
- **Tradeoffs / risks:** mostly perceptual; needs a clear loading state so the
  human does not read a blank chart as "broken". Largely moot if Fix B cuts the
  cold read to tens of ms.

### Recommended sequence

A + B together attack the dominant `render_dashboard_ms` (~230 ms, both paths)
and `db_read_ms` (~1.2 s, cold path) and should bring warm perceived latency from
~350 ms to roughly ~100–130 ms (dominated then by the unavoidable OS first-paint
~76 ms). Add C for the last ~20 ms of poll lag. D is optional polish for the very
first activation once B has shrunk the cold read.

## Data-handling / lane compliance

- All measurement used a **task-owned synthetic SQLite DB** under
  `Tracking/Task-0013/Testing/Runtime/` (gitignored; not staged). The human's
  live `dashboard.db`, live `config.json`, `C:\Users\gregs\.codex`, and
  `~/.claude` were **not** opened, read, or sized. The synthetic DB was sized to
  the ~270 MB-class state the human described, per
  `HUMAN-DIRECTIVES-FOR-WORKER.md` and repo `REGRESSION.md`/`DATA-HANDLING.md`.
- The harness registers a **distinct** hotkey (`Ctrl+Alt+Shift+F24`) so it does
  not contend with the human's running `Ctrl+Alt+Space` overlay.
- No product code was changed in this investigation run (the human asked to
  investigate + postulate + report first). Only this report and the new
  `activation_e2e_harness.py` were added.

## Artifacts

- This report: `ACTIVATION-LATENCY-INVESTIGATION.md`
- New end-to-end harness: [`activation_e2e_harness.py`](./activation_e2e_harness.py)
- Measured result (294 MB synthetic DB): [`E2E-TIMING-RESULT.json`](./E2E-TIMING-RESULT.json)
- Prior narrow proof (for contrast): [`activation_timing_harness.py`](./activation_timing_harness.py),
  [`TIMING-RESULT.json`](./TIMING-RESULT.json)
