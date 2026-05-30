# PASS-0001 Audit — O2: Real N>1 per-repo concurrency

Task: Task-0015. Pass: PASS-0001 (O2). Verdict: **READY** (O2 PASS, committed).

## Independence

Implementer and QA were SEPARATE clean-context subagents. For the HARD live
criterion (A2.1) the QA worker did NOT trust the implementer's saved evidence —
it **re-ran the live concurrency proof from scratch** on the isolated validation
lane (it even created fresh sibling tasks because the implementer's tasks carried
stale Temporal run records), and re-ran `go build`/`go test` itself.

## Per-criterion verdict (independent re-derivation)

- **A2.1 (HARD) PASS** — QA launched a fresh backend bound `127.0.0.1:14318`
  against validation Temporal `127.0.0.1:17233`, worktree root =
  `C:\Agent\QueueDrainTestbed` (throwaway). Dispatching two sibling tasks produced
  TWO detached owned-lane checkouts existing **simultaneously**
  (`git worktree list --porcelain`) with both run views `state=running` at once —
  genuine concurrency, not sequential.
- **A2.2 PASS** — at the cap, a further same-repo dispatch was refused with
  `repo_slots_exhausted`; raising the cap re-admitted it and produced a real third
  concurrent owned-lane worktree.
- **A2.3 PASS** — same-repo tasks read `dispatch_readiness.ready=true` with zero
  block reasons while a free slot remained (no `active_run_exists`).
- **AX.1 PASS** — `go build ./...` exit 0; `go test -count=1 ./...` all packages
  `ok` (incl. 6 new `internal/taskrun` O2 tests + 6 new `internal/queue` tests).
- **F-O2 NOT triggered** — >1 worktree per repo proven (3), the gate no longer
  blocks a same-repo dispatch while a slot is free, and it is real concurrency, not
  a config field.

## Genuineness (logic + tests, not hollow)

- Per-repo limit sourced from `queue_workers` in `REPO-MANIFEST.json` at the
  declared worktree root (`internal/queue/manifest.go`,
  `service.go:manifestQueueWorkers`); `EvaluateSlot` admits while `used < limit`
  (`internal/queue/slots.go:39-53`).
- "Used" is a REAL count of owned-lane worktrees under the backend-owned lane root
  (`service.go:countOwnedLaneWorktrees`, via `git worktree list --porcelain`).
- Gate: `repoSlotBlock` (`service.go:866-893`) replaces the per-task
  `active_run_exists` block; a task already owning a live story is still blocked
  from a DUPLICATE dispatch via a distinct `task_already_running` reason.
- `releasePreviousOwnedLane` acts only on the SAME task (`GetActiveTaskRun`),
  structurally unable to tear down a sibling — locked by
  `TestReleasePreviousOwnedLaneDoesNotTearDownSiblingLane`.
- New tests drive admit/refuse/re-admit, no-`active_run_exists`-for-free-slot,
  same-task-still-blocked, two-concurrent-siblings, and the teardown invariant.

## Isolation & cleanup (verified by QA)

- Live lane (`:4318`/`:7233`/`:5432`) and the live CodexDashboard worktree were
  NEVER used as dispatch targets. Only the untouched live `controlplane-service-lane`
  (PID 24708) ran alongside.
- QA stopped its backend (`:14318` free, no orphan), removed/pruned all worktrees
  it created, removed its temp runs dir/binary, and reset the test repo to baseline.

## Scope / churn

`service.go` +91/−6; new `internal/queue` package + `internal/taskrun/slots_test.go`;
gofmt-clean; no edits to `cleanupOwnedLane`, the close path, `Human Needed`, park,
or terminal semantics (O3/O4 correctly untouched).

## Carry-forward note

The validation Temporal namespace retains stale `running` records for the test
repo's Task-1/2/3 from these proofs. Future live proofs should use fresh task ids
(or reset the validation namespace) to avoid `task_already_running` on re-dispatch.
