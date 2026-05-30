# Task-0015 Handoff

## Current Baseline (2026-05-29)

- Phase: **implementation** — PLAN APPROVED by the human on 2026-05-29.
  Decision 2 settled: the PASS-0006 regression app surface is GitHub, driven via
  the human's Chrome debug tab on a single test issue in a NEW throwaway test repo
  under `C:\Agent` (see [HUMAN-DIRECTIVES-FOR-WORKER.md](./HUMAN-DIRECTIVES-FOR-WORKER.md)).
  Implementation begins at PASS-0000.
- The TaskDispatch task leader took Task-0015 from creation through Research and
  Planning; the coordinator ground-truthed the plan-gate package (see
  [Design/plan-gate/COORDINATOR-REVIEW.md](./Design/plan-gate/COORDINATOR-REVIEW.md),
  zero blocking discrepancies) and presented the gate; the human approved.
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

1. DONE: coordinator verified the plan-gate package and presented the gate; human
   APPROVED on 2026-05-29.
2. DONE: **PASS-0000** (O1 manifest rename → `REPO-MANIFEST.json` + per-repo
   `queue_workers`). A1.1–A1.4 PASS, F-O1 not falsified —
   [Testing/PASS-0000-AUDIT.md](./Testing/PASS-0000-AUDIT.md). Committed + pushed.
3. DONE: **PASS-0001** (O2 real N>1 per-repo slots). New `internal/queue` per-repo
   slot semaphore (limit = `queue_workers`, used = live owned-lane worktree count);
   the per-task `active_run_exists` 1:1 gate replaced by a per-repo slot check;
   `releasePreviousOwnedLane` untouched (cannot tear down siblings). Independent QA
   **re-ran the live proof from scratch** on the isolated validation lane against
   the throwaway `C:\Agent\QueueDrainTestbed` repo: A2.1 (HARD) two/three
   concurrent worktrees + live runs, A2.2 refuse-when-full + re-admit, A2.3 no
   `active_run_exists` with a free slot, AX.1 `go test ./...` clean, F-O2 not
   triggered — [Testing/PASS-0001-AUDIT.md](./Testing/PASS-0001-AUDIT.md) /
   [Testing/PASS-0001/](./Testing/PASS-0001/). Committed + pushed.
   NEXT: **PASS-0002**.
4. DONE: **PASS-0002** (O6 binding schema + `GET /api/v1/worktrees` + operator
   command). `RepoBinding` {repo, task, worktree path, session id, transcript path,
   run/gate state} persisted on the owned-lane record; endpoint enumerates active
   worktrees; `Get-ActiveWorktreeSessions.ps1` surfaces it. Independent QA
   re-derived live (fresh Task-12/13): A6.1 / A6.2 (HARD) / A6.3 PASS, F-O6
   boundary holds (no `vscodium://` link), AX.1 clean —
   [Testing/PASS-0002-AUDIT.md](./Testing/PASS-0002-AUDIT.md). Committed + pushed.
   Honest deferrals: A6.1 real-agent session → PASS-0005; A6.4 parked-listing → PASS-0003.
   NEXT: **PASS-0003**.
5. DONE: **PASS-0003** (O4 done-contract MECHANISM). `internal/queue/decision.go`
   `DecideQueueAction` (closed⇒terminal precedence; Human Needed=Yes⇒park, never
   redispatch; open+Ready+not-parked⇒dispatch) + `EffectiveFreeConcurrency`
   (limit−parked); parked run/gate enum + `Service.SetRunGateState` (parks without
   deallocating, persists to the owned-lane binding); agent done-contract
   `Set-TaskDoneContract.ps1` (sets Human Needed=Yes on abandon AND
   perceived-completion, NEVER `gh issue close`) + SKILL.md "Queue Done-Contract".
   Independent QA: A4.5/A4.6 PROVEN, decision/invariants/skill contract genuine,
   dry-run with a `gh` tripwire (gh never fired), O6 A6.4 re-proven at service +
   endpoint layers, `go test` clean —
   [Testing/PASS-0003-AUDIT.md](./Testing/PASS-0003-AUDIT.md). Committed + pushed.
   DEFERRED (honest, grep-confirmed not built): A4.3/A4.4 in-loop ⇒ PASS-0004 (O3);
   A4.1/A4.2/A4.7 agent-driven ⇒ PASS-0005 (O5).
   NEXT: **PASS-0004**.
