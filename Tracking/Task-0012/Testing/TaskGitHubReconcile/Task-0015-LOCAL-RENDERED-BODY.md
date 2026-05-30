<!-- task-sync: repo=CodexDashboard; task_id=Task-0015; task_path=Tracking/Task-0015/TASK.md -->

# Task-0015: Build the Temporal-backed GitHub queue-drain consumer: per-repo worktree slots (N>1), park-on-human-gate with human-only closure, an external liveness watchdog, and a worktree↔session inventory endpoint. (Full behavior in Summary; the agent never self-closes — closure is always an explicit human action.)

## Source Of Truth

Local `Tracking/Task-0015/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0015:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

## Summary

Today, flipping a GitHub issue's `Queue` field to `Ready` does nothing. No
process consumes that field to dispatch work. "Drain my queue" was an explicit
Non-Goal of Task-0012 (`Tracking/Task-0012/TASK.md:421-422`), and the only queue
draining that exists is a manual proof-of-concept over a local `queue.json`
(`Tracking/Task-0012/Testing/DrainQueueDemo/`), not GitHub. The `Queue` field is
currently only WRITTEN by sync and READ by reconcile for drift reporting; nothing
acts on it.

The operator wants this: set an issue's `Queue=Ready`, and Temporal
automatically picks it up, allocates a git worktree for it, launches a full
top-level coding agent inside that worktree, and lets the agent work. The ONLY
event that reclaims the worktree and frees the slot is the GitHub issue reaching
its terminal state (closed); on close the consumer pulls the next Ready issue.
The dispatched agent NEVER closes the issue itself — not on abandon and not on
perceived successful completion. Closing the issue (the single terminal state) is
ALWAYS an explicit human action: when the agent believes the work is complete (or
should be abandoned / closed as a bad idea), it sets `Human Needed=Yes` with a
run/gate state of "awaiting closure approval" and PARKS in place, then asks for an
explicit closure directive; the human closes the issue directly or via the
human-gated `obsidian-operator` close path. Approvals for OTHER gates — research,
plan, pass, regression run — are NOT a proxy for closure approval; closure is a
distinct, final human gate. When the agent instead flags that it needs a human
mid-work (`Human Needed=Yes`, e.g. at a research, plan, or regression gate), the
task is PARKED in place: the worktree stays allocated, the slot stays occupied,
and the consumer does NOT redispatch it — it resumes in the same worktree once the
human clears the gate. "Awaiting closure approval" is the same kind of park
(`Human Needed=Yes`, worktree + slot retained); only the human-approved CLOSE then
deallocates the worktree and frees the slot. All running
work is bounded by a per-repo `queue_workers` count of how many worktrees may run
at once; effective free concurrency is `queue_workers` minus the number of parked
needs-human tasks (an accepted consequence). If a dispatched agent silently
stalls ("falls asleep") WHILE WORKING, Temporal detects it within ~5 minutes,
attempts to wake/poke it, and if that fails, logs an incident and emails the
operator the observed state plus the agent's session transcript; a task that is
legitimately parked on a human gate is NOT treated as asleep and its watchdog is
suspended. At dispatch the consumer also records a worktree↔session binding
(worktree path, agent session id, session transcript path, run/gate state) and
exposes it through a backend HTTP endpoint, and the `obsidian-operator` skill can
enumerate those bindings so the operator can open the right session in VSCodium
to kick a parked or slow agent along.

## Goals

1. **O1 — Manifest rename + `queue_workers` field.** Rename
   `CODEX-REPO-MANIFEST.json` → `REPO-MANIFEST.json` and update ONLY the live-code
   references listed in `## Proposed Changes`; add a per-repo `queue_workers`
   integer (default 4) to each `repos[]` entry. Leave historical `Tracking/` and
   `.codex` session artifacts unchanged.
2. **O2 — Real N>1 per-repo concurrency.** Replace the hard 1:1 dispatch gate with
   a per-repo slot mechanism that allows up to `queue_workers` concurrent owned
   lanes for a repo, reusing `provisionOwnedLane`/`bootstrapOwnedLane`/
   `cleanupOwnedLane` unchanged per slot; freeing a slot dequeues the next Ready
   issue.
3. **O3 — GitHub queue-drain consumer.** A Temporal workflow (new
   `internal/queue` package, sibling to `TaskRunWorkflow`, started by a new HTTP
   endpoint and registered in `StartWorker`) that polls the provider repo for
   issues whose org issue-field `Queue == Ready`, maps issue `#N` →
   `Tracking/Task-N`, and dispatches each into an available slot/worktree. The
   GitHub issues ARE the durable queue; no separate queue database is added.
