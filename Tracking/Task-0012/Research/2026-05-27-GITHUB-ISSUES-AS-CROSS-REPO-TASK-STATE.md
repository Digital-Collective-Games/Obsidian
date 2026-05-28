# GitHub Issues As Cross-Repo Task State Research

Capture date: 2026-05-27

Status: research capture for a future TaskCreate draft. This is not an audited or enqueue-ready task writeup.

## Human Need

The human wants CodexDashboard to move beyond repo-local `Tracking/Task-<id>/TASK.md` discovery as the only task inventory mechanism.

The motivating need is:

- query tasks across multiple repos
- use `gh` CLI as the first practical GitHub integration path instead of direct hand-written API calls
- keep local task docs for rich context that can be committed and pushed
- add a central coordinator that can drain a task queue
- dispatch tasks onto owned worktrees or worktree slots
- add and remove worktrees as part of dispatch lifecycle
- enforce a configured concurrency limit on active tasks

The key product tension is source-of-truth split:

- GitHub Issues are better for cross-repo backlog discovery and state queries.
- Local task docs are still better for rich context, acceptance, research, pass history, proof, and handoff.
- CodexDashboard backend runtime remains the right home for live execution truth, worktree locks, waits, poke/interrupt state, and active-run concurrency.

## Captured Slack Conversation

Source tab:

- title: `Alex Schearer (DM) - Eleventh Hour Games - Slack`
- URL: `https://app.slack.com/client/T02K59MT70S/D05DL6SQEHM`
- capture method: Chrome debug session scrape through `C:\Agent\Orchestrator\Scripts\Scrape-OpenChromeTabs.ps1 -UrlContains slack -Format text`
- captured at: `2026-05-27T20:01:46-04:00`

Relevant excerpt from today:

```text
Greg Semple, 6:59 PM:
thanks fer the idea to do 1 subagent per "node" - im using my coordinator-worker pattern for each "process" in my thing now, using the blind subagent pattern everywhere

Alex Schearer, 7:12 PM:
yeah... what's neat is that you can also get the shells to do it -- or at least claude (haven't tried codex). e.g. you can have 4 clones of a repo in a folder, open claude code in the parent folder, and then have it start running work with subagents against them pipeline-like (plan, implement, review, merge, etc.) crazy

Greg Semple, 7:13 PM:
i had to teach codex about git worktrees, and how to "reserve" them when dispatching a task

Greg Semple, 7:13 PM:
be interesting to see how many UE folders i can plate spin at a time lol

Alex Schearer, 7:14 PM:
I have this setup: https://code.claude.com/docs/en/channels-reference

Greg Semple, 7:15 PM:
hmm so each task/worktree gets its own two way channel?

Alex Schearer, 7:15 PM:
so I communicate with claude via discord ... it's setup in a folder with 8 sub-folders spanning two github projects. the projects all have github issues for work to do. on discord I say, "run through the sub-agent workflow and drain the backlog. then dogfood the tool, opening high/medium issues. repeat until no new issues found" -- and it does what you expect

Alex Schearer, 7:16 PM:
I say that via discord. it then pings me with updates, questions, or if it gets stuck

Alex Schearer, 7:16 PM:
(the two projects are interrelated or I wouldn't both mixing them like this)

Greg Semple, 7:18 PM:
that's nice, i'll have to do the task queue thing next

Greg Semple, 7:18 PM:
"drain task queue pls"

Alex Schearer, 7:19 PM:
if you haven't already discovered the gh CLI from GitHub it's worth it, assuming you are doing things on there. agent can then interact with issues to read/write as well as the normal git operations

Greg Semple, 7:19 PM:
ah yeah im still using local TASK.md docs, but it is getting to the point where i can't remember statuses

Greg Semple, 7:20 PM:
gh CLI would be a nice upgrade - i was fearing it would be harder like having to make direct http calls to some mystery api

Alex Schearer, 7:20 PM:
I was using a local folder filled with task files but I am finding github issues superior

Greg Semple, 7:20 PM:
i figure i'd probably sync TASK.md from gh, just so i have a durable copy to make sense of the work being committed to git locally

Greg Semple, 7:21 PM:
but being able to query gh issues is just too hard to pass up

Greg Semple, 7:21 PM:
yeah, you just convinced me... next thing on my plate

Greg Semple, 7:21 PM:
after this JS -> UE port -.-
```

## Durable Docs And Code Reviewed

Repo-local docs:

