# Task 0014

## Title

Make the Obsidian overlay window tab-aware: move it up on every tab, and make the Jobs and Tasks tabs fill the usable height above the taskbar (configurable padding, taskbar never covered).

## Writeup Type

Concrete implementation task.

The solution shape is already chosen and pinned by the human's clarifying answers
(`HUMAN-DIRECTIVES-FOR-WORKER.md`, three AUTHORITATIVE answers on 2026-05-29): make
the overlay window geometry a pure function of the active tab plus the Windows work
area, recomputed on every tab change, with a configurable vertical padding fraction.
The exact geometry formula, the configurable padding, the work-area source, and the
"never cover the taskbar" constraint are all fixed by the human; this task documents
and implements them. It is NOT a consensus or research task — the files and seams are
named below and the math is specified to the line. Do not downgrade the writeup type
and do not re-open the pinned decisions.

## Summary

The human runs an always-on-top desktop overlay ("Obsidian"), toggled by a global
hotkey, to watch agent token usage, jobs, and tasks at a glance. Today the overlay
opens as a fixed `980×660` panel in the upper-right and never changes size or position
when the operator switches tabs. Two things degrade the long-list tabs:

1. The window sits lower than the human wants on every tab, and its position is
   computed from full-screen dimensions that include the taskbar.
2. The Jobs and Tasks tabs show long lists/tables but are stuck at the same short
   660px height as the Usage tab, so only a few rows are visible at once.

The intended outcome: switching tabs re-lays-out the same persistent window so it sits
near the top of the monitor on every tab, and on the Jobs and Tasks tabs it grows to
fill the usable vertical space (the area above the taskbar) minus a small, configurable
top/bottom padding — without ever covering the Windows taskbar. The Usage tab keeps its
current size but is also moved up.

## Who Is Affected

The single human operator who runs this overlay on their own Windows machine
(`admin@digitalcollective.games`). There are no other users and there is no remote
deployment of the desktop app.

## Human-Facing Outcome (state first)

After this task, when the operator triggers the overlay and clicks between tabs:

- On every tab the window sits near the top of the monitor (its top edge is one
  `pad` below the top of the usable work area), not in the lower-right.
- On the Jobs and Tasks tabs the window is tall: it fills the usable height (work
  area, taskbar excluded) minus `pad` at the top and `pad` at the bottom, so many more
  list rows are visible at once. The window width is unchanged (still 980px) and it
  stays right-aligned.
- On the Usage tab the window is its current size (`980×660` clamp), just moved up to
  the same top position.
- The Windows taskbar is never covered on any tab: a `pad`-sized gap always remains
  between the bottom of the window and the taskbar.
- The padding amount is configurable (a fraction of full screen height, default 5%).

## Current Truth

All line references are in `app/codex_dashboard/` unless noted, confirmed by reading
the files on 2026-05-29.

### The window is positioned once, from full-screen dimensions including the taskbar

- The overlay is a persistent `tk.Toplevel`, created withdrawn, `overrideredirect(True)`,
  `-topmost True`. Its geometry is computed ONCE by `_overlay_geometry()` and applied
  ONCE at construction: `ui.py:381` calls `self.overlay.geometry(self._overlay_geometry())`.
  It is never recomputed afterward.
- `_overlay_geometry()` (`ui.py:409-420`) uses full-screen dimensions that INCLUDE the
  taskbar:
  - `screen_width = self.root.winfo_screenwidth()` (`ui.py:414`)
  - `screen_height = self.root.winfo_screenheight()` (`ui.py:415`)
  - `width = min(980, max(860, screen_width - 80))` (`ui.py:416`)
  - `height = min(660, max(620, screen_height - 80))` (`ui.py:417`)
  - `x = min(940, max(20, screen_width - width - 40))` (`ui.py:418`)
  - `y = min(100, max(20, screen_height - height - 40))` (`ui.py:419`)
  - returns `f"{width}x{height}+{x}+{y}"` (`ui.py:420`).
  Because `winfo_screenheight()` includes the taskbar, the `y` formula can place the
  window over the taskbar region, and there is no work-area awareness at all.

