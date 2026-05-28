# Task 0012

## Title

Publish one local `TASK.md` to a GitHub Issue for human inspection.

## Summary

CodexDashboard needs a GitHub-backed task publication path, but the first slice must stay deliberately small.

The first useful outcome is not autonomous queue draining and not bulk cross-repo publication. The first useful outcome is:

- choose one local task
- publish a GitHub Issue that represents that task clearly enough for a human to inspect
- preserve the mapping between the local task document and the GitHub Issue
- prove that missing, unusable, unauthenticated, timed-out, or failing `gh` commands block honestly instead of producing fake publication output
- decide from the pilot issue whether the issue title, body, labels, and links meet the quality bar before scaling up

GitHub Issues should become the cross-repo backlog and triage layer. Local task docs remain the rich task source of truth. CodexDashboard backend and Temporal remain the live execution source of truth.

Successful issue creation or update is not terminal closure for this task. The only terminal closure path is explicit human acceptance of the pilot issue shape as good enough for the next bulk-publication slice. If the issue is published but not yet accepted, the task remains in schema-valid nonterminal review state with `status: "in_progress"`, `phase: "closure"`, `current_gate: "handoff"`, no blockers, and the review package listed in `next_expected_artifacts`. If the pilot issue shape is rejected or publication/readback fails, the task remains in nonterminal `blocked` state with the blocker named.

## Writeup Type

Burden-reduction concrete implementation task.

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
- remote and provenance validation so GitHub source links are only claimed when true
- fail-closed behavior for missing, unusable, unauthenticated, timed-out, or otherwise failing `gh`
- data-handling and restore expectations for the new durable mapping state

The local machine has `gh` installed, but `gh auth status -h github.com` currently reports no logged-in GitHub hosts.

## Target Truth

After this task succeeds, CodexDashboard can publish exactly one selected local `TASK.md` as a GitHub Issue pilot, preserve durable mapping state for that publication, and return enough local and GitHub evidence for human inspection.

The pilot issue should be readable as a backlog item without hidden chat context, while still making clear that the committed local task docs remain the richer source of task truth.

The target terminal truth is:

- human accepted the pilot issue shape for follow-on bulk publication

Until that happens, the durable task state must stay nonterminal:

- `status: "in_progress"`, `phase: "closure"`, `current_gate: "handoff"`, `blockers: []`, and `next_expected_artifacts` naming the review package and `HANDOFF.md` when the pilot was published or updated and the review package is waiting for human decision
- `status: "blocked"` only for real blockers such as missing `gh` auth, failed `gh` commands, unreadable issue readback, invalid publication inputs, invalid provenance that cannot be rendered honestly, a readable closed mapped issue, or a rejected pilot issue shape

Those nonterminal states must be recorded using only the shared `TASK-STATE.json` schema fields and summarized in `Tracking/Task-0012/HANDOFF.md`. `Tracking/Task-0012/Testing/PILOT-ISSUE-REVIEW-PACKAGE.md` is the human action target when review is pending, and `HANDOFF.md` must carry the human review request.

## Causal Claim

If CodexDashboard can publish one task through `gh`, preserve the local-to-GitHub mapping, and show exactly what was published, then the human can judge the GitHub representation before risking duplicate, low-quality, or over-broad issue creation across all repos.

## Evidence

- [Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md](./Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md) records the human need for cross-repo task querying, `gh` as the first integration path, and the source-of-truth split.
- [TASK-CREATE-OBJECTIVE.md](./TASK-CREATE-OBJECTIVE.md) records the required sequence: one published `TASK.md`, inspection, agreement on quality, then bulk publication, then queue draining.
- [HUMAN-DIRECTIVES-FOR-WORKER.md](./HUMAN-DIRECTIVES-FOR-WORKER.md) explicitly says not to draft this first task as full queue draining or autonomous dispatch.
- [../Task-0008/TASK.md](../Task-0008/TASK.md) and [../Task-0008/Design/DURABLE-EXECUTION-STATE-CONTRACT.md](../Task-0008/Design/DURABLE-EXECUTION-STATE-CONTRACT.md) define the backend-owned execution-state split this task must preserve.
- [../Task-0009/TASK.md](../Task-0009/TASK.md) shows the dashboard direction toward humane task supervision, but that UI work does not own GitHub publication.
- [../../backend/orchestration/README.md](../../backend/orchestration/README.md) documents the existing backend task APIs and validation/service lanes.
- [../../DATA-HANDLING.md](../../DATA-HANDLING.md) requires CodexDashboard tasks that change task state storage or restore expectations to update data-handling before closure.
- [../../TESTING.md](../../TESTING.md) requires backend proof to use the isolated validation lane by default and treat the service or human lane as off-limits unless explicitly authorized.
- [TASK-AUDIT-0001.md](./TASK-AUDIT-0001.md) identified the missing data-handling scope, human-inspection gate, and broader `gh` failure contract in the first draft.
- [TASK-AUDIT-0002.md](./TASK-AUDIT-0002.md) identified the missing burden-reduction sections, proxy closeout wording, and missing validation-lane proof requirement in the second draft.
- [TASK-AUDIT-0003.md](./TASK-AUDIT-0003.md) identified the schema-invalid review-pending status, missing remote/provenance validation, and undefined closed mapped issue behavior in the third draft.

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

