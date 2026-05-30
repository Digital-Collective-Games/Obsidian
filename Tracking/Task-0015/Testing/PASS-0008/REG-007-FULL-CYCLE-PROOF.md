# PASS-0008 — Full-cycle deallocate/reuse + env-gated auto-close + BUG-0002 fix (live)

Task: Task-0015. Verdict: **PASS**. Proves the complete queue lifecycle the human
required: allocate both slots -> agents complete + announce done -> (simulated-human)
auto-close -> BOTH worktrees DEALLOCATE + slots free -> queue another task -> it REUSES a
freed slot. Resolves [BUG-0002](../../BUG-0002.md).

## What was built (this pass)
1. **Env-gated TEST-ONLY auto-close** (`OBSIDIAN_AUTO_CLOSE_QUEUED`, default OFF). When on,
   the consumer closes the GitHub issue of any active dispatched task that ANNOUNCED
   completion (worktree `TASK-STATE.json current_gate == "closure"`), simulating a human
   approving closure. Default OFF keeps the consumer read-only (production = human-only
   closure). The launched-agent prompt instructs the agent to announce via `current_gate`
   (it still NEVER closes the issue itself).
2. **[BUG-0002](../../BUG-0002.md) fix** (closed task's worktree never deallocated; consumer
   wedged): (A) persist the launched agent PID and TERMINATE its process tree
   (Windows image-guarded) before worktree removal; (B) make `cleanupOwnedLane`
   idempotent/self-healing (`git worktree remove` -> on failure `prune` + `os.RemoveAll`
   retry -> nil once the dir is gone). A close now ALWAYS deallocates.

3 independent workflow verifiers (gating, signal-path, build-scope for the feature;
idempotency, kill-safety, build-scope for the fix) all returned `sound`; `go build/vet/test`
green; `gofmt` clean.

## Live run (registry-driven, isolated `reg007h`, `queue_workers=2`, auto-close ON)
Three throwaway issues `#5/#6/#7`, each flipped `Queue=Ready` via the **real GitHub UI**
(`github-operator` skill). Production Obsidian never polled; real cron namespace untouched.

### Phase A — allocate 2 -> announce -> auto-close -> DEALLOCATE (PASS)
- `dispatched [Task-0005]`, `dispatched [Task-0006]` (both slots, cap=2).
- Both agents announced done: `Task-0005:closure`, `Task-0006:closure` (worktree `current_gate`).
- `auto-close: closed issue #5 ... #6 ... (simulated human closure approval)`.
- `reclaimed [Task-0005]`, then `reclaimed [Task-0006]` -> worktrees **2 -> 1 -> 0**.
- **No `is not a working tree` wedge** (the BUG-0002 failure mode is gone).
- Evidence: [phaseA-allocate2-announce-autoclose-deallocate.txt](./evidence/phaseA-allocate2-announce-autoclose-deallocate.txt).

### Phase B — queue a 3rd after both freed -> REUSE a freed slot (PASS)
- Flipped `#7` Ready (both slots free) -> `dispatched [Task-0007]` -> worktree created
  (slot REUSED).
- Announced -> auto-closed `#7` -> `reclaimed [Task-0007]` -> worktrees **-> 0**.
- Evidence: [phaseB-queue3rd-reuse-freed-slot.txt](./evidence/phaseB-queue3rd-reuse-freed-slot.txt),
  [backend-drain-log.txt](./evidence/backend-drain-log.txt).

## Net
The allocate -> complete -> announce -> close -> deallocate -> reuse lifecycle is proven
live end-to-end. The deallocate fix is NOT auto-close-specific: it equally fixes the real
human-close reclaim path (the earlier PASS-0006 "release on close" only appeared to work
because the worktree was pruned by hand afterward). Agents exited cleanly on their own
(explicit fixtures: announce via `current_gate`, do not attempt `gh`). Cleanup: backend
stopped, `#5/#6/#7` closed, worktrees pruned.
