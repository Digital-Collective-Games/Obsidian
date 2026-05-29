# Task 0015

## Title

Build the Temporal-backed GitHub queue-drain consumer: per-repo worktree slots (N>1), park-on-human-gate with human-only closure, an external liveness watchdog, and a worktree↔session inventory endpoint. (Full behavior in Summary; the agent never self-closes — closure is always an explicit human action.)

## Writeup Type

Concrete implementation task with ONE embedded research item.

The chosen solution shape (a new backend `internal/queue` Temporal workflow, plus
a per-repo slot semaphore, plus reuse of the existing owned-lane and
poke/follow-up machinery, plus a worktree↔session binding on the owned-lane
record exposed through a new backend HTTP endpoint) is fixed. One sub-objective
(O5, liveness detection) depends on an observable signal that is a genuine KNOWN
UNKNOWN for a headless agent; that single unknown is carved out as a bounded,
durable research artifact that must be produced before O5's watchdog can be
called done. The whole task is NOT downgraded to research: O1–O4, O6, and the
watchdog/poke/escalation/email plumbing of O5 are concrete implementation work
today.

This is one feature ("the Temporal-backed consumer") with six internally
separable sub-objectives the human deliberately bundled. Each sub-objective has
its own acceptance criteria and its own falsifier. The merge is earned in
`## Why One Task` and the per-objective boundaries are kept explicit in
`## Internal Mechanism Map`. The sub-objectives are NOT to be split, narrowed,
broadened, or re-sequenced on auditor preference; if staging is believed
necessary, raise it as a human-gate note rather than a unilateral split (see
`HUMAN-DIRECTIVES-FOR-WORKER.md`, "Scope / Process Notes").

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

## Who Is Affected

The single human operator who runs this orchestration backend on their own
Windows machine (`admin@digitalcollective.games`). The backend runs as an
always-on Windows scheduled-task service lane (Temporal + Postgres,
`backend/orchestration/README.md:132-167`). There is no remote/multi-user
deployment. The audience for the incident email is that same operator.

## Human-Facing / Operator-Facing Outcome (state first)

After this task:

- The operator flips a GitHub issue's `Queue` field to `Ready` (on
  `Digital-Collective-Games/Obsidian`, issue `#N` ⇒ `Tracking/Task-N`). Within
  the consumer's poll interval, Temporal dispatches that task into a free
  worktree slot and a top-level coding agent begins working in it. No further
  manual dispatch step is required.
- More than one repo task can run at once, up to the per-repo `queue_workers`
  count (default 4). When a slot frees, the consumer immediately dispatches the
  next `Queue=Ready` issue.
- The dispatched agent is a full, top-level agent that can spawn its own
  subagents (it is NOT a nested subagent that would be blocked from nesting).
- The dispatched agent NEVER closes the GitHub issue itself — not on abandon and
  not on perceived successful completion. When the agent believes the work is
  complete, it does NOT self-close: it sets `Human Needed=Yes` with a run/gate
  state of "awaiting closure approval," PARKS in place, and asks for an explicit
  closure directive. The issue is CLOSED (terminal) ONLY by an explicit human
  closure approval — performed by the human directly or via the human-gated
  `obsidian-operator` close path — and ONLY THEN does the consumer reclaim the
  worktree and free the slot. Approvals for OTHER gates — research, plan, pass,
  regression run — are NOT a proxy for task-closure approval; closure is a
  distinct, final human gate, so passing tests or a regression/plan/research
  approval never closes the task. When the agent decides mid-work that the task
  should be abandoned or that it needs a human (including hitting the research,
  plan, or regression gate, or judging the task a bad idea), it likewise sets the
  GitHub issue's `Human Needed=Yes` and stops — it does NOT close the issue. The
  consumer then PARKS the task in place: it keeps the worktree allocated, keeps
  the slot occupied, and does NOT redispatch the issue. The task resumes IN THE
  SAME WORKTREE once the human clears the gate (or the human, after reviewing,
  explicitly closes the issue — whether to accept completion or to abandon it).
  Re-provisioning a worktree can be expensive (especially Unreal Engine), so the
  three normal human-approval pauses — research gate, plan gate, regression gate —
  and the "awaiting closure approval" park must not cause the task to be evicted,
  torn down, or redispatched.
- Because parked tasks keep their slots, effective free concurrency for a repo is
  `queue_workers` minus the number of tasks currently parked needs-human (an
  accepted consequence). The operator plans to add an Obsidian review surface to
  survey needs-human work and does not expect to be blocked long; that review UI
  is a sibling task, not part of this one (see `## Non-Goals` and O6).
- If a dispatched agent falls asleep (stops actively working) WHILE WORKING, the
  operator learns within ~5 minutes via an incident email containing the
  watchdog's observed state and the agent's session transcript, after a single
  automated poke attempt to wake it has failed. A task that is legitimately
  parked on a human gate (`Human Needed=Yes`) is NOT "asleep": its watchdog is
  suspended, so it produces no stall poke and no incident email.
- For any active worktree (running or parked), the operator can query a backend
  endpoint (and an `obsidian-operator` command that calls it) to see the session
  bound to that worktree and the path to its session transcript, then open that
  exact session in VSCodium to kick a parked or slow agent along or for special
  interactions — without guessing which session belongs to which worktree.

### Current fallback / proxy that must not be mistaken for success

- A config field named `queue_workers` existing in the manifest, with the
  backend still running 1:1, does NOT count as N>1 concurrency. The proof is
  more than one worktree running simultaneously for one repo.
- A nested subagent that "works" but cannot spawn its own subagents does NOT
  satisfy the top-level-agent requirement.
- A watchdog that flips a run to `sleeping_or_stalled` on a fixed timer with no
  evidence-backed liveness signal (a "feels alive" proxy) does NOT satisfy O5.
