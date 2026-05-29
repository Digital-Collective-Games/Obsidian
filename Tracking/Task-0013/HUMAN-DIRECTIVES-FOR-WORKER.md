# Human Directives For Codex — Task-0013

Captured for Task-0013 from the current Codex session on 2026-05-29.

These directives are authoritative scope for Task-0013. Human directives override
subagent, coordinator, and agent-auditor preferences.

## Verbatim Human Directive

> I think we should have the desktop app be rebranded as Obsidian, and have the
> token counts taken from claude added to the token counts in codex. Right now
> its just codex. We ingest into a database, which by the way is very slow when
> i hit the keystroke Ctrl+Alt+Shift. So I think use TaskCreate to document this;
> and don't let the auditor dissuade you, just override if it says something
> stupid.

## Worker-Safe Normalization

The human bundled three improvements to the **same desktop app** and asked to
document them together as one task ("document this"). Treat them as one task with
three distinct objectives unless a genuine durable-rule or correctness problem
forces otherwise:

1. **Rebrand** the desktop app from "CodexDashboard" to "Obsidian".
2. **Merge token sources**: add Claude (Claude Code) token counts to the existing
   Codex token counts, which today are Codex-only, so displayed totals reflect
   both sources.
3. **Fix the overlay-activation slowness**: activating the overlay via the global
   hotkey is "very slow." The human said the keystroke is `Ctrl+Alt+Shift`; the
   shipped default is `Ctrl+Alt+Space`. Do not assume — the task must confirm the
   actual configured hotkey and target the real slow path (overlay activation
   currently runs a synchronous DB refresh on the UI thread).

## Auditor-Override Directive (explicit)

The human directed: "don't let the auditor dissuade you, just override if it says
something stupid."

Interpretation for the TaskCreate coordinator:

- Explicit human scope (the three bundled objectives, kept as one task) is
  authoritative. Do NOT let an agent auditor split, broaden, narrow, re-sequence,
  or block this task merely because it prefers a different shape. Override
  preference-based pushback and present the gate as human-overridden.
- "Override if it says something stupid" means override *preference* objections.
  It does NOT mean ignore genuine findings. Real correctness risks, durable
  shared/repo-rule violations, missing proof, or infeasibility are still valid and
  should be incorporated. Examples of findings that are NOT "stupid" and must be
  addressed if raised:
  - renaming the `%LOCALAPPDATA%\CodexDashboard` data/config/DB path (or the
    scheduled-task name / startup scripts) without a migration would orphan the
    user's existing database and settings;
  - merging Claude tokens requires correctly parsing Claude's session/token
    format, which differs from Codex's, and must not double-count or mislabel.

## Scope Boundary Notes

- This is a CodexDashboard (soon "Obsidian") desktop-app task.
- The GitHub provider repo is already named `Digital-Collective-Games/Obsidian`,
  so the rebrand aligns the local app name with the existing remote identity.
- Out of scope unless the human expands: the drain-queue consumer work, backend
  orchestration changes beyond what the rebrand path strictly requires, and any
  cross-repo work.

## 2026-05-29 — Dispatch Directive (auto-approve gates; coordinator owns quality)

### Verbatim Human Directive (TaskDispatch launch, 2026-05-29)

> Ground yourself in TaskDispatch for task 13 and start implementation.
> Auto-approve all gates; coordinator you are responsible for quality on this
> task, pay close attention to human directives.

### Worker-Safe Normalization

- The human authorized auto-approval of this task's own progress gates. Proceed
  through the plan gate, pass gates, and phase gates without stopping to wait for
  a separate human approval click. The coordinator (not you) owns the quality
  review of those gates, so you do not need to pause for human sign-off to move
  between phases.
- Auto-approval covers ordinary lifecycle gates only. It does NOT authorize, and
  you must still surface (not silently take), any action that would: expand scope
  beyond the three documented objectives; spend money or use a paid provider;
  delete, overwrite, or migrate the human's live data without the
  data-preservation guarantee this task already requires; or violate a shared or
  repo-root workflow rule.
