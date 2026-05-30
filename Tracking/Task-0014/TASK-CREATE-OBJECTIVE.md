# Task-0014 TaskCreate Objective

Worker-safe objective for the TaskCreate writer-worker. This states WHAT the task
should accomplish and WHY. It does not replace the writer's job of choosing the
honest writeup type and writing falsifiable acceptance criteria per
`C:\Users\gregs\.codex\Orchestration\Processes\TASK-CREATE.md`. The chosen
solution shape is constrained by the human's explicit clarifying answers (see
`HUMAN-DIRECTIVES-FOR-WORKER.md`); do not re-open those decisions.

## One-Line Objective

Make the Obsidian desktop overlay window resize and reposition based on which tab
is active: move the window up into the usable area above the taskbar on every tab,
and on the Jobs and Tasks tabs make it as tall as the usable area minus a
configurable top/bottom padding (default 5% of screen height), without ever
covering the Windows taskbar.

## Human-Facing Outcome (state first)

The operator runs this always-on-top overlay (toggled by a global hotkey). Today
it opens as a fixed 980×660 panel in the upper-right, positioned using full-screen
dimensions that include the taskbar, and it never changes size or position when
the operator switches tabs. The operator wants:

- The window to sit higher (closer to the top of the monitor) on every tab.
- The Jobs and Tasks tabs — which show long lists/tables — to use a tall window
  that fills the usable vertical space (above the taskbar) minus a small,
  configurable padding, so more rows are visible at once.
- The Usage tab to keep its current size but also be moved up.
- The taskbar never to be covered.

## Current Truth (from a codebase map — factual, not a prescribed solution)

- The overlay is a persistent `tk.Toplevel`, `overrideredirect(True)`,
  `-topmost True`, created withdrawn and toggled show/hide by the hotkey
  (`app/codex_dashboard/ui.py:376-381`, `show_overlay` 2446-2464, `hide_overlay`
  2466-2470, `toggle_overlay` 2438-2444). The hotkey toggle does NOT rebuild or
  re-render (Task-0013 show/hide fix); it only deiconifies/withdraws.
- Window geometry is computed ONCE by `_overlay_geometry()`
  (`ui.py:409-420`) and applied ONCE at `ui.py:381`; it is never recomputed.
  The formula uses `self.root.winfo_screenwidth()` / `winfo_screenheight()`
  (`ui.py:414-415`), which INCLUDE the taskbar area. Defaults: width clamp
  `min(980, max(860, screen_w-80))`, height clamp `min(660, max(620, screen_h-80))`,
  `x = min(940, max(20, screen_w-width-40))`, `y = min(100, max(20, screen_h-height-40))`.
- There are exactly three tabs, created by a tuple loop at `ui.py:600`:
  `("usage","Usage")`, `("jobs","Jobs")`, `("tasks","Tasks")` (labels rendered
  upper-cased). Active tab is `self.active_tab` (default `"usage"` at `ui.py:313`).
- Tab changes flow through `select_tab(tab_id)` (`ui.py:1081-1087`): it sets
  `self.active_tab = tab_id`, optionally primes jobs/tasks data, then calls
  `_render_active_tab()` (`ui.py:1089-1103`) which packs/unpacks the per-tab body
  frames. `select_tab()` is the single natural place to react to a tab change;
  there is no external event/hook today (the writer may add the geometry call
  there directly).
- There is NO work-area / taskbar / multi-monitor query anywhere in
  `app/codex_dashboard` (no `SPI_GETWORKAREA`, `GetMonitorInfo`,
  `MonitorFromWindow`). This is the gap the task fills.
- ctypes is already used and there is a reusable WinDLL pattern:
  `hotkey.py:8-9` does `user32 = ctypes.WinDLL('user32', use_last_error=True)` and
  `kernel32 = ctypes.WinDLL('kernel32', ...)`; `ui.py:3` imports `ctypes` and uses
  `ctypes.windll.user32` / `gdi32` elsewhere. A work-area query can reuse this.

## Chosen Solution Shape (constrained by human answers — do not re-open)

The human pinned the rule (see `HUMAN-DIRECTIVES-FOR-WORKER.md`). The task must
document and implement:

1. A Windows WORK-AREA query (usable area excluding the taskbar). Reuse the
   existing `ctypes` WinDLL pattern; `SystemParametersInfo(SPI_GETWORKAREA)` for
   the primary monitor is acceptable. Using the work-area rect (not full-screen
   dimensions) automatically handles a taskbar docked on any edge.
2. A configurable vertical padding fraction (default 0.05 = 5% of full screen
   height). The task must name where this config lives and that it round-trips
   like the other config fields. `pad = round(pad_fraction × screen_height)`.
3. Tab-aware geometry computation, ideally factored as a PURE function of
   `(screen dimensions, work-area rect, tab_id, pad_fraction, current width/height
   clamps)` returning a `WxH+X+Y` string, so it is unit-testable WITHOUT a live
   display:
   - All tabs: top `y = work_area_top + pad`; width = current 980 clamp,
     right-aligned WITHIN the usable width (compute x from the work-area rect, not
     full screen, so left/right taskbars are honored); do not widen.
   - `usage` / default: height = current 660 clamp (unchanged size), just moved up.
   - `jobs` / `tasks`: height = `(work_area_bottom − work_area_top) − 2 × pad`.
     Bottom = `work_area_bottom − pad` → always a `pad` gap above the taskbar.
4. Recompute + re-apply `self.overlay.geometry(...)` when the active tab changes
   (in/after `select_tab`), AND apply the correct initial geometry for the default
   tab at construction. Because the window is persistent and overrideredirect, a
   geometry re-apply on tab switch is cheap and must NOT trigger a data rebuild
   (preserve the Task-0013 show/hide behavior).

## Proof / Constraints The Draft Must Honor

- Unit-testable geometry math (mock the work-area query) is the primary proof:
  assert jobs/tasks height = usable − 2·pad, top y = wa_top + pad, bottom ≤
  wa_bottom (never covers taskbar), usage keeps current size, width stays 980.
- A new repo-root `REGRESSION.md` named case (next id `REG-006`) for the
  human-facing tab-resize interaction (repo rule: new human-facing UI behavior
  gets a named case; do not fold into an existing case).
- Tests run with `python -m unittest discover -s tests -p "test_*.py" -v`.
- Pinned-release deployment: the running overlay only changes after publish +
  restart; the draft's proof plan must say source edits + unit tests alone do not
  deliver the live outcome, and the publish/restart live-lane step is a separately
  human-gated action, not part of task creation.
- Honor `REGRESSION.md` / `DATA-HANDLING.md`: do not point validation at the
  human's live config/DB; use isolated/task-owned fixtures.

## Audit-Readiness Notes

- Writeup type is `concrete implementation` (the solution shape is chosen and the
  files/seams are named). Do not downgrade to consensus/research.
- Pin the exact geometry formula (the human gave it); leave no "improve sizing"
  vagueness. Every acceptance criterion should be pass/fail.
- The "never cover the taskbar" constraint needs its own explicit falsifier.
- The padding-is-configurable requirement needs an explicit acceptance criterion.
- Out of scope unless the human expands: widening the window, changing the Usage
  tab's size, per-monitor follow-the-cursor multi-monitor behavior, changing the
  hotkey or the show/hide toggle, and any non-geometry UI work.
