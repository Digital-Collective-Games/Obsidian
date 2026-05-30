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
   **UPDATE 2026-05-30 — claude-only launcher (committed): A5.1 + poke-wake SOLVED.**
   Per the "dispatch CLAUDE only" directive, the launcher is now claude-only (codex
   path removed). **A5.1 (HARD) PROVEN**: a real bounded top-level `claude -p`
   launched via the launcher spawned its own subagent (subagent transcript present);
   session id set up front → deterministic transcript path. **Poke-wake REAL**:
   `DeliverWake` = `claude --resume <id> -p "<nudge>"`, QA-confirmed to append to the
   same transcript. Independent QA re-derived both. Remaining for FULL O5 closure:
   (a) wire the launcher + watchdog into the live consumer dispatch path; (b) a full
   integrated real-agent stall→detect→resume-poke→email repro. See PASS-0005 checklist.
8. PARTIAL / BLOCKED: **PASS-0006 (REG-007)** — the no-proxy real-GitHub-UI A3.1
   end-to-end. The backend half is BUILT + VALIDATED and reproducible (see the
   REG-007 status section below); the real-UI flip is BLOCKED on a workflow/config
   question (the `Queue` org field has no editable control in the testbed issue UI).

## REG-007 status (2026-05-30, supervised) — backend READY, real-UI flip BLOCKED

Human decisions this session: dispatch CLAUDE only (done, committed); REG-007 =
full chain (notice → dispatch → **launch the claude agent**); the human chose
"I (agent) drive the Queue→Ready flip via CDP".

Backend harness VALIDATED + reproducible (all isolated, no live lane / no real jobs):
- Built `cp-reg007.exe` from `cmd/controlplane`; ran it launch-enabled
  (`CODEX_ORCHESTRATION_QUEUE_LAUNCH_AGENT=true`,
  `QUEUE_AGENT_ALLOWED_TOOLS=Read,Write,Edit`,
  `CLAUDE_BIN=...vscode-oss...claude.exe`) bound `127.0.0.1:14318`.
- Temporal isolation: the validation `default` namespace on `:17233` is
  CONTAMINATED with orphan REAL job executions (a prior proof reconciled the real
  `.codex` jobs in) — a worker there would execute real digests. Worked around by
  creating + running in a FRESH `reg007` Temporal namespace (`temporal operator
  namespace create -n reg007 --retention 24h --address 127.0.0.1:17233`). `/healthz`
  ok, `job_count:0`. (Flag for the human: the validation `default` namespace should
  be cleaned of orphan real-job runs.)
