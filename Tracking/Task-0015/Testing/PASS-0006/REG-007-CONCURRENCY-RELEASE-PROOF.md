# REG-007 — Concurrency / queuing + worktree-release + park (live, registry-driven)

Task: Task-0015. Pass: PASS-0006 / REG-007 (extended coverage). Verdict: **PASS**.
Augments [REG-007-PROOF.md](./REG-007-PROOF.md) (the single-issue end-to-end) with the
multi-issue queuing + worktree lifecycle scenarios.

Setup: registry-driven backend (`OBSIDIAN_REGISTRY_PATH` → testbed-only registry),
isolated `reg007f` Temporal namespace, **`queue_workers=2`** (lowered from 4 for a
cheap test), launch enabled, 30s poll. Three throwaway issues `#5/#6/#7` (→
`Task-0005/0006/0007`), each flipped `Queue=Ready` via the **real GitHub UI**
(`github-operator` `Set-IssueFieldViaUi.ps1`). Production Obsidian never polled; real
cron namespace untouched.

## 1. Concurrency / queuing (cap honored, extra deferred) — PASS

3 issues `Queue=Ready` with `queue_workers=2` → the consumer dispatched **exactly 2**
and **deferred the 3rd**, stable for 120s:
- backend: `queue-drain poll acted ... dispatched [Task-0005 Task-0006] parked [] reclaimed []`
- `/api/v1/worktrees` = 2 (`Task-0005`, `Task-0006`); `Task-0007` Ready but **not**
  dispatched (no worktree). The per-repo cap held — no over-dispatch.
- Evidence: [evidence/concurrency-2of3-cap2.txt](./evidence/concurrency-2of3-cap2.txt).

## 2. Worktree release on close + dequeue-next — PASS

Closed `#5` (human-approved `gh issue close --reason completed`) → the consumer's next
poll: `dispatched [Task-0007] parked [] reclaimed [Task-0005]`.
- `Task-0005`'s worktree was **reclaimed/released** (no longer listed by the endpoint).
- The freed slot **dequeued the deferred `Task-0007`** (now dispatched).
- Worktrees → {`Task-0006`, `Task-0007`}; cap 2 maintained.
- Evidence: [evidence/close-reclaim-dequeue.txt](./evidence/close-reclaim-dequeue.txt).

This answers the lifecycle question: the worktree is released **only** on a
human-approved CLOSE (the agent never self-closes), and the release frees the slot to
dequeue the next Ready issue ([A4.3](../../TASK.md)). Before a close, a dispatched run
correctly holds its worktree.

## 3. Park retains the worktree (the contrast) — PASS

Set `#6` `Human Needed=Yes` (the agent-side park path) → the consumer's next poll:
`parked [Task-0006]` (NOT reclaimed, NOT redispatched).
- `Task-0006`'s worktree was **retained**, `run_gate_state = parked_awaiting_closure`,
  stable for 60s, while `Task-0007` kept `running` (cap 2 held).
- Evidence: [evidence/human-needed-park-retains.txt](./evidence/human-needed-park-retains.txt).

This is [A4.4](../../TASK.md): `Human Needed=Yes` parks in place (worktree + slot
retained); only a CLOSED issue deallocates.

## Net

The consumer-driven concurrency/queuing (cap dispatched + extra deferred), the
worktree release-on-close + dequeue-next, and the park-retains contrast are all
exercised live end-to-end through the registry-driven binding. Cleanup: backend
stopped, agents killed, `#5/#6/#7` closed, worktrees pruned.
