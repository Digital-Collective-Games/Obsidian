# Task Audit 0007

Audit date: 2026-05-27

Audit path: TaskCreate clean-context auditor-worker

Reviewed draft:

- [TASK-DRAFT-0007-BEFORE-AUDIT.md](./TASK-DRAFT-0007-BEFORE-AUDIT.md)

Current draft path:

- [TASK.md](./TASK.md)

Audit standard:

- `C:\Users\gregs\.codex\Orchestration\Processes\TASK-AUDIT.md`

Auditor worker:

- `019e6c78-b0bf-7780-b5ae-edb77150f4b3`

## Intact Auditor Response

**Verdict**

`revise`

**Primary Findings**

1. The draft's terminal-closure language conflicts with the repo regression contract. The draft says the "only terminal closure path" is human acceptance of the pilot issue shape in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L21) and repeats that terminal closure is allowed when the review package records `accepted` in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L564). But [REGRESSION.md](/c:/Agent/CodexDashboard/REGRESSION.md#L7) says task-level regression starts from the real desktop app surface, and [REGRESSION.md](/c:/Agent/CodexDashboard/REGRESSION.md#L23) requires the matrix to be updated when current human-facing functionality changes. The draft creates a human/operator-facing publication and review flow, but its proof and closure path do not say how repo-root regression is satisfied, updated, or explicitly gated.

2. The implementation mechanism is otherwise concrete. The chosen boundary, backend home, `gh` fail-closed behavior, mapping artifact, marker/idempotency rules, recovery ordering, provenance validation, human review package, and nonterminal states are specific enough to implement and audit.

**Blocking Clarifying Questions**

- Is this task intended to add a new human-facing/operator publication flow that must be covered by a named repo-root regression case?
- If not, what durable repo-level authority permits terminal closure without satisfying or updating the [REGRESSION.md](/c:/Agent/CodexDashboard/REGRESSION.md#L7) task-level regression contract?

**Strengthening Clarifying Questions**

- Should the selected pilot repo/task id be captured in a dedicated "Execution Inputs" section so implementers do not treat them as design unknowns?
- Should the hidden stable marker format be specified exactly, or is "stable marker containing local task id and declared task root" intentionally implementation-defined?

**Required Rewrites**

- Replace the "only terminal closure path" wording with language that keeps human acceptance as the publication-quality gate but does not bypass repo-root regression/closure requirements.
- Add acceptance/proof language requiring either a repo-root regression matrix update for the GitHub publication/review operator flow or a durable, explicit reason why existing regression coverage remains sufficient.
- Add a proof-plan step that records the regression disposition before terminal closure, separate from the pilot issue review package.

**Optional Strengthening**

- Add a short "Execution Inputs" section for `github_repo`, pilot `task_id`, and auth timing.
- Specify the stable marker format if downstream recovery tests would otherwise invent incompatible marker shapes.

**Relevant Rule Or Exemplar**

- [TASK-AUDIT.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-AUDIT.md#L511): reject when acceptance proves a weaker proxy rather than mechanism success.
- [TASK-CREATE.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-CREATE.md#L568): concrete task acceptance must test the chosen solution.
- [REGRESSION.md](/c:/Agent/CodexDashboard/REGRESSION.md#L7): repo task-level regression starts from the real desktop app surface.
- [REGRESSION.md](/c:/Agent/CodexDashboard/REGRESSION.md#L23): new human-facing functionality requires matrix coverage.

## Coordinator Disposition

Status: revision required before rerunning clean audit.

Rewrite instructions to writer:

- Replace "only terminal closure path" wording so human acceptance is the publication-quality gate, not a bypass for repo-root regression and closure requirements.
- Add acceptance and proof-plan language requiring a regression disposition before terminal closure: either update `REGRESSION.md` with a named case for the GitHub publication/review operator flow, or record a durable explicit reason why existing regression coverage remains sufficient.
- Add a proof-plan step that records regression disposition separately from `PILOT-ISSUE-REVIEW-PACKAGE.md`.
- Add a short `Execution Inputs` section for `github_repo`, pilot `task_id`, and auth timing if it improves clarity without expanding scope.
- Specify the stable marker format if needed to prevent incompatible downstream recovery tests.
