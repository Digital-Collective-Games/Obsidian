# Bulk Issue Bootstrap Proof

## Purpose

Bootstrap the remaining existing local `TASK.md` files into GitHub Issues with
the one-to-one identity rule:

- `Tracking/Task-0002/TASK.md` maps to issue `#2`
- `Tracking/Task-0003/TASK.md` maps to issue `#3`
- continuing through `Tracking/Task-0012/TASK.md` mapping to issue `#12`

The reusable script stops immediately if GitHub returns an issue number that
does not match the local task folder number.

## Script

- [../../../skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1](../../../skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1)
- Uses [../../../skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1](../../../skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1)
  for rendering, body/title sync, issue-field sync, readback, and
  `TASK-META.json` writes.

## Command

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1 -StartTaskNumber 2
```

## Result

The script created issues `#2` through `#12` in
`Digital-Collective-Games/Obsidian` and wrote per-task `TASK-META.json` files.

Machine-readable latest actual script result:

- [BulkIssueBootstrap/BOOTSTRAP-ISSUES-RESULT.json](./BulkIssueBootstrap/BOOTSTRAP-ISSUES-RESULT.json)

The first real run created issues `#2` through `#12`. After a script output-path
fix, the latest actual run reports `sync_existing` for those same tasks, which
is the expected idempotent state after creation.

Machine-readable idempotency dry-run result:

- [BulkIssueBootstrap/BOOTSTRAP-ISSUES-DRY-RUN.json](./BulkIssueBootstrap/BOOTSTRAP-ISSUES-DRY-RUN.json)

Generated readback/body artifacts:

- [BulkIssueBootstrap/](./BulkIssueBootstrap/)

## Issue Readback

`gh issue list --repo Digital-Collective-Games/Obsidian --state all --limit 20
--json number,title,state,url,labels,projectItems` returned these issue
numbers and titles:

| Issue | Local Task | URL |
| --- | --- | --- |
| `#1` | `Task-0001` | <https://github.com/Digital-Collective-Games/Obsidian/issues/1> |
| `#2` | `Task-0002` | <https://github.com/Digital-Collective-Games/Obsidian/issues/2> |
| `#3` | `Task-0003` | <https://github.com/Digital-Collective-Games/Obsidian/issues/3> |
| `#4` | `Task-0004` | <https://github.com/Digital-Collective-Games/Obsidian/issues/4> |
| `#5` | `Task-0005` | <https://github.com/Digital-Collective-Games/Obsidian/issues/5> |
| `#6` | `Task-0006` | <https://github.com/Digital-Collective-Games/Obsidian/issues/6> |
| `#7` | `Task-0007` | <https://github.com/Digital-Collective-Games/Obsidian/issues/7> |
| `#8` | `Task-0008` | <https://github.com/Digital-Collective-Games/Obsidian/issues/8> |
| `#9` | `Task-0009` | <https://github.com/Digital-Collective-Games/Obsidian/issues/9> |
| `#10` | `Task-0010` | <https://github.com/Digital-Collective-Games/Obsidian/issues/10> |
| `#11` | `Task-0011` | <https://github.com/Digital-Collective-Games/Obsidian/issues/11> |
| `#12` | `Task-0012` | <https://github.com/Digital-Collective-Games/Obsidian/issues/12> |

All readback issues reported:

- `state = OPEN`
- no labels
- no Project items

## Field Readback

For issues `#1` through `#12`, `gh api
/repos/Digital-Collective-Games/Obsidian/issues/<n>/issue-field-values`
reported:

- `Queue = Never`
- `Human Needed = No`
- `Priority = P2`

## Metadata Readback

Local metadata exists for all bootstrapped tasks:

- `Tracking/Task-0001/TASK-META.json`
- `Tracking/Task-0002/TASK-META.json`
- `Tracking/Task-0003/TASK-META.json`
- `Tracking/Task-0004/TASK-META.json`
- `Tracking/Task-0005/TASK-META.json`
- `Tracking/Task-0006/TASK-META.json`
- `Tracking/Task-0007/TASK-META.json`
- `Tracking/Task-0008/TASK-META.json`
- `Tracking/Task-0009/TASK-META.json`
- `Tracking/Task-0010/TASK-META.json`
- `Tracking/Task-0011/TASK-META.json`
- `Tracking/Task-0012/TASK-META.json`

## Idempotency Dry-Run

After the real bootstrap, this command:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1 -StartTaskNumber 2 -DryRun
```

reported one unique action:

```text
dry_run_sync_existing
```

That proves rerunning the script sees the created issues as existing task-bound
issues rather than planning duplicate creates.
