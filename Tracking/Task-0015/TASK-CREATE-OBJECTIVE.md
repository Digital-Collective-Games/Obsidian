# Task-0015 TaskCreate Objective

Worker-safe objective for the TaskCreate writer-worker. States WHAT and WHY; the
writer chooses the honest writeup type and writes falsifiable acceptance criteria
per `C:\Users\gregs\.codex\Orchestration\Processes\TASK-CREATE.md`. The solution
shape is constrained by the human's decisions in `HUMAN-DIRECTIVES-FOR-WORKER.md`
(do not re-open D1–D4).

## One-Line Objective

Build a Temporal-backed consumer that drains the GitHub task queue: it watches the
provider repo for issues with `Queue=Ready`, allocates a git worktree per task
(up to a configurable per-repo slot count), dispatches a full top-level coding
agent into it, supervises that agent's liveness externally (detect "fell asleep",
poke, escalate via incident email), and frees the slot when the task reaches its
terminal GitHub state — then pulls the next Ready issue.

## Human-Facing Outcome (state first)

Today, flipping a GitHub issue's `Queue` field to `Ready` does nothing — no
process consumes it (verified: nothing reads the Queue field to dispatch; "drain
my queue" was explicitly deferred from Task-0012). The operator wants: set an
issue's `Queue=Ready`, and Temporal automatically picks it up, runs an agent on it
in an isolated worktree until the task is closed (or flagged needs-human), then
reclaims the worktree and starts the next Ready task — bounded by a per-repo
number of concurrent "queue workers" (worktrees). If a dispatched agent silently
stalls, the operator is notified by email with the agent's state and transcript.

## Current Truth (from evidence; do not re-derive — cite as needed)

- **Nothing consumes the Queue field today.** Task-0012 made "central queue
  draining" / "dispatching from GitHub Issues" explicit Non-Goals (`Tracking/
  Task-0012/TASK.md:421-422`). `DrainQueueDemo` is a manual PoC over a local
  `queue.json`, not GitHub. The Queue field is only WRITTEN by sync and READ by
  reconcile for drift reporting.
- **Worktree allocate/reclaim ALREADY EXISTS** in the backend and is reusable
  as-is: `provisionOwnedLane` / `bootstrapOwnedLane` / `cleanupOwnedLane`
  (`backend/orchestration/internal/taskrun/service.go:934-1045`), driven by
  `dispatchWithDirective` (`service.go:126-185`), under the Temporal
  `TaskRunWorkflow` (`backend/orchestration/internal/taskexec/taskexec.go:40-118`).
  The backend runs as a Windows scheduled-task service (Temporal + Postgres).
- **The backend is HARD 1:1 today.** `deriveDispatchReadiness` blocks dispatch
  when an active run exists for the repo (`service.go:747-809`, ~777-781);
  `releasePreviousOwnedLane` tears down the prior lane before a new dispatch
  (`service.go:968-986`). Supporting N>1 concurrent owned lanes per repo is
  net-new and is the largest piece of this task.
- **Liveness primitives already exist** to reuse: `StateEnvelope.SuspiciousAfter`
  + `LastProgressAt` (`taskrun/types.go:47,186`), `staleRunUpdate` auto-transition
  to `StateSleepingOrStalled` (`service.go:1254-1268`), `PokeRun` creating a
  `poke_worker_check` FollowUp with a 5-min DueAt (`service.go:226-255`), overdue
  FollowUp -> `AttentionUrgent` escalation (`service.go:1214-1244`). SuspiciousAfter
  is currently +15min for running states (`taskexec.go:81,165,...`).
- **Org issue fields exist and are queryable:** `Queue` (Never/Ready, id
  42656828), `Priority` (id 42656780), `Human Needed` (Yes/No, id 42656829), all
  on org `Digital-Collective-Games`, read/written via
  `/repos/<repo>/issues/<n>/issue-field-values`. The `obsidian-operator` skill
  already writes these and closes issues on task completion. NOTE: GitHub gates the
  issue Fields panel on the issue having a TYPE; the operator skill now sets
  `type=Task` (see Task-0014 fix). Issues are the durable queue.
- **Manifest:** `CODEX-REPO-MANIFEST.json` (repos array; `id`, `local_root`,
  `source_control_provider`, `task_provider` -> Digital-Collective-Games/Obsidian,
  `task_proposal_provider` -> .../ObsidianProposals). To be renamed to
  `REPO-MANIFEST.json` and given a per-repo `queue_workers` field.

## Chosen Solution Shape (constrained by D1–D4 — do not re-open)

A new backend package/workflow (recommended: `internal/queue`, a Temporal
queue-drain workflow sibling to `TaskRunWorkflow`, started by a new HTTP endpoint
and registered like existing workflows), composed of these internally-separable
sub-objectives:

1. **Manifest rename + `queue_workers`.** Rename `CODEX-REPO-MANIFEST.json` ->
   `REPO-MANIFEST.json`; update ONLY live-code references (see context manifest);
   add per-repo `queue_workers` int (default ~4). Leave historical Tracking/
   session artifacts untouched.
2. **N>1 per-repo concurrency (slots).** Replace the hard 1:1 readiness gate with
   a per-repo slot semaphore (recommended: a `RepoSlotManagerWorkflow` holding
   {repo -> used/limit} and a pending queue) allowing up to `queue_workers`
   concurrent owned lanes per repo; freeing a slot dequeues the next Ready issue.
   Reuse `provisionOwnedLane`/`cleanupOwnedLane` unchanged per slot.
3. **GitHub queue-drain consumer.** A Temporal workflow that polls the provider
   repo for `Queue=Ready` issues (the issues ARE the queue), maps issue ->
   `Tracking/Task-<n>` task, and dispatches into an available slot/worktree.
4. **Done-contract via GitHub issue state (D2).** Terminal = issue CLOSED (free
   slot). `Human Needed=Yes` = stop + free slot + leave for human (no
   auto-redispatch). The agent sets `Human Needed=Yes` to abandon/flag; it closes
   the issue on successful completion (reusing the obsidian-operator close path).
   The consumer reads issue state for all queue decisions.
5. **Liveness watchdog + incident email (D3, D4).** Dispatch the agent as a
   top-level process that can spawn its own subagents (NOT a nested subagent),
   invisibly supervised. A Temporal monitor detects "fell asleep" within ~5min
   using an active-inference signal (REQUIRES durable research to pin the exact
   observable signal for a headless agent), pokes it to resume-or-write-a-stop,
   and on confirmed sleep / failed poke logs an incident and EMAILS the operator
   the observed state + session transcript. Reuse SuspiciousAfter/PokeRun/FollowUp
   machinery; the agent emits no supervision-aware signal.

## Proof / Constraints The Draft Must Honor

- This touches the live backend service + a live manifest + the operator skill.
  Tests/validation must run on an isolated lane with task-owned fixtures; do NOT
  point validation at the human's live config/DB or live GitHub writes beyond what
  is explicitly gated. Honor `REGRESSION.md` / `DATA-HANDLING.md` / `TESTING.md`.
- The liveness-detection signal is a genuine UNKNOWN; per `TASK-CREATE.md`, require
  a durable research artifact pinning that signal before its implementation is
  called done (no "feels alive" proxy).
- Each sub-objective needs pass/fail acceptance criteria AND a falsifier. Hard
  constraints needing their own falsifier: (a) N>1 actually runs >1 concurrent
  worktree per repo (not just a field); (b) the agent can spawn its own subagents
  (not a nested subagent); (c) `Human Needed=Yes` is what an abandoning agent sets
  (it must NOT self-close for abandonment); (d) a stalled agent is detected within
  the stated window and produces an incident email with state + transcript.
- Go backend changes must build and pass `go test ./...` (or the repo's Go test
  command); confirm the exact command from the backend README.

## Audit-Readiness Notes
- Writeup type = `concrete implementation` with one embedded research item (the
  liveness signal). Do not downgrade the whole task to research.
- Keep the GitHub issue as the queryable state source (D2); do not reintroduce a
  parallel local terminal enum as the source of truth.
- Out of scope unless the human expands: a non-GitHub queue source; cross-repo
  global fairness beyond a simple per-repo slot cap (note default behavior);
  rewriting historical artifacts during the rename; changing the agent runtime
  (Codex vs Claude) selection logic beyond what dispatch needs.