- Consumer started: `POST /api/v1/queue/consumer/start` {repo:QueueDrainTestbed,
  poll_interval_seconds:20} → `workflow_id queue-drain--consumer`, status started.
  `GET /api/v1/worktrees` empty (no dispatch; #5 Queue unset) — correct pre-state.
- Test issue: QueueDrainTestbed **#5** (type set to Task via REST JSON body) +
  `Tracking/Task-0005/TASK.md` (trivial bounded "create AGENT-RAN.txt") committed to
  the testbed; #1/#2 closed so #5 is the only active issue.

BLOCKER (needs human): the org `Queue` field (`visibility: all`, options Never/Ready)
is settable via the `issue-field-values` API but renders **no editable control in
the testbed issue UI** (CDP nav shows only a ~1.9KB skeleton; the human confirmed
"there is no Queue field for #5"). GitHub org issue-fields appear API/Project-
surfaced here, not an issue-page sidebar control. So a real-UI flip on the testbed
issue is not possible as assumed. OPEN QUESTION for the human: in the real workflow,
how is `Queue=Ready` set — (a) an issue-sidebar field (then enable/repair it for the
testbed repo), (b) a GitHub Project board field (then REG-007's "UI" = the Project,
add #5 to it), or (c) the `obsidian-operator` skill (the operator's normalized
write surface — if so the skill-driven flip is the legitimate operator action, not a
"proxy")? This determines REG-007's real app surface.

Background backend STOPPED at compaction (restartable with the env above); fresh
task ids + `JOBS_ROOT` override + the `reg007` namespace are the live-proof hygiene.

## REG-007 RESOLVED — PASS (2026-05-30, supervised)

The Queue-field-UI blocker was resolved: the org `Queue` field renders in the issue
sidebar once the issue has a TYPE **and an initial field VALUE** (the
obsidian-operator Sync sets both; I had only set the type). The flip is driven at
the real GitHub web UI via the Chrome debug session (CDP) — centralized in the new
**`github-operator` skill** (`Set-IssueFieldViaUi.ps1` clicks the `Edit <Field>`
control + the option; `Get-IssueQueueState.ps1` reads state). Policy codified in
`TESTING.md` ("Issue-Provider Integration Testing"): the agent drives the provider
UI end-to-end (human only authenticates); no pass/excuse to skip end-to-end.

**REG-007 end-to-end PASS** (isolated `reg007b` namespace, throwaway
`QueueDrainTestbed`, launch-enabled consumer @30s poll): agent-driven real-UI
`Queue=Ready` flip → consumer `dispatched [Task-0005]` (≤30s) → worktree + O6
binding → the launched top-level **claude** agent ran (transcript at the bound
path, `AGENT-RAN.txt`). See [Testing/PASS-0006/REG-007-PROOF.md](./Testing/PASS-0006/REG-007-PROOF.md),
[Testing/PASS-0006-AUDIT.md](./Testing/PASS-0006-AUDIT.md), [Testing/PASS-0006-CHECKLIST.json](./Testing/PASS-0006-CHECKLIST.json).

**Defect found + fixed by the live regression — [BUG-0001](./BUG-0001.md):** the
launcher used `exec.CommandContext(<dispatch-activity ctx>)`, so the agent was
killed when the poll activity returned. Fixed: launch under a detached context.
Consumer poll default set to 30s (always-on, ≤1-min notice). Independently QA'd.

## Task status (2026-05-30)

ALL sub-objectives O1–O6 + cross-cutting are implemented + proven, each
independently QA'd, committed + pushed: O1 (PASS-0000), O2 (PASS-0001), O6
(PASS-0002), O4 (PASS-0003), O3 (PASS-0004), O5 (PASS-0005: claude-only launcher,
A5.1, real `claude --resume` poke-wake, launcher+watchdog wired into dispatch),
cross-cutting REG-007 (PASS-0006, real-GitHub-UI end-to-end). BUG-0001 fixed.

Remaining: (a) O5 hardening follow-ups (orphan-agent lifecycle: kill on
reclaim/shutdown + launcher logging — BUG-0001 follow-up; and a full integrated
real-agent stall→incident-email repro — components proven); (b) **task CLOSURE is a
distinct, final human gate — the agent never self-closes; awaiting explicit human
closure approval.**

## Reopened O3 — registry-driven binding (2026-05-30)

The human flagged a real gap: the backend bound to a repo via env (`WORKTREE_ROOT` +
`QUEUE_DRAIN_REPO`) + a co-located/default `queue_workers`, not via the registry.
Fixed: the queue-drain consumer is now **registry-driven** — it reads the central
`REPO-MANIFEST.json` at **`OBSIDIAN_REGISTRY_PATH`** (default the backend repo-root
manifest), enumerates ALL registered repos each poll (global awareness), and per repo
polls that entry's **`task_provider.repo`** (first-class; `QUEUE_DRAIN_REPO` no longer
the consumer's source), maps `#N → <local_root>/Tracking/Task-N`, and dispatches into
that entry's `local_root` capped at its `queue_workers` (per-repo slot count via
`taskrun.NewServiceForRepo`; `git worktree list` is per-repo). `task_provider` +
`source_control_provider` are first-class decoded types; `local_root` is arbitrary
(no location assumption). Legacy manual `/dispatch` (`WORKTREE_ROOT`) path intact;
production `REPO-MANIFEST.json` untouched; new env uses the `OBSIDIAN_` prefix.

Built + verified via a workflow (1 implementer + 3 independent verifiers, all
**sound**, zero blocking; `go build/vet/test ./...` green). **Live registry-driven
REG-007 re-run — PASS** (committed): backend bound to the testbed PURELY via the
registry (`OBSIDIAN_REGISTRY_PATH` → testbed-only registry; **no `QUEUE_DRAIN_REPO`**),
isolated `reg007c`; real-UI `Queue=Ready` flip (github-operator skill) →
`dispatched [Task-0005]` → binding `repo: QueueDrainTestbed` into the registry's
`local_root` → the launched claude agent ran (transcript 54KB, `AGENT-RAN.txt`).
Production Obsidian never polled; real cron namespace untouched. See
[Testing/PASS-0007-AUDIT.md](./Testing/PASS-0007-AUDIT.md) /
[Testing/PASS-0007/evidence/](./Testing/PASS-0007/evidence/). The repo-binding gap is
closed. Cosmetic follow-ups: drop the unused `StartWorker` taskService param +
decorative `QueueDrainConfig.Repo`.

**REG-007 coverage extended (2026-05-30):** the regression now also exercises — live,
registry-driven, `queue_workers=2` — the consumer-driven **concurrency/queuing** (3
Ready → 2 dispatched, 1 deferred), **worktree release on close + dequeue-next** (close
#5 → `reclaimed [Task-0005]` + `dispatched [Task-0007]`), and **park-retains** (#6
`Human Needed=Yes` → `parked [Task-0006]`, worktree retained, not redispatched).
The `REGRESSION.md` REG-007 case now lists these as required sub-scenarios. Proof:
[Testing/PASS-0006/REG-007-CONCURRENCY-RELEASE-PROOF.md](./Testing/PASS-0006/REG-007-CONCURRENCY-RELEASE-PROOF.md).

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
