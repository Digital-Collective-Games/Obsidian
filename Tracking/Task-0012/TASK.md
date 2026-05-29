# Task 0012

## Title

Codex-operated pilot for GitHub-backed task state.

## Summary

CodexDashboard needs GitHub-backed task state through `gh`, but the first slice
must stay deliberately small and must not begin as backend orchestration
plumbing.

The first useful outcome is:

- choose one local task
- have regular Codex push that local `TASK.md` into one GitHub-backed
  representation through `gh`
- read the same GitHub Issue back through `gh`
- preserve task metadata and sync state beside the local task
- inspect whether this issue shape is good enough before bulk publication,
  cross-repo query dashboards, or queue draining

GitHub Issues plus `gh` should become the cross-repo backlog, query, triage,
and shallow task-state layer. Local `TASK.md` files remain the rich task source
for scope, acceptance, research, proof, audits, pass history, and local review
artifacts. CodexDashboard backend and Temporal remain the live execution source
for runs, waits, interrupts, cleanup, worktree ownership, and active
concurrency.

## Writeup Type

Concrete Codex-operated pilot task.

The chosen first solution is not a backend endpoint. Regular Codex runs `gh`
for one task, keeps the local task file and GitHub Issue in sync, writes the
task metadata/proof artifacts, and asks the human whether the representation
should be accepted before scaling.

## Burden Being Reduced

The human is starting to lose track of task status across local
`Tracking/Task-*` trees and repos.

The exported work today is repeated memory and reconstruction work:

- remembering which local tasks exist
- remembering which tasks are ready, running, or stale
- finding task docs across repos
- deciding whether a task is visible enough to coordinate through GitHub

This task reduces only the first piece of that burden: proving that one local
task can become readable, queryable GitHub-backed task state without losing the
richer local task contract.

## Current Truth

CodexDashboard has local task readback and backend task-run state, but no
accepted GitHub Issue shape for cross-repo task state.

The local machine has `gh` installed. A real write still depends on GitHub auth
for the selected target repo.

Current durable split:

- local task docs own rich scope, acceptance, rationale, research, proof,
  audits, pass history, and local review artifacts
- GitHub Issues should own cross-repo backlog identity, discoverability,
  shallow status, triage, open/closed visibility, and queryable task discovery
  once the representation is accepted
- `gh` is the first integration path for reading and writing that GitHub task
  state
- CodexDashboard backend and Temporal own live execution state, run ownership,
  waits, interrupts, cleanup, lane ownership, worktree ownership, active
  concurrency, and restore commits

What is missing:

- a Codex-operated `gh` sync procedure for one local task
- a concrete co-ownership contract between local `TASK.md`, GitHub Issues, and
  repo registry bindings
- an inspection-friendly issue title/body shape
- a durable repo registry that tells CodexDashboard which repos matter and which
  providers own source control, accepted tasks, and task proposals
- a TaskCreate follow-on provider contract for turning promoted proposals or
  approved drafts into GitHub-backed task folders
- temporary local task metadata for this pilot task doc and its GitHub Issue
- idempotent create/update/readback behavior for the pilot issue
- fail-closed behavior when `gh` is missing, unauthenticated, or returns
  unusable output

## Target Truth

After this task succeeds, Codex can push exactly one selected local `TASK.md`
into a GitHub Issue, read it back through `gh`, write local task metadata/proof
artifacts, and present the issue shape for human inspection. The task also
records the agreed steady-state direction: CodexDashboard reads a repo registry
to discover each repo's source-control provider, accepted-task provider, and
task-proposal provider.

The pilot issue should be readable and queryable as backlog/task state without
hidden chat context, while making clear that local task docs remain the richer
source of task truth.

The pilot publishes `Tracking/Task-0001/TASK.md` to GitHub issue `#1` because
the accepted-task convention is one-to-one: issue number `N` maps to
`Tracking/Task-NNNN/`. Task-0012 owns the pilot proof artifacts, not issue
`#1` identity.

## Causal Claim

If regular Codex can sync one local task to GitHub through `gh`, preserve
metadata/readback evidence, and show exactly what co-owned fields changed, then
the human can judge the representation before risking duplicate, low-quality, or
over-broad issue creation across all repos. If the pilot exposes a better
steady-state identity model, this task must record that model before closure
instead of treating the first issue metadata as permanent policy.

