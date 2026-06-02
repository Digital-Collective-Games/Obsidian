# Task-0017 — TaskCreate Context Manifest (worker-safe)

Durable files the writer-worker may read to draft `Tracking/Task-0017/TASK.md`.
Read what you need to make the draft concrete and cold-readable; you may also
read other repo files directly implied by these.

## Shared standards (authoritative for the writeup)

- `C:\Users\gregs\.codex\Orchestration\Processes\TASK-CREATE.md` — the writeup
  standard. Follow it exactly.
- `C:\Users\gregs\.codex\Orchestration\Exemplars\TASK.md` — intended TASK.md shape.
- `C:\Users\gregs\.codex\AGENTS.md` — glossary, human-facing outcome rule.

## The code the task changes / tests

- `app/codex_dashboard/ui.py`
  - `open_assign_popup` (~L1586) — the ASSIGN popup open path; the synchronous
    `fetch_tasks_snapshot(...)` at ~L1590 is the stall (part 1).
  - non-blocking pattern to mirror: `_start_activation_load` / `schedule_ingest`
    (~L2290–2331), the ingest-queue + `self.root.after(...)` drain, and
    `_poll_ingest_results`.
  - `_confirm_assign` (~L1983) and `eject_worktree_action` (~L1551) — the bind /
    clear actions (part 2).
- `app/codex_dashboard/tasks_backend.py` — `fetch_tasks_snapshot` (L29), the
  blocking HTTP call.
- `app/codex_dashboard/worktrees_backend.py` — `assign_worktree` (L51),
  `eject_worktree` (L64), and the other lifecycle client calls.
- `app/codex_dashboard/worktrees_tab.py` — worktree status/colors and the card
  helpers (so you know what "allocated" vs "idle" renders as on a card).

## Lifecycle + provider truth (so "done"/"cleared" is precise)

- `Tracking/Task-0016/TASK.md` — the worktree-pool backend contract
  (Create / Assign / Eject / Destroy / Dequeue, discover-on-startup, the
  human-only-closure done-contract, `Eject dequeues via the provider`). Part 2 is
  a test of this existing lifecycle.
- `skills/obsidian-operator/SKILL.md` — the Queue Done-Contract section: the
  agent never self-closes; only a human-closed issue deallocates a worktree;
  `Human Needed=Yes` parks in place.
- `REGRESSION.md` (repo root) — the existing WORKTREES-tab regression cases
  (REG-007..019). Cite the relevant case ids in the proof plan; do NOT redefine
  what counts as a regression lane.

## Repo conventions

- `AGENTS.md` (repo root) — repo-local operator lanes, the isolated validation
  lane (`http://127.0.0.1:14318`) vs the live human service lane (`:4318`).
- `TESTING.md` (repo root) — unit vs regression test conventions for this repo.

## Existing tasks (format reference only)

- `Tracking/Task-0014/TASK.md`, `Tracking/Task-0016/TASK.md` — examples of the
  in-repo TASK.md shape and the `<!-- task-sync ... -->` body header convention.

Prefer durable artifacts over chat memory. If a needed fact is missing, name it
as an Open Question rather than inventing it.
