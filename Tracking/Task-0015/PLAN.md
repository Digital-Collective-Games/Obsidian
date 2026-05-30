# Task-0015 Plan — Temporal-backed GitHub queue-drain consumer

Status: AWAITING EXPLICIT HUMAN PLAN APPROVAL. No implementation has started.

Grounded in [TASK.md](./TASK.md), [RESEARCH.md](./RESEARCH.md),
[Research/LIVENESS-SIGNAL.md](./Research/LIVENESS-SIGNAL.md), and
[HUMAN-DIRECTIVES-FOR-WORKER.md](./HUMAN-DIRECTIVES-FOR-WORKER.md).

Single-context note: planning was performed directly by the TaskDispatch task
leader because no nested implementation-leader dispatch tool was available in
this runtime; the same planning discipline and the explicit human plan gate are
honored. The same limitation means a clean-context QA lane and per-pass
delegated implementers may not be creatable from here — flagged to the
coordinator (see [HANDOFF.md](./HANDOFF.md), "Process gaps").

## Scope Discipline

- The six sub-objectives O1–O6 are ONE feature and are NOT split, narrowed,
  broadened, or re-sequenced on preference (`TASK.md` Writeup Type; directives
  "Scope / Process Notes"). The passes below are an EXECUTION ORDER for one
  coherent feature, chosen so each pass is independently provable per the task's
  own "Internal separability" note — not a scope split.
- Each pass maps to its sub-objective's acceptance criteria and falsifier.
- Closure is a DISTINCT, FINAL human gate; the agent never self-closes (this
  applies to how THIS feature behaves, and is honored at this task's own
  closure too).

## Pass Order And Dependencies

The dependency chain from `TASK.md` "Why One Task": O1 (config) → O2 (slots) →
O3 (consumer) needs slots; O4 (done-contract) defines when slots free/park; O6
(binding) is read by O5 (watchdog); O5 needs top-level agent dispatch. Chosen
order maximizes independent provability:

- PASS-0000 → O1 (manifest rename + `queue_workers`) — smallest, unblocks slot sizing.
- PASS-0001 → O2 (real N>1 per-repo slots) — provable as two concurrent worktrees before any GitHub polling.
- PASS-0002 → O6 (binding fields on the owned-lane record + `GET /api/v1/worktrees` + operator command) — binding is a prerequisite the O5 watchdog reads; provable against one dispatched + one parked worktree.
- PASS-0003 → O4 (done-contract: park-in-place, human-only closure, no second write path) — provable by driving each issue transition.
- PASS-0004 → O3 (GitHub queue-drain consumer workflow + start/stop endpoint + StartWorker registration) — provable against a fixture issue set once slots (O2) exist.
- PASS-0005 → O5 (top-level agent dispatch + external liveness watchdog + parked-suspension + one poke + incident email) — largest; reads O6 binding; provable with controlled stall and gate-parked repros.
- PASS-0006 → Cross-cutting (named REGRESSION.md case; `go test ./...`; task-level regression run) — closes the feature for regression.

Rationale for O6 before O4/O3/O5: the binding record (O6) is the shared
run/gate-state home that O4 (park vs running, which gate), O5 (transcript path +
parked flag), and the endpoint all read. Building it early avoids reworking the
owned-lane record in three later passes. O4 before O3 so the consumer's
slot-free/park decisions (which O3 dispatch depends on) are defined first. This
ordering is provability-driven, not a narrowing of scope; all six ship in this
one task.

Each closed pass: implement → unit tests → pass audit under `Testing/` →
HANDOFF update → commit → push → final toast (per ORCHESTRATION.md Required
Sequence). Rotate to a fresh implementation context per pass where the runtime
allows.

---

## PASS-0000 — O1: Manifest rename + `queue_workers`

Objective: Rename `CODEX-REPO-MANIFEST.json` → `REPO-MANIFEST.json` at repo root;
add a per-repo integer `queue_workers` (default `4`) to each `repos[]` entry;
update ONLY the verified live-code references.

Concrete changes (all verified present, RESEARCH.md §O1):
- `git mv CODEX-REPO-MANIFEST.json REPO-MANIFEST.json`; add `"queue_workers": 4`
  to the `CodexDashboard` repo entry.