6. DONE: **PASS-0004** (O3 GitHub queue-drain consumer). `internal/queue`
   provider interface + `Consumer.DrainOnce` (reuses O2 `EvaluateSlot` + O4
   `DecideQueueAction`; maps #N→Task-N; dispatches via the taskrun seam, no manual
   call; skips Never; parks on Human Needed=Yes; reclaims on closed) +
   `QueueDrainWorkflow` (~2min poll, stoppable) registered in `StartWorker` +
   `POST /api/v1/queue/consumer/{start,stop}`. Read-only against GitHub (A4.6).
   Independent QA re-ran tests + re-derived the live gh-read smoke with its own
   fresh issues (#3 Ready→dispatched, #4 Never→skipped); A3.1–A3.4 / A4.3/A4.4-loop
   / F-O3 PASS — [Testing/PASS-0004-AUDIT.md](./Testing/PASS-0004-AUDIT.md).
   Committed + pushed. The live attempt also fixed a real issue-field-values
   parse bug (value is a numeric option id). Honest deferral: the no-proxy
   real-GitHub-UI end-to-end A3.1 → PASS-0006.
   NEXT: **PASS-0005**.
7. PARTIAL: **PASS-0005** (O5 — largest/highest-risk). DETERMINISTIC SCOPE DONE +
   committed: the external liveness watchdog (`internal/queue/watchdog.go`) —
   signal-anchored detection (transcript size+mtime → `last_active_signal_at`,
   refreshed on append, never dispatch), FULL parked-suspension (re-anchors on
   un-park), exactly one poke via an injected seam, incident email (state +
   transcript) via an injected MOCK sink, configurable thresholds, long-build
   `ProcessBusy` guard — and the launcher + POST-LAUNCH session discovery
   (`internal/queue/launcher.go` + `Service.BindLaunchedSession`). Independent QA
   re-derived the watchdog live against a real transcript: A5.2/A5.4/A5.5 (HARD) +
   A5.3-watchdog-half PASS, F-O5-signal/parked not triggered, `go test` clean —
   [Testing/PASS-0005-AUDIT.md](./Testing/PASS-0005-AUDIT.md).
   **PENDING SUPERVISED (honestly deferred, not faked):** A5.1 (HARD — real
   top-level agent spawning its own subagent: cannot be made bounded-safe
   unattended) and the poke "actually wake" input-delivery (no mechanism exists;
   `DeliverWake`=false). See the PASS-0005 checklist blockers.
8. PENDING (NEEDS HUMAN): **PASS-0006** (regression) — the no-proxy real-GitHub-UI
   A3.1 end-to-end via the Chrome debug tab flipping `Queue=Ready` on a
   `QueueDrainTestbed` issue → backend pickup; add REG-007 to `REGRESSION.md`.
   - Live-proof hygiene: use fresh task ids; override `CODEX_ORCHESTRATION_JOBS_ROOT`.

## Where the unsupervised run stopped (2026-05-30)

5 passes fully done + independently verified + committed/pushed (O1, O2, O6, O4,
O3). PASS-0005 (O5) deterministic half done + committed; its HARD A5.1 + the
poke-wake mechanism are PENDING a SUPERVISED session (launching a real autonomous
agent + designing wake-input delivery — both unsafe/under-specified to run
unattended). PASS-0006 (regression) and task CLOSURE are human gates. The
remaining work is human-gated, not time-gated.
3. Before PASS-0001 proof: stand up the dedicated test repo under `C:\Agent`
   (confirm the GitHub repo name/org with the human — outward-facing) and add its
   `REPO-MANIFEST.json` entry.
4. Fold the COORDINATOR-REVIEW.md corrections into the affected passes (O2
   per-repo semaphore vs the per-task gate; O5 post-launch session discovery; O5
   wake-input mechanism).
5. Closure is a DISTINCT, FINAL explicit human gate — never self-close.

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
