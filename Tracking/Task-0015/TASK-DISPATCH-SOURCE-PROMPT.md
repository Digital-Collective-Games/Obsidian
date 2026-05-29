# Task-0015 TaskDispatch Source Prompt

Preserved verbatim per the TaskDispatch coordinator contract (no compression /
rewording). Only initial whitespace normalization applied.

## Verbatim Human Launch Directive (2026-05-29)

> Agreed, great.  Then kick of TaskDispatch for it, you're the coordinator.

Context: "Agreed, great" approved the coordinator-reviewed Task-0015 draft (the
Temporal-backed GitHub queue-drain consumer) in the same session; "it" = Task-0015.

## Launch Normalization (worker-safe)

- `TASK_ID=0015` (the approved task; issue #15, typed `Task`).
- The coordinator (this Codex/Claude instance) supervises one context-blind
  top-level task leader for the task lifetime.
- NO auto-approval rule was given. Per the coordinator contract, this does NOT
  override explicit human plan approval, destructive-action gates, paid-provider
  gates, scope expansion, or shared/repo workflow violations. The **plan gate is a
  human gate**: the leader must reach a plan + review package and the coordinator
  must present it to the human for explicit approval before implementation.
- Task-0015's own contract also makes **task closure a distinct, explicit human
  gate** (the agent never self-closes; gate approvals are not closure approval) —
  honor that at closure.
- Worker-safe human directives (D1–D4 + the park / human-only-closure / O6
  revisions) live in `HUMAN-DIRECTIVES-FOR-WORKER.md` and are passed via
  `HUMAN_DIRECTIVES_FOR_WORKER_DOC`.
- Data/isolation: Task-0015 touches the LIVE backend, the manifest, and the
  operator skill; the leader must use isolated lanes / task-owned fixtures per
  repo `REGRESSION.md` / `DATA-HANDLING.md` / `TESTING.md`, never the human's live
  config/DB or ungated live GitHub writes. The working tree also has unrelated
  pre-existing changes; commit only Task-0015 in-scope files.
