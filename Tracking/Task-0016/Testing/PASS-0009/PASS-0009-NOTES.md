# PASS-0009 — Final-code in-app re-capture of the WORKTREES tab (post UPDATE 5 / B1+B2 fix)

These captures satisfy INTERFACE-DESIGNER finding **B3** (re-take the human-surface
regression set on the SHIPPED, post-fidelity-fix panel) and are the PASS-0009 UI-surface
evidence for REG-010 … REG-016. They were taken on the FINAL code at fix commit
`4967cf9` (short-repo heading B1 + chip state mark B2 + empty-pool message).

## Isolated throwaway lane (never the human/service/validation lanes or live data)

- **Backend:** the control-plane binary (built from the local checkout) on a FRESH port
  `http://127.0.0.1:24319`, Temporal namespace `reg016fix`, isolated `TEMP`/owned-lane
  root (`<lane>\temp\cdxow`), isolated runs root, isolated jobs root. NOT the human
  service lane (`:4318`), NOT the pre-existing validation lane (`:14318`), NOT the human
  dashboard config/db, NOT live Codex data. Lane root:
  `Tracking/Task-0016/Testing/Runtime/reg-lane-fix`.
- **NO production GitHub provider.** The throwaway registry (`reg-lane-fix/REPO-MANIFEST.json`)
  registers `RepoX` + `RepoY` with **no `task_provider`** at all. Per
  `registryDequeueProvider.resolveSlug` (queuedrain.go), a repo with no GitHub provider is
  a SAFE no-op, so the post-fix Eject's `Queue=Never` dequeue and the standalone Dequeue
  are guaranteed local no-ops (no real GitHub write, no production queue touched). The
  queue-drain consumer also has no real repo to poll.
- **App:** the real desktop `DashboardApp` Tk surface, task-owned config
  (`Runtime/config.json`, isolated SQLite + fixture codex_root), pointed at `:24319` via
  `CODEX_DASHBOARD_WORKTREES_BACKEND_URL` / `CODEX_DASHBOARD_TASKS_BACKEND_URL`. Each
  interaction driven through the REAL app action methods / widget callbacks via the
  task-owned [Runtime/capture_filtered.py](../Runtime/capture_filtered.py) driver; PNGs via
  the product `write_overlay_capture`.
- **Seed:** Create provisioned idle worktrees in `RepoX`; a committed dispatch-ready local
  task `Task-9001` was Assigned onto one to produce an ALLOCATED worktree, leaving a
  mixed pool (1 allocated + 1 idle) for the pool view.

## Fidelity fix confirmed in the new captures

- **B1 (short repo heading):** every panel HEADING shows the short repo token `RepoX` in
  BOTH allocated and idle states — no full filesystem path in the heading. The full bound
  checkout path appears only in the DETAILS reveal.
- **B2 (chip state mark):** the status chip carries a leading state MARK — a FILLED cyan
  square for allocated, a HOLLOW/outlined green square for idle — within the existing
  cyan/green family, 0px-radius, no emoji.

## Captures (REG-010 … REG-016)

- **REG-010** pool view + allocated/idle color + short-repo heading + chip mark:
  [reg010-pool-view/overlay.png](./reg010-pool-view/overlay.png)
  (POOL 02 / ALLOCATED 01 / IDLE 01; `RepoX` headings; filled-vs-hollow chip marks).
  - DETAILS reveal: [reg010-details-reveal/overlay.png](./reg010-details-reveal/overlay.png)
    (short `RepoX` heading + filled chip; full path + worktree id + task/run/gate in the body).
  - backend-unavailable: [reg010-backend-unavailable/overlay.png](./reg010-backend-unavailable/overlay.png)
    (clear human-facing error, counts 00, no crash).
- **Empty-pool message** (reachable backend, zero worktrees in the filtered repo):
  [reg012-filter-tabB/overlay.png](./reg012-filter-tabB/overlay.png) shows
  "No worktrees in this repo yet - use CREATE WORKTREE to add one." (RepoY filter).
- **REG-011** copy-path: [reg011-copy-path/overlay-summary.txt](./reg011-copy-path/overlay-summary.txt)
  — `clipboard_matches_backend_path=True`.
- **REG-012** registry-sourced repo filter:
  [reg012-filter-tabA/overlay.png](./reg012-filter-tabA/overlay.png) (RepoX) +
  [reg012-filter-tabB/overlay.png](./reg012-filter-tabB/overlay.png) (RepoY, empty).
  Dropdown = `All repos, RepoX, RepoY` (from `GET /api/v1/repos`).
- **REG-013** Create: [reg013-create/overlay-summary.txt](./reg013-create/overlay-summary.txt)
  — `new_worktree_ids=RepoX/wt-0003`; pool grew, appears idle.
- **REG-014** Assign popup → bound:
  [reg014-assign-popup/overlay.png](./reg014-assign-popup/overlay.png) (popup lists
  `Task-9001 - Reg lane seed task. [Running]`, id+title+state, no progress bars — E6) +
  [reg014-assign-bound/overlay-summary.txt](./reg014-assign-bound/overlay-summary.txt)
  (`after_status=allocated`, `after_task_id=Task-9001`, `after_gate=running`; folder reused).
- **REG-015** Eject → idle, folder kept:
  [reg015-eject/overlay-summary.txt](./reg015-eject/overlay-summary.txt)
  (`after_status=idle`, `folder_still_present=True`). The in-eject dequeue was a safe
  no-op (no provider on RepoX).
- **REG-016** Destroy idle / destroy-allocated reject / standalone Dequeue:
  - [reg016-destroy-idle/overlay-summary.txt](./reg016-destroy-idle/overlay-summary.txt)
    (`destroyed_worktree=RepoX/wt-0002`, `still_present=False`).
  - [reg016-destroy-allocated-reject/overlay.png](./reg016-destroy-allocated-reject/overlay.png)
    + summary (HTTP 409 "worktree is allocated; eject it before destroy"; `still_present=True`,
    `still_status=allocated`).
  - [reg016-dequeue/overlay-summary.txt](./reg016-dequeue/overlay-summary.txt)
    (`after_status=allocated`, "Dequeued Task-9001 (Queue=Never); the issue stays open." —
    safe local no-op, not a close).

## Honest caveats

- **No live GitHub dequeue write is claimed here.** The throwaway lane has no GitHub
  provider, so Eject/Dequeue resolve to a safe local no-op. The LIVE `Queue=Never` provider
  write + the no-re-dispatch consequence remain a separate PASS-0009 step needing the
  human-authenticated Chrome session against a throwaway GitHub testbed; NOT attempted here
  and NOT proven by these captures.
- Producer-driven captures via the task-owned harness; the independent clean-context QA
  verdict + a follow-up INTERFACE-DESIGNER re-check on this final surface remain
  coordinator-arranged. The harness `capture_filtered.py` got one guard so the
  backend-unavailable pool readback tolerates an unreachable backend (harness-only; not
  product code).
