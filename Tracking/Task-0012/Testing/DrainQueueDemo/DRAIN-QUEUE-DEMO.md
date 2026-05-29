# Drain-Queue Consumer Demonstration ("drain my queue pls")

This is the durable proof package for the end-to-end drain-queue consumer
demonstration requested in
[../../HUMAN-DIRECTIVES-FOR-WORKER.md](../../HUMAN-DIRECTIVES-FOR-WORKER.md)
"Demonstration Directive (2026-05-28) - Prove It End-To-End".

It demonstrates the `drain my queue pls` capability: a consumer that pulls
queued tasks, allocates a git worktree per task, dispatches a subagent to make a
minor program modification, captures the result, respects a concurrency limit,
and releases the worktree.

## What This Is And Is Not

- This is a **minimal but genuinely working slice / spike**, not a
  production-ready coordinator. It runs for real, makes real commits, and
  releases real worktrees.
- The production accepted-task queue is the **GitHub Issues provider**
  (`Digital-Collective-Games/Obsidian`), already proven earlier in this task
  (issues `#1`..`#12`, clean reconcile). For the demonstration the queue backing
  store is a **local JSON file** ([queue.json](./queue.json)) to keep the
  external footprint lowest, exactly as the directive allows. No new
  outward-facing GitHub repository was created.

## Components

| File | Role |
| --- | --- |
| [queue.json](./queue.json) | The task queue: 4 minor-modification tasks against the demo program. |
| [Drain-Queue.ps1](./Drain-Queue.ps1) | The consumer. Pulls ready tasks, allocates/binds/releases worktrees, dispatches subagents, enforces concurrency. |
| [Invoke-Subagent.ps1](./Invoke-Subagent.ps1) | The dispatched subagent runner. Isolated per task: applies the modification, runs tests, commits, emits a result JSON. |
| [apply_modification.py](./apply_modification.py) | The real edit logic the subagent performs, driven by each task's `modification` spec. |
| [worktree-allocations-demo.json](./worktree-allocations-demo.json) | Worktree allocation registry following the `WORKTREES.md` schema. |
| [DRAIN-RUN-SUMMARY.json](./DRAIN-RUN-SUMMARY.json) | Machine-readable run summary. |
| [DRAIN-RUN.log](./DRAIN-RUN.log) | Human-readable run log (allocate/dispatch/result/release per task). |
| [results/](./results/) | Per-task subagent result JSON and per-task `.diff` files. |
| [logs/](./logs/) | Per-task spec snapshots and subagent stdout logs. |

## Target Repo

`C:\Agent\YourTestRepo` is a small standalone git repo containing `calc.py`
(a tiny CLI calculator) and `test_calc.py`. Baseline commit at run time:
`9af2adf` (`Add .gitignore for Python bytecode`), parent `9da9192`
(`Initial calc.py demo program`).

## How To Reproduce

```powershell
cd C:\Agent\CodexDashboard\Tracking\Task-0012\Testing\DrainQueueDemo
pwsh -NoProfile -File ./Drain-Queue.ps1 `
  -QueuePath ./queue.json `
  -TargetRepo "C:\Agent\YourTestRepo" `
  -MaxConcurrency 2 `
  -OutDir .
```

## Run Result (last run)

All 4 queued tasks drained, all 4 succeeded, all subagent tests passed:

| Task | Modification | Branch | Commit | Tests |
| --- | --- | --- | --- | --- |
| DQ-001 | Add `div` operation | `drain/DQ-001` | `e6b8656` | pass |
| DQ-002 | Bump VERSION to 0.2.0 | `drain/DQ-002` | `4aa87e3` | pass |
| DQ-003 | Add `--version` flag | `drain/DQ-003` | `8ff911b` | pass |
| DQ-004 | Add `pow` operation | `drain/DQ-004` | `cb59fd9` | pass |

The commits live on branches in `C:\Agent\YourTestRepo` and are real,
inspectable, minimal diffs (see [results/](./results/) `.diff` files). Functional
spot-checks: `python calc.py div 6 2` -> `3.0` on `drain/DQ-001`; VERSION is
`0.2.0` on `drain/DQ-002`.

## Worktree Allocation / Release Proof

Each task got a dedicated git worktree (`git worktree add` on a per-task branch)
recorded in [worktree-allocations-demo.json](./worktree-allocations-demo.json)
with the `WORKTREES.md` fields (`slot_id`, `repo_id`, `base_repo_path`, `path`,
`state`, `owning_task_id`, `branch`, `head`, `dirty_state`, `release_condition`,
allocate/release timestamps). After the subagent result was captured, each
worktree was removed (`git worktree remove --force`) and its registry entry set
to `state: released`. `git worktree list` after the run shows only `main` -
every demo worktree was released.

These are **disposable** demo worktrees (`reusable: false`), so they are named by
slot index (`C:\Agent\YourTestRepo_drainslot1`..`4`) and removed on release, in
keeping with `WORKTREES.md`'s rule that task-id/disposable names are acceptable
for worktrees removed after the task. The demo registry is kept **separate** from
the shared cross-task `C:\Users\gregs\.codex\Orchestration\WORKTREE-ALLOCATIONS.json`
on purpose: disposable demo allocations should not pollute the shared
coordination surface, and the shared registry's existing `Task-0013` slot was
left untouched.

## Concurrency Proof

The run used `-MaxConcurrency 2` against 4 tasks. The run log shows DQ-001 and
DQ-002 allocated first; DQ-003 was only allocated **after** DQ-001 released, and
DQ-004 only after DQ-002 released. At no point were more than 2 worktrees bound
or more than 2 subagents in flight. The scheduler caps in-flight jobs at
`MaxConcurrency` and backfills as each completes.

## Honesty / Caveats

- **Subagent model.** In this run the dispatched "subagent" is a deterministic
  local executor process ([Invoke-Subagent.ps1](./Invoke-Subagent.ps1) +
  [apply_modification.py](./apply_modification.py)) running in its own
  background job with only its task spec and worktree path. It stands in for a
  dispatched Codex subagent because nested Codex subagent-spawn tooling was not
  exposed to this run. The **execution shape** it proves - isolated per-task
  work inside an allocated worktree, structured result capture, concurrency
  capping, worktree release - is the same shape a real Codex subagent dispatch
  would use. Swapping the executor for a real Codex subagent invocation is the
  remaining production step.
- **Local vs GitHub queue.** The demo queue is local JSON; production accepted
  tasks already live as GitHub Issues. Wiring the consumer to pull from the
  GitHub provider (via the existing obsidian-operator scripts) instead of local
  JSON is straightforward follow-on, not demonstrated here.
- **Concurrency mechanism.** Concurrency is enforced with PowerShell background
  jobs and a simple in-flight cap; it is not a durable/Temporal-backed scheduler
  and has no crash-recovery or persistence of in-flight state.
- **No merge/PR step.** The consumer leaves each task's work on its own branch
  with a commit. It does not merge, open PRs, or push - intentionally out of
  scope for the demonstration.
- **Minor cosmetic churn.** The `add_operation` insertions leave one extra blank
  line before the inserted function. The diffs are still minimal and faithful;
  this is a known cosmetic nit, not a behavioral issue.
