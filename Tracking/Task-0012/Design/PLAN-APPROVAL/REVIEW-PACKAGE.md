# Task-0012 Plan Approval Review Package

## Gate

Plan approval for Task-0012.

Approved by human direction on 2026-05-28.

## Approval Question

Approve [../../PLAN.md](../../PLAN.md) as the implementation plan for the
Codex-operated GitHub task-state pilot?

## If Approved

Implementation started with `PASS-0001`:

- regular Codex renders the pilot issue title/body/labels
- regular Codex runs `gh` for the one-issue pilot when auth and target repo are
  available
- local [../../TASK.md](../../TASK.md) stays the rich task source
- GitHub Issue becomes the queryable backlog/task-state surface for the pilot
- `GITHUB-ISSUE.json` records mapping and sync state only after successful
  publish/update/readback
- bulk publication, queue draining, worktree allocation, backend endpoint work,
  and dashboard UI changes remain out of scope

`PASS-0001` produced the preview artifacts required before any GitHub write.

## Not Approved Yet

Approval of this package does not approve:

- backend orchestration publication work
- publishing all task docs
- implementing `drain my tasks pls`
- touching the human service lane or dashboard lane for proof
- using GitHub as canonical live execution state
- closing the task while GitHub auth is missing, publication/readback proof is
  absent, human inspection is pending, or the pilot issue shape is rejected

## Change Notes

Task-owned planning artifacts:

- [../../TASK.md](../../TASK.md)
- [../../RESEARCH.md](../../RESEARCH.md)
- [../../PLAN.md](../../PLAN.md)
- [../../TASK-STATE.json](../../TASK-STATE.json)

`HANDOFF.md` was removed by human direction.

No product code has been changed for this gate.

## Evidence And Validation

Evidence used:

- [../../TASK.md](../../TASK.md)
- [../../HUMAN-DIRECTIVES-FOR-WORKER.md](../../HUMAN-DIRECTIVES-FOR-WORKER.md)
- [../../Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md](../../Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md)
- [../../../../DATA-HANDLING.md](../../../../DATA-HANDLING.md)

Validation status:

- Build: not run
- Unit tests: not run
- Regression: not run

This is expected for a planning gate. Product validation starts only after plan
approval and implementation.

## Caveats

- A real authenticated publish/update/readback proof depends on GitHub auth and
  a target `owner/name` repo.
- Without auth, the task can only reach an honest blocked state after proving
  missing-auth behavior.
- Successful publication alone is not closure. A real issue must be packaged for
  human inspection, and pending or rejected inspection keeps the task open.

## Rejection Path

If rejected, revise [../../PLAN.md](../../PLAN.md), keep
[../../TASK-STATE.json](../../TASK-STATE.json) in the planning gate, and update
this review package with the revised approval question.