4. **O4 — Done-contract via GitHub issue state, with parked needs-human and
   human-only closure.** The consumer makes all queue decisions from GitHub issue
   state: `Queue=Ready` ⇒ eligible to dispatch; issue `closed` ⇒ TERMINAL — and
   ONLY THEN reclaim the worktree and free the slot; `Human Needed=Yes` ⇒ PARK in
   place — keep the worktree allocated, keep the slot occupied, do NOT redispatch,
   and leave the issue open for the human; the task is resumable in the same
   worktree once the human clears the gate. The dispatched agent NEVER self-closes
   the issue — not on abandon and not on perceived completion. When it believes
   the work is complete (or should be abandoned / closed as a bad idea), it sets
   `Human Needed=Yes` with a run/gate state of "awaiting closure approval," PARKS
   in place, and asks for an explicit closure directive; it likewise sets
   `Human Needed=Yes` when it hits the research, plan, or regression gate. The
   issue is CLOSED ONLY by an EXPLICIT human closure approval — given after the
   work is reviewed and performed by the human directly or via the human-gated
   `obsidian-operator` close path (reusing the existing close command builder),
   never autonomously by the agent. Approvals for OTHER gates — research, plan,
   pass, regression run — are NOT a proxy for closure approval; closure is a
   distinct, final human gate. Consumer consequence: a local "complete"/terminal
   status is NOT sufficient to close the issue in the autonomous queue flow — the
   close is gated on the explicit human closure approval (this intentionally
   tightens the existing close-from-local-terminal-status behavior for the
   queue-driven path). Accepted consequence: effective free concurrency =
   `queue_workers` minus the number of parked needs-human tasks (including tasks
   parked awaiting closure approval).