If the human rejects the pilot issue shape, the rejection blocks bulk `TASK.md` publication and central queue-draining follow-ons until this pilot task is revised or rerun with an accepted issue representation.

Smaller rejected boundary:

- only documenting a proposed issue format, because that would not prove `gh` auth, create/update behavior, local mapping, or rendered issue readability.

Larger rejected boundaries:

- bulk-publishing all task docs across configured repos, because a bad issue shape would be multiplied before inspection.
- implementing `drain my tasks pls`, because queue draining depends on trustworthy publication, repo mapping, concurrency limits, and worktree allocation rules that are separate follow-on tasks.

## Human Relief If Successful

If this task succeeds, the human gets one trustworthy GitHub-backed task index entry they can inspect without rereading hidden chat or reconstructing local task state by memory.

The immediate relief is limited but concrete:

- the human can judge one real issue shape before it is multiplied across repos
- the human does not have to remember which local task maps to which GitHub Issue
- the human can reject the representation before bulk publication or queue draining builds on it
- later agents have a durable local mapping instead of asking the human to recover the GitHub issue identity

The larger relief remains follow-on work. This task only earns the first visible publication gate.

## Remaining Uncertainty

- The exact pilot GitHub repo and pilot task id are still execution inputs.
- The human may reject the first issue body shape after inspection, which would require revising this pilot before bulk publication.
- `gh` authentication may remain unavailable until the human authorizes or completes login for the selected GitHub host.
- Source commits may be unavailable, unpushed, or not reachable from the selected GitHub repo; the issue body must use explicit local-only provenance in that case instead of pretending local paths are durable GitHub links.
- The final bulk-publication label set, repo registry, and queue-draining eligibility rules remain outside this task.

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

Repo data-handling home:

- `DATA-HANDLING.md`

Default proof lane:

- CodexDashboard validation lane documented in `TESTING.md`
- backend URL `http://127.0.0.1:14318`
- Temporal `127.0.0.1:17233`
- Postgres `15432`

## Implementation Home Rationale

This belongs in `backend/orchestration/` because the product behavior is task publication and task readback integration, not standalone operator scripting.

The backend already knows the configured worktree root, tracking root, task ids, task titles, meaning summaries, declared task roots, and task revisions. The GitHub pilot publisher should reuse that task-definition readback instead of reparsing local task folders in an unrelated script.

The durable mapping belongs beside the local task docs because local task artifacts remain the rich task source of truth and need to record which GitHub Issue indexes them.

That mapping is task state storage, not disposable proof. Because it changes restore expectations for task identity and GitHub synchronization, this task must also update repo data-handling documentation before closure.

All backend proof belongs in the isolated validation lane by default. Publishing or proving against the service lane, the human's dashboard lane, the human's active config/database, or live human data requires explicit human authorization for this task run.

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
- include committed GitHub task-doc links only when remote/provenance validation proves the selected GitHub repo matches a configured or discovered local remote and the source commit is pushed/reachable there
- otherwise render explicit local-only provenance and no committed GitHub task-doc links

### Mechanism 3: Durable Local Mapping Artifact

Failure reduced:

- future runs cannot tell whether a task was already published, where it lives, or whether an update would duplicate it

Mechanism:

