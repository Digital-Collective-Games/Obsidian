---
name: obsidian-operator
description: Operate the CodexDashboard/Obsidian repo task-provider workflow safely. Use when Codex is working in this repo on CODEX-REPO-MANIFEST.json, TASK-META.json, GitHub Issues as accepted tasks, issue field state, skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1, skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1, skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1, task/GitHub text conflicts, or any "drain my tasks" precursor that touches task sync or dispatch state.
---

# Obsidian Operator

## Grounding

Read repo-root `AGENTS.md`, `CODEX-REPO-MANIFEST.json`, and the relevant `Tracking/Task-<id>/TASK.md` before changing task-provider behavior.

Human directives in the current session override subagent output, coordinator/auditor preferences, and older task drafts. Preserve older artifacts as history, but do not treat them as active scope when the human has narrowed or corrected the task.

## Provider Contract

Use GitHub Issues through `gh` as the accepted-task provider for this repo.

- Provider repo: `Digital-Collective-Games/Obsidian`.
- Local task id equals GitHub issue number: issue `#12` maps to `Tracking/Task-0012/`.
- Do not create mapping layers where `Task-0012` points to an unrelated issue id.
- For new accepted tasks, TaskCreate must create the GitHub issue first, then materialize `Tracking/Task-<issue-number>/TASK.md`.
- Pull requests can create holes in local task numbering. Do not reuse rejected proposal ids as accepted task ids.
- Proposal workflow is separate from accepted tasks and belongs to the configured proposal provider, not this repo's accepted-task issue space.

## Issue Type (required for the Fields panel)

GitHub's org-level **issue fields** (`Priority`, `Queue`, `Human Needed`) only
render in an issue's right-hand **Fields** panel when the issue has an **issue
type**. A typeless issue shows "No fields configured for issues without a type"
in the UI, even though the field *values* are still stored and returned by
`/repos/<repo>/issues/<n>/issue-field-values`. Introducing org issue types
(`Task`/`Bug`/`Feature`) is what flipped the Fields display to type-gated, so
issues created before/without a type lose the visible fields until typed.

Therefore every accepted-task issue must carry an issue type. The default is
`Task`; the org also defines `Bug` and `Feature`. The scripts set it automatically:

- `Sync-TaskToGitHubIssue.ps1` and `Bootstrap-TaskGitHubIssues.ps1` take
  `-IssueType` (default `Task`). The name must exist in `/orgs/<org>/issue-types`;
  the sync validates it, sets it on the issue (REST update-issue `type`), and
  asserts it on readback.
- Pass `-IssueType Bug` / `-IssueType Feature` for a bug/feature accepted issue.

Setting the field *values* alone does NOT make them visible; the issue type is
what unlocks the Fields panel.

## Authority Model

Keep each state owner narrow.

- Local `TASK.md` owns rich task truth: scope, goals, acceptance, non-goals, research, proof, audits, and pass history.
- GitHub Issue owns queryable accepted-task identity and shallow state: issue number, URL, title, open/closed state, and discoverability through `gh`.
- GitHub Issue Fields own `Priority` and `Queue`.
- Codex/local task state owns `Human Needed`.
- Local `TASK-STATE.json` status maps to GitHub issue open/closed state.
- Labels and GitHub Projects are not the queue, priority, human-needed, or identity surface for accepted tasks.

## TASK-META.json

Keep `TASK-META.json` small provider binding metadata.

Required fields:

```json
{
  "schema_version": 1,
  "provider_kind": "github_issues",
  "provider_repo": "Digital-Collective-Games/Obsidian",
  "issue_number": 12,
  "issue_url": "https://github.com/Digital-Collective-Games/Obsidian/issues/12",
  "last_synced_at": "2026-05-28T17:20:32.604105-04:00"
}
```

`last_synced_at` means "Codex accepted this local/GitHub readback as a sync checkpoint at this local time." It is not GitHub's latest remote edit time and must not be named `issue_updated_at` or `last_checked_at`.