5. **O5 — Liveness watchdog (suspended while parked) + incident email.** Dispatch
   the agent as a top-level agent process able to spawn its own subagents,
   invisibly supervised. A Temporal monitor detects "fell asleep" within
   ~5 minutes using an active-inference signal whose exact observable form is
   pinned in a durable research artifact before this sub-objective is called done.
   The watchdog MUST distinguish "parked on a human gate" (`Human Needed=Yes` —
   expected, indefinite) from "fell asleep mid-work" (unexpected): for a
   gate-parked run the watchdog is SUSPENDED (no stall detection, no poke, no
   incident email). On a detected mid-work sleep it attempts one poke that tries
   to actually wake the process ("write a durable stop update — set
   `Human Needed=Yes` or close — or get back to work"); on a confirmed sleep /
   failed poke it logs an incident and emails the operator the observed state plus
   the agent's session transcript.
6. **O6 — Worktree↔session binding registry + enumeration endpoint.** At dispatch,
   the consumer records, per owned worktree, the binding
   `{ repo, issue #N / Task-N, worktree path, agent session id, session transcript
   path, run/gate state }`, attached to the owned-lane record (extending
   `RepoLane` / the `owned-lane-bootstrap.json` record at
   `backend/orchestration/internal/taskrun/service.go:988-1026` and
   `types.go:121-138`). A backend ("Obsidian") HTTP endpoint
   (e.g. `GET /api/v1/worktrees`) registered in
   `backend/orchestration/internal/httpapi/mux.go` returns the active worktrees
   with their bound session + transcript path + state. An `obsidian-operator`
   skill capability/command calls that endpoint and enumerates the sessions bound
   to each worktree, so the operator can open the right session in VSCodium to
   kick a parked or slow agent along or for special interactions.

## Acceptance Criteria

Each criterion is pass/fail. Sub-objective criteria are grouped; the hard
requirements each have their own criterion AND a matching falsifier in
`## Falsifiers`.

### O1 — Manifest rename + `queue_workers`

- A1.1: A file named `REPO-MANIFEST.json` exists at repo root, parses as JSON,
  retains the existing `schema_version`/`manifest_type`/`repos[]` shape, and each
  `repos[]` entry has an integer `queue_workers` field (default `4`). PASS/FAIL.
- A1.2: `CODEX-REPO-MANIFEST.json` no longer exists at repo root, and the exact
  live-code references listed in `## Proposed Changes` O1 resolve to
  `REPO-MANIFEST.json`. PASS/FAIL.
- A1.3: `python -m unittest tests.test_obsidian_title_roundtrip` passes against
  the renamed manifest. PASS/FAIL.
- A1.4: No file under `Tracking/Task-0012/`, `Tracking/Task-0013/`, or `.codex`
  sessions was modified by the rename. PASS/FAIL.

### O2 — Real N>1 per-repo concurrency

- A2.1 (HARD): With `queue_workers=4` and at least two Ready issues, more than one
  owned worktree for the same repo is observed running SIMULTANEOUSLY (proven by
  two concurrent `git worktree` checkouts under the owned-lane root with two live
  task runs, captured in a Testing artifact). PASS/FAIL.
- A2.2: A 5th dispatch attempt for the same repo while 4 slots are occupied is
  refused/queued (not run), and is dispatched only after a slot frees. PASS/FAIL.
- A2.3: `deriveDispatchReadiness` no longer emits `active_run_exists` for a
  same-repo dispatch while a free slot remains. PASS/FAIL.

### O3 — GitHub queue-drain consumer

- A3.1: With the consumer running and an issue flipped to `Queue=Ready`, the
  consumer dispatches `Tracking/Task-N` into a worktree without any manual
  `POST .../dispatch` call. PASS/FAIL.
- A3.2: An issue with `Queue=Never` (or unset) is NOT dispatched by the consumer.
  PASS/FAIL.
- A3.3: The consumer maps issue `#N` to `Tracking/Task-N` exactly (no mapping
  layer), consistent with the provider contract
  (`skills/obsidian-operator/SKILL.md:19`). PASS/FAIL.
- A3.4: A new HTTP endpoint starts the consumer, and the queue-drain workflow plus
  slot mechanism are registered in `StartWorker`. PASS/FAIL.

### O4 — Done-contract via GitHub issue state, with parked needs-human and human-only closure

- A4.1 (HARD): When the dispatched agent abandons/flags a task, the GitHub issue's
  `Human Needed` becomes `Yes` and the issue stays OPEN (the agent did NOT
  self-close). PASS/FAIL.
- A4.2 (HARD): When the dispatched agent believes the work is complete, it does
  NOT close the issue: it sets `Human Needed=Yes` with a run/gate state of
  "awaiting closure approval," parks in place (worktree + slot retained), and the
  issue stays OPEN until an explicit human closure approval. The issue is CLOSED
  only by the explicit human closure approval (human directly or via the
  human-gated `obsidian-operator` close path), and ONLY THEN does the consumer
  reclaim the worktree (`cleanupOwnedLane`) and free the slot (shown in a Testing
  artifact: completion ⇒ open + parked-awaiting-closure; then explicit human
  approval ⇒ closed + reclaimed). PASS/FAIL.
- A4.3 (HARD): On `closed` the consumer treats the task as TERMINAL: it reclaims
  the worktree, frees the slot, and dequeues the next Ready issue. On
  `Human Needed=Yes` the consumer PARKS the task in place — it KEEPS the worktree
  allocated, KEEPS the slot occupied, leaves the issue open, and does NOT
  auto-redispatch it; the worktree is observably still present and the slot still
  counted as used (captured in a Testing artifact). PASS/FAIL.
- A4.4 (HARD): A task parked at a research, plan, or regression gate, or parked
  "awaiting closure approval" (`Human Needed=Yes`), is NOT evicted, torn down, or
  redispatched, and can be RESUMED in the SAME worktree after the gate is cleared
  (same `OwnedRepoRoot`, no re-provision), shown in a Testing artifact. PASS/FAIL.
- A4.5: Effective free concurrency is computed as `queue_workers` minus the number
  of parked needs-human tasks for the repo (including tasks parked awaiting closure
  approval; a 4-slot repo with 1 parked task admits at most 3 new dispatches).
  PASS/FAIL.
- A4.6: No second/parallel GitHub-write path is introduced; all issue-state and
  issue-field writes go through the obsidian-operator skill surface. PASS/FAIL.
- A4.7 (HARD): The dispatched agent NEVER self-closes the issue for ANY reason
  (abandon OR perceived completion); closure occurs ONLY on an explicit human
  closure approval; a pass / regression-run / plan / research approval is NOT
  treated as closure approval; and the consumer does NOT auto-close the issue from
  a local "complete"/terminal status without an explicit human closure approval.
  Shown by a Testing artifact: an agent that reaches "complete" parks
  `Human Needed=Yes` (awaiting closure approval) with the issue OPEN and remains
  open across a cleared pass/regression/plan/research gate, until the explicit
  human closure approval closes it. PASS/FAIL.

### O5 — Liveness watchdog (suspended while parked) + incident email

- A5.1 (HARD): The dispatched agent runs as a top-level process and successfully
  spawns at least one of its own subagents during a proof run (captured in a
  Testing artifact). PASS/FAIL.
- A5.2 (HARD): A durable research artifact (e.g.
  `Tracking/Task-0015/Research/LIVENESS-SIGNAL.md`) exists that names the exact
  observable liveness signal for a headless agent, how it is sampled, and why it
  separates "working" from "asleep." The watchdog implementation reads THAT signal
  (not a fixed timer alone). PASS/FAIL.
- A5.3 (HARD): In a controlled stall repro, the watchdog detects the stall within
  the configured window (~5 min), issues exactly one wake/poke, and — when the
  poke ack window elapses without a fresh active signal — produces an incident
  email to `admin@digitalcollective.games` that contains BOTH the watchdog's
  observed state AND the agent's session transcript. PASS/FAIL.
- A5.4: Detection/poke/escalation thresholds (stall ~5 min, poke ack ~5 min, one
  poke, human-escalation ~30 min) are configurable, not hard-coded magic numbers.
  PASS/FAIL.
- A5.5 (HARD): A run parked needs-human (`Human Needed=Yes`, e.g. at the
  research/plan/regression gate) generates NO stall transition, NO poke, and NO
  incident email for the duration it is parked, even past the ~5 min window;
  shown by a controlled repro where a gate-parked run sits idle beyond the stall
  threshold and the watchdog stays silent (captured in a Testing artifact). The
  watchdog demonstrably distinguishes "parked on a human gate" from "fell asleep
  mid-work." PASS/FAIL.

### O6 — Worktree↔session binding registry + enumeration endpoint

- A6.1 (HARD): After a dispatch, the owned-lane record for that worktree carries
  the binding fields `{ repo, issue #N / Task-N, worktree path, agent session id,
  session transcript path, run/gate state }` populated (not empty placeholders),
  shown in a Testing artifact. PASS/FAIL.
- A6.2 (HARD): A backend HTTP endpoint (e.g. `GET /api/v1/worktrees`), registered
  in `httpapi/mux.go`, returns every active worktree (running and parked) with its
  bound session id, session transcript path, and run/gate state. PASS/FAIL.
- A6.3: An `obsidian-operator` command (new script or documented SKILL.md command)
  calls that endpoint and prints, per active worktree, the worktree path,
  issue/Task, run/gate state, and the session transcript path the operator would
  open in VSCodium. PASS/FAIL.
- A6.4: For a PARKED needs-human task, the endpoint and operator command still
  list its worktree, bound session, transcript path, and parked run/gate state
  (so the operator can reach a parked agent's session). PASS/FAIL.

### Cross-cutting

- AX.1: `go test ./...` (run from `backend/orchestration`) builds and passes with
  the new `internal/queue` package and the modified `taskrun`/`taskexec`/
  `httpapi`/`temporalbackend` code. PASS/FAIL.
- AX.2: A named REGRESSION.md case covers the new queue-drain operator behavior
  (including the simulated-stall incident email) and was exercised on an isolated
  validation/task lane, not the human's live config/DB. PASS/FAIL.

## Non-Goals

- Non-GitHub queue sources (no local-JSON queue, no label/Project-based queue).
- Cross-repo global fairness or scheduling beyond a simple per-repo slot cap; the
  default behavior is per-repo `queue_workers` only.
- Rewriting historical artifacts during the manifest rename. Do NOT rewrite
  references under `Tracking/Task-0012/*`, `Tracking/Task-0013/*`, or
  `C:\Users\gregs\.codex\sessions\*` (durable history).
- Changing the Codex-vs-Claude runtime selection logic beyond what dispatch needs
  to launch a top-level agent in the worktree.
- Token-ingest, scanner, or dashboard-UI work (Task-0013/0014 areas).
- The Obsidian dashboard REVIEW SURFACE / UI for surveying needs-human work. That
  is the human's sibling task. THIS task delivers only the worktree↔session
  enumeration endpoint and the `obsidian-operator` command that surface will
  consume (O6); it does not build the review UI.
- Renaming the `CodexDashboard` repo `id`, data root, or other OS identifiers
  (Task-0013 Decision A keeps those literal; `app/codex_dashboard/paths.py:48-56`,
  `DATA-HANDLING.md:14-25`). Only the manifest FILENAME and the per-repo
  `queue_workers` field change here.
- Semantics for an agent voluntarily deciding it no longer needs its worktree —
  i.e. releasing/relinquishing a worktree WITHOUT a task closure — are explicitly
  OUT OF SCOPE / DEFERRED for this task. The human flagged this as likely-future
  ("we'll probably need to add semantics for an agent deciding it no longer needs
  a worktree, but let's not complicate matters right now"). In this task the ONLY
  thing that deallocates a worktree/frees a slot is the human-approved CLOSE of the
  GitHub issue; do NOT build a voluntary-worktree-release path here.

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `15`
- Local task path: `Tracking/Task-0015/TASK.md`
- Source commit: `ed4b29411673c462f5294dabbe0be38df4e13305`
- Local task SHA-256: `586A96E0F7A4AD38A96D2E1D1EFF6C72160533A953A46A561FCC0409CD1DED5C`
- Rendered at: `2026-05-29T17:24:32.0162708-04:00`