## Human Directives From Session

These directives were mined from the current Codex session on 2026-05-28 and
are authoritative scope for this task.

TaskCreate and research setup:

- Ground in the TaskCreate process before drafting.
- Capture the human need as task-owned research.
- Use the Chrome debug Slack tab to capture the current-day conversation with
  Alex.
- Put the capture under the task `Research/` subfolder.
- This is a CodexDashboard task.

Product direction from the Alex discussion:

- Alex's `gh` command-line approach to task state is valuable.
- The current local `TASK.md` docs are still useful, but CodexDashboard should
  use `gh` at least to query tasks from different repos.
- Research existing durable doc conventions before deciding the final split.
- Decide explicitly which state should have GitHub Issues and `gh` as source of
  truth, and which state should remain in local docs that can be pushed to
  GitHub for context.

Required rollout sequence:

1. Push a single `TASK.md` into a GitHub-backed representation.
2. Inspect that single published representation.
3. Agree that it meets the quality bar.
4. Then push all `TASK.md` files across all configured repos.
5. Then wire the `drain my tasks pls` workflow where a central coordinator
   allocates a worktree and dispatches work.

Future coordinator shape:

- The later central coordinator should be able to dispatch tasks on worktrees.
- Worktree management should include allocation and release.
- Active task execution should have a configured concurrency limit.
- Those coordinator, worktree, and concurrency behaviors are follow-on work, not
  Task-0012 closure.

Human override of audit drift:

- The active task is the restored first draft by human direction.
- Agent auditor preferences are secondary to human directives.
- Auditor feedback may identify real contradictions, missing proof, or durable
  rule violations, but it must not broaden this task beyond the human-selected
  first slice.
- The later audit/revision artifacts are preserved as history; they do not
  override this section's scope.

## Human Directives Mapped To Codex Operations

- `Push a single TASK.md`: Codex selects `Tracking/Task-0001/TASK.md` because
  GitHub issue `#1` must map to local `Task-0001`, renders a GitHub Issue, and
  runs the repo script that wraps `gh issue edit`.
- `Inspect the resulting representation`: Codex runs `gh issue view --json
  number,title,body,state,labels,url`, saves the raw JSON, and creates
  `Testing/PILOT-ISSUE-REVIEW-PACKAGE.md`.
- `Agree that it meets the quality bar`: Codex stops at a human gate with the
  issue URL, body preview, task metadata file, readback JSON, and exact approval
  question.
- `Then push all TASK.md files`: explicitly follow-on work; this pilot records
  whether the issue shape is accepted before bulk publication exists.
- `Then drain my tasks pls`: explicitly follow-on work; this pilot must not
  implement dispatch, worktree allocation, or concurrency.
- `Use gh to query tasks`: this pilot proves the published issue can be read
  back through `gh` and that its provider repo/state/title are suitable for
  later cross-repo queries; it does not build the full query dashboard.
- `Provider registry`: human discussion after the pilot selected a repo
  registry model. Accepted tasks and task proposals are provider bindings, while
  local task folder conventions remain fixed shared workflow rather than a
  configurable local store.

## Co-Ownership Semantics

Local `TASK.md` and the GitHub Issue are co-owned, but not equal mirrors.

Local `TASK.md` owns:

- full scope and acceptance criteria
- rationale, research links, proof plans, audits, pass history, and local review
  artifacts
- detailed implementation notes that would bloat a GitHub Issue

GitHub Issue owns:

- cross-repo backlog identity
- issue number, URL, state, title, assignee/milestone if later used
- shallow task summary that can be read without hidden chat context
- queryable discovery fields for `gh issue list` and `gh search issues`

Codex owns the sync operation:

- render desired GitHub title/body from local `TASK.md`
- validate that local task folder id, embedded task id, GitHub issue number,
  and generated body marker agree before writing
- update the matching issue number for existing accepted tasks
- read the issue back after every update
- write or update `TASK-META.json` beside the selected local task only after
  successful update and readback
