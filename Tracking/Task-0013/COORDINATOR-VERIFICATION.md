# Task-0013 — Coordinator Verification & Acceptance (2026-05-29)

TaskDispatch coordinator independent verification of leader commit `e99ac89`
(pushed to `upstream/master`). Method: 8-agent adversarial verification workflow
(each verifier instructed to *refute*, defaulting to refuted/uncertain unless
evidence is airtight) plus a clean-context human-surface QA verdict. The
coordinator did not implement; an agent did the work.

## Verdicts — all CONFIRMED, no falsifier triggered

- **Commit hygiene:** 35 Task-0013 files only. Unrelated pre-existing work
  (`token_time.py`, the CSV feature, Task-0009/0012 edits) left uncommitted and
  untouched. 49 MB `timing.db` correctly gitignored; only committed binary is
  `overlay.png` (31 KB).
- **Obj 1 rebrand:** all seven user-facing strings read "Obsidian"; no residual
  user-facing product name; Decision A recorded in `paths.py` + `DATA-HANDLING.md`;
  data root / OS identifiers deliberately NOT renamed (existing data preserved).
- **Obj 2 Claude merge:** per-`requestId` dedup using the LAST assistant event
  (independently fixture-verified — 2 events, not the per-line sum); canonical
  total = input + cache_creation + cache_read + output; `source` /
  `source_event_id` columns + additive migration + idempotent upsert (3x re-ingest
  → stable row count); merged window total = Codex + Claude.
- **Obj 3 activation:** `event_timestamp` index present (verified via EXPLAIN
  QUERY PLAN); `show_overlay` performs NO synchronous UI-thread DB read;
  cold-start dispatches off-thread then populates (not blank); measured
  340 ms → ~5 ms (independently re-run ~390 → ~4.8 ms) on a 250k-row / 49 MB
  task-owned synthetic DB, under the stated 50 ms budget. The worker's honest
  caveat (the index barely changes the full-window read; the real win is removing
  the synchronous read) is accurate.
- **Obj 4 filter:** Codex/Claude checkbox control (`Menubutton`); toggling
  re-renders from the in-memory snapshot via `aggregation.filter_events_by_source`
  with no DB read; unit test asserts filtered total = sum of selected sources;
  `REG-005` added to `REGRESSION.md`.
- **Test suite:** 133 OK (130 committed + 3 from the untracked token-time work,
  left alone); no skips/xfails; independently re-run.
- **Regression isolation:** REG-001 + filter case on an isolated task-owned lane;
  every DB row uses fixture paths (zero live paths); live `dashboard.db` / config
  untouched (mtimes predate the run). `overlay.png` shows the real OBSIDIAN
  surface, 138.5M merged total, and the Source filter control.
- **QA (clean-context, human-surface only):** PASS — `overlay.png` visibly shows
  the OBSIDIAN brand, populated totals (7D 138.5M, projected 339.4M, headroom
  +5.4M), a real Token-Velocity bar chart, and a "Source: All" filter control, in
  one coherent real app capture.

## Honest residuals (none block source-level acceptance)

1. **Pinned-release gate (the remaining human-authorized step):** the human's
   running overlay is a *pinned release*. The changes are NOT visible to the human
   until a new release is published (`scripts\Publish-DashboardRelease.ps1`) and
   the overlay restarted — a live-lane action the worker correctly did NOT take
   (outside the auto-approval scope, which excludes live-data/human-lane actions).
2. **Filter visual:** the captured screenshot shows only the collapsed
   "Source: All" state; the expanded Codex/Claude checkboxes and a toggled
   before/after chart delta are proven by code + tests + `SOURCE-FILTER-RESULT.json`,
   not by a screenshot.
3. **Obj-3 DB size:** the synthetic DB (49 MB / 250k rows) is realistic but smaller
   in bytes than the live ~194 MB DB (the directive forbade opening the live DB to
   size it). The activation budget is on the size-independent snapshot render, so
   the outcome holds and would only be stronger on a larger DB.
4. The screenshot's token figures come from the isolated fixture lane, not live
   spend (expected — live data is off-limits).

## Coordinator acceptance

All four objectives + commit hygiene + test suite + regression + QA are ACCEPTED
at the source/repo level on the human's behalf (auto-approve per
`HUMAN-DIRECTIVES-FOR-WORKER.md`; quality independently verified above).
Implementation is complete, committed, and pushed. Realizing the human-visible
outcome on the running overlay requires the publish + restart step (residual 1),
which is pending explicit human authorization.