- [AGENTS.md](../../../AGENTS.md)
- [DATA-HANDLING.md](../../../DATA-HANDLING.md)
- [Task-0008 TASK.md](../../Task-0008/TASK.md)
- [Task-0008 PLAN.md](../../Task-0008/PLAN.md)
- [Task-0008 durable execution-state contract](../../Task-0008/Design/DURABLE-EXECUTION-STATE-CONTRACT.md)
- [Task-0009 TASK.md](../../Task-0009/TASK.md)
- [backend/orchestration/README.md](../../../backend/orchestration/README.md)
- [backend taskrun types](../../../backend/orchestration/internal/taskrun/types.go)
- [backend taskrun service](../../../backend/orchestration/internal/taskrun/service.go)
- [backend config](../../../backend/orchestration/internal/config/config.go)

Shared docs:

- `C:\Users\gregs\.codex\Orchestration\TASK-STATE.md`
- `C:\Users\gregs\.codex\Orchestration\WORKTREES.md`
- `C:\Users\gregs\.codex\Orchestration\Processes\TASK-CREATE.md`
- `C:\Users\gregs\.codex\Orchestration\Processes\TaskCreate\README.md`

CLI facts checked locally:

- `gh version 2.88.1 (2026-03-12)` is installed.
- `gh issue list` supports repo-specific issue queries with `--repo`, filters, search, and JSON fields.
- `gh search issues` supports cross-repo issue search with repeated `--repo`, `--owner`, labels, assignee, state, and JSON fields.
- `gh issue view`, `gh issue create`, and `gh issue edit` support reading and writing issues.
- `gh api` is available for direct REST or GraphQL gaps.
- Current auth status: `gh auth status -h github.com` reports no logged-in GitHub hosts. A real implementation needs `gh auth login` or a configured `GH_TOKEN`/`GITHUB_TOKEN`.

## Current Durable Convention

CodexDashboard already has a deliberate state split.

Task-0008 says local task docs are declared task-definition truth:

- `TASK.md`
- `PLAN.md`
- `HANDOFF.md`
- `TASK-STATE.json`

The backend then snapshots those docs into runtime state at dispatch and preserves live execution facts in Temporal-backed run state.

The Task-0008 contract explicitly rejects:

- UI state as canonical runtime truth
- ad hoc agent HTTP calls as the primary state-update mechanism
- current git inspection alone replacing a captured runtime snapshot
- a git rewind silently erasing live wait, interrupt, cleanup, or run truth

Shared `TASK-STATE.json` convention is narrower. It owns compact current orchestration state for one local task, not rich evidence or cross-repo backlog discovery.

Shared `WORKTREES.md` already defines a current-availability registry for expensive reusable local worktree slots:

- `C:\Users\gregs\.codex\Orchestration\WORKTREE-ALLOCATIONS.json`

Task-0008 implementation already has disposable backend-owned worktree support for runs:

- dispatch provisions a detached owned worktree from the current declared worktree baseline
- cleanup removes the owned worktree through `git worktree remove --force`
- run readback exposes `repo_lane` fields such as owned root, checkout mode, baseline commit, current commit, restore commit, and reset status

What is missing for this new need:

- GitHub Issues as a task inventory source
- cross-repo issue querying
- a repo map that connects GitHub repos to local worktree roots
- a central queue-draining coordinator
- configured max active task count
- issue-to-local-doc sync rules
- issue comments/labels as durable but non-canonical mirrors of run progress
- integration with reusable worktree slots across repos, not only Task-0008 disposable per-run worktrees

## Recommended Source-Of-Truth Split

### GitHub Issues Should Own Cross-Repo Backlog State

GitHub Issues should be the source of truth for:

- cross-repo task identity:
  - repo
  - issue number
  - issue URL
  - title
- backlog membership:
  - open or closed
  - candidate for Codex dispatch or not
- triage and prioritization state:
  - labels
  - milestone
  - assignee
  - project fields when available
- queue queries:
  - tasks across multiple repos
  - tasks assigned to the human or automation actor
  - tasks labeled ready for Codex
  - recently updated tasks

The key reason: GitHub Issues solve the cross-repo query problem directly. A local `Tracking/Task-*` tree does not.

Candidate query shape:

```powershell
gh search issues `
  --repo gregsemple2003/CodexDesktop `
  --repo <owner>/<other-repo> `
  --state open `
  --label codex-task `
  --json repository,number,title,state,labels,assignees,milestone,updatedAt,url
```

Repo-specific query shape:

```powershell
gh issue list `
  --repo gregsemple2003/CodexDesktop `
  --state open `
  --label codex-task `
  --json number,title,state,labels,assignees,milestone,updatedAt,url
```

Issue detail shape:

```powershell
gh issue view 123 `
  --repo gregsemple2003/CodexDesktop `
  --comments `
  --json number,title,body,state,labels,assignees,milestone,comments,updatedAt,url
```

### Local Task Docs Should Own Rich Context And Proof

Local docs should remain the source of truth for:

- exact task scope
- acceptance criteria
- implementation home
- research
- solution rationale
- pass plan
- proof artifacts
- bugs and audits
- handoff status
- local evidence paths

