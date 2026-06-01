# Task-0016 Plan — Manual persistent worktree pool (Go backend) + the desktop WORKTREES tab

Status: **AWAITING EXPLICIT HUMAN PLAN APPROVAL. No implementation has started.**

Grounded in [TASK.md](./TASK.md), [HUMAN-DIRECTIVES-FOR-WORKER.md](./HUMAN-DIRECTIVES-FOR-WORKER.md)
(**UPDATE 3** at the top of that file is the LATEST authoritative directive — it
**reverses** the earlier backend-only split: the Tk `WORKTREES` tab is **back in scope**
and "done" requires the **working in-app human surface**; the manual-pool MODEL from the
DESIGN PIVOT and the Eject-dequeue from UPDATE 2 still stand), the TaskCreate coordinator
notes ([TASK-CREATE-COORDINATOR-NOTES.md](./TASK-CREATE-COORDINATOR-NOTES.md) v2/v3), a
direct read of the live backend code (verified below), and a direct read of the live
desktop-app structure ([ui.py](../../app/codex_dashboard/ui.py) nav tuple L671 / tab
dispatch, [tasks_backend.py](../../app/codex_dashboard/tasks_backend.py) HTTP-client
pattern, [tasks_tab.py](../../app/codex_dashboard/tasks_tab.py) pure-helper pattern).

**Scope reversal note (UPDATE 3).** An earlier version of this plan was sequenced
backend-only (PASS-0000…PASS-0006) under the now-RESCINDED backend-only split. This plan
is revised to the reversed scope: **one testable chunk** delivering BOTH the Go backend
manual worktree-pool lifecycle AND the Tkinter `WORKTREES`-tab UI that consumes it, closed
as ONE unit, with closure gated on the **working in-app surface** plus its named in-app
regression cases (REG-010…REG-016). No Task-0017 is created; the UI is not deferred.

**Single-context note (honest labeling).** Planning was performed directly by the
TaskDispatch task leader because **no nested implementation-leader / sub-agent
dispatch tool is available in this runtime** (only task-stop / worktree / todo
tooling is exposed). The same planning discipline and the explicit human plan gate
are honored. The same limitation means a clean-context QA lane and per-pass
delegated implementers may not be creatable from here — this is flagged to the
TaskDispatch coordinator in [HANDOFF.md](./HANDOFF.md) ("Process gaps"), not silently
absorbed.

## Scope Discipline

- This is **one testable chunk with two homes**: the **Go backend** worktree-pool
  lifecycle AND the **Tkinter desktop `WORKTREES` tab**
  ([`app/codex_dashboard/*`](../../app/codex_dashboard/)) that consumes it. Both ship
  in Task-0016; there is **no Task-0017** and the UI is **not** deferred.
- **"Done" = the working in-app human surface** (Human-Facing Outcome Rule, UPDATE 3).
  Backend endpoints + unit/server-smoke do **not** satisfy closure on their own; the
  `WORKTREES` tab must actually work in-app and the named in-app cases
  **REG-010…REG-016** must pass on the isolated validation lane before closure.
- The backend mechanisms in [TASK.md "Why these mechanisms belong in one task"](./TASK.md)
  are **one model swap**; the UI is the **one human surface** for that model. The passes
  below are an **execution order** chosen so each pass is independently provable per the
  task's own internal-separability note — not a scope split. The backend passes come
  first (the UI consumes their endpoints), then the UI passes, then the
  regression-authoring/run + closure pass.
- The Stitch HTML mockup is a **structural guide only** (Q1 = a): re-implement in the
  existing Python/Tkinter app, reuse its `ttk` styles + palette; D1 = a (replace the
  `TASKS`-tab content); `TASKS` → `WORKTREES` rename; mockup exclusions **E2–E7** out of
  scope; **E1 reversed** (Create/Destroy in scope).
- Each pass maps to specific [Acceptance Criteria](./TASK.md) ([BE] 1–15, [FE] 16–22)
  and to the matching [What Does Not Count](./TASK.md) falsifier(s).
- **Human-only closure is preserved.** Eject/dequeue are operator-initiated human
  actions; the new dequeue write only sets `Queue=Never` and **never** closes the
  issue. The agent never self-closes. This is honored at this task's own closure too
  (the agent never marks Task-0016 closed; that is a human gate).

## Verified code baseline (pre-implementation)

Confirmed against the live tree (the line refs in `TASK.md` are accurate):

