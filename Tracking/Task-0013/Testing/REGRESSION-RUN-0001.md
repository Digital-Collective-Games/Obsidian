# Regression Run 0001

Test type: `regression test`

Claimed lane:

- `REG-001 Desktop Overlay Launch And Data Smoke`
- `REG-005 Usage Source Filter (Codex/Claude)` (Task-0013 Objective 4 case)

Actual flow exercised:

1. Built a task-owned isolated lane (no live data):
   - `python Tracking/Task-0013/Testing/build_regression_fixture.py`
   - creates `Runtime/codex-fixture/.codex/sessions/.../*.jsonl` (real-shaped
     `token_count`), `Runtime/claude-fixture/.claude/projects/.../*.jsonl`
     (real-shaped `type:"assistant"` with repeated `requestId`s), and an isolated
     `Runtime/config.json` pointing `codex_root` + `claude_root` at the fixtures
     and `db_path` at `Runtime/dashboard-regression.db`.
2. Ran one ingest cycle on the isolated config:
   - `python -m app.codex_dashboard --config-path Runtime/config.json --scan-once --print-summary`
   - result: `files_scanned=2 files_updated=2 events_ingested=100`,
     `last_7d_tokens=138539000`.
3. Verified the persisted SQLite state directly: 60 `codex` rows + 40 `claude`
   rows (80 Claude assistant lines de-duplicated to 40 requests), merged total
   138,539,000 = 137,700,000 + 839,000, 0 Claude rows with non-null advisory,
   indexes `idx_token_events_event_timestamp` and `idx_token_events_source_event`
   present.
4. Launched the real desktop app with smoke artifact capture on the isolated
   config, exercising the real hotkey -> `show_overlay` path:
   - `python -m app.codex_dashboard --config-path Runtime/config.json --smoke-artifact-dir Runtime/smoke-usage --smoke-tab usage`
   - artifacts preserved at `Testing/smoke-usage/`.
5. Demonstrated the source filter (REG-005) toggling against the same loaded
   snapshot (no per-toggle DB read):
   - `python Tracking/Task-0013/Testing/source_filter_demo.py` -> `SOURCE-FILTER-RESULT.json`.

Why this counts:

- the smoke run started the real Tk app and opened the real overlay through the
  real hotkey trigger (`hotkey_triggered=True`, `overlay_fallback=False`), not a
  text-only backend dump
- the captured overlay window (PNG) shows the rebranded **OBSIDIAN** brand, the
  merged Codex+Claude 7d total (138.5M), interval/budget/redline state, and the
  Task-0013 **Source: All** filter control on the live surface
- the lane is isolated (task-owned fixtures + task-owned SQLite), satisfying the
  repo-root `REGRESSION.md` lane-isolation rule
- the source-filter selections were computed from one in-memory snapshot,
  matching the overlay's `_toggle_source` -> `_render_dashboard` path, which never
  reads SQLite (preserving Objective 3)

Disqualifiers / limitations:

- the smoke harness drives the hotkey/activation programmatically rather than
  requiring a literal human mouse click; the REG-005 per-toggle UI before/after
  is demonstrated at the render/aggregation level (`SOURCE-FILTER-RESULT.json`)
  plus the live control captured in `overlay.png`, rather than four separate
  GUI screenshots of each checkbox state
- the backend `Jobs`/`Tasks` lanes (REG-002/003/004) were not exercised; this
  task does not change those surfaces

## Environment

- desktop app repo: `C:\Agent\CodexDashboard`
- isolated config: `Tracking/Task-0013/Testing/Runtime/config.json`
- isolated DB: `Tracking/Task-0013/Testing/Runtime/dashboard-regression.db`
- fixtures: `Runtime/codex-fixture/.codex`, `Runtime/claude-fixture/.claude`
- no live `%LOCALAPPDATA%\CodexDashboard`, `C:\Users\gregs\.codex`, or `~/.claude`
  data was read

## Results

### REG-001

Result: `passed`

Artifacts:

- [overlay.png](./smoke-usage/overlay.png)
- [overlay-summary.txt](./smoke-usage/overlay-summary.txt)

Observed:

- `active_tab=usage`, overlay visible without fallback-only behavior
- `hotkey_triggered=True`, `overlay_fallback=False`
- header brand renders `OBSIDIAN`
- `7d_total=138.5M` (merged Codex+Claude), `status=Operational`, budget line and
  advisory rendered on the live surface

### REG-005

Result: `passed`

Artifacts:

- [overlay.png](./smoke-usage/overlay.png) (shows the `Source: All` control)
- [SOURCE-FILTER-RESULT.json](./SOURCE-FILTER-RESULT.json)

Observed:

- the live overlay shows a Source filter control with Codex and Claude
  checkboxes, styled with the overlay (`Source: All` label)
- All: 138,539,000 ; Codex only: 137,700,000 ; Claude only: 839,000 ; None: 0
- `merged_equals_sum_of_parts=true`, `none_is_zero=true`
- filtering operated on one loaded snapshot with no per-selection DB read
