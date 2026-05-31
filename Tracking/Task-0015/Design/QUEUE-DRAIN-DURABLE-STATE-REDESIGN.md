# Queue-drain durable-state redesign (fixes BUG-0003 + the Temporal-bypass audit)

Owner: Task-0015. Status: **Landing 1 COMPLETE** (Steps 1–5, build/vet/test green + live
verified — [Testing/PASS-0010](../Testing/PASS-0010/REG-007-BUG-0003-FIX-PROOF.md)); Landing 2
pending. This is the durable design contract for moving the queue-drain consumer off its
filesystem-glob authority.
Drivers: [BUG-0003](../BUG-0003.md) (cross-repo Task-ID collision reclaims a live
agent) and the Temporal-bypass audit (run/working state kept outside Temporal).

## Principle (human directive)

The system's own durable mechanism is the source of truth. Do not bolt a side-store
onto Temporal for convenience. When state is a real external resource (a git worktree,
an OS process), ask the resource (git / the OS) — do not cache it in a parallel store
that can desync. Run-lifecycle facts that have no external resource belong in durable
Temporal state.

## Authority map (target)

| State | Authority (target) | Why |
|---|---|---|
| Which owned-lane worktrees exist / per-repo slot count | **git** (`git -C <repo> worktree list`, repo-scoped) | the worktree IS the resource; git is its durable ledger; no dual-write |
| Run identity (Temporal workflow id, runs-root/lane path) | **repo-namespaced** (`taskrun--<repoID>--Task-NNNN--active`) | two repos each have a #1; identity must be unique per repo |
| Run-lifecycle facts: gate state (running/parked_*), worktree↔session binding (session id / transcript / PID), closure-announce | **per-run Temporal workflow** (`TaskRunWorkflow`) via query/signal | pure data, no external resource → belongs in durable workflow state, not a JSON file |
| `owned-lane-bootstrap.json` | **recovery breadcrumb only** | never the decision authority |
| worktree checkout + launched OS process | **on disk / OS** (irreducible) | Temporal stores data, not checkouts/processes → handled by startup **reconciliation** |

## Two honest landings

**Landing 1 — kill the bug + remove the identity/slot-count bypasses (Steps 1–7).**
Repo-namespace the run identity, make git the (repo-scoped) authority for slot count,
repo-scope the consumer's active-lane accounting AND `findActiveLaneRecord`, route the
watchdog through one run-id path, reconstruct supervision on restart, and add falsifiable
cross-repo collision regression tests. This **fully closes BUG-0003** and the
identity/slot-count bypass families. Verified by `go build/vet/test`, the BUG-0003 repro
(no cross-repo reclaim), PASS-0009 (cap=1 still green), and a new two-repos-both-#1 test.

**Landing 2 — move gate/binding into Temporal (remaining bypass).** The gate-state +
worktree↔session binding still live in the (now repo-scoped, namespaced) JSON file after
Landing 1. Landing 2 relocates them into the per-run `TaskRunWorkflow`: the consumer
**signals** bind/park transitions into the workflow and **queries** them back
(the workflow already exposes `taskrun.current_state` + signal channels), and the JSON
becomes a true recovery breadcrumb. This is the larger piece and is tracked separately so
it is not silently conflated with "relabel the file in a comment."

### Landing 2 — validated step sequence (design-hardened; tree green per step)

Authority discipline (adversarial-checked): a fact backed by an **external resource** keeps
that resource authoritative (the **worktree** → git `worktree list`/`prune`; the launched
**process/transcript** → not git-managed, lives in the workflow); only **resource-free**
facts (the gate label) move into Temporal as the sole live writer. Human-only closure is
untouched — `ClosureRequested` stays a READ of the agent's own `current_gate` and no new
signal closes an issue.

1. **Prunable slot-count fix** (pure `countOwnedLaneWorktreesFromPorcelain`): exclude
   `prunable` (stale) git worktrees from the slot count — directly unblocks the Landing-1
   regression. Self-contained, ships independently.
2. **Workflow signal surface** (additive): `TaskRunWorkflow` gains `taskrun.set_gate_state`
   + `taskrun.bind_session` signals that update `view.RepoLane.Binding`; no caller cutover.