- A consumer that frees the slot or removes the worktree when an issue is marked
  `Human Needed=Yes` does NOT count. `Human Needed=Yes` is a PARKED pause that
  RETAINS both the worktree and the slot; ONLY a CLOSED issue deallocates. A
  task that gets evicted, torn down, or redispatched because it hit the research,
  plan, or regression gate is exactly the failure this task forbids.
- The agent self-closing the issue for ANY reason (abandon OR perceived
  completion) does NOT count. Closing is ALWAYS an explicit human action; on
  perceived completion the agent must PARK "awaiting closure approval"
  (`Human Needed=Yes`) and ask, never close. Likewise, treating a pass,
  regression-run, plan, or research approval as task-closure approval does NOT
  count — closure is a distinct, final human gate. A consumer that auto-closes the
  GitHub issue from a local "complete"/terminal status, without an explicit human
  closure approval, does NOT count either.
- A watchdog that pokes, raises a stall incident, or emails the operator about a
  task that is legitimately parked needs-human does NOT count. The watchdog must
  be suspended for a gate-parked run and must distinguish "parked on a human gate"
  (expected, indefinite) from "fell asleep mid-work" (unexpected).
- A worktree↔session binding that is recorded only in memory or in scattered logs,
  with no backend endpoint to enumerate active worktrees and their bound session +
  transcript path + state, does NOT satisfy O6. The operator must be able to query
  it and an `obsidian-operator` command must surface it.
- The local backend run state (e.g. `TASK-STATE.json` status or the internal
  task-run state machine) is an INTERNAL supervision aid only. The
  human-queryable source of truth for queue decisions is the GitHub issue. A
  parallel local terminal-status enum used as the queryable source of truth does
  NOT count (D2).

## Current Truth

- **Nothing consumes the `Queue` field today.** Central queue draining /
  dispatching from GitHub Issues were explicit Non-Goals of Task-0012
  (`Tracking/Task-0012/TASK.md:421-422`). `DrainQueueDemo` is a local-JSON PoC,
  not GitHub.
- **Worktree allocate/reclaim already exists and is reused as-is.**
  `dispatchWithDirective` is the owned-lane dispatch entry point
  (`backend/orchestration/internal/taskrun/service.go:126-185`).
  `provisionOwnedLane` runs `git worktree add --detach`
  (`service.go:934-966`), `bootstrapOwnedLane` writes the lane bootstrap artifact
  (`service.go:988-1026`), and `cleanupOwnedLane` runs `git worktree remove
  --force` (`service.go:1028-1045`). These run under the Temporal
  `TaskRunWorkflow` (`backend/orchestration/internal/taskexec/taskexec.go:40-118`).
- **The backend is HARD 1:1 per repo today.** `deriveDispatchReadiness` appends
  the `active_run_exists` block reason whenever an active run exists, blocking a
  second dispatch (`service.go:747-809`, specifically `:777-781`); and
  `releasePreviousOwnedLane` tears down the prior lane before a new dispatch
  (`service.go:968-986`, called at `service.go:138`). There is no per-repo slot
  or pending-queue concept; `RepoLane` is a single owned checkout
  (`backend/orchestration/internal/taskrun/types.go:121-138`).
- **Liveness / poke / escalation primitives already exist to reuse.**
  `StateEnvelope.SuspiciousAfter` (`types.go:47`) plus `LastProgressAt`
  (`types.go:186`); `StateSleepingOrStalled` and `ActionPoke` constants
  (`types.go:20,36`); `staleRunUpdate` auto-transitions a running/dispatching run
  to `sleeping_or_stalled` once `SuspiciousAfter` passes
  (`service.go:1254-1268`); `PokeRun` creates a `poke_worker_check` follow-up with
  a 5-min `DueAt` (`service.go:226-255`); an overdue `backend_worker` follow-up
  escalates attention to `urgent` (`service.go:1214-1222`). The current
  `SuspiciousAfter` window for running states is +15 min
  (`taskexec.go:81,165`; `service.go:805`); activities use a 2-min StartToClose
  (`taskexec.go:126`).
- **Org issue fields exist and are queryable.** `Queue` (id 42656828,
  Never/Ready), `Human Needed` (id 42656829, No/Yes), `Priority` (id 42656780),
  on org `Digital-Collective-Games`, read/written via
  `/repos/Digital-Collective-Games/Obsidian/issues/<n>/issue-field-values`. The
  `obsidian-operator` skill already writes these field values
  (`skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1:300,455-461`) and
  reads them in reconcile
  (`skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1`). GitHub gates
  the Fields panel on the issue having a TYPE; the sync now sets `type=Task`
  (`skills/obsidian-operator/SKILL.md:25-46`).
- **A close path already exists to reuse.** Reconcile builds a
  `gh issue close ... --reason completed|not planned --comment ...` command from
  local terminal state
  (`skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1:557-567`), and
  the reciprocal `gh issue reopen` at `:571-575`. Terminal local statuses map to
  CLOSED (`Reconcile-TaskGitHubState.ps1:140-141`). NOTE: this existing
  close-from-local-terminal-status behavior is INTENTIONALLY TIGHTENED for the
  queue-driven path by this task — in the autonomous queue flow a local
  "complete"/terminal status is NOT sufficient to close the issue; the close must
  be gated on an explicit human closure approval (O4 below). The same close
  command builder is reused, but it is invoked only on the human's explicit
  closure directive, never autonomously by the dispatched agent.
- **An owned-lane record already exists to extend for the binding.** `RepoLane`
  (`backend/orchestration/internal/taskrun/types.go:121-138`) already carries the
  owned worktree path (`OwnedRepoRoot`) and artifact paths, and
  `bootstrapOwnedLane` already writes an `owned-lane-bootstrap.json` record at
  dispatch (`backend/orchestration/internal/taskrun/service.go:988-1026`). There
  is currently NO field binding a worktree to its agent session id, session
  transcript path, or run/gate state, and NO endpoint that enumerates active
  worktrees with their bound session. The HTTP mux registers all routes in one
  place (`backend/orchestration/internal/httpapi/mux.go:16-89`), so a new
  `GET` worktrees endpoint slots in next to the existing routes.