- Repo-root `REGRESSION.md` and `DATA-HANDLING.md` still bind. Task-closure
  regression and the Objective-3 activation timing proof must run on an isolated
  lane with task-owned fixtures and a task-owned SQLite database. Do not point
  validation, regression, ingest, or timing measurement at the human's live
  dashboard config, the live `%LOCALAPPDATA%\CodexDashboard\dashboard.db`, or the
  live `C:\Users\gregs\.codex` Codex data unless the human explicitly authorizes
  that specific run. For the "database at least as large as the current live DB"
  requirement, build and measure against a task-owned synthetic database seeded
  to a documented realistic size and state/justify that assumption; do not open
  or read the human's live database to size it.
- Git hygiene: the working tree contains pre-existing changes unrelated to
  Task-0013 (other tasks' tracking edits and an in-progress token-time CSV
  feature: e.g. `app/codex_dashboard/token_time.py`,
  `scripts/add_total_time_to_token_usage_csvs.py`, `tests/test_token_time.py`,
  and modified `Tracking/Task-0009`/`Task-0012` files). Stage and commit ONLY
  Task-0013 files. Do not stage, commit, revert, or "clean up" the unrelated
  pre-existing changes, and do not let them block your work.
- "Pay close attention to human directives": the three bundled objectives stay
  one task; the auditor-override directive above remains authoritative; preserve
  the human-facing outcomes (the app reads "Obsidian", totals include Claude,
  activation feels instant) rather than narrowing them into weaker technical
  proxies.

## 2026-05-29 — Added Objective 4 (source filter dropdown)

### Verbatim Human Directive

> Should include a drop-down with checkboxes for claude, codex to filter token
> aggregation - at least make sure token ingestion carries that distinction so we
> can later iterate on the distinction

### Worker-Safe Normalization

- This adds a fourth objective to Task-0013 by explicit human direction. It
  OVERRIDES the prior "per-source split UI view" non-goal. The concrete spec is
  recorded in `TASK.md` as `### Objective 4 — Source filter (Codex/Claude)` with
  its own acceptance criteria and falsifier; re-ground on `TASK.md` before
  building it.
- Hard floor (non-negotiable, holds regardless of how far the UI gets): token
  ingestion stores the per-event `source` distinction (`codex` / `claude`). This
  is already required by Objective 2 — keep it.
- Build the dropdown within Task-0013 now (the human chose "build now" over
  deferring it). It is a source filter over the displayed token aggregation: a
  control with separate Codex and Claude checkboxes; unchecking a source removes
  its events from the displayed totals, projections, and charts.
- The filter operates on the stored `source` column and must NOT reintroduce a
  synchronous UI-thread database read (filter the already-loaded in-memory
  snapshot and re-render), so Objective 3's non-blocking activation is preserved.
