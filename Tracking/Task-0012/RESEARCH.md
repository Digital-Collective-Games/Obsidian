# Task-0012 Research Summary

## Status

Planning-ready summary distilled from existing task-owned research.

This is not a new independent research run. It preserves the actionable
conclusions from
[Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md](./Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md)
so implementation planning can start from a stable task-root research artifact.

## Decision-Shaping Findings

- GitHub Issues should own cross-repo backlog identity, shallow discovery,
  triage, labels, and issue URLs.
- Local task docs should remain the rich source of truth for scope,
  acceptance, research, proof, audits, and handoff.
- CodexDashboard backend and Temporal state should remain the live execution
  source of truth for active runs, waits, interrupts, cleanup, owned lanes, and
  concurrency.
- `gh` is the smallest useful first integration path because it is installed
  locally and supports issue create, edit, view, list, and search.
- The current environment has `gh`, but GitHub auth was not available during
  task creation, so the implementation must fail closed when auth is missing.
- The first rollout slice must publish one selected `TASK.md` only. Bulk
  publication and queue draining are follow-on work.

## Recommended Implementation Direction

Build a backend-mediated pilot publisher under `backend/orchestration/`:

- render a bounded issue title/body/labels from one local task definition
- check `gh auth status -h github.com` before any non-dry-run write
- create or update exactly one issue through `gh`
- persist `Tracking/Task-<id>/GITHUB-ISSUE.json` only after a successful write
- expose a read endpoint for the mapping artifact
- preserve preview and blocked-auth proof under this task's `Testing/` folder

## Open Execution Inputs

These inputs affect the real publish run, not the implementation shape:

- target GitHub repo, formatted as `owner/name`
- selected pilot task id
- whether/when GitHub auth is available for the real write proof

Until those are provided, implementation can still prove renderer behavior,
dry-run behavior, fake-`gh` create/update paths, mapping persistence, and
missing-auth blocked behavior.