GitHub issue bodies are not the right home for the entire durable task packet. They are better as an index and shallow shared backlog record.

Recommended issue body shape:

- concise human summary
- status/triage labels
- link to committed local task docs:
  - `Tracking/Task-<id>/TASK.md`
  - `Tracking/Task-<id>/PLAN.md`
  - `Tracking/Task-<id>/HANDOFF.md`
  - research or proof docs when relevant
- branch or commit references when the docs have been pushed

Recommended local task metadata additions for GitHub-backed tasks:

- `github_repo`
- `github_issue_number`
- `github_issue_url`
- `github_issue_state_snapshot`
- `github_issue_updated_at_snapshot`
- `github_sync_status`
- `github_sync_last_checked_at`

Those fields could live in `TASK-STATE.json` only if they are current compact orchestration state. Longer GitHub context, issue-body captures, and sync notes should live in markdown research, handoff, or sync artifacts.

### Backend Runtime Should Own Live Execution State

CodexDashboard backend and Temporal-backed task-run state should remain authoritative for:

- active run existence
- dispatch state
- wait contract
- poke eligibility
- interrupt state
- sleeping or stalled classification
- meaningful progress freshness
- current run ownership
- owned checkout identity
- worktree add/remove operations
- restore commit and cleanup state
- max active task concurrency

GitHub labels such as `codex:running` or comments like `Dispatched by CodexDashboard` can be useful mirrors, but they must not become the lock or runtime source of truth.

Reason: GitHub Issues are durable and queryable, but they are too slow, remote, and human-editable to be the safe owner of local worktree locks or live run state.

### Worktree Allocation Should Stay Local/Backend-Owned

There are two different worktree classes:

- disposable per-run owned worktrees
  - already modeled by Task-0008 backend `RepoLane`
  - can be created and removed for a single task run
- expensive reusable worktree slots
  - already governed by shared `WORKTREES.md`
  - current availability is recorded in `WORKTREE-ALLOCATIONS.json`

A central coordinator should use both:

- prefer disposable owned worktrees for ordinary small tasks
- reserve expensive reusable slots for repos where checkout/build cost justifies reuse
- sync reusable slot state with `WORKTREE-ALLOCATIONS.json`
- expose the active assignment through backend runtime readback

Do not make a GitHub issue label the lock for a local worktree. It can mirror `assigned_to_slot` for human visibility, but the backend/local registry must be canonical.

## Central Coordinator Shape

Proposed backend-side components:

### 1. Repository Registry

Purpose:

- define which GitHub repos CodexDashboard may query
- define which local worktree root or slot pool maps to each repo
- define per-repo labels and dispatch policy

Candidate fields:

- `repo_id`
- `github_repo`
- `default_branch`
- `local_primary_worktree`
- `tracking_root`
- `slot_pool_id`
- `issue_query_labels`
- `dispatch_ready_labels`
- `max_active_runs_for_repo`

### 2. GitHub Issue Task Source

Purpose:

- use `gh` to query issues
- normalize issue data into backend task candidates
- preserve issue URL, repo, issue number, labels, assignees, milestone, and updated time

This should be a source adapter, not the whole task system.

Candidate interface shape:

- `ListBacklogTasks(ctx, repoSet, filters)`
- `GetIssueTask(ctx, repo, number)`
- `MirrorRunEvent(ctx, issueRef, eventSummary)`
- `ApplyTriageLabel(ctx, issueRef, add/remove labels)`

Implementation may shell out to `gh` first because that is the smallest practical path. If shelling out becomes brittle, the adapter can later move to `gh api` or native GitHub API calls without changing the coordinator model.

### 3. Local Task Doc Sync

Purpose:

- create or update `Tracking/Task-<id>/` docs from an issue when local context is needed
- keep the GitHub issue linked to the local task docs
- push committed docs so GitHub issue links resolve for future agents

Recommended rule:

- issue title/body/labels define shallow backlog identity and priority
- local `TASK.md` defines rich implementation scope once a task is selected for real work
- local docs must be committed and pushed before they are used as cross-repo context from an issue link

Do not rely on unpushed local docs as the cross-repo source of truth.

### 4. Queue-Draining Coordinator

Purpose:

- answer `drain task queue`
- find candidate issues across configured repos
- enforce global and per-repo concurrency
- acquire a repo lane or reusable slot
- dispatch task runs
- post issue comments or labels as mirrors
- stop when no eligible tasks remain or concurrency is saturated

Canonical active count should come from backend task-run state, not GitHub labels.

Candidate config:

- `CODEX_ORCHESTRATION_MAX_ACTIVE_TASKS`
- `CODEX_ORCHESTRATION_MAX_ACTIVE_TASKS_PER_REPO`
- tracked service-lane config for repo list and dispatch policy

Current `backend/orchestration/internal/config/config.go` has no concurrency or GitHub repo config yet.

