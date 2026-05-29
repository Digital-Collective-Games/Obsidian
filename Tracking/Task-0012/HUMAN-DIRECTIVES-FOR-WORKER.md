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