- Update filename references: `skills/obsidian-operator/SKILL.md:3,10`;
  `skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1:4`;
  `Sync-TaskToGitHubIssue.ps1:6`; `Reconcile-TaskGitHubState.ps1:4`;
  `tests/test_obsidian_title_roundtrip.py:110,154`;
  `app/codex_dashboard/paths.py:52` (comment only; `id` stays `CodexDashboard`);
  `DATA-HANDLING.md:20,161,239`.
- Do NOT edit `Tracking/Task-0012/*`, `Tracking/Task-0013/*`, or `.codex` sessions.

Verification: `python -m unittest tests.test_obsidian_title_roundtrip` (A1.3);
a guard search proving no live-code ref still resolves to the old name and no
historical artifact changed (A1.2, A1.4); JSON parse + `queue_workers` integer
present (A1.1).

Exit bar / acceptance: A1.1–A1.4. Falsifier guard: F-O1 (old name still resolves
in live code, OR a `repos[]` entry lacks integer `queue_workers`, OR a
historical artifact was rewritten).

## PASS-0001 — O2: Real N>1 per-repo concurrency (slots)

Objective: Allow up to `queue_workers` concurrent owned lanes per repo; reuse
`provisionOwnedLane`/`bootstrapOwnedLane`/`cleanupOwnedLane` unchanged per slot.

Concrete changes:
- Introduce a per-repo slot mechanism in a new
  `backend/orchestration/internal/queue` package (recommended:
  `RepoSlotManagerWorkflow` holding `{repo → {used, limit}}` + a pending queue,
  OR an equivalent durable slot counter the consumer consults). `limit` =
  repo `queue_workers`. (Temporal-object choice left open per `TASK.md`; behavior
  fixed by acceptance.)
- Change the dispatch gate in `deriveDispatchReadiness` (`service.go:777-781`)
  from "no active owned lane for this repo" to "fewer than `queue_workers`
  active owned lanes for this repo."
- Stop `releasePreviousOwnedLane` (`service.go:138,968-986`) from tearing down a
  same-repo SIBLING that legitimately holds its own slot (it must still release a
  truly superseded lane for the SAME task, not a sibling).

Verification: a Testing artifact capturing TWO concurrent `git worktree`
checkouts under the owned-lane root + two live runs for one repo (A2.1); a 5th
dispatch at `queue_workers=4` refused/queued then dispatched after a slot frees
(A2.2); `deriveDispatchReadiness` no longer emits `active_run_exists` while a
free slot remains (A2.3); `go test ./...`. Isolated lane only.

Exit bar / acceptance: A2.1 (HARD), A2.2, A2.3. Falsifier guard: F-O2 (cannot run
>1 worktree for one repo at once, OR gate still blocks a same-repo dispatch while
a free slot exists, OR only a config field changed).

## PASS-0002 — O6: Worktree↔session binding + enumeration endpoint

Objective: Persist the binding on the durable owned-lane record and expose it
via a read endpoint + operator command.

Concrete changes:
- Extend `RepoLane` (`types.go:121-138`) and the `owned-lane-bootstrap.json`
  record written by `bootstrapOwnedLane` (`service.go:988-1026`) with:
  `repo`, `issue #N / Task-N`, worktree path (already `OwnedRepoRoot`), agent
  session id, session transcript path, run/gate state (running vs
  parked-needs-human + which gate). Populate at dispatch. (Reuse the existing
  `DeepContext.SessionID`/`TranscriptPath` plumbing, `taskexec.go:388-462`, as
  the source; persist it durably rather than only on the in-memory view.)
- Add `GET /api/v1/worktrees` in `httpapi/mux.go` (register at `mux.go:16-89`,
  follow the `handleTasksList` read pattern `mux.go:91-108`) returning every
  active worktree (running AND parked) with its bound `{ repo, issue/Task,
  worktree path, agent session id, transcript path, run/gate state }`.
- Endpoint supplies fields to CONSTRUCT a VSCodium link (worktree path/cwd,
  session id, transcript path) but does NOT emit a `vscodium://` link.
- Add an `obsidian-operator` command — new script
  `skills/obsidian-operator/scripts/Get-ActiveWorktreeSessions.ps1` (+ documented
  in `SKILL.md`) — that calls the endpoint and prints, per worktree, the worktree
  path, issue/Task, run/gate state, and transcript path.

