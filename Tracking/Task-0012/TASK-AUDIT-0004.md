# Task Audit 0004

Audit date: 2026-05-27

Audit path: TaskCreate auditor-worker attempt

Conformance: non-conformant. The auditor reported `non_blind_contaminated`, so this artifact is preserved as useful audit history but does not clear the independent TaskCreate audit gate.

Reviewed draft:

- [TASK-DRAFT-0004-BEFORE-AUDIT.md](./TASK-DRAFT-0004-BEFORE-AUDIT.md)

Current draft path:

- [TASK.md](./TASK.md)

Audit standard:

- `C:\Users\gregs\.codex\Orchestration\Processes\TASK-AUDIT.md`

Auditor worker:

- `019e6c67-12b7-7890-81f0-85740b9c69fa`

## Intact Auditor Response

**Verdict**

`revise`

Blindness note: `non_blind_contaminated`. The active conversation included repo/session instructions outside the three allowed launch parameters. I did not rely on that extra context for the findings, but a strict clean-context TaskCreate audit should treat this run as contaminated.

**Primary Findings**

1. The selected GitHub repo is not enforced at the `gh` command boundary. The draft defines `github_repo` as a required `owner/name` request field in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L293), but the concrete `gh issue create/edit/view` commands listed in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L319) do not require `--repo <github_repo>` or an equivalent explicit target. That leaves a proxy path where validation records one repo while `gh` defaults to the current checkout remote and publishes elsewhere.

2. Explicit `issue_number` update behavior is still ambiguous for closed issues. The draft blocks a closed issue only when it comes from `GITHUB-ISSUE.json` in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L353), but says an explicitly supplied `issue_number` should be updated in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L356). Two implementers could either edit a closed explicit issue or block it and both claim support from the draft.

**Blocking Clarifying Questions**

- Must every `gh issue create`, `gh issue edit`, and `gh issue view` invocation pass the selected `github_repo` explicitly via `--repo`, regardless of current working directory or local remotes?
- If `issue_number` is supplied and `gh issue view` shows the issue is closed, should the endpoint block, edit the closed issue, or require explicit human override?

**Strengthening Clarifying Questions**

- Should the review package record the actual GitHub authenticated identity/host used for publication, so the human can confirm the pilot was published under the expected account?

**Required Rewrites**

- Add an acceptance criterion and proposed-change bullet requiring all issue read/write commands to target the selected `github_repo` explicitly.
- Define closed-issue behavior for both mapping-derived and explicitly supplied issue numbers.

**Optional Strengthening**

- Add a proof capture showing the exact argv used by the fake `gh` runner includes `--repo <github_repo>` for create, edit, and view.

**Relevant Rule Or Exemplar**

- Task audit rejects drafts where mechanism blur lets two implementers build materially different behavior and both claim done: [TASK-AUDIT.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-AUDIT.md#L505).

## Coordinator Disposition

Status: non-conformant audit attempt with useful revision findings.

Rewrite instructions to writer:

- Require every `gh issue create`, `gh issue edit`, and `gh issue view` invocation to target the selected `github_repo` explicitly via `--repo <github_repo>` or an equivalent explicit repo argument. Do not rely on current working directory defaults.
- Define closed-issue behavior for both mapping-derived and explicitly supplied `issue_number` paths. Choose one concrete behavior. Prefer fail-closed unless the task makes an explicit human override mechanism in scope.
- Add fake-runner proof expectations showing exact argv includes `--repo <github_repo>` for create, edit, and view.
- If natural, require the review package to record authenticated GitHub identity/host used for the publication attempt.
