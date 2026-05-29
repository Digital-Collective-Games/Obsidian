# Review Package — Task-0015 Plan Gate

Gate: PLAN approval (the explicit human plan gate; implementation is blocked
until this is approved). Task: Task-0015 — Temporal-backed GitHub queue-drain
consumer. Issue: [Obsidian #15](https://github.com/Digital-Collective-Games/Obsidian/issues/15).

Prepared by the TaskDispatch task leader (single-context; no nested
implementation-leader dispatch tool available in this runtime — see Caveats).
Follows `C:\Users\gregs\.codex\Orchestration\Processes\REVIEW-PACKAGES.md`.

## Exact Approval Question

Approve the 7-pass implementation plan and its provability-driven ordering
(O1 → O2 → O6 → O4 → O3 → O5 → cross-cutting) as safe and specific enough to
BEGIN implementation at PASS-0000?

Secondary decision bundled here: for PASS-0006, is adding a NEW operator-lane
regression case to `REGRESSION.md` (rather than forcing the new backend/operator
behavior into the app-surface-centric matrix) the right home? See "Open
Question" below.

## Primary Review Surface (read these directly)

- The plan: [../../PLAN.md](../../PLAN.md) — 7 passes, each with concrete
  changes, verification, exit bar, and falsifier guard, mapped 1:1 to O1–O6 +
  cross-cutting.
- Research synthesis: [../../RESEARCH.md](../../RESEARCH.md) — verified seam map
  (file:line) for all six sub-objectives.
- The embedded research item (the load-bearing KNOWN UNKNOWN), now PINNED:
  [../../Research/LIVENESS-SIGNAL.md](../../Research/LIVENESS-SIGNAL.md) — chosen
  O5 liveness signal, sampling, and why it separates "working" from "asleep."
- Authoritative scope / directives:
  [../../HUMAN-DIRECTIVES-FOR-WORKER.md](../../HUMAN-DIRECTIVES-FOR-WORKER.md).
- Task contract: [../../TASK.md](../../TASK.md).
- Machine state: [../../TASK-STATE.json](../../TASK-STATE.json).

## What Changed In This Iteration (plan/research artifacts authored)

- `RESEARCH-PLAN.md` — bounded decision-shaping problem list (P1–P7).
- `RESEARCH.md` — planning-ready synthesis; verified the O1–O6 seams against
  real code (incl. confirming the hard 1:1 gate at `service.go:777-781,138`,
  that the binding does NOT exist durably today, and that the current execution
  model does NOT launch a coding agent — so O5's top-level agent dispatch is
  net-new).
- `Research/LIVENESS-SIGNAL.md` — pinned the O5 signal from REAL transcript
  shapes inspected on this machine (Codex `~/.codex/sessions/.../rollout-*.jsonl`
  and Claude `~/.claude/projects/.../*.jsonl`, both append-only JSONL with
  per-line RFC3339 timestamps; Claude also writes per-subagent transcripts,
  evidence for A5.1).
- `PLAN.md` — the 7-pass plan under review.
- `TASK-STATE.json` — advanced to `phase: research` then (with this gate)
  `phase: planning`, `current_gate: planning`, `plan_approved: false`.

Nothing did NOT change in product code — NO implementation has started. Only
durable task artifacts (research/plan/state) were written. The task baseline
(`TASK.md` + creation artifacts) was committed because TaskCreate had left it
untracked.

## What Approval Authorizes / What Stays Forbidden

- Authorizes: starting PASS-0000 (manifest rename + `queue_workers`) and
  proceeding through the planned passes, each gated by its own pass closeout.
- Stays forbidden without further explicit human approval: task CLOSURE (a
  distinct final human gate — the agent never self-closes); any proof against
  the human live service lane/config/DB; ungated live GitHub writes; scope
  changes to O1–O6.

## Validation / Regression / Proof Status

- Build/unit/regression: `not_run` (planning phase; no code yet) — see
  [../../TASK-STATE.json](../../TASK-STATE.json).
- Go test command for implementation: `go test ./...` from
  `backend/orchestration` (AX.1).
- Isolated proof lane fixed: validation lane backend `:14318`, Temporal
  `:17233`, Postgres `15432` (REGRESSION.md:11-19, TESTING.md). Never the human
  service lane or live data unless explicitly authorized.

## Known Caveats / Residual Risk

- Single-context execution: no nested-subagent dispatch tool is available in
  this runtime, so research+planning were done directly by the task leader
  (honestly labeled in every artifact). Per-pass delegated implementers and a
  clean-context QA worker may not be creatable here; if not, the QA gate will be
  recorded as blocked/non-conformant and surfaced to the coordinator rather than
  satisfied by producer self-review.
- O5 is the largest, highest-risk pass (net-new top-level headless agent
  launcher + external watchdog + email). Its signal is pinned, but the launcher
  is new ground; expect it to need the most care and possibly a debug loop.
- The O5 signal relies on transcript flush latency being well under the ~5 min
  window (mitigated by the conservative, configurable window + process
  corroboration); see `LIVENESS-SIGNAL.md` Risks.

## Open Question Folded Into Approval (PASS-0006 regression home)

`REGRESSION.md` Canonical Rule makes the desktop APP surface the regression lane.
The new queue-drain behavior is backend/operator-facing, not a desktop screen.
The plan PROPOSES adding a NEW operator-lane regression case (e.g. REG-006) for
the queue-drain consumer incl. the simulated-stall incident email, exercised on
the isolated lane. Please confirm an operator-lane case is the right home (vs.
trying to force it into the app-surface matrix).

## Consequence Of Rejection / Revision

If rejected or revised, the task stays in planning; the leader revises `PLAN.md`
(and re-grounds research if scope shifts) and re-presents this gate. No
implementation begins until the plan is explicitly approved.
