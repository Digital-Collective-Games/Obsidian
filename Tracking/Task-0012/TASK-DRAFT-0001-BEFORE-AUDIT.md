# Task 0012

## Title

Publish one local `TASK.md` to a GitHub Issue for human inspection.

## Summary

CodexDashboard needs a GitHub-backed task publication path, but the first slice must stay deliberately small.

The first useful outcome is not autonomous queue draining and not bulk cross-repo publication. The first useful outcome is:

- choose one local task
- publish a GitHub Issue that represents that task clearly enough for a human to inspect
- preserve the mapping between the local task document and the GitHub Issue
- prove that missing GitHub authentication blocks honestly instead of producing fake publication output
- decide from the pilot issue whether the issue title, body, labels, and links meet the quality bar before scaling up

GitHub Issues should become the cross-repo backlog and triage layer. Local task docs remain the rich task source of truth. CodexDashboard backend and Temporal remain the live execution source of truth.

## Writeup Type

Concrete implementation task.

The first solution shape is chosen: add a backend-mediated pilot publication path for one selected local `TASK.md` through `gh`, then inspect the resulting GitHub Issue and local mapping before bulk publication or queue draining.

## Burden Being Reduced

The human is starting to lose track of task status across local `Tracking/Task-*` trees and repos.

The exported work today is repeated memory and reconstruction work:

- remembering which local tasks exist
- remembering which tasks are ready, running, or stale
- finding task docs across repos
- deciding whether a task is visible enough to coordinate through GitHub

This task reduces only the first piece of that burden: proving that one local task can become a readable GitHub-backed backlog item without losing the richer local task contract.

## Current Truth

CodexDashboard currently has local task readback and backend task-run state, but no GitHub Issue publication path for tasks.

Current durable split:

- local task docs own scope, acceptance, research, proof, audits, and handoff
- the backend task-run contract owns live execution state, run ownership, waits, interrupts, cleanup, owned lanes, and restore commits
- shared worktree docs own reusable worktree-slot availability

What is missing:

- a `gh`-backed issue write path
- a durable local mapping from a task doc to its GitHub Issue
- an inspection-friendly issue body shape
- idempotent update behavior for a pilot issue
- fail-closed behavior when `gh` is not authenticated

The local machine has `gh` installed, but `gh auth status -h github.com` currently reports no logged-in GitHub hosts.

## Target Truth

After this task succeeds, CodexDashboard can publish exactly one selected local `TASK.md` as a GitHub Issue pilot and return enough local and GitHub evidence for human inspection.

The pilot issue should be readable as a backlog item without hidden chat context, while still making clear that the committed local task docs remain the richer source of task truth.

## Causal Claim

If CodexDashboard can publish one task through `gh`, preserve the local-to-GitHub mapping, and show exactly what was published, then the human can judge the GitHub representation before risking duplicate, low-quality, or over-broad issue creation across all repos.

## Evidence

- [Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md](./Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md) records the human need for cross-repo task querying, `gh` as the first integration path, and the source-of-truth split.
- [TASK-CREATE-OBJECTIVE.md](./TASK-CREATE-OBJECTIVE.md) records the required sequence: one published `TASK.md`, inspection, agreement on quality, then bulk publication, then queue draining.
- [HUMAN-DIRECTIVES-FOR-WORKER.md](./HUMAN-DIRECTIVES-FOR-WORKER.md) explicitly says not to draft this first task as full queue draining or autonomous dispatch.
- [../Task-0008/TASK.md](../Task-0008/TASK.md) and [../Task-0008/Design/DURABLE-EXECUTION-STATE-CONTRACT.md](../Task-0008/Design/DURABLE-EXECUTION-STATE-CONTRACT.md) define the backend-owned execution-state split this task must preserve.
- [../Task-0009/TASK.md](../Task-0009/TASK.md) shows the dashboard direction toward humane task supervision, but that UI work does not own GitHub publication.
- [../../backend/orchestration/README.md](../../backend/orchestration/README.md) documents the existing backend task APIs and validation/service lanes.

## Why This Mechanism

The right first mechanism is a backend-mediated `gh` publisher, not:

- a one-off manual issue creation process
- a bulk exporter
- a central queue-draining coordinator
- a GitHub-label-based runtime lock

The backend already owns task readback and task-run coordination in this repo. Putting the pilot publisher under that backend keeps publication close to the existing task parser, makes later UI/API inspection possible, and avoids turning a local script into an untracked source of task state.

## Scope Rationale

The task intentionally stops at one pilot publication.

This is the right first boundary because the human explicitly wants to inspect one GitHub-backed representation and agree that it meets the quality bar before scaling up.