### 5. Worktree Manager

Purpose:

- create and remove disposable worktrees for runs
- reserve and release reusable slots
- refuse dispatch when no lane is available
- expose lane state through task/run readback

Task-0008 already implements the core disposable lane primitives. The missing extension is cross-repo slot selection and a concurrency-aware dispatcher above it.

## Human Sequencing Decision

Update from the human after the initial research capture:

1. First, push a single `TASK.md` into the GitHub-backed path and inspect it.
2. Agree that the single pushed task meets the quality bar.
3. Then push all `TASK.md` files across configured repos.
4. Only after task publication is trustworthy, wire in the `drain my tasks pls` workflow where a central coordinator allocates a worktree and dispatches work.

This changes the first implementation boundary. The first slice is not central queue draining and is not full multi-repo automation.

The first slice is a pilot publication and inspection loop for one task.

## Suggested First Implementation Boundary

The first implementation task should prove the smallest write path that can be reviewed by a human before scale-up.

Recommended first slice:

1. Select one pilot local `TASK.md`.
2. Publish it to GitHub through `gh`.
3. Preserve a durable mapping between:
   - local repo
   - local task id
   - local `TASK.md` path
   - GitHub repo
   - GitHub issue number
   - GitHub issue URL
   - publish timestamp
   - source commit when available
4. Inspect the resulting GitHub issue and local mapping.
5. Decide whether the issue body, title, labels, links, and local-doc references meet the task quality bar.
6. Record what should change before bulk publication.

Acceptance for this first slice should include:

- the GitHub issue is readable without hidden local context
- the issue links back to committed local task docs when those docs are meant to be durable context
- the local task docs record the GitHub issue mapping
- the publication path is idempotent enough to update the pilot without creating duplicates
- the write path does not move live execution state into GitHub
- the write path does not claim the task is enqueue-ready merely because it exists as an issue
- `gh` authentication failure is surfaced as a clear blocked state

After the pilot passes human inspection, the second slice can bulk-publish `TASK.md` files across configured repos with dry-run, duplicate detection, and per-repo mapping rules.

Only after bulk task publication is trustworthy should the third slice add queue draining:

- query GitHub issues across configured repos
- use backend runtime state for active count and concurrency limits
- allocate a disposable owned worktree or reusable worktree slot
- dispatch the selected task through the central coordinator
- mirror high-level run events back to GitHub without making GitHub the runtime lock

This preserves the Task-0008 state split while giving the human a visible quality gate before mass issue creation or autonomous dispatch.

## What GitHub Should Not Own

GitHub Issues should not be canonical for:

- whether a local worker is actually alive
- whether a run is waiting, sleeping, interrupted, or blocked
- which local worktree is currently safe to mutate
- whether cleanup/reset completed
- whether a poke is justified
- active concurrency locks
- local uncommitted proof artifacts
- high-frequency progress updates

Mirroring these into issue comments can be useful, but the backend must remain the source of truth.

## Open Questions For TaskCreate

These questions would change the eventual task boundary:

- Which single `TASK.md` should be used as the pilot publication?
- What exact issue title/body/label/link shape counts as meeting the bar after inspection?
- Does the pilot create a new issue, update an existing issue, or support both with explicit duplicate detection?
- Which repos should be in the first configured repo set?
- What label convention should define `eligible for Codex dispatch`?
- Should GitHub Projects fields be part of the first slice, or deferred until issues and labels work?
- Should local `Task-<id>` ids be preserved, or should GitHub-backed tasks use a composite id such as `<repo>#<issue>` in backend readback?
- When an issue becomes selected for implementation, should the local task folder be generated immediately or only when a human approves the draft?
- How should this interact with completed local tasks that predate GitHub Issues?

## Research Verdict

Alex's approach maps well onto CodexDashboard, but only if GitHub is introduced as the cross-repo backlog and query layer, not as a replacement for the existing runtime contract.

Recommended state ownership:

- GitHub Issues: backlog identity, cross-repo discovery, triage, priority, high-level open/closed lifecycle.
- Local task docs: rich task specification, research, plan, proof, audit, and handoff context that can be pushed to GitHub.
- CodexDashboard backend/Temporal: live execution state, active run ownership, wait/poke/interrupt semantics, worktree locks, restore commits, and concurrency.
- Shared worktree registry: reusable expensive slot availability, coordinated with backend runtime when slots are used.

The next TaskCreate draft should likely be a concrete implementation task for the pilot publication loop:

- publish one selected local `TASK.md` through `gh`
- inspect the resulting GitHub issue
- record the local/GitHub mapping
- preserve the existing source-of-truth split
- block on `gh` authentication instead of silently falling back to fake local output

Bulk `TASK.md` publication and central queue-draining dispatch should be separate follow-on tasks unless the pilot proves the issue shape and mapping rules are already stable.
