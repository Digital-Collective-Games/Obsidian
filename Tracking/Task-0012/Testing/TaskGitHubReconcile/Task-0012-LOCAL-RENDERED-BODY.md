<!-- task-sync: repo=CodexDashboard; task_id=Task-0012; task_path=Tracking/Task-0012/TASK.md -->

# Task-0012: Codex-operated pilot for GitHub-backed task state.

## Source Of Truth

Local `Tracking/Task-0012/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0012:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

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

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `12`
- Local task path: `Tracking/Task-0012/TASK.md`
- Source commit: `75177b6dee23399358ee66676791fb41dc01d51e`
- Local task SHA-256: `386941295E7B2D80450A4F233391F22214AE62EE334772A5FAE26377CF53C0A5`
- Rendered at: `2026-05-28T23:29:44.9878310-04:00`