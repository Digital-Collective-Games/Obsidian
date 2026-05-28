# Task Audit 0003

Audit date: 2026-05-27

Audit path: TaskCreate clean-context auditor-worker

Reviewed draft:

- [TASK-DRAFT-0003-BEFORE-AUDIT.md](./TASK-DRAFT-0003-BEFORE-AUDIT.md)

Current draft path:

- [TASK.md](./TASK.md)

Audit standard:

- `C:\Users\gregs\.codex\Orchestration\Processes\TASK-AUDIT.md`

Auditor worker:

- `019e6c5f-70f6-7af2-80ef-527ab75ba16d`

## Intact Auditor Response

**Verdict**

`revise`

**Primary Findings**

1. The draft requires an invalid `TASK-STATE.json` status. It repeatedly says to record nonterminal `waiting_for_human` in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L75), [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L359), and [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L452), but the shared task-state contract only allows `pending`, `in_progress`, `blocked`, `complete`, and `cancelled` for `status` in [TASK-STATE.md](/c:/Users/gregs/.codex/Orchestration/TASK-STATE.md#L100) and [TASK-STATE.schema.json](/c:/Users/gregs/.codex/Orchestration/TASK-STATE.schema.json#L31). This would let implementation either write schema-invalid state or invent an unscoped schema change.

2. The source-link/provenance bar can still produce misleading GitHub links. The draft validates `github_repo` only as `owner/name` and says to include committed task-doc links when a source commit is available in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L264) and [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L308), but it does not require proving that the selected GitHub repo actually corresponds to the local repo or that the source commit is pushed/reachable there. A local commit existing is not enough to make a durable GitHub link honest.

3. Idempotency is under-specified for existing mappings to closed issues. The draft defines update behavior for mappings that point to open issues in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L314), and stale unreadable mappings block in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L425), but it does not say what happens when `GITHUB-ISSUE.json` points to a readable closed issue. Two implementers could reasonably block, reopen/update, or create a replacement and both claim the draft supports them.

**Blocking Clarifying Questions**

- Should the review-pending state be represented in shared `TASK-STATE.json` using existing schema values, or is this task intended to extend the shared schema? If the latter, that schema/docs change must be in scope.
- When the chosen `github_repo` does not match the local repo remote, or the source commit is not pushed/reachable in that repo, should the endpoint block publication or publish with explicit local-only provenance?

**Strengthening Clarifying Questions**

- Should a closed mapped issue be treated as stale/blocking, reopened/updated, or replaced by a new issue with explicit supersession metadata?
- Should the pilot endpoint require a `task_id` other than `Task-0012` by caller input, or is `Task-0012` acceptable as the default pilot target?

**Required Rewrites**

- Replace `waiting_for_human` as a `TASK-STATE.json` status with schema-valid state wording, or explicitly add shared schema/contract updates to scope.
- Add remote/provenance validation: only create GitHub links when the local repo, selected `github_repo`, source commit, and pushed reachability make those links true.
- Define the closed-issue mapping path so idempotency cannot fork into duplicate or ambiguous behavior.

**Optional Strengthening**

- Require body submission through a temp file or equivalent argv-safe mechanism for large Markdown bodies.
- State whether closed pilot issues can ever be reused, and how the review package records that decision.

**Relevant Rule Or Exemplar**

- Task audit rejects vague mechanism, weak acceptance, proxy closure, and unresolved blocking questions: [TASK-AUDIT.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-AUDIT.md#L505)
- `TASK-STATE.json` status enum: [TASK-STATE.md](/c:/Users/gregs/.codex/Orchestration/TASK-STATE.md#L100)
- Burden-reduction concrete tasks must isolate exact state/artifact changes under proposed changes: [TASK-CREATE.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-CREATE.md#L336)

## Coordinator Disposition

Status: revision required before rerunning clean audit.

Rewrite instructions to writer:

- Do not extend the shared `TASK-STATE.json` schema in this task. Rewrite review-pending state using schema-valid fields, for example `status: in_progress` with `phase`, `current_gate`, `blockers`, `next_expected_artifacts`, and `HANDOFF.md` carrying the human review request. Use `status: blocked` only for real blockers such as missing `gh` auth or invalid repo/provenance.
- Add remote/provenance validation. GitHub links to committed task docs are allowed only when the selected `github_repo` matches a configured or discovered local remote and the referenced source commit is pushed/reachable in that GitHub repo. Otherwise the endpoint must block or publish only explicit local-only provenance; choose one concrete behavior.
- Define the closed mapped issue path. Choose one concrete behavior for a readable closed issue in `GITHUB-ISSUE.json` so duplicate/update/reopen behavior is unambiguous.
- Include body submission through a temp file or equivalent argv-safe mechanism if it fits naturally.
