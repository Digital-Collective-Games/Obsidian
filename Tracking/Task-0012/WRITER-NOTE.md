# TaskCreate Writer Note

Date: 2026-05-27

## Role

TaskCreate writer-worker for `Task-0012`.

## Writeup Type Chosen

Burden-reduction concrete implementation task.

The objective and human directives support a first implementation slice: publish one selected local `TASK.md` to GitHub for inspection. The draft keeps bulk publication and queue draining out of scope.

## Evidence Used

- `TASK-CREATE-OBJECTIVE.md`
- `TASK-CREATE-CONTEXT-MANIFEST.md`
- `HUMAN-DIRECTIVES-FOR-WORKER.md`
- `TASK-AUDIT-0001.md`
- `TASK-DRAFT-0001-BEFORE-AUDIT.md`
- `TASK-AUDIT-0002.md`
- `TASK-DRAFT-0002-BEFORE-AUDIT.md`
- `TASK-AUDIT-0003.md`
- `TASK-DRAFT-0003-BEFORE-AUDIT.md`
- `TASK-AUDIT-0004.md`
- `TASK-DRAFT-0004-BEFORE-AUDIT.md`
- `TASK-AUDIT-0005.md`
- `TASK-DRAFT-0005-BEFORE-AUDIT.md`
- `TASK-AUDIT-0006.md`
- `TASK-DRAFT-0006-BEFORE-AUDIT.md`
- `TASK-AUDIT-0007.md`
- `TASK-DRAFT-0007-BEFORE-AUDIT.md`
- `TASK-AUDIT-0008.md`
- `TASK-DRAFT-0008-BEFORE-AUDIT.md`
- `Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md`
- `../Task-0008/TASK.md`
- `../Task-0008/PLAN.md`
- `../Task-0008/Design/DURABLE-EXECUTION-STATE-CONTRACT.md`
- `../Task-0009/TASK.md`
- `../../AGENTS.md`
- `../../DATA-HANDLING.md`
- `../../TESTING.md`
- `../../backend/orchestration/README.md`
- `../../backend/orchestration/internal/config/config.go`
- `../../backend/orchestration/internal/httpapi/mux.go`
- `../../backend/orchestration/internal/taskrun/types.go`
- selected `service.go` symbols found with `rg`
- shared `.codex` TaskCreate, Task Audit, task-state, and worktree standards named by the manifest

## Audit-Readiness Caveats

- Revision after `TASK-AUDIT-0001.md` addresses the auditor's required rewrites: durable mapping data handling, human inspection gate, broader `gh` failure contract, and blocked follow-ons after rejected issue shape.
- Revision after `TASK-AUDIT-0002.md` keeps the draft in burden-reduction concrete mode, adds `Human Relief If Successful` and `Remaining Uncertainty`, requires explicit human acceptance of the pilot issue shape, pins proof to the validation lane by default, and adds the exact approval question.
- Revision after `TASK-AUDIT-0003.md` removes schema-invalid review-pending status wording, requires schema-valid `TASK-STATE.json` fields plus `HANDOFF.md` for human review, adds remote/provenance validation with local-only provenance behavior, blocks readable closed mapped issues without duplicate/reopen behavior, and requires argv-safe body-file submission.
- Revision after non-conformant `TASK-AUDIT-0004.md` uses the coordinator-mediated concrete findings only: every `gh issue create/edit/view` must target the selected repo explicitly, both mapping-derived and explicit closed issue paths fail closed, fake-runner proof must capture exact repo-targeted argv, and the review package records authenticated host/login when available.
- Revision after `TASK-AUDIT-0005.md` adds marker-search duplicate recovery before create, defines partial-success recovery artifacts and rerun behavior, splits blocker/recovery proof from the pilot review package, and requires fake-runner proof for create-succeeds/readback-fails followed by no-duplicate rerun.
- Revision after `TASK-AUDIT-0006.md` requires supplied explicit issues to already contain the hidden marker before edit, blocks unrelated open issues, orders partial-success recovery by known candidate first and marker search second, adds marker-status fields, and requires fake-runner proof for unrelated explicit issues plus candidate-first recovery.
- Revision after `TASK-AUDIT-0007.md` makes human acceptance the publication-quality gate rather than a closure bypass, adds regression-disposition acceptance and proof requirements, adds execution inputs, and specifies the exact hidden stable marker format.
- Revision after `TASK-AUDIT-0008.md` makes fake-runner marker-search proof supporting coverage only, adds a visible machine-readable marker line, requires real non-mutating marker-discovery proof after authenticated publish/update, and blocks closure/follow-ons when production marker discovery cannot rediscover the pilot issue.
- The draft is still a writer draft awaiting rerun by a separate clean-context TaskCreate auditor. It is not agent-audited, approved, or enqueue-ready.
- The local `gh` auth fact is currently a known implementation blocker for real publish proof, so the draft requires fail-closed blocked behavior and does not allow fake publication output.
- The exact target GitHub repo and pilot task for the first real publish are execution inputs. The draft treats them as operator inputs, not task-boundary blockers.

## Blocking Questions

No blocking drafting questions remain that would change writeup type, implementation home, first-slice scope, enforcement boundary, acceptance criteria, or falsifier.
