# Task-0015 Handoff

## Current Baseline (2026-05-29)

- Phase: **planning** — STOPPED at the explicit human PLAN gate. No
  implementation has started; no product code changed.
- The TaskDispatch task leader took Task-0015 from creation through Research and
  Planning. The plan is written and a plan-gate review package is ready for the
  human.
- Machine state: [TASK-STATE.json](./TASK-STATE.json) (`phase: planning`,
  `current_gate: planning`, `plan_approved: false`, blocker = awaiting human plan
  approval), schema-valid.

## Durable Artifacts Produced

- [RESEARCH-PLAN.md](./RESEARCH-PLAN.md) — bounded problem list (P1–P7).
- [RESEARCH.md](./RESEARCH.md) — planning-ready synthesis; verified O1–O6 seams
  from real code.
- [Research/LIVENESS-SIGNAL.md](./Research/LIVENESS-SIGNAL.md) — the embedded O5
  research item, COMPLETE: signal = bound-transcript append growth (size+mtime →
  `last_active_signal_at`), corroborated by process state, anchored to last
  activity (never dispatch time); pinned from real Codex+Claude transcript
  shapes.
- [PLAN.md](./PLAN.md) — 7-pass plan (O1→O2→O6→O4→O3→O5→cross-cutting), each pass
  with concrete changes, verification, exit bar, and falsifier guard.
- [Design/plan-gate/REVIEW-PACKAGE.md](./Design/plan-gate/REVIEW-PACKAGE.md) —
  the plan-gate review package (approval question + primary review surface +
  caveats + the PASS-0006 regression-home open question).
- Task baseline (`TASK.md` + creation artifacts) committed (TaskCreate had left
  it untracked).

## Next Step (explicit)

1. Coordinator verifies the plan-gate review package is complete, then presents
   the plan gate to the human for EXPLICIT approval.
2. On approval: begin **PASS-0000** (O1 manifest rename + `queue_workers`), then
   proceed through the planned passes, each closed with unit tests + pass audit +
   handoff + commit + push + toast, rotating implementation context per pass.
3. Closure is a DISTINCT, FINAL explicit human gate — never self-close.

## Active Watchouts / Blockers

- BLOCKER (intended): implementation is gated on explicit human PLAN approval.
- Two questions are folded into the plan gate (see review package): (a) approve
  the 7-pass ordering; (b) confirm a NEW operator-lane REGRESSION.md case is the
  right home for the backend/operator-facing queue-drain behavior (the existing
  matrix is desktop-app-surface-centric).
- O5 is the largest/highest-risk pass: the top-level headless agent launcher is
  net-new (the current execution model runs backend activities, not a coding
  agent). Expect the most care / possible debug loop there.

## Process Gaps (for the TaskDispatch coordinator; do NOT self-patch `.codex`)

- **No nested-subagent dispatch tool in this runtime.** The task leader could not
  dispatch `RESEARCH-LEADER`, `IMPLEMENTATION-LEADER`, per-pass implementers, or a
  clean-context QA worker. Research and planning were executed single-context
  (honestly labeled in every artifact, per the worker doc's Single-Context
  Fallback). The coordinator should decide whether to:
  - arrange a clean-context QA lane and delegated per-pass implementers in an
    environment that has dispatch tooling, or
  - explicitly accept single-context execution with the QA gate recorded as
    blocked/non-conformant rather than satisfied by producer self-review.
- No `.codex` process edits were made; this is reported for the coordinator who
  owns `.codex` process debt.

## Git

- Branch: `master` (repo convention; tracks `upstream/master`). Task-0015 files
  only were staged each commit.
- Checkpoints pushed to `upstream`:
  - `b560f4e` — research open / initial TASK-STATE.
  - `3440ca3` — research artifacts + task baseline.
  - planning checkpoint — this handoff + PLAN + review package + state (pushed
    with this commit).
