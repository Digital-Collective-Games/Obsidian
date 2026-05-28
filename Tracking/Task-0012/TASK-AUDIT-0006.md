# Task Audit 0006

Audit date: 2026-05-27

Audit path: TaskCreate clean-context auditor-worker

Reviewed draft:

- [TASK-DRAFT-0006-BEFORE-AUDIT.md](./TASK-DRAFT-0006-BEFORE-AUDIT.md)

Current draft path:

- [TASK.md](./TASK.md)

Audit standard:

- `C:\Users\gregs\.codex\Orchestration\Processes\TASK-AUDIT.md`

Auditor worker:

- `019e6c73-325c-7dc1-8d61-ceb55cbac77d`

## Intact Auditor Response

**Verdict**

`revise`

**Primary Findings**

1. Explicit `issue_number` handling can still target the wrong issue. The draft says a supplied open issue is viewed, updated, and mapped automatically ([draft](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L386), [acceptance](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L514)), but it does not require the existing issue to already contain the selected task's hidden marker, nor require an explicit "attach/overwrite this existing issue" confirmation. That leaves a proxy success path where the endpoint overwrites an unrelated open issue and still claims the pilot published.

2. Partial-success recovery is still thinner than the duplicate-prevention claim. The draft records candidate issue number/URL when known ([draft](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L357)), but the readback-failure acceptance path only requires marker-search-based recovery on rerun ([draft](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L532)). If `gh issue create` returns an issue URL/number but follow-up readback fails, the recovery rule should require using that concrete candidate identity before falling back to marker search, or explain why marker search alone is authoritative enough to satisfy the no-duplicate falsifier ([draft](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L588)).

**Blocking Clarifying Questions**

- For a supplied `issue_number`, must the existing issue already contain this task's hidden stable marker before the backend edits it?
- If not, what explicit request field or human confirmation makes "attach this local task to this existing open issue" safe and durable?
- When create/edit may have succeeded and the recovery artifact contains an issue number or URL, must rerun recovery view that candidate issue before any marker-search or create path?

**Strengthening Clarifying Questions**

- Should the recovery artifact distinguish `remote_identity_known` from `remote_identity_unknown` so reruns have deterministic recovery order?
- Should fake-runner tests cover "explicit open issue without matching marker blocks unless attach is explicitly requested"?

**Required Rewrites**

- Add explicit issue-target validation for supplied `issue_number`: marker match, explicit attach mode, or blocked result.
- Add `What Does Not Count` and falsifier coverage for overwriting or mapping an unrelated open issue.
- Tighten partial-success rerun order: candidate issue identity from recovery artifact first when known, marker search second, create only after both prove no existing target.

**Optional Strengthening**

- Add response fields such as `existing_issue_marker_status` and `recovery_identity_status`.
- Include prior issue title/body marker status in blocker or recovery artifacts for explicit issue updates.

**Relevant Rule Or Exemplar**

- [Task audit proxy-closure rule](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-AUDIT.md#L252): reject drafts that can close through a weaker proxy.
- [Task-create concrete implementation bar](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-CREATE.md#L181): concrete tasks must name exact enforcement boundaries and weak closure blockers.

## Coordinator Disposition

Status: revision required before rerunning clean audit.

Rewrite instructions to writer:

- For a supplied `issue_number`, require the existing issue to already contain the selected task's hidden stable marker before editing it. Do not add a broad attach/overwrite override to this first task unless it is fully specified. Prefer fail-closed with a blocker artifact when the marker is absent.
- Add explicit `What Does Not Count` and falsifier language that overwriting, mapping, or publishing into an unrelated open issue does not count.
- Tighten partial-success recovery order: if the recovery artifact has a candidate issue number or URL, rerun must view that candidate first with `--repo <github_repo>`; marker search is second; create is allowed only after candidate and marker search prove no existing target.
- Add response/artifact fields if helpful, such as `existing_issue_marker_status`, `recovery_identity_status`, and prior issue title/body marker status in blocker or recovery artifacts.
- Add fake-runner tests for explicit open issue without matching marker blocking, and for known-candidate recovery before marker search/create.