- write `Tracking/Task-<id>/GITHUB-ISSUE.json` with repo, issue number, issue URL, source commit, task revision, timestamps, and sync status
- classify that file as durable task mapping state that must be backed up and restored with the repo task artifacts

### Mechanism 4: `gh` Execution Failure Contract

Failure reduced:

- tooling could produce fake success, hang indefinitely, crash the endpoint, or write misleading mapping state when `gh` is missing, unusable, unauthenticated, timed out, or returns a non-auth command failure

Mechanism:

- resolve and invoke `gh` through argv-style command execution with bounded timeouts
- submit rendered issue bodies through a temp file, preview file, or equivalent `--body-file` path rather than shell interpolation or large Markdown text on a composed command line
- run `gh auth status -h github.com` before write operations
- return a blocked result without creating, editing, or updating the mapping artifact when `gh` cannot complete the required operation truthfully

### Mechanism 5: Inspection Artifacts

Failure reduced:

- the human cannot judge whether the issue shape is good enough before bulk rollout

Mechanism:

- capture the published issue URL, returned issue JSON, rendered body preview, and local mapping under task-owned testing artifacts

### Mechanism 6: Human Review Gate

Failure reduced:

- a task could be closed after successful publication even though the issue shape has not been accepted for bulk rollout

Mechanism:

- create a review package after real publication or update
- require recorded human acceptance before terminal closure
- record pending review as `status: "in_progress"`, `phase: "closure"`, `current_gate: "handoff"`, `blockers: []`, and `next_expected_artifacts` naming `Testing/PILOT-ISSUE-REVIEW-PACKAGE.md` and `HANDOFF.md`
- record rejected issue shape or failed publish/readback as `status: "blocked"` in `Tracking/Task-0012/TASK-STATE.json`, with `HANDOFF.md` linking the rejection or failure proof
- keep bulk publication and queue draining blocked until the review package records explicit acceptance

### Mechanism 7: Remote And Provenance Validation

Failure reduced:

- the issue body could contain GitHub links to committed task docs that are not actually reachable in the selected GitHub repo

Mechanism:

- discover local Git remotes and compare their GitHub owner/name targets to the selected `github_repo`
- check whether the referenced source commit is pushed/reachable in that matched GitHub remote before rendering committed task-doc links
- if the repo/commit link proof fails but the publication target is otherwise valid, publish or update only with `provenance_mode: "local_only"` and explicit local-path/source-commit wording
- block only when provenance inputs are malformed or local repo metadata cannot be inspected well enough to choose committed-link mode versus local-only mode honestly

### Mechanism 8: Closed Mapping Handling

Failure reduced:

- a readable closed issue in `GITHUB-ISSUE.json` could lead one implementation to update, reopen, replace, or duplicate it

Mechanism:

- if `GITHUB-ISSUE.json` points to a readable closed issue, return `status = blocked` with `block_reason = "mapped_issue_closed"`
- do not reopen the closed issue, do not edit it, do not create a replacement issue, and do not overwrite the mapping in this task
- record the closed mapped issue as a blocker requiring explicit human follow-up outside the automatic pilot path

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
    - `provenance_mode`: `committed_link` or `local_only`
    - `remote_match_status`
    - `source_commit_reachable`
    - `declared_task_revision`
    - `auth_status`
    - `body_preview_path` when captured
    - `block_reason` when blocked
    - `review_package_path` after successful publish or update
- Add a read endpoint:
  - `GET /api/v1/tasks/{task_id}/github-publication`
  - reads the local `GITHUB-ISSUE.json` mapping when present
- Add a backend GitHub publication package that invokes `gh` with argv-style command construction, no shell-composed command strings, and bounded per-command timeouts for:
  - `gh auth status -h github.com`
  - `gh issue create --body-file <body-file>`
  - `gh issue edit --body-file <body-file>`
  - `gh issue view --json number,title,body,state,labels,updatedAt,url`
- Add a `gh` failure classifier that returns `status = blocked` without writing mapping state for:
  - `gh` executable not found
  - `gh` executable found but unusable
  - auth failure
  - command timeout
  - non-auth `gh` command failure during create, edit, or view
  - malformed or missing JSON from `gh issue view`
- Validate publication inputs before invoking `gh`:
  - `github_repo` must be one `owner/name` pair
  - labels must be non-empty GitHub label names without command syntax
  - `issue_number`, when supplied, must be a positive integer