### There is exactly one natural hook for "the tab changed"

- There are exactly three tabs, created by a tuple loop at `ui.py:600`:
  `("usage","Usage"), ("jobs","Jobs"), ("tasks","Tasks")` (labels rendered upper-cased),
  each `<Button-1>`-bound to `select_tab(tab_id)` at `ui.py:620`.
- The active tab is `self.active_tab`, defaulting to `"usage"` at `ui.py:313`.
- A tab click flows through `select_tab(tab_id)` (`ui.py:1081-1087`): it sets
  `self.active_tab = tab_id`, primes jobs/tasks data when relevant, then calls
  `_render_active_tab()` (`ui.py:1089-1103`), which only packs/unpacks the per-tab body
  frames. `select_tab()` is the single natural place to react to a tab change; there is
  no external event/hook today.

### The hotkey toggle must stay show/hide-only (Task-0013 behavior)

- `toggle_overlay` (`ui.py:2438-2444`) calls `show_overlay` / `hide_overlay`.
  `show_overlay` (`ui.py:2446-2464`) only `deiconify`/`lift`/`focus_force`s an
  already-rendered window — by Task-0013's explicit design it performs NO re-aggregation,
  NO bucket rebuild, NO DB read, and NO full re-render. `hide_overlay` (`ui.py:2466-2470`)
  only `withdraw`s. This task must not regress that: a tab-switch geometry re-apply must be
  a cheap `self.overlay.geometry(...)` call and must NOT trigger a data rebuild.

### There is no work-area / taskbar query anywhere, but ctypes is already in use

- No work-area / taskbar / multi-monitor query exists anywhere in `app/codex_dashboard`
  (no `SPI_GETWORKAREA`, `GetMonitorInfo`, or `MonitorFromWindow`). This is the gap this
  task fills.
- ctypes WinDLL is already used: `hotkey.py:3,6,8` (`import ctypes`,
  `from ctypes import wintypes`, `user32 = ctypes.WinDLL("user32", use_last_error=True)`).
  `ui.py:3` imports `ctypes`; `ui.py` already makes `ctypes.windll.user32` / `gdi32`
  calls elsewhere. A work-area query can reuse this pattern.

### Config has no padding field today

- `config.py` defines `DashboardConfig` (`config.py:19-37`) and round-trips each field
  through `defaults()` (`config.py:31-37`), `load_config()` (`config.py:68-86`), and
  `save_config()` (`config.py:89-95`). There is no `pad_fraction` field today.

### No test asserts geometry math

- `tests/` is run with `python -m unittest discover -s tests -p "test_*.py" -v`
  (`TESTING.md:16-17`). `tests/test_desktop_support.py:474-488` already mocks Tk
  `winfo_*` accessors for an overlay capture, establishing the pattern for mocking Tk
  geometry calls in a test. No existing test asserts overlay geometry math (a gap).

## Target Truth

- The overlay window geometry is a function of the ACTIVE TAB and the real Windows work
  area, recomputed when the tab changes and applied with the correct initial value for
  the default (`usage`) tab at construction.
- "Usable space" comes from the Windows WORK AREA (taskbar excluded), queried via
  ctypes (`SystemParametersInfo(SPI_GETWORKAREA)` for the primary monitor is acceptable),
  NOT from `winfo_screenwidth()` / `winfo_screenheight()`. Using the work-area rect
  automatically honors a taskbar docked on any edge.
- A configurable vertical padding fraction (default `0.05` = 5% of full screen height)
  lives in `DashboardConfig` and round-trips like the other fields.
  `pad = round(pad_fraction × screen_height)`.
- All tabs: top `y = work_area_top + pad`; width stays the current `980` clamp (NOT
  widened); right-aligned WITHIN the usable width (x computed from the work-area rect,
  not full screen).
