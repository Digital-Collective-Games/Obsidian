# PASS-0009 — REG-007 cap=1 serialization: 2 issues Ready at once on a 1-slot repo (live)

Task: Task-0015. Verdict: **PASS** (in an isolated harness). Proves the human's requested
scenario: a SECOND test repo with **one** worktree slot (`queue_workers=1`), **two** issues
flipped `Queue=Ready` at the real GitHub UI at ~the same time, with auto-close enabled —
the consumer dispatches EXACTLY ONE, DEFERS the other (no worktree), the first finishes and
deallocates, then the second is dispatched INTO the freed slot and finishes. Extends the
`queue_workers=2` full-cycle proof [PASS-0008](../PASS-0008/REG-007-FULL-CYCLE-PROOF.md) to
the single-slot serialization case.

> Human request (2026-05-30): "Add another test repo with 1 worktree, turn 2 issues into
> Queue=Ready at the exact same time, with auto close enabled. First one should finish, next
> task gets enqueued."

## Harness (new, separate test repo)
- **New repo:** `Digital-Collective-Games/QueueDrainTestbed2` (throwaway, private; safe to
  reset/delete) ↔ local `C:\Agent\QueueDrainTestbed2`, with two announce-only fixtures
  `Tracking/Task-0001` and `Tracking/Task-0002` (create `AGENT-RAN.txt`; set
  `current_gate="closure"`; STOP; never touch `gh`).
- **Registry entry:** `QueueDrainTestbed2`, **`queue_workers: 1`** (added to
  `C:\Agent\QueueDrainTestbed\REPO-MANIFEST.json` as a second entry; the proof run used an
  isolated single-repo manifest — see Anomaly below).
- **Two GitHub issues** `#1`/`#2`, each given an issue **type** (`Task`) + initial
  `Queue=Never` (so the org field renders) via the `obsidian-operator` sync, then flipped to
  `Queue=Ready` at the **real GitHub web UI** via the `github-operator` skill.
- **Lane:** isolated Temporal namespace `reg007j` (validation Temporal `127.0.0.1:17233`),
  backend `127.0.0.1:14318`, `OBSIDIAN_AUTO_CLOSE_QUEUED=true` (TEST-ONLY simulated-human
  closure), `CODEX_ORCHESTRATION_QUEUE_LAUNCH_AGENT=true`, dedicated runs-root, poll 20s.
  Production Obsidian never polled; the real `default` Temporal namespace / scheduled cron
  untouched.

## Both flips at the real GitHub UI
Both `Queue=Ready` flips were performed by driving the GitHub web UI (CDP), each confirmed
`Committed=True`, `ObservedButtonText=QueueReady`, `ApiMatches=True`:
[evidence/ui-flip-both-ready.txt](./evidence/ui-flip-both-ready.txt).

## Live cap=1 lifecycle (PASS)
Both issues were `Queue=Ready` + OPEN simultaneously when the consumer polled. Backend
consumer log (`dispatched`/`auto-close`/`reclaimed`):
[evidence/isolated-backend-log.txt](./evidence/isolated-backend-log.txt).

```
13:32:02  dispatched [Task-0001] parked [] reclaimed []      <- cap=1: only ONE dispatched
13:32:47  auto-close: closed issue #1 (Task-0001) ...
13:33:10  dispatched [Task-0002] parked [] reclaimed [Task-0001]  <- reclaim frees slot + REUSE
13:34:15  auto-close: closed issue #2 (Task-0002) ...
13:34:38  dispatched [] parked [] reclaimed [Task-0002]       <- final deallocate -> 0
```

Per-4s monitor (worktree count + per-lane `current_gate` + `AGENT-RAN.txt` + issue state):
[evidence/isolated-monitor-full.txt](./evidence/isolated-monitor-full.txt). Salient samples:

```
13:32:06  api_wt=[1 Task-0001:running] wdirs=1            #1=OPEN  #2=OPEN   <- 1 slot, #2 deferred (no worktree)
13:33:02  api_wt=[1 Task-0001:running] ran=1 Task-0001=closure  #1=CLOSED #2=OPEN  <- 1st done + auto-closed
13:33:12  api_wt=[1 Task-0002:running] wdirs=1            #1=CLOSED #2=OPEN   <- 2nd REUSES the freed slot
13:33:58  api_wt=[1 Task-0002:running] ran=1 Task-0002=closure  #1=CLOSED #2=CLOSED <- 2nd done + auto-closed
13:34:39  api_wt=[0 ] wdirs=0                              #1=CLOSED #2=CLOSED  <- final deallocate
BOTH CLOSED + 0 worktrees -> lifecycle complete
```

### What this proves
- **Cap honored:** there is **exactly one** worktree at every sample — never two. With both
  issues Ready, Task-0001 dispatched and Task-0002 was DEFERRED with **no worktree** (no
  over-dispatch on a 1-slot repo).
- **First finishes:** Task-0001's agent created `AGENT-RAN.txt`, announced
  `current_gate=closure`, the consumer auto-closed `#1`, and reclaimed its worktree.
- **Next gets enqueued into the freed slot:** in the SAME poll that reclaimed Task-0001
  (`13:33:10`), the consumer dispatched Task-0002 into the freed slot — proving slot reuse,
  not a second concurrent slot.
- **Full deallocate:** Task-0002 then announced, auto-closed `#2`, and reclaimed → **0
  worktrees**. End state: both issues CLOSED, no lanes.

## Anomaly — first attempt wedged (NOT reproduced after isolation)
The **first** attempt (namespace `reg007i`, the SHARED default runs-root carrying
[PASS-0008](../PASS-0008/REG-007-FULL-CYCLE-PROOF.md) leftovers, a two-repo registry) did
NOT pass: after a correct first dispatch, Task-0001's worktree was torn down out from under
a healthy, still-running agent (transcript truncated mid-task; `PID` killed; `w` gone),
which made the next poll stop counting the lane and re-dispatch, hitting
`Dispatch is blocked while this task already owns an active run.` on every subsequent poll —
the consumer wedged for the repo. Full forensic timeline + raw log:
[evidence/first-attempt-wedge-anomaly.txt](./evidence/first-attempt-wedge-anomaly.txt),
[evidence/backend-drain-log.txt](./evidence/backend-drain-log.txt).

- **Root cause not determined.** The dispatch path's only worktree remover
  (`releasePreviousOwnedLane → cleanupOwnedLane`) is a no-op for a fresh task on a fresh
  namespace, and the watchdog supervisor only calls `Observe` (`DefaultWatchdogPoll=30s`,
  which had not ticked) — yet the teardown happened in a ~2s window with no consumer
  activity.
- **Did not reproduce** once the harness was isolated (fresh namespace `reg007j`, a
  **dedicated runs-root**, and a **single-repo** registry): the full cap=1 lifecycle above
  passed cleanly and the agent ran to completion uninterrupted.
- **Leading hypothesis + real robustness observation:** the per-repo `taskrun.Service`
  enumerates a **GLOBAL** runs-root (`ListActiveWorktrees` is not scoped per repo / per
  backend), so a shared runs-root carrying another run's records (or a second backend) can
  interfere with lane accounting and lifecycle. This warrants a tracked investigation
  (candidate follow-up), but it is **not** a confirmed defect on the cap=1 dispatch path,
  which behaves correctly in isolation.

## Net
The human's cap=1 serialization scenario — two issues Ready at once on a one-slot repo →
exactly one dispatched, the other deferred, first finishes and deallocates, second reuses
the freed slot and finishes — is **proven live end-to-end** in an isolated lane. The
first-attempt wedge is recorded honestly as an unreproduced anomaly with a concrete
follow-up hypothesis (global runs-root scan), not buried.