- stop with a blocked proof artifact when `gh` cannot truthfully complete the
  operation

Manual GitHub edits are allowed only as human-authored backlog edits. Codex must
read the current issue before update and compare the live title/body readback to
the rendered local task. If they differ, Codex must stop unless the operator
explicitly chooses a merge, overwrite, or repair path.

## Provider Registry Decision

The steady-state binding is repo-registry based, not a per-task lookup table.

`CODEX-REPO-MANIFEST.json` answers:

- which repos CodexDashboard should care about
- how source control writes should identify the default agent user and remote
- which GitHub Issues repo owns accepted tasks for each repo
- which GitHub Issues repo owns task proposals for each repo

For CodexDashboard, the current registry entry should name:

- source control provider: git `upstream`,
  `Digital-Collective-Games/Obsidian`
- task provider: GitHub Issues in `Digital-Collective-Games/Obsidian`
- task proposal provider: GitHub Issues in
  `Digital-Collective-Games/ObsidianProposals`

Accepted GitHub-backed tasks should use the GitHub issue number as the local
task number at task materialization time. Proposals are not dispatchable tasks.
Rejected proposals stay in the proposal provider and do not create local task
folders.

TaskCreate needs follow-on process work: a GitHub provider-interface subdocument
should define how TaskCreate creates proposal issues, promotes proposals,
creates accepted task issues, and materializes
`Tracking/Task-<issue-number>/TASK.md` after GitHub returns the issue number.
That follow-on is the last pass of this task's closeout planning; implementing
the full Review tab or proposal workflow remains out of scope.

## Sync Contract

Pilot sync may use a hidden stable marker in the issue body, but it must not
depend on `codex-*` labels or tags for task identity. Task/proposal meaning
comes from the provider repo binding in `CODEX-REPO-MANIFEST.json`.

Queue, priority, and human-needed state must not be represented as labels.
GitHub's label picker cannot enforce mutual exclusion, so labels distort the
human operation. For the current org-backed pilot, GitHub Issue Fields on the
configured task provider own the visible dropdowns: `Queue`, `Priority`, and
`Human Needed`. This repo manifest must not define queue or priority option
schemas; it only points Codex at the provider surface.

`CODEX-REPO-MANIFEST.json` only binds provider surfaces for this repo. It does
not own the queue schema.

Example marker:

```markdown
<!-- task-sync: repo=CodexDashboard; task_id=Task-0001; task_path=Tracking/Task-0001/TASK.md -->
```

Pilot issue fields:

- title: `Task-0001: Codex token-velocity dashboard with a hotkey-first overlay.`
- state: `open` for the pilot unless the human explicitly closes it
- body:
  - stable marker
  - source-of-truth statement
  - local task path
  - summary
  - human outcome
  - acceptance criteria
  - non-goals/follow-ons

`Tracking/Task-0001/TASK-META.json` should record:

- schema version
- provider kind
- provider repo
- issue number
- issue URL
- last synced time

These fields are immediately relevant to rerunning the `gh` sync against the
same issue and verifying the provider binding. `last_synced_at` records when
Codex accepted the local/GitHub sync checkpoint; it is not a claim about the
latest remote edit time. Remote edit detection comes from live GitHub
title/body readback at the time Codex is preparing to write. Rich task truth
stays in `TASK.md`; source-control truth stays in git and the repo registry.

The normalized sync surface is
`skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1`. It derives the target issue number from
the local `Tracking/Task-<id>/TASK.md` folder, checks the embedded top-level
task id, generates the issue marker/title/body, rejects mismatches, and writes
metadata only after `gh issue view` readback succeeds. It also creates and
removes rejected workflow/identity labels if they were previously applied.

## Why This Mechanism

The right first mechanism is regular Codex running `gh`, not:

- a backend endpoint
- a desktop UI button
- a bulk exporter
- a query-only dashboard with no published task state to inspect
- a central queue-draining coordinator
- a GitHub-label-based runtime lock

The first risk is representation quality and co-ownership semantics. Codex can
answer that with less plumbing than a backend path, while still preserving a
durable task metadata and proof trail.

## Scope Rationale

The task intentionally stops at one pilot GitHub-backed task-state
representation.

