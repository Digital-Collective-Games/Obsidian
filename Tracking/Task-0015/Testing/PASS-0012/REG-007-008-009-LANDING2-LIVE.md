# PASS-0012 — REG-007 / REG-008 / REG-009 live on the Landing-2 code

Task: Task-0015. Verdict: **REG-007 PASS, REG-008 PASS, REG-009 PASS** — all run in-app
(GitHub web surface → registry-driven consumer → real launched claude agents) against the
**Landing-2 binary** (`go build ./cmd/controlplane` @ `27e6a7e`), on the isolated validation
Temporal `127.0.0.1:17233`, throwaway repos only, production lane untouched. This closes the
gap recorded in [PASS-0011](../PASS-0011/REG-007-LANDING2-LIVE-PROOF.md) (mechanism-only) and
in the run-vs-not-run reconciliation: prior REG-007 PASSes (PASS-0006/08/09/10) were on
pre-Landing-2 code.

Lanes: namespaces `reg007s` (REG-007/cap=1) + `reg007t` (REG-009/008), dedicated runs-roots,
multi-repo registry (`QueueDrainTestbed` qw=2 + `QueueDrainTestbed2` qw=1), launch ON. Queue
flips for REG-007 driven at the **real GitHub web UI** via `github-operator`
(`Set-IssueFieldViaUi.ps1`, each `Committed=True ObservedButtonText=QueueReady ApiMatches=True`).

## REG-007 (+ cap=1 serialization) — PASS (ns reg007s, autoclose ON)
Both Testbed2 issues set `Queue=Never` (UI) → consumer idle. Flipped **#1 + #2 → Ready at the
real UI** at 15:28:02. Consumer log + `/worktrees` (sampled every ~14s):
```
15:28:30  dispatched [Task-0001] parked [] reclaimed []        <- cap=1: ONE dispatched, #2 deferred (no worktree)
15:29:08  wt=1 [Task-0001:running]  #1=OPEN  #2=OPEN
15:29:20  auto-close: closed issue #1 (Task-0001)              <- agent ran (AGENT-RAN + current_gate=closure) -> autoclose
15:29:46  dispatched [Task-0002] parked [] reclaimed [Task-0001] <- reclaim frees slot + REUSE (still wt=1)
15:30:35  auto-close: closed issue #2 (Task-0002)
15:30:58  dispatched [] parked [] reclaimed [Task-0002]        <- final deallocate
15:31:04  wt=0  #1=CLOSED #2=CLOSED
```
Every sample `wt<=1` (never 2): cap=1 honored, slot reused, full deallocate. UI flip → dispatch
→ a top-level claude agent launched and RAN to completion (no starvation).

## REG-009 cross-repo isolation (BUG-0003) — PASS (ns reg007t, autoclose OFF)
Created + committed a Task-0001 announce-only fixture in `QueueDrainTestbed` so BOTH repos have
issue `#1 -> Task-0001` (the exact collision). Both flipped Ready → both dispatched with **distinct
repo-namespaced run ids**:
```
dispatched [Task-0001 Task-0001]
  taskrun--QueueDrainTestbed--Task-0001--active   repo=QueueDrainTestbed   pid=70484  wt=...1041326997\w
  taskrun--QueueDrainTestbed2--Task-0001--active  repo=QueueDrainTestbed2  pid=30084  wt=...1156500720\w
```
**Closed `QueueDrainTestbed#1`** (human close; autoclose OFF). Next poll:
```
reclaimed [Task-0001]                              <- ONLY the QueueDrainTestbed lane
/worktrees -> taskrun--QueueDrainTestbed2--Task-0001--active | running   <- Testbed2 SURVIVES
on disk: Testbed worktree GONE (reclaimed) ; Testbed2 worktree EXISTS (intact)
```
Repo A's closed `#1` reclaimed ONLY repo A's `Task-0001`; repo B's live `Task-0001` lane was
untouched (worktree intact, lane still listed). The BUG-0003 collision does not occur on
Landing 2. (bindings — session id / transcript / pid — are read LIVE from each workflow.)

## REG-008 durable state + backend-restart survival — PASS (ns reg007t, autoclose OFF)
On the surviving Testbed2 `Task-0001` lane:
1. **Park (consumer-driven):** set `Human Needed=Yes` (UI) → consumer logged `parked [Task-0001]`;
   `/worktrees` → `parked_awaiting_closure` (read from the workflow) while the on-disk
   `owned-lane-bootstrap.json` stayed `run_gate_state=running` — **demotion proven in the live
   consumer path** (closes gap G4: real consumer→dispatcher→Service→runtime, not a CLI signal).
2. **Restart-survives:** killed the backend (port down), restarted on `reg007t`. `/worktrees`
   reconstructed `taskrun--QueueDrainTestbed2--Task-0001--active = parked_awaiting_closure` from
   the durable workflow; the next poll RETAINED it (`parked [Task-0001]`, not redispatched/reclaimed).
3. **Reclaim:** closed Testbed2 `#1` → `reclaimed [Task-0001]`, worktree gone, `/worktrees` = 0.

### Observed anomaly (flagged, not buried) — first post-restart poll timeout
The poll activity `StartToClose` is **2 minutes** ([queue/workflow.go:108](../../../../backend/orchestration/internal/queue/workflow.go#L108)).
The FIRST poll after the restart exceeded it: `WARN queue-drain poll failed ... activity StartToClose
timeout` at 15:43:30 (non-fatal — "continuing on next tick"); the NEXT poll (15:43:55, ~25s later)
succeeded and correctly retained the parked lane. Likely contribution from Landing 2's first-build
hook (`reconstructSupervision` + per-record `GetActiveTaskRun` query against a just-restarted,
replaying worker, + `ReconcileOwnedLanes` prune, + a cold GitHub poll). Self-corrected here, but a
~2-min dead first poll on every backend restart is a real latency item. Status: **observed once,
cause unconfirmed — candidate follow-up (BUG/investigation), NOT a correctness failure** (lane was
retained, no data loss, no wrong reclaim).

## Net
All three regression cases PASS on the Landing-2 code, in-app, with real agents. Cleanup: per-run
+ consumer workflows terminated, both testbeds pruned to main-only, backend stopped, issues closed.
One latency anomaly (first post-restart poll StartToClose timeout) is recorded for follow-up.
