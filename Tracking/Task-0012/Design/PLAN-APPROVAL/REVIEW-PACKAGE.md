# Task-0012 Plan Approval Review Package

## Gate

Plan approval for Task-0012.

## Approval Question

Approve [../../PLAN.md](../../PLAN.md) as the implementation plan for the first
GitHub-backed task publication slice?

## If Approved

Implementation may start with `PASS-0001` only:

- add the backend task publication core under `backend/orchestration/`
- keep the scope to one selected local `TASK.md`
- preserve missing-auth blocked behavior
- defer bulk publication, queue draining, worktree allocation, and dashboard UI
  changes

## Not Approved Yet

Approval of this package does not approve:

- publishing all task docs
- implementing `drain my tasks pls`
- touching the human service lane or dashboard lane for proof
- using GitHub as canonical live execution state
- closing the task without either real GitHub publish/update proof or an
  explicit auth-blocked state

## Change Notes

New task-owned planning artifacts:

- [../../RESEARCH.md](../../RESEARCH.md)
- [../../PLAN.md](../../PLAN.md)
- [../../HANDOFF.md](../../HANDOFF.md)
- [../../TASK-STATE.json](../../TASK-STATE.json)

No product code has been changed for this gate.

## Evidence And Validation

Evidence used:

- [../../TASK.md](../../TASK.md)
- [../../HUMAN-DIRECTIVES-FOR-WORKER.md](../../HUMAN-DIRECTIVES-FOR-WORKER.md)
- [../../Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md](../../Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md)
- [../../../Task-0008/Design/DURABLE-EXECUTION-STATE-CONTRACT.md](../../../Task-0008/Design/DURABLE-EXECUTION-STATE-CONTRACT.md)
- [../../../../REGRESSION.md](../../../../REGRESSION.md)
- [../../../../TESTING.md](../../../../TESTING.md)
- [../../../../DATA-HANDLING.md](../../../../DATA-HANDLING.md)

Validation status:

- Build: not run
- Unit tests: not run
- Regression: not run

This is expected for a planning gate. Product validation starts only after plan
approval and implementation.

## Caveats

- A real authenticated publish/update proof depends on GitHub auth and a target
  `owner/name` repo. Without those inputs, the task can only reach an honest
  blocked state after implementation proves missing-auth behavior.
- Clean-context QA is required before claiming the resulting issue shape is
  human-approved. If no QA worker can be launched, that gate must be recorded as
  blocked or non-conformant.

## Rejection Path

If rejected, revise [../../PLAN.md](../../PLAN.md), keep
[../../TASK-STATE.json](../../TASK-STATE.json) in the planning gate, and update
this review package with the revised approval question.