This is the right boundary because the human explicitly wants to inspect one
GitHub-backed representation and agree that it meets the quality bar before
scaling up.

Smaller rejected boundary:

- only documenting a proposed issue format, because that would not prove `gh`
  auth, create/update behavior, readback/queryability, local task metadata, or
  rendered issue readability

Larger rejected boundaries:

- bulk-publishing all task docs across configured repos, because a bad issue
  shape would be multiplied before inspection
- implementing `drain my tasks pls`, because queue draining depends on
  trustworthy publication, repo registry bindings, concurrency limits, and worktree
  allocation rules that are separate follow-on tasks
- building a cross-repo issue dashboard before a single issue representation is
  accepted

### Scope Update (2026-05-28)

The one-issue pilot remains the first acceptance gate, and the human accepted it
(see [Testing/PILOT-ISSUE-REVIEW-PACKAGE.md](./Testing/PILOT-ISSUE-REVIEW-PACKAGE.md)).
After that acceptance, the human approved continuing within this repo only:
the accepted issue shape was used to bootstrap CodexDashboard's existing local
tasks `Task-0001`..`Task-0012` into the configured accepted-task provider
(`Digital-Collective-Games/Obsidian`, issues `#1`..`#12`), with reconcile and
issue-state correction. This is repo-local bootstrap, not cross-repo bulk
publication. Cross-repo bulk publication, the Review tab proposal workflow,
TaskCreate provider-interface implementation, worktree allocation, concurrency,
and `drain my tasks pls` remain follow-on work and out of scope unless the human
explicitly rescopes Task-0012. Proof:
[Testing/BULK-ISSUE-BOOTSTRAP-PROOF.md](./Testing/BULK-ISSUE-BOOTSTRAP-PROOF.md)
and [Testing/RECONCILE-PROOF.md](./Testing/RECONCILE-PROOF.md).

## Implementation Home

Task-owned implementation/proof artifacts:

- `CODEX-REPO-MANIFEST.json`
- `skills/obsidian-operator/SKILL.md` and bundled scripts:
  - `skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1`
  - `skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1`
  - `skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1`
- `Tracking/Task-0001/TASK-META.json` through `Tracking/Task-0012/TASK-META.json`
  (provider bindings written after readback for each bootstrapped task)
- `Tracking/Task-0012/Testing/`
- `Tracking/Task-0012/Testing/PILOT-ISSUE-BODY.md`
- `Tracking/Task-0012/Testing/PILOT-ISSUE-VIEW.json`
- `Tracking/Task-0012/Testing/PILOT-ISSUE-REVIEW-PACKAGE.md`
- `Tracking/Task-0012/Testing/BULK-ISSUE-BOOTSTRAP-PROOF.md` and
  `Tracking/Task-0012/Testing/BulkIssueBootstrap/`
- `Tracking/Task-0012/Testing/RECONCILE-PROOF.md` and
  `Tracking/Task-0012/Testing/TaskGitHubReconcile/`
- `Tracking/Task-0012/BUG-0001.md` (gh UTF-8 readback decode fix)

No backend package or HTTP API is required for Task-0012.

## Goals

- Push one selected local `TASK.md` into GitHub task state through regular Codex
  and `gh`.
- Add a repo-root registry that records source-control, accepted-task, and
  task-proposal providers for CodexDashboard.
- Keep the local `TASK.md` file as the rich task source of truth.
- Define co-ownership semantics between local task docs and GitHub Issues.
- Make the resulting issue readable and queryable enough for human inspection
  without hidden chat context.
- Preserve durable local task metadata from task doc to GitHub Issue.
- Make rerunning the pilot update the same issue instead of creating duplicates.
- Fail closed when `gh` is missing authentication or cannot read/write the
  issue truthfully.
- Leave behind proof artifacts that support a human decision about whether the
  issue shape is good enough for bulk publication and later queue draining.

## Non-Goals

- Backend orchestration endpoint for publication.
- Desktop UI for publication.
- Cross-repo bulk publication of `TASK.md` files across all other configured
  repos. (Repo-local bootstrap of CodexDashboard's own existing tasks
  `Task-0001`..`Task-0012` was later performed under explicit human approval;
  see the dated scope update at the end of `Scope Rationale`.)
