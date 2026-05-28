# Task Audit 0008

Audit date: 2026-05-27

Audit path: TaskCreate clean-context auditor-worker

Reviewed draft:

- [TASK-DRAFT-0008-BEFORE-AUDIT.md](./TASK-DRAFT-0008-BEFORE-AUDIT.md)

Current draft path:

- [TASK.md](./TASK.md)

Audit standard:

- `C:\Users\gregs\.codex\Orchestration\Processes\TASK-AUDIT.md`

Auditor worker:

- `019e6c80-ff0b-7f83-b7a6-4871562e77e2`

## Intact Auditor Response

**Verdict**

`revise`

**Primary Findings**

1. Weak acceptance / proxy closure remains in duplicate prevention. The draft makes marker search mandatory before creation in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L541), but the proof bar still allows "marker-search output or fake-runner marker-search proof" in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L614). The durable research only says `gh issue list` supports search generally in [the research capture](/c:/Agent/CodexDashboard/Tracking/Task-0012/Research/2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md#L119); it does not prove the exact hidden HTML marker is discoverable through real GitHub issue search. A fake runner can satisfy the test while real GitHub search still fails to rediscover the issue, allowing duplicate creation after missing mapping or partial-success recovery.

2. The rest of the draft is materially concrete. Scope, backend home, source-of-truth split, fail-closed `gh` handling, mapping durability, human review gating, nonterminal state handling, provenance validation, closed issue handling, and regression disposition are specific enough to implement and audit cold.

**Blocking Clarifying Questions**

- Does real `gh`/GitHub issue search reliably rediscover a published issue by the exact hidden stable marker string, including the HTML comment delimiters and fields?
- If exact marker search is not reliable, what concrete duplicate-detection mechanism replaces it before any create path?
- Should terminal proof require real non-mutating marker-search/readback evidence after an authenticated publish or update?

**Strengthening Clarifying Questions**

- Should the issue body include a visible machine-readable marker line in addition to the hidden comment if GitHub search indexing is uncertain?
- Should the issue title formula be pinned more tightly than "selected title" so pilot issue shape is less implementation-dependent?

**Required Rewrites**

- Tighten the proof plan and acceptance criteria so fake-runner marker-search proof is supporting unit coverage only. When a real issue is published or updated, closure must include real `gh` or equivalent GitHub API evidence that the selected repo can rediscover that issue by the stable marker before any create path would run.
- Add a fallback requirement: if exact marker search cannot be proven against real GitHub, the implementation must block or use a specified safer discovery mechanism, not silently rely on the fake-runner path.
- Add a falsifier saying the task is incomplete if real duplicate-prevention discovery cannot find the published pilot issue by the mechanism the endpoint will use in production.

**Optional Strengthening**

- Add a dedicated "marker discovery proof" artifact under the task testing package, separate from the human issue-shape review package.
- Pin the renderer's title/body extraction fallback rules for tasks that lack goals, acceptance criteria, or summary sections.

**Relevant Rule Or Exemplar**

- [TASK-AUDIT.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-AUDIT.md#L315): acceptance must prove the chosen intervention worked.
- [TASK-AUDIT.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-AUDIT.md#L328): reject proxy closure paths.
- [TASK-AUDIT.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-AUDIT.md#L377): unknown external data/protocol behavior needs durable proof rather than implementation-time improvisation.
- [TASK-CREATE.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-CREATE.md#L488): when proof is easy to fake or misread, say what proof will count.

## Coordinator Disposition

Status: revision required before rerunning clean audit.

Rewrite instructions to writer:

- Tighten acceptance and proof language so fake-runner marker search is only supporting unit coverage.
- Require real non-mutating `gh` or GitHub API evidence after a real publish/update that the selected repo can rediscover the pilot issue by the exact stable marker mechanism the endpoint will use before any future create.
- Add a fallback/fail-closed requirement: if exact marker discovery cannot be proven against real GitHub, the task cannot close as accepted and must block further bulk publication or queue-draining follow-ons unless a specified safer discovery mechanism is implemented.
- Add a falsifier that the task is incomplete if real duplicate-prevention discovery cannot find the published pilot issue by the production mechanism.
- Add a dedicated marker-discovery proof artifact under task testing, separate from `PILOT-ISSUE-REVIEW-PACKAGE.md`.
- Prefer a visible machine-readable marker line in addition to any hidden comment if that makes discovery proof more reliable; specify the marker format if used.