Do not use timestamp comparison as overwrite permission. Safe writes require live GitHub title/body readback and conflict checks in the same operation.

## Script Use

Use the bundled Obsidian operator scripts as the normalized surfaces.

- `skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1`
  - Sync exactly one local `TASK.md` to its same-number GitHub issue.
  - Derive issue number from `Tracking/Task-<id>/TASK.md`.
  - Check `gh auth status -h github.com`.
  - Fetch live issue title/body before writing.
  - Refuse mismatched task ids, missing markers, or live text differences unless the operator has explicitly decided to repair or overwrite.
  - Write `TASK-META.json` only after successful GitHub readback.
- `skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1`
  - Bootstrap existing tasks in order.
  - Stop immediately if GitHub returns a different issue number than the local task number.
- `skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1`
  - Report differences between local tasks and live GitHub issues.
  - Treat text title/body mismatches as `text_conflict`.
  - With `-DispatchActions`, show commands/steps only. Keep it dry-run unless the human explicitly approves actual execution.

Preferred checks before a real write:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1 -DryRun -DispatchActions
```

For one task preview:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1 -TaskPath Tracking/Task-0001/TASK.md -DryRun
```

## Reconcile Semantics

Use precise terminology.

- A reconciliation run shows `differences`.
- A text title/body mismatch is a `text_conflict`.
- A conflict blocks GitHub writes for that issue.
- Clean tasks without conflicts may still have dispatchable differences.
- Do not say every difference is an action.
- Do not touch a GitHub issue where there is an unresolved text conflict.

When dispatch planning shows a clean issue-state difference, the intended command may be a `gh issue close` or `gh issue reopen`. When the same task has `text_conflict`, the command must be blocked for that task until the conflict is resolved.

## Text Conflict Resolution

Do not let a script or generic merge tool make semantic task-doc decisions.

The script preserves evidence. The Codex operator must inspect:

- current rendered local issue body
- current live GitHub issue body
- generated text diff
- local `TASK.md`
- `TASK-META.json` `last_synced_at`
- GitHub `userContentEdits` through GraphQL when edit history would clarify who changed the issue body

Resolution choices:

- **Local wins**: after inspection, push the local rendered body to GitHub. Use force/repair flags only because the operator has made this explicit decision.
- **Remote has useful prose**: edit local `TASK.md` to incorporate the GitHub text, then sync from local to GitHub.
- **Both have useful prose**: manually merge into local `TASK.md`, then sync from local to GitHub.
- **Unclear**: ask the human a narrow question with links to the conflicting blocks.

After successful write/readback, refresh `TASK-META.json` and advance `last_synced_at`.

## GitHub Edit History

GitHub GraphQL exposes issue edit history with `userContentEdits`.

Use it as review evidence, not as the canonical version store. GitHub can redact/delete edit history, and the API returns diffs rather than a clean local baseline. Keep local checkpoint metadata and live readback checks.

Useful query shape:

```powershell
gh api graphql `
  -f owner=Digital-Collective-Games `
  -f name=Obsidian `
  -F number=12 `
  -f query='query($owner:String!, $name:String!, $number:Int!) { repository(owner:$owner, name:$name) { issue(number:$number) { number title lastEditedAt editor { login } userContentEdits(first:20) { totalCount nodes { editedAt editor { login } diff } } } } }'
```

## Guardrails

- Prefer dry runs and readbacks before writes.
- Do not use labels or Projects for accepted-task queue, priority, human-needed, or identity state.
- Do not broaden a task from follow-on coordinator, worktree, concurrency, dashboard UI, or queue-draining ideas unless the human explicitly rescope it.
- Do not create backend orchestration endpoints for this workflow unless a later task explicitly asks for them.
- Keep proof and conflict artifacts under the owning `Tracking/Task-<id>/Testing/` folder.
- Link issue URLs and local artifacts when reporting results.