- **Manifest today.** `CODEX-REPO-MANIFEST.json` has `schema_version`,
  `manifest_type: codex_repo_registry`, and `repos[]` entries with `id`,
  `local_root`, `source_control_provider`, `task_provider`
  (`Digital-Collective-Games/Obsidian`), and `task_proposal_provider`
  (`.../ObsidianProposals`). There is no `queue_workers` field. The file is named
  in DATA-HANDLING.md as a must-backup manifest (`DATA-HANDLING.md:20,161,239`).

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

## Implementation Home

- **Backend (Go + Temporal)** is the home for O2, O3, O4 (consumer side), O5
  (watchdog/poke/escalation), and O6 (binding registry + enumeration endpoint).
  The owned-lane worktree machinery, the `TaskRunWorkflow`, the worker
  registration, the poke/follow-up primitives, the `RepoLane`/owned-lane-bootstrap
  record, and the HTTP mux all already live in
  `backend/orchestration/internal/{taskrun,taskexec,temporalbackend,httpapi}` and
  are the correct seam to extend. A new `internal/queue` package (sibling to
  `taskexec`) hosts the queue-drain workflow and the per-repo slot mechanism,
  keeping the new consumer concern separate from the per-run execution concern
  while reusing the same `taskrun.Service`. The O6 binding is recorded on the
  existing owned-lane record (extending `RepoLane` /
  `bootstrapOwnedLane`, `service.go:988-1026`, `types.go:121-138`) rather than in
  a new store, because that record is already created per worktree at dispatch;
  the enumeration endpoint is a new `GET` route in
  `backend/orchestration/internal/httpapi/mux.go` alongside the existing routes.
- **The manifest file at repo root** (`REPO-MANIFEST.json` after rename) is the
  home for O1's `queue_workers` config, because it already binds per-repo
  providers and is the documented per-repo registry
  (`DATA-HANDLING.md:161`).
- **The `obsidian-operator` skill + scripts** are the home for the agent-side
  done-contract writes in O4 (set `Human Needed=Yes` on abandon AND on perceived
  completion / awaiting closure approval — the agent NEVER self-closes) and for the
  human-gated CLOSE path (the issue is closed only on an explicit human closure
  approval, via the human-gated close command in this skill), because that skill is
  already the normalized, dry-run-guarded surface for all GitHub issue-state and
  issue-field writes (`skills/obsidian-operator/SKILL.md:79-108`), and the reconcile
  close/reopen command builders already exist
  (`Reconcile-TaskGitHubState.ps1:557-577`). The consumer must NOT create a second
  parallel GitHub-write path, and must NOT auto-close the issue from a local
  terminal status. The same `obsidian-operator` skill is also the home
  for O6's operator-facing enumeration command (a new script or a documented
  command in `skills/obsidian-operator/SKILL.md`) that calls the backend worktrees
  endpoint, because that skill is already the operator's normalized surface for
  interacting with the queue/provider state.

Why not elsewhere: putting the consumer in the desktop app or a standalone script
would lose Temporal durability (the human explicitly wants Temporal to own queue
draining, worktree allocation, and supervision — `HUMAN-DIRECTIVES-FOR-WORKER.md`
verbatim directive). Putting the slot logic inside `obsidian-operator` would
duplicate dispatch logic that already lives in the backend.

## Why One Task

These six sub-objectives are one coherent feature — "the Temporal-backed
consumer" — and the human explicitly bundled them
(`HUMAN-DIRECTIVES-FOR-WORKER.md`). They belong together because the consumer
(O3) cannot dispatch more than one task without N>1 slots (O2); slots cannot be
sized without the `queue_workers` config (O1); a slot can only be freed (and a
worktree only torn down) by the human-approved terminal-close path (the agent
never self-closes), while needs-human — including awaiting closure approval —
must PARK in place — both halves of that slot/worktree lifecycle are the
done-contract (O4); an agent that silently dies WHILE WORKING would hold a slot forever without
the liveness watchdog (O5), which in turn must NOT fire on a legitimately parked
task; and once tasks can park in place indefinitely, the operator needs the
worktree↔session binding + enumeration endpoint (O6) to reach the right session
and kick a parked or slow agent along. Each reduces a distinct failure and acts
at a distinct seam, and each keeps its own acceptance criteria and falsifier, so
they remain independently reviewable and shippable. See
`## Internal Mechanism Map`.

## Proposed Changes

### O1 — Manifest rename + `queue_workers`

- Rename `CODEX-REPO-MANIFEST.json` → `REPO-MANIFEST.json` at repo root and add a
  per-repo integer `queue_workers` (default `4`) to each `repos[]` entry.
- Update ONLY these live-code references to the filename:
  - `skills/obsidian-operator/SKILL.md` (frontmatter `description` line 3; body
    line 10).
  - `skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1` default
    `-ManifestPath` (line 4).
  - `skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1` default
    `-ManifestPath` (line 6).
  - `skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1` default
    `-ManifestPath` (line 4).
  - `tests/test_obsidian_title_roundtrip.py` (fixture file write at line 110 and
    the `-ManifestPath` argument at line 154).
  - `app/codex_dashboard/paths.py` (the doc comment naming the file at line 52 —
    update the filename in the comment; the `"id"` value stays `CodexDashboard`).
  - `DATA-HANDLING.md` (lines 20, 161, 239).
- Do NOT edit historical references under `Tracking/Task-0012/*`,
  `Tracking/Task-0013/*`, or `.codex` sessions.

### O2 — N>1 per-repo concurrency (slots)

- Add a per-repo slot mechanism (recommended: a `RepoSlotManagerWorkflow` in the
  new `internal/queue` package holding `{repo → {used, limit}}` plus a pending
  queue, OR an equivalent durable slot counter the consumer consults). `limit`
  comes from the repo's `queue_workers`.
