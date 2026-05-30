# PASS-0003 (O4) — Done-contract MECHANISM evidence

Scope of this pass: the done-contract MECHANISM + unit/dry-run proof + the A6.4
parked-listing re-proof. Integrated, consumer-driven and agent-driven proofs are
DEFERRED (see "Deferred" below).

## Code delivered

- Run/gate enum + helper:
  [../../../../backend/orchestration/internal/taskrun/types.go](../../../../backend/orchestration/internal/taskrun/types.go)
  — `RunGateStateParkedAwaitingClosure|ParkedResearch|ParkedPlan|ParkedRegression`
  and `IsParkedRunGateState`.
- State-set API:
  [../../../../backend/orchestration/internal/taskrun/service.go](../../../../backend/orchestration/internal/taskrun/service.go)
  — `Service.SetRunGateState(taskID, state)` persists the run/gate state on the
  active `owned-lane-bootstrap.json` record so `GET /api/v1/worktrees` reflects it;
  it never deallocates (only the human-approved close path frees a slot).
- Pure decision + free-concurrency:
  [../../../../backend/orchestration/internal/queue/decision.go](../../../../backend/orchestration/internal/queue/decision.go)
  — `DecideQueueAction` (closed=>terminal; HumanNeeded=>park; open+Ready+not-parked
  =>dispatch; else none) and `EffectiveFreeConcurrency(limit, parked)` = limit −
  parked.
- Skill done-contract:
  [../../../../skills/obsidian-operator/scripts/Set-TaskDoneContract.ps1](../../../../skills/obsidian-operator/scripts/Set-TaskDoneContract.ps1)
  + the "Queue Done-Contract" section in
  [../../../../skills/obsidian-operator/SKILL.md](../../../../skills/obsidian-operator/SKILL.md).

## TIER 1 — Go unit tests (deterministic)

Full `go test ./...`: [go-test-all.txt](./go-test-all.txt) (all packages ok).

- Decision function + free-concurrency:
  `internal/queue/decision_test.go`
  (`TestDecideQueueActionEncodesDoneContract`, `TestDecideQueueActionInvariants`,
  `TestEffectiveFreeConcurrencyIsLimitMinusParked`).
- State transition + parked worktree still surfaced by `ListActiveWorktrees`
  (code-level A6.4): `internal/taskrun/gatestate_test.go`
  (`TestSetRunGateStateParksAndStaysListed`, plus reject/no-lane/drift-guard tests).
- A6.4 at the HTTP endpoint: `internal/httpapi/worktrees_test.go`
  (`TestWorktreesEndpointListsParkedWorktree`).

## TIER 2 — skill dry-run proof (no live GitHub writes)

- (i) abandon => `Human Needed=Yes`: [dry-run-abandon.json](./dry-run-abandon.json)
  (`human_needed: Yes`, `run_gate_state: parked_research`, `closes_issue: false`).
- (ii) perceived completion => `Human Needed=Yes` + awaiting-closure:
  [dry-run-completion.json](./dry-run-completion.json)
  (`run_gate_state: parked_awaiting_closure`, `closes_issue: false`).
- Underlying single write path emits the field write:
  [dry-run-sync-field-write.json](./dry-run-sync-field-write.json)
  (`issue_fields."Human Needed": "Yes"`).
- (iii) NO `gh issue close` in either path: `Set-TaskDoneContract.ps1` has no
  executable close command (the only source mention is a comment stating it has
  none); the Sync dry-run output contains no `issue close`.
- (iv) human-only close is a SEPARATE, human-gated command (the Reconcile close
  builder): [human-only-close-builder.txt](./human-only-close-builder.txt).

Fixture (throwaway, NOT a real issue): `fixture/` (a Task-9001 TASK.md + a manifest
pointing at a `QueueDrainTestbed` provider). Dry-run only — no `gh` was invoked.

## A4 criteria: PROVEN now vs DEFERRED

- A4.1 (agent abandon => Human Needed=Yes, stays open): MECHANISM proven via
  dry-run (i). Full agent-driven proof DEFERRED to PASS-0005 (needs the launched
  agent of O5).
- A4.2 (perceived completion => parked awaiting-closure, then human close =>
  reclaimed): MECHANISM proven via dry-run (ii) + the human-only close builder
  (iv). Full agent-driven + consumer-driven reclaim DEFERRED to PASS-0004 (O3
  consumer) / PASS-0005 (O5 agent).
- A4.3 (closed=>terminal cleanup/free/dequeue; HumanNeeded=>park, no cleanup, no
  redispatch): decision-logic MECHANISM proven (`DecideQueueAction`,
  `SetRunGateState` retains worktree). Full in-the-loop consumer behavior DEFERRED
  to PASS-0004 (O3).
- A4.4 (parked resumes in SAME worktree, no re-provision): MECHANISM proven —
  parking does not deallocate; the lane stays listed and transitions back to
  running (`TestSetRunGateStateParksAndStaysListed`). Full consumer-driven resume
  DEFERRED to PASS-0004.
- A4.5 (effective free concurrency = queue_workers − parked): PROVEN now
  (`EffectiveFreeConcurrency` + test). In-the-loop use DEFERRED to PASS-0004.
- A4.6 (no second GitHub-write path): PROVEN now — the Go backend performs no
  GitHub writes (verified: every `github.com` reference is a Go import path), and
  the agent done write goes through the existing `Sync-TaskToGitHubIssue.ps1`
  field-value write.
- A4.7 (agent NEVER self-closes; closure distinct from pass/regression/plan/research;
  no auto-close from local status): MECHANISM proven — `Set-TaskDoneContract.ps1`
  has no close path; the close is a separate human-gated builder; the Go backend
  never closes. Full agent-driven proof DEFERRED to PASS-0005.

Re-proof of O6 A6.4 (parked worktree still listed) is PROVEN now at both the
service and HTTP-endpoint layers.

## Deferred (NOT built here)

- O3 consumer loop reading real issue state (PASS-0004): full consumer-driven
  park/close/dequeue (A4.3/A4.4 in-the-loop, A4.5 in-the-loop).
- O5 launched agent (PASS-0005): agent-driven abandon (A4.1), agent perceived-
  completion park (A4.2), agent never self-closes (A4.7).