- `usage` / default tab: height = current `660` clamp (size UNCHANGED), only moved up.
- `jobs` / `tasks` tabs: height = `(work_area_bottom − work_area_top) − 2 × pad`;
  bottom = `work_area_bottom − pad`, so a `pad`-sized gap always remains above the
  taskbar (the taskbar is NEVER covered).
- The tab-switch geometry re-apply is a cheap geometry call only; it does NOT rebuild,
  re-aggregate, or re-fetch tab data. The hotkey show/hide toggle path is unchanged.

## Chosen Solution Shape

One concrete implementation: factor the geometry math as a PURE, unit-testable
function, add a configurable padding fraction to config, add a ctypes work-area query,
and hook the recompute into `select_tab()` (plus the correct initial geometry at
construction).

This is the only live solution shape. The math is fully specified below; a later
implementer should not have to invent the approach.

## Implementation Home

Primary home: the Python desktop app package `app/codex_dashboard/`, specifically
`ui.py` (geometry function + the `select_tab` hook + construction-time apply + the
ctypes work-area query) and `config.py` (the `pad_fraction` field). This is correct
because window geometry, the tab-change seam (`select_tab`), and the construction-time
`geometry(...)` call all already live in `ui.py`, and the existing per-field config
round-trip lives in `config.py`. Tests live under `tests/` and the human-facing
regression case lives in repo-root `REGRESSION.md`, matching where the repo already
keeps unit tests and named regression cases.

Out of home (not changed here): backend orchestration / Temporal / jobs / tasks backend
logic; token ingest, scanner, storage, aggregation; the hotkey binding; the show/hide
toggle behavior; and any package/path renames. See `## Non-Goals`.

## Constraints And Baseline

- Python 3.13, Tkinter. Entry point `python -m app.codex_dashboard`. Tests:
  `python -m unittest discover -s tests -p "test_*.py" -v` (`TESTING.md:16-17`).
- The overlay is a single persistent `overrideredirect` `Toplevel` whose geometry is
  set with `self.overlay.geometry("WxH+X+Y")`. Re-applying geometry on a persistent
  window is cheap and does not re-render its content.
- Pinned-release deployment (critical for closure): the overlay the human actually runs
  is NOT the repo source. `scripts/Publish-DashboardRelease.ps1` snapshots `app/` into
  `%LOCALAPPDATA%\CodexDashboard\dashboard-releases\<id>`, writes a hash manifest, and
  PINS it via `%LOCALAPPDATA%\CodexDashboard\dashboard-current-release.json`. The launcher
  (`scripts/Start-CodexDashboard.ps1`) runs the pinned copy. Editing repo source does NOT
  change the running overlay: a new release must be published and the overlay restarted.
  So the live human-facing resize/reposition appears ONLY after publishing a new pinned
  release and restarting the overlay. Source edits plus passing unit tests do NOT, by
  themselves, deliver the live outcome. That publish + restart is a separately
  human-gated live-lane step, NOT part of task creation. See `## Proof Plan`.
- Data/lane isolation (`REGRESSION.md`, `DATA-HANDLING.md`, `TESTING.md`): do not point
  any validation run at the human's live config or database. Use isolated / task-owned
  fixtures, and a task-owned config + isolated SQLite for any app-surface run, per
  `TESTING.md:138-161`.

## Proposed Changes

### 1. Add a configurable padding fraction to config

- `config.py:DashboardConfig` (`config.py:19-37`): add a field
  `pad_fraction: float = 0.05` (5% of full screen height). Thread it through:
  - `defaults()` (`config.py:31-37`) keeps the `0.05` default (or sets it explicitly),
  - `load_config()` (`config.py:68-86`) reads
    `pad_fraction=float(payload.get("pad_fraction", defaults.pad_fraction))`,
  - `save_config()` (`config.py:89-95`) already serializes the whole dataclass via
    `asdict(config)`, so the new field round-trips automatically.
