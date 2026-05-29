# Live Text Difference Block Proof

## Purpose

Prove `skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1` reads the live GitHub issue
title/body before writing and blocks when that live text differs from the
rendered local task.

`TASK-META.json` is provider binding/readback metadata. Its `last_synced_at`
field records when Codex accepted a local/GitHub sync checkpoint; it does not
claim to know the latest remote edit time.

## Command

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1 `
  -TaskPath Tracking/Task-0012/TASK.md `
  -OutputBodyPath Tracking/Task-0012/Testing/LIVE-TEXT-DIFF-BLOCK-BODY.md `
  -MetadataPath Tracking/Task-0012/TASK-META.json
```

## Result

Exit code: `1`

Blocked as expected:

```text
Live GitHub issue #12 title/body differs from the rendered local task. Run reconcile and merge/review the diff, or use -ForceRemoteOverwrite after deciding the local render should replace the remote issue text.
```

The command rendered the local body preview but stopped before any GitHub write.