- Validate remote/provenance before rendering committed GitHub links:
  - discover configured local GitHub remotes for the local repo
  - compare the selected `github_repo` to those discovered remote owner/name targets
  - verify that `source_commit` is pushed/reachable in the matching GitHub repo before generating committed task-doc URLs
  - if remote or commit reachability cannot be proven but the target repo is otherwise valid, set `provenance_mode: "local_only"` and render no committed GitHub task-doc links
  - if repo/provenance inputs are malformed or cannot be inspected honestly, return `status = blocked` with a clear provenance `block_reason`
- Add an issue-body renderer that produces a bounded GitHub body with:
  - a hidden stable marker containing local task id and declared task root
  - a source-of-truth statement saying the GitHub Issue is a backlog index and local docs own rich task truth
  - task title
  - summary or context excerpt
  - goals
  - acceptance criteria
  - what does not count, when present
  - links to committed local task docs only when `provenance_mode: "committed_link"` is proven
  - source commit and declared task revision
  - clear local-only path wording when a source commit, matching remote, or pushed link is unavailable, so the issue does not pretend an unpushed or wrong-repo local path is a durable GitHub link
- Submit the rendered body to `gh` through a temp file, captured preview file, or equivalent argv-safe `--body-file` mechanism. Do not interpolate the Markdown body into a shell command.
- Add idempotency behavior:
  - if `GITHUB-ISSUE.json` exists and points to an open issue, update that issue
  - if `GITHUB-ISSUE.json` exists and points to a readable closed issue, return `status = blocked` with `block_reason = "mapped_issue_closed"`; do not reopen, edit, replace, duplicate, or overwrite the mapping
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
  - `provenance_mode`
  - `remote_match_status`
  - `source_commit_reachable`
  - `declared_task_revision`
  - `github_sync_status`
  - `last_checked_at`
- Treat `Tracking/Task-<id>/GITHUB-ISSUE.json` as durable task mapping state:
  - include it in task restore expectations
  - do not classify it as throwaway proof output
  - do not write it after blocked or failed `gh` operations
  - reconcile it with GitHub issue readback before using it as an update target
- Update `DATA-HANDLING.md` before task closure to document:
  - `Tracking/Task-<id>/GITHUB-ISSUE.json` as repo-backed task mapping state
  - how it is backed up and restored with the repo task artifacts
  - how stale or missing mappings should be handled during restore
- Add task-owned proof under `Tracking/Task-0012/Testing/` for:
  - blocked behavior for each supported `gh` failure class
  - renderer output for the selected pilot task
  - successful publish or update when GitHub auth is available
  - readback of the local mapping and GitHub issue JSON
  - human review package for the pilot issue shape
- Add a review package at `Tracking/Task-0012/Testing/PILOT-ISSUE-REVIEW-PACKAGE.md` after real publish or update with:
  - selected task id
  - GitHub repo and issue URL
  - rendered body preview path
  - `GITHUB-ISSUE.json` path
  - raw `gh issue view` capture path
  - source commit or explicit source-commit-unavailable note
  - acceptance checklist for issue title, body, labels, links, source-of-truth wording, and local mapping
  - exact approval question: `Do you accept this pilot GitHub Issue shape as the representation to use for bulk TASK.md publication?`
  - human decision field: `accepted`, `rejected`, or `pending`
- Add or update task-owned state artifacts for nonterminal review outcomes:
  - pending human review uses `Tracking/Task-0012/TASK-STATE.json` with `status: "in_progress"`, `phase: "closure"`, `current_gate: "handoff"`, `blockers: []`, and `next_expected_artifacts` naming `Testing/PILOT-ISSUE-REVIEW-PACKAGE.md` and `HANDOFF.md`
  - blocked outcomes use `status: "blocked"` only for real blockers and name the blocker in `blockers`
  - `Tracking/Task-0012/HANDOFF.md` summarizes the same nonterminal state and carries the human review request or blocker next action

## Goals

