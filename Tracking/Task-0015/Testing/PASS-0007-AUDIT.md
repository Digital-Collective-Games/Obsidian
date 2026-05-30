# PASS-0007 Audit — Reopened O3: registry-driven binding

Task: Task-0015. Pass: PASS-0007 (reopened O3 binding). Verdict: **READY**
(registry-driven binding implemented, independently verified, and validated live;
committed).

## Why

The human identified a real gap: the backend bound to a repo via env
(`CODEX_ORCHESTRATION_WORKTREE_ROOT` + `CODEX_ORCHESTRATION_QUEUE_DRAIN_REPO`) plus a
co-located/default `queue_workers` lookup — NOT via the registry. The orchestrator
needs global awareness from the central registry; the registry must make no
assumptions about repo location; the consumer must poll via `task_provider`
(first-class), which defines the integration with local source.

## What changed

The queue-drain consumer is now **registry-driven**: it reads the central
`REPO-MANIFEST.json` at **`OBSIDIAN_REGISTRY_PATH`** (default the backend repo-root
manifest), enumerates ALL registered repos each poll (global awareness), and per
repo polls that entry's **`task_provider.repo`**, maps `#N → <local_root>/Tracking/
Task-N`, and dispatches into that entry's `local_root` capped at its `queue_workers`
(per-repo slot accounting via `taskrun.NewServiceForRepo`; `git worktree list` is
per-repo). `task_provider` + `source_control_provider` are first-class decoded
registry types; `local_root` is taken verbatim (no location assumption).
`task_proposal_provider` is out of scope. The legacy manual `/dispatch`
(`CODEX_ORCHESTRATION_WORKTREE_ROOT`) path and the production `REPO-MANIFEST.json`
are untouched. New env uses the `OBSIDIAN_` prefix.

## Verification (independent)

- **Workflow (1 implementer + 3 independent verifiers)** — all three verifier
  dimensions returned **`sound`, zero blocking**: (a) registry-driven binding (reads
  `OBSIDIAN_REGISTRY_PATH`, enumerates all repos, polls each `task_provider.repo` —
  a decoy-env test proves `QUEUE_DRAIN_REPO` is no longer the source, dispatches into
  each `local_root`); (b) Service repo-parameterization + per-repo slots (cap passed
  in from the registry, used = that repo's `git worktree list`; a cap-1 repo with 1
  used dispatches none while a sibling cap-2 dispatches both); (c) build/config/
  naming/scope (`OBSIDIAN_REGISTRY_PATH`, existing `CODEX_` vars + production manifest
  untouched, no scope creep, gofmt-clean, `go build/vet/test ./...` green).
- **Live registry-driven REG-007 re-run — PASS.** Backend bound to the testbed
  PURELY via the registry (`OBSIDIAN_REGISTRY_PATH` → a testbed-only registry; **no
  `QUEUE_DRAIN_REPO`**), isolated `reg007c` namespace. Real-UI `Queue=Ready` flip (via
  the `github-operator` skill) → `queue-drain poll acted … dispatched [Task-0005]` →
  binding `{ repo: QueueDrainTestbed (registry id), worktree under the registry's
  local_root, session 05778727… }` → the launched claude agent ran (transcript
  54,453 bytes; `AGENT-RAN.txt = "reg-007 agent launched ok"`). Evidence:
  [PASS-0007/evidence/](./PASS-0007/evidence/). Production Obsidian was never polled
  (the test registry lists only the testbed); the real cron `default` namespace was
  never used.

## Remaining (follow-ups, not blockers)

- Cosmetic: `StartWorker(cfg, taskService)` keeps an unused `taskService` param;
  `QueueDrainConfig.Repo` / `CODEX_ORCHESTRATION_QUEUE_DRAIN_REPO` are retained but
  decorative for the registry-driven consumer (legacy/manual only). Future cleanup.
- Prior O5 hardening follow-ups still open (orphan-agent lifecycle + launcher
  logging; full real-agent stall→incident-email repro).
- Task CLOSURE remains a distinct, final human gate.
