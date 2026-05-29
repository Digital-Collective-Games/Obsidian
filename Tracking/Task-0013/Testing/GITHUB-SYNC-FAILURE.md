# Task-0013 GitHub Sync Failure: Diagnosis And Recommended Fix

- Status: READ-ONLY diagnosis. No GitHub issue was created, edited, or closed. No
  push or sync/reconcile state was mutated.
- Generated: 2026-05-29
- Provider repo: `Digital-Collective-Games/Obsidian`
- Grounding read first: `AGENTS.md` guardrail, `skills/obsidian-operator/SKILL.md`
  (canonical task<->GitHub sync entry), and Task-0012's TaskGitHubReconcile
  artifacts that built the sync mechanism.

## Observable Symptom (confirmed)

On `Digital-Collective-Games/Obsidian`, `gh issue list --state all --limit 50`
returns issues `#1`–`#12`, each titled `Task-00NN: ...`. There is **no issue #13**.
Task-0013 was completed locally (`Tracking/Task-0013/` has `TASK.md`,
`HANDOFF.md`, and `TASK-STATE.json` with `"status": "complete"`), but it has no
GitHub issue.

## Failure Mode (evidence-based)

Task-0013 was **never run through the provider-sync step**, so its GitHub issue
was never created and no provider binding was written. This is a missed
operator/lifecycle trigger, not a tooling bug or a blocked conflict.

The single most direct piece of evidence:

- Tasks 0001–0012 each have a `Tracking/Task-00NN/TASK-META.json` provider
  binding (`provider_kind`, `provider_repo`, `issue_number`, `issue_url`,
  `last_synced_at`). **Task-0013 has no `TASK-META.json` at all.** It is the only
  task folder missing this file. `TASK-META.json` is written by
  `Sync-TaskToGitHubIssue.ps1` only after a successful GitHub readback, so its
  absence proves the sync/create step never completed for Task-0013.

Corroborating timeline evidence:

- The last reconcile run captured under
  `Tracking/Task-0012/Testing/TaskGitHubReconcile/RECONCILE-DIFFERENCES.md` is
  dated `2026-05-28T23:32` and reports `Local tasks: 12, Remote issues: 12,
  Difference count: 0`. Task-0013 did not exist yet at that run.
- Task-0013's folder was first created `2026-05-29 10:12:55` (git commit
  `e99ac89`, "task-0013: rebrand to Obsidian, ..."), i.e. **after** the last
  reconcile/bootstrap run.
- No bootstrap, sync, or reconcile run touched Task-0013 between its creation and
  its closeout commits (`247f297`, `ac25bc9`, `8fb7752`). The create/sync step
  simply never fired for #13.

Task-0013 is **not** skipped for any blocking reason: it has a valid
`Tracking/Task-0013/TASK.md`, a four-digit folder name, and a schema-valid
`TASK-STATE.json` (`status: complete`). Both `Bootstrap-TaskGitHubIssues.ps1` and
`Reconcile-TaskGitHubState.ps1` discover any `Tracking/Task-*/TASK.md` and would
pick it up; the scripts were just never invoked after #13 existed.

## How The Sync Is Supposed To Happen (and the process gap)

Per `skills/obsidian-operator/SKILL.md` (Provider Contract):

> "For new accepted tasks, TaskCreate must create the GitHub issue first, then
> materialize `Tracking/Task-<issue-number>/TASK.md`."

So the intended order is **issue-first, folder-second**, with the local task id
equal to the GitHub issue number. Concretely the sync surfaces are:

- `skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1` — discovers
  `Tracking/Task-*` folders with a `TASK.md`; for any task whose issue does not
  exist, it creates the issue (`gh api -X POST /repos/<repo>/issues`), stops hard
  if GitHub returns a non-matching number, then runs the sync and writes
  `TASK-META.json`. Supports `-StartTaskNumber N -EndTaskNumber N` to target one
  task.
- `skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1` — syncs exactly
  one local `TASK.md` to its same-number issue and writes `TASK-META.json` after
  readback.
- `skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1` — iterates all
  local tasks; for any task with **no matching GitHub issue** it emits a
  `create_github_issue` action whose dispatch command is
  `Bootstrap-TaskGitHubIssues.ps1 -StartTaskNumber N -EndTaskNumber N`
  (Reconcile script lines 817-822 and 561-565). So reconcile DOES detect and plan
  the fix for a missing issue — it is just dry-run/operator-gated by design.

The underlying process gap that let this happen silently:

1. **The create-the-issue obligation lives only in the repo-local skill, not in
   the shared TaskCreate process the writer actually follows.** The shared
   `C:\Users\gregs\.codex\Orchestration\Processes\TASK-CREATE.md` and the
   `TaskCreate/` coordinator/worker docs make **no mention** of GitHub issues,
   provider sync, `TASK-META.json`, `Bootstrap`, `Reconcile`, or `Sync-Task`. A
   TaskCreate run can therefore complete a `TASK.md` (and the task can run to
   completion) with the provider-sync step entirely skipped.
2. **The intended issue-first ordering was inverted for #13.** The TASK.md was
   materialized (and the whole task executed/closed) without an issue ever being
   created first.