- The geometry computation uses `pad = round(pad_fraction × screen_height)`, where
  `screen_height` is the full screen height (so the padding fraction is "of full screen
  height" exactly as the human specified).

### 2. Add a Windows work-area query (ctypes), reusing the existing pattern

- Add a small helper (in `ui.py`, or a tiny module imported by `ui.py`) that returns the
  primary-monitor work-area rectangle as `(left, top, right, bottom)` using
  `SystemParametersInfo(SPI_GETWORKAREA)` (`SPI_GETWORKAREA = 0x0030`) via the existing
  `ctypes` user32 pattern (mirror `hotkey.py:8`'s
  `ctypes.WinDLL("user32", use_last_error=True)`). The work-area rect excludes the
  taskbar on whatever edge it is docked.
- This query is the ONLY display-dependent input to geometry. It must be injectable /
  mockable so the pure geometry function can be unit-tested without a live display (the
  function takes the work-area rect as an argument; the live caller supplies the real
  query result, tests supply a mock rect). The full-screen dimensions
  (`winfo_screenwidth/height`) remain the source of the `screen_height` used to compute
  `pad`, and are likewise passed in as arguments.

### 3. Factor a pure, tab-aware geometry function

- Add a pure function (module-level in `ui.py`, e.g. `compute_overlay_geometry(...)`)
  that takes:
  - `screen_width: int`, `screen_height: int` (full screen, from `winfo_screen*`),
  - `work_area: tuple[int,int,int,int]` = `(wa_left, wa_top, wa_right, wa_bottom)`,
  - `tab_id: str`,
  - `pad_fraction: float`,
  and returns a Tk geometry string `"WxH+X+Y"`. It contains NO Tk calls and reads NO
  global state, so it is unit-testable with mocked inputs.
- The math (pinned by the human's answers):
  - `pad = round(pad_fraction * screen_height)`
  - `usable_width = wa_right - wa_left`
  - `usable_height = wa_bottom - wa_top`
  - `width = min(980, max(860, usable_width - 80))` — the current 980 clamp, but computed
    against the usable width (do NOT widen beyond 980).
  - Height by tab:
    - `usage` (and any non-`jobs`/non-`tasks`/default tab): `height = min(660, max(620,
      usable_height - 80))` — the current 660 clamp; the Usage tab's SIZE is unchanged.
    - `jobs` or `tasks`: `height = usable_height - 2 * pad` — the tall layout. Guard
      against a misconfigured large `pad_fraction`: if `usable_height - 2*pad` would
      fall below the existing `620` minimum, clamp the height to `620` so the window
      stays usable. For any sane `pad_fraction` (including the `0.05` default), the
      canonical `usable_height - 2*pad` value applies and the bottom stays at
      `wa_bottom - pad`; the floor only triggers in degenerate configuration.
  - `x = wa_right - width - margin_x` clamped to `>= wa_left` (right-aligned WITHIN the
    usable width; computed from the work-area rect, not full screen — preserve the
    existing right-margin so the window does not bleed off the usable area). The existing
    horizontal margin (`40`) is preserved.
  - `y = wa_top + pad` for ALL tabs (move the window up; this is the "implied y").
  - Result: for `jobs`/`tasks`, `bottom = y + height = (wa_top + pad) + (usable_height -
    2*pad) = wa_bottom - pad`, so a `pad` gap always remains above the taskbar.

### 4. Apply geometry at construction and on tab change

- Construction: replace the single `_overlay_geometry()` call at `ui.py:381` so the
  default (`usage`) tab gets the new tab-aware geometry. Either rewrite
  `_overlay_geometry()` (`ui.py:409-420`) to gather the live inputs (screen dims via
  `winfo_screen*`, work-area via the new ctypes query, `self.active_tab`,
  `self.config.pad_fraction`) and delegate to the pure function, or add a thin
  `_apply_overlay_geometry()` wrapper that does so. The window opens at the `usage`-tab
  geometry, already moved up.
- Tab change: in `select_tab()` (`ui.py:1081-1087`), after `self.active_tab = tab_id`
  (and the existing jobs/tasks priming), re-apply geometry via the same thin wrapper —
  a single `self.overlay.geometry(compute_overlay_geometry(...))` call. This must run
  whether or not the overlay is currently visible, and must NOT call `refresh_data`,
  re-aggregate, re-fetch, or rebuild any tab body. `_render_active_tab()` continues to
  do only its pack/unpack work.
- The hotkey path (`toggle_overlay` / `show_overlay` / `hide_overlay`,
  `ui.py:2438-2470`) is NOT modified.

## Expected Resolution (human-visible)

- Triggering the overlay opens it near the top of the monitor (top edge one `pad` below
  the top of the usable area), at the current `980×660` size on the Usage tab.
- Clicking Jobs or Tasks makes the same window grow tall — filling the usable height
  minus the top/bottom padding — so many more rows are visible; the width stays 980 and
  it stays right-aligned.
- Clicking back to Usage returns the window to the `980×660` size at the same top
  position.
- On every tab, a visible gap remains above the Windows taskbar; the taskbar is never
  covered.
- Increasing or decreasing `pad_fraction` in config visibly changes how far the window
  sits from the top/bottom edges after a restart.

## Goals

- Add a configurable `pad_fraction` field (default `0.05`) to `DashboardConfig` that
  round-trips through `load_config`/`save_config`.
- Add a ctypes Windows work-area query (`SPI_GETWORKAREA`, primary monitor) and use the
  work-area rect — not `winfo_screenwidth/height` — for the usable bounds.
- Factor a pure, unit-testable geometry function of
  `(screen_width, screen_height, work_area, tab_id, pad_fraction)` returning `"WxH+X+Y"`,
  implementing the pinned per-tab math.
- Apply the correct tab-aware geometry at construction for the default tab and recompute
  + re-apply it on every tab change via `select_tab()`, without rebuilding tab data.
- Cover the geometry math with unit tests (mocking the work-area query) and add a new
  repo-root `REGRESSION.md` named case `REG-006` for the human-facing tab-resize
  interaction.

## Non-Goals

- Widening the window. The width stays at the current `980` clamp and stays right-aligned.
- Changing the Usage tab's SIZE. The Usage tab keeps `980×660`; only its position changes.
- Per-monitor follow-the-cursor multi-monitor behavior. Targeting the primary monitor's
  work area is sufficient; deeper multi-monitor placement is out unless the human asks.
- Changing the hotkey binding or the Task-0013 show/hide toggle behavior.
- Any non-geometry UI work (styling, new controls, content/layout of tab bodies,
  scrolling behavior, etc.).
- Backend orchestration / Temporal / jobs / tasks backend logic; token ingest, scanner,
  storage, aggregation; package or repo-path renames.

## Acceptance Criteria

Each criterion is pass/fail. The unit-test criteria run via
`python -m unittest discover -s tests -p "test_*.py" -v`.

1. `DashboardConfig` has a `pad_fraction` field with a default of `0.05`, and a test
   asserts it round-trips through `save_config` → `load_config` (write a config with a
   non-default `pad_fraction`, read it back, get the same value; a config missing the
   field loads the `0.05` default cleanly).
2. A pure geometry function exists that takes `(screen_width, screen_height, work_area
   rect, tab_id, pad_fraction)` and returns a `"WxH+X+Y"` string, with NO Tk calls
   (callable directly from a unit test with mocked inputs).
3. For `tab_id` in `{"jobs","tasks"}`, the returned height equals
   `(work_area_bottom − work_area_top) − 2 × round(pad_fraction × screen_height)`
   (asserted by a unit test against a mocked work-area rect).
4. For every tab, the returned top `y` equals `work_area_top + round(pad_fraction ×
   screen_height)` (asserted by a unit test for usage, jobs, and tasks).
5. Taskbar-never-covered (its own criterion): for `tab_id` in `{"jobs","tasks"}`, the
   computed bottom (`y + height`) is `≤ work_area_bottom`, and specifically equals
   `work_area_bottom − round(pad_fraction × screen_height)` — i.e. a positive `pad` gap
   remains above the taskbar — asserted by a unit test using a work-area rect whose
   bottom is strictly less than the full screen height (taskbar present). The test also
   asserts that for the usage tab `y + height ≤ work_area_bottom`.
6. The Usage (default) tab keeps the current size: the returned width equals the current
   width clamp value and the returned height equals the current `660` height clamp value
   for representative screen dimensions (asserted by a unit test); the usage-tab height
   is NOT the tall `usable − 2·pad` value.
7. Width stays 980 (not widened): for representative dimensions where the screen is wide
   enough, the returned width is `980` for all three tabs, and is never greater than
   `980`, asserted by a unit test.
8. Padding-is-configurable (its own criterion): a unit test computes geometry for the
   same screen/work-area/tab with two different `pad_fraction` values (e.g. `0.05` and
   `0.10`) and asserts the results differ in the expected direction — a larger
   `pad_fraction` produces a smaller jobs/tasks height and a larger top `y`.
9. The usable bounds come from the Windows work area, not full-screen dimensions: the
   geometry source is the `SPI_GETWORKAREA` (or equivalent work-area) rect, and the
   computed `x` is right-aligned within the work-area width (verified by code inspection
   plus a unit test where `work_area_right < screen_width`, asserting `x + width ≤
   work_area_right`). The implementation does not feed `winfo_screenwidth/height` into
   the usable-width/usable-height inputs of the geometry function.
10. Tab switch re-applies geometry without rebuilding data: `select_tab()` re-applies
    `self.overlay.geometry(...)` on tab change, and a test (mirroring the existing
    `SimpleNamespace` mocking style in `tests/test_desktop_support.py`) asserts that the
    tab-change path issues the geometry call and does NOT call `refresh_data` (or the
    aggregation/rebuild path). The construction path applies the `usage`-tab geometry.
11. The hotkey show/hide path (`show_overlay`/`hide_overlay`/`toggle_overlay`) is
    unchanged (no geometry recompute added there; Task-0013 behavior preserved),
    verified by code inspection.
12. Repo-root `REGRESSION.md` contains a new named case `REG-006` for the human-facing
    tab-resize/reposition interaction (a distinct case, not folded into REG-001/002/003).
13. The full unit-test suite passes via
    `python -m unittest discover -s tests -p "test_*.py" -v`.

## What Does Not Count

- Using `winfo_screenwidth()` / `winfo_screenheight()` for the usable bounds. Those
  include the taskbar; the usable bounds must come from the work-area rect. (Full-screen
  height may be used ONLY to compute `pad = pad_fraction × screen_height`.)
- Hardcoding the padding (e.g. a literal `50` px or a constant) instead of a configurable
  `pad_fraction` field that round-trips through config. The human said "configurable."
- Widening the window beyond `980` or changing the Usage tab's height to the tall value.
  Both are explicit fails.
- Making the Jobs/Tasks window tall enough that its bottom reaches or passes
  `work_area_bottom` (no `pad` gap), or that it extends below the work area onto the
  taskbar. The taskbar must never be covered.
- Recomputing geometry by triggering a data refresh, re-aggregation, or full re-render
  on tab switch (regressing the Task-0013 cheap-show/hide behavior). The tab-switch
  geometry change must be a cheap geometry-only call.
- "It looks right on my machine" with no unit test on the pure geometry math. The math
  must be proven by tests that mock the work-area query (no live display required).
- Claiming the live overlay is fixed from source edits + passing tests alone. The live
  outcome requires a published pinned release + restart, which is a separate human-gated
  step (see `## Proof Plan`).

## Proof Plan

- Primary proof is unit tests on the pure geometry function, run with
  `python -m unittest discover -s tests -p "test_*.py" -v`. The work-area query is mocked
  (the function takes the rect as an argument), so no live display is needed. Tests assert,
  at minimum: jobs/tasks height = `usable − 2·pad` (AC 3); top `y = wa_top + pad` for all
  tabs (AC 4); jobs/tasks bottom `= wa_bottom − pad ≤ wa_bottom` with a taskbar-present
  rect (AC 5); usage keeps the current size (AC 6); width stays `980` (AC 7);
  `pad_fraction` changes the result in the expected direction (AC 8); `x` is right-aligned
  within the work-area width (AC 9); and the config round-trip of `pad_fraction` (AC 1).
- A behavior test (using the existing `SimpleNamespace` Tk-mock style in
  `tests/test_desktop_support.py:474-488`) asserts that `select_tab()` issues the geometry
  re-apply and does NOT invoke the data-rebuild/refresh path (AC 10).
- The new `REGRESSION.md` `REG-006` case documents the human-facing app-surface check:
  trigger the overlay on an isolated/task-owned lane (per `TESTING.md:138-161` — task-owned
  config + isolated SQLite, fixture telemetry, NOT the human's live config/DB), switch
  Usage → Jobs → Tasks → Usage, and confirm: window moves up on every tab; Jobs/Tasks are
  tall and right-aligned at width 980; a visible gap remains above the taskbar on every
  tab; the Usage tab stays `980×660`. `write_overlay_capture()` (`ui.py:275-282`, which
  reads live `winfo_rootx/rooty/width/height`) can produce a screenshot/geometry artifact
  as supporting evidence.
- Deployment honesty: source edits + passing unit tests do NOT change what the human sees
  on the running overlay. The live resize appears ONLY after a new pinned release is
  published (`scripts/Publish-DashboardRelease.ps1`) and the overlay restarted
  (`scripts/Start-CodexDashboard.ps1`; `dashboard-current-release.json`). That publish +
  restart is a separately human-gated live-lane step and is NOT part of this task's
  creation or its unit-test proof. The proof plan does not claim the live overlay is fixed
  until that human-gated step is performed.

## Falsifiers

- Taskbar-cover falsifier (its own falsifier): for the `jobs` or `tasks` tab on a monitor
  with a taskbar (work-area bottom strictly less than full screen height), the computed
  window bottom (`y + height`) is `≥ work_area_bottom` (no `pad` gap) — i.e. the window
  reaches or covers the taskbar region. If the geometry ever yields `bottom > wa_bottom`
  (or `bottom == wa_bottom` with no remaining `pad` gap), the task is wrong.
- Padding-configurable falsifier (its own falsifier): changing `pad_fraction` in config
  has no effect on the computed geometry, OR the padding is a hardcoded constant (e.g.
  `50`) with no config field, OR `pad_fraction` does not round-trip through
  `save_config`/`load_config`.
- Work-area falsifier: the usable bounds are derived from `winfo_screenwidth/height`
  (taskbar-inclusive) instead of the work-area rect, so the window can be placed over the
  taskbar or sized as if the taskbar did not exist.
- Tall-height falsifier: on the Jobs or Tasks tab the height is not
  `usable_height − 2·pad` (e.g. it stays `660`, or uses full screen height instead of
  work-area height).
- Top-position falsifier: on any tab the top `y` is not `work_area_top + pad` (the window
  is not moved up as specified).
- Usage-size falsifier: the Usage tab's width or height changed from the current
  `980`/`660` clamp values (size was altered, not just position).
- Width falsifier: the returned width exceeds `980` (the window was widened).
- No-rebuild falsifier: switching tabs triggers a data refresh / re-aggregation / full
  re-render rather than a cheap geometry-only re-apply (Task-0013 behavior regressed).
- Proof falsifier: there is no unit test on the pure geometry math (only a "looks right"
  claim), OR there is no `REG-006` named case in repo-root `REGRESSION.md`, OR the suite
  does not pass.

## Open Questions

None that change the writeup type, implementation home, solution shape, enforcement
boundary, acceptance bar, or falsifiers. The three load-bearing ambiguities (tall-mode
width, height-vs-padding reconciliation, Usage-tab size) were surfaced to the human and
answered authoritatively (`HUMAN-DIRECTIVES-FOR-WORKER.md`). Minor implementer latitude
that does not change scope: whether the work-area helper lives inline in `ui.py` or in a
tiny imported module, and whether `_overlay_geometry()` is rewritten in place or wrapped
by a new `_apply_overlay_geometry()`. Either is acceptable as long as the pure function,
the config field, and the `select_tab` hook exist as specified.

## References

- This task folder: `HUMAN-DIRECTIVES-FOR-WORKER.md`, `TASK-CREATE-OBJECTIVE.md`,
  `TASK-CREATE-CONTEXT-MANIFEST.md`.
- Geometry to change: `app/codex_dashboard/ui.py:376-381` (Toplevel creation + the single
  `geometry(...)` apply), `ui.py:409-420` (`_overlay_geometry()` current formula using
  `winfo_screenwidth/height`), `ui.py:275-282` (`write_overlay_capture()` reads live window
  bounds).
- Tab hook: `ui.py:313` (`self.active_tab = "usage"`), `ui.py:600,605,620` (three-tab tuple
  loop, upper-cased labels, `<Button-1>` → `select_tab`), `ui.py:1081-1087` (`select_tab()`,
  the single hook), `ui.py:1089-1103` (`_render_active_tab()` pack/unpack only).
- Show/hide invariant (do not regress): `ui.py:2438-2444` (`toggle_overlay`),
  `ui.py:2446-2464` (`show_overlay`, no recompute), `ui.py:2466-2470` (`hide_overlay`).
- Work-area / ctypes pattern: `app/codex_dashboard/hotkey.py:3,6,8` (`import ctypes`,
  `wintypes`, `ctypes.WinDLL("user32", use_last_error=True)`); `ui.py:3` (`import ctypes`).
- Config: `app/codex_dashboard/config.py:19-37` (`DashboardConfig` + `defaults()`),
  `config.py:68-86` (`load_config`), `config.py:89-95` (`save_config`, serializes via
  `asdict`).
- Tests / regression / deployment: `TESTING.md:16-17` (test command), `TESTING.md:138-161`
  (isolated task-owned config + SQLite, fixture telemetry), `tests/test_desktop_support.py:474-488`
  (Tk-mock pattern), `REGRESSION.md` (named cases REG-001..REG-005; add REG-006),
  `DATA-HANDLING.md` (data isolation), `scripts/Publish-DashboardRelease.ps1`,
  `scripts/Start-CodexDashboard.ps1`, `%LOCALAPPDATA%\CodexDashboard\dashboard-current-release.json`
  (pinned-release model).
- Same-repo style/quality exemplar (structure + pinned-release caveat; not scope):
  `Tracking/Task-0013/TASK.md`.

## Audit Status

Human-approved on 2026-05-29: the human reviewed the coordinator-reviewed draft and
approved it, authorizing creation of the bound GitHub issue. TaskCreate model of
2026-05-29: coordinator + blind writer-worker with a coordinator concreteness review
(no separate agent-auditor lane). See
[TASK-CREATE-COORDINATOR-NOTES.md](./TASK-CREATE-COORDINATOR-NOTES.md).

Provider-binding gate SATISFIED: GitHub issue #14
(https://github.com/Digital-Collective-Games/Obsidian/issues/14) was created
issue-first at the matching number and is bound by [TASK-META.json](./TASK-META.json);
a reconcile dry-run shows the local task and issue #14 in sync with zero differences.
The issue is OPEN — the task is created, not yet executed (Queue=Never, Priority=P2,
Human Needed=No by default; raise Queue when ready to dispatch). TASK-STATE.json is
intentionally not written at task-creation time; it is owned by the dispatch lifecycle
(TaskDispatch).
