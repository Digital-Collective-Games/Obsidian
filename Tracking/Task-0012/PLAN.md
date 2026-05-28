# Task-0012 Plan

## Status

Plan awaiting human approval.

Implementation must not start until this plan is approved or revised. The
approval gate is packaged in
[Design/PLAN-APPROVAL/REVIEW-PACKAGE.md](./Design/PLAN-APPROVAL/REVIEW-PACKAGE.md).

## Human Outcome

The human should be able to publish one selected local `TASK.md` as a readable
GitHub Issue pilot, inspect the issue shape, and decide whether the
representation is good enough before any bulk publication or queue-draining work
exists.

If GitHub authentication is missing, the backend should say that publication is
blocked and avoid fake issue numbers, fake URLs, or misleading mapping files.

## Implementation Passes

### PASS-0001: Backend Task Publication Core

Objective:

- add the backend package that renders issue bodies, validates publication
  requests, checks `gh` auth, decides create versus update, and reads/writes
  `GITHUB-ISSUE.json`

Expected product changes:

- new package under `backend/orchestration/internal/taskpub/`
- fakeable `gh` runner interface
- renderer for a bounded issue title/body/labels
- mapping model and persistence for `Tracking/Task-<id>/GITHUB-ISSUE.json`
- create/update decision logic that refuses duplicate creation when a valid
  mapping or explicit issue number exists

Verification:

- focused Go unit tests for renderer content, hidden stable marker,
  source-of-truth language, request validation, auth-block behavior, mapping
  read/write, and create/update selection

Exit bar:

- core behavior is unit-tested without contacting GitHub
- failed auth does not write the mapping artifact

### PASS-0002: HTTP API Integration

Objective:

- expose the pilot publication and mapping readback through the orchestration
  HTTP API

Expected product changes:

- `POST /api/v1/tasks/{task_id}/github-publication/pilot`
- `GET /api/v1/tasks/{task_id}/github-publication`
- route parsing in `backend/orchestration/internal/httpapi/mux.go`
- response/request types wired to task readback and publication core

Verification:

- focused Go HTTP tests covering dry-run, missing-auth blocked response, mapping
  readback not-published response, and successful fake create/update responses

Exit bar:

- endpoint responses match the fields named in `TASK.md`
- unsupported methods and malformed task ids/repos fail clearly

### PASS-0003: Pilot Proof Artifacts And Task Closeout Evidence

Objective:

- produce the task-owned proof artifacts needed for human inspection and
  closure or an honest auth blocker

Expected proof artifacts:

- missing-auth blocked response from the current local environment
- dry-run body preview for the selected pilot task
- backend response and mapping readback from fake or real endpoint proof
- successful authenticated publish/update record only when GitHub auth and a
  target repo are available
- `gh issue view --json number,title,body,state,labels,updatedAt,url` output
  only when real authenticated proof is available

Verification:

- backend Go tests
- repo-level Python unit suite if affected by task surface changes
- supporting backend smoke against an isolated validation lane if practical

Exit bar:

- if auth is unavailable, the task is blocked with durable proof and no closure
  claim
- if auth is available and the human provides a target repo/task, one real
  issue is created or updated and mapping/readback proof is captured

## Regression And QA Plan

This task adds backend/operator publication endpoints, not a new visible
desktop dashboard interaction. The required proof is therefore unit coverage,
backend API proof, and task-owned operator evidence for the pilot publication
path.

Repo-root `REGRESSION.md` still controls task closure. If implementation adds a
visible dashboard button or changes the real overlay surface, the plan must be
revised to add or update a named regression case before closure.

Clean-context QA is required before calling the issue shape human-approved if
the task produces a real GitHub issue for inspection. If no clean QA worker can
be launched, QA status must be recorded as blocked or non-conformant rather
than replaced with producer self-review.

## Data Handling

Backup impact expected:

- inventory unchanged

Reasoning:

- the task adds a local task-owned mapping artifact beside existing task docs
- it does not change human-lane service roots, dashboard config, SQLite
  behavior, Temporal/Postgres persistence, job specs, or restore procedures

Before closure, this impact must be confirmed against the actual diff.

## Out Of Scope

- bulk publishing all `TASK.md` files
- cross-repo issue query dashboards
- `drain my tasks pls`
- worktree allocation or release
- GitHub labels or comments as canonical live execution state
- GitHub Projects fields
