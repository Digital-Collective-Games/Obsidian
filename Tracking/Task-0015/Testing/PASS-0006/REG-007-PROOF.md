# REG-007 Proof — Queue-Drain Consumer Dispatch From The GitHub Web Surface

Task: Task-0015. Pass: PASS-0006. Verdict: **END-TO-END PASS** (agent-driven real
GitHub web-UI flip → backend dispatch → top-level claude agent launched + ran).

This is the human-surface regression required by the directive: integration tests
against the issue PROVIDER are exercised at the real GitHub web UI via the Chrome
debug session (the `github-operator` skill / CDP); the human only authenticates the
debug Chrome profile, the AGENT drives the UI. No proxy / no API-simulated flip.

## Isolation

- Backend: built from the working tree, run launch-enabled
  (`CODEX_ORCHESTRATION_QUEUE_LAUNCH_AGENT=true`, agent tools limited to
  `Read,Write,Edit`) bound `127.0.0.1:14318`, against validation Temporal
  `127.0.0.1:17233` in a FRESH isolated namespace **`reg007b`** (no real jobs;
  the real cron `default` namespace was never used).
- Provider/target: the throwaway repo `Digital-Collective-Games/QueueDrainTestbed`
  (issue #5, type Task). Never the production Obsidian repo, never the live
  service lane, never live data. No real email.

## The end-to-end (observed)

1. **Real-UI flip (agent-driven):** via the Chrome debug session (CDP), the agent
   clicked the issue-sidebar `Queue` field control → "Ready" option. UI control
   read `QueueReady`; API readback `Queue => Ready`. (The flip was a real
   browser-UI interaction, not an API write.)
2. **Consumer noticed + dispatched (≤30s poll):** backend log —
   `queue-drain poll acted ... dispatched [Task-0005] parked [] reclaimed []`,
   then `codex.task.run` `taskrun.execution_preflight` / `workload_step` /
   `execute_workload_step` for `taskrun--Task-0005--active`.
3. **Worktree + O6 binding:** `GET /api/v1/worktrees` returned
   `{ repo: QueueDrainTestbed, task_id: Task-0005, worktree_path:
   ...\cdxow\Task-0005-731cfb8c-1658051126\w, agent_session_id:
   bcd2a9b2-21f2-4f37-b118-26e2c3299de7, session_transcript_path:
   ...\projects\<slug>\bcd2a9b2-....jsonl, run_gate_state: running }`.
4. **Top-level claude agent launched + ran:** its session transcript appeared at
   the bound deterministic path (**46,381 bytes**), and the agent performed the
   bounded task — `AGENT-RAN.txt = "reg-007 agent launched ok"` in the worktree.
   (Detection-to-agent-running observed within ~30s.)

Evidence: [evidence/observation-after-fix.txt](./evidence/observation-after-fix.txt),
[evidence/backend-dispatch.log](./evidence/backend-dispatch.log),
[evidence/AGENT-RAN.txt](./evidence/AGENT-RAN.txt),
[evidence/agent-transcript-head.jsonl](./evidence/agent-transcript-head.jsonl).

## Defect found + fixed by this regression

The FIRST end-to-end attempt dispatched + bound the session but the agent never
ran (no transcript, no `AGENT-RAN.txt`). Root cause: the launcher used
`exec.CommandContext(<dispatch-activity ctx>, …)`, so claude was killed the instant
the Temporal `queue.drain.poll` activity returned. Fixed by launching under a
detached context (the agent is supervised externally by the watchdog + reclaimed on
close, not by the activity). See [../../BUG-0001.md](../../BUG-0001.md). The re-run
above is post-fix.

## Cadence

Consumer ran always-on, polling every 30s → noticed the flip within ≤1 minute
(human requirement). The code default poll interval is being set to ≤60s to make
the always-on requirement the default.

## Standard satisfied

This exercises the queue-drain feature end-to-end at the real GitHub web surface
(no proxy), per the TESTING.md "Issue-Provider Integration Testing" policy — the
mandatory end-to-end that has no pass/excuse to skip.