Smaller rejected boundary:

- only documenting a proposed issue format, because that would not prove `gh` auth, create/update behavior, local mapping, or rendered issue readability.

Larger rejected boundaries:

- bulk-publishing all task docs across configured repos, because a bad issue shape would be multiplied before inspection.
- implementing `drain my tasks pls`, because queue draining depends on trustworthy publication, repo mapping, concurrency limits, and worktree allocation rules that are separate follow-on tasks.

## Implementation Home

Primary product home:

- `backend/orchestration/`

Expected implementation surfaces:

- `backend/orchestration/internal/config/config.go`
- `backend/orchestration/internal/httpapi/mux.go`
- `backend/orchestration/internal/taskrun/types.go`
- `backend/orchestration/internal/taskrun/service.go`
- a new backend package for GitHub Issue publication, for example `backend/orchestration/internal/taskpub/`
- focused tests under `backend/orchestration/internal/...`

Task-owned inspection artifacts:

- `Tracking/Task-0012/Testing/`

Per-published-task mapping artifact:

- `Tracking/Task-<id>/GITHUB-ISSUE.json`

## Implementation Home Rationale

This belongs in `backend/orchestration/` because the product behavior is task publication and task readback integration, not standalone operator scripting.

The backend already knows the configured worktree root, tracking root, task ids, task titles, meaning summaries, declared task roots, and task revisions. The GitHub pilot publisher should reuse that task-definition readback instead of reparsing local task folders in an unrelated script.

The durable mapping belongs beside the local task docs because local task artifacts remain the rich task source of truth and need to record which GitHub Issue indexes them.

## Internal Mechanism Map

### Mechanism 1: Pilot Publication API

Failure reduced:

- one-off manual issue creation cannot be inspected as a repeatable product path

Mechanism:

- add a backend endpoint that publishes or updates one selected local task through `gh`

### Mechanism 2: Issue Body Renderer

Failure reduced:

- GitHub Issues could become low-context links or bloated copies of every local artifact

Mechanism:

- render a bounded issue body from the local `TASK.md` with explicit source-of-truth language, local doc links, key task sections, and a hidden stable marker

### Mechanism 3: Local Mapping Artifact

Failure reduced:

- future runs cannot tell whether a task was already published, where it lives, or whether an update would duplicate it

Mechanism:

- write `Tracking/Task-<id>/GITHUB-ISSUE.json` with repo, issue number, issue URL, source commit, task revision, timestamps, and sync status

### Mechanism 4: Authentication And Failure Contract

Failure reduced:

- tooling could produce fake success or local-only output when GitHub auth is missing

Mechanism:

- run `gh auth status -h github.com` before write operations and return a blocked result without creating or updating the mapping artifact when auth is missing

### Mechanism 5: Inspection Artifacts

Failure reduced:

- the human cannot judge whether the issue shape is good enough before bulk rollout

Mechanism:

- capture the published issue URL, returned issue JSON, rendered body preview, and local mapping under task-owned testing artifacts

## Proposed Changes

- Add a pilot publication endpoint:
  - `POST /api/v1/tasks/{task_id}/github-publication/pilot`
  - request fields:
    - `github_repo`, formatted as `owner/name`
    - `labels`, defaulting to at least `codex-task` when omitted
    - `dry_run`, defaulting to `false`
    - optional `issue_number` for an explicit update target
  - response fields:
    - `status`: `published`, `updated`, `dry_run`, or `blocked`
    - `task_id`
    - `github_repo`
    - `issue_number`
    - `issue_url`
    - `mapping_path`
    - `source_commit`
    - `declared_task_revision`
    - `auth_status`
    - `body_preview_path` when captured
    - `block_reason` when blocked
- Add a read endpoint:
  - `GET /api/v1/tasks/{task_id}/github-publication`
  - reads the local `GITHUB-ISSUE.json` mapping when present
- Add a backend GitHub publication package that shells out to `gh` for:
  - `gh auth status -h github.com`
  - `gh issue create`
  - `gh issue edit`
  - `gh issue view --json number,title,body,state,labels,updatedAt,url`
- Add an issue-body renderer that produces a bounded GitHub body with:
  - a hidden stable marker containing local task id and declared task root
  - a source-of-truth statement saying the GitHub Issue is a backlog index and local docs own rich task truth
  - task title
  - summary or context excerpt
  - goals
  - acceptance criteria
  - what does not count, when present
  - links to committed local task docs when a source commit is available
  - source commit and declared task revision
- Add idempotency behavior:
  - if `GITHUB-ISSUE.json` exists and points to an open issue, update that issue
  - if `issue_number` is supplied, update that issue and then write the mapping
  - if neither exists, create one issue
  - do not create a duplicate when a valid mapping already exists