Verification: Testing artifact showing the owned-lane record carries populated
binding fields after dispatch (A6.1); endpoint returns active worktrees with
binding (A6.2); operator command prints per-worktree binding (A6.3); a PARKED
needs-human worktree is still listed (A6.4); `go test ./...`. Isolated lane.

Exit bar / acceptance: A6.1 (HARD), A6.2 (HARD), A6.3, A6.4. Falsifier guard:
F-O6 (no endpoint enumerates active worktrees with session+transcript+state, OR
record lacks the binding, OR no operator command, OR a parked worktree is not
listed).

## PASS-0003 — O4: Done-contract (park-in-place, human-only closure)

Objective: Make every queue decision from GitHub issue state; agent NEVER
self-closes; `Human Needed=Yes` parks in place (retain worktree+slot, no
redispatch); ONLY human-approved CLOSE deallocates. No second GitHub-write path.

Concrete changes:
- Agent done-contract writes (perceived-completion AND abandon/gate) set
  `Human Needed=Yes` via the existing field-values write
  (`Sync-TaskToGitHubIssue.ps1:455-461,548-553`) with a run/gate state
  ("awaiting closure approval" | research | plan | regression). The agent calls
  NO `gh issue close`.
- Human-only CLOSE reuses the existing close builder
  (`Reconcile-TaskGitHubState.ps1:557-567`), invoked only on explicit human
  closure approval. That human-approved close is the ONLY event that calls
  `cleanupOwnedLane` + frees the slot + dequeues next.
- Consumer reads issue state: `closed` ⇒ terminal (cleanup+free+dequeue);
  `Human Needed=Yes` ⇒ park (retain worktree+slot, no redispatch, no cleanup);
  open+`Queue=Ready`+not parked ⇒ eligible. Local "complete"/terminal status may
  stop supervision but MUST NOT auto-close the issue.
