# Task-0013 — Publish + Restart Deploy Proof (2026-05-29)

Evidence for the human-authorized live-lane step in
[../../HUMAN-DIRECTIVES-FOR-WORKER.md](../../HUMAN-DIRECTIVES-FOR-WORKER.md)
("2026-05-29 — Publish + restart authorization": *"Publish + restart now."*).

Canonical dashboard frontend release-isolation flow per repo
[../../../../TESTING.md](../../../../TESTING.md) `SMOKE-003`,
[../../../../DATA-HANDLING.md](../../../../DATA-HANDLING.md), and
`scripts\Publish-DashboardRelease.ps1` / `scripts\Start-DashboardRelease.ps1` /
`scripts\Test-DashboardRelease.ps1`.

## 1. Release-script unit coverage (SMOKE-003 step 1)

`python -m unittest tests.test_dashboard_release_scripts tests.test_desktop_support -v`
-> **44 tests OK** (3 dashboard-release-script tests + 41 desktop-support tests;
no skips/failures). Re-run of `tests.test_dashboard_release_scripts` alone: 3 OK.

## 2. Published a new pinned release FROM THE COMMITTED GIT TREE

`scripts\Publish-DashboardRelease.ps1` (default `source_mode=git_commit`).

- `release_id`: **20260529T143554Z-e99ac895ee61**
- `git_commit`: **e99ac895ee619239638549e0e42c6443cc133722** (committed Task-0013 HEAD on `master`/upstream)
- `source_mode`: **git_commit**, `source_dirty`: **false**
- `repository_dirty`: true — the unrelated untracked files (`token_time.py`, the
  token-usage CSVs, Task-0009/0012 edits) are recorded in `source_status` for
  transparency but were **NOT shipped**: the release `files[]` manifest contains
  only the 21 committed `app/` tree files (verified — no `token_time.py`).
- Full manifest: [published-release-manifest.json](./published-release-manifest.json).

The publish copies `git archive HEAD app`, so a dirty working tree cannot leak
into the release. `-PlanOnly` first confirmed `source_mode=git_commit`.

## 3. Restarted the human's overlay onto the pinned release (data preserved)

- Before: one overlay (PID 36756) pinned to the OLD release
  `20260427T162803Z-3c0fabb2f1d5` (pre-Task-0013 commit `3c0fabb2`).
- Stopped ONLY that old-release overlay (matched on the old release id), then ran
  `scripts\Start-DashboardRelease.ps1` (validates the manifest hashes, launches
  the runtime-root launcher, then runs `Test-DashboardRelease.ps1`).
- After: one overlay (PID 67656) pinned to the NEW release.

### Data / config / startup preservation (Decision A — kept the data root)

[live-data-preservation.json](./live-data-preservation.json):

- `config.json` — byte-identical (sha256 `D1BF7990…`, mtime 2026-04-21 unchanged).
  **Untouched.**
- Startup `CodexDashboard.cmd` — content-identical (sha256 `BA8F5401…` matches the
  pre-publish baseline); mtime updated only because `Install-DashboardStartup`
  rewrote the same bytes. Still points at the runtime-root pinned launcher.
- `dashboard.db` — same file at `%LOCALAPPDATA%\CodexDashboard\dashboard.db`, now
  held open by the restarted overlay (the human's own running instance using it).
  **Not migrated, reset, deleted, or overwritten.** The pinned release includes
  the additive + idempotent `source`/`source_event_id` migration (per the
  Coordinator Verification), which is non-destructive and preserves existing rows.

## 4. Isolation verification (SMOKE-003 step 4)

`scripts\Test-DashboardRelease.ps1` -> [test-dashboard-release.json](./test-dashboard-release.json):

- `current_release.release_id` = `20260529T143554Z-e99ac895ee61`,
  `git_commit` = `e99ac895…`, `source_mode` = `git_commit`, `source_dirty` = false.
- `current_release_error` = **null** (all 21 copied source hashes validate).
- `startup_uses_pinned_launcher` = **true** (startup invokes the runtime-root
  launcher, not `C:\Agent\CodexDashboard`).
- `running_process_count` = **1**; the running `pythonw.exe` command line includes
  `--release-id 20260529T143554Z-e99ac895ee61 --release-root
  …\dashboard-releases\20260529T143554Z-e99ac895ee61`.

Re-checked after all capture activity: still 1 process, still pinned, error null.

## 5. Human-surface proof from the restarted, release-pinned overlay

All captures load the dashboard package FROM THE PINNED RELEASE ROOT
(`release_overlay_capture.py`, `loaded_from_release_root: true`) against the
task-owned isolated config + SQLite DB under `../Runtime/` (synthetic Codex +
Claude fixtures). **No live data was read** (`reads_live_data: false`); the live
`dashboard.db`, `config.json`, `C:\Users\gregs\.codex`, and `~/.claude` were not
touched, so the human's live spend is not exposed.

- [smoke-all/overlay.png](./smoke-all/overlay.png) — **BEFORE**: the **OBSIDIAN**
  brand, **7D TOTAL TOKENS 138.5M** (merged Codex+Claude), control reads
  **Source: All**. Summary `7d_total=138.5M`, `hotkey_triggered=True`.
- [smoke-claude-off/overlay.png](./smoke-claude-off/overlay.png) — **AFTER**
  unchecking Claude via the released `_toggle_source`: control reads
  **Source: Codex**, **7D TOTAL TOKENS 137.7M** (Claude's 0.839M removed),
  projected 339.4M -> 336M. Summary `7d_total=137.7M`. Distinct image hash from
  the BEFORE shot.
- [release-overlay-expanded.png](./release-overlay-expanded.png) — the released
  source-filter dropdown **expanded**, showing the **Codex** and **Claude**
  checkbuttons.
- [release-capture-summary.json](./release-capture-summary.json) — deterministic,
  occlusion-proof numeric record: merged 138,539,000 = Codex 137,700,000 +
  Claude 839,000 (`merged_equals_codex_plus_claude: true`, `claude_present: true`);
  displayed before/after All/138.5M -> Codex/137.7M (`before_after_changed: true`).

### GUI-capture limitation (recorded honestly, not a product failure)

The expanded-dropdown screenshot uses an absolute-screen-region grab of the posted
native Tk popup menu. In this contended/automated desktop, the overrideredirect
topmost overlay window cannot be reliably forced un-occluded to the foreground for
a `CopyFromScreen` grab from a background process, so the menu in
`release-overlay-expanded.png` is captured over the agent's editor rather than
over the OBSIDIAN body. The released checkbutton control (Codex + Claude) is fully
legible. The OBSIDIAN body + before/after totals are proven by the two
`smoke-*/overlay.png` images (captured via the app's own designed smoke timeline,
which screenshots at the fresh-foreground moment) plus the deterministic
`release-capture-summary.json`. This is a screenshot-compositing limitation of the
environment, not a defect in the published release.
