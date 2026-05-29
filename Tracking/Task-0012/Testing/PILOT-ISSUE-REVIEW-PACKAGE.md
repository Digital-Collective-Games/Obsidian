# Pilot Issue Review Package

## Gate

Human inspection of the single GitHub-backed task representation.

Status: accepted by human on 2026-05-28. The approved GitHub issue surface is:
org-owned repo, no workflow labels, visible GitHub Issue Fields for `Queue`,
`Priority`, and `Human Needed`, and no attached Project.

Status update: this review package is now pilot evidence, not final acceptance
of the steady-state task identity convention. Human discussion after issue
creation selected a repo registry model where accepted tasks and task proposals
are provider bindings, and future accepted task folder numbers come from GitHub
issue creation at TaskCreate promotion/materialization time.

## Issue

- URL: <https://github.com/Digital-Collective-Games/Obsidian/issues/1>
- Title: `Task-0001: Codex token-velocity dashboard with a hotkey-first overlay.`
- Repo: `Digital-Collective-Games/Obsidian`
- State: `OPEN`
- Labels: none; accepted-task identity is implied by the provider repo and the
  issue number, not identity labels.
- Issue fields:
  - `Queue = Never`
  - `Priority = P2`
  - `Human Needed = No`
- Project items: none; the temporary user Project is no longer attached to this
  issue after the repo transfer.
- Local task: [../../Task-0001/TASK.md](../../Task-0001/TASK.md)

## Local Artifacts

- Body preview: [PILOT-ISSUE-BODY.md](./PILOT-ISSUE-BODY.md)
- Raw readback: [PILOT-ISSUE-VIEW.json](./PILOT-ISSUE-VIEW.json)
- Chrome inspection: [CHROME-ISSUE-INSPECTION.md](./CHROME-ISSUE-INSPECTION.md)
- Issue fields proof: [ISSUE-FIELDS-PROOF.md](./ISSUE-FIELDS-PROOF.md)
- Chrome issue fields inspection:
  [CHROME-ISSUE-FIELDS-INSPECTION.md](./CHROME-ISSUE-FIELDS-INSPECTION.md)
- Superseded Project deletion proof:
  [PROJECT-DELETION-PROOF.md](./PROJECT-DELETION-PROOF.md)
- Bulk issue bootstrap proof:
  [BULK-ISSUE-BOOTSTRAP-PROOF.md](./BULK-ISSUE-BOOTSTRAP-PROOF.md)
- Task/GitHub reconcile proof:
  [RECONCILE-PROOF.md](./RECONCILE-PROOF.md)
- Superseded Project queue proof: [PROJECT-QUEUE-PROOF.md](./PROJECT-QUEUE-PROOF.md)
- Pilot task metadata: [../../Task-0001/TASK-META.json](../../Task-0001/TASK-META.json)
- Task metadata example: [TASK-META-EXAMPLE.json](./TASK-META-EXAMPLE.json)
- Remote drift block proof: [REMOTE-DRIFT-BLOCK-PROOF.md](./REMOTE-DRIFT-BLOCK-PROOF.md)
- Create proof: [PILOT-ISSUE-CREATE.txt](./PILOT-ISSUE-CREATE.txt)
- Auth proof: [PILOT-ISSUE-AUTH-STATUS.txt](./PILOT-ISSUE-AUTH-STATUS.txt)
- Owning pilot task: [../TASK.md](../TASK.md)
- Sync script: [../../../skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1](../../../skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1)

## Readback Summary

`gh issue view --repo Digital-Collective-Games/Obsidian 1 --json
number,title,body,state,labels,projectItems,url` and
`gh api /repos/Digital-Collective-Games/Obsidian/issues/1/issue-field-values`
returned:

- number: `1`
- URL: `https://github.com/Digital-Collective-Games/Obsidian/issues/1`
- state: `OPEN`
- labels: none
- project items: none
- queue: `Never`
- priority: `P2`
- human needed: `No`
- updated at: `2026-05-28T20:37:56Z`
- body includes the stable marker:
  `task-sync: repo=CodexDashboard; task_id=Task-0001; task_path=Tracking/Task-0001/TASK.md`

## Drift And Duplicate Check

- The original pilot body incorrectly mapped issue `#1` to `Task-0012`.
  That violated the issue-number-to-local-task-folder convention and has been
  repaired.
- `skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1` now derives the issue number from the
  local task folder and refuses mismatched embedded task ids.
- On rerun, the script compares the live GitHub title/body readback with the
  rendered local task and stops before overwrite if text differs without an
  explicit merge, overwrite, or repair path.
- No existing Task-0001 task metadata existed before the repaired sync.
- `gh issue list --repo Digital-Collective-Games/Obsidian --state all --search
  "Task-0001"` resolves to issue `#1` after repair.
- The Task-0001 metadata now points reruns at issue `#1` instead of creating
  duplicates or allowing Task-0012 to claim issue `#1`.
- The sync script now sets and verifies org issue fields by name on each sync
  run: `Queue`, `Priority`, and `Human Needed`.
- The superseded `gregsemple2003` user Project was deleted after broad `gh`
  auth refresh. `gh project list --owner gregsemple2003` now returns no
  Projects.

## Source Metadata

- source commit: `75177b6dee23399358ee66676791fb41dc01d51e`
- local task SHA-256:
  `9800D5DF5B67ACB2AE8C39CEE6E207A50323BB6F2313F195BEBEA3C4B7378E73`
- sync status: `synced_by_script`
- checked at: `2026-05-28T16:38:00.2143291-04:00`

## Approval Question

Do you accept this pilot as proof that `gh` auth, issue creation, and readback
work, while revising the final steady-state convention to the repo registry and
TaskCreate provider-interface model recorded in [../TASK.md](../TASK.md)?

Human answer on 2026-05-28: accepted. The issue shape is good for GitHub-backed
task state.
