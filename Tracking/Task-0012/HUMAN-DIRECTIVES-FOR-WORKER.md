# Human Directives For Codex

Captured for Task-0012 from the current Codex session.

These directives are authoritative scope for Task-0012. Human directives
override subagent, coordinator, and implementation-surface preferences. Agent
auditor preferences are secondary to these human directives.

## TaskCreate And Research Setup

- Ground in the TaskCreate process before drafting.
- Capture the human need as task-owned research.
- Use the Chrome debug Slack tab to capture the current-day conversation with
  Alex.
- Put the capture under the task `Research/` subfolder.
- This is a CodexDashboard task.

## Product Direction From The Alex Discussion

- Alex's `gh` command-line approach to task state is valuable.
- The current local `TASK.md` docs are still useful, but CodexDashboard should
  use `gh` at least to query tasks from different repos.
- Research existing durable doc conventions before deciding the final split.
- Decide explicitly which state should have GitHub Issues and `gh` as source of
  truth, and which state should remain in local docs that can be pushed to
  GitHub for context.

## Required Rollout Sequence

1. Push a single `TASK.md` into a GitHub-backed representation.
2. Inspect that single published representation.
3. Agree that it meets the quality bar.
4. Then push all `TASK.md` files across all configured repos.
5. Then wire the `drain my tasks pls` workflow where a central coordinator
   allocates a worktree and dispatches work.

## Future Coordinator Shape

- The later central coordinator should be able to dispatch tasks on worktrees.
- Worktree management should include allocation and release.
- Active task execution should have a configured concurrency limit.
- Those coordinator, worktree, and concurrency behaviors are follow-on work,
  not Task-0012 closure.

## Human Override Of Audit Drift

- The active task is the restored first draft by human direction.
- Agent auditor preferences are secondary to human directives.
- Auditor feedback may identify real contradictions, missing proof, or durable
  rule violations, but it must not broaden this task beyond the human-selected
  first slice.
- The later audit/revision artifacts are preserved as history; they do not
  override this section's scope.

## Current Resume Directive (2026-05-28)

Verbatim human directive this session:

> Ground yourself in TaskDispatch, continue work on task 12

The human also pointed the resume at `Tracking/Task-0012/HANDOFF-GH.md`.

Worker-safe normalization (routing only, not a preferred solution):

- The canonical resume handoff for this task is
  [HANDOFF-GH.md](./HANDOFF-GH.md), not `HANDOFF.md`. There is no
  `CURRENT-TASK.json`; resolve the task from the `TASK_ID`/`REPO_ROOT` routing
  parameters and ground from disk.
- The human explicitly deleted `HANDOFF.md` earlier. Do not recreate it. Keep
  `HANDOFF-GH.md` as the GitHub-specific handoff.
- Standing instruction: continue the task toward honest closure. Reconcile any
  drift between `TASK-STATE.json` / `TASK.md` / `PLAN.md` and the work already
  proven under `Testing/`, then complete the remaining closeout obligations
  named in `HANDOFF-GH.md` "Next Steps".
- The `HANDOFF-GH.md` "Next Steps" and "Proposed Changes" are the prior
  session's recommendations preserved as durable task history. Evaluate them
  against the durable evidence; they are inputs to your judgment, not commands.
- Hold the line on scope: cross-repo bulk publication, the Review tab proposal
  workflow, implementing the TaskCreate provider-interface in shared `.codex`
  process docs, worktree allocation, concurrency limits, and
  `drain my tasks pls` remain follow-on work and are out of scope unless the
  human explicitly rescopes Task-0012.

## Explicit Rescope (2026-05-28) — Drain Queue Consumer

The human explicitly rescoped after the leader reached the publication-slice
closure gate. Verbatim directive this turn:

> Its not anywhere near ready, we pushed tasks to github but we need to setup a
> task queue consumer that allocates worktrees and pulls tasks assigning them to
> a subagent for completion ("drain my queue pls")

Worker-safe normalization (routing/scope only, not a preferred design):

- The publication/pilot slice is NOT the human's finish line. Do not present or
  accept Task-0012's closure gate. The required deliverable is the queue-drain
  consumer.
- Required capability ("drain my queue pls"): a task queue consumer that
  (a) pulls accepted tasks from the GitHub-backed queue, (b) allocates a
  worktree per task, (c) dispatches a subagent to complete each task,
  (d) respects a configured concurrency limit, and (e) releases worktrees when
  work finishes.
- This supersedes the earlier "drain is out of scope" guidance for the
  task-tracking home the human selects.
- Open forks being confirmed with the human before research/planning: whether
  this is tracked as a new task vs an expansion of Task-0012, and the consumer's
  execution model. Do not guess these; await the captured decision.

## Demonstration Directive (2026-05-28) — Prove It End-To-End

Verbatim follow-up directive this turn:

> Status update? If you can i'd like you to demonstrate the task queue with
> proof, along with worktrees, just put something in C:\Agent\YourTestRepo with
> a small repo, and ask it to do minor things to modify some program?

This resolves the execution-model fork: the human wants a Codex-operated
subagent-dispatch model ("assigns to a subagent", "drain my queue pls"), and a
working demonstration rather than a plan-only deliverable.

Worker-safe normalization (requirements, not a prescribed design):

- Build a minimal but genuinely working slice of the drain-queue consumer and
  prove it end-to-end. Design-only or dry-run-only does not satisfy this.
- Set up a small local git repo at `C:\Agent\YourTestRepo` containing a small
  program (your choice of a simple, self-contained program).
- Create a small queue of minor modification tasks against that program (small,
  low-risk edits — the point is to exercise the pipeline, not the difficulty).
- Run the consumer so it: pulls each queued task, allocates a git worktree per
  task, dispatches a subagent to complete that task (make the minor program
  modification), captures the result, and releases the worktree when done.
- Respect a configured concurrency limit during the run.
- Produce durable proof artifacts: the queue contents, worktree
  allocation/release records, per-task subagent results, the actual code
  diffs/commits the subagents produced in `C:\Agent\YourTestRepo`, and a
  consumer run log/summary. Proof must show real modifications, not just that
  endpoints/objects exist.
- Worktree allocation must use or extend the existing
  `C:\Users\gregs\.codex\Orchestration\WORKTREES.md` and
  `WORKTREE-ALLOCATIONS.json` conventions, not a parallel ad-hoc scheme.
- Demo queue backing store: prefer the lowest external footprint. A local queue
  representation is acceptable for the demonstration; the production queue is the
  already-proven GitHub provider. Do NOT create new outward-facing GitHub
  repositories without explicit human confirmation.
- Honesty: clearly label what is a demonstration/spike versus what would be
  production-ready, and record any limitation (nested-dispatch tooling limits,
  concurrency simplifications, local-vs-GitHub queue) as a caveat rather than
  smoothing it over.