- Record parked-vs-running + which gate on the owned-lane record (shared with
  O6's binding) for O5 + O6 to read.
- Document the "Queue Done-Contract" in `skills/obsidian-operator/SKILL.md`
  (agent never self-closes; closure is a distinct human gate; park retains
  worktree+slot; only human-approved close deallocates).

Verification (isolated lane, GitHub dry-run-first): abandon ⇒ `Human Needed=Yes`
+ open (A4.1); perceived-completion ⇒ parked "awaiting closure approval" + open,
then explicit human approval ⇒ closed + reclaimed (A4.2); `closed` reclaims/frees/
dequeues while `Human Needed=Yes` retains worktree+slot and is not redispatched
(A4.3); parked task resumes in SAME `OwnedRepoRoot` after gate cleared (A4.4);
effective free concurrency = `queue_workers` − parked (A4.5); no second
write-path (A4.6); agent never self-closes + cleared pass/regression/plan/research
gate does NOT close + consumer never auto-closes from local terminal status
(A4.7).

Exit bar / acceptance: A4.1–A4.7 (A4.1,A4.2,A4.3,A4.4,A4.7 HARD). Falsifier
guard: F-O4-abandon, F-O4-closure, F-O4-park, F-O4-source.

## PASS-0004 — O3: GitHub queue-drain consumer

Objective: A Temporal workflow that polls for `Queue == Ready` issues and
dispatches each into a free slot, with no manual dispatch call.

Concrete changes:
- New queue-drain workflow in `internal/queue` (sibling to `TaskRunWorkflow`,
  `taskexec.go:40-118`), registered in `StartWorker`
  (`temporalbackend/backend.go:134-142`) next to `taskexec.Register(w)`.
- New HTTP endpoint to start/stop the consumer in `httpapi/mux.go`
  (follow the dispatch-route pattern `mux.go:122-145`).
- Consumer polls the org issue-field-values endpoint for `Queue == Ready`, maps
  issue `#N` → `Tracking/Task-N` exactly (`SKILL.md:19`), checks slot
  availability (O2), dispatches Ready issues into free slots via the existing
  `taskrun` dispatch path. GitHub issues are the durable queue; no queue DB.

Verification (isolated lane, fixture/test issue set): `Queue=Ready` ⇒ dispatched
with no manual `POST .../dispatch` (A3.1); `Queue=Never`/unset ⇒ not dispatched
(A3.2); `#N`→`Task-N` mapping (A3.3); endpoint starts the consumer + workflow/slot
registered in `StartWorker` (A3.4); `go test ./...`.

Exit bar / acceptance: A3.1–A3.4. Falsifier guard: F-O3 (Ready does not
dispatch, OR Never gets dispatched, OR a manual dispatch call is required).

## PASS-0005 — O5: Top-level agent dispatch + liveness watchdog + incident email

Objective (two halves):
- 5a: Dispatch the agent as a TOP-LEVEL headless process (`codex`/`claude`) in
  its owned worktree, able to spawn its OWN subagents (NOT a nested subagent),
  and record its session id + transcript path into the O6 binding.
- 5b: External, invisible liveness watchdog reading the pinned signal
  (transcript append growth → `last_active_signal_at`,
  `Research/LIVENESS-SIGNAL.md`), suspended while parked, one poke on detected
  mid-work sleep, then incident + email.

Concrete changes:
- Net-new launcher (RESEARCH.md §O5 P4: current `runOwnedLaneExecution` runs
  backend activities, NOT a coding agent): launch the headless top-level agent
  in `OwnedRepoRoot`; capture its session id + transcript path into the binding.
- Watchdog reuses `staleRunUpdate` (`service.go:1254-1268`) + `PokeRun`
  `poke_worker_check` (`service.go:226-255`) + overdue escalation
  (`service.go:1214-1222`), but redefines `SuspiciousAfter` from "dispatch+15min"
  (`taskexec.go:81,165,296`) to "last_active_signal_at + stall_window (~5min)",
  REFRESHED on each transcript append (never anchored to dispatch).
- SUSPEND the watchdog (no stale transition, no poke, no email) whenever the run
  is gate-parked (`Human Needed=Yes` / owned-lane gate state).
- One poke that attempts to actually WAKE the process and asks it to write a
  durable stop update (set `Human Needed=Yes`/request closure) or resume.
- On confirmed sleep / failed poke: log incident + EMAIL
  `admin@digitalcollective.games` via `gmail-digest-email` skill / gmail MCP with
  (a) observed state and (b) session transcript. Email send may be captured/mocked
  on the isolated lane; artifact shows full contents.
- All thresholds (stall ~5min, poke ack ~5min, one poke, human-escalation ~30min)
  configurable, not magic numbers.

Verification (isolated lane, controlled repros): top-level agent spawns ≥1 of its
own subagents in a proof run (A5.1); watchdog reads the pinned signal not a fixed
timer (A5.2 — research artifact exists + impl reads it); stall repro detects
within ~5min, one poke, incident email containing observed state + transcript
(A5.3); thresholds configurable (A5.4); gate-parked-idle repro stays silent past
the stall threshold — no stall/poke/email (A5.5); `go test ./...`.

Exit bar / acceptance: A5.1,A5.2,A5.3,A5.5 (HARD), A5.4. Falsifier guard:
F-O5-toplevel, F-O5-signal, F-O5-detect+email, F-O5-parked.

## PASS-0006 — Cross-cutting: GitHub-app-surface regression + task-level regression

Objective: Add a named REGRESSION.md case proving the queue-drain consumer END
TO END FROM THE REAL GITHUB APP SURFACE, and ensure `go test ./...` builds and
passes with the new `internal/queue` package + modified `taskrun`/`taskexec`/
`httpapi`/`temporalbackend`.

Regression app surface = GitHub (HUMAN DECISION at the plan gate, 2026-05-29 —
see HUMAN-DIRECTIVES-FOR-WORKER.md "PASS-0006 regression surface" + "dedicated
test repo"). The human-facing surface for this feature is the GitHub issue, not a
desktop screen and not a backend-only fixture. The regression MUST be driven from
the human's Chrome debug session (CDP — see the `chrome-session-scraper` tooling)
on a SINGLE dedicated test issue in a NEW throwaway test repo under `C:\Agent`:
- In the real GitHub issue web UI, set that issue's `Queue` field to `Ready`
  (a manual human browser action — not an API/fixture write).
- The backend's polling consumer must NOTICE the change and PROCESS it (dispatch
  the task into a worktree) with NO manual dispatch call.
- ANY proxy — a fixture issue set, an issue-field-values API write that simulates
  the flip, or a backend-only test — is UNACCEPTABLE for regression closure.
  (Per-pass DEV-LOOP proof during O3/PASS-0004 may still use a fixture for speed;
  that is iteration, NOT regression closure.)
- Named case: add the next free id (REG-007; REG-006 is taken by Task-0014) as a
  GitHub-app-surface case in REGRESSION.md; update TESTING.md if a lane note is
  needed. The O5 simulated-stall incident-email behavior is exercised separately
  on the isolated lane with the gmail send MOCKED/CAPTURED.

Open isolation sub-question (confirm before this pass runs): which backend polls
and dispatches during the regression — the isolated validation-lane backend
pointed at the test repo (default, preferred per REGRESSION.md isolation), or the
live service-lane backend. The manual `Queue` flip is the human's browser action;
the agent's only GitHub interaction in this regression is read-only polling.

Verification: the named GitHub-app-surface REGRESSION.md case exercised via the
Chrome debug tab → backend pickup (AX.2); `go test ./...` from
`backend/orchestration` passes (AX.1).

Exit bar / acceptance: AX.1, AX.2 (regression closure requires the real GitHub-UI
flip → backend pickup; no proxy). Falsifier guard: F-X (`go test ./...` fails or
won't build, OR no named REGRESSION.md case, OR regression closed on proxy
evidence instead of the real GitHub app-surface flip, OR proof ran against the
human live config/DB or did ungated live GitHub writes beyond the single
authorized test issue).

---

## Global Constraints (every pass)

- Proof on the isolated validation lane ONLY (backend `:14318`, Temporal
  `:17233`, Postgres `15432`); never the human service lane (`:4318`/`:7233`/
  `5432`) or live Codex/dashboard data unless the human explicitly authorizes
  (REGRESSION.md:11-19, TESTING.md).
- GitHub interaction dry-run-first + human-gated (`SKILL.md:96-108,161-168`);
  task-owned fixtures / explicitly gated test issue; never ungated live writes.
- Mutating queue proofs (flip `Queue=Ready`, park, close, dispatch, agent work)
  run against a NEW throwaway TEST REPO under `C:\Agent` (+ its GitHub repo) so
  they can be reset arbitrarily — never the production Obsidian repo or the live
  CodexDashboard repo. The PASS-0006 regression is driven from the real GitHub UI
  via the human's Chrome debug tab; proxy/fixture evidence is NOT regression
  closure. See HUMAN-DIRECTIVES-FOR-WORKER.md.
- All GitHub state/field writes go through the obsidian-operator skill surface
  (no second write path).
- Backend builds + passes `go test ./...` from `backend/orchestration`.
- Minimize diff churn; preserve surrounding style (shared AGENTS.md diff rules).
- The agent never self-closes; closure (of THIS task too) is a distinct, final
  explicit human gate.

## Post-Plan Lifecycle (after approval)

Implementation (PASS-0000 → PASS-0006) → task-level regression → debugging if a
required lane fails (open `BUG-<NNNN>.md`) → closure ONLY on explicit human
closure approval. QA: route pass/proof review through a clean-context QA worker
where the runtime supports it; if a clean QA lane cannot be created in this
single-context runtime, flag it to the coordinator rather than self-reviewing as
QA.

## Plan Approval — GRANTED (2026-05-29)

Human APPROVED this 7-pass plan (O1→O2→O6→O4→O3→O5→cross-cutting) and the
provability-driven ordering to begin implementation at PASS-0000.

PASS-0006 regression home SETTLED: the regression app surface is GitHub, driven
via the human's Chrome debug tab on a single test issue in a NEW throwaway test
repo under `C:\Agent` (flip `Queue=Ready` → backend polling picks it up); proxy
evidence is unacceptable. See HUMAN-DIRECTIVES-FOR-WORKER.md and PASS-0006.

Coordinator-found corrections to fold into the affected passes (verified against
code, recorded in Design/plan-gate/COORDINATOR-REVIEW.md): (1) O2 must add a
per-repo slot count/semaphore — the existing `active_run_exists` gate is
per-task, not per-repo; (2) O5/O6 cannot source the launched agent's session
id/transcript path from the existing dispatch-time `DeepContext` plumbing (that
captures the backend's env) — PASS-0005 must add post-launch session discovery;
(3) the O5 poke "actually wake the process (deliver input)" mechanism is net-new
and unspecified — A5.3 must not be satisfied by a no-op state poke.
