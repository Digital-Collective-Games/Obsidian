# PASS-0002 (O6) — Worktree↔session binding + enumeration endpoint: proof

Pass scope: O6 from [PLAN.md](../../PLAN.md) "PASS-0002" — binding SCHEMA +
endpoint + operator command. Acceptance: A6.1 (HARD), A6.2 (HARD), A6.3 from
[TASK.md](../../TASK.md). Per the plan-gate ordering caveat
([COORDINATOR-REVIEW.md](../../Design/plan-gate/COORDINATOR-REVIEW.md)) and the
worker directive, two pieces are explicitly DEFERRED out of this pass:

- **A6.4 (parked needs-human worktree listed) → PASS-0003 (O4).** Park state does
  not exist until O4. This pass did NOT fake a parked state. The endpoint already
  returns whatever `run_gate_state` a record carries, so it will list parked
  worktrees unchanged once O4 records them (no further endpoint change needed).
- **A6.1's REAL launched-agent session id / transcript → PASS-0005 (O5).** This
  pass populates the binding's `agent_session_id` / `session_transcript_path` from
  the BACKEND dispatch process's env (`captureDispatchContext`), which is the
  best-available value today. Real launched-agent session capture is PASS-0005
  work. These two fields are marked PLACEHOLDER in code comments
  ([types.go RepoBinding](../../../../backend/orchestration/internal/taskrun/types.go)).

Proof captured on the isolated validation lane (backend `127.0.0.1:14318`,
Temporal `127.0.0.1:17233`) against the throwaway test repo
`C:\Agent\QueueDrainTestbed`. Never the live lane (`:4318`/`:7233`) or the live
CodexDashboard worktree. FRESH task ids `Task-10`/`Task-11` were used (the
validation namespace holds stale `running` records for Task-1/2/3).

## What was built

- **Binding schema on the owned-lane record**
  ([types.go](../../../../backend/orchestration/internal/taskrun/types.go)):
  new `RepoBinding` struct `{ repo, task_id, worktree_path, agent_session_id,
  session_transcript_path, run_gate_state }`, attached as `RepoLane.Binding`
  and persisted on `ownedLaneBootstrapRecord.Binding`
  ([service.go](../../../../backend/orchestration/internal/taskrun/service.go)).
  New `RunGateStateRunning = "running"` constant is the default run/gate state at
  dispatch (the parked/which-gate enum is O4/PASS-0003).
- **Population at dispatch**: `bootstrapOwnedLane` now takes the dispatch
  `*DeepContext` and builds the binding via `bindingForLane` — task id +
  `OwnedRepoRoot` (genuine), repo id resolved from `REPO-MANIFEST.json`
  (`RepoManifest.RepoIDForRoot`, new in
  [manifest.go](../../../../backend/orchestration/internal/queue/manifest.go)),
  and session id/transcript from the dispatch context (placeholder, see above).
  No values are invented.
- **`GET /api/v1/worktrees`**
  ([mux.go](../../../../backend/orchestration/internal/httpapi/mux.go) —
  `handleWorktreesList`, registered next to `/api/v1/tasks`): enumerates active
  owned-lane worktrees from the durable `owned-lane-bootstrap.json` records via
  `Service.ListActiveWorktrees` (a worktree is active while its checkout
  directory still exists on disk; a reclaimed lane drops out). Returns the
  binding per worktree. It SUPPLIES the fields to construct a VSCodium link but
  emits NO `vscodium://` link.
- **Operator command**
  [`Get-ActiveWorktreeSessions.ps1`](../../../../skills/obsidian-operator/scripts/Get-ActiveWorktreeSessions.ps1)
  (+ documented in
  [SKILL.md](../../../../skills/obsidian-operator/SKILL.md) "Script Use"): calls
  the endpoint (configurable `-BaseUrl`, default the service-lane bind) and
  prints per worktree the worktree path, issue/Task, run/gate state, session id,
  and transcript path.

## TIER 1 — Go unit tests (REQUIRED): PASS

`go test ./...` from `backend/orchestration` builds and passes — see
[evidence/TIER1-go-test-all.txt](./evidence/TIER1-go-test-all.txt).

