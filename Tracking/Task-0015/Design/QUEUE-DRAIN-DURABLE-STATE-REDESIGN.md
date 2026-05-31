# Queue-drain durable-state redesign (fixes BUG-0003 + the Temporal-bypass audit)

Owner: Task-0015. Status: **Landing 1 COMPLETE** (build/vet/test green + live verified —
[Testing/PASS-0010](../Testing/PASS-0010/REG-007-BUG-0003-FIX-PROOF.md)). **Landing 2 code
COMPLETE** (Steps 1–8, build/vet/test green; live verification pending) — gate/binding now
live in the per-run workflow, JSON demoted to a recovery breadcrumb, durable per-repo
supervision, and safe startup reconciliation. This is the durable design contract for
moving the queue-drain consumer off its filesystem-glob authority.
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

#### As-built (Steps 4–6, committed) — two honest deviations from the wording above

Steps 4, 5, and 6 shipped as **one coherent change** to `internal/taskrun/service.go`
(write + read authority both move to the workflow in the same commit, because once
`SetRunGateState` stops writing the JSON gate, `ListActiveWorktrees` MUST read the live
gate from the workflow or its assertions/operators would see an ossified `running`). The
existing unit tests passed **unmodified** — the `fakeRuntime` already mirrors the same
binding the JSON used to hold, keyed by the same run id.

1. **No ctx threading; `context.Background()` instead.** The design's Step 5 said "thread
   ctx through the dispatchBinder seam." As-built, the Service `SetRunGateState` /
   `BindLaunchedSession` / `ListActiveWorktrees` signatures stay **ctx-less** and use
   `context.Background()` for the Landing-2 Temporal signal/query calls. Rationale: this is
   consistent with **BUG-0001**, where the launch deliberately detaches from the dispatch
   activity ctx so the durable run outlives the activity; a park/bind/list signal should
   likewise not be cancelled by a returning poll activity. It also avoids churning the
   `dispatchBinder`, `fakeBinder`, `queue.Dispatcher`, and four `*_test.go` signatures for
   no correctness gain (the read-backs are bounded by `waitForTaskRunCondition`'s attempt
   cap, not a ctx deadline). The leaf `queue` package stays ctx-less, as intended.
2. **`ReclaimOwnedLane` keeps reading the breadcrumb PID (not cut to a workflow query).**
   The launched PID never changes during a run and is faithfully mirrored to the breadcrumb
   by `BindLaunchedSession` (the R2 write). At reclaim time the workflow is frequently
   already terminal (the issue was closed → the run completed), so the breadcrumb is the
   robust PID source and a workflow-first query would usually fall back to it anyway. This
   is the documented recovery copy, not a Temporal bypass: the worktree's existence stays
   git-authoritative and BUG-0002's terminate-before-remove is preserved.

The watchdog's live gate read (`gateStateForTaskFn`) is unchanged in wiring — it reads
`ListActiveWorktrees`, which is now workflow-backed, so the watchdog suspends on a live
park. Cost: a per-poll global query fan-out (the stated reconciliation/load residual).

**Step 7 (committed).** `newQueueDrainActivities` now caches one watchdog supervisor per
repo id ACROSS polls (the dispatch binding is still rebuilt each poll so a registry edit is
picked up). Previously a fresh supervisor was built every poll, so a run's `Start` (the
dispatching poll) and `Stop` (a later poll's `Reclaim`) hit different supervisors — leaking
the watchdog goroutine and making `Stop` a no-op. On first build per repo (backend restart)
`reconstructSupervision` re-establishes supervision for already-active lanes
(`ListActiveWorktreesForRepo`), skipping lanes with no bound transcript.

**Step 8 (committed) — `ReconcileOwnedLanes` is prune-only; autonomous worktree-reclaim is
DELIBERATELY EXCLUDED.** The design's Step 8 wording was "git worktree prune + reclaim
orphan lanes (worktree gone OR workflow terminal/absent)." During implementation the
reclaim half was found to CONFLICT with the paramount human-only-closure / park-in-place
contract (HUMAN-DIRECTIVES O4: the agent NEVER self-closes), so it was not built:

- A queue-drain `TaskRunWorkflow` sets `StateRunning`/`StateBlocked` after its owned-lane
  execution (both → `Status:"active"`, non-terminal) and loops in the select forever;
  parking only updates `Binding.RunGateState`, never `Status`. So a **live, parked, or
  blocked** lane has a non-terminal, queryable workflow — `GetActiveTaskRun` finds it.
- The workflow becomes **absent** only via Temporal history expiry or external termination.
  A lane **parked awaiting human closure** can legitimately outlive its workflow history →
  `GetActiveTaskRun` → `ErrRunNotFound`. Reclaiming on that signal would **self-close a
  lane the human never approved** — a direct contract violation. Neither `ErrRunNotFound`
  nor a terminal status proves a *human* closed the work; only the GitHub-issue-closed
  signal does, and that is already the consumer's authoritative reclaim path.

`ReconcileOwnedLanes` therefore does **`git worktree prune` only** (safe hygiene: it touches
only worktrees whose directory is already gone, with git's built-in grace period). It is
wired once per repo at startup, before `reconstructSupervision`. The "worktree gone" half of
the original wording IS covered (prune clears its stale git metadata); the "worktree still
exists + workflow gone" half is intentionally NOT auto-reclaimed.

> Open question for the human (raised, not silently decided): do you want ANY autonomous
> worktree-reclaim in reconcile, and if so under what definitive signal? As-built, the
> answer is "no — reclaim stays human-gated via the closed GitHub issue."

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
