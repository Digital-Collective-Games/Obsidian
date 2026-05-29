# Task-0012 Plan

## Status

Plan approved by human direction on 2026-05-28. The original approval gate is
packaged in
[Design/PLAN-APPROVAL/REVIEW-PACKAGE.md](./Design/PLAN-APPROVAL/REVIEW-PACKAGE.md).

Progress as of 2026-05-28:

- `PASS-0001` provider registry: done (`CODEX-REPO-MANIFEST.json`).
- `PASS-0002` one-issue `gh` pilot and `TASK-META.json`: done; readback proven.
- `PASS-0003` human inspection and convention decision: pilot accepted by the
  human; the human then approved repo-local continuation for this repo only.
- Post-pilot human-approved repo-local bootstrap: existing tasks
  `Task-0001`..`Task-0012` bootstrapped into issues `#1`..`#12`, with reconcile,
  text-conflict review, and issue-state correction. This is repo-local
  bootstrap, not cross-repo bulk publication. Proof:
  [Testing/BULK-ISSUE-BOOTSTRAP-PROOF.md](./Testing/BULK-ISSUE-BOOTSTRAP-PROOF.md),
  [Testing/RECONCILE-PROOF.md](./Testing/RECONCILE-PROOF.md).
- A false `text_conflict` from `gh` UTF-8 mis-decode under Windows PowerShell
  5.1 was found at closeout and fixed; see [BUG-0001.md](./BUG-0001.md).
- `PASS-0004` TaskCreate GitHub provider follow-on: recorded as required
  follow-on work; not implemented in this task.
