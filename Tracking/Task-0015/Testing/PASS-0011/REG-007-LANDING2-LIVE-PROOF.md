# PASS-0011 — Landing 2 live verification (real-Temporal cutover + running-binary demotion/restart)

Task: Task-0015. Verdict: **PASS** for the Landing-2-specific behaviors, proven live against
real Temporal and the real control-plane binary. Landing 2 moves the run/gate label +
worktree↔session binding off the `owned-lane-bootstrap.json` side-store and into the per-run
`TaskRunWorkflow` (durable Temporal state), demoting the JSON to a recovery breadcrumb. See the
design contract: [QUEUE-DRAIN-DURABLE-STATE-REDESIGN.md](../../Design/QUEUE-DRAIN-DURABLE-STATE-REDESIGN.md).

Code: commits `31f57b8` (Steps 4–6 read+write cutover), `c880e3f` (Step 7 durable supervision),
`87a92b3` (Step 8 prune-only reconcile), `9039974` (stage-1 smoke). All pushed to `upstream/master`.

## Static verification (all green)
- `go build/vet/test ./...` green across all packages; **binary builds**.
- Cutover footprint: 2 code files (`internal/taskrun/service.go`, `internal/temporalbackend/queuedrain.go`),
  **zero test files changed** — the existing gate/binding tests pass unmodified because the
  `fakeRuntime` already mirrors the workflow's binding, keyed by the same run id.

## Stage 1 — real-Temporal signal round-trip (deterministic, no GitHub/agents)
Gated integration smoke [`landing2_live_smoke_test.go`](../../../../backend/orchestration/internal/temporalbackend/landing2_live_smoke_test.go)
against validation Temporal `127.0.0.1:17233` ns `reg007r`. It starts a real `TaskRunWorkflow`
and exercises the real `Backend` methods (not the fake):

```
OBSIDIAN_LIVE_TEMPORAL=1 CODEX_ORCHESTRATION_TEMPORAL_ADDRESS=127.0.0.1:17233 \
  CODEX_ORCHESTRATION_NAMESPACE=reg007r \
  go test ./internal/temporalbackend/ -run TestLanding2GateBindingRoundTripLiveTemporal -count=1 -v
=> PASS: LIVE round-trip OK: gate=parked_awaiting_closure session=sess-xyz pid=4242
```
Proves: `BindLaunchedSession` and `SetRunGateState` SIGNAL the workflow and the query reads the
binding back; a fresh `GetActiveTaskRun` confirms BOTH signals are durably applied to the same
workflow state. This is the highest-risk piece (real Temporal vs the unit fake) — proven.

## Stage 2 — running control-plane binary (no GitHub/agents; isolated lane)
Real binary `cp-reg007q.exe`, ns `reg007r`, `WorktreeRoot=C:\Agent\QueueDrainTestbed2`, dedicated
runs-root, launch OFF, consumer NOT started. Production service lane untouched.

1. **Live workflow read via `/worktrees`.** `POST /api/v1/tasks/Task-0001/dispatch` (HTTP 202)
   provisioned a worktree + started the workflow; `GET /api/v1/worktrees` returned the binding
   `run_gate_state=running` read from the workflow.
2. **Demotion proof.** Signaled the workflow gate directly to `parked_awaiting_closure` (the temporal
   CLI `taskrun.set_gate_state` signal — exactly what the consumer's park does), then re-read:
   - `GET /api/v1/worktrees` → `run_gate_state = parked_awaiting_closure` (binary reads the LIVE workflow)
   - on-disk `owned-lane-bootstrap.json` → `run_gate_state = running` (DEMOTED — ossified, never written)
   ⇒ the running binary trusts the workflow, not the JSON side-store.
3. **Restart-survives-state.** Killed the backend (simulated crash; port 14318 down), restarted with
   the same config. `GET /api/v1/worktrees` reconstructed `Task-0001 = parked_awaiting_closure` from
   the durable Temporal workflow — the state survived the crash (the whole point of Landing 2).

Evidence transcript: [evidence/landing2-live-transcript.md](./evidence/landing2-live-transcript.md).
Cleanup: workflow terminated, worktree removed (`git worktree list` → main only), runs-root removed,
testbed git clean.

## Honest residuals (not run live; low marginal risk)
- **Full GitHub-driven cap=1 serialization lifecycle** (dispatch→park→close→reclaim→dequeue with real
  claude agents) was NOT re-run here. Landing 2 did not change the consumer's dispatch/park/reclaim
  DECISION logic — only the gate/binding STORAGE backend (proven above). The cap=1 serialization on the
  unchanged decision path was proven pre-Landing-2 in [PASS-0009](../PASS-0009/REG-007-CAP1-SERIALIZATION-PROOF.md).
- **Supervision-goroutine reconstruction** (`reconstructSupervision`) runs only on the launch-enabled
  consumer's first-build hook, so this no-agent harness does not exercise it. It is covered by code review
  + the supervisor-cache leak fix being correct-by-construction; the restart-survives-STATE proof above
  exercises the same durable-state recovery principle. A full launch+consumer restart run would
  black-box-confirm the goroutine reconstruction.

## Net
The Landing-2 cutover — gate/binding live in the per-run workflow, JSON demoted to a recovery
breadcrumb, surviving a backend crash — is **proven live end-to-end** against real Temporal and the
real binary. Human-only closure is untouched (no new signal closes an issue; reclaim stays the
consumer's GitHub-issue-closed path).