- Change `deriveDispatchReadiness` so the dispatch gate is "fewer than
  `queue_workers` active owned lanes exist for this repo" instead of "no active
  owned lane exists for this repo" (`service.go:777-781`). The 1:1
  `releasePreviousOwnedLane` teardown on every dispatch (`service.go:138,968-986`)
  must no longer fire for a same-repo sibling that legitimately holds its own
  slot.
- Reuse `provisionOwnedLane`/`bootstrapOwnedLane`/`cleanupOwnedLane` unchanged,
  once per slot. Each slot is an independent owned lane / worktree.
- A slot is freed (and its worktree reclaimed via `cleanupOwnedLane`) ONLY when
  the GitHub issue is CLOSED (terminal). A task parked `Human Needed=Yes` KEEPS
  its slot and worktree and is NOT torn down. Freeing a slot on close triggers a
  dequeue of the next Ready issue. Parking on needs-human does NOT dequeue
  anything (the slot stays occupied).

### O3 — GitHub queue-drain consumer

- New `backend/orchestration/internal/queue` package with a Temporal queue-drain
  workflow, sibling to `TaskRunWorkflow`
  (`taskexec.go:40-118`), registered in `StartWorker`
  (`backend/orchestration/internal/temporalbackend/backend.go:134-142`) next to
  `taskexec.Register(w)`.
- A new HTTP endpoint to start/stop the consumer, registered in
  `backend/orchestration/internal/httpapi/mux.go` alongside the existing routes
  (`mux.go:16-89`), following the dispatch-route pattern
  (`mux.go:122-145`).
- The consumer polls the provider repo for issues with `Queue == Ready` via the
  same issue-field-values endpoint the skill uses, maps issue `#N` →
  `Tracking/Task-N`, checks slot availability for the repo, and dispatches Ready
  issues into free slots through the existing `taskrun` dispatch path.
- No separate queue DB. The GitHub issues are the durable queue.

### O4 — Done-contract via GitHub issue state, with parked needs-human and human-only closure

- The dispatched agent NEVER self-closes the GitHub issue — not on abandon and not
  on perceived successful completion. On perceived completion the agent's "done"
  path sets `Human Needed=Yes` via the issue-field-values write
  (`Sync-TaskToGitHubIssue.ps1:455-461,548-553`) with a run/gate state of
  "awaiting closure approval," PARKS in place, and asks for an explicit human
  closure directive. It does NOT call any `gh issue close`.
- The issue is CLOSED ONLY by an EXPLICIT human closure approval, performed by the
  human directly or via the human-gated `obsidian-operator` close path (reusing
  the existing close command builder,
  `Reconcile-TaskGitHubState.ps1:557-567`, `gh issue close --reason completed|not planned`).
  That human-approved CLOSE is the ONLY event that calls `cleanupOwnedLane` to
  reclaim the worktree and free the slot. In the autonomous queue flow the
  consumer MUST NOT auto-close the issue from a local "complete"/terminal status;
  the local terminal status may stop the backend from supervising the run, but it
  does NOT close the issue (this intentionally tightens the existing
  close-from-local-terminal-status behavior for the queue-driven path).
- Approvals for OTHER gates — research, plan, pass, regression run — are NOT a
  proxy for task-closure approval. Closure is a distinct, final human gate; the
  agent must obtain an explicit closure directive even when it believes the work
  is complete and even when a pass/regression/plan/research gate was just cleared.
- The dispatched agent's abandon/flag path (including hitting the research, plan,
  or regression gate, or judging the task a bad idea) likewise sets the issue's
  `Human Needed=Yes` via the issue-field-values write
  (`Sync-TaskToGitHubIssue.ps1:455-461,548-553`) and does NOT close the issue.
- On `Human Needed=Yes` (including the "awaiting closure approval" park) the
  consumer PARKS the task in place: it MUST NOT call `cleanupOwnedLane`, MUST NOT
  release the slot, and MUST NOT redispatch the issue. The owned worktree stays
  allocated and the task is resumable in place once the human clears the gate (or,
  after review, explicitly closes the issue — to accept completion or to abandon
  it). This supersedes any earlier "free the slot on needs-human" behavior — the
  three human gates (research, plan, regression) and the awaiting-closure park
  must not bump the task.
- The consumer reads issue state (open/closed, `Queue`, `Human Needed`) to make
  every queue decision: `closed` ⇒ terminal (cleanup + free slot + dequeue next);
  `Human Needed=Yes` ⇒ parked (retain worktree + slot, no redispatch);
  `Queue=Ready` and open and not parked ⇒ eligible. The local backend run state
  may still be consulted to stop supervising a run, but the queryable source of
  truth is the GitHub issue, and a local terminal status never auto-closes it.
