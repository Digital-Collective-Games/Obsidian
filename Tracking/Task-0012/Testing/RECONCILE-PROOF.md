# Task/GitHub Reconcile Proof

## Purpose

Add a reusable read-only reconcile script that compares repo-local task
documents/state with GitHub task state and prints differences.

The script does not edit local task files and does not edit GitHub.

## Script

- [../../../skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1](../../../skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1)

## Policy Encoded

- Local `TASK-STATE.json` status maps to GitHub issue open/closed state.
- GitHub issue text and local `TASK.md` text are co-owned. Any live title/body
  mismatch is a `text_conflict` difference and blocks GitHub writes for that
  task until resolved.
- GitHub Issue Field `Priority` is authoritative.
- GitHub Issue Field `Queue` is authoritative.
- Codex/local task state is authoritative for GitHub Issue Field
  `Human Needed`.
- `TASK-META.json` records `last_synced_at`, the local time when Codex accepted
  the provider readback as a sync checkpoint. It does not claim to know the
  latest remote edit time.

## Command

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1 -DryRun
```

Dispatch dry run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1 -DryRun -DispatchActions
```

## Outputs

- [TaskGitHubReconcile/RECONCILE-RESULT.json](./TaskGitHubReconcile/RECONCILE-RESULT.json)
- [TaskGitHubReconcile/RECONCILE-DIFFERENCES.md](./TaskGitHubReconcile/RECONCILE-DIFFERENCES.md)
- [TaskGitHubReconcile/RECONCILE-DISPATCH-DRYRUN.md](./TaskGitHubReconcile/RECONCILE-DISPATCH-DRYRUN.md)
- [TaskGitHubReconcile/](./TaskGitHubReconcile/)

The output folder includes rendered local issue bodies and remote issue bodies
for text conflict review. Text conflicts include a generated patch file linked
from the difference details, plus the task metadata `last_synced_at` checkpoint
time when available.

When run with `-DispatchActions`, the script still does not write fixes. It
adds a dry-run dispatch plan showing the exact `gh` command or Codex step
that would be used for each dispatchable difference. If any task has a
conflict, the dispatch plan blocks GitHub writes for that task while still
showing commands for other tasks that have no conflicts.

## Current Summary

The latest reconcile run is clean
([TaskGitHubReconcile/RECONCILE-DIFFERENCES.md](./TaskGitHubReconcile/RECONCILE-DIFFERENCES.md),
[TaskGitHubReconcile/RECONCILE-RESULT.json](./TaskGitHubReconcile/RECONCILE-RESULT.json)):

- local task count: `12`
- remote issue count: `12`
- differences: `0`
- conflicts: `0`
- dispatch dry-run items: `0`
  ([TaskGitHubReconcile/RECONCILE-DISPATCH-DRYRUN.md](./TaskGitHubReconcile/RECONCILE-DISPATCH-DRYRUN.md))

## Resolution History

During the bootstrap/reconcile work the run was not clean and was driven to
zero:

- Issue-state differences: nine terminal local tasks had open GitHub issues.
  After text conflicts cleared, issues `#1`,`#2`,`#3`,`#5`,`#7`,`#8`,`#9` were
  closed `completed` and `#4`,`#6` closed `not planned`. Issues `#10`,`#11`,
  `#12` remain open because their local state is non-terminal. Close commands
  use `--reason completed` for `complete` tasks and `--reason 'not planned'`
  for `cancelled` tasks, with a follow-up `TASK-META.json` refresh from
  readback. No issue with an unresolved `text_conflict` was closed.
- `Task-0003` text conflict: the embedded marker SHA matched local, and the
  visible body difference was traced to a `gh` UTF-8 readback decode defect
  under Windows PowerShell 5.1 (smart quotes in `“total tokens”` mojibaked on
  capture). The GitHub-stored body was already correct; only the script's
  readback/compare was lossy. Fixed by forcing UTF-8 console encoding in the
  obsidian-operator scripts. See [../BUG-0001.md](../BUG-0001.md).
- `Task-0012` text conflict: occurred whenever the local task doc changed after
  the issue body was last published. Resolved by re-syncing issue `#12` from the
  current local body after closeout edits, then re-running reconcile to confirm
  a clean state.