- Add `Tracking/Task-<id>/GITHUB-ISSUE.json` writing with at least:
  - `schema_version`
  - `local_repo_root`
  - `task_id`
  - `task_path`
  - `github_repo`
  - `issue_number`
  - `issue_url`
  - `issue_state_snapshot`
  - `issue_updated_at_snapshot`
  - `published_at`
  - `source_commit`
  - `declared_task_revision`
  - `github_sync_status`
  - `last_checked_at`
- Add task-owned proof under `Tracking/Task-0012/Testing/` for:
  - missing-auth blocked behavior
  - renderer output for the selected pilot task
  - successful publish or update when GitHub auth is available
  - readback of the local mapping and GitHub issue JSON

## Goals

- Publish one selected local `TASK.md` to GitHub through a repeatable backend path.
- Make the resulting issue readable enough for human inspection without hidden chat context.
- Keep GitHub as backlog identity, shallow discovery, triage, and links.
- Keep local task docs as rich scope, acceptance, research, proof, audits, and handoff.
- Keep backend/Temporal as live execution state owner.
- Preserve a durable local mapping from task doc to GitHub Issue.
- Make rerunning the pilot update the same issue instead of creating duplicates.
- Fail closed when `gh` is missing authentication.
- Leave behind proof artifacts that support a human decision about whether the issue shape is good enough for bulk publication.

## Non-Goals

- Bulk publication of all `TASK.md` files across configured repos.
- Cross-repo issue query dashboards.
- Central queue draining.
- Dispatching work from GitHub Issues.
- Concurrency limits for active tasks.
- Worktree allocation or release.
- Making GitHub labels canonical for live run locks, waits, interrupts, cleanup, or active concurrency.
- Replacing local task docs with GitHub Issue bodies.
- Replacing backend task-run state with GitHub comments or labels.
- Adding GitHub Projects fields in this first slice.

## Expected Resolution

The human can run one pilot publication against a selected local task, open the resulting GitHub Issue, and inspect whether the issue is a trustworthy task-index representation.

If `gh` is not authenticated, the backend returns a clear blocked result with no fake issue URL and no misleading mapping artifact.

If `gh` is authenticated, the backend creates or updates one issue, writes the local mapping artifact, and captures enough evidence for the human to decide whether to proceed to bulk publication.

## What Does Not Count

- A dry-run body preview with no real publish or explicit blocked state.
- A manually created GitHub Issue that bypasses the backend publication path.
- A GitHub Issue that only links to `TASK.md` without enough summary, goals, and acceptance context to inspect.
- Copying every local research, proof, audit, and handoff artifact into the issue body and pretending GitHub now owns rich task truth.
- Creating duplicate issues when the pilot is rerun.
- Writing `GITHUB-ISSUE.json` after auth failure.
- Treating issue labels or comments as canonical live execution state.
- Treating this task as proof that bulk publication or queue draining is ready.

## Acceptance Criteria

- `POST /api/v1/tasks/{task_id}/github-publication/pilot` exists and can target one selected local task by task id.
- The endpoint validates `github_repo` as an explicit `owner/name` target for this first slice.
- Before any non-dry-run write, the backend checks `gh auth status -h github.com`.
- When `gh` is not authenticated, the endpoint returns `status = blocked` with a clear `block_reason`, does not create or edit a GitHub Issue, and does not write `GITHUB-ISSUE.json`.
- When `dry_run = true`, the endpoint renders the issue title/body/labels and returns or captures the preview without creating or editing a GitHub Issue.
- When authenticated and no mapping exists, the endpoint creates exactly one GitHub Issue with the selected title, body, and labels.
- When a valid mapping or explicit `issue_number` exists, the endpoint updates that issue instead of creating a duplicate.
- The issue body includes:
  - the hidden stable marker
  - source-of-truth statement
  - task title
  - summary or context
  - goals
  - acceptance criteria
  - source commit when available
  - local task doc link or path
- The issue body does not claim the GitHub Issue is the canonical home for research, proof, audits, handoff, or live runtime state.
- After successful publish or update, `Tracking/Task-<id>/GITHUB-ISSUE.json` records the local/GitHub mapping and source revision fields named in `Proposed Changes`.
- `GET /api/v1/tasks/{task_id}/github-publication` returns the mapping when present and a clear not-published result when absent.
- Focused unit tests cover issue body rendering, auth-block behavior, mapping write/read, and create-versus-update decision logic with a fake `gh` runner.
- Task-owned proof under `Tracking/Task-0012/Testing/` includes:
  - the missing-auth blocked response from the current local environment
  - a body preview for the selected pilot task
  - a successful authenticated publish or update record
  - the raw `gh issue view --json ...` output when authenticated proof is available

