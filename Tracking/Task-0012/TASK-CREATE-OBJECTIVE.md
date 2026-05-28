# TaskCreate Objective

TaskCreate run date: 2026-05-27

Task id: `Task-0012`

Repository root: `C:\Agent\CodexDashboard`

## Objective

Create a concrete implementation `TASK.md` for the first GitHub-backed task-publication slice in CodexDashboard.

The task must use the research in `Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md` and the human's latest sequencing decision:

1. First, push a single `TASK.md` into the GitHub-backed path and inspect it.
2. Agree that the single pushed task meets the quality bar.
3. Then push all `TASK.md` files across configured repos.
4. Only after task publication is trustworthy, wire in the `drain my tasks pls` workflow where a central coordinator allocates a worktree and dispatches work.

## Intended Writeup Type

Use `concrete implementation` unless durable evidence proves that a consensus or research task is more honest.

The likely concrete task is not full cross-repo queue draining. The likely first task boundary is a pilot publication and inspection loop for one selected local `TASK.md`.

## Required Boundary

The task draft must keep these stages separate:

- pilot one `TASK.md` publication to GitHub
- bulk publication of all `TASK.md` files across repos
- central queue draining with concurrency and worktree allocation

Only the first stage should be in Task-0012 unless the writer can make a stronger, falsifiable case from durable evidence.

## Required Source-Of-Truth Split

Preserve the existing CodexDashboard Task-0008 split:

- GitHub Issues can own cross-repo backlog identity, shallow task discovery, triage, and links.
- Local task docs own rich scope, acceptance, research, proof, audits, and handoff.
- CodexDashboard backend/Temporal owns live execution state, active runs, concurrency, wait/poke/interrupt state, worktree locks, and restore commits.
- Shared worktree registry or backend worktree state owns reusable worktree availability; GitHub labels may mirror this but must not be canonical locks.

## Known Constraint

`gh` is installed locally, but `gh auth status -h github.com` currently reports no logged-in GitHub hosts. The task must require clear blocked behavior for missing authentication rather than silently producing fake publication output.

## Output Expected

Write:

- `Tracking/Task-0012/TASK.md`

When useful, also write a short writer note under `Tracking/Task-0012/` naming evidence used and any audit-readiness caveats.
