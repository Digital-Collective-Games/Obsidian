# Task-0012 GitHub Handoff

## Purpose

This handoff is for the next Codex session continuing Task-0012 GitHub-backed
task-state work.

The current repo state is ahead of the active `TASK.md` and `PLAN.md` wording.
Do not treat the old "one issue only" wording as the full current truth without
also reading this handoff and the reconcile artifacts.

## Current State

Provider registry exists at repo root:

- [`../../CODEX-REPO-MANIFEST.json`](../../CODEX-REPO-MANIFEST.json)
- accepted-task provider: `Digital-Collective-Games/Obsidian`
- task-proposal provider: `Digital-Collective-Games/ObsidianProposals`
- source-control provider: `Digital-Collective-Games/Obsidian`
- default agent user: `gregsemple2003`

Repo-local Obsidian operator skill exists:

- [`../../skills/obsidian-operator/SKILL.md`](../../skills/obsidian-operator/SKILL.md)
- [`../../skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1`](../../skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1)
- [`../../skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1`](../../skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1)
- [`../../skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1`](../../skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1)

GitHub issue identity convention is active for this repo:

- GitHub issue `#N` maps to local `Tracking/Task-NNNN/`.
- `Tracking/Task-0012/` maps to GitHub issue `#12`, not issue `#1`.
- New accepted tasks should create a GitHub issue first, then materialize
  `Tracking/Task-<issue-number>/TASK.md`.
- Pull requests may create numbering holes. Rejected proposal ids should not be
  reused as accepted task ids.

## What Was Done

The original one-issue pilot was completed against `Task-0001` / issue `#1`.
The human then approved continuing beyond the one-issue pilot for this repo.

Existing local tasks were bootstrapped into GitHub Issues:

- `Task-0001` through `Task-0012` now have matching GitHub issues `#1` through
  `#12` in `Digital-Collective-Games/Obsidian`.
- Each task has a local `TASK-META.json` provider binding after successful
  sync/readback.
- Issues use GitHub Issue Fields for `Queue`, `Priority`, and `Human Needed`.
- Labels are not the task identity, queue, priority, or human-needed surface.

Issue state differences were corrected:

- Issues `#1`, `#2`, `#3`, `#5`, `#7`, `#8`, and `#9` were closed as
  `completed`.
- Issues `#4` and `#6` were closed as `not planned`.
- Issues `#10`, `#11`, and `#12` remain open because their local task state is
  non-terminal.

Final reconcile result is clean:

- [`Testing/TaskGitHubReconcile/RECONCILE-DIFFERENCES.md`](./Testing/TaskGitHubReconcile/RECONCILE-DIFFERENCES.md)
- [`Testing/TaskGitHubReconcile/RECONCILE-RESULT.json`](./Testing/TaskGitHubReconcile/RECONCILE-RESULT.json)
- [`Testing/TaskGitHubReconcile/RECONCILE-DISPATCH-DRYRUN.md`](./Testing/TaskGitHubReconcile/RECONCILE-DISPATCH-DRYRUN.md)

At the time of this handoff, final reconcile reports:

- `difference_count: 0`
- `conflict_count: 0`
- dispatch dry-run `item_count: 0`

## Important Bug Fixed

During conflict review, `Task-0003` still showed a `text_conflict` after a push.
The actual cause was Windows PowerShell reading UTF-8 Markdown without an
explicit encoding. Smart quotes in `Tracking/Task-0003/TASK.md` were rendered as
mojibake in the GitHub body.

The fix was to add `-Encoding UTF8` to script reads in:

- `Sync-TaskToGitHubIssue.ps1`
- `Bootstrap-TaskGitHubIssues.ps1`
- `Reconcile-TaskGitHubState.ps1`

After the encoding fix, `Task-0003` was repushed and reconcile became clean.
Keep this as durable script behavior. Do not remove the explicit UTF-8 reads.

### Second UTF-8 seam found at closeout (2026-05-28) — see BUG-0001

