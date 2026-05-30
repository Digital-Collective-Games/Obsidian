# Coordinator Self-Review — Task-0015 Plan Gate

Author: TaskDispatch coordinator (this run). Companion to the leader-authored
[REVIEW-PACKAGE.md](./REVIEW-PACKAGE.md). This is the coordinator's own
inspection required before presenting the plan gate to the human
(`TASK-DISPATCH-COORDINATOR.md` "Coordinator Review Packet" / "Claim
Verification"). It does NOT replace the leader's package or change scope.

## Method

The leader's plan rests on an extensive file:line seam map and "evidence-based"
claims. Rather than accept those as self-proving, the coordinator ran a
5-dimension verification that opened the REAL repo files and machine artifacts:
(1) backend Go seam map, (2) manifest + obsidian-operator seams incl. a
completeness grep, (3) the O5 liveness-signal empirical claims, (4)
acceptance-criteria coverage + verbatim-directive faithfulness + scope integrity
+ the REGRESSION.md open-question legitimacy, (5) adversarial risk hunt.

## Verdict: APPROVABLE — no blocking discrepancies

| Dimension | Result | Notes |
| --- | --- | --- |
| Backend Go seam map | sound | 14/15 claims verified exactly; 1 partial (`service.go:805` is `FirstSuspiciousAfter`, a pre-dispatch window — correct line/value, loose label). O5-is-net-new **confirmed**: no codex/claude launch anywhere in the dispatch/execution path. |
| Manifest + skill seams | sound | All 12 refs verified at cited lines; **completeness grep confirms the rename list is exhaustive — no missed live-code ref.** Field-write + close/reopen builders confirmed. |
| Liveness signal | sound_with_caveats | Claude + Codex append-only JSONL transcripts (and per-subagent transcripts) **really exist on this machine**; chosen size+mtime→`last_active_signal_at` signal is sound and correctly anchored to last activity (honors F-O5-signal). |
| Coverage + directives | sound | All 27 criteria (A1.1–AX.2) map 1:1 to passes with verification + exit bar; every HARD marker + falsifier carried forward; every verbatim directive honored; scope intact; REGRESSION.md open question legitimate. |
| Adversarial risk | sound_with_caveats | Real implementation-time risks (below); none a gate blocker. |

## Corrections to FOLD INTO the plan (substantive; not blockers)

These are accurate-against-code refinements the implementation passes must adopt;
the plan as written would otherwise mislead the implementer. Recommend the leader
patch `PLAN.md` (and the affected pass) on/after approval.

1. **Launched-agent session discovery is net-new — the plan names the wrong
   source (O5/O6).** [PLAN.md](../../PLAN.md) PASS-0002 says to source the
   binding's session id / transcript path by reusing the existing
   `DeepContext.SessionID`/`TranscriptPath` plumbing. That plumbing
   (`captureDispatchContext`, `backend/orchestration/internal/taskrun/service.go:1485-1495`,
   copied in `buildDeepContext`, `backend/orchestration/internal/taskexec/taskexec.go:388-393`)
   reads the **backend process's** `CODEX_SESSION_ID`/`CODEX_TRANSCRIPT_PATH`
   env — i.e. whoever launched the control plane — **not** a freshly launched
   agent, whose own session id/rollout file does not exist until after launch.
   PASS-0005 must add a **post-launch session-discovery** step (detect the new
   agent's emitted session metadata / new `rollout-*.jsonl` or
   `<session>.jsonl`). O5's transcript-growth signal is useless without the
   correct per-agent transcript path, so this is the **highest-risk net-new
   work** (already flagged in [HANDOFF.md](../../HANDOFF.md)). Recommend a
   feasibility spike inside PASS-0005 **before** the watchdog half.

2. **"Poke must actually wake the process (deliver input)" has no existing
   mechanism (O5).** `PokeRun` (`backend/orchestration/internal/taskrun/service.go:226-255`)
   only mutates run state + writes a `poke_worker_check` follow-up; it delivers
   no input. The only existing agent launcher (`codex exec` in
   `backend/orchestration/internal/jobexec/jobexec.go:135-141`) is
   non-interactive with no stdin. Nothing today can inject input into a running
   headless codex/claude process. Guard **A5.3** against being satisfied by a
   no-op state-only poke (that would narrow the human's directive — Human-Facing
   Outcome rule). Record "how to deliver a wake input" as an explicit
   implementation-research item for PASS-0005.

3. **O2 seam description targets the wrong enforcement point.**
   [PLAN.md](../../PLAN.md) PASS-0001 says to change `deriveDispatchReadiness`
   from "no active owned lane for this repo" to "fewer than `queue_workers`."
   But `active_run_exists` is computed **per-task** via `GetActiveTaskRun(taskID)`
   (`backend/orchestration/internal/taskrun/service.go:563,593,1423-1425`);
   sibling queue lanes are distinct `Task-N` and would not collide there, and
   `releasePreviousOwnedLane` (`service.go:968-986`) also acts per-task. The true
   net-new work is a **per-repo slot count/semaphore that does not exist
   anywhere**. Correct the seam description before PASS-0001 so the implementer
   builds per-repo counting, not an edit to a per-task gate.

## Caveats to surface at the gate (accepted risks / reading guidance)

- **O6-before-O4/O5 ordering (PASS-0002).** A6.1 ("binding populated, not empty
  placeholders", HARD) and A6.4 ("parked worktree listed", HARD) cannot be
  genuinely satisfied at PASS-0002 — real agent sessions (O5) and parked state
  (O4) don't exist yet. PASS-0002 should scope only the binding **schema +
  endpoint** and explicitly **defer** A6.1 real-session population to PASS-0005
  and A6.4 parked-state listing to PASS-0003 re-proof, rather than HARD-passing
  them early against stubs.
- **Incident-email "session transcript" (A5.3).** Read as "tail last N events
  inline + link/attach the full transcript" (whose path the binding records) —
  not a literal multi-MB inline dump.
- **Pending-queue semantics (A2.2)** are thin but bounded. Acceptable to satisfy
  via "consumer declines while full and re-picks the Ready issue on the next poll
  after a close frees a slot," since GitHub issues ARE the durable queue; a
  bespoke durable pending-queue object may be unnecessary.
- **Gmail isolation.** [PLAN.md](../../PLAN.md):236-237 says the incident email
  send "may be" mocked. Tighten to **MUST** be mocked/captured during proof — the
  watchdog samples real `~/.codex` / `~/.claude` transcripts and targets the real
  `admin@digitalcollective.games`, so a stall repro must not emit a real email.
- **Long quiet tool call.** The C2 process/child-busy corroboration that prevents
  a false "asleep" during a multi-minute build is kept *optional* in PASS-0005.
  Make "do not declare sleep while the agent process has a live busy child / is
  CPU-busy" a **required** guard, or set the default stall window comfortably
  above expected tool-call duration.
- **Clean-context QA is AVAILABLE now (stale hedge).** [PLAN.md](../../PLAN.md)
  and [HANDOFF.md](../../HANDOFF.md) hedge that per-pass implementers / a
  clean-context QA worker "may not be creatable." The PRIOR (Codex) runtime
  lacked nested dispatch; the CURRENT runtime (this Claude Code harness) **does**
  support subagent dispatch. The implementation phase SHOULD use per-pass
  implementers + a clean-context QA worker, and must NOT accept producer
  self-review as the QA gate.
- **REGRESSION.md case id collision.** [PLAN.md](../../PLAN.md):264 proposes
  "REG-006 or similar," but `REGRESSION.md` REG-006 is already the Task-0014
  Tab-Aware Overlay Resize case. Use the next free id (REG-007).
- **Poll cadence in plan text.** PASS-0004 plan text omits the human-confirmed
  ~2-min default poll cadence and the explicit no-webhooks/no-tunnel exclusion
  (both fully captured in [HUMAN-DIRECTIVES-FOR-WORKER.md](../../HUMAN-DIRECTIVES-FOR-WORKER.md)).
  Carry them forward into PASS-0004.

## The two human-gate decisions (unchanged from the leader's package)

1. Approve the 7-pass plan + provability-driven order (O1→O2→O6→O4→O3→O5→
   cross-cutting) to begin implementation at PASS-0000?
2. Confirm a NEW operator-lane regression case in `REGRESSION.md` (vs. forcing
   the backend/operator queue-drain behavior into the desktop-app-surface matrix)
   is the right home for PASS-0006? (Confirmed legitimate: `REGRESSION.md`
   Canonical Rule makes the desktop app surface the lane; REG-001…REG-006 are all
   app-surface cases.)

## Provenance

Verification run: workflow `task0015-plan-gate-review` (5 parallel agents, real
file/machine inspection). All four code/seam dimensions returned **no blocking
discrepancies**; the two `sound_with_caveats` verdicts are the
implementation-time risks recorded above.
