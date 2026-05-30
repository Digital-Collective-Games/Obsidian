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

## Verbatim Human Directive — queue detection = polling (2026-05-29, at plan gate)

> i think you convinced me with the "polling every 2m is cheap and robust" and
> doing a tunnel does seem like more moving parts than we need right now

### Worker-Safe Normalization (AUTHORITATIVE for O3)
- O3 detection mechanism = POLLING (pull), not push. The consumer polls GitHub
  for `Queue == Ready` on a configurable interval, default ~2 minutes.
- Webhooks / inbound push / any tunnel (smee / cloudflared / ngrok) are OUT OF
  SCOPE for this task: no public ingress on the localhost backend, org
  issue-FIELD-change webhook delivery is unverified, and it is not worth the
  moving parts now.
- The exact poll QUERY shape — one field-filtered search/GraphQL query IF GitHub
  supports filtering issues by the org `Queue` field, otherwise enumerate open
  issues and read each one's `issue-field-values` — is an IMPLEMENTATION DETAIL.
  Pick the cheaper supported form; observable behavior is fixed by O3 acceptance
  (A3.1–A3.4). It is not a blocker and not a separate research gate.

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

## Verbatim Human Directive — PASS-0006 regression surface = GitHub app surface (2026-05-29, at plan gate)

> App surface for pass 6 is github, use my chrome debug tab on a single test
> issue: set queue to Ready, then the backend must notice that and process it.
> Anything else is proxy evidence and unacceptable.

### Worker-Safe Normalization (AUTHORITATIVE for PASS-0006 / AX.2; settles the plan-gate Decision 2)
- The regression APP SURFACE for this feature is **GitHub** (the real GitHub
  issue web UI). The human-facing surface where the operator acts on this feature
  is the GitHub issue, NOT a desktop screen and NOT a backend-only fixture. This
  supersedes the earlier "operator-lane backend-only case" proposal AND any
  reading of TASK.md's Proof Plan that treated a fixture/isolated-lane-only
  queue-detection check as sufficient for regression closure.
- MECHANISM: the human's **Chrome debug session** (CDP — see the
  `chrome-session-scraper` tooling) driving the human's authenticated GitHub tab
  on a **SINGLE dedicated test issue**.
- REQUIRED FLOW (regression closure): in the real GitHub issue UI, set that
  issue's `Queue` field to `Ready` (a manual human browser action) ⇒ the
  backend's polling consumer must NOTICE the change and PROCESS it (dispatch the
  task into a worktree) with NO manual dispatch call.
- PROXY IS UNACCEPTABLE for regression closure: a fixture issue set, an
  issue-field-values API write that simulates the flip, or any backend-only test
  does NOT satisfy PASS-0006. (Per-pass DEV-LOOP proof during O3/PASS-0004 may
  still use a fixture for speed; that is iteration, NOT regression closure.)
- Named case: add the next free id (**REG-007**; REG-006 is taken by Task-0014)
  as a GitHub-app-surface case in `REGRESSION.md`; update `TESTING.md` if a lane
  note is needed. The O5 simulated-stall incident-email behavior is exercised
  separately on the isolated lane with the gmail send MOCKED/CAPTURED.