A closeout reconcile re-run again reported a `Task-0003` `text_conflict`, even
though the earlier handoff claimed a clean reconcile. The remaining seam was not
the local file read but the native `gh` stdout decode: under Windows PowerShell
5.1, `[Console]::OutputEncoding` defaults to a non-UTF-8 code page, so `gh`
JSON output mojibaked smart quotes on capture. The GitHub-stored body was
already correct; only the script readback/compare was lossy, which produced a
false conflict.

Fix: force UTF-8 console input/output encoding at the top of all three
obsidian-operator scripts. After the fix, reconcile under `powershell
-NoProfile` reports `difference_count: 0` and `conflict_count: 0`. Full
narrowing path and verification: [BUG-0001.md](./BUG-0001.md). Do not remove the
UTF-8 console-encoding guard.

## Authority Model

Current agreed authority model:

- Local `TASK.md` owns rich task truth: scope, goals, acceptance, non-goals,
  research, proof, audits, and pass history.
- GitHub Issue owns queryable accepted-task identity and shallow state: issue
  number, URL, title, open/closed state, and discoverability through `gh`.
- GitHub Issue Fields own `Priority` and `Queue`.
- Codex/local task state owns `Human Needed`.
- Local `TASK-STATE.json` terminal status maps to GitHub open/closed issue
  state.
- `TASK-META.json` is a small provider binding and sync checkpoint, not a rich
  task state file.

`TASK-META.json` should stay small:

```json
{
  "schema_version": 1,
  "provider_kind": "github_issues",
  "provider_repo": "Digital-Collective-Games/Obsidian",
  "issue_number": 12,
  "issue_url": "https://github.com/Digital-Collective-Games/Obsidian/issues/12",
  "last_synced_at": "2026-05-28T21:36:41.3333046-04:00"
}
```

`last_synced_at` means Codex accepted a local/GitHub readback as a sync
checkpoint at that local time. It is not GitHub's latest remote edit time.

## Commands Worth Reusing

Run reconcile before and after task-provider writes:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1 -DryRun -DispatchActions
```

Preview one task sync:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1 -TaskPath Tracking/Task-0012/TASK.md -DryRun
```