- Default selection is all sources checked (today's merged behavior). Match the
  overlay's existing visual style.
- New human-facing functionality: update repo-root `REGRESSION.md` with a named
  case (or extend an existing case) that covers the filter interaction, per the
  repo regression rule.

## 2026-05-29 — Publish + restart authorization (live-lane, explicit)

### Human Directive (coordinator gate, 2026-05-29)

The human was asked whether to publish a new dashboard release and restart their
pinned overlay so the Task-0013 changes go live, and chose **"Publish + restart
now."**

### Worker-Safe Normalization

- This EXPLICITLY authorizes, for this specific run, the otherwise-forbidden
  live-lane action: publish a new pinned dashboard **frontend** release and
  restart the human's running overlay so the committed Task-0013 changes become
  what the human sees.
- Publish from the **committed Git tree** (the Task-0013 work is committed on
  `master`/`upstream`), following the repo's canonical dashboard release-isolation
  flow — see repo `TESTING.md` (SMOKE-003), `DATA-HANDLING.md`, and
  `scripts\Publish-DashboardRelease.ps1` / `scripts\Start-DashboardRelease.ps1` /
  `scripts\Test-DashboardRelease.ps1`. Do not publish a dirty working tree (the
  tree contains unrelated untracked files that must not ship).
- Decision A kept the `%LOCALAPPDATA%\CodexDashboard` data root, so the human's
  existing `dashboard.db`, `config.json`, and startup entry MUST carry over
  unchanged. Do NOT migrate, reset, delete, or overwrite the human's data or
  config; the restart must preserve them.
- After restart, verify the running overlay process is the pinned release (release
  id + release root), per the release-isolation checks.
- Capture human-surface proof from the restarted, release-pinned overlay under
  `Tracking/Task-0013/Testing/`: the OBSIDIAN brand, merged Codex+Claude totals,
  and the source-filter control **expanded** showing the Codex and Claude
  checkboxes plus a visible before/after of toggling a source.
- This authorization is limited to publishing + restarting this task's release and
  capturing proof. It does not authorize touching unrelated lanes, data, or
  changing the human's settings.

## 2026-05-29 — Activation latency follow-up investigation (Objective 3)

### Verbatim Human Directive

> The desktop ctrl alt space does seem much faster, but its still kind of clunky i
> don't think its <50ms. Time from key hit to rendered window seems more like
> >250ms. Can you investigate what its doing and postulate a fix? Give me a full
> report on what its doing.

Revised estimate (same session):

> revise that more like 500 ms to rendered window, clunky

### Worker-Safe Normalization

- This is an Objective-3 follow-up. The implemented fix met its WRITTEN budget
  (UI-thread blocking time ~5 ms), but the human reports the PERCEIVED end-to-end
  latency from key-press to a fully rendered window is roughly >250 ms and still
  feels clunky. The human-facing outcome ("feels instant") — not the UI-thread
  block proxy — is what matters here. (Human revised the estimate to ~500 ms,
  up from >250 ms.)
- Deliverable for THIS run is an EVIDENCE-BASED INVESTIGATION REPORT (durable, e.g.
  `Tracking/Task-0013/Testing/ACTIVATION-LATENCY-INVESTIGATION.md`). It must:
  - Explain, in plain terms, exactly what the activation pipeline does from key
    press to a visible, painted window (hotkey detection / poll interval ->
    toggle_overlay -> show_overlay -> deiconify/lift/focus -> window map/visible ->
    `_render_dashboard`: source filter + bucket build + chart draw + labels).
  - MEASURE the end-to-end wall-clock broken down by phase (not the UI-thread block
    alone), so the dominant contributor(s) to the >250 ms are identified with real
    numbers. Instrument the actual activation code path and/or extend the existing
    `activation_timing_harness.py`; do not rely on speculation.
  - Explain why the prior ~5 ms proof did NOT capture this (it measured only the
    synchronous UI-thread DB read, not poll latency + deiconify + actual paint).
  - POSTULATE one or more concrete fixes targeting the dominant phase(s), with
    tradeoffs and an expected improvement. Do NOT implement the fix in this run —
    the human asked to investigate + postulate + report first.
- Measure on a task-owned synthetic database sized comparably to a large real DB
  (the live DB is large, ~270 MB-class). Honor `REGRESSION.md`/`DATA-HANDLING.md`:
  isolated lane / task-owned fixtures only; do NOT open or use the human's live
  `dashboard.db`, live config, `C:\Users\gregs\.codex`, or `~/.claude` data.
- Stage/commit only Task-0013 files if you commit the report; leave unrelated
  pre-existing changes untouched.

## 2026-05-29 — Activation fix approach: show/hide only, no rebuild on toggle

### Verbatim Human Directive

> Can't we just show/hide the desktop app? I don't want to rehydrate window state
> every time its toggled. The intent is just a hotkey to hide the window, not
> "rebuild your state from scratch" - the latter sounds asinine to me.

### Worker-Safe Normalization (authorized fix for the Objective-3 follow-up)

This run IMPLEMENTS the fix (the investigation report is done; the human chose the
approach). Authorized design:

- The overlay is a PERSISTENT window built and rendered ONCE. The global hotkey
  TOGGLES VISIBILITY ONLY (show/hide). Toggling must do NO re-aggregation, NO
  bucket rebuild, NO DB read, and NO full re-render — it only reveals/hides the
  already-rendered window. Remove the render call from the hotkey activation path.
- Keep displayed content current via the EXISTING background ingest poll (already
  off the hotkey path), not via work at toggle time. Freshness within one poll
  interval (~5 s) is acceptable; an instant show is the priority over
  perfectly-fresh-at-show.
- Pre-render at startup so even the FIRST hotkey press is fast (no first-show
  rebuild).
- Do not let the "keep it fresh in the background" path become wasteful: if the
  background poll would otherwise re-aggregate the full ~467k-event 7-day window
  every cycle, reduce that per-event cost (e.g. aggregate only the charted window
  plus a cheap rolling 7-day total — the report's Fix B) so background freshness
  is cheap. This is secondary to, and in service of, the show/hide decoupling.
- DO NOT regress the four shipped objectives. In particular, the Objective-4
  source-filter dropdown is a deliberate USER action that may re-render (or update
  from precomputed per-source buckets) — that is separate from the hotkey
  show/hide and must keep working without a synchronous UI-thread DB read.
- Prove it: extend the end-to-end harness (activation_e2e_harness.py) to measure
  the new TOGGLE latency (show/hide of the persistent window with no re-render) and
  record before/after. Target: toggle latency dominated only by OS window
  map/paint + hotkey detection, no per-event work (expect ~tens of ms vs the
  measured ~350 ms warm path). Add/keep unit coverage proving the hotkey toggle
  path performs no aggregation/DB read/full re-render.
- Scope = implement + unit tests + measured before/after proof + durable state.
  Do NOT publish or restart the human's live overlay in this run; that live-lane
  step is gated separately by the coordinator/human. Honor
  `REGRESSION.md`/`DATA-HANDLING.md` (isolated lane / task-owned fixtures; never the
  human's live config/DB/`.codex`/`~/.claude`). Commit ONLY Task-0013 files.

## 2026-05-29 — Publish authorization for the show/hide fix release (commit 247f297)

The human was asked whether to publish + restart for the verified show/hide fix
(commit `247f297`) and chose **"Publish + restart now."** The publish/restart
rules from the earlier "Publish + restart authorization" section apply unchanged:
publish from the COMMITTED tree (now `247f297`), restart the human's overlay onto
the new pinned release while PRESERVING the existing `%LOCALAPPDATA%\CodexDashboard`
`dashboard.db` / `config.json` / startup entry, verify release isolation + that the
running process is the pinned release on the live config, and capture human-surface
proof. Also (housekeeping, in-scope): delete the stray untracked junk file with the
mangled path under the repo root (e.g. `C:AgentCodexDashboard...verify_ui.py`), and
commit any still-uncommitted Task-0013 proof artifact (e.g.
`Testing/SOURCE-FILTER-RESULT.json`). Do not touch unrelated lanes/data or the
unrelated pre-existing working-tree changes.

## 2026-05-29 — Human acceptance + close-out

### Verbatim Human Directive

> Yes this does feel a lot better. I think we're good for now. You can close out
> the task.

### Worker-Safe Normalization

- The human tested the LIVE overlay after the show/hide fix went live and ACCEPTED
  the result (the hotkey now feels good). This satisfies the human-acceptance gate.
- Close out Task-0013: set durable state to complete/accepted and finalize the
  handoff. The four objectives plus the activation show/hide fix are implemented,
  verified, committed/pushed, and live on the pinned release; data was preserved.
- This is bookkeeping closure ONLY — make NO product code changes. Commit ONLY
  Task-0013 state/handoff files; leave the unrelated pre-existing working-tree
  changes untouched.

## 2026-05-29 — GitHub sync fix authorized (Task-0013 issue + recurrence prevention)

### Verbatim Human Directive

> Agree completely with the recommendations, proceed.

### Worker-Safe Normalization

Context: Task-0013 was never synced to GitHub (it is the only task with no
`TASK-META.json`; issue #13 does not exist; #13 is free — zero PRs in the repo).
See `Tracking/Task-0013/Testing/GITHUB-SYNC-FAILURE.md` for the diagnosis.

The human authorized BOTH recommended fixes:
- Instance fix (outward-facing GitHub writes now AUTHORIZED): via the
  `obsidian-operator` skill, create GitHub issue #13 from `Tracking/Task-0013`,
  write `TASK-META.json`, and — because local status is `complete` — close issue
  #13 as completed; then reconcile to 13/13/0. Number 13 must equal the task
  number; if the bootstrap safety stop fires (number ≠ 13), STOP and escalate.
- Recurrence prevention (shared `.codex` process edit AUTHORIZED): make
  "GitHub issue exists and `TASK-META.json` binds the same-number issue" a hard
  gate in the shared TaskCreate process (and at task closure), and optionally add
  a scheduled reconcile backstop. The coordinator owns/authorizes the `.codex`
  edit and its commit/push.