- Full cross-repo issue query dashboard.
- Central queue draining.
- Dispatching work from GitHub Issues.
- Concurrency limits for active tasks.
- Worktree allocation or release.
- Making GitHub labels canonical for live run locks, waits, interrupts, cleanup,
  or active concurrency.
- Replacing local task docs with GitHub Issue bodies.
- Replacing backend task-run state with GitHub comments or labels.
- Implementing the TaskCreate GitHub provider-interface subdocument in shared
  `.codex` process docs, beyond recording it as required follow-on work.
- Building the Review tab proposal promotion workflow.

## Expected Resolution

The human can inspect one GitHub Issue created or updated from one local
`TASK.md`, see the raw `gh` readback and local task metadata, and decide whether
this co-owned issue shape is accepted for later bulk publication.

The final closeout should also make clear that the pilot issue metadata is not
the steady-state identity convention. The accepted steady-state shape is the
repo registry plus a future TaskCreate provider-interface workflow where
GitHub issue creation allocates the accepted task number before
`Tracking/Task-<issue-number>/TASK.md` is materialized.

If `gh` is not authenticated, Codex records a clear blocked result with no fake
issue URL and no misleading task metadata artifact.

If `gh` is authenticated, Codex creates or updates one issue, reads it back,
writes the local task metadata artifact, and packages the result for human
review.

## What Does Not Count

- A backend endpoint with no real pilot issue inspection.
- A dry-run body preview with no real publish or explicit blocked state.
- A manually created GitHub Issue that bypasses the Codex sync procedure.
- A GitHub Issue that only links to `TASK.md` without enough summary, goals, and
  acceptance context to inspect.
- Copying every local research, proof, audit, pass-history, and review artifact
  into the issue body and pretending GitHub now owns rich task truth.
- A write-only publisher that cannot read the issue back through `gh`.
- A sync that overwrites live GitHub title/body differences without reporting a
  `text_conflict` and resolving it before any GitHub write for that issue.
- A reconciliation flow that treats every difference as a dispatchable action,
  or allows a GitHub write on an issue with an unresolved text conflict.
- Creating duplicate issues when the pilot is rerun.
- Writing `TASK-META.json` after auth failure or failed readback.
- Treating issue labels or comments as canonical live execution state.
- Treating this task as proof that bulk publication or queue draining is ready.
- Treating the pilot `Task-0012` to issue `#1` metadata as the final identity
  convention for future GitHub-backed tasks.
- Hiding the TaskCreate provider-interface follow-on after deciding that
  GitHub issue creation should allocate accepted task numbers.

## Acceptance Criteria

- Task-0012 uses a Codex-operated `gh` workflow, not a backend endpoint.
- Codex selects exactly one pilot local task:
  `Tracking/Task-0001/TASK.md`, because GitHub issue `#1` maps to
  local `Task-0001`.
- Codex validates the target GitHub repo as explicit `owner/name`.
- Before any write, Codex runs `skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1`, which
  performs task-id consistency checks and `gh auth status -h github.com`.
- When `gh` is not authenticated or cannot operate, Codex records `blocked`
  proof and does not create/update `TASK-META.json`.
- Codex renders issue title/body and saves the preview under
  `Tracking/Task-0012/Testing/`.
- When authenticated, Codex updates GitHub issue `#1` with the selected title
  and body for `Task-0001`.
- The sync script refuses to let issue `#1` carry `Task-0012` identifiers.
- Codex reads the issue back with `gh issue view --json
  number,title,body,state,labels,url`.
- The issue title/body include:
  - hidden stable marker
  - source-of-truth statement
  - task id and local task path
  - task title
  - summary or context
  - human outcome
  - acceptance criteria
  - follow-on/non-goal boundary
- The issue does not need `codex-*` labels or tags to be identifiable as an
  accepted task; the provider repo binding supplies that meaning.
- The issue does not use labels for queue, priority, or human-needed state.
- Queue and priority values are not defined by this repo or its manifest.
- The issue representation is suitable for later cross-repo `gh` queries by
  provider repo and state.
