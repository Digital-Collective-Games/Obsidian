# Task Audit 0001

Audit date: 2026-05-27

Audit path: TaskCreate clean-context auditor-worker

Reviewed draft:

- [TASK-DRAFT-0001-BEFORE-AUDIT.md](./TASK-DRAFT-0001-BEFORE-AUDIT.md)

Current draft path:

- [TASK.md](./TASK.md)

Audit standard:

- `C:\Users\gregs\.codex\Orchestration\Processes\TASK-AUDIT.md`

Auditor worker:

- `019e6c51-dd47-7ef3-90ba-f2e2eece6946`

## Intact Auditor Response

Verdict: `revise`

Blindness: no `non_blind_contaminated` issue detected. I used the supplied auditor docs, the draft, and durable files cited by the draft or required by the audit standard. I did not edit files.

**Primary Findings**
1. The new mapping artifact is not reconciled with repo data-handling rules. The draft adds per-task `GITHUB-ISSUE.json` state in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L124) and requires writing it after publish in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L316), but repo rules say tasks that change task state storage must update data-handling/restore expectations in [DATA-HANDLING.md](/c:/Agent/CodexDashboard/DATA-HANDLING.md#L187). This is a real durable-state gap, not copy polish.

2. The human inspection gate is still too easy to bypass. The draft's purpose is to publish one issue and decide whether the issue shape is good enough before scale-up in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L16), but the acceptance criteria mainly prove endpoint behavior and artifacts. Add an explicit closeout rule: successful publish/update is not enough unless the task records either a human inspection decision or a clear waiting-for-human review gate.

3. The fail-closed contract is narrower than the `gh` execution surface. The draft covers unauthenticated `gh` in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L301), but the backend shells out to `gh` in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L213). Add blocked behavior and tests for missing/unusable `gh`, timeout, and non-auth command failure so the endpoint cannot crash, hang, or produce ambiguous publication state.

**Blocking Clarifying Questions**
- Should Task-0012 closure require actual human acceptance of the pilot issue shape, or only a review package that makes the issue ready for human inspection?
- Should `GITHUB-ISSUE.json` be treated as task state storage requiring a repo data-handling update, or as task-owned proof only? The current draft uses it as durable mapping state, so the safer answer is data-handling update required.

**Strengthening Clarifying Questions**
- Which task should be the default pilot if the caller does not choose one?
- Should failed `gh issue view` for an existing mapping block update, recreate, or return a stale-mapping state?

**Required Rewrites**
- Add a data-handling/restore acceptance criterion for the GitHub mapping artifact.
- Add an explicit human-inspection gate or waiting state to acceptance criteria.
- Expand auth-block criteria into a broader `gh` failure contract with fake-runner tests.
- State that rejected issue shape keeps bulk publication blocked.

**Optional Strengthening**
- Require safe argv-style command invocation, not shell-composed strings.
- Validate labels and `issue_number` inputs, not only `github_repo`.
- Distinguish committed GitHub links from local-only paths when `source_commit` is unavailable.

**Relevant Rule Or Exemplar**
- Weak acceptance and proxy closure checks: [TASK-AUDIT.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-AUDIT.md#L315), [TASK-AUDIT.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-AUDIT.md#L328)
- Human-acceptance drift: [TASK-AUDIT.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-AUDIT.md#L334)
- Data-handling closeout rule: [DATA-HANDLING.md](/c:/Agent/CodexDashboard/DATA-HANDLING.md#L187)
- First-slice research acceptance: [2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md#L435)

## Coordinator Disposition

Status: revision required before rerunning clean audit.

Rewrite instructions to writer:

- Treat `GITHUB-ISSUE.json` as durable task mapping state, not throwaway proof. Add data-handling and restore requirements to the task scope and acceptance criteria.
- Add a clear human-inspection gate. Successful publish/update alone must not close the task unless the human accepts the pilot issue shape; otherwise the task must end in an explicit waiting-for-human review state with a review package.
- Expand the `gh` failure contract beyond auth failure to include missing/unusable `gh`, timeout, and non-auth command failures. Require fake-runner or equivalent tests.
- State that a rejected pilot issue shape blocks bulk `TASK.md` publication and central queue-draining follow-ons.
- Consider optional strengthening where it fits without expanding scope: safe argv-style command invocation, label and issue-number validation, and clear behavior when source commit is unavailable.