Push one task after conflict review explicitly decides local wins:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1 -TaskPath Tracking/Task-0012/TASK.md -ForceRemoteOverwrite
```

Do not use `-ForceRemoteOverwrite` as a convenience flag. Use it only after
reviewing the generated diff and deciding that local rendered text should win.

## Current GitHub Links

- Task-0001: <https://github.com/Digital-Collective-Games/Obsidian/issues/1>
- Task-0002: <https://github.com/Digital-Collective-Games/Obsidian/issues/2>
- Task-0003: <https://github.com/Digital-Collective-Games/Obsidian/issues/3>
- Task-0004: <https://github.com/Digital-Collective-Games/Obsidian/issues/4>
- Task-0005: <https://github.com/Digital-Collective-Games/Obsidian/issues/5>
- Task-0006: <https://github.com/Digital-Collective-Games/Obsidian/issues/6>
- Task-0007: <https://github.com/Digital-Collective-Games/Obsidian/issues/7>
- Task-0008: <https://github.com/Digital-Collective-Games/Obsidian/issues/8>
- Task-0009: <https://github.com/Digital-Collective-Games/Obsidian/issues/9>
- Task-0010: <https://github.com/Digital-Collective-Games/Obsidian/issues/10>
- Task-0011: <https://github.com/Digital-Collective-Games/Obsidian/issues/11>
- Task-0012: <https://github.com/Digital-Collective-Games/Obsidian/issues/12>

## Evidence Artifacts

Read these before changing task scope or declaring closeout:

- [`Testing/PILOT-ISSUE-REVIEW-PACKAGE.md`](./Testing/PILOT-ISSUE-REVIEW-PACKAGE.md)
- [`Testing/BULK-ISSUE-BOOTSTRAP-PROOF.md`](./Testing/BULK-ISSUE-BOOTSTRAP-PROOF.md)
- [`Testing/RECONCILE-PROOF.md`](./Testing/RECONCILE-PROOF.md)
- [`Testing/REMOTE-DRIFT-BLOCK-PROOF.md`](./Testing/REMOTE-DRIFT-BLOCK-PROOF.md)
- [`Testing/ISSUE-FIELDS-PROOF.md`](./Testing/ISSUE-FIELDS-PROOF.md)
- [`Testing/TaskGitHubReconcile/RECONCILE-DIFFERENCES.md`](./Testing/TaskGitHubReconcile/RECONCILE-DIFFERENCES.md)
- [`Testing/TaskGitHubReconcile/RECONCILE-DISPATCH-DRYRUN.md`](./Testing/TaskGitHubReconcile/RECONCILE-DISPATCH-DRYRUN.md)

## Next Steps

Closeout progress (2026-05-28, this session):

1. Done. `TASK.md` (Scope Update note, Non-Goals cross-repo split, Implementation
   Home, Additional Acceptance block) and `PLAN.md` (Status progress) realigned
   with the human-approved repo-local bootstrap, without broadening scope.
2. Done. `TASK-STATE.json` moved to `phase: closure` / `current_gate: closure`;
   `PASS-0004-CHECKLIST.json` set to `ready_for_closeout`; `RECONCILE-PROOF.md`
   "Current Summary" realigned to the clean state with a resolution-history
   section.
3. Done. Finding recorded in `PLAN.md` Data Handling: the `TASK-META.json` files
   are tracked repo artifacts fully covered by the existing repo-state delta
   must-backup-delta class. No `DATA-HANDLING.md` broadening required.
   (`CODEX-REPO-MANIFEST.json` is already named there explicitly.)
4. In progress at handoff time: a `gh` UTF-8 readback decode bug
   ([BUG-0001.md](./BUG-0001.md)) was found and fixed; reconcile is now clean.
   Run raw `git diff -- <paths>` before the closeout commit and keep the
   reviewer-facing diff clean.
5. Still follow-on / out of scope: cross-repo bulk publication, Review tab
   proposal promotion, TaskCreate provider-interface implementation, worktree
   allocation, concurrency, and `drain my tasks pls`, unless the human
   explicitly rescopes Task-0012 again.

## Active Bug

- [BUG-0001.md](./BUG-0001.md): `gh` UTF-8 readback mojibake under Windows
  PowerShell 5.1 caused a false `text_conflict`. Status: fixed and verified by a
  clean reconcile. The UTF-8 console-encoding guard must stay in the scripts.

## Closure Gate

Task-0012 is at the human closure-acceptance gate. The closure review package
is [Testing/Closure/REVIEW-PACKAGE.md](./Testing/Closure/REVIEW-PACKAGE.md). It
links the clean reconcile proof, the BUG-0001 fix, the doc realignment, and the
exact closure approval question. The task is not marked `complete` until the
human accepts closure.

## Proposed TASK.md Changes

Do not apply these automatically from this handoff. They are annotations for the
next Codex session.

### TASK.md: Scope Realignment

The current `TASK.md` still describes the task as stopping at one pilot issue.
That was true when the task was drafted, but the human later approved continuing
with this repo's remaining existing tasks.

Recommended edit:

- Keep the one-issue pilot as the first acceptance gate.
- Add that the accepted issue shape was then used to bootstrap all existing
  CodexDashboard local tasks into the configured accepted-task provider.
- State explicitly that this remains repo-local publication, not cross-repo
  bulk publication.

### TASK.md: Target Truth

Recommended edit:

- Extend Target Truth to say `Task-0001` through `Task-0012` are represented by
  GitHub issues `#1` through `#12`.
- Say `Task-0001` through `Task-0009` have terminal local state and matching
  closed GitHub issue state.
- Say `Task-0010`, `Task-0011`, and `Task-0012` remain open because local state
  is non-terminal.
- Preserve the rule that GitHub issue number is the accepted task id.

### TASK.md: Sync Contract

Recommended edit:

- Add explicit UTF-8 read/write requirement for PowerShell scripts.
- Record that UTF-8 handling was validated because Task-0003 had smart quotes
  that previously rendered as mojibake.