New tests:
- `internal/taskrun/binding_test.go`:
  - `TestDispatchPopulatesOwnedLaneBinding` — binding fields populated on the
    returned `RepoLane.Binding` AND on the durable `owned-lane-bootstrap.json`
    record after dispatch (A6.1 schema/population).
  - `TestListActiveWorktreesReturnsBinding` — `ListActiveWorktrees` enumerates the
    dispatched worktree with its full binding.
  - `TestListActiveWorktreesSkipsReclaimedWorktree` — a cleaned-up worktree drops
    out of the listing (active = checkout still present).
- `internal/httpapi/worktrees_test.go`:
  - `TestWorktreesEndpointReturnsBindingAfterDispatch` — `GET /api/v1/worktrees`
    returns 200 with the binding after a real dispatch, and the body contains NO
    `vscodium://` link (A6.2 + O6 boundary).
  - `TestWorktreesEndpointRejectsNonGet` — non-GET → 405.

## TIER 2 — Live proof on the isolated validation lane: PASS (A6.1, A6.2, A6.3)

Infra: backend `127.0.0.1:14318`, validation Temporal `127.0.0.1:17233`,
`CODEX_ORCHESTRATION_WORKTREE_ROOT=C:\Agent\QueueDrainTestbed`, isolated runs root
under temp. `/healthz` returned `ok` against Temporal `127.0.0.1:17233`. The
backend env carried `CODEX_SESSION_ID=pass0002-dispatch-session` and a
`CODEX_TRANSCRIPT_PATH` placeholder to exercise the dispatch-context capture.

- **A6.1 (HARD)** — after `POST /api/v1/tasks/Task-10/dispatch` (and `Task-11`),
  both runs reached state `running` and the returned `repo_lane.binding` carried
  every binding field populated: `repo=QueueDrainTestbed`, `task_id=Task-10`,
  `worktree_path=...\cdxow\Task-10-...\w`,
  `agent_session_id=pass0002-dispatch-session`,
  `session_transcript_path=...pass0002-transcript.jsonl`,
  `run_gate_state=running`. See
  [evidence/A6.1-dispatch-Task-10.json](./evidence/A6.1-dispatch-Task-10.json),
  [evidence/A6.1-dispatch-Task-11.json](./evidence/A6.1-dispatch-Task-11.json).
- **A6.2 (HARD)** — `GET http://127.0.0.1:14318/api/v1/worktrees` returned HTTP
  200 with TWO active worktrees, each carrying the bound `{ repo, task_id,
  worktree_path, agent_session_id, session_transcript_path, run_gate_state }`,
  plus the originating `run_id`. The response body contains NO `vscodium://`
  string (confirmed by a case-insensitive search). Empty before dispatch
  (count 0). See
  [evidence/A6.2-worktrees-before-dispatch.json](./evidence/A6.2-worktrees-before-dispatch.json),
  [evidence/A6.2-worktrees-after-dispatch.json](./evidence/A6.2-worktrees-after-dispatch.json).
- **A6.3** — `Get-ActiveWorktreeSessions.ps1 -BaseUrl http://127.0.0.1:14318`
  printed, per active worktree, the worktree path, issue/Task, run/gate state,
  session id, and transcript path. See
  [evidence/A6.3-operator-script-output.txt](./evidence/A6.3-operator-script-output.txt).

## DEFERRED (not proven this pass; not faked)

- **A6.4 (parked needs-human worktree still listed) → PASS-0003 (O4).** Park
  state does not exist yet. The endpoint already surfaces `run_gate_state`
  unchanged, so once O4 records a parked state on the binding, the existing
  endpoint and operator command will list parked worktrees with no further change.
- **A6.1 real launched-agent session id / transcript → PASS-0005 (O5).** The two
  session fields are dispatch-context placeholders today (clearly marked in code).

## Teardown

The validation backend was stopped (`:14318` freed, no orphan). The two owned-lane
worktrees created for Task-10/Task-11 were removed (`git worktree remove --force`
+ `git worktree prune`); the testbed is back to a single main worktree. The
isolated temp runs dir, the proof binary, the log files, and the two leftover
`cdxow` worktree dirs were deleted. Validation Temporal (`:17233`) and the live
lane (`:4318`/`:7233`) were left untouched. The testbed Task-10/Task-11 fixture
commit was made INSIDE the throwaway repo only.
