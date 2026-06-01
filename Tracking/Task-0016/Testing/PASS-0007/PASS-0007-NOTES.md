# PASS-0007 — Frontend foundation: rename `TASKS` → `WORKTREES` + replace + read-only pool view

Scope: TASK.md Goals 12–15; AC16–AC19; in-app cases REG-010 / REG-011 / REG-012.
Single-context implementation (no nested dispatch available); producer testing only —
the independent clean-context QA verdict on the in-app surface is coordinator-arranged.

## What changed (surgical, matched existing style)

- **New HTTP client** [`app/codex_dashboard/worktrees_backend.py`](../../../../app/codex_dashboard/worktrees_backend.py)
  mirroring [`tasks_backend.py`](../../../../app/codex_dashboard/tasks_backend.py): URL
  config + env override (`CODEX_DASHBOARD_WORKTREES_BACKEND_URL`, falling back to the
  shared `CODEX_DASHBOARD_TASKS_BACKEND_URL`, then the `:4318` default),
  `fetch_pool_snapshot()` (GET `/api/v1/worktrees`), `fetch_repos()` (GET `/api/v1/repos`),
  the POST helpers (`create_worktree` / `assign_worktree` / `eject_worktree` /
  `destroy_worktree` / `dequeue_task`), the same `urllib` `_request_json`, and a
  backend-unavailable error snapshot. Backend `{"error": ...}` bodies are surfaced in the
  raised message (used for the 409 allocated-Destroy rejection).
- **New pure helpers** [`app/codex_dashboard/worktrees_tab.py`](../../../../app/codex_dashboard/worktrees_tab.py)
  mirroring [`tasks_tab.py`](../../../../app/codex_dashboard/tasks_tab.py): allocated-vs-idle
  **background-color selection** (`#173a44` allocated vs `#181c22` idle — a real,
  perceivable distinction in the existing palette), status accent/label, per-row detail
  lines, registry-sourced repo-filter options + repo matching, sort, summary counts, and
  the Assign-popup open-task projection (id + title + state only — mockup exclusion E6).
  The repo matcher joins on the stable `worktree_id` repo segment OR the `repo` field ==
  registry id OR the `repo` field path == registry `local_root`, because the backend's
  flattened `repo` field is the registry id for idle members but the bound checkout's repo
  path for allocated members.
- **`ui.py` rename + replace (D1=replace):** nav tuple `("tasks","Tasks")` →
  `("worktrees","Worktrees")`; `select_tab` / `_render_active_tab` / geometry tab-list /
  smoke-capture / mousewheel updated to `worktrees`; the old `_build_tasks_lane` +
  task-stream / detail / dispatch-pause-poke render/handler methods **removed** and
  replaced with the worktree-management surface (`_build_worktrees_lane`, pool render, repo
  filter, copy-path). The orphaned task-launch helpers (`_open_task_launch_target`,
  `_is_allowed_launch_command`, `_resolve_allowed_launch_command`, `_normalize_launch_uri`)
  and their two module constants — used only by the removed task tab — were removed, plus
  the now-unused `urllib.parse` import. `Usage` and `Jobs` tabs untouched; tab switch is
  read-only (no backend mutation on switch).

## Unit proof (producer)

- New [`tests/test_worktrees_tab.py`](../../../../tests/test_worktrees_tab.py): 18 tests —
  pure helpers (allocated/idle color distinct, repo-filter registry-sourced not hardcoded,
  repo matching joins on id segment for the allocated path-form repo, filter narrows /
  All-repos restores / unknown shows nothing, sort, counts, popup projection no progress)
  + the HTTP client (URL precedence, pool/repos mapping, POST bodies, the 409 message
  surfaced).
- Existing `test_desktop_support.py` / `test_overlay_geometry.py` updated for the rename
  (the geometry tall-layout tab list and the prime/select-tab tests now reference
  `worktrees`; the removed task-action/launch-target tests were dropped and replaced with
  worktrees action tests).
- Full suite green: `python -m unittest discover -s tests -p "test_*.py"` → **178 OK**.

## In-app proof (the closure bar) — real Tk surface vs the live backend

Tk pre-flight PASSED (the desktop surface renders and `write_overlay_capture` captures a
real app-surface PNG — see [Runtime/preflight/overlay.png](../Runtime/preflight/overlay.png)).

All captures ran the REAL desktop app against an **isolated** validation backend started
on `http://127.0.0.1:24318` (namespace `reg007*`, isolated `TEMP`/runs root, a throwaway
2-repo registry `WorktreesTabA`/`WorktreesTabB` over throwaway git repos) — never the
human's service lane (`:4318`), the pre-existing validation lane (`:14318`), the human
dashboard config/db, or live Codex data. The app was pointed at it via
`CODEX_DASHBOARD_WORKTREES_BACKEND_URL`.

