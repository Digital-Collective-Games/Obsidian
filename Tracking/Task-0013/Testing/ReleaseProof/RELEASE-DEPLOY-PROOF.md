# Task-0013 — Publish + Restart Deploy Proof: show/hide fix (2026-05-29)

Evidence for the human-authorized live-lane step in
[../../HUMAN-DIRECTIVES-FOR-WORKER.md](../../HUMAN-DIRECTIVES-FOR-WORKER.md)
("2026-05-29 — Publish authorization for the show/hide fix release (commit
247f297)": *"Publish + restart now."*).

This deploy supersedes the earlier `e99ac895` deploy: it publishes and restarts
onto commit **`247f2973`** (PASS-0002, "hotkey toggles visibility only"), so the
human's running overlay now runs the show/hide activation fix.

Canonical dashboard frontend release-isolation flow per repo
[../../../../TESTING.md](../../../../TESTING.md) `SMOKE-003`,
[../../../../DATA-HANDLING.md](../../../../DATA-HANDLING.md), and
`scripts\Publish-DashboardRelease.ps1` / `scripts\Start-DashboardRelease.ps1` /
`scripts\Test-DashboardRelease.ps1`.

## 1. Release-script unit coverage (SMOKE-003 step 1)

`python -m unittest tests.test_dashboard_release_scripts tests.test_desktop_support -v`
-> **44 tests OK** (no skips/failures). Includes the show/hide coverage
(`test_show_overlay_only_reveals_persistent_window_no_rebuild`,
`test_show_overlay_cold_snapshot_still_only_reveals_no_load`,
`test_start_activation_load_runs_load_off_thread`) plus the dashboard
release-script isolation tests and the pinned-launcher startup-command test.

## 2. Published a new pinned release FROM THE COMMITTED GIT TREE

`scripts\Publish-DashboardRelease.ps1` (default `source_mode=git_commit`).
`-PlanOnly` first confirmed `source_mode=git_commit` (no `-FromWorkingTree`).

- `release_id`: **20260529T161636Z-247f2973b2a4**
- `git_commit`: **247f2973b2a47cbef232bcc41524825f329b4b30** (committed Task-0013
  HEAD on `master`; subject "task-0013 PASS-0002: hotkey toggles visibility only")
- `source_mode`: **git_commit**, `source_dirty`: **false**
- `repository_dirty`: true — the unrelated untracked/working-tree files
  (`token_time.py`, the token-usage CSVs, Task-0009/0012 edits, the stray junk
  file) are recorded in `source_status` for transparency but were **NOT shipped**:
  the release `files[]` manifest contains only the **21** committed `app/` tree
  files, and a programmatic check confirmed **no `token_time` file** is present in
  the release (`contains_token_time=false`).
- Full manifest: [published-release-manifest.json](./published-release-manifest.json).

The publish copies `git archive HEAD app`, so a dirty working tree cannot leak
into the release.

## 3. Restarted the human's overlay onto the pinned release (data preserved)

- Before: one overlay (PID 67656) pinned to the prior release
  `20260529T143554Z-e99ac895ee61` (commit `e99ac895`, pre-show/hide fix).
- Stopped ONLY that prior-release overlay (matched on the prior release id), then
  ran `scripts\Start-DashboardRelease.ps1` (validates the manifest hashes,
  launches the runtime-root launcher, then runs `Test-DashboardRelease.ps1`).
- After: one overlay (PID **64592**) pinned to the NEW release.

### Data / config / startup preservation (Decision A — kept the data root)

[live-data-preservation.json](./live-data-preservation.json):

- `config.json` — byte-identical (sha256 `D1BF7990…`, 251 bytes, mtime
  2026-04-21 unchanged). **Untouched.** The restarted overlay uses this LIVE
  config: its command line has **no `--config-path` override**
  (`running_cmdline_has_config_path_override=false`).
- Startup `CodexDashboard.cmd` — content-identical (sha256 `BA8F5401…` matches the
  pre-publish baseline). Still points at the runtime-root pinned launcher.
- `dashboard.db` — **same file** at `%LOCALAPPDATA%\CodexDashboard\dashboard.db`,
  same creation time `2026-04-03T00:33:38Z` (not recreated/reset). With the old
  overlay stopped the DB was readable: sha256 `C4ADB6DC…`, ~272.96 MB. It is now
  reopened and being written by the human's restarted live overlay (byte count
  grows as the live instance ingests into its own DB). **Not migrated, reset,
  deleted, or overwritten.**

## 4. Isolation verification (SMOKE-003 step 4)

`scripts\Test-DashboardRelease.ps1` -> [test-dashboard-release.json](./test-dashboard-release.json):

- `current_release.release_id` = `20260529T161636Z-247f2973b2a4`,
  `git_commit` = `247f2973…`, `source_mode` = `git_commit`, `source_dirty` = false.
- `current_release_error` = **null** (all 21 copied source hashes validate).
- `startup_uses_pinned_launcher` = **true** (startup invokes the runtime-root
  launcher, not `C:\Agent\CodexDashboard`).
- `running_process_count` = **1**; the running `pythonw.exe` command line includes
  `--release-id 20260529T161636Z-247f2973b2a4 --release-root
  …\dashboard-releases\20260529T161636Z-247f2973b2a4` and no isolated-config
  override, so it serves the human's LIVE config.

## 5. Human-surface proof from the restarted, release-pinned overlay

All captures load the dashboard package FROM THE NEW PINNED RELEASE ROOT
(`release_overlay_capture.py`, `loaded_from_release_root: true`,
`loaded_ui_module_path` under the `20260529T161636Z-247f2973b2a4` release) against
the task-owned isolated config + SQLite DB under `../Runtime/` (synthetic Codex +
Claude fixtures). **No live data was read** (`reads_live_data: false`); the live
`dashboard.db`, `config.json`, `C:\Users\gregs\.codex`, and `~/.claude` were not
touched, so the human's live spend is not exposed.

- [smoke-all/overlay.png](./smoke-all/overlay.png) — the **OBSIDIAN** brand,
  **7D TOTAL TOKENS 138.5M** (merged Codex+Claude), control reads **Source: All**.
  Summary `7d_total=138.5M`, `hotkey_triggered=True`, `active_tab=usage`.
- [smoke-claude-off/overlay.png](./smoke-claude-off/overlay.png) — after
  unchecking Claude via the released `_toggle_source`: control reads
  **Source: Codex**, **7D TOTAL TOKENS 137.7M** (Claude's 0.839M removed).
  Summary `7d_total=137.7M`.
- [release-overlay-expanded.png](./release-overlay-expanded.png) — the released
  source-filter dropdown **expanded**, showing the **Codex** and **Claude**
  checkbuttons.
- [release-capture-summary.json](./release-capture-summary.json) — deterministic,
  occlusion-proof numeric record: merged 138,539,000 = Codex 137,700,000 +
  Claude 839,000 (`merged_equals_codex_plus_claude: true`, `claude_present: true`);
  displayed before/after All/138.5M -> Codex/137.7M (`before_after_changed: true`).

Per the human directive for this run, the toggle latency itself is **already
proven by the committed e2e harness** (`activation_e2e_harness.py` /
[../E2E-TIMING-RESULT.json](../E2E-TIMING-RESULT.json) /
[../ACTIVATION-FIX-PROOF.md](../ACTIVATION-FIX-PROOF.md)) and was NOT re-measured
live in this run.

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