- Keep `last_synced_at`, not `issue_updated_at` or `last_checked_at`.

### TASK.md: Acceptance Criteria

Recommended edit:

- Add acceptance criteria for `Bootstrap-TaskGitHubIssues.ps1`.
- Add acceptance criteria for `Reconcile-TaskGitHubState.ps1` final clean run.
- Add a criterion that reconcile must report `difference_count: 0` and
  `conflict_count: 0` before closeout.
- Add a criterion that issue-state corrections are allowed only when no
  `text_conflict` blocks that task.

### TASK.md: Non-Goals

Recommended edit:

- Split "bulk publication" into two meanings:
  - repo-local bootstrap of current CodexDashboard tasks: now completed in this
    task by human approval
  - cross-repo bulk publication across all configured repos: still follow-on
    and out of scope

### TASK.md: Implementation Home

Recommended edit:

- Add the repo-local skill directory as part of the implementation home.
- Add all `Tracking/Task-0001` through `Tracking/Task-0012` `TASK-META.json`
  files as task-provider artifacts.
- Add `Testing/BulkIssueBootstrap/` and `Testing/TaskGitHubReconcile/`.

## Proposed PLAN.md Changes

Do not apply these automatically from this handoff. They are annotations for the
next Codex session.

### PLAN.md: Status

Recommended edit:

- Update status from "Implementation has started with PASS-0001" to reflect
  that provider registry, pilot sync, issue-field setup, repo-local bootstrap,
  conflict review, and final reconcile have run.

### PLAN.md: Pass Structure

Recommended edit:

- Preserve existing pass order through the one-issue pilot.
- Add a pass for repo-local bootstrap after human acceptance of the pilot
  surface.
- Add a pass for reconcile/conflict-resolution and issue-state correction.
- Keep TaskCreate provider-interface follow-on as the final documentation pass.

Suggested pass names:

- `PASS-0005: Repo-Local Existing Task Bootstrap`
- `PASS-0006: Reconcile, Conflict Review, And Issue-State Correction`
- `PASS-0007: Closeout Documentation And Follow-On Queue`

If renumbering pass artifacts is too noisy, add these as post-pilot subpasses
or "additional human-approved work" instead. Avoid rewriting historical
checklists just to make numbering pretty.

### PLAN.md: PASS-0002/PASS-0003 Text

Recommended edit:

- PASS-0002 should remain the one-issue pilot.
- PASS-0003 should say the human accepted the visible issue/field surface and
  authorized continuing within this repo.
- Do not claim no bulk task publication starts in this task without qualifying
  that as cross-repo bulk publication.

### PLAN.md: Verification

Recommended edit:

- Add final reconcile as the closeout proof:
  [`Testing/TaskGitHubReconcile/RECONCILE-DIFFERENCES.md`](./Testing/TaskGitHubReconcile/RECONCILE-DIFFERENCES.md)
- Add that the dispatch dry run currently reports zero actions:
  [`Testing/TaskGitHubReconcile/RECONCILE-DISPATCH-DRYRUN.md`](./Testing/TaskGitHubReconcile/RECONCILE-DISPATCH-DRYRUN.md)
- Add UTF-8 script read validation as a regression guard for PowerShell runs.

### PLAN.md: Data Handling

Recommended edit:

- Update the data handling section from one `TASK-META.json` file to
  task-provider metadata for all currently bootstrapped local tasks.
- Decide whether this remains covered by normal git-backed repo-state backup.
  My recommendation is yes: these are tracked repo artifacts, not a new runtime
  data store.

## Cautions For Next Session

- Do not recreate `HANDOFF.md`; the human explicitly deleted it earlier. This
  file is the GitHub-specific handoff requested later.
- Do not revert unrelated worktree dirt.
- Do not use GitHub Projects or labels as queue/priority/human-needed state.
- Do not run broad cross-repo publication unless explicitly asked.
- Do not dispatch work from these issues yet. This task has only established
  accepted-task publication and reconcile semantics.
- Before closing Task-0012, run reconcile again and link the latest clean
  result.