- Publish one selected local `TASK.md` to GitHub through a repeatable backend path.
- Make the resulting issue readable enough for human inspection without hidden chat context.
- Keep GitHub as backlog identity, shallow discovery, triage, and links.
- Keep local task docs as rich scope, acceptance, research, proof, audits, and handoff.
- Keep backend/Temporal as live execution state owner.
- Preserve a durable local mapping from task doc to GitHub Issue.
- Make rerunning the pilot update the same issue instead of creating duplicates.
- Fail closed when `gh` is missing, unusable, unauthenticated, times out, returns a non-auth command failure, or returns unusable JSON.
- Leave behind proof artifacts and a review package that support a human decision about whether the issue shape is good enough for bulk publication.
- Require human acceptance before this pilot can unblock bulk publication or queue draining.

## Non-Goals

- Bulk publication of all `TASK.md` files across configured repos.
- Cross-repo issue query dashboards.
- Central queue draining.
- Dispatching work from GitHub Issues.
- Using the service lane or the human's active dashboard lane for proof without explicit human authorization.
- Concurrency limits for active tasks.
- Worktree allocation or release.
- Making GitHub labels canonical for live run locks, waits, interrupts, cleanup, or active concurrency.
- Replacing local task docs with GitHub Issue bodies.
- Replacing backend task-run state with GitHub comments or labels.
- Adding GitHub Projects fields in this first slice.

## Expected Resolution

The human can run one pilot publication against a selected local task, open the resulting GitHub Issue, and inspect whether the issue is a trustworthy task-index representation.

If `gh` is missing, unusable, unauthenticated, timed out, or fails during a required command, the backend returns a clear blocked result with no fake issue URL and no misleading mapping artifact.

If `gh` works in the isolated validation lane, the backend creates or updates one pilot issue, writes the local mapping artifact, and captures a review package. Any service-lane or human-lane publication or proof requires explicit human authorization before it runs.

The task reaches terminal closure only when the human accepts the review package's pilot issue shape. If the review is pending or rejected, the task remains in a nonterminal state recorded in `Tracking/Task-0012/TASK-STATE.json` and `Tracking/Task-0012/HANDOFF.md`; bulk publication plus queue-draining follow-ons stay blocked.

## What Does Not Count

- A dry-run body preview with no real publish or explicit blocked state.
- A manually created GitHub Issue that bypasses the backend publication path.
- A GitHub Issue that only links to `TASK.md` without enough summary, goals, and acceptance context to inspect.
- Copying every local research, proof, audit, and handoff artifact into the issue body and pretending GitHub now owns rich task truth.
- Creating duplicate issues when the pilot is rerun.
- Writing `GITHUB-ISSUE.json` after any blocked or failed `gh` operation.
- Treating `GITHUB-ISSUE.json` as disposable proof instead of durable task mapping state.
- Treating issue labels or comments as canonical live execution state.
- Treating this task as proof that bulk publication or queue draining is ready.
- Closing after successful publish or update when the human has not accepted the pilot issue shape.
- Treating pending human review or `blocked` as task closure.
- Running backend proof against the service lane or human lane without explicit human authorization.
- Proceeding to bulk `TASK.md` publication or central queue draining after a rejected pilot issue shape.

## Acceptance Criteria

- `POST /api/v1/tasks/{task_id}/github-publication/pilot` exists and can target one selected local task by task id.
- The endpoint validates `github_repo` as an explicit `owner/name` target for this first slice, validates labels as labels rather than command fragments, and validates any supplied `issue_number` as a positive integer.
- The implementation invokes `gh` through argv-style command execution with bounded per-command timeouts, not shell-composed strings, and submits issue bodies through `--body-file` or an equivalent argv-safe file mechanism.
- Before any non-dry-run write, the backend checks `gh auth status -h github.com`.
- When `gh` is missing, unusable, unauthenticated, timed out, returns a non-auth command failure, or returns malformed issue JSON, the endpoint returns `status = blocked` with a clear `block_reason`, does not claim publish success, and does not write or update `GITHUB-ISSUE.json`.
- When `dry_run = true`, the endpoint renders the issue title/body/labels and returns or captures the preview without creating or editing a GitHub Issue.
- When authenticated and no mapping exists, the endpoint creates exactly one GitHub Issue with the selected title, body, and labels.
- When a valid mapping or explicit `issue_number` exists, the endpoint updates that issue instead of creating a duplicate.
- When an existing mapping points to an issue that cannot be viewed through `gh issue view`, the endpoint returns a blocked stale-mapping state rather than silently creating a duplicate or overwriting the mapping.
- When an existing mapping points to a readable closed issue, the endpoint returns `status = blocked` with `block_reason = "mapped_issue_closed"`, does not reopen or edit the issue, does not create a replacement issue, and does not overwrite the mapping.
- The endpoint discovers configured local GitHub remotes, compares them to the selected `github_repo`, and verifies the source commit is pushed/reachable before rendering committed GitHub task-doc links.
- If the selected repo does not match a discovered remote, or the source commit is unavailable, unpushed, or unreachable in that repo, the endpoint uses `provenance_mode: "local_only"` and the issue body/review package contain no committed GitHub task-doc links.
- The issue body includes:
  - the hidden stable marker
  - source-of-truth statement
  - task title
  - summary or context
  - goals
  - acceptance criteria
  - source commit or explicit source-commit-unavailable wording
  - committed GitHub task-doc link only when remote and commit reachability were proven, otherwise explicit local-only path/provenance wording