- Record the parked vs running distinction — and, when parked, which gate
  (research / plan / regression / awaiting-closure-approval) — in the run/gate
  state on the owned-lane record (shared with O6's binding) so the watchdog (O5)
  and the enumeration endpoint (O6) can both read it.
- Document this done-contract — including that the agent NEVER self-closes, that
  closure requires an explicit human closure approval (distinct from any
  pass/regression/plan/research gate), that `Human Needed=Yes` (including
  "awaiting closure approval") is a PARKED pause that retains the worktree and
  slot, and that ONLY the human-approved close deallocates — in
  `skills/obsidian-operator/SKILL.md` (Authority Model / a new "Queue
  Done-Contract" subsection) so the agent-side behavior is durable.

### O5 — Liveness watchdog (suspended while parked) + incident email

- Dispatch launches the agent as a TOP-LEVEL agent process (e.g. a headless
  `codex`/`claude` run) in its owned worktree, able to spawn its own subagents —
  not as a nested subagent.
- Produce a durable research artifact (recommended path
  `Tracking/Task-0015/Research/LIVENESS-SIGNAL.md`) that pins the EXACT observable
  signal distinguishing "actively working/thinking" from "fell asleep/idle" for a
  headless agent process. Candidate signals to evaluate and decide among: session
  transcript/log file growth, process busy/idle, an in-flight model request,
  stdout activity. The artifact must name the chosen signal, how it is sampled,
  and why it reliably separates the two states.
- Implement the watchdog as a Temporal monitor that observes that signal
  externally (the agent emits no supervision-aware signal), reusing
  `SuspiciousAfter`/`staleRunUpdate` (`service.go:1254-1268`) but with the
  detection window proposed at ~5 min instead of the current +15 min
  (`taskexec.go:81,165`; `service.go:805`). All thresholds configurable.
  The detection deadline MUST be refreshed by the active-inference signal
  (track a last-active-signal timestamp and reset `SuspiciousAfter` whenever the
  signal shows activity), so "stale/asleep" means "no active signal for ~5 min",
  NOT "~5 min since dispatch." A deadline anchored to dispatch time rather than
  last observed activity is the fixed-timer proxy F-O5-signal falsifies.
- SUSPEND the watchdog while the run is parked on a human gate. A run whose
  GitHub issue is `Human Needed=Yes` (e.g. parked at the research, plan, or
  regression gate) is NOT "asleep" and must generate NO stall detection, NO poke,
  and NO incident email. The watchdog MUST distinguish "parked on a human gate"
  (expected, indefinite — read from the issue `Human Needed` state and the
  owned-lane run/gate state shared with O4/O6) from "fell asleep mid-work"
  (unexpected → poke + incident below). Concretely, the `staleRunUpdate`
  transition to `sleeping_or_stalled` (`service.go:1254-1268`) and any poke MUST
  be skipped for a gate-parked run; resume watchdog liveness tracking only after
  the human clears the gate and the run returns to active work.
- On detected mid-work sleep (a non-parked, actively-working run that goes idle),
  issue one poke that attempts to actually wake the process and asks it to write a
  durable stop update (set `Human Needed=Yes` or close) or resume, reusing
  `PokeRun`'s `poke_worker_check` follow-up with a poke-ack window
  (`service.go:226-255`).
- On a confirmed sleep / failed poke (ack window elapsed without fresh active
  signal), LOG an incident and EMAIL the operator
  (`admin@digitalcollective.games`) through the configured Gmail path
  (`gmail-digest-email` skill / local gmail MCP) with (a) the watchdog's observed
  state and (b) the agent's session transcript.
- Default thresholds (all configurable): stall detect ~5 min of no active signal;
  poke ack window ~5 min; exactly one poke then escalate; human-escalation
  ~30 min.

### O6 — Worktree↔session binding registry + enumeration endpoint

- Extend the owned-lane record to carry the binding fields per worktree:
  `repo`, `issue #N / Task-N`, worktree path (already present as
  `OwnedRepoRoot`), agent session id, session transcript path
  (e.g. `~/.codex/sessions/…` or `~/.claude/projects/…`), and run/gate state
  (running vs parked-needs-human, and which gate when parked — shared with O4).
  Add these to `RepoLane` (`backend/orchestration/internal/taskrun/types.go:121-138`)
  and populate them in `bootstrapOwnedLane`
  (`backend/orchestration/internal/taskrun/service.go:988-1026`) at dispatch, so
  the binding is durable on the same record that already tracks the worktree.
- Add a backend ("Obsidian") HTTP endpoint — `GET /api/v1/worktrees` (or
  `/api/v1/queue/lanes`) — registered in
  `backend/orchestration/internal/httpapi/mux.go` alongside the existing routes
  (`mux.go:16-89`), following the read-route pattern of `handleTasksList`
  (`mux.go:91-108`). It returns every active worktree (running or parked) with its
  bound `{ repo, issue/Task, worktree path, agent session id, session transcript
  path, run/gate state }`.
- Orchestrator boundary (per human direction): the endpoint SUPPLIES the raw
  fields needed to CONSTRUCT a VSCodium link to the session — notably the worktree
  path/cwd, the agent session id, and the session transcript path — but it does
  NOT itself emit a VSCodium link. Generating the `vscodium://` / resume link to
  "talk to the agent" is a downstream/consumer concern (the `obsidian-operator`
  command and/or the human's future review surface), NOT an orchestrator concern.
  The exact link form (how to open/resume a running Claude/Codex session in
  VSCodium for live interaction) is decided and, if needed, researched on the
  consumer side. This task's hard guarantee for the endpoint is only that it
  returns fields SUFFICIENT to build that link; the backend never constructs it.
- Add an `obsidian-operator` skill capability/command — a new script under
  `skills/obsidian-operator/scripts/` (e.g. `Get-ActiveWorktreeSessions.ps1`) or a
  documented command in `skills/obsidian-operator/SKILL.md` — that calls the
  endpoint and enumerates the sessions bound to each worktree (printing the
  worktree path, issue/Task, run/gate state, and the session transcript path) so
  the operator can open the right session in VSCodium to kick a parked or slow
  agent along or for special interactions.
- OUT OF SCOPE (sibling task, see `## Non-Goals`): the Obsidian dashboard review
  surface / UI for surveying needs-human work. This task delivers only the
  enumeration endpoint + operator command that the surface will consume.

### Cross-cutting

- Add a named REGRESSION.md case for the new operator-facing behavior (the
  queue-drain consumer end-to-end on an isolated lane, including the incident
  email on a simulated stall). New human-facing/operational behavior must be
  covered by a named case (`REGRESSION.md:23`).

## Expected Resolution

The operator sets `Queue=Ready` on a Ready issue and, without any further manual
step, sees a worktree spin up and a top-level agent begin work; sees up to four
such worktrees running at once for the repo; sees a slot free and the next Ready
issue picked up ONLY when an issue is CLOSED — and the issue is only ever closed
by the operator's own explicit closure approval, never by the agent. When an agent
believes the work is complete, the operator sees the task PARKED "awaiting closure
approval" (`Human Needed=Yes`) — worktree and slot retained — and must give an
explicit closure directive to close it; a cleared pass, regression, plan, or
research gate does NOT close the task. When an agent marks `Human Needed=Yes`
(research/plan/regression gate or abandon), the operator likewise sees the task
PARKED in place — its worktree and slot retained, no redispatch — and resumable in
the same worktree once they clear the gate (or, after review, explicitly close the
issue to accept or abandon it); the operator understands that parked tasks reduce
free concurrency (`queue_workers` minus parked tasks) until cleared or closed. When
an agent silently stalls WHILE WORKING, the operator receives
within ~5 minutes one incident email containing the watchdog's observed state and
the agent's session transcript; a parked task never generates such an email. For
any active worktree the operator can query the worktrees endpoint (and the
`obsidian-operator` command that calls it) to see the bound session and its
transcript path and open that exact session in VSCodium. The GitHub issue remains
the single queryable place the operator can look to know what the queue thinks
each task's state is.

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

## What Does Not Count

- A `queue_workers` field present while the backend still enforces 1:1 dispatch.
- A second worktree that exists only sequentially (one finishes before the next
  starts) presented as "N>1 concurrency."
- A dispatched agent launched as a nested subagent that cannot spawn its own
  subagents.
- The agent self-closing the issue for ANY reason — abandon OR perceived
  completion — instead of setting `Human Needed=Yes` and asking for an explicit
  human closure approval.
- Treating a pass, regression-run, plan, or research gate approval as task-closure
  approval. Closure is a distinct, final human gate.
- The consumer auto-closing the issue from a local "complete"/terminal status
  without an explicit human closure approval.
- A consumer that frees the slot, reclaims the worktree, or redispatches the issue
  when a task is marked `Human Needed=Yes` (parked). Only a CLOSED issue
  deallocates; parking on the research/plan/regression gate must retain the
  worktree and slot in place.
- A watchdog that pokes, raises a stall incident, or emails about a task that is
  legitimately parked needs-human, or that cannot tell "parked on a human gate"
  from "fell asleep mid-work."
- A watchdog that declares "asleep" purely on a fixed timer with no
  evidence-backed liveness signal, or that never sends an email, or sends an email
  missing the observed state or the session transcript.
- A worktree↔session binding kept only in memory or scattered logs, with no
  backend endpoint to enumerate active worktrees and their bound session +
  transcript path + state, and no `obsidian-operator` command to surface it.
- Building the Obsidian review-surface UI here instead of the sibling task (this
  task delivers only the enumeration endpoint + operator command).
- Treating a local terminal-status enum (not the GitHub issue) as the queryable
  source of truth for queue decisions.
- Proof run against the human's live service-lane config/DB or live GitHub writes
  beyond what is explicitly gated (REGRESSION.md isolation rule, `REGRESSION.md:11-13`).
- A second/duplicate GitHub-write path that bypasses the obsidian-operator skill.

## Proof Plan

- Run all backend proof on the isolated validation lane (backend
  `http://127.0.0.1:14318`, Temporal `127.0.0.1:17233`, Postgres `15432`,
  runtime root `...\orchestration-validation-lane`) per `REGRESSION.md:14-19`,
  with task-owned fixtures — never the human's live config/DB.
- O2 concurrency proof: capture two live worktree checkouts under the owned-lane
  root with two concurrent task runs for one repo (e.g. `git worktree list` plus
  the two active run views), stored under `Tracking/Task-0015/Testing/`.
- O3 consumer proof: with the consumer pointed at a task-owned fixture/test issue
  set (NOT live GitHub writes beyond an explicitly gated test issue), flip
  `Queue=Ready` and show the consumer dispatching without a manual call; show a
  `Queue=Never` issue ignored.
- O4 done-contract proof: drive the abandon/gate path and show `Human Needed=Yes`
  + issue still open + the worktree STILL PRESENT and the slot STILL counted as
  used (parked, not freed); then resume in the SAME worktree after clearing the
  gate (same `OwnedRepoRoot`, no re-provision). Drive the perceived-completion path
  and show the agent does NOT self-close: it parks `Human Needed=Yes` "awaiting
  closure approval" with the issue STILL OPEN and the worktree STILL PRESENT, and
  show that a cleared pass/regression/plan/research gate does NOT close it; THEN
  apply an explicit human closure approval (the human-gated obsidian-operator close
  command) and show ONLY THEN the issue closed AND the worktree reclaimed + slot
  freed. Also show the consumer does NOT auto-close from a local "complete" status.
  Show the 4-slot-minus-parked free-concurrency accounting (counting
  awaiting-closure parks). Keep any live GitHub interaction dry-run-first per the
  skill's guardrail (`skills/obsidian-operator/SKILL.md:96-108,161-168`).
- O5 liveness proof: produce the research artifact pinning the signal; then in a
  controlled stall repro (a dispatched, actively-working agent that goes idle),
  show detection within the window, one poke, and an incident email containing the
  observed state + transcript. The email send may be captured/mocked on the
  isolated lane; the artifact must show the full email contents. Separately, in a
  controlled PARKED repro (a run marked `Human Needed=Yes` that sits idle past the
  stall threshold), show the watchdog stays silent — no stall transition, no poke,
  no email — proving it distinguishes parked from asleep.
- O6 binding/endpoint proof: after a dispatch, show the owned-lane record carries
  the populated binding fields; call the worktrees endpoint and show it returns
  the active worktrees (running and parked) with bound session id + transcript
  path + run/gate state; run the `obsidian-operator` enumeration command and show
  it prints, per worktree, the worktree path, issue/Task, run/gate state, and the
  transcript path. Stored under `Tracking/Task-0015/Testing/`.
- Cross-cutting: `go test ./...` from `backend/orchestration` (the repo's Go test
  command, confirmed by Task-0008 backend smoke artifacts, e.g.
  `Tracking/Task-0008/Testing/PASS-0002-BACKEND-SMOKE-0001.md:26`); add/exercise
  the named REGRESSION.md case.

## Falsifiers

- F-O1: After the change, `CODEX-REPO-MANIFEST.json` still resolves in any
  live-code reference, OR a `repos[]` entry lacks an integer `queue_workers`, OR a
  historical `Tracking/`/`.codex` artifact was rewritten by the rename.
- F-O2 (N>1): It is impossible to run more than one worktree for one repo at the
  same time, OR `deriveDispatchReadiness` still blocks a same-repo dispatch while a
  free slot exists. Only a config field changed, not real concurrency.
- F-O3: Flipping `Queue=Ready` does not cause the consumer to dispatch, OR a
  `Queue=Never` issue gets dispatched, OR the consumer requires a manual dispatch
  call.
- F-O4-abandon (HARD): An abandoning agent self-CLOSES the issue (instead of
  setting `Human Needed=Yes` and leaving it open), OR `Human Needed=Yes` triggers
  an auto-redispatch.
- F-O4-closure (HARD): The agent self-closes the issue for ANY reason (abandon OR
  perceived completion) without an explicit human closure approval; OR a pass /
  regression / plan / research approval is treated as closure approval; OR the
  consumer auto-closes the issue from a local "complete"/terminal status without an
  explicit human closure approval. (Closure is a distinct, final human gate; the
  agent NEVER self-closes.)
- F-O4-park (HARD): A task marked `Human Needed=Yes` (research/plan/regression
  gate, abandon, or awaiting closure approval) has its worktree reclaimed, its slot
  freed, or is redispatched, instead of being PARKED in place; OR a parked task
  cannot be resumed in the same worktree and must be re-provisioned. (Only a
  human-approved CLOSED issue may deallocate.)
- F-O4-source: Queue decisions are driven by a local enum rather than the GitHub
  issue state.
- F-O5-toplevel (HARD): The dispatched agent cannot spawn its own subagents (it
  was launched as a nested subagent).
- F-O5-signal (HARD): The liveness "asleep" decision is made with no durable
  research artifact pinning the observable signal (a "feels alive" / fixed-timer
  proxy).
- F-O5-detect+email (HARD): A stalled (actively-working) agent is NOT detected
  within the stated window, OR no incident email is produced, OR the email omits
  the observed state or the session transcript.
- F-O5-parked (HARD): A task legitimately parked needs-human triggers a stall
  transition, a poke, or an incident email (a spurious stall incident), OR the
  watchdog cannot distinguish "parked on a human gate" from "fell asleep
  mid-work."
- F-O6 (HARD): No backend endpoint enumerates active worktrees with their bound
  session id + transcript path + run/gate state, OR the owned-lane record does not
  carry the binding, OR no `obsidian-operator` command surfaces it, OR a parked
  needs-human worktree/session is not listed by the endpoint/command.
- F-X: `go test ./...` fails or does not build, OR no named REGRESSION.md case
  covers the new behavior, OR proof was run against the human's live config/DB or
  performed ungated live GitHub writes.

## Internal Mechanism Map

| Sub | Mechanism | Acts at | Reduces | Acceptance / Falsifier |
| --- | --- | --- | --- | --- |
| O1 | Rename manifest + add `queue_workers` int | `REPO-MANIFEST.json` + named live-code refs | No way to size per-repo slots; stale Codex-only filename | A1.1–A1.4 / F-O1 |
| O2 | Per-repo slot semaphore + pending queue; relax 1:1 gate | `internal/queue` + `service.go:747-809,968-986` | Only one task per repo can run at a time | A2.1–A2.3 / F-O2 |
| O3 | Temporal queue-drain workflow + HTTP start endpoint + `StartWorker` registration | new `internal/queue`, `httpapi/mux.go`, `temporalbackend/backend.go` | `Queue=Ready` does nothing today | A3.1–A3.4 / F-O3 |
| O4 | Issue-state done-contract: agent NEVER self-closes; closure ONLY on explicit human approval (distinct from pass/regression/plan/research gates); `Human Needed=Yes` (incl. "awaiting closure approval") PARKS in place (retain worktree+slot, no redispatch); only human-approved CLOSE deallocates; consumer reads issue state | `obsidian-operator` scripts/SKILL + consumer + owned-lane gate state | Ambiguous/local-only terminal state; agent self-closing on abandon OR perceived completion; gate approvals mistaken for closure; auto-close from local terminal status; expensive worktrees bumped at human gates | A4.1–A4.7 / F-O4-abandon, F-O4-closure, F-O4-park, F-O4-source |
| O5 | Top-level agent dispatch + external liveness watchdog (research-pinned signal, SUSPENDED while parked) + one poke + incident email | dispatch launcher + Temporal monitor (`service.go:226-255,1214-1244,1254-1268`) + gmail path | A silently dead agent holds a slot forever; operator never learns; spurious stall incidents on parked tasks | A5.1–A5.5 / F-O5-toplevel, F-O5-signal, F-O5-detect+email, F-O5-parked |
| O6 | Worktree↔session binding on owned-lane record + backend enumeration endpoint + operator command | `RepoLane`/`bootstrapOwnedLane` (`service.go:988-1026`, `types.go:121-138`) + `httpapi/mux.go` + `obsidian-operator` | Operator cannot find which session belongs to a parked/slow worktree to kick it along | A6.1–A6.4 / F-O6 |

Internal separability: O1 is shippable/reviewable on its own (rename + field). O2
is provable on its own (two concurrent worktrees) before any GitHub polling. O3
can be exercised against a fixture issue set. O4 is provable by driving each issue
transition — perceived-completion ⇒ park "awaiting closure approval" (open, no
self-close), explicit human closure approval ⇒ closed (deallocate), and
`Human Needed=Yes` (park-in-place + resume) — plus showing a cleared
pass/regression/plan/research gate does not close the task.
O5's research artifact and watchdog are provable independently of the queue-drain
loop using a controlled stall repro, and its parked-suspension is provable with a
separate gate-parked-idle repro. O6's binding + endpoint + operator command are
provable against a single dispatched (and a single parked) worktree without the
full drain loop.

## Constraints And Baseline

- This touches the LIVE backend service, a LIVE manifest, and the operator skill.
  All tests/validation run on an isolated lane with task-owned fixtures. Do NOT
  point validation at the human's live config/DB; do NOT perform live GitHub
  writes beyond what is explicitly gated. Honor `REGRESSION.md` (isolation rule
  `:11-13`), `DATA-HANDLING.md`, and `TESTING.md`.
- Backend Go changes must build and pass the repo's Go test command,
  `go test ./...` run from `backend/orchestration` (confirmed convention:
  `Tracking/Task-0008/Testing/PASS-0002-BACKEND-SMOKE-0001.md:24-26`).
- Keep destructive GitHub provider writes dry-run-first and human-gated per the
  obsidian-operator guardrails (`skills/obsidian-operator/SKILL.md:96-108,161-168`).
- New human-facing/operational behavior gets a named REGRESSION.md case
  (`REGRESSION.md:23`).

## Open Questions

These do not change the writeup type, home, or chosen solution shape (they are
implementation-detail decisions for the executor / coordinator review), but are
recorded for transparency:

- O5 signal: the exact observable liveness signal is intentionally left to the
  required research artifact (A5.2). This is the embedded research item, not a
  gap in the task shape.
- O2 slot mechanism: `RepoSlotManagerWorkflow` vs an equivalent durable slot
  counter is left as an implementation choice; the acceptance criteria (real
  concurrent worktrees + a refused/queued 5th dispatch) constrain the behavior,
  not the exact Temporal object.
- O3 poll cadence and the start/stop endpoint shape are implementation details;
  the acceptance criteria fix the observable behavior.

## References

- Human directives + decisions D1–D4 and the 2026-05-29 revisions (parked
  needs-human, suspended watchdog, O6 binding/endpoint, and the closure tightening:
  agent NEVER self-closes; closure only on explicit human approval; gate approvals
  are not closure approval; deferred voluntary-worktree-release Non-Goal):
  `Tracking/Task-0015/HUMAN-DIRECTIVES-FOR-WORKER.md`.
- Objective + current truth: `Tracking/Task-0015/TASK-CREATE-OBJECTIVE.md`.
- Context manifest: `Tracking/Task-0015/TASK-CREATE-CONTEXT-MANIFEST.md`.
- Owned-lane dispatch + worktree machinery:
  `backend/orchestration/internal/taskrun/service.go:126-185,934-1045,968-986`.
- 1:1 dispatch gate: `backend/orchestration/internal/taskrun/service.go:747-809` (`:777-781`).
- Owned-lane bootstrap record (O6 binding home):
  `backend/orchestration/internal/taskrun/service.go:988-1026` (`bootstrapOwnedLane`).
- `RepoLane` type (O6 binding fields home): `backend/orchestration/internal/taskrun/types.go:121-138`.
- Liveness / poke / escalation: `service.go:226-255,1214-1244,1254-1268`;
  `types.go:20,36,47,186`; `taskexec.go:81,126,165`.
- `TaskRunWorkflow`: `backend/orchestration/internal/taskexec/taskexec.go:40-118`.
- Worker registration: `backend/orchestration/internal/temporalbackend/backend.go:134-156`.
- HTTP routes (route registration + read-route pattern for O6 endpoint):
  `backend/orchestration/internal/httpapi/mux.go:16-89,91-108,122-145`.
- Obsidian operator authority + scripts (home of O4 done-contract writes —
  `Human Needed=Yes` on abandon/awaiting-closure, agent never self-closes — the
  human-gated CLOSE path, and the new O6 enumeration command):
  `skills/obsidian-operator/SKILL.md`;
  `skills/obsidian-operator/scripts/`;
  `Sync-TaskToGitHubIssue.ps1:300,455-461,548-553`;
  `Reconcile-TaskGitHubState.ps1:140-141,557-577`.
- Manifest + DATA-HANDLING: `CODEX-REPO-MANIFEST.json`; `DATA-HANDLING.md:20,161,239`.
- Rename live-code refs: `skills/obsidian-operator/SKILL.md:3,10`;
  `Bootstrap-TaskGitHubIssues.ps1:4`; `Sync-TaskToGitHubIssue.ps1:6`;
  `Reconcile-TaskGitHubState.ps1:4`; `tests/test_obsidian_title_roundtrip.py:110,154`;
  `app/codex_dashboard/paths.py:52`.
- Regression isolation + named-case rule: `REGRESSION.md:11-13,23`.
- Style/quality exemplar (structure only, not scope): `Tracking/Task-0013/TASK.md`.

## Audit Status

Human-approved on 2026-05-29: the human reviewed the coordinator-reviewed draft
(through v3 + the O6 boundary edit) and approved it ("Agreed, great"), authorizing
creation of the bound GitHub issue and a TaskDispatch run. TaskCreate model of
2026-05-29: coordinator + blind writer-worker with a coordinator concreteness
review (no separate agent-auditor lane). See
[TASK-CREATE-COORDINATOR-NOTES.md](./TASK-CREATE-COORDINATOR-NOTES.md).

Provider-binding gate SATISFIED: GitHub issue #15 created issue-first at the
matching number, typed `Task`, and bound by [TASK-META.json](./TASK-META.json)
(reconcile clean). The task is created and is now under a TaskDispatch run
(coordinator: this Codex/Claude instance) — see
[TASK-DISPATCH-SOURCE-PROMPT.md](./TASK-DISPATCH-SOURCE-PROMPT.md).
