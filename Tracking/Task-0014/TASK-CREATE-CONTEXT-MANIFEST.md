# Task-0014 TaskCreate Context Manifest

Worker-safe list of durable files the writer-worker may read to draft `TASK.md`.
All paths are relative to the repo root `C:\Agent\CodexDashboard` unless absolute.
These citations were produced by a read-only recon pass on 2026-05-29; the writer
should re-open the files as needed and may cite exact lines.

## Authoritative human context (read first)

- `Tracking/Task-0014/HUMAN-DIRECTIVES-FOR-WORKER.md` — verbatim directive + the
  three clarifying answers that pin the geometry rule. AUTHORITATIVE.
- `Tracking/Task-0014/TASK-CREATE-OBJECTIVE.md` — objective + current truth +
  chosen solution shape + proof constraints.

## Shared standards (the writeup contract)

- `C:\Users\gregs\.codex\Orchestration\Processes\TASK-CREATE.md`
- `C:\Users\gregs\.codex\Orchestration\Exemplars\TASK.md`
- `Tracking/Task-0013/TASK.md` — strong recent same-repo exemplar for structure,
  concreteness, and the pinned-release deployment caveat. Use as a style/quality
  reference; do NOT copy its scope.

## Tab structure (where to hook tab-aware geometry)

- `app/codex_dashboard/ui.py:313` — `self.active_tab = "usage"` (default).
- `app/codex_dashboard/ui.py:600,605,620` — three-tab tuple loop
  `("usage","Usage"),("jobs","Jobs"),("tasks","Tasks")`, labels upper-cased,
  `<Button-1>` binding → `select_tab(tab_id)`.
- `app/codex_dashboard/ui.py:1081-1087` — `select_tab()` sets `self.active_tab`,
  primes jobs/tasks data, calls `_render_active_tab()`. SINGLE natural hook point.
- `app/codex_dashboard/ui.py:1089-1103` — `_render_active_tab()` packs/unpacks the
  per-tab body frames.
- `app/codex_dashboard/ui.py:835-951` — `_build_jobs_lane()` (jobs body).
- `app/codex_dashboard/ui.py:953-1051` — `_build_tasks_lane()` (tasks body).

## Window geometry (what to change)

- `app/codex_dashboard/ui.py:376-381` — Toplevel creation: `withdraw()`,
  `overrideredirect(True)`, `-topmost True`, `geometry(self._overlay_geometry())`.
- `app/codex_dashboard/ui.py:409-420` — `_overlay_geometry()` current formula
  (980×660 clamps, upper-right, uses `winfo_screenwidth/height` INCLUDING taskbar).
- `app/codex_dashboard/ui.py:2438-2470` — `toggle_overlay` / `show_overlay`
  (deiconify/lift/focus, NO geometry recompute) / `hide_overlay` (withdraw).
- `app/codex_dashboard/ui.py:277-282` — `write_overlay_capture()` reads live
  `winfo_rootx/rooty/width/height` (useful for a screenshot proof of the resize).

## Work-area / taskbar / ctypes (the gap to fill + reusable pattern)

- No work-area/taskbar/multi-monitor query exists anywhere in
  `app/codex_dashboard` (confirmed by recon). This is what the task adds.
- `app/codex_dashboard/hotkey.py:3,6,8-9` — reusable pattern:
  `import ctypes`, `from ctypes import wintypes`,
  `user32 = ctypes.WinDLL('user32', use_last_error=True)`, `kernel32 = ...`.
- `app/codex_dashboard/ui.py:3` — `import ctypes`; `ui.py:127,136` gdi32 calls;
  `ui.py:2584-2585` `ctypes.windll.user32.keybd_event`. Establishes that ctypes
  user32 calls are already used in this module.

## Config (where the configurable padding fraction belongs)

- `app/codex_dashboard/config.py` — `DashboardConfig` and `defaults()` /
  `load_config` / `save_config`. The new `pad_fraction` (default 0.05) field
  should round-trip like existing fields (e.g. `codex_root`, `hotkey`).

## Tests, regression, deployment (proof path)

- `tests/` + `python -m unittest discover -s tests -p "test_*.py" -v` (TESTING.md:16-17).
- `tests/test_desktop_support.py:474-488` — existing test mocks `winfo_width/height`
  for capture; pattern for mocking Tk geometry calls in tests. No existing test
  asserts geometry math (gap).
- `REGRESSION.md` — named cases `REG-001`..`REG-005`; new human-facing UI behavior
  must add a named case (next id `REG-006`). REG-001 is the overlay launch smoke.
- `scripts/Publish-DashboardRelease.ps1`, `scripts/Start-CodexDashboard.ps1`,
  `scripts/DashboardReleaseHelpers.ps1`, `%LOCALAPPDATA%\CodexDashboard\dashboard-current-release.json`
  — pinned-release model: source edits do not change the running overlay until
  publish + restart. (Live-lane publish/restart is human-gated, not part of task
  creation.)
- `DATA-HANDLING.md`, `TESTING.md` — isolation rules; do not use the human's live
  config/DB for validation.

## Out of scope (do not pull in)

- Backend orchestration / Temporal / jobs / tasks backend logic.
- Token ingest, scanner, storage, aggregation (Task-0013 territory).
- Renaming packages/paths, the hotkey binding, or the show/hide toggle behavior.