- `PASS-0005` drain-queue consumer demonstration: done as a working spike. The
  human explicitly rescoped Task-0012 on 2026-05-28 (see
  [HUMAN-DIRECTIVES-FOR-WORKER.md](./HUMAN-DIRECTIVES-FOR-WORKER.md) "Explicit
  Rescope" and "Demonstration Directive"): the publication slice is the
  foundation, not the finish line; the required deliverable is a working
  `drain my queue pls` consumer proven end-to-end. Built and ran a consumer that
  pulls a local queue, allocates a git worktree per task, dispatches a subagent
  to make a minor program modification, respects a concurrency limit, captures
  the result, and releases the worktree, against `C:\Agent\YourTestRepo`. Proof:
  [Testing/DrainQueueDemo/DRAIN-QUEUE-DEMO.md](./Testing/DrainQueueDemo/DRAIN-QUEUE-DEMO.md).

## Human Outcome

The human should be able to inspect one GitHub Issue produced from one local
`TASK.md` by regular Codex running `gh`, see exactly how local and GitHub state
co-own the task, and decide whether that issue shape is good enough before any
bulk publication or queue-draining work exists.

## Sync Ownership Model

Local `TASK.md` owns rich task truth. GitHub owns queryable backlog/task-state
surface. Codex owns synchronization between the two for this pilot.

Codex sync steps:

1. Read local `TASK.md`.
2. Render desired issue title/body.
3. Validate the local task folder id, embedded task id, issue number, and body
   marker cannot diverge.
4. Read existing `TASK-META.json` beside the selected local task if present.
5. Run `gh auth status -h github.com`.
6. Create or update exactly one issue through `gh`.
7. Run `gh issue view --json number,title,body,state,labels,url`.
8. Write task metadata only after successful readback.
9. Package the result for human inspection.

## Implementation Passes

The pass structure follows the human-directed rollout sequence captured in
[Research/SESSION-HUMAN-DIRECTIVES-PASS-STRUCTURE.md](./Research/SESSION-HUMAN-DIRECTIVES-PASS-STRUCTURE.md).
The one-issue pilot is proof of the `gh` path. The final steady-state decision
is the repo registry plus a TaskCreate GitHub-provider interface, not a
per-task mapping layer.

### PASS-0001: Provider Registry Shape

Objective:

- create the root repo registry shape that tells CodexDashboard which repos it
  cares about and how to interact with each repo

Expected artifacts:

- repo-root `CODEX-REPO-MANIFEST.json`
- any backup/restore inventory note needed for that registry

Registry requirements:

- one CodexDashboard repo entry
- `source_control_provider` with git remote, GitHub repo, URL, and default
  agent user `gregsemple2003`
- `task_provider` for accepted tasks in `Digital-Collective-Games/Obsidian`
- `task_proposal_provider` for proposals in
  `Digital-Collective-Games/ObsidianProposals`
- no configurable local task store abstraction

Verification:

- `CODEX-REPO-MANIFEST.json` parses as JSON
- raw `git diff` shows no backend, dashboard UI, queue draining, or local
  task-store abstraction changes

Exit bar:

- provider bindings are discoverable from the repo root
- local task folder conventions remain fixed shared workflow, not registry
  indirection

### PASS-0002: One-Issue `gh` Export And `TASK-META.json`

Objective:

- run the Codex-operated `gh` pilot for exactly one selected local task,
  [../Task-0001/TASK.md](../Task-0001/TASK.md), and preserve minimal provider
  metadata only after readback

Expected actions:

- run [../../skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1](../../skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1)
  for `Tracking/Task-0001/TASK.md`
- have the script validate that `Task-0001`, `Tracking/Task-0001/TASK.md`, and
  GitHub issue `#1` agree before update/readback
- if auth is unavailable, save blocked proof under `Tracking/Task-0012/Testing/`
  and stop without writing `TASK-META.json`
- if auth is available, update issue `#1` in `Digital-Collective-Games/Obsidian`
- run `gh issue view --repo Digital-Collective-Games/Obsidian 1 --json number,title,body,state,labels,url`
- write `Tracking/Task-0001/TASK-META.json` only after successful readback

Verification:

- saved auth proof or publish/update command result
- saved raw `gh issue view` JSON with no queue, priority, human-needed, or
  identity labels
- saved minimal `Tracking/Task-0001/TASK-META.json`
- script proof that embedded task ids cannot mismatch the issue number and
  rejected workflow labels are removed
- raw `git diff` check showing only expected task-owned and registry artifacts
  changed

Exit bar:

- exactly one issue is updated, or the task is honestly blocked
- no duplicate issue is created on rerun
- the issue can be read back through `gh`
- no extra `codex-*` labels or tags are required to identify the issue as a
  task; provider repo binding supplies that meaning
- queue, priority, and human-needed state are not labels and are surfaced
  through provider-owned GitHub Issue Fields
- issue `#1` says `Task-0001`, not `Task-0012`

### PASS-0003: Human Inspection And Convention Decision

Objective:

- package the pilot issue for human review and record the human-selected
  steady-state convention before any bulk publication

Expected artifact:

- `Tracking/Task-0012/Testing/PILOT-ISSUE-REVIEW-PACKAGE.md`

Review package must link or summarize:

- issue URL
- issue title/body
- raw `gh issue view` JSON
- `Tracking/Task-0001/TASK-META.json`
- local task path
- any remote/local drift found before update
- the convention decision that task/proposal identity is implied by provider
  repo binding, not identity labels
- the convention decision that future accepted task IDs come from GitHub issue
  creation before `Tracking/Task-<issue-number>/TASK.md` is materialized

Exit bar:

- the pilot is accepted or rejected as proof of the `gh` create/update/readback
  path
- if rejected, Task-0012 remains open with required changes recorded
- no bulk `TASK.md` publication or queue-draining work starts in this task

### PASS-0004: TaskCreate GitHub Provider Follow-On

Objective:

- preserve the required follow-on work for TaskCreate without implementing the
  full provider interface in this task

Expected artifacts:

- [TASK.md](./TASK.md) records the TaskCreate GitHub-provider interface
  requirement
- pass closeout/checklist records that this is follow-on process work

The follow-on provider-interface subdocument must eventually define:

- creating accepted task issues through `gh`
- creating proposal issues through the proposal provider
- promoting proposals into accepted task issues
- materializing `Tracking/Task-<issue-number>/TASK.md` only after GitHub
  returns the accepted issue number
- rejected proposals staying in the proposal provider and not creating local
  task folders

Verification:

- `TASK.md`, `PLAN.md`, and pass checklists agree on this pass structure
- raw `git diff` shows no `.codex` process-doc implementation, backend
  endpoint, dashboard UI, bulk publication, queue draining, or broad
  source-control abstraction changes

Exit bar:

- the task cannot close with the pilot `Task-0012` to issue `#1` metadata
  treated as the final identity convention
- closeout names the registry as the durable provider binding and the
  TaskCreate provider subdocument as required follow-on process work

### PASS-0005: Drain-Queue Consumer Demonstration

Status: done as a working spike (2026-05-28), under the human's explicit rescope.

Objective:

- prove `drain my queue pls` end-to-end with real worktrees and real program
  modifications, not a design-only or dry-run deliverable

Expected artifacts:

- demo target repo `C:\Agent\YourTestRepo` (small `calc.py` program + tests)
- [Testing/DrainQueueDemo/queue.json](./Testing/DrainQueueDemo/queue.json)
- [Testing/DrainQueueDemo/Drain-Queue.ps1](./Testing/DrainQueueDemo/Drain-Queue.ps1)
  (consumer)
- [Testing/DrainQueueDemo/Invoke-Subagent.ps1](./Testing/DrainQueueDemo/Invoke-Subagent.ps1)
  (dispatched subagent runner)
- [Testing/DrainQueueDemo/apply_modification.py](./Testing/DrainQueueDemo/apply_modification.py)
  (real edit logic)
- [Testing/DrainQueueDemo/worktree-allocations-demo.json](./Testing/DrainQueueDemo/worktree-allocations-demo.json)
  (allocation registry following `WORKTREES.md` schema)
- [Testing/DrainQueueDemo/DRAIN-RUN-SUMMARY.json](./Testing/DrainQueueDemo/DRAIN-RUN-SUMMARY.json),
  [Testing/DrainQueueDemo/DRAIN-RUN.log](./Testing/DrainQueueDemo/DRAIN-RUN.log),
  per-task results/diffs under
  [Testing/DrainQueueDemo/results/](./Testing/DrainQueueDemo/results/)
- [Testing/DrainQueueDemo/DRAIN-QUEUE-DEMO.md](./Testing/DrainQueueDemo/DRAIN-QUEUE-DEMO.md)
  (proof package, including the subagent-model and local-queue caveats)

Verification:

- 4 queued tasks drained; 4 succeeded; each produced a real commit on a per-task
  branch in `C:\Agent\YourTestRepo` with minimal, faithful diffs and passing
  tests
- worktree allocation/release lifecycle recorded; `git worktree list` after the
  run shows only `main` (all demo worktrees released)
- concurrency limit of 2 honored against 4 tasks (verified in the run log)

Exit bar:

- the consumer genuinely runs and modifies the program; this is a working spike,
  explicitly labeled as not production-ready (see caveats in the proof package)

## Data Handling

Backup impact expected:

- task-owned persistent task metadata introduced

Reasoning:

- the task adds `Tracking/Task-0001/TASK-META.json` through
  `Tracking/Task-0012/TASK-META.json`, tracked local task metadata beside each
  local task doc
- proof artifacts live under `Tracking/Task-0012/Testing/`
- the task does not change human-lane service roots, dashboard config, SQLite
  behavior, Temporal/Postgres persistence, job specs, or restore procedures

Closeout finding (2026-05-28): no [DATA-HANDLING.md](../../DATA-HANDLING.md)
broadening is required for the `TASK-META.json` files. They are tracked repo
artifacts under `Tracking/` and are fully covered by the existing "repo state
delta" must-backup-delta class (`C:\Agent\CodexDashboard` plus upstream git).
`CODEX-REPO-MANIFEST.json` is already named explicitly in `DATA-HANDLING.md` as
a must-backup-delta tracked manifest. No new runtime data store is introduced.

## Out Of Scope

- backend orchestration endpoint for publication
- desktop UI for publication
- bulk publishing all `TASK.md` files
- full cross-repo issue query dashboard
- GitHub labels or comments as canonical live execution state

Note (2026-05-28 rescope): `drain my queue pls` and worktree allocation/release
were originally out of scope, but the human explicitly rescoped Task-0012 to
require a working drain-queue consumer demonstration. That demonstration is
PASS-0005 above. A production-ready coordinator (GitHub-backed queue source,
real Codex subagent dispatch, durable concurrency/recovery, merge/PR step)
remains follow-on work beyond this spike.