3. **There is no automatic post-TaskCreate sync trigger and no scheduled
   reconcile.** The scheduled job specs under
   `C:\Users\gregs\.codex\Orchestration\Jobs\specs\` are digests, a payment
   reminder, a billing true-up, and `dc-nightly-checks` (a Krafton/Wise payment
   check) — none runs the obsidian-operator sync/reconcile. So nothing
   periodically catches a task that missed its create step. Sync is 100%
   operator-triggered, and for #13 the operator never triggered it.

Net: this is a **missing post-TaskCreate sync trigger / missing periodic
bootstrap-on-reconcile for new tasks**, combined with the create obligation
living outside the shared TaskCreate process.

## Recommended Fix (do not execute yet — diagnose-first per the human)

All write steps must go through the obsidian-operator skill. Run from REPO_ROOT
`C:\Agent\CodexDashboard`.

### Step 1 — Confirm the gap with a dry-run reconcile

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1 -DryRun -DispatchActions
```

Expect a `create_github_issue` difference for `Task-0013` (authority
`local_task_document`, "Local task has no matching GitHub issue #13"), with the
dispatch command pointing at `Bootstrap-TaskGitHubIssues.ps1 -StartTaskNumber 13
-EndTaskNumber 13`. Confirm no `text_conflict` blocks it.

### Step 2 — Preview the exact issue body for #13

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1 -TaskPath Tracking/Task-0013/TASK.md -DryRun
```

Verify the rendered `issue_number` is `13` and the title is `Task-0013: ...`.

### Step 3 — Create the issue (issue-number safety stop is built in)

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1 -StartTaskNumber 13 -EndTaskNumber 13
```

The script POSTs the new issue and **stops immediately** if GitHub returns any
number other than 13 (e.g. if a PR or prior issue consumed #13). If it stops with
a number mismatch, do not improvise a mapping layer — escalate, because issue
number must equal task number per the skill's Provider Contract. After a clean
create it runs the sync and writes `Tracking/Task-0013/TASK-META.json`.

### Step 4 — Bring issue state into line with local status

Task-0013 `TASK-STATE.json` is `status: complete` (terminal -> issue should be
CLOSED). After the issue exists, a reconcile dispatch dry run will surface a
`close_github_issue_to_match_local_task_state` action. Close it the skill's way:

```powershell
gh issue close 13 --repo Digital-Collective-Games/Obsidian --reason completed --comment "Closed by task reconcile: local Tracking/Task-0013/TASK-STATE.json status is 'complete'."
```

Then refresh `TASK-META.json` from a fresh readback (re-running the per-task sync
or reconcile updates `last_synced_at`).

### Step 5 — Verify clean

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1 -DryRun -DispatchActions
```

Expect `Local tasks: 13, Remote issues: 13, Difference count: 0` and a
`TASK-META.json` present for Task-0013.

## Preventing Recurrence (the real fix)

Creating issue #13 fixes the instance. The process gap needs one of these:

1. **Wire the provider-sync step into TaskCreate closure.** Add the
   issue-first-then-materialize obligation (and a TaskCreate completion check that
   `Tracking/Task-<id>/TASK-META.json` exists and binds the same-number issue)
   into the TaskCreate path so a new accepted task cannot be called created
   without its GitHub issue. Today that obligation is only in the repo-local
   skill, which TaskCreate workers do not necessarily read.
2. **Add a scheduled reconcile/bootstrap for new tasks.** Add a job spec under
   `C:\Users\gregs\.codex\Orchestration\Jobs\specs\` (backend/Temporal path per
   `AGENTS.md` "Jobs And Scheduling") that periodically runs
   `Reconcile-TaskGitHubState.ps1` and surfaces (or, on explicit policy,
   auto-bootstraps) any `create_github_issue` difference, so a task that misses
   its create step is caught instead of silently never appearing on GitHub. Keep
   destructive/write actions human-gated per the skill's dry-run-first guardrail;
   the value is the detection signal.
3. **Add a closure/handoff check.** Make task closeout assert
   `TASK-META.json` exists and the bound issue resolves, so a task cannot reach
   `status: complete` while invisible to the GitHub accepted-task provider.

The minimal durable prevention is (1)+(3): make "issue exists and TASK-META binds
it" a hard gate at task create and at task closure, with (2) as a backstop.

## Evidence Index

- `Tracking/Task-0013/` — has TASK.md, HANDOFF.md, TASK-STATE.json (complete);
  **no TASK-META.json**.
- `Tracking/Task-0012/TASK-META.json` — example of the binding every synced task
  has (issue_number 12, issue_url, last_synced_at).
- `Tracking/Task-0012/Testing/TaskGitHubReconcile/RECONCILE-DIFFERENCES.md` — last
  clean reconcile baseline at 12 tasks / 12 issues, dated 2026-05-28T23:32, before
  #13 existed.
- `skills/obsidian-operator/SKILL.md` — Provider Contract (issue-first), script
  roles, reconcile semantics.
- `skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1` — create path
  + same-number safety stop.
- `skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1` — emits
  `create_github_issue` for tasks with no matching issue (lines ~817-822, dispatch
  ~561-565).
- `C:\Users\gregs\.codex\Orchestration\Processes\TASK-CREATE.md` and
  `...\TaskCreate\*` — no GitHub/provider-sync obligation (the process gap).
- `C:\Users\gregs\.codex\Orchestration\Jobs\specs\*.json` — no task<->GitHub
  reconcile job exists.
- `gh issue list --repo Digital-Collective-Games/Obsidian --state all` — issues
  #1–#12 only; no #13.
- git: Task-0013 first created at commit `e99ac89` (2026-05-29 10:12:55), after
  the last reconcile run.