- When source commit, matching remote, or pushed reachability is unavailable, the issue body and review package say so explicitly and do not present local-only paths as committed GitHub links.
- The issue body does not claim the GitHub Issue is the canonical home for research, proof, audits, handoff, or live runtime state.
- After successful publish or update, `Tracking/Task-<id>/GITHUB-ISSUE.json` records the local/GitHub mapping and source revision fields named in `Proposed Changes`.
- `Tracking/Task-<id>/GITHUB-ISSUE.json` is treated as durable task mapping state in the implementation and in task closeout evidence, not as throwaway test proof.
- `DATA-HANDLING.md` is updated before task closure to document backup and restore expectations for `Tracking/Task-<id>/GITHUB-ISSUE.json` and any GitHub publication mapping state introduced by this task.
- `GET /api/v1/tasks/{task_id}/github-publication` returns the mapping when present and a clear not-published result when absent.
- Backend endpoint proof runs against the isolated validation lane by default, using `http://127.0.0.1:14318` and validation-lane data, not the service lane or the human's active dashboard lane.
- Any service-lane or human-lane publication, proof, config access, database access, or live-data use is absent from task proof unless explicit human authorization for this task run is recorded.
- Focused unit tests cover issue body rendering, body-file submission, missing/unusable `gh`, auth-block behavior, command timeout, non-auth command failure, malformed JSON, mapping write/read, stale-mapping behavior, closed mapped issue behavior, remote/provenance validation, local-only provenance rendering, input validation, and create-versus-update decision logic with a fake `gh` runner or equivalent test double.
- Task-owned proof under `Tracking/Task-0012/Testing/` includes:
  - blocked responses for the locally reproducible `gh` failure class and fake-runner proof for the remaining failure classes
  - a body preview for the selected pilot task
  - a successful authenticated publish or update record
  - the raw `gh issue view --json ...` output when authenticated proof is available
- `Tracking/Task-0012/Testing/PILOT-ISSUE-REVIEW-PACKAGE.md` exists after real publish or update and links the published issue, rendered body preview, local mapping, raw `gh issue view` capture, source commit state, and inspection checklist.
- The review package asks this exact approval question: `Do you accept this pilot GitHub Issue shape as the representation to use for bulk TASK.md publication?`
- Terminal task closure is allowed only when the review package records explicit human decision `accepted`.
- If the human decision is `pending`, the task remains nonterminal with `status: "in_progress"`, `phase: "closure"`, `current_gate: "handoff"`, `blockers: []`, and `next_expected_artifacts` naming `Testing/PILOT-ISSUE-REVIEW-PACKAGE.md` and `HANDOFF.md`; `HANDOFF.md` records the review package as the human action target.
- If the human decision is `rejected` or a required publish/readback step fails, the task remains nonterminal `blocked`; `Tracking/Task-0012/TASK-STATE.json` and `Tracking/Task-0012/HANDOFF.md` record the blocker and link the review package or failure proof.
- Bulk `TASK.md` publication and central queue-draining follow-ons remain blocked unless the pilot review package records human acceptance.

## Proof Plan

- Run focused backend tests for the renderer, fake `gh` client, missing/unusable `gh`, auth failure, timeout, non-auth command failure, malformed JSON, input validation, mapping persistence, stale mapping, and create/update decision path.
- Include focused tests for body-file submission, closed mapped issue blocking, remote/provenance validation, and local-only provenance rendering.
- Start and use the CodexDashboard isolated validation lane for backend endpoint proof by default:
  - `backend/orchestration/scripts/Start-ValidationLane.ps1`
  - backend URL `http://127.0.0.1:14318`
  - validation-lane data roots only
