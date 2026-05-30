<!-- task-sync: repo=CodexDashboard; task_id=Task-0014; task_path=Tracking/Task-0014/TASK.md -->

# Task-0014: Make the Obsidian overlay window tab-aware: move it up on every tab, and make the Jobs and Tasks tabs fill the usable height above the taskbar (configurable padding, taskbar never covered).

## Source Of Truth

Local `Tracking/Task-0014/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0014:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

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

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `14`
- Local task path: `Tracking/Task-0014/TASK.md`
- Source commit: `ed4b29411673c462f5294dabbe0be38df4e13305`
- Local task SHA-256: `3A5D7FC8B317DF99D721D346C793E7229A730A0D7E68F0C5075D0BEC3C07DEE1`
- Rendered at: `2026-05-29T15:00:46.3339754-04:00`