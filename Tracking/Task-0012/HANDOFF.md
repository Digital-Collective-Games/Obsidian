# Task-0012 Handoff

## Current State

Task-0012 is in planning.

The task has an approved-by-human scope boundary from task creation: publish one
selected local `TASK.md` to a GitHub-backed representation for inspection. It
must not become bulk publication or autonomous queue draining in this slice.

## Resume Point

Wait for human approval of
[PLAN.md](./PLAN.md) through
[Design/PLAN-APPROVAL/REVIEW-PACKAGE.md](./Design/PLAN-APPROVAL/REVIEW-PACKAGE.md).

If approved, start `PASS-0001` from the plan. If rejected, revise the plan first
and keep `TASK-STATE.json` in the planning gate.

## Important Constraints

- Do not use hidden chat context beyond task-owned docs and shared/repo docs.
- Do not write shared `.codex` process docs from this worker.
- Do not use the human's dashboard lane, live Codex data, or service lane for
  disposable proof unless explicitly authorized.
- Do not write `Tracking/Task-<id>/GITHUB-ISSUE.json` after auth failure.
- Do not claim task closure without real publish/update proof or an explicit
  blocked state for missing GitHub auth.

## Current Artifacts

- [TASK.md](./TASK.md)
- [RESEARCH.md](./RESEARCH.md)
- [PLAN.md](./PLAN.md)
- [TASK-STATE.json](./TASK-STATE.json)
- [Design/PLAN-APPROVAL/REVIEW-PACKAGE.md](./Design/PLAN-APPROVAL/REVIEW-PACKAGE.md)

## Validation Status

- Build: not run
- Unit tests: not run
- Regression: not run

These are expected to remain not run until the implementation passes start.
