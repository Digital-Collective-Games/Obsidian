# PASS-0008 — Frontend mutating controls (Create / Assign / Eject / Destroy / Dequeue)

Scope: TASK.md Goals 16–18; AC20–AC22; in-app cases REG-013 / REG-014 / REG-015 / REG-016.
Single-context implementation; producer testing only — the independent clean-context QA
verdict on the in-app surface is coordinator-arranged.

## What changed

The mutating controls were wired in the SAME [`ui.py`](../../../../app/codex_dashboard/ui.py)
edit as PASS-0007 (one cohesive replacement of the tab; doing it in two churny passes over
the same region would have been worse). The `worktrees_backend.py` POST helpers landed in
PASS-0007. PASS-0008 is the control wiring + behavior:

- **Create** — `CREATE WORKTREE` toolbar control → `create_worktree(selected_repo)` →
  refresh; requires a specific repo selected in the filter (or the sole registered repo).
- **Assign** — `ASSIGN TASK` on an **idle** row opens a popup listing open tasks from the
  existing `GET /api/v1/tasks` (via `tasks_backend.fetch_tasks_snapshot`) as id + title +
  state (no progress bars — E6); confirm → `assign_worktree(task_id, repo, worktree_id)` →
  the worktree flips allocated.
- **Eject** — on an **allocated** row → `eject_worktree(run_id, worktree_id)` → returns to
  idle in the view (folder kept); the freed task is dequeued by the backend.
- **Destroy** — on an **idle** row → `destroy_worktree(worktree_id)` → row disappears; on
  an **allocated** row the backend 409 is surfaced as a clear human-facing message and
  nothing is removed.
- **Dequeue** — standalone `DEQUEUE` on an allocated row → `dequeue_task(repo, task_id)`
  without ejecting (worktree stays allocated; issue stays open).
- Action methods reload the pool with `keep_status=True` so the action's own outcome
  message (notably the allocated-Destroy rejection) is not overwritten by a generic
  "pool refreshed" line.

## Unit proof (producer)

- `tests/test_worktrees_tab.py` covers the POST bodies for all five operations and the 409
  rejection-message surfacing; `tests/test_desktop_support.py` adds worktrees action tests
  (eject calls backend + reloads + renders; destroy-allocated surfaces the 409 message;
  dequeue calls backend without eject). Full suite green: **178 OK**.

## In-app proof (the closure bar) — real Tk surface vs the live backend

Same isolated backend setup as PASS-0007 (`http://127.0.0.1:24318`, throwaway registry +
git repos, isolated `TEMP`/runs/namespace; never the human service/validation lanes or
live data). Each interaction was driven through the REAL app action method / widget
callback (the task-owned [Runtime/capture_filtered.py](../Runtime/capture_filtered.py)
driver) and verified against the backend pool state before/after; result-state screenshots
that needed a guaranteed-foreground grab were captured with the product `--smoke-tab
worktrees` harness.

- **REG-013 Create** — [reg013-create-result/overlay.png](./reg013-create-result/overlay.png) +
  [reg013-create/overlay-summary.txt](./reg013-create/overlay-summary.txt): clicking
  CREATE provisioned a new idle worktree via `POST /api/v1/worktrees/create`
  (`new_worktree_ids=...wt-0004` first run; pool grew) and it appears idle in the view.
- **REG-014 Assign popup → bind** —
  [reg014-assign-popup/overlay.png](./reg014-assign-popup/overlay.png) shows the populated
  popup (`Task-0007 - Task-0007 [Waiting on you]`, no progress bars);
  [reg014-assign-bound-result/overlay.png](./reg014-assign-bound-result/overlay.png) +
  [reg014-assign-bound/overlay-summary.txt](./reg014-assign-bound/overlay-summary.txt)
  show the confirm bound the task onto the chosen idle worktree, which flipped to
  **allocated** (`after_status=allocated`, `after_task_id`, `after_gate=running`), reusing
  the same folder.
- **REG-015 Eject returns idle (folder kept)** — before/after via the smoke harness:
  [reg015-eject-before/overlay.png](./reg015-eject-before/overlay.png) (wt-0001 allocated,
  EJECT control) → UI eject → [reg015-eject-after/overlay.png](./reg015-eject-after/overlay.png)
  (ALLOCATED 00 / IDLE 02; same wt-0001 now idle, same path/folder kept). The eject endpoint
  returned `status=idle`; [reg015-eject/overlay-summary.txt](./reg015-eject/overlay-summary.txt)
  records `after_status=idle`, `folder_still_present=True`.
- **REG-016 Destroy (idle-only) + allocated rejection + standalone Dequeue** —
  - [reg016-destroy-idle/overlay-summary.txt](./reg016-destroy-idle/overlay-summary.txt):
    Destroy on an idle worktree removed it (`still_present=False`).
  - [reg016-destroy-allocated-reject/overlay.png](./reg016-destroy-allocated-reject/overlay.png) +
    summary: Destroy on the allocated worktree was **rejected** (status message "Destroy
    failed: ... HTTP 409: worktree is allocated; eject it before destroy"); `still_present=True`,
    `still_status=allocated` — nothing removed.
  - [reg016-dequeue/overlay-summary.txt](./reg016-dequeue/overlay-summary.txt): standalone
    Dequeue on the allocated worktree left it **allocated** (`after_status=allocated`),
    status "Dequeued ... (Queue=Never); the issue stays open." — no eject, not a close.

## Caveats (honest)

- **In-app dequeue is a safe no-op against this binding.** The mux's standalone
  `taskService` is built with `NewService` (no `DequeueProvider` wired — that wiring lives
  only in the per-repo queue-drain `dispatchFor` path), so Eject's dequeue and the
  standalone Dequeue call through `DequeueTask` resolve to a safe no-op (no real GitHub
  write) on this single-Service control plane. This is consistent with "never touch a
  production-owned queue." The actual `Queue=Never` provider write + the no-bounce-back
  consequence are unit-proven against a fake provider in the backend batch
  ([PASS-0004](../PASS-0004/PASS-0004-NOTES.md) / [PASS-0005](../PASS-0005/PASS-0005-NOTES.md));
  PASS-0009's REG-007 re-run exercises the live provider write against the throwaway
  testbed.
- **Test-environment churn (recorded, not a product defect).** Reaching clean in-app
  captures required isolating the backend's owned-lane root (it defaults to
  `%TEMP%\cdxow`, shared across backends) via an isolated `TEMP`, and using a pristine
  throwaway git repo + a fresh run id, because reusing the same git repo / run id
  (`taskrun--Task-0007--active`) across repos and namespaces created git-worktree
  admin-name collisions and a stale cross-repo binding. These are harness-setup artifacts
  of repeated re-seeding; the UI controls invoked the real endpoints correctly throughout
  and faithfully surfaced backend results/errors.
- Producer testing only; the independent in-app QA verdict is coordinator-arranged
  (PASS-0009 + the human Chrome session for the REG-007 real-UI Ready flip remain).