- F-X reconciliation: the manual `Queue=Ready` flip is the HUMAN's browser action
  (the authorized single-test-issue app-surface step), so it is NOT an agent
  "ungated live GitHub write." The dispatched agent's only GitHub interaction in
  this regression is READ-ONLY polling. F-X (no ungated live agent writes; no
  proof against the human's live config/DB) still holds.
- OPEN isolation sub-question to confirm BEFORE PASS-0006 runs: which backend
  instance polls/dispatches during the regression — the isolated validation-lane
  backend pointed at the test repo (default, preferred per `REGRESSION.md`
  isolation), vs. the live service-lane backend.

## Verbatim Human Directive — dedicated test repo for proof (2026-05-29, at plan gate)

> For the task you choose, it must be part of a test repo - make something new
> under C:\Agent so we can iterate on it arbitrarily

### Worker-Safe Normalization (AUTHORITATIVE — proof infrastructure for O2/O3/O4/O5/O6 + PASS-0006)
- All proof that involves the queue-drain consumer dispatching, flipping the
  GitHub `Queue` field, parking/closing issues, or an agent working in a worktree
  MUST target a NEW, throwaway TEST REPO created under `C:\Agent`
  (e.g. `C:\Agent\<TestRepo>`), with a matching GitHub repo so the org `Queue` /
  `Human Needed` issue fields can be flipped in the browser. NEVER the production
  `Digital-Collective-Games/Obsidian` repo/issues, and NEVER the live
  `CodexDashboard` repo, for these mutating proofs.
- The dispatched "task you choose" (the `Task-N` the consumer picks up and an
  agent works on) lives in this test repo, so issues can be created, flipped to
  `Queue=Ready`, parked, closed, and RESET arbitrarily without polluting any real
  queue.
- Setup (a task prerequisite before PASS-0001 proof): (a) create the local git
  repo under `C:\Agent` with a baseline commit so `git worktree add` works;
  (b) create the matching GitHub repo in the org (it inherits the org issue
  fields); (c) add a `repos[]` entry for it (with `queue_workers`) to
  `REPO-MANIFEST.json` so the consumer polls it. Creating the GitHub repo is
  OUTWARD-FACING — confirm the repo name/org with the human before creating the
  GitHub side.
- This test repo is the provider AND target for the proof (the issues that carry
  `Queue` and the worktree the dispatched agent works in), mirroring how
  `CodexDashboard` is its own provider+target via the manifest.

## Verbatim Human Directive — dispatch CLAUDE only, not codex (2026-05-30)

> 1 - don't dispatch codex on the tasks, only dispatch claude
> 2 - get claude working first

### Worker-Safe Normalization (AUTHORITATIVE for O5 — refines D3)
- The dispatched queue agent (O5/PASS-0005) is **`claude` ONLY** — never `codex`.
  The O5 launcher must launch a headless TOP-LEVEL `claude` process in the owned
  worktree (able to spawn its OWN subagents via the Task tool — NOT a nested
  subagent), and post-launch session discovery resolves the agent's OWN session id
  + transcript under `~/.claude/projects/<slug>/<session>.jsonl` (with per-subagent
  transcripts under `.../<session>/subagents/agent-*.jsonl`).
- This supersedes the PASS-0005 launcher's `codex exec` default. Remove/disable the
  codex dispatch path for the queue agent; keep claude as the only dispatched
  runtime. (The watchdog's transcript-append liveness signal already works for the
  Claude transcript shape per `Research/LIVENESS-SIGNAL.md`.)
- Priority: get the real claude dispatch working first (prove A5.1 — a real
  top-level claude spawns ≥1 of its own subagents, discoverable) before the
  PASS-0006 regression.

## Verbatim Human Directive — consumer cadence + don't touch real cron jobs (2026-05-30)

> i want the queue consumer to run all the time, and take no more than 1 min to
> notice an issue has been flagged for the queue
>
> I don't want you running things to activate jobs that are scheduled to run at 4am
> say though.
>
> i don't understand how temporal works; so take what i say with a grain of salt as
> far as how to set it up

### Worker-Safe Normalization (AUTHORITATIVE for O3 + all Temporal proofs)
- The queue-drain consumer runs ALWAYS-ON (continuous) and must NOTICE a
  `Queue=Ready` flip within **≤ 1 minute**. Set the default poll interval to **≤60s
  (use ~30s)**. This supersedes the earlier "~2 min default".
- NEVER trigger/activate the operator's REAL scheduled Temporal cron jobs (the
  daily/4am automations) — they are critical to the operator's daily pipeline. Run
  ALL queue-drain proofs in an ISOLATED Temporal namespace (e.g. `reg007`), NEVER the
  `default` namespace where the real jobs live, and never run a worker that would
  pick up the real job workflows. (Temporal setup details are the agent's call.)

## Verbatim Human Directive — registry-driven binding + OBSIDIAN_ env prefix (2026-05-30)

> Yeah this should be registry driven since the backend orchestrator needs global
> awareness. The registry should make no assumptions about where repos are located.
> The queue consumer must use the task provider to poll - you'll need to make that a
> first class citizen. Don't worry about the task proposals, that's coming later. But
> task provider must define the integration with local source.
>
> ... prefer OBSIDIAN_REGISTRY_PATH [for the env var, not a codex-prefixed name]

### Worker-Safe Normalization (AUTHORITATIVE for O3 binding)
- The central `REPO-MANIFEST.json` registry is the SINGLE SOURCE OF TRUTH; the
  backend orchestrator has GLOBAL AWARENESS (reads + enumerates ALL registered repos
  each poll). The registry makes NO assumption about where repos live — each
  `local_root` is an arbitrary absolute path taken verbatim.
- Each `repos[]` entry is the first-class binding: `{ id, local_root,
  source_control_provider, task_provider, queue_workers }`. The queue consumer polls
  via each entry's **`task_provider`** (FIRST-CLASS) — it is the abstraction that
  defines the integration with local source (`local_root` + `source_control_provider`).
  Dispatch lands in that entry's `local_root`, capped at that entry's `queue_workers`,
  with per-repo slot accounting.
- `task_proposal_provider` is OUT OF SCOPE for now (coming later).
- NEW env vars use the **`OBSIDIAN_`** prefix, not `CODEX_` (e.g.
  `OBSIDIAN_REGISTRY_PATH`). A global rename of the existing `CODEX_ORCHESTRATION_*`
  vars is a separate, riskier follow-up (it touches the live pinned service lane +
  LaneHelpers) — not done in this pass.

## Verbatim Human Directive — env-gated auto-closure + full-cycle deallocate/reuse regression (2026-05-30)

> Its true that worktrees should sit there allocated, but an important regression
> test is to simulate human approval for a close (only within the scope of that
> particular regression test is that allowed) then have the worker be given the
> directive to shut down once they announce they're done. You could code something
> like if the task-state is requesting closure, then if some env variable allows
> auto-closure for queued tasks, then do that. And follow it to its completion to
> make sure its deallocated, then throw another task at it to make sure the new slot
> can be reclaimed. Like you could have only 2 worker slots. In that case, allocate
> them both, let one or both complete, make sure they deallocate, then queue up
> another slot. This is all simulating what humans are going to assume with your surface.

### Worker-Safe Normalization (AUTHORITATIVE)
- PRODUCTION is UNCHANGED: the agent NEVER self-closes; only an explicit human
  closure approval closes a task and deallocates its worktree.
- TEST-ONLY affordance: an env-gated auto-closure (`OBSIDIAN_AUTO_CLOSE_QUEUED`,
  default OFF) SIMULATES the human approving a close, allowed ONLY within a
  regression test. When ON, if a dispatched run's TASK-STATE is REQUESTING CLOSURE
  (the agent announced done), the consumer auto-closes the GitHub issue (via the
  existing human-gated close path) → the existing close → reclaim → dequeue flow
  deallocates the worktree + frees the slot. Default OFF preserves the human-only
  closure rule (so this does not reintroduce a production auto-close-from-local-state).
- The dispatched worker is directed to: do its work → ANNOUNCE DONE (mark its
  TASK-STATE as requesting closure, e.g. `current_gate: "closure"`) → SHUT DOWN.
- Full-cycle regression (simulate the human-expected surface): with
  `queue_workers = 2`, allocate BOTH slots, let the agents complete + announce done,
  (env-gated) auto-close → verify BOTH worktrees DEALLOCATE + slots free, then queue
  ANOTHER task and verify it REUSES a freed slot. Prove allocate → complete →
  announce → (simulated-human) close → deallocate → reuse.
