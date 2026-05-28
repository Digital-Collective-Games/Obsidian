# Task Audit 0005

Audit date: 2026-05-27

Audit path: TaskCreate clean-context auditor-worker

Reviewed draft:

- [TASK-DRAFT-0005-BEFORE-AUDIT.md](./TASK-DRAFT-0005-BEFORE-AUDIT.md)

Current draft path:

- [TASK.md](./TASK.md)

Audit standard:

- `C:\Users\gregs\.codex\Orchestration\Processes\TASK-AUDIT.md`

Auditor worker:

- `019e6c6c-c575-7a32-93e0-de9bea656f54`

## Intact Auditor Response

**Verdict**

`revise`

**Primary Findings**

1. Idempotency is still under-specified for the no-mapping and partial-success path. The draft requires a hidden stable marker in the issue body, but the create/update decision says "if neither exists, create one issue" without requiring a marker search or other duplicate recovery when [GITHUB-ISSUE.json](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L367) is missing. If `gh issue create` succeeds but `gh issue view` fails or returns malformed JSON, the draft also requires no mapping write, so a rerun can create a duplicate while still following [the proposed flow](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L360). That conflicts with the falsifier that reruns must not create duplicates in [the draft](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L543).

2. The review-package scope is slightly inconsistent for blocked authentication or blocked publication. The draft says [PILOT-ISSUE-REVIEW-PACKAGE.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L400) is created after real publish or update, but also says it can include an auth-unavailable note when publication was blocked before identity was available in [the same package definition](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L406). A blocked auth result should not look like a pilot-issue review package for human acceptance, because no pilot issue shape exists to accept.

**Blocking Clarifying Questions**

- When no mapping exists, must the backend search the selected GitHub repo for the hidden stable marker before creating a new issue?
- If create/edit appears to succeed but readback fails before a trustworthy mapping can be written, what durable recovery artifact or marker-search rule prevents the next run from duplicating the remote issue?

**Strengthening Clarifying Questions**

- Should blocked auth/publication create a separate failure-proof artifact rather than [PILOT-ISSUE-REVIEW-PACKAGE.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L400)?
- Should marker search include closed issues, or only open issues, before deciding whether a missing mapping means "create"?

**Required Rewrites**

- Add an explicit duplicate-recovery mechanism for the missing-mapping path, likely using the hidden stable marker before any create.
- Define the partial-success path where remote mutation may have happened but readback/mapping failed.
- Clarify that the pilot review package exists only after a readable real issue is published or updated; blocked auth/publication should instead record blocker proof in task-owned testing artifacts, [TASK-STATE.json](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L411), and [HANDOFF.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L414).

**Optional Strengthening**

- Add fake-runner tests for create-succeeds/readback-fails followed by rerun, proving no duplicate issue is created.
- Add acceptance criteria for stable-marker search argv or API behavior.

**Relevant Rule Or Exemplar**

- [TASK-AUDIT.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-AUDIT.md#L505) rejects drafts where mechanism blur lets two implementers build materially different behavior and both claim done.
- [TASK-CREATE.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-CREATE.md#L242) requires concrete implementation tasks to name exact mechanisms and fields.

## Coordinator Disposition

Status: revision required before rerunning clean audit.

Rewrite instructions to writer:

- Add explicit duplicate recovery for missing mappings. Before any `gh issue create`, the backend must search the selected repo for the task's hidden stable marker and reuse or block on the discovered issue according to its state.
- Define the partial-success path where `gh issue create` or `gh issue edit` may have mutated GitHub but readback or mapping write failed. Require durable recovery proof and marker-search-based rerun behavior that prevents duplicates.
- Clarify that `PILOT-ISSUE-REVIEW-PACKAGE.md` exists only after a readable real issue is published or updated. Blocked authentication/publication must instead create a separate task-owned blocker/proof artifact plus `TASK-STATE.json` and `HANDOFF.md` updates.
- Add fake-runner tests for create-succeeds/readback-fails followed by rerun, proving no duplicate issue is created.