- `Tracking/Task-0001/TASK-META.json` records the provider metadata and
  minimal readback snapshot only after successful publish/update/readback.
- `Testing/PILOT-ISSUE-REVIEW-PACKAGE.md` distinguishes the `gh` pilot proof
  from the final registry/provider convention before bulk publication.
- `CODEX-REPO-MANIFEST.json` exists at repo root, parses as JSON, and names the
  CodexDashboard source-control provider, task provider, and task-proposal
  provider.
- Task-0012 records that a TaskCreate GitHub provider-interface subdocument is
  required follow-on work before the new issue-number task materialization
  convention should be treated as fully operational.

### Additional Acceptance (post-pilot, human-approved 2026-05-28)

These criteria cover the human-approved repo-local continuation. They do not
replace the one-issue pilot criteria above.

- `Bootstrap-TaskGitHubIssues.ps1` created issues `#2`..`#12` for
  `Task-0002`..`Task-0012` with one-to-one folder/issue identity and wrote each
  `TASK-META.json` after readback.
- `Reconcile-TaskGitHubState.ps1` reports `difference_count: 0` and
  `conflict_count: 0`, and the dispatch dry run reports zero items, as the
  closeout gate.
- Issue-state corrections (close/reopen) only run for a task with no
  unresolved `text_conflict`.
- The obsidian-operator scripts force UTF-8 console encoding so `gh` native
  output decodes correctly under Windows PowerShell 5.1; this is validated by a
  clean reconcile and recorded in [BUG-0001.md](./BUG-0001.md).

## Proof Plan

- Run `gh auth status -h github.com` and capture blocked proof if auth is
  unavailable.
- Render and save the pilot issue preview.
- If authenticated and a target repo is approved, run
  `skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1` for `Tracking/Task-0001/TASK.md`.
- Run `gh issue view --json number,title,body,state,labels,url`.
- Write `Tracking/Task-0001/TASK-META.json` only after successful readback.
- Create `Testing/PILOT-ISSUE-REVIEW-PACKAGE.md` with:
  - issue URL
  - rendered body preview
  - raw `gh issue view` JSON path
  - task metadata path
  - drift/conflict notes
  - exact approval question
- Parse `CODEX-REPO-MANIFEST.json` after adding the repo registry.
- Record the TaskCreate GitHub provider-interface follow-on before closure.

If GitHub auth is unavailable, the task can honestly enter a blocked state with
missing-auth proof, but it should not close as complete.

If GitHub publication succeeds but human inspection is pending, the task pauses
at a human gate. It does not close until the issue representation is accepted
for the next rollout step or rejected with required changes recorded.

## Falsifier

This task is wrong or incomplete if:

- the pilot can be marked successful without a real issue or explicit blocked
  state
- rerunning the pilot creates duplicate issues
- the issue body is too thin for human inspection without opening hidden chat
  context
- the issue body implies GitHub owns rich task truth or live execution state
- the task does not prove the published issue can be read back through `gh`
- co-ownership semantics are left implicit or contradicted by the artifacts
- local task metadata does not survive as a durable artifact beside the task docs
- missing auth produces a fake URL, fake issue number, or misleading success
  response
- the result cannot support a clear human decision about whether bulk
  publication should proceed
- the task closes without recording the registry/provider decision that
  supersedes the pilot issue metadata as steady-state policy
- the TaskCreate GitHub provider-interface follow-on is omitted after deciding
  that accepted task folder numbers should come from GitHub issue creation

## References

- [TASK-CREATE-OBJECTIVE.md](./TASK-CREATE-OBJECTIVE.md)
- [HUMAN-DIRECTIVES-FOR-WORKER.md](./HUMAN-DIRECTIVES-FOR-WORKER.md)
- [TASK-CREATE-COORDINATOR-DISPOSITION.md](./TASK-CREATE-COORDINATOR-DISPOSITION.md)
- [TASK-CREATE-CONTEXT-MANIFEST.md](./TASK-CREATE-CONTEXT-MANIFEST.md)
- [Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md](./Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md)
- [../Task-0008/Design/DURABLE-EXECUTION-STATE-CONTRACT.md](../Task-0008/Design/DURABLE-EXECUTION-STATE-CONTRACT.md)
