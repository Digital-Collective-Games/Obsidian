# Human Directives For Codex — Task-0015

Captured for Task-0015 from the current Codex/Claude session on 2026-05-29.
These directives are authoritative scope. Human directives override subagent,
coordinator, and agent-auditor preferences.

## Verbatim Human Directive — feature shape (2026-05-29)

> Yeah so the workflow I'd want to use is temporal backed, i think its better if
> its temporal that handles queue draining, worktree allocation, there needs to
> be a durable contract executed by the TaskDispatch agent to signal im done
> though - and temporal should watch the agent it dispatched, and if it isn't
> thinking then poke it - "hey, either write a durable stop update or get back to
> work" (something like that).
>
> I think that would work for the consumer. Then we'd need some kind of
> configuration to say how many slots we want each repo to own, there's alreay a
> json file indexing the repos in
> C:\Agent\CodexDashboard\CODEX-REPO-MANIFEST.json, obviously rename that to
> REPO-MANIFEST.json now that claude is running along side codex. My thought would
> be to have a # of queue workers, which translates to "how many worktrees" that
> temporal would be able to dispatch to.

## Verbatim Human Directive — the four design decisions (2026-05-29)

> 1 - yep like N = 4 or so typically, but > 1 for sure
>
> 2 - needs human state is owned by the gh issue (so its queryable), and closed
> is the only terminate state for a task regardless if its "fixed not doing it" or
> "closed bad idea", and if the agent that is assigned to that worktree figures
> out that the task should be abandoned while doing it, the gh needs human state
> is the appropriate one to choose
>
> 3 - yes, correct, agent shouldn't know its being supervised if at all possible,
> shouldn't just be a subagent since it needs to run subagents (hard requirement
> there)
>
> 4 - well you should know within 1min if a poke is necessary (in vscodium its
> always apparent when claude is thinking, if they're not, ive never seen them
> "come back to thinking" so i think whatever mechanism that is, we should use on
> temporal side so we can quickly confirm "this agent fell asleep". To be safe
> maybe 5min, and log an incident email with the state seen and session transcript

## Worker-Safe Normalization (AUTHORITATIVE resolved decisions)

### D1 — Concurrency (per-repo slots)
- Build REAL N>1 concurrency, not just a config field. The current backend is
  hard 1:1 (it blocks dispatch while any active owned lane exists for the repo);
  supporting multiple concurrent owned lanes per repo is in-scope net-new work.
- `queue_workers` is a per-repo integer in the renamed `REPO-MANIFEST.json`.
  Default ~**4**. `queue_workers` == max concurrent worktrees Temporal may
  dispatch for that repo. Must be > 1 capable.

### D2 — Task state lives on the GitHub issue (queryable), not a local enum
- The ONLY terminal state for a task is the **GitHub issue being CLOSED**. This
  covers BOTH "done / fixed" and "won't-do / bad idea / fixed-not-doing-it" — the
  distinction is NOT a separate machine state; both are simply "closed."
- **"Needs human" is owned by the GitHub issue's `Human Needed` field = `Yes`**
  (the existing org issue field; queryable via `gh`). It is a non-terminal,
  human-gated pause.
- If the agent assigned to a worktree decides mid-work that the task should be
  abandoned / is a bad idea, it does NOT close the issue itself. It sets
  **`Human Needed = Yes`** and stops; the human then closes-as-bad-idea or
  redirects. Closing for abandonment is a human action.
- The consumer makes queue decisions from GitHub issue state: `Queue=Ready` =>
  eligible to dispatch; issue `closed` => terminal, free the slot; `Human
  Needed=Yes` => stop, free the slot, leave open for the human (do not
  auto-redispatch). Do NOT invent a parallel local terminal-status enum as the
  queryable source of truth; the GitHub issue is the queryable source of truth.
- A local durable "done" signal (e.g. TASK-STATE.json terminal status) may still
  be used internally to let the backend stop supervising a run, but the
  human-queryable state is the GitHub issue.

### D3 — The dispatched agent is a full, unsupervised-feeling, top-level agent
- The agent dispatched into a worktree MUST be able to run its OWN subagents.
  HARD REQUIREMENT: it must NOT be launched as a nested subagent of a parent that
  would prevent it from spawning subagents. Launch it as a top-level agent
  process (e.g. a headless `codex`/`claude` run) in its owned worktree.
- The agent should NOT know it is being supervised, if at all possible. Temporal
  supervises it EXTERNALLY by observing durable side-effects (its log/transcript
  activity, git activity, files), not by injecting supervision the agent is aware
  of. The watchdog is invisible to the agent.

### D4 — Liveness ("fell asleep") detection + poke + incident email
- Detection must be FAST: the human can tell within ~1 minute whether the agent
  is "thinking." A stalled agent does NOT spontaneously resume ("I've never seen
  them come back to thinking"). So Temporal must detect "this agent fell asleep"
  using the same kind of signal that makes "thinking vs not thinking" obvious in
  the IDE (an active-inference / active-turn signal), applied on the Temporal
  side against the headless agent's observable activity.
- KNOWN UNKNOWN (requires durable research before/within implementation): exactly
  what observable signal reliably distinguishes "actively working/thinking" from
  "fell asleep / idle-waiting" for a HEADLESS agent process (candidates: session
  transcript/log file growth, process busy/idle, an in-flight model request,
  stdout activity). The task must identify and document this signal in a durable
  research artifact, because the whole watchdog depends on it.
- Threshold: you'd know within ~1 min; use ~**5 min** of no active-inference
  signal to be safe before declaring "asleep" and acting.
- Poke semantics: the nudge is "either write a durable stop update (set
  `Human Needed=Yes` or close the issue) or get back to work." Because a slept
  agent typically won't self-recover, the poke must actually attempt to WAKE the
  process (deliver input), and if it does not resume:
- On a confirmed sleep / failed poke: **log an INCIDENT and EMAIL the human** with
  (a) the observed state seen by the watchdog and (b) the agent's session
  transcript. (Use the configured Gmail path / gmail email skill.)

## Verbatim Human Directive — revisions (2026-05-29, after draft v1)

> For now, let's make human-needed not deallocate the worktree. I see what you're
> saying, but for UE flipping the worktree could be an expensive operation, and i
> have research gate, plan gate, and regression gate, each of those should not be
> bumped. I will have a review surface be added to Obsidian, so i can survey any
> "needs human" work, so I don't anticipate being blocked for long. I will need an
> additional capability in obsidian-operator skill to enumerate the sessions bound
> to each worktree so i can follow up in vscodium to kick them along manually, or
> for special interactions. So there should be an obsidian endpoint I would think
> to supply that information?

## Worker-Safe Normalization of the revisions (AUTHORITATIVE — overrides draft v1)

### D2/O4 revision — `Human Needed=Yes` RETAINS the worktree and the slot
- `Human Needed=Yes` is a PARKED pause, NOT a deallocation. When a task is parked
  needs-human, the consumer MUST: keep the owned worktree allocated, keep the
  slot occupied, and NOT redispatch the issue. The task is resumable IN PLACE in
  the same worktree.
- ONLY the terminal state (GitHub issue CLOSED) deallocates the worktree and frees
  the slot. (This supersedes draft v1's "Human Needed=Yes => free the slot.")
- Rationale: re-provisioning a worktree can be expensive (especially Unreal
  Engine), and the three human-approval gates below are normal mid-task pauses
  that must not cause the task to be bumped (evicted / torn down / redispatched).
- The three named human gates that must NOT be bumped (TaskDispatch lifecycle):
  **research gate, plan gate, regression gate.** A task parked at any of these
  sets `Human Needed=Yes`, keeps its worktree+slot, and resumes in place once the
  human clears the gate.
- Slot-accounting consequence (accepted by the human): effective free concurrency
  = `queue_workers` minus the number of parked needs-human tasks. The human will
  add an Obsidian review surface to survey needs-human work and does not expect to
  be blocked long.

### O5 revision — watchdog is SUSPENDED while parked on a human gate
- A task legitimately parked needs-human (`Human Needed=Yes`, e.g. at a
  research/plan/regression gate) is NOT "asleep." The liveness watchdog MUST be
  suspended for that run while parked: no stall detection, no poke, no incident
  email for a gate-parked task. The watchdog distinguishes "parked on a human
  gate" (expected, indefinite) from "fell asleep mid-work" (unexpected; poke +
  incident per D4).

### NEW O6 — worktree↔session binding registry + enumeration endpoint
- At dispatch, the consumer records the binding for each owned worktree:
  `{ repo, issue #N / Task-N, worktree path, agent session id, session transcript
  path (e.g. ~/.codex/sessions/… or ~/.claude/projects/…), run/gate state }`,
  attached to the owned-lane record.
- Expose it via a backend ("Obsidian") HTTP endpoint (e.g.
  `GET /api/v1/worktrees` or `/api/v1/queue/lanes`) returning the active
  worktrees with their bound session + transcript path + state.
- Add an `obsidian-operator` skill capability/command that calls the endpoint and
  enumerates the sessions bound to each worktree, so the human can open the right
  session in VSCodium to kick a parked/slow agent along, or for special
  interactions.
- OUT OF SCOPE here (the human's sibling task): the Obsidian dashboard REVIEW
  SURFACE / UI itself for surveying needs-human work. THIS task provides the
  enumeration endpoint that surface will consume; it does not build the UI.

## Verbatim Human Directive — closure tightening (2026-05-29, after draft v2)

> Yeah, we better tighten the circumstances under which tasks are allowed to be
> closed. Only a human's explicit approval for task closure after work has been
> completed. Approval for pass, regression run, etc are not a proxy for task
> closure approval. If the agent believes the task should be closed, it must still
> ask for an explicit directive.
>
> We'll probably need to add semantics for an agent deciding it no longer needs a
> worktree, but let's not complicate matters right now.

## Worker-Safe Normalization of the closure tightening (AUTHORITATIVE — overrides earlier drafts)

### D2/O4 FURTHER revision — the agent NEVER self-closes; closure = explicit human approval
- The ONLY way a task reaches its terminal state (GitHub issue CLOSED) is an
  EXPLICIT human closure approval/directive, given AFTER the work is completed.
- The dispatched agent MUST NEVER self-close the issue — not on abandon, and NOT
  on perceived successful completion. (This supersedes the earlier draft text that
  had the agent close the issue on success.)
- When the agent believes the task is complete (or should be abandoned / closed as
  a bad idea), it sets `Human Needed=Yes` and PARKS in place (worktree + slot
  retained, per the park revision), with its run/gate state indicating it is
  "awaiting closure approval." It then asks for an explicit closure directive. The
  human's explicit approval is what closes the issue — performed by the human
  directly or via the human-gated `obsidian-operator` close path; never by the
  agent autonomously.
- Approvals for OTHER gates — research, plan, pass, regression run — are NOT a
  proxy for task-closure approval. Closure is a DISTINCT, FINAL human gate.
  Passing tests, a regression run, or a plan approval NEVER authorizes closing the
  task.
- Consumer consequence: a local "complete"/terminal status is NOT sufficient to
  close the GitHub issue in the autonomous queue flow. The close MUST be gated on
  explicit human closure approval. (This intentionally tightens the existing
  close-from-local-terminal-status behavior for the queue-driven path.)
- Worktree lifecycle: an "awaiting closure approval" state is a PARK
  (`Human Needed=Yes`) — worktree + slot retained, watchdog suspended (O5),
  worktree/session still enumerable (O6) — until the human explicitly approves
  closure. ONLY the human-approved CLOSE then deallocates the worktree and frees
  the slot.

### Deferred (explicitly OUT OF SCOPE now)
- Semantics for an agent voluntarily deciding it no longer needs its worktree
  (releasing the worktree without a task closure). The human flagged this as
  likely-future but said not to complicate matters now. Do NOT build it in this
  task; record it as a deferred Non-Goal.

## Scope / Process Notes
- This is ONE coherent feature ("the Temporal-backed consumer") with internally
  separable sub-objectives. Do not narrow/split on auditor preference; if you
  believe it should be staged, raise it as a human-gate note, not a unilateral
  split. Keep per-sub acceptance criteria and falsifiers so sub-objectives remain
  independently reviewable and shippable.
- The manifest rename (CODEX-REPO-MANIFEST.json -> REPO-MANIFEST.json) updates
  the file plus ONLY the live-code references; historical Tracking/ artifacts and
  `.codex` session transcripts are durable history and must NOT be rewritten.
- This is a backend (Go + Temporal) + manifest + operator-skill feature. Reuse the
  existing owned-lane worktree machinery rather than reinventing it.
