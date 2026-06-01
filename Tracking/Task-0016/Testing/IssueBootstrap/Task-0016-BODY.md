<!-- task-sync: repo=CodexDashboard; task_id=Task-0016; task_path=Tracking/Task-0016/TASK.md -->

# Task-0016: Manual persistent worktree pool in the Go backend (Create / Assign / Eject / Destroy + discover-on-startup; Eject dequeues via the task provider)

## Source Of Truth

Local `Tracking/Task-0016/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0016:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

## Summary

Today the orchestration backend has **no concept of an idle, reusable worktree**.
Every dispatch provisions a *fresh, random* temp checkout
([`provisionOwnedLane()`](../../backend/orchestration/internal/taskrun/service.go#L1507)
uses `os.MkdirTemp(s.ownedLaneRoot, ...)` then `git worktree add --detach`), and
the only way a checkout disappears is the human-approved close path, which
**deletes** it ([`cleanupOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1640)
→ [`removeOwnedLaneWorktree`](../../backend/orchestration/internal/taskrun/service.go#L1688)).
Per-repo concurrency is gated by a separate runtime cap, `queue_workers`
([`RepoEntry.QueueWorkers`](../../backend/orchestration/internal/queue/manifest.go#L32)),
read through [`RepoSlotLimit()`](../../backend/orchestration/internal/taskrun/service.go#L616)
and enforced by [`EvaluateSlot`](../../backend/orchestration/internal/queue/slots.go#L39)
inside the queue-drain consumer ([`consumer.go` L141/L205](../../backend/orchestration/internal/queue/consumer.go#L141)).

This task replaces that model in the **Go backend only** with a **manually-managed,
persistent worktree pool per repo**. An operator pre-creates worktrees as real
on-disk folders with **stable paths and stable ids**; those folders persist as
**idle** pool members until assigned, and an Eject cleans an allocated worktree
back to idle **without deleting the folder**. Concurrency is then bounded **by the
number of idle worktrees in the pool, by construction** — `queue_workers` is
removed entirely. Both the manual **Assign** action and the autonomous queue-drain
consumer **draw from the same shared pool**; an empty pool means Ready issues wait
(no auto-create).

**Operator-perceived outcome (state this first):** an operator can, via headless
HTTP calls against the backend (`http://127.0.0.1:4318`):

- **pre-create** one or more idle worktrees for a repo (`POST /api/v1/worktrees/create`),
- **see the whole pool** — every worktree the backend owns across all registered
  repos, each with a real `worktree_path`, a stable `worktree_id`, and a `status`
  of `idle` or `allocated` (allocated ones carry the bound task/run/gate)
  (`GET /api/v1/worktrees`),
- **manually assign** an open task onto a chosen idle worktree, which resets that
  *existing* checkout to baseline and launches the run **in it** (no fresh dir)
  (`POST /api/v1/worktrees/assign`),
- **eject** an allocated worktree — stop the agent, clean the checkout to baseline,
  unbind the run — and get the **same folder back as idle, re-assignable**
  (`POST /api/v1/worktrees/eject`),
- **destroy** an idle worktree to shrink the pool, rejected if it is allocated
  (`POST /api/v1/worktrees/destroy`),
- and after a **backend restart** still see the pool reconstructed: which folders
  are idle and which are allocated to a live run (discover-on-startup).

The autonomous queue-drain consumer keeps working: when an issue flips
`Queue=Ready` it now **draws an idle worktree from that repo's pool** instead of
provisioning a fresh one, and if the pool is empty the issue simply waits.

**Eject must also dequeue the freed task (UPDATE 2 — 2026-05-31).** Today the
queue-drain consumer is **read-only** against the task provider — it only ever
*reads* the `Queue` single-select for `Ready`
([`ghQueueProvider.ListReadyIssues`](../../backend/orchestration/internal/queue/provider.go#L119)).
If Eject frees a worktree but leaves the task's issue at `Queue=Ready`, the very
next consumer poll **re-dispatches it** ("throws itself back in"). So this task
adds the **first task-provider queue WRITE**: a `TaskProvider`-abstraction
**dequeue** that sets the task's provider queue state to **not-ready**
(`github_issues`: set the issue's `Queue` single-select to `Never`, the same field
the consumer polls). Eject calls it; a standalone dequeue endpoint exposes it
alone. **Dequeue is not close** — it only sets `Queue=Never`; the issue stays
**open**, preserving human-only closure (only a human-closed issue deallocates; the
agent never self-closes). To restart a dequeued task the operator sets
`Queue=Ready` again.

**Current truth that must not masquerade as success:** the backend can enumerate
only *active* worktrees today ([`ListActiveWorktrees()`](../../backend/orchestration/internal/taskrun/service.go#L223),
served at [`GET /api/v1/worktrees`](../../backend/orchestration/internal/httpapi/mux.go#L130)),
where "active" means *a dispatch happened and the random temp checkout still exists
on disk* — there is no notion of a pre-created idle folder, no stable id, and
[`ReconcileOwnedLanes()`](../../backend/orchestration/internal/taskrun/service.go#L1663)
is prune-only (it removes stale git metadata; it does **not** enumerate folders or
classify them idle-vs-allocated). A change that keeps the per-dispatch random-temp
model, or that only renames `queue_workers`, or whose "discover" loses bound state
across a restart, would look adjacent but is **not** this task.

## Goals

1. **[BE] Remove `queue_workers`.** Delete the `QueueWorkers` field from
   [`RepoEntry`](../../backend/orchestration/internal/queue/manifest.go#L29) and
   from `REPO-MANIFEST.json` documentation, and remove its use from admission:
   [`RepoSlotLimit()`](../../backend/orchestration/internal/taskrun/service.go#L616),
   the [`SlotSizer`](../../backend/orchestration/internal/queue/consumer.go#L46)
   path / [`EvaluateSlot`](../../backend/orchestration/internal/queue/slots.go#L39),
   [`EffectiveFreeConcurrency`](../../backend/orchestration/internal/queue/decision.go#L144),
   and the `queueWorkers` argument threaded into
   [`NewServiceForRepo`](../../backend/orchestration/internal/taskrun/service.go#L102)
   from [`queuedrain.go` L196](../../backend/orchestration/internal/temporalbackend/queuedrain.go#L196).
   Concurrency is bounded by the count of idle pool worktrees, by construction.
2. **[BE] Make worktrees a manually-managed persistent pool per repo.** Idle
   worktrees are real folders with **stable paths and stable ids**, not the
   `os.MkdirTemp` random dirs of
   [`provisionOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1507).
   Pin a durable **pool record** (`worktree_id` ↔ stable path ↔ repo ↔ current
   `run_id`-or-null), grounded in the existing
   [`owned-lane-bootstrap.json`](../../backend/orchestration/internal/taskrun/service.go#L68)
   record + [`bindingsFromRecords`](../../backend/orchestration/internal/taskrun/service.go#L251)
   enumeration.
3. **[BE] Discover-on-startup.** Replace/extend prune-only
   [`ReconcileOwnedLanes()`](../../backend/orchestration/internal/taskrun/service.go#L1663)
   so startup **enumerates the worktrees that exist on disk** for the repo and
   reconstructs each one's **idle** vs **allocated** state (allocated = bound to a
   live run, read from the per-run workflow per Task-0015 Landing-2). Survives a
   backend restart (REG-008). No auto-seeding to a number.
4. **[BE] Create** — `POST /api/v1/worktrees/create` provisions one new **idle**
   pool worktree (stable path + id; `git worktree add --detach`; persists; no
   task) and returns it.
5. **[BE] Assign** — `POST /api/v1/worktrees/assign` resets a chosen **existing
   idle** worktree to baseline, then binds + starts the run **in that worktree**
   (reuse the bootstrap→start tail of
   [`dispatchWithDirective`](../../backend/orchestration/internal/taskrun/service.go#L671)),
   without provisioning a fresh dir. The queue-drain consumer's auto-assign draws
   **any** idle worktree in the repo. Reject when no idle worktree is available.
6. **[BE] Eject** — `POST /api/v1/worktrees/eject` stops the launched agent,
   cleans the checkout to baseline (`git reset --hard` + `git clean -fdx`),
   unbinds/terminates the run, **keeps the folder, marking it idle**, and then
   **dequeues the freed task through the task provider** (Goal 10) so the
   still-`Queue=Ready` task is **not** re-dispatched on the next poll — this is what
   prevents the bounce-back. Must **not** delete the folder and must **not** close
   the issue. Works regardless of parked state.
7. **[BE] Destroy** — `POST /api/v1/worktrees/destroy` removes an **idle** worktree
   from the pool (reuse
   [`removeOwnedLaneWorktree`](../../backend/orchestration/internal/taskrun/service.go#L1688)
   delete mechanics). **Reject if allocated** (operator must Eject first).
8. **[BE] Repos + full-pool reads.** `GET /api/v1/repos` returns repo id +
   `local_root` (with `queue_workers` **removed** from the response).
   `GET /api/v1/worktrees` returns the **full pool** — allocated + idle — each with
   a real `worktree_path`, stable `worktree_id`, and `status`; allocated ones carry
   the bound task/run/gate. The existing REG-008 read of `/worktrees` (parked lane
   reported from the workflow) keeps working.
9. **[BE] Dispatch-path change.** Replace the per-dispatch
   [`provisionOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1507)
   call in
   [`dispatchWithDirective`](../../backend/orchestration/internal/taskrun/service.go#L671)
   with **pool-draw** (pick an idle pool worktree → reset → bind → start). Worktree
   **creation** happens only via the manual Create action. An empty pool ⇒ Ready
   issues wait (no auto-create). Each repo's pool is independent (REG-009).
10. **[BE] Task-provider dequeue write.** Add a **queue WRITE** to the
    [`QueueProvider`](../../backend/orchestration/internal/queue/provider.go#L22)
    abstraction — `DequeueIssue(repo, number)` (or `SetQueueState(repo, number, QueueNever)`)
    — that sets the task's provider queue state to **not-ready**. For
    `github_issues` ([`ghQueueProvider`](../../backend/orchestration/internal/queue/provider.go#L49))
    this sets the issue's **`Queue` single-select to `Never`**
    ([`QueueNever`](../../backend/orchestration/internal/queue/decision.go#L23)),
    the same field [`ListReadyIssues`](../../backend/orchestration/internal/queue/provider.go#L119)
    polls for `Ready`. It must be implemented **on the provider** (symmetric to the
    read/poll and to the existing write [`CloseIssue`](../../backend/orchestration/internal/queue/provider.go#L166)),
    **not** a one-off hardcoded `gh`/GitHub call inside Eject. It is **idempotent**
    (already `Never` ⇒ no-op) and a **safe no-op** when the worktree has no
    provider-backed task. It must **not** close the issue (the issue stays open).
    Concrete home: the new method on
    [`provider.go`](../../backend/orchestration/internal/queue/provider.go) (interface
    + `ghQueueProvider` gh-CLI implementation), wired into the per-repo Service the
    same way the read provider is built in
    [`queuedrain.go` L189](../../backend/orchestration/internal/temporalbackend/queuedrain.go#L189).
    Eject (Goal 6) and the standalone dequeue endpoint (Goal 11) both call it.
11. **[BE] Standalone dequeue endpoint.** `POST /api/v1/worktrees/dequeue` invokes
    the provider dequeue (Goal 10) **without** ejecting, so the operator can take a
    task out of the queue while leaving the run alone, and Task-0017's UI has the
    seam. Method-guarded like the other handlers. Does **not** close the issue.

## Acceptance Criteria

Each criterion is pass/fail. All are **[BE]** (backend-only).

1. **queue_workers removed.** `QueueWorkers` no longer exists on
   [`RepoEntry`](../../backend/orchestration/internal/queue/manifest.go#L29), and
   the queue-drain consumer admits a dispatch based on idle-pool availability, not
   a numeric cap: with a pool of 1 idle worktree and two Ready issues, the consumer
   dispatches exactly one and skips the second (it waits). Asserted by an updated
   [`consumer_test.go`](../../backend/orchestration/internal/queue/consumer_test.go)
   that drives admission from a pool (no `fixedSizer`/`EvaluateSlot` cap). A grep
   proof shows `queue_workers` / `RepoSlotLimit` / `EvaluateSlot` are no longer a
   live admission cap anywhere.
2. **Create.** `POST /api/v1/worktrees/create {repo}` provisions one new idle
   worktree at a **stable** path (not `os.MkdirTemp`), persists its pool record with
   `run_id=null`, and a follow-up `GET /api/v1/worktrees` lists it `status:"idle"`
   with that `worktree_id` + `worktree_path`. Go unit test (fake runtime as in
   [`service_test.go`](../../backend/orchestration/internal/taskrun/service_test.go)).
3. **Assign reuses an idle worktree.** `POST /api/v1/worktrees/assign {task_id, repo, worktree_id}`
   resets the chosen **existing** idle worktree and starts the run in it; a Go unit
   test asserts (a) `StartTaskRun` was called for that task, (b) the run is bound to
   the **same** `worktree_path` that was idle (no fresh dir provisioned), and (c)
   the pool count did not grow.
4. **Assign rejects when none idle.** `POST /api/v1/worktrees/assign` returns 409
   and starts no run when the repo has no idle worktree. Go unit test.
5. **Auto-assign draws from the pool.** With `worktree_id` omitted (the consumer
   path), Assign picks any idle worktree in the repo; the consumer dispatches into a
   pool worktree and **defers** (skips) when the pool is empty. Go unit test on the
   consumer + service seam.
6. **Eject keeps the folder + returns idle + dequeues.** `POST /api/v1/worktrees/eject {run_id}`
   stops the agent, resets the checkout to baseline, unbinds the run, and the folder
   **still exists on disk** afterward, listed `status:"idle"`. A Go unit test
   asserts the directory still exists and the pool record `run_id` is null —
   covering **both** a `running` lane and a `parked_*` lane (works regardless of
   parked state). A test that finds the folder deleted **fails**. The same test (or a
   sibling) asserts that Eject called the provider dequeue for the freed task's issue
   (`Queue → Never`) via a **fake provider**, and that it did **not** call the
   provider close path. A test where Eject leaves the issue `Queue=Ready` **fails**.
7. **Destroy rejects allocated; removes idle.** `POST /api/v1/worktrees/destroy {worktree_id}`
   returns 409 and removes nothing when the worktree is allocated; when idle it
   removes the folder (via
   [`removeOwnedLaneWorktree`](../../backend/orchestration/internal/taskrun/service.go#L1688))
   and drops the pool record. Go unit tests for both cases.
8. **Discover-on-startup across restart.** Given pool folders on disk — some bound
   to a live run, some idle — a fresh `Service` (simulating restart) reconstructs
   each worktree's allocated-vs-idle state from disk + the live workflow, with no
   bound state lost. Go unit test that builds the on-disk pool, constructs a new
   service, and asserts `ListPoolWorktrees()` reports the correct `status` and bound
   `task_id`/`run_id` per worktree.
9. **Full-pool read.** `GET /api/v1/worktrees` returns allocated + idle entries,
   each with `worktree_id`, `repo`, `worktree_path`, `status`; allocated entries add
   `task_id`, `run_id`, `run_gate_state` from the live binding. Asserted by a Go
   unit test on the handler/service.
10. **Repos read without queue_workers.** `GET /api/v1/repos` returns one entry per
    registered repo with `id` + `local_root` and **no** `queue_workers`, matching
    [`RegistryRepos()`](../../backend/orchestration/internal/queue/registry_consumer.go#L33)
    for a fixture registry. Go unit test.
11. **Method/path guards.** The new routes return 405 on the wrong method and 404
    on unknown sub-paths, matching the existing handler guards. Asserted in
    `mux_test.go`.
12. **Provider dequeue write sets not-ready.** The new
    [`QueueProvider`](../../backend/orchestration/internal/queue/provider.go#L22)
    dequeue write sets the task's queue state to not-ready (`github_issues`:
    `Queue → Never`) **through the provider**. A Go unit test against a **fake/mock
    provider** asserts the dequeue write call was made with the correct repo + issue
    number and the not-ready value. The test must **not** hit real GitHub (it uses
    the fake provider / injectable `run` func, exactly as the existing provider tests
    do). A dequeue implemented as a hardcoded inline `gh`/GitHub call inside Eject
    instead of on the provider **fails** this criterion.
13. **Eject does not re-dispatch (no bounce-back).** After Eject of a task whose
    issue is `Queue=Ready`, a subsequent consumer poll does **not** re-dispatch that
    task (because Eject dequeued it). Asserted by a Go test on the consumer + service
    seam with a **fake provider**: Eject sets the fake's `Queue` to `Never`, the next
    `DrainOnce` observes `Never`, and the task is **not** in `Dispatched`. A variant
    where Eject skips the dequeue (issue stays `Ready`) shows the task **is**
    re-dispatched — proving the dequeue is load-bearing.
14. **Dequeue leaves the issue open (not closed).** The provider dequeue write and
    the Eject dequeue **only** set `Queue=Never`; they do **not** call any close
    path. A Go unit test asserts the fake provider's `CloseIssue` was **never**
    invoked by Eject / dequeue (the issue stays open). A dequeue that closes the
    issue **fails**.
15. **Standalone dequeue endpoint.** `POST /api/v1/worktrees/dequeue` invokes the
    provider dequeue for the named task **without** ejecting (the run is not stopped
    and the worktree is not returned to idle) and does **not** close the issue;
    method/path guards return 405 on the wrong method and 404 on unknown sub-paths.
    Asserted in `mux_test.go` + a service test with a fake provider.

**Regression [must not break]:** `REG-007`, `REG-008`, `REG-009` in
[`REGRESSION.md`](../../REGRESSION.md#L325) re-run **green under the new model**,
with the pool **seeded (via Create) before the drain can dispatch**:

- **REG-007** — "cap=1" is now "**a pool of 1**": with one idle worktree, the
  consumer dispatches exactly one Ready issue and the second **waits for an idle
  worktree**; on close/eject the freed worktree is reused. (The
  [REG-007 cap=1 sub-scenario](../../REGRESSION.md#L438) is reinterpreted: one idle
  pool worktree, not `queue_workers=1`.)
- **REG-008** — durable state survives a backend restart: the parked lane is still
  reported from the workflow, and the pool's allocated-vs-idle classification is
  reconstructed by **discover-on-startup**.
- **REG-009** — each repo's pool is independent: a Create/Assign/Eject/Destroy or a
  close on repo A never touches repo B's pool worktrees.

## Non-Goals

- **The Tkinter UI redesign is deferred to a future Task-0017.** Task-0016 does
  **not** touch [`app/codex_dashboard/*`](../../app/codex_dashboard/) — no Tk code,
  no changes to `ui.py` / `tasks_tab.py` / `tasks_backend.py`. The worktree-pool
  UI (the worktree-management view, D1=replace) is a follow-on task to be drafted
  separately; Task-0016 only adds the HTTP endpoints so the lifecycle is
  drivable/testable headlessly and Task-0017 has a backend seam.
- **No auto-create / auto-seed.** The backend never grows the pool on its own; an
  empty pool defers Ready issues. Capacity is operator-owned (the human explicitly
  accepts this).
- **No change to the done-contract / park-in-place closure rules or human-only
  closure** ([`decision.go`](../../backend/orchestration/internal/queue/decision.go):
  only a closed issue deallocates; `Human Needed=Yes` parks in place; the agent
  never self-closes). Eject is an operator action that returns a folder to idle; it
  is **not** a new autonomous deallocation path and does not change when the
  consumer reclaims. The new dequeue write only sets `Queue=Never`; it **never
  closes** the issue, so human-only closure is preserved (Eject/dequeue are
  operator-initiated human actions, not an agent self-close).
- **No change to the queue-drain consumer's poll/decision semantics** beyond the
  cap removal and the pool-draw dispatch path. The consumer's
  [`DecideQueueAction`](../../backend/orchestration/internal/queue/decision.go#L102)
  logic is unchanged; the new dequeue is a Service/provider **write** invoked by
  Eject and the dequeue endpoint, not a new consumer decision branch. The consumer
  remains read-only in its own poll path (it still only *reads* `Queue`).
- **No change to the Usage or Jobs tabs**, or to any other endpoint.
- The original Stitch-mockup UI exclusions **E2–E7** still stand (drag-to-bind /
  Task Browser pane, Register New Task, model chip, animated transitional states,
  decorative progress bars, top-nav chrome changes) — but they are Task-0017's
  concern, not this task's. **E1 (manual worktree create/destroy) is reversed: it
  is now in scope** as the Create / Destroy operations above.

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `16`
- Local task path: `Tracking/Task-0016/TASK.md`
- Source commit: `917e4c752a8d2c76b9717bf8ec3149b92136b4d7`
- Local task SHA-256: `2549EE1695327F80E765F9147F7CF7C6FCDB73D4D0E5B8240F7300E4DA5A6438`
- Rendered at: `2026-05-31T22:52:33.5299909-04:00`