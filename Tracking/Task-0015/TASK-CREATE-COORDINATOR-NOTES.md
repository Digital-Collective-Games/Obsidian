# Task-0015 TaskCreate Coordinator Notes

## Process model

TaskCreate (2026-05-29 model): coordinator + blind writer-worker, with a
coordinator concreteness review (no separate agent-auditor lane). See
`C:\Users\gregs\.codex\Orchestration\Processes\TaskCreate\README.md`.

## Grounding / design pass

Before drafting, the coordinator ran two read-only fan-outs: (1) a state recon
confirming nothing consumes the `Queue` field today (drain-queue was deferred from
Task-0012) and that the backend already has worktree allocate/reclaim + Temporal +
poke/follow-up primitives, but is HARD 1:1 per repo; (2) a design pass producing
options for the architecture, done-contract, liveness watchdog, and slot model.
Those findings ground `TASK-CREATE-OBJECTIVE.md` and `TASK-CREATE-CONTEXT-MANIFEST.md`.

## Human decisions (load-bearing) — captured verbatim + normalized

The human gave the feature shape plus four decisions (D1–D4), recorded verbatim in
`HUMAN-DIRECTIVES-FOR-WORKER.md`:
- D1: real N>1 per-repo concurrency, default ~4.
- D2: the GitHub issue is the queryable state; CLOSED is the only terminal state
  (covers done and won't-do); `Human Needed=Yes` is the abandon/needs-human signal;
  an abandoning agent sets `Human Needed=Yes` and does NOT self-close.
- D3: dispatch a TOP-LEVEL agent that can spawn its own subagents (hard req — not a
  nested subagent), invisibly supervised.
- D4: detect "fell asleep" fast (~5 min) via an active-inference signal like the
  IDE "thinking" indicator; poke; on confirmed sleep log an incident + email the
  operator the observed state + session transcript.

## Writer draft

The blind writer-worker produced [TASK.md](./TASK.md) as a `concrete
implementation` task with one embedded research item (the O5 liveness signal),
five internally-separable sub-objectives (O1 manifest rename + `queue_workers`,
O2 real N>1 concurrency, O3 Temporal queue-drain consumer, O4 issue-state
done-contract, O5 liveness watchdog + incident email), each with pass/fail
acceptance criteria and a falsifier. Grounded in named backend file:line seams and
the operator-skill surfaces. Fuzziness check passed.

## Coordinator review — concreteness change made (scope preserved)

One concreteness pin (increases specificity; no scope change):

1. **Liveness deadline must be SIGNAL-REFRESHED (ambiguous mechanism → pinned).**
   The watchdog reuses `SuspiciousAfter`/`staleRunUpdate`, which is a deadline.
   Pinned that the deadline must be refreshed by the active-inference signal (track
   last-active-signal time; reset on activity), so "asleep" = "no active signal for
   ~5 min", NOT "~5 min since dispatch." A dispatch-anchored timer is exactly the
   fixed-timer "feels-alive" proxy `F-O5-signal` falsifies. Edit applied directly to
   `## Proposed Changes` → O5 (re-launching the writer would add no value).

Everything else already satisfied the concreteness/specificity lens
(`TASK-AUDIT.md`): named home + seams, pinned mechanism, pass/fail acceptance, a
dedicated falsifier for each of the five hard requirements (N>1 real concurrency;
top-level agent can spawn subagents; abandon sets `Human Needed=Yes` and does not
self-close; stall detected within window + incident email with state+transcript;
liveness signal pinned in a research artifact), explicit `What Does Not Count`,
honest isolation/proof constraints, and a clear Internal Mechanism Map + Why-One-Task.

## Scope integrity

No sub-objective was split, narrowed, re-sequenced, or removed. All five remain.
The task notes internal separability and offers staging only as a human-gate note,
not a unilateral split (per the human directive).

## Provider-binding gate (PENDING)

Per `TASK-CREATE.md` Task-Provider Binding Gate + the `obsidian-operator` skill:
NOT created/ready/enqueue-ready until GitHub issue exists at the matching number
and `Tracking/Task-0015/TASK-META.json` binds it (issue-first, number == task
number, type `Task`). Provisional id Task-0015 (latest is Task-0014 → issue #14;
zero PRs, so #15 expected). Verify the next issue number at bind time. Creating the
issue is an outward-facing write, gated on explicit human approval of this draft.

## Revision round (2026-05-29) — draft v2

The human reviewed draft v1 and gave three revisions (preserved verbatim in
`HUMAN-DIRECTIVES-FOR-WORKER.md`, "revisions" section). The coordinator preserved
the feedback, re-dispatched the blind writer-worker with process-safe revision
instructions, and re-reviewed:

1. **O4 — `Human Needed=Yes` PARKS in place** (retain worktree + slot, no
   redispatch, resumable in same worktree); ONLY a CLOSED issue deallocates.
   Rationale captured: re-provisioning is expensive (esp. Unreal Engine) and the
   research/plan/regression gates are normal pauses that must not be bumped.
   Integrated across Summary/Outcome/Goals/Proposed Changes/Acceptance
   (A4.3, A4.4, A4.5 — HARD)/Falsifier (F-O4-park — HARD)/What-Does-Not-Count/Map.
2. **O5 — watchdog SUSPENDED while parked on a human gate** (no stall/poke/incident
   for a gate-parked run; distinguish parked from asleep). New A5.5 (HARD) +
   F-O5-parked (HARD).
3. **O6 (new sub-objective) — worktree↔session binding** on the owned-lane record +
   a backend `GET /api/v1/worktrees` enumeration endpoint + an `obsidian-operator`
   command, so the operator can open the right session in VSCodium to kick a
   parked/slow agent along. New A6.1–A6.4 + F-O6 (HARD). The Obsidian review-surface
   UI is explicitly the human's SIBLING task (Non-Goals), not this one.

Coordinator re-review verdict: revisions faithfully integrated; six sub-objectives
coherent; no scope narrowed; the v1 concreteness pin (signal-refreshed liveness
deadline) preserved. No further concreteness gap found. The earlier provisional
slot-free-on-needs-human behavior is fully superseded by park-in-place.

## Revision round (2026-05-29) — draft v3 (closure tightening)

The human added a closure constraint (preserved verbatim in
`HUMAN-DIRECTIVES-FOR-WORKER.md`, "closure tightening" section). Coordinator
preserved it, re-dispatched the writer, and re-reviewed:

1. **Agent NEVER self-closes; closure = EXPLICIT human approval.** The agent may
   never close the issue — not on abandon, not on perceived completion. On
   believed-completion it sets `Human Needed=Yes` with run/gate state "awaiting
   closure approval" and PARKS (worktree+slot retained, watchdog suspended,
   enumerable); the issue is CLOSED only by an explicit human closure approval
   (human directly or the human-gated obsidian-operator close path).
2. **Non-closure gate approvals are NOT closure approval.** research / plan / pass
   / regression-run approvals never authorize closing; closure is a distinct final
   gate. The consumer must NOT auto-close from a local "complete"/terminal status —
   this intentionally tightens the existing close-from-local-status behavior for
   the queue-driven path.
3. Integrated across Title/Summary/Outcome/Goals O4/Proposed Changes O4/Expected
   Resolution/Acceptance (A4.2 rewritten; new **A4.7 HARD**)/What-Does-Not-Count/
   Falsifier (**F-O4-closure HARD**)/Map. Park semantics ("awaiting closure
   approval" = Human-Needed park) stay consistent with O5 suspension and O6
   enumeration.
4. **Deferred Non-Goal added:** voluntary agent worktree-release (no closure) is
   explicitly out of scope now, per the human ("don't complicate matters now").

Re-review verdict: faithfully integrated; no scope narrowed; no stale "agent
closes on success" text; all O4 hard requirements paired with falsifiers. Only a
human-approved CLOSE deallocates a worktree.

## Coordinator concreteness edit — O6 orchestrator boundary (2026-05-29)

Confirmed via the backend routes (`httpapi/mux.go`: health/jobs/tasks/task-runs/
webhooks/runs/sync) that NO sessions/worktrees endpoint exists today; O6 is the
net-new vehicle. Human clarified the boundary: the endpoint should supply the
fields needed to CONSTRUCT a VSCodium "talk to the agent" link (worktree path/cwd
+ session id + transcript path), but must NOT emit the link itself — link
construction is a consumer concern (obsidian-operator command / future review
surface), not an orchestrator concern. Applied a small concreteness bullet to
`## Proposed Changes` → O6 stating this boundary and that the exact resume-link
form (and any feasibility research) lives on the consumer side. Acceptance already
aligned (A6.2 returns fields, not a link); no scope change.

## Status

Coordinator-reviewed for concreteness (through draft v3 + the O6 boundary edit),
awaiting direct human approval. Note: this is a large feature; the six sub-objectives are independently
shippable if the human later wants to stage dispatch (raised as a note, not a
narrowing). Provider-binding gate (issue #15 + TASK-META) still PENDING and gated
on human approval.
