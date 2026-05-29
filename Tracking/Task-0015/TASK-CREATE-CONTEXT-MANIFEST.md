# Task-0015 TaskCreate Context Manifest

Worker-safe durable files for the writer-worker. Paths relative to repo root
`C:\Agent\CodexDashboard` unless absolute. Citations come from two read-only recon
fan-outs on 2026-05-29 (state recon + design recon); re-open files as needed and
cite exact lines.

## Authoritative human context (read first)
- `Tracking/Task-0015/HUMAN-DIRECTIVES-FOR-WORKER.md` — verbatim directives + the
  four resolved decisions (D1–D4). AUTHORITATIVE.
- `Tracking/Task-0015/TASK-CREATE-OBJECTIVE.md` — objective, current truth, chosen
  solution shape, proof constraints.

## Shared standards
- `C:\Users\gregs\.codex\Orchestration\Processes\TASK-CREATE.md`
- `C:\Users\gregs\.codex\Orchestration\Exemplars\TASK.md`
- `Tracking/Task-0013/TASK.md` — same-repo STYLE/QUALITY exemplar (multi-objective,
  earned merge, pinned-release caveat). Reference for structure, NOT scope.

## Backend: reusable worktree + Temporal machinery (reuse as-is)
- `backend/orchestration/internal/taskrun/service.go:126-185` — `dispatchWithDirective`
  (owned-lane dispatch entry point).
- `backend/orchestration/internal/taskrun/service.go:934-966` — `provisionOwnedLane`
  (`git worktree add --detach`).
- `backend/orchestration/internal/taskrun/service.go:988-1026` — `bootstrapOwnedLane`.
- `backend/orchestration/internal/taskrun/service.go:1028-1045` — `cleanupOwnedLane`
  (`git worktree remove --force`).
- `backend/orchestration/internal/taskrun/service.go:968-986` — `releasePreviousOwnedLane`.
- `backend/orchestration/internal/taskexec/taskexec.go:40-118` — `TaskRunWorkflow`
  (signal channels, lifecycle, `shouldExit` at 114-117).
- `backend/orchestration/internal/temporalbackend/backend.go:134-156` — `StartWorker`
  (workflow/activity registration), `StartTaskRun`/`ExecuteWorkflow`, `cfg.TaskQueue`.
- `backend/orchestration/internal/httpapi/mux.go:16-89,122-145,505-530` — HTTP route
  registration, task dispatch route, webhook route pattern.
- `backend/orchestration/internal/controlplane/controlplane.go:333-363` — `TriggerWebhook`
  dispatch pattern; `Reconcile` (job-spec -> Temporal schedule).
- `backend/orchestration/internal/jobs/spec.go` — job Spec/Trigger/Executor model.
- `backend/orchestration/README.md` — how the service runs (Temporal + Postgres,
  Windows scheduled task), dispatch + owned-lane contract; find the Go test command here.

## Backend: HARD 1:1 today (what N>1 must change)
- `backend/orchestration/internal/taskrun/service.go:747-809` — `deriveDispatchReadiness`
  (lines ~777-781 block dispatch when an active run exists; the 1:1 gate).
- `backend/orchestration/internal/taskrun/types.go:121-138` — `RepoLane` (single owned
  checkout; no slot/queue semantics yet).

## Backend: liveness / poke / escalation primitives (reuse)
- `backend/orchestration/internal/taskrun/types.go:20,36,47,186-187,140-148` —
  `StateSleepingOrStalled`, `ActionPoke`, `StateEnvelope.SuspiciousAfter`,
  `LastProgressAt`/`LastProgressSummary`, `RunFollowUp`.
- `backend/orchestration/internal/taskrun/service.go:226-255` — `PokeRun`
  (`poke_worker_check` FollowUp, 5-min DueAt).
- `backend/orchestration/internal/taskrun/service.go:1214-1244` — overdue FollowUp ->
  `AttentionUrgent` escalation.
- `backend/orchestration/internal/taskrun/service.go:1254-1268` — `staleRunUpdate`
  (auto -> `StateSleepingOrStalled` on SuspiciousAfter deadline).
- `backend/orchestration/internal/taskexec/taskexec.go:57-59,81,126-130,165` — signal
  channels; SuspiciousAfter +15min; activity StartToClose 2min.

## Done-contract / terminal state (D2)
- `backend/orchestration/internal/taskrun/service.go:828-848` — `isTerminalTaskState`/
  `isCompletedTaskStatus`/`isCancelledTaskStatus` (backend's internal terminal check;
  may be used to stop supervising a run, but the QUERYABLE state is the GitHub issue).
- `skills/obsidian-operator/SKILL.md` — Authority Model (GitHub issue owns
  open/closed + queryable identity; `Human Needed` owned as a field; issue close on
  completion) and the new Issue-Type requirement (set `type=Task`).
- `skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1` — writes issue-field
  values (Queue/Priority/Human Needed) + sets issue type + closes/opens to match
  local status; reuse its close path for "done".
- `skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1` — reads issue-field
  values incl. Queue/Human Needed (pattern for reading queue state).
- Org issue fields: `Queue` id 42656828 (Never/Ready), `Human Needed` id 42656829
  (No/Yes), `Priority` id 42656780; org `Digital-Collective-Games`; endpoint
  `/repos/Digital-Collective-Games/Obsidian/issues/<n>/issue-field-values`.

## Manifest + rename scope (live-code refs to update; leave history alone)
- `CODEX-REPO-MANIFEST.json` — RENAME to `REPO-MANIFEST.json`; add per-repo
  `queue_workers` (default ~4). Current shape: `schema_version`, `manifest_type`,
  `repos[]` with `id`/`local_root`/`source_control_provider`/`task_provider`/
  `task_proposal_provider`.
- Live references to the filename `CODEX-REPO-MANIFEST.json` to update:
  - `skills/obsidian-operator/SKILL.md`
  - `skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1` (default `-ManifestPath`)
  - `skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1` (default `-ManifestPath`)
  - `skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1`
  - `tests/test_obsidian_title_roundtrip.py`
  - `app/codex_dashboard/paths.py`
  - `DATA-HANDLING.md`
- DO NOT rewrite historical references in `Tracking/Task-0012/*`, `Tracking/Task-0013/*`,
  or `C:\Users\gregs\.codex\sessions\*` (durable history).

## Incident email (D4)
- Email path: the `gmail-digest-email` skill / configured Gmail
  (`admin@digitalcollective.games`) via the local gmail MCP. Incident must include
  the observed watchdog state + the agent's session transcript.

## State / queue facts already established (from recon)
- Verified: no current consumer reads `Queue` to dispatch; flipping `Queue=Ready`
  does nothing today. (`Tracking/Task-0012/TASK.md:421-422`; DrainQueueDemo is a
  local-JSON PoC.)
- Agent dispatch must launch a TOP-LEVEL agent able to spawn its own subagents
  (D3) — not a nested subagent.

## Out of scope (do not pull in)
- Non-GitHub queue sources; cross-repo global fairness beyond a per-repo slot cap;
  rewriting historical artifacts; token-ingest/scanner/UI (Task-0013/0014 areas);
  changing the Codex-vs-Claude runtime choice beyond what dispatch needs.