## Proof Plan

- Run focused backend tests for the renderer, fake `gh` client, auth failure, mapping persistence, and create/update decision path.
- In the current unauthenticated environment, call the pilot endpoint and capture the blocked response showing no fake publication happened.
- Run a dry-run preview for the selected pilot task and save the rendered body under `Tracking/Task-0012/Testing/`.
- After GitHub auth is available, run one real publish or update through the backend endpoint.
- Capture:
  - backend response
  - `GITHUB-ISSUE.json`
  - `gh issue view --json number,title,body,state,labels,updatedAt,url`
  - a human-inspection note saying whether the issue shape is accepted for bulk publication or what must change first

If GitHub auth is not available, the task can honestly enter a blocked state with the missing-auth proof, but it should not be closed as complete.

## Falsifier

This task is wrong or incomplete if:

- the pilot can be marked successful without a real issue or an explicit blocked state
- rerunning the pilot creates duplicate issues
- the issue body is too thin for human inspection without opening hidden chat context
- the issue body implies GitHub owns rich task truth or live execution state
- local mapping does not survive as a durable artifact beside the task docs
- missing auth produces a fake URL, fake issue number, or misleading success response
- the result cannot support a clear human decision about whether bulk publication should proceed

## Rival Explanations Considered

- `The real problem is only that local task docs need better status fields.`
  - rejected because the human specifically needs cross-repo backlog querying and GitHub-backed visibility.
- `The real problem is only missing queue draining.`
  - rejected because queue draining should not run on top of an unproven issue representation.
- `The real problem is only missing frontend display.`
  - rejected because the first risk is publication quality and source-of-truth split, not dashboard rendering.

## Rival Mechanisms Considered

- manual GitHub Issue creation
  - rejected because it would not create a repeatable backend path, local mapping, or idempotency proof
- direct native GitHub API calls first
  - rejected because `gh` is installed and is the smallest practical integration path
- bulk exporter first
  - rejected because issue-shape mistakes would be multiplied before human inspection
- queue-draining coordinator first
  - rejected because dispatch, concurrency, and worktree allocation depend on trustworthy task publication and should remain follow-on work

## Tradeoffs

- A backend-mediated pilot is heavier than a script, but it attaches to the existing task readback contract and can later support the dashboard.
- A bounded issue body preserves source-of-truth split, but it may require iteration if the human wants more or less task detail in GitHub.
- Requiring auth for real writes blocks final publication proof on local credentials, but it prevents fake success.

## Shared Substrate

- Existing task readback and task-run contract under `backend/orchestration/`
- Existing local task docs under `Tracking/Task-*`
- Future bulk GitHub publication task
- Future cross-repo queue-draining task
- Shared worktree-slot rules in the user-level orchestration docs, for later dispatch work only

## Not Solved Here

- Selecting every repo that should participate in bulk publication.
- Defining the final cross-repo repo registry.
- Defining the label convention for dispatch eligibility beyond the pilot `codex-task` label.
- Importing existing GitHub Issues back into local task folders.
- Publishing completed historical tasks.
- GitHub Projects integration.
- Backend queue draining and worktree allocation.

## Open Questions

No blocking question changes the writeup type, implementation home, first-slice boundary, enforcement point, or acceptance bar.

Implementation still needs operator input for the first real publish run:

- which GitHub repo should receive the pilot issue
- whether the default pilot should be this task's `Tracking/Task-0012/TASK.md` or another selected local task
- when GitHub auth should be enabled for real publication proof

These are execution inputs for the pilot run, not reasons to broaden the task.

## References

- [TASK-CREATE-OBJECTIVE.md](./TASK-CREATE-OBJECTIVE.md)
- [HUMAN-DIRECTIVES-FOR-WORKER.md](./HUMAN-DIRECTIVES-FOR-WORKER.md)
- [TASK-CREATE-CONTEXT-MANIFEST.md](./TASK-CREATE-CONTEXT-MANIFEST.md)
- [Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md](./Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md)
- [../Task-0008/TASK.md](../Task-0008/TASK.md)
- [../Task-0008/PLAN.md](../Task-0008/PLAN.md)
- [../Task-0008/Design/DURABLE-EXECUTION-STATE-CONTRACT.md](../Task-0008/Design/DURABLE-EXECUTION-STATE-CONTRACT.md)
- [../Task-0009/TASK.md](../Task-0009/TASK.md)
- [../../backend/orchestration/README.md](../../backend/orchestration/README.md)