3. **Backend signal+read-back methods** (`SetRunGateState`/`BindLaunchedSession` → `SignalWorkflow`
   + `waitForTaskRunCondition`) + grow the `Runtime` interface + all fakes.
4. **Cutover Set/Bind** to the workflow signals; stop the JSON gate side-store; **keep the
   breadcrumb's launched-PID write** (R2 mitigation: the reclaim fallback needs the real PID
   or it regresses BUG-0002's terminate-before-remove).
5. **Thread ctx** through the `dispatchBinder` seam (leaf `queue` stays ctx-less); point the
   supervisor's live gate read at a direct query (not a stale per-poll Service).
6. **Cutover reads** (`ListActiveWorktrees`/`ReclaimOwnedLane`) to the workflow query with
   breadcrumb fallback on `ErrRunNotFound`; keep BUG-0002 terminate-before-remove.
7. **Durable supervision**: cache the per-repo supervisor across polls (fix the leak) +
   reconstruct supervisors on first poll from the repo-scoped active set.
8. **Reconciliation** `ReconcileOwnedLanes`: `git worktree prune` stale entries + reclaim
   orphan lanes (worktree gone OR workflow terminal/absent).
9. **Breadcrumb demotion** (doc-only) + full-suite + raw-diff churn check.

Residuals (stated): the breadcrumb's gate field ossifies at `running` (only read when the
workflow is already gone — harmless); the breadcrumb keeps a faithful launched-PID for the
reclaim recovery path; reconciliation issues one query per active lane per poll (load, not
correctness).

> Honesty note: Landing 1 alone does NOT satisfy the full approved architecture — it
> leaves gate/binding in the JSON file (repo-correct, but still a side-store). That gap is
> Landing 2 and is named, not papered over.

## Landing 1 step sequence (validated against the real code; tree stays green per step)

1. **Run-id constructor + shim.** Add `ActiveRunIDForRepo(repoNamespace, taskID)` and a
   `Service.repoNamespace` field + `s.runID(taskID)` helper. `ActiveRunID(taskID)` stays
   as the empty-namespace shim (byte-identical). No call sites change. Pure addition.
2. **`Runtime.GetActiveTaskRun(ctx, runID)`** (was `taskID`) so the Service owns namespace
   construction and start/read use ONE path. Update the four fakes. Namespace still empty →
   byte-identical.
3. **Fix A (the agent-killer).** Repo-scope `ActiveOwnedLaneTasks` AND `findActiveLaneRecord`
   by `record.DeclaredWorktreeRoot == s.declaredWorktreeRoot` (field already on every
   record). Keep `ListActiveWorktrees` global for `GET /api/v1/worktrees`. A repo can no
   longer see/act on another repo's lane.
4. **One run-id path for the watchdog.** `Reclaim` stops the supervisor via `d.runID(taskID)`
   (same constructor `Start` used) — prevents a leaked goroutine when namespacing flips on.
5. **Cutover.** Set `repoNamespace = repo.ID` in `NewServiceForRepo` (one production wiring
   point). Two repos' #1 → distinct workflow-ids and distinct artifact paths.
6. **Durable supervision.** Reconstruct watchdog supervisors on the first poll from the
   (git-derived, repo-scoped) active-lane list, so a backend restart doesn't silently lose
   supervision.
7. **Regression tests.** Unit-level (`ActiveRunIDForRepo` distinct per repo; repo-scoped
   accounting) + consumer-level (repo A's closed #1 does NOT reclaim repo B's live Task-0001).

## Top risks (carried from the design pass)

- `Runtime.GetActiveTaskRun` signature change touches the interface + 4 fakes (loud build
  break if one is missed — do it while namespace is empty).
- Supervisor `Stop` must use the exact namespaced id `Start` used (else one leaked goroutine
  per reclaim) — Step 4 routes both through one helper.
- In-flight cutover orphans: runs started under the old global id become unfindable after
  Step 5; the single-repo control plane (`NewService`, empty namespace) is protected by the
  shim; multi-repo in-flight runs need the reconciler.
- `repos[].id` now flows into a Temporal workflow id + a filesystem dir via
  `sanitizePathSegment` — document the safe-slug convention (ids are clean tokens today).