- Cap path exists exactly as described: [`RepoEntry.QueueWorkers`](../../backend/orchestration/internal/queue/manifest.go#L32),
  `QueueWorkersForRoot` / `DefaultQueueWorkers` (manifest.go), the
  `repoSlotLimit`/`manifestQueueWorkers` field + [`RepoSlotLimit()`](../../backend/orchestration/internal/taskrun/service.go#L616),
  [`EvaluateSlot`](../../backend/orchestration/internal/queue/slots.go#L39) + the
  `SlotSizer` interface ([consumer.go L46](../../backend/orchestration/internal/queue/consumer.go#L46)),
  [`EffectiveFreeConcurrency`](../../backend/orchestration/internal/queue/decision.go#L144),
  and the `queueWorkers` arg threaded into
  [`NewServiceForRepo`](../../backend/orchestration/internal/taskrun/service.go#L102)
  from [`queuedrain.go` L196](../../backend/orchestration/internal/temporalbackend/queuedrain.go#L196).
- Worktree mechanics exist: [`provisionOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1507)
  (`os.MkdirTemp` random dirs), [`bootstrapOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1561),
  [`restoreOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1726),
  [`cleanupOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1640) →
  [`removeOwnedLaneWorktree`](../../backend/orchestration/internal/taskrun/service.go#L1688),
  prune-only [`ReconcileOwnedLanes`](../../backend/orchestration/internal/taskrun/service.go#L1663),
  [`ListActiveWorktrees`](../../backend/orchestration/internal/taskrun/service.go#L223),
  the durable [`ownedLaneBootstrapRecord`](../../backend/orchestration/internal/taskrun/service.go#L68)
  + [`collectActiveLaneRecords`](../../backend/orchestration/internal/taskrun/service.go#L271) /
  [`bindingsFromRecords`](../../backend/orchestration/internal/taskrun/service.go#L251) /
  [`liveBindingForRecord`](../../backend/orchestration/internal/taskrun/service.go#L358),
  [`ReclaimOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L506) +
  [`terminateAgentProcess`](../../backend/orchestration/internal/taskrun/service.go#L528).
- Provider surface exists: [`QueueProvider`](../../backend/orchestration/internal/queue/provider.go#L22)
  interface, [`ghQueueProvider`](../../backend/orchestration/internal/queue/provider.go#L49)
  with the injectable `run` func ([provider.go L58](../../backend/orchestration/internal/queue/provider.go#L58)),
  [`ListReadyIssues`](../../backend/orchestration/internal/queue/provider.go#L119) Queue
  read, [`fieldIDMap`](../../backend/orchestration/internal/queue/provider.go#L174) /
  [`fieldValues`](../../backend/orchestration/internal/queue/provider.go#L202), the
  existing WRITE [`CloseIssue`](../../backend/orchestration/internal/queue/provider.go#L166),
  and [`NewGitHubQueueProvider`](../../backend/orchestration/internal/temporalbackend/queuedrain.go#L189).
- Routes: [`NewMux`](../../backend/orchestration/internal/httpapi/mux.go#L25),
  [`handleWorktreesList`](../../backend/orchestration/internal/httpapi/mux.go#L130)
  (active-only today), [`handleTaskAPIRoute`](../../backend/orchestration/internal/httpapi/mux.go#L202),
  [resolve-interrupt-review](../../backend/orchestration/internal/httpapi/mux.go#L429).
- Consumer: [`DrainOnce`](../../backend/orchestration/internal/queue/consumer.go#L126)
  admission, [`ActionDispatch`](../../backend/orchestration/internal/queue/consumer.go#L199),
  [`IssueNumberFromTaskID`](../../backend/orchestration/internal/queue/consumer.go#L93).
- Baseline is green: `go build ./...` exit 0; `go test ./internal/queue/...
  ./internal/taskrun/... ./internal/httpapi/...` all `ok` (cached) before any change.
- Existing test files to extend exist:
  [`service_test.go`](../../backend/orchestration/internal/taskrun/service_test.go),
  [`worktrees_test.go`](../../backend/orchestration/internal/httpapi/worktrees_test.go),
  `mux_test.go`, [`consumer_test.go`](../../backend/orchestration/internal/queue/consumer_test.go),
  `slots_test.go`, [`provider_test.go`](../../backend/orchestration/internal/queue/provider_test.go).
- **Frontend baseline confirmed (this re-plan).** The desktop app structure the [FE]
  passes target is present and matches the task's references: the nav-tab tuple at
  [ui.py L671](../../app/codex_dashboard/ui.py#L671) (`("usage","Usage"),("jobs","Jobs"),("tasks","Tasks")`),
  `select_tab` ([L1152](../../app/codex_dashboard/ui.py#L1152)) / `_render_active_tab`
  ([L1164](../../app/codex_dashboard/ui.py#L1164)) tab dispatch, the `_configure_styles`
  palette constants ([L100-102](../../app/codex_dashboard/ui.py#L100): `#0a0e14`/`#1c2026`
  surfaces, `#00e5ff`/`#c3f5ff` accents), the `urllib` HTTP-client pattern in
  [`tasks_backend.py`](../../app/codex_dashboard/tasks_backend.py)
  (`fetch_tasks_snapshot` L29 reused by the Assign popup, dispatch-POST pattern L38), and
  the pure-helper pattern in [`tasks_tab.py`](../../app/codex_dashboard/tasks_tab.py). New
  files to add: `worktrees_backend.py`, `worktrees_tab.py`, `tests/test_worktrees_tab.py`.
  The Stitch mockup folder `C:\Users\gregs\Downloads\stitch_codex_token_velocity_overlay (4)`
  exists (structural guide only).

## Pass Order And Dependencies

Dependency reasoning: the backend is built first **because the UI consumes its
endpoints** — the `WORKTREES` tab cannot work in-app until the pool reads + lifecycle
endpoints exist. Within the backend, the **pool record + stable identity** is the thing
every other mechanism reads or mutates, so it is built first; **discover-on-startup**
reconstructs that record; the **four lifecycle ops + reads** mutate/expose it; the **cap
removal + pool-draw dispatch** is the consumer-side swap that depends on pool-draw Assign
existing; the **provider dequeue write** is a self-contained provider addition that Eject
couples to. Then the **UI** is built on those endpoints (read view first, then the
mutating controls), and finally the **named in-app regression cases** are run against the
live backend to satisfy the "done = working surface" bar. The ordering is
provability-driven, not a scope split; all mechanisms + the UI ship in this one task.

Backend passes (endpoints the UI consumes):

- **PASS-0000** → Pool record + stable identity (TASK.md §2; AC2 foundation).
- **PASS-0001** → Discover-on-startup replacing prune-only reconcile (§3; AC8; REG-008).
- **PASS-0002** → Create + Destroy + full-pool/repos reads + route guards
  (§4, §7, §8; AC2, AC7, AC9, AC10, AC11).
- **PASS-0003** → Assign (pool-draw reuse, reject-when-none) + dispatch-path change +
  `queue_workers` removal (§5, §9, §1; AC1, AC3, AC4, AC5; REG-007 "pool of 1").
- **PASS-0004** → Provider dequeue write + standalone dequeue endpoint
  (§10, §11; AC12, AC14, AC15).
- **PASS-0005** → Eject (keep-folder + return-idle + dequeue) + the
  consumer+service no-bounce-back seam (§6; AC6, AC13, AC14).
- **PASS-0006** → Backend cross-cut: full `go test ./...` + server-only smoke of the
  endpoints (supporting proof for the UI; the REG-007/008/009 in-app re-run moves to the
  final pass alongside the new in-app cases).

UI passes (the human surface — "done" depends on these):

- **PASS-0007** → Frontend foundation: rename `TASKS` → `WORKTREES`, replace the tab
  content (D1=a), the `worktrees_backend.py` HTTP client + `worktrees_tab.py` pure
  helpers, the read-only **pool view** (allocated/idle color + repo + path), the
  **copy-path** control, and the **registry-sourced repo filter** (§12 Goals 12–15;
  AC16–AC19; REG-010, REG-011, REG-012).
- **PASS-0008** → Frontend mutating controls wired to the endpoints: **Create**, the
  **Assign** popup (open-tasks from `GET /api/v1/tasks` → bind), **Eject**, **Destroy**
  (idle-only + allocated rejection), and the standalone **Dequeue** (Goals 16–18;
  AC20–AC22; REG-013, REG-014, REG-015, REG-016).
- **PASS-0009** → Cross-cutting closure: the **in-app regression run** of the new
  `WORKTREES`-tab cases **REG-010…REG-016** on the isolated validation lane against the
  live backend, PLUS the **REG-007/008/009** in-app re-run under the new model (pool
  seeded via Create). Capture `REGRESSION-RUN-<NNNN>.md` artifacts under
  [`Testing/`](./Testing/); confirm full `go test ./...` and the Python unit suite green.

Each closed pass: implement → unit tests → pass audit under
[`Testing/`](./Testing/) → HANDOFF update → commit → push → final toast (per
[`ORCHESTRATION.md`](../../../../Users/gregs/.codex/Orchestration/ORCHESTRATION.md)
Required Sequence). Where the runtime allows, rotate to a fresh implementation
context per pass; under the single-context fallback this is the same context and is
labeled as such. The UI passes additionally require the repo-local UI-fidelity / clean
in-app proof discipline (see Risks) and a clean-context QA verdict arranged by the
coordinator — not producer self-review.

---

## PASS-0000 — Pool record + stable identity

**Objective (TASK.md §2, Goal 2).** Replace the random-temp model's *identity* with a
**stable path + stable `worktree_id`** and a durable **pool record** that can carry an
**idle** (`run_id=null`) member.

**Changes**
- Add a stable layout helper in [`service.go`](../../backend/orchestration/internal/taskrun/service.go):
  pool path `<ownedLaneRoot>/<repoID>/wt-<NNNN>/w`, `worktree_id = <repoID>/wt-<NNNN>`.
  Zero-pad width is an impl detail (default 4). A helper allocates the next free
  `wt-<NNNN>` for a repo by scanning existing pool folders.
- Extend the durable [`ownedLaneBootstrapRecord`](../../backend/orchestration/internal/taskrun/service.go#L68)
  (per the task's Open-Questions default) to persist `worktree_id`, stable
  `worktree_path`, `repo`, and **`run_id`-or-null**. Idle members persist with
  `run_id=null` so they survive with no run bound. (A sibling `worktree-pool.json`
  per folder is the acceptable alternate if idle persistence is cleaner; either way
  the four fields are mandatory.)
- Extend [`collectActiveLaneRecords`](../../backend/orchestration/internal/taskrun/service.go#L271)
  / [`bindingsFromRecords`](../../backend/orchestration/internal/taskrun/service.go#L251)
  so an idle folder that **exists** is surfaced as idle (today a record whose checkout
  dir is gone is dropped; idle members must not be dropped just because no run is
  bound). Add the `status` (`idle`/`allocated`) discriminator to the binding/view type
  in [`types.go`](../../backend/orchestration/internal/taskrun/types.go) used by the
  pool reads.

**Proof** — Go unit test (fake runtime, as in
[`service_test.go`](../../backend/orchestration/internal/taskrun/service_test.go)):
write a pool record with `run_id=null`, read it back, assert id stability across two
reads and that the idle member is enumerated (not dropped). `go build ./...` + the
three affected packages green.

**Falsifier guard** — a record that cannot represent `run_id=null`, or an id that is
not stable across reads, fails this pass.

---

## PASS-0001 — Discover-on-startup (replaces prune-only reconcile)

**Objective (TASK.md §3, Goal 3; REG-008).** Startup **enumerates** the pool folders
that exist on disk per repo and reconstructs each one's **idle vs allocated** state;
allocated = bound to a **live run**, read from the per-run `TaskRunWorkflow`
(Task-0015 Landing-2 authority). No auto-seeding to a count; no folder created or
destroyed by discovery. Survives a backend restart.

**Changes**
- Replace the body of [`ReconcileOwnedLanes()`](../../backend/orchestration/internal/taskrun/service.go#L1663)
  (or add `DiscoverPool()` invoked at the same wiring point,
  [`queuedrain.go` L226](../../backend/orchestration/internal/temporalbackend/queuedrain.go#L226)):
  per repo, enumerate pool folders (stable-path layout from PASS-0000), classify each
  via the live binding ([`liveBindingForRecord`](../../backend/orchestration/internal/taskrun/service.go#L358),
  the Landing-2 workflow query), and **keep** the existing `git worktree prune`
  hygiene for genuinely stale metadata.
- Add `Service.ListPoolWorktrees()` returning the **full** pool (idle + allocated) per
  repo, used by both discover assertions and the §8 read.

**Proof** — Go unit test (TASK.md AC8): build an on-disk pool — some folders bound to a
live run, some idle — construct a **fresh** `Service` (simulating restart), assert
`ListPoolWorktrees()` reports correct `status` and bound `task_id`/`run_id` per
worktree with **no bound state lost**.

**Falsifier guard** (TASK.md "What Does Not Count") — a discover that **loses bound
state across a restart** (reclassifies a live-allocated worktree as idle, or drops it)
fails. Maps to **REG-008** (pool classification reconstructed by discover-on-startup,
parked lane still reported from the workflow).

---

## PASS-0002 — Create + Destroy + full-pool/repos reads + route guards

**Objective (TASK.md §4, §7, §8; Goals 4, 7, 8).** The operator can pre-create idle
pool members, destroy idle ones (reject allocated), and read the full pool + the repo
list — all method/path-guarded.

**Changes**
- `Service.CreatePoolWorktree(repo)`: provision one new **idle** worktree at the next
  stable path (`git worktree add --detach <stablePath> <baselineCommit>` — the
  [`provisionOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1507)
  git mechanics, but **stable, non-temp** path and **no task bound**), write its pool
  record `run_id=null`, return it. **`POST /api/v1/worktrees/create`**.
- `Service.DestroyPoolWorktree(worktreeID)`: **reject (409, nothing removed)** if
  allocated (record `run_id` non-null / live run bound); else remove the idle folder
  via [`cleanupOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1640)
  → [`removeOwnedLaneWorktree`](../../backend/orchestration/internal/taskrun/service.go#L1688)
  (the BUG-0002-hardened PID-safe `git worktree remove --force` mechanics) and drop the
  pool record. **`POST /api/v1/worktrees/destroy`**.
- `Service.ListRepos()` from [`queue.LoadRegistry`](../../backend/orchestration/internal/queue/manifest.go#L70)
  / [`RegistryRepos()`](../../backend/orchestration/internal/queue/registry_consumer.go#L33),
  projecting `id` + `local_root` (+ `task_provider_repo`), with **no `queue_workers`**.
  **`GET /api/v1/repos`** via new `handleReposList`.
- Extend [`handleWorktreesList`](../../backend/orchestration/internal/httpapi/mux.go#L130)
  to return the **full pool** (idle + allocated) from `ListPoolWorktrees()`, each entry
  with `worktree_id`, `repo`, `worktree_path`, `status`; allocated entries add
  `task_id`/`run_id`/`run_gate_state` from the live binding (existing
  [`RunGateState`](../../backend/orchestration/internal/taskrun/types.go#L147) enum).
- Add a method-guarded `handleWorktreeAPIRoute` on the `/api/v1/worktrees/*` surface
  mirroring [`handleTaskAPIRoute`](../../backend/orchestration/internal/httpapi/mux.go#L202)
  (405 wrong method, 404 unknown sub-path); register the new routes in
  [`NewMux`](../../backend/orchestration/internal/httpapi/mux.go#L25).

**Proof** — Go unit tests: Create (AC2 — idle at a **stable** path, not `os.MkdirTemp`;
follow-up list shows it idle); Destroy rejects-allocated + removes-idle (AC7); full-pool
read shape (AC9); repos read **without** `queue_workers` against a fixture registry
(AC10); method/path guards in `mux_test.go` (AC11).

**Falsifier guards** — a Destroy that deletes an **allocated** worktree fails (AC7); a
repos response that still carries `queue_workers` fails (AC10).

---

## PASS-0003 — Assign + dispatch-path change + `queue_workers` removal

**Objective (TASK.md §5, §9, §1; Goals 5, 9, 1).** Assign **reuses an existing idle**
worktree (no fresh dir); the consumer dispatch path becomes **pool-draw**; the numeric
cap is removed so concurrency is bounded by idle-pool count by construction.

**Changes**
- `Service.AssignTaskToPoolWorktree(ctx, taskID, repo, worktreeID)`: resolve the named
  idle `worktree_id`, else (consumer path, id omitted) **any** idle worktree in the
  repo; **reject (409, no run started)** if none idle; reset that **existing** checkout
  via [`restoreOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1726)
  (`git reset --hard` + `git clean -fd`); then **bind + start in that worktree** via the
  bootstrap→start tail of [`dispatchWithDirective`](../../backend/orchestration/internal/taskrun/service.go#L671)
  ([`bootstrapOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1561)
  → `runtime.StartTaskRun`), **without** calling
  [`provisionOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1507);
  update the pool record `run_id`. **`POST /api/v1/worktrees/assign`**, returns the
  started [`TaskRunView`](../../backend/orchestration/internal/taskrun/types.go#L242) (202).
- Dispatch-path change: in [`dispatchWithDirective`](../../backend/orchestration/internal/taskrun/service.go#L671)
  replace the `provisionOwnedLane(task.TaskID)` call with **pool-draw** (idle worktree →
  reset → bootstrap→start). Empty pool ⇒ dispatch refused; the consumer
  ([`ActionDispatch`](../../backend/orchestration/internal/queue/consumer.go#L199)) skips
  and re-picks on a later poll. Creation is no longer a dispatch side effect.
- `queue_workers` removal: delete `QueueWorkers` from
  [`RepoEntry`](../../backend/orchestration/internal/queue/manifest.go#L32) and the
  `QueueWorkersForRoot`/`DefaultQueueWorkers` admission helpers; remove
  [`RepoSlotLimit()`](../../backend/orchestration/internal/taskrun/service.go#L616) +
  `repoSlotLimit`/`manifestQueueWorkers`, the `SlotSizer` consumption +
  [`EvaluateSlot`](../../backend/orchestration/internal/queue/slots.go#L39), and
  [`EffectiveFreeConcurrency`](../../backend/orchestration/internal/queue/decision.go#L144)'s
  cap arithmetic; drop the `queueWorkers` param from
  [`NewServiceForRepo`](../../backend/orchestration/internal/taskrun/service.go#L102)
  and its call site ([`queuedrain.go` L196](../../backend/orchestration/internal/temporalbackend/queuedrain.go#L196)).
  Admission becomes "is there an idle pool worktree for this repo?". Update tests that
  drove a fixed `SlotSizer`/`EvaluateSlot` cap (`consumer_test.go`, `slots_test.go`) to
  drive admission from a pool of idle worktrees. **Migration:** the JSON key is simply
  no longer consulted (unknown keys already ignored), so an old `REPO-MANIFEST.json`
  still loads.

**Proof** — Go unit tests: Assign reuses idle (AC3 — `StartTaskRun` called, run bound to
the **same** idle `worktree_path`, pool count did **not** grow); Assign rejects-when-none
(AC4 — 409, no run); auto-assign + consumer defers on empty pool (AC5, consumer+service
seam); cap-removal admission (AC1 — pool of 1 + two Ready issues ⇒ exactly one dispatched,
second waits; updated `consumer_test.go` with no `fixedSizer`). A grep proof shows
`queue_workers`/`RepoSlotLimit`/`EvaluateSlot` are no longer a live admission cap.

**Falsifier guards** — an Assign that provisions a **fresh dir** (`provisionOwnedLane`/
`os.MkdirTemp`) or grows the pool fails (AC3); leaving `queue_workers` a live cap (or
merely renaming it) fails (AC1); auto-creating a worktree to satisfy a Ready issue
instead of deferring on an empty pool fails. Maps to **REG-007 "pool of 1"**.

---

## PASS-0004 — Provider dequeue write + standalone dequeue endpoint

**Objective (TASK.md §10, §11; Goals 10, 11).** The **first task-provider queue WRITE**:
set the issue's `Queue` single-select to `Never` **through the provider abstraction**
(symmetric to the `Queue` read and to `CloseIssue`), idempotent, never closes the issue.
Expose it standalone so it can run without ejecting.

**Changes**
- Add a dequeue write to the [`QueueProvider`](../../backend/orchestration/internal/queue/provider.go#L22)
  interface — `DequeueIssue(repo string, number int) error` (default name; equivalently
  `SetQueueState(repo, number, QueueNever)`), one-line doc: provider WRITE that sets the
  queue state to not-ready and **never** closes the issue.
- Implement on [`ghQueueProvider`](../../backend/orchestration/internal/queue/provider.go#L49)
  as the symmetric sibling of the `Queue` read: resolve the `Queue` field id via
  [`fieldIDMap()`](../../backend/orchestration/internal/queue/provider.go#L174), resolve
  the `Never` option id ([`QueueNever`](../../backend/orchestration/internal/queue/decision.go#L23)),
  and `gh api` the issue-field-value for `Queue` to `Never` through the injectable `run`
  func ([provider.go L58](../../backend/orchestration/internal/queue/provider.go#L58)) so a
  test never touches real GitHub. Idempotent (already `Never` ⇒ no-op).
- Wire the write capability into the per-repo Service that runs Eject/dequeue: add a small
  **write-provider seam** on the Service (a `DequeueProvider`-style field, fake-able like
  the existing fake runtime), injected the same way the read provider is built via
  [`NewGitHubQueueProvider`](../../backend/orchestration/internal/temporalbackend/queuedrain.go#L189).
- `Service.DequeueTask(repo, taskID)`: resolve the issue number with
  [`IssueNumberFromTaskID`](../../backend/orchestration/internal/queue/consumer.go#L93)
  and call the provider dequeue — **without** stopping the agent, cleaning the checkout,
  or unbinding. Safe no-op when the task has no parseable issue number.
  **`POST /api/v1/worktrees/dequeue`**, method/path-guarded; does **not** close the issue.

**Proof** — Go unit tests against a **fake/mock provider** (or injectable `run`),
**never** real GitHub: dequeue sets not-ready with the correct repo + issue number
(AC12); a dequeue implemented as an inline hardcoded `gh` call **fails** (AC12); the fake
provider's `CloseIssue` is **never** invoked (AC14); standalone endpoint invokes dequeue
without ejecting and does not close (AC15); guards in `mux_test.go` (AC15).

**Falsifier guards** — a dequeue that bypasses the provider (inline `gh`) fails (AC12); a
dequeue that **closes** the issue fails (AC14).

---

## PASS-0005 — Eject (keep folder + return idle + dequeue) + no-bounce-back seam

**Objective (TASK.md §6; Goals 6; AC6, AC13, AC14).** Eject stops the agent, cleans the
checkout to baseline, unbinds the run, **keeps the folder** (returns it idle), and
**dequeues the freed task** so the still-`Ready` task is **not** re-dispatched. Works
regardless of parked state. Never deletes the folder, never closes the issue.

**Changes**
- `Service.EjectWorktree(ctx, runID)` (default key `run_id`; accept `worktree_id` as
  alternate): terminate the launched agent (the
  [`terminateAgentProcess`](../../backend/orchestration/internal/taskrun/service.go#L528)
  PID kill that [`ReclaimOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L506)
  already performs) and unbind/terminate the run; clean to baseline via
  [`restoreOwnedLane`](../../backend/orchestration/internal/taskrun/service.go#L1726)
  with `-fdx` (drop ignored files for a true baseline); **keep the folder** and set the
  pool record `run_id=null` — must **not** call
  [`removeOwnedLaneWorktree`](../../backend/orchestration/internal/taskrun/service.go#L1688);
  **then dequeue** the freed task via the PASS-0004 provider write (safe no-op when no
  provider-backed task). Works on a `running` **and** a `parked_*` lane — unlike
  [resolve-interrupt-review](../../backend/orchestration/internal/httpapi/mux.go#L429),
  which only reclaims a parked run. **`POST /api/v1/worktrees/eject`**, returns the
  now-idle worktree (200).

**Proof** — Go unit tests: Eject keeps folder + returns idle (AC6 — directory **still
exists** afterward, listed `status:"idle"`, record `run_id` null; covers **both** a
`running` and a `parked_*` lane; a test that finds the folder deleted **fails**); same/
sibling test asserts Eject called the provider dequeue (`Queue → Never`) via the **fake
provider** and did **not** call close (AC6, AC14); the consumer+service **no-bounce-back**
seam (AC13 — after Eject, `DrainOnce` observes `Never` and the task is **not** in
`Dispatched`; the load-bearing variant where Eject skips the dequeue shows the task **is**
re-dispatched).

**Falsifier guards** — an Eject that **deletes** the folder fails (AC6); an Eject that
leaves the issue `Queue=Ready` so the consumer re-dispatches it (the bounce-back) fails
(AC13); an Eject that closes the issue fails (AC14).

---

## PASS-0006 — Backend cross-cut (full Go suite + server-only smoke)

**Objective.** Prove the backend model swap holds together as a whole before the UI
consumes it. (The in-app regression re-runs move to PASS-0009 alongside the new
`WORKTREES`-tab cases.)

**Changes / proof**
- Full `go test ./...` under [`backend/orchestration`](../../backend/orchestration/) green.
- **Server-only smoke** (supporting proof, not regression): start the backend on the
  isolated **validation lane** (`http://127.0.0.1:14318`, per [TESTING.md](../../TESTING.md))
  against a throwaway registry; exercise Create → list → Assign → list → Eject → list →
  Destroy → list (and Dequeue) with PowerShell/`curl`; assert JSON shapes, that the
  **Ejected folder persists** on disk, and that the **Destroyed** one is gone. Capture
  under [`Testing/`](./Testing/). This also confirms the exact JSON the UI client
  (`worktrees_backend.py`) will consume in PASS-0007/0008.

**Falsifier guard** — any backend AC (1–15) still failing here blocks the UI passes,
because the UI cannot work in-app against broken endpoints.

---

## PASS-0007 — Frontend foundation: rename + replace + read-only pool view

**Objective (TASK.md §12, Goals 12–15; AC16–AC19; REG-010/011/012).** The desktop
`TASKS` tab is renamed to `WORKTREES` and its content replaced (D1=a) with the read-only
worktree-management surface: the full-pool view (allocated/idle color + repo + path + a
stable id), the copy-path control, and the registry-sourced repo filter. No mutating
controls yet (those are PASS-0008).

**Changes**
- **Rename + dispatch:** in [`ui.py`](../../app/codex_dashboard/ui.py), change the nav
  tuple entry at [L671](../../app/codex_dashboard/ui.py#L671) `("tasks", "Tasks")` →
  `("worktrees", "Worktrees")` (visible label `WORKTREES`), and rewire `select_tab`
  ([L1152](../../app/codex_dashboard/ui.py#L1152)) / `_render_active_tab`
  ([L1164](../../app/codex_dashboard/ui.py#L1164)) so the renamed tab builds the worktree
  surface. **Remove** the old task stream/detail/dispatch-pause-poke widgets + render
  methods from this tab (D1=replace). Tab switch performs no backend write. `Usage` and
  `Jobs` untouched.
- **HTTP client:** new [`worktrees_backend.py`](../../app/codex_dashboard/worktrees_backend.py)
  mirroring [`tasks_backend.py`](../../app/codex_dashboard/tasks_backend.py): URL config +
  env override, `fetch_pool_snapshot()` (GET `/api/v1/worktrees`), `fetch_repos()`
  (GET `/api/v1/repos`), the same `urllib` `_request_json`, and the backend-unavailable
  error-snapshot shape (mirroring
  [`tasks_backend_error_snapshot`](../../app/codex_dashboard/tasks_backend.py#L58)).
- **Pure helpers:** new [`worktrees_tab.py`](../../app/codex_dashboard/worktrees_tab.py)
  mirroring [`tasks_tab.py`](../../app/codex_dashboard/tasks_tab.py): group/sort
  worktrees, allocated-vs-idle **color selection**, per-row detail formatting.
- **Tk widgets:** the pool list (one row per worktree: repo, local dir, id, allocated
  task/run/status), reusing the existing `_configure_styles` palette; an allocated-row
  background style distinct from idle; a per-row **copy-path** control writing
  `worktree_path` to the clipboard; the registry-sourced **repo filter** dropdown
  (options from `fetch_repos()` + "All repos") that filters the rendered list.

**Proof**
- **Unit:** `tests/test_worktrees_tab.py` covers the pure helpers (grouping, color
  selection, formatting) and `worktrees_backend.py` mapping + error-snapshot against
  backend-shaped fixtures. `python -m unittest discover -s tests -p "test_*.py"` green.
- **In-app (the bar):** REG-010 (pool view + allocated/idle color + rename/replace +
  read-only switch + backend-unavailable message), REG-011 (copy-path), REG-012
  (registry-sourced repo filter) run in the running app on the validation lane against
  the live backend. Capture app-surface artifacts under [`Testing/`](./Testing/).

**Falsifier guards** ([TASK.md What Does Not Count]) — a hardcoded repo filter fails
(AC19); allocated/idle with no visible color distinction fails (AC17); a copy control
copying the wrong/no path fails (AC18); leaving the old `TASKS` content or not renaming
fails (AC16).

---

## PASS-0008 — Frontend mutating controls (Create / Assign / Eject / Destroy / Dequeue)

**Objective (TASK.md §12, Goals 16–18; AC20–AC22; REG-013/014/015/016).** Wire the tab's
action controls to the [BE] endpoints so the operator can drive the full lifecycle in-app.

**Changes**
- **`worktrees_backend.py` POST helpers:** `create_worktree(repo)`,
  `assign_worktree(task_id, repo, worktree_id)`, `eject_worktree(run_id)`,
  `destroy_worktree(worktree_id)`, `dequeue_task(repo, task_id)` — each a method-correct
  POST mirroring the `tasks_backend` dispatch helpers, surfacing backend errors (incl. the
  409 allocated-Destroy rejection) as human-facing messages.
- **Create control** → `create_worktree` for the selected repo → refresh the pool view.
- **Assign popup** → lists open tasks via the existing
  [`tasks_backend.fetch_tasks_snapshot`](../../app/codex_dashboard/tasks_backend.py#L29)
  (`GET /api/v1/tasks`) as id + title + state (no progress bars / file-ref lines — E6);
  confirm → `assign_worktree` → the worktree flips allocated. The open-tasks projection is
  a pure helper in `worktrees_tab.py` (unit-tested).
- **Eject control** (allocated rows; keyed on `run_id`) → `eject_worktree` → the worktree
  returns to idle in the view; the freed task is dequeued (reflected by the refresh).
- **Destroy control** (idle rows) → `destroy_worktree` → the row disappears; on an
  allocated worktree the backend's 409 is surfaced as a clear message and nothing is
  removed.
- **Dequeue control** → `dequeue_task` without ejecting (worktree stays allocated, issue
  stays open).

**Proof**
- **Unit:** extend `tests/test_worktrees_tab.py` for the Assign-popup open-task projection
  and the error-message mapping (incl. the allocated-Destroy rejection). Python unit suite
  green.
- **In-app (the bar):** REG-013 (Create), REG-014 (Assign popup → bind), REG-015 (Eject
  returns idle + dequeues), REG-016 (Destroy idle-only + allocated rejection + standalone
  Dequeue) run in the running app on the validation lane against the live backend. Capture
  app-surface artifacts under [`Testing/`](./Testing/).

**Falsifier guards** — controls that render but do not call the real endpoints fail; an
Assign popup with an empty/hardcoded list or that does not bind fails (AC21); a Destroy
that deletes an allocated worktree fails (AC22); a Dequeue that closes the issue fails
(AC22 / AC14).

---

## PASS-0009 — Cross-cutting closure (in-app regression run)

**Objective.** Prove the whole chunk — backend model swap + the working `WORKTREES` tab —
holds together in-app, and the named regressions pass under the new model. **This pass
carries the "done = working human surface" closure bar.**

**Changes / proof**
- Confirm full `go test ./...` ([`backend/orchestration`](../../backend/orchestration/))
  and the Python unit suite (`python -m unittest discover -s tests -p "test_*.py"`) green.
- **New in-app `WORKTREES`-tab regression run** — execute **REG-010 … REG-016** in the
  running desktop app on the isolated **validation lane** (`CODEX_DASHBOARD_*_BACKEND_URL`
  → `http://127.0.0.1:14318`, task-owned config + isolated SQLite per
  [TESTING.md](../../TESTING.md)) against the live backend, pool seeded via Create. Capture
  an app-surface artifact per case.
- **REG-007 / REG-008 / REG-009 in-app re-run** on the isolated `reg007` lane against the
  throwaway `QueueDrainTestbed`/`QueueDrainTestbed2` repos, **pool seeded via Create**
  before the drain can dispatch:
  - **REG-007** "cap=1" → "**pool of 1**": one idle worktree ⇒ consumer dispatches exactly
    one Ready issue, the second **waits for an idle worktree**; close/eject frees it for
    reuse. The Ready flip is driven at the **real GitHub web UI** via the `github-operator`
    Chrome debug session (the REG-007 surface rule — see [TESTING.md](../../TESTING.md)
    "Issue-Provider Integration Testing"); the product `Queue=Never` dequeue write is a
    distinct backend write against the throwaway testbed and does **not** conflict with
    that rule (recorded in [TASK.md Proof Plan](./TASK.md)).
  - **REG-008** durable state survives a backend restart; the pool's allocated-vs-idle
    classification is reconstructed by **discover-on-startup** (PASS-0001).
  - **REG-009** each repo's pool is independent: Create/Assign/Eject/Destroy/close on repo
    A never touches repo B's pool.
- Capture proof under [`Testing/`](./Testing/) as `REGRESSION-RUN-<NNNN>.md` artifacts,
  each naming the claimed lane/case, the flow exercised, why it counts, and disqualifiers.

**Regression caveat (record, do not call a new bug yet).** [REG-008 Status](../../REGRESSION.md)
records a known non-fatal harness latency: the **first** consumer poll after a restart can
exceed the 2-minute poll `StartToClose` timeout and self-corrects next tick. If observed,
follow [REGRESSION.md](../../REGRESSION.md) and treat it as the documented latency item, not
a new product bug, before opening any `BUG-<NNNN>.md`.

**Frontend-release caveat (record).** Per [REGRESSION.md](../../REGRESSION.md) REG-006
interpretation + [TESTING.md](../../TESTING.md), the human's pinned dashboard release does
not change from source edits alone; the in-app regression here runs the validation-lane
app surface, and any human-lane rollout is a separate human-gated publish + restart
(`scripts/Publish-DashboardRelease.ps1`), not part of this task's closure proof.

---

## Acceptance-criteria → pass map

| AC (TASK.md) | Pass | Proof |
| --- | --- | --- |
| 1 queue_workers removed (admission from pool) | PASS-0003 | `consumer_test.go` pool-of-1; grep proof |
| 2 Create at stable path | PASS-0002 (+PASS-0000 record) | `service_test.go` |
| 3 Assign reuses idle (no fresh dir, pool count flat) | PASS-0003 | `service_test.go` |
| 4 Assign rejects when none idle (409) | PASS-0003 | `service_test.go` |
| 5 Auto-assign draws from pool / defers empty | PASS-0003 | consumer+service seam |
| 6 Eject keeps folder + idle + dequeue (running & parked) | PASS-0005 | `service_test.go` + fake provider |
| 7 Destroy rejects allocated; removes idle | PASS-0002 | `service_test.go` |
| 8 Discover across restart | PASS-0001 | `service_test.go` restart sim |
| 9 Full-pool read | PASS-0002 | handler/service test |
| 10 Repos read without queue_workers | PASS-0002 | `mux_test.go`/service |
| 11 Method/path guards | PASS-0002 | `mux_test.go` |
| 12 Provider dequeue sets not-ready (via provider) | PASS-0004 | fake provider |
| 13 Eject does not re-dispatch (no bounce-back) | PASS-0005 | consumer+service seam |
| 14 Dequeue leaves issue open (CloseIssue never called) | PASS-0004/0005 | fake provider |
| 15 Standalone dequeue endpoint | PASS-0004 | `mux_test.go` + service |
| 16 [FE] Tab renamed + replaced (read-only switch) | PASS-0007 | REG-010 in-app |
| 17 [FE] Full-pool view + allocated/idle color | PASS-0007 | REG-010 in-app + `test_worktrees_tab.py` |
| 18 [FE] Copy-path control | PASS-0007 | REG-011 in-app |
| 19 [FE] Registry-sourced repo filter | PASS-0007 | REG-012 in-app |
| 20 [FE] Create from the UI | PASS-0008 | REG-013 in-app |
| 21 [FE] Assign popup binds an open task | PASS-0008 | REG-014 in-app + popup-projection unit |
| 22 [FE] Eject / Destroy / Dequeue from the UI | PASS-0008 | REG-015 + REG-016 in-app |
| REG-010…REG-016 new in-app cases pass | PASS-0007/0008 build; PASS-0009 run | in-app under `Testing/` |
| REG-007/008/009 green under new model | PASS-0009 | in-app re-run under `Testing/` |

## Risks / caveats carried into implementation

- **Stable-path migration of existing dispatched lanes.** Today's lanes live at
  `os.MkdirTemp` random dirs. The new pool uses stable `wt-<NNNN>` paths. Plan: the new
  model applies to pool members created via Create / drawn by pool-draw; discover
  classifies whatever pool folders exist. Pre-existing random-temp lanes from the old
  model are out of the pool layout; the implementer must confirm discover does not
  mis-handle them (treat a non-pool-layout folder as not-a-pool-member). This is an
  implementation detail to pin in PASS-0001, not a scope change.
- **`-fdx` on Eject vs `-fd` on Assign reset.** Eject uses `git clean -fdx` (true
  baseline, drops ignored files) per §6; Assign reuses `restoreOwnedLane` (`-fd`). The
  implementer must confirm `restoreOwnedLane` is parameterized or a sibling is used so
  Eject gets `-fdx` without weakening Assign's reset.
- **No clean-context QA lane / nested implementers from this runtime.** Flagged to the
  coordinator (HANDOFF "Process gaps"); QA must be arranged by the coordinator as a
  QA-designated clean-context lane, not producer self-review. This applies especially to
  the UI passes (PASS-0007/0008) where a clean-context QA verdict on the in-app surface is
  required, not producer self-review.
- **REG re-runs need the human-authenticated Chrome debug session** for the REG-007 Ready
  flip (the human authenticates once; the agent drives the UI). This is a human
  prerequisite for PASS-0009's in-app REG-007 re-run, not a code blocker.
- **[FE] In-app proof requires a runnable desktop surface.** The new
  `WORKTREES`-tab cases (REG-010…REG-016) must be exercised in the **running app** on the
  validation lane (Tk overlay capture / app-surface artifact), not by unit tests or
  endpoint smoke alone. If the environment cannot drive the real Tk surface, that is a
  blocker to surface to the coordinator, not a reason to downgrade the closure bar to
  endpoint/unit proof.
- **[FE] D1=replace removes existing `TASKS`-tab behavior.** Ripping out the old task
  stream/detail/dispatch-pause-poke widgets changes what REG-003/REG-004 (the old `Tasks`
  surface cases) describe. The implementer must confirm with the coordinator how those
  older cases are retired/superseded (the task lifecycle now lives on the GitHub Issues
  queue surface) so REGRESSION.md stays internally consistent — record it, do not silently
  leave stale cases pointing at a removed tab.
- **[FE] Allocated/idle color must be a real, perceivable distinction**, reusing the
  existing palette — not a near-invisible tint. Confirm against the mockup's intent
  (distinct background for allocated) during the UI pass.

## What approval authorizes

Approving this plan authorizes implementation of **PASS-0000 → PASS-0009 as scoped above**
— the Go backend manual worktree-pool lifecycle **and** the desktop `WORKTREES`-tab UI that
consumes it — with all proof on the isolated validation (`http://127.0.0.1:14318`) /
`reg007` lanes, task-owned config + isolated SQLite, and throwaway testbed repos only. It
does **not** authorize: any work against the human's production repo, service lane, live
Codex data, or human dashboard config/database; publishing/rolling out a new pinned
dashboard release to the human lane (a separate human-gated publish + restart); or closing
GitHub issue #16 (closure is a separate human gate — the agent never self-closes). "Done"
is gated on the **working in-app `WORKTREES` surface** plus the named in-app cases
(REG-010…REG-016, and REG-007/008/009 green under the new model) passing.
