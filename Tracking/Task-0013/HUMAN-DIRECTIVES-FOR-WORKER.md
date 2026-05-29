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