- **REG-010** pool view + allocated/idle color + rename/replace + backend-unavailable:
  - [reg010-pool-view/overlay.png](./reg010-pool-view/overlay.png) — nav shows `WORKTREES`
    (no `TASKS`); 1 **allocated** row (distinct teal-navy bg, `ALLOCATED - RUNNING` chip,
    repo + local dir + id + task/run/gate) and 2 **idle** rows (darker bg, `IDLE` chip);
    `POOL SIZE 03 / ALLOCATED 01 / IDLE 02`.
  - [reg010-backend-unavailable/overlay.png](./reg010-backend-unavailable/overlay.png) —
    with the backend stopped, a clear human-facing error message, no crash, counts 00.
- **REG-011** copy-path: [reg011-copy-path/overlay.png](./reg011-copy-path/overlay.png) +
  [reg011-copy-path/overlay-summary.txt](./reg011-copy-path/overlay-summary.txt) — the
  copy control placed the **exact backend `worktree_path`** on the system clipboard
  (`clipboard_matches_backend_path=True`).
- **REG-012** registry-sourced repo filter:
  [reg012-filter-tabA/overlay.png](./reg012-filter-tabA/overlay.png) (WorktreesTabA →
  wt-0001 allocated + wt-0002 idle) and
  [reg012-filter-tabB/overlay.png](./reg012-filter-tabB/overlay.png) (WorktreesTabB →
  only wt-0003). Dropdown options = `All repos, WorktreesTabA, WorktreesTabB` (from
  `GET /api/v1/repos`, not hardcoded); selecting a repo narrows the pool, All-repos
  restores it.

## UPDATE 5 refinement — panel/card fidelity + information hierarchy

[HUMAN-DIRECTIVES UPDATE 5](../../HUMAN-DIRECTIVES-FOR-WORKER.md) raised the [FE] bar (it
does NOT change functional scope): each worktree must render as a PANEL/CARD matching the
Stitch mockup concept, glanceable by default with secondary/diagnostic info behind a
reveal, structurally adhering to
[INTERFACE-DESIGNER.md](../../../../../Users/gregs/.codex/Orchestration/Prompts/INTERFACE-DESIGNER.md).
Applied (against the mockup `screen.png` + `DESIGN.md` "Monolithic Terminal": 0px radius,
no dividers/borders, tonal surface elevation, Space Grotesk data/headings + Inter
metadata, status chips):

- **Glanceable face**: each panel shows the status chip, repo heading (Space Grotesk),
  the bound `Task` (allocated, cyan data), a **shortened** local dir (Inter metadata), and
  the action buttons. New `worktree_face_lines` + `shorten_path` pure helpers.
- **Secondary behind a reveal**: a per-panel **DETAILS** button opens a popup with the
  full secondary/diagnostic fields (full path, `worktree_id`, `run_id`, full
  `run_gate_state`, `agent_session_id`, `session_transcript_path`, `launched_pid`) — empty
  fields truthfully omitted (no fabricated agent-model chip, E4). `worktree_detail_lines`
  now carries the full field set; `worktrees_backend` maps `session_transcript_path` +
  `launched_pid`.
- **Full-path hover tooltip** on the on-face short path (`_bind_tooltip`).
- Allocated vs idle panels stay tonally distinct in the same family (`#173a44` vs
  `#181c22` on the `#1c2026` shell); tooltip uses the `#353940` interaction tier.

Re-captured: [reg010-pool-view-v2/overlay.png](./reg010-pool-view-v2/overlay.png) (the
refined glanceable panels) and [reg010-details-reveal/overlay.png](./reg010-details-reveal/overlay.png)
(the DETAILS popup with the full secondary fields). Unit suite green at **180** (added
face-lines, shorten-path, and detail-lines secondary-field tests).

A clean-context **INTERFACE-DESIGNER review** comparing the implemented tab against the
mockup concept (no blocking fidelity discrepancies) remains a coordinator-arranged closure
gate per UPDATE 5, alongside the functional in-app QA.

## Caveats

- Producer testing only; the independent in-app QA verdict + INTERFACE-DESIGNER review are
  coordinator-arranged.
- Captures use the smoke-capture harness (`--smoke-tab worktrees`) and a task-owned UI
  driver ([Runtime/capture_filtered.py](../Runtime/capture_filtered.py)) that drives the
  REAL app action methods / widget callbacks; both use the product's
  `write_overlay_capture`.
- The PASS-0009 consolidated in-app re-run (REG-010…016 + REG-007/008/009 under the new
  model, the latter needing the human-authenticated Chrome session for the real-UI Ready
  flip) remains a later coordinator-arranged step.
