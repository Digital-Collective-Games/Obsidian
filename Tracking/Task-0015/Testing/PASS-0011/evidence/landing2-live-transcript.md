# PASS-0011 evidence — Landing 2 live verification transcript

Captured 2026-05-31. Validation Temporal `127.0.0.1:17233`, namespace `reg007r` (isolated,
created for this run). Test binary `cp-reg007q.exe`, backend `127.0.0.1:14318`. Production
Obsidian service lane + real `default` Temporal namespace untouched.

## Stage 1 — gated real-Temporal smoke
```
=== RUN   TestLanding2GateBindingRoundTripLiveTemporal
INFO  Started Worker Namespace reg007r TaskQueue landing2-smoke-...
landing2_live_smoke_test.go:101: LIVE round-trip OK: run=landing2-smoke--... gate=parked_awaiting_closure session=sess-xyz pid=4242
--- PASS: TestLanding2GateBindingRoundTripLiveTemporal (0.11s)
PASS
ok  github.com/.../internal/temporalbackend  0.203s
```

## Stage 2 — running binary
Backend health (isolated lane):
```
{"namespace":"reg007r","status":"ok","task_queue":"reg007r-tq","temporal_address":"127.0.0.1:17233"}
INFO  Started Worker Namespace reg007r TaskQueue reg007r-tq ...
codex orchestration control-plane listening on 127.0.0.1:14318
```

Dispatch + initial worktrees read (live workflow):
```
POST /api/v1/tasks/Task-0001/dispatch  -> [HTTP 202]
GET  /api/v1/worktrees ->
  {"run_id":"taskrun--Task-0001--active","repo":"C:\\Agent\\QueueDrainTestbed2",
   "task_id":"Task-0001","worktree_path":"...\\cdxow\\Task-0001-4e982457-1801009807\\w",
   "run_gate_state":"running"}
```

Pre-signal:
```
breadcrumb run_gate_state = running
worktrees  run_gate_state = running
```

Signal the workflow gate directly (the temporal CLI; simulates the consumer's park):
```
temporal workflow signal --address 127.0.0.1:17233 --namespace reg007r \
  --workflow-id taskrun--Task-0001--active --name taskrun.set_gate_state \
  --input '{"state":"parked_awaiting_closure"}'
=> Signal workflow succeeded
```

Post-signal (DEMOTION PROOF):
```
worktrees  run_gate_state = parked_awaiting_closure   <- binary reads the LIVE workflow
breadcrumb run_gate_state = running                   <- on-disk JSON DEMOTED (ossified, never written)
VERDICT: PASS
```

Kill backend (simulated crash) -> port 14318 down. Restart (same config) -> healthy in 1s,
worker re-registered on reg007r. Post-restart worktrees read (RESTART-SURVIVES-STATE):
```
GET /api/v1/worktrees ->
  {"run_id":"taskrun--Task-0001--active","task_id":"Task-0001","run_gate_state":"parked_awaiting_closure"}
VERDICT: PASS — reconstructed parked from the durable Temporal workflow after a crash+restart
```

Cleanup:
```
temporal workflow terminate ... taskrun--Task-0001--active  => Workflow terminated
Stop-Process cp-reg007q  => port 14318 alive: False
git -C C:\Agent\QueueDrainTestbed2 worktree remove --force ...\w ; worktree prune
git worktree list => main only ; runs-root removed ; testbed git clean
```