- Do not use the service lane, the human's active dashboard lane, the human's active config/database, or live human data for proof unless explicit human authorization for this task run is recorded.
- In the current unauthenticated environment, call the pilot endpoint in the validation lane and capture the blocked response showing no fake publication happened.
- Run a dry-run preview for the selected pilot task through the validation lane and save the rendered body under `Tracking/Task-0012/Testing/`.
- After GitHub auth is available and the target repo is selected, run one real publish or update through the validation-lane backend endpoint.
- Capture:
  - backend response
  - `GITHUB-ISSUE.json`
  - `gh issue view --json number,title,body,state,labels,updatedAt,url`
  - `Tracking/Task-0012/Testing/PILOT-ISSUE-REVIEW-PACKAGE.md`
  - a human-inspection decision saying whether the issue shape is accepted for bulk publication or what must change first

If GitHub auth is not available, the task can honestly enter nonterminal `blocked` state with the missing-auth proof recorded in `Tracking/Task-0012/TASK-STATE.json` and `Tracking/Task-0012/HANDOFF.md`, but it should not be closed as complete.

If GitHub publication succeeds but human inspection is still pending, the task must enter nonterminal review state with `status: "in_progress"`, `phase: "closure"`, `current_gate: "handoff"`, `blockers: []`, and the review package plus `HANDOFF.md` listed in `next_expected_artifacts`; it must not close.

If human inspection rejects the pilot issue shape, the task must enter nonterminal `blocked` in `Tracking/Task-0012/TASK-STATE.json` and `Tracking/Task-0012/HANDOFF.md`; it must not close or unblock follow-ons.

## Falsifier

This task is wrong or incomplete if:

- the pilot can be marked successful without a real issue or an explicit blocked state
- rerunning the pilot creates duplicate issues
- the issue body is too thin for human inspection without opening hidden chat context
- the issue body implies GitHub owns rich task truth or live execution state
- local mapping does not survive as a durable artifact beside the task docs
- the mapping is not covered by data-handling and restore expectations
- any `gh` failure produces a fake URL, fake issue number, misleading success response, or stale mapping update
- the result cannot support a clear human decision about whether bulk publication should proceed
- the task closes without recorded human acceptance of the pilot issue shape
- the task treats pending human review or `blocked` as terminal closure instead of nonterminal state
- a rejected pilot issue shape does not block bulk publication and queue draining
- committed GitHub task-doc links are rendered without proving a matching GitHub remote and pushed/reachable source commit
- a readable closed mapped issue can be silently reopened, edited, replaced, duplicated, or used to overwrite the mapping
- rendered Markdown bodies are passed through shell-composed command strings instead of an argv-safe body-file mechanism
- backend proof uses the service lane or human lane without explicit human authorization

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
- Requiring human acceptance before closure slows the first slice slightly, but it protects the rollout from scaling an issue shape the human rejects.
- Treating `GITHUB-ISSUE.json` as durable mapping state adds restore obligations, but it prevents future agents from losing or duplicating the GitHub task identity.

## Shared Substrate

- Existing task readback and task-run contract under `backend/orchestration/`
- Existing local task docs under `Tracking/Task-*`
- Repo data-handling and restore rules in `DATA-HANDLING.md`
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
- [TASK-AUDIT-0001.md](./TASK-AUDIT-0001.md)
- [TASK-AUDIT-0002.md](./TASK-AUDIT-0002.md)
- [TASK-AUDIT-0003.md](./TASK-AUDIT-0003.md)
- [Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md](./Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md)
- [../Task-0008/TASK.md](../Task-0008/TASK.md)
- [../Task-0008/PLAN.md](../Task-0008/PLAN.md)
- [../Task-0008/Design/DURABLE-EXECUTION-STATE-CONTRACT.md](../Task-0008/Design/DURABLE-EXECUTION-STATE-CONTRACT.md)
- [../Task-0009/TASK.md](../Task-0009/TASK.md)
- [../../DATA-HANDLING.md](../../DATA-HANDLING.md)
- [../../TESTING.md](../../TESTING.md)
- [../../backend/orchestration/README.md](../../backend/orchestration/README.md)
