# CodexDashboard Regression Checklist

This file is the canonical regression matrix for CodexDashboard.

## Canonical Rule

Task-level regression in this repo starts from the real desktop app surface.

Supporting parser or CLI proof is useful, but it does not replace the required app-surface lane for closure.

Regression must not use the human's personal dashboard lane, config, database, or live Codex data unless the human explicitly authorizes that specific run.
The default or persistent backend lane is not automatically the human lane, but task-closure regression still must run on a separate isolated validation or task-specific regression lane.
Use an isolated lane with task-owned or temp data, task-owned config, and task-owned SQLite persistence.
The current default agent validation lane is documented in [TESTING.md](./TESTING.md):

- backend URL: `http://127.0.0.1:14318`
- Temporal: `127.0.0.1:17233`
- Postgres: `15432`
- runtime root: `%LOCALAPPDATA%\CodexDashboard\orchestration-validation-lane`

If a task creates another regression lane, its ports, data roots, start/stop commands, and cleanup expectations must be documented in [TESTING.md](./TESTING.md) before that lane is used as closure evidence.

When current human-facing functionality changes, this repo-local `REGRESSION.md` must be updated so the changed interaction is covered by a named case. Do not treat a new clickable surface as covered by an older case unless the written steps and expected result explicitly include that interaction.

## Default Regression Lane

### REG-001 Desktop Overlay Launch And Data Smoke

Goal:

Confirm the real desktop app can ingest Codex telemetry, open the real overlay, and render interval data plus budget state.

Steps:

1. Launch the real app from repo root.
2. Point it at a task-owned fixture tree with real-shaped `token_count` events. Do not use `C:\Users\gregs\.codex` unless the human explicitly authorizes that run.
3. Allow at least one ingest cycle to complete.
4. Trigger the real overlay path.
5. Capture an artifact from the running app surface that shows:
   - the selected interval
   - bar data
   - weekly budget state
6. Exit cleanly.

Expected result:

- the app starts without crashing
- the ingest loop persists real token events
- the overlay becomes visible
- interval data appears
- budget and redline state appear

## Required Current Cases

### REG-002 Jobs Tab Interaction And Status Surface

Goal:

Confirm the real desktop app can switch from the default `Usage` surface to the backend-backed `Jobs` surface, show desired-vs-runtime job state from the orchestration backend, and invoke the bounded `Sync now` control without disruptive side effects.

Steps:

1. Launch the real app from repo root.
2. Point it at a task-owned fixture tree with real-shaped `token_count` events. Do not use `C:\Users\gregs\.codex` unless the human explicitly authorizes that run.
3. Start the separate validation-lane orchestration backend and keep it reachable for the app without disturbing the always-on service lane.
4. Set `CODEX_DASHBOARD_JOBS_BACKEND_URL=http://127.0.0.1:14318` for the app process that will be used for the regression run.
5. Allow at least one ingest cycle to complete.
6. Trigger the real overlay path.
7. Click the `Jobs` tab in the running overlay.
8. Verify the tab switch completes immediately and does not trigger `Sync now` work on its own.
9. Verify the `Jobs` surface shows backend-derived job rows with trigger labels, desired/runtime state, and drift status.
10. Open `Details` for one job and verify it shows the backend job payload, including trigger and executor data.
11. Verify `Run now` is visible for manual-capable jobs and disabled for jobs without a manual trigger.
12. Click `Sync now` in the `Jobs` surface and observe the backend-backed state reload to completion.
13. Capture an artifact from the running app surface that shows:
   - the `Jobs` tab as the active surface
   - job summary counts
   - per-job status rows
14. Exit cleanly.

Expected result:

- the tab switch completes without hitching the overlay
- no extra console or app windows are spawned by the interaction
- clicking `Jobs` alone does not trigger `Sync now`
- the `Jobs` surface renders backend-derived jobs, in-sync count, needs-attention count, and per-job rows
- `Details` shows the backend job payload rather than a local Windows task snapshot
- `Sync now` completes through the backend without crashing or spawning stray windows
- `Run now` availability matches whether a job actually supports manual triggering
- visible copy remains human-facing
- the validation lane can be exercised without taking down the default service lane

### REG-003 Tasks Tab Committed-Work Surface

**SUPERSEDED (Task-0016 D1=replace).** The committed-work `Tasks` surface this case
describes **no longer exists**: Task-0016 renamed the `TASKS` tab to `WORKTREES` and
replaced its content (the task stream / detail / dispatch-pause-poke widgets were
removed). The task lifecycle now lives on the **GitHub Issues queue surface** (the
queue-drain consumer), not a dashboard `Tasks` tab. The new `WORKTREES` tab is covered
by **REG-010…REG-016** below. This case is retained for history; it is **not** a current
closure gate and must not be run against the removed `Tasks` tab. (See
[Tracking/Task-0016/HUMAN-DIRECTIVES-FOR-WORKER.md](./Tracking/Task-0016/HUMAN-DIRECTIVES-FOR-WORKER.md)
UPDATE 4.)

Goal:

Confirm the real desktop app can switch from the default `Usage` surface to the committed-work `Tasks` surface, render backend-shaped task readback from an isolated lane or task-owned fixture, and communicate task state without exposing unpromoted candidates or false progress bars.

Steps:

1. Launch the real app from repo root.
2. Point it at a task-owned fixture tree with real-shaped `token_count` events. Do not use `C:\Users\gregs\.codex` unless the human explicitly authorizes that run.
3. Use an isolated config and isolated SQLite database as documented in [TESTING.md](./TESTING.md).
4. Point task readback at the validation lane URL `http://127.0.0.1:14318` or a task-owned backend-shaped snapshot fixture through `CODEX_DASHBOARD_TASKS_SNAPSHOT_PATH`.
5. Trigger the real overlay path.
6. Click the `Tasks` tab in the running overlay.
7. Verify the tab switch completes immediately and does not dispatch, pause, poke, resume, or otherwise change backend run state by switching tabs.
8. Verify the `Tasks` surface shows:
   - `Needs you`
   - `Sleeping`
   - `Running`
   - `Blocked`
   - `Ready`
9. Verify committed-task rows show title, state, reason, freshness, source provenance, and a selected-task detail pane.
10. Verify unpromoted candidates are not displayed as normal tasks.
11. Verify committed promoted work uses provenance such as `Promoted from Dream` or `Promoted from Review`, not `Candidate` or `Prov: Candidate`.
12. Verify visible stop/hold copy says `Pause` rather than backend-internal `Interrupt`.
13. Verify the surface does not display AI-run progress bars.
14. Capture an artifact from the running app surface that shows:
   - the `Tasks` tab as the active surface
   - committed-work summary counts
   - grouped task rows
   - the selected-task detail pane
15. Exit cleanly.

Expected result:

- the tab switch completes without hitching the overlay
- no extra console or app windows are spawned by the interaction
- switching to `Tasks` is read-only unless the human explicitly clicks a bounded action
- summary counts and rows render from isolated backend-shaped task readback
- candidate/intake work is absent unless it has been promoted into committed work
- promoted committed-work provenance is explicit and does not use candidate labels
- visible control language uses `Pause`, not `Interrupt`
- no progress bar implies false precision for AI task-run progress
- the validation lane or fixture can be exercised without using the persistent service lane, the human's dashboard config, the human's active database, or live Codex data

### REG-004 Semantic Dashboard State Reconciliation

**SUPERSEDED (Task-0016 D1=replace).** This case reconciles the **`Tasks` surface**
(visible task rows, summary counts, completed/human-wait/run-control semantics) that
Task-0016 **removed** — the `TASKS` tab is now `WORKTREES` and its task-stream content is
gone (the task lifecycle lives on the **GitHub Issues queue surface**). The semantic
reconciliation for the new `WORKTREES` tab (allocated/idle state, repo, path, bound
task/run/gate, and each mutating interaction) is covered by **REG-010…REG-016** below.
This case is retained for history; it is **not** a current closure gate and must not be
run against the removed `Tasks` surface. (See
[Tracking/Task-0016/HUMAN-DIRECTIVES-FOR-WORKER.md](./Tracking/Task-0016/HUMAN-DIRECTIVES-FOR-WORKER.md)
UPDATE 4.)

Goal:

Confirm the real dashboard window makes true claims. This case is not satisfied
by proving that the app launched, the backend responded, or a screenshot exists.
Every visible dashboard claim under test must reconcile against the authoritative
durable source for that claim.

Steps:

1. Launch the real dashboard app surface under the pinned release or an isolated
   validation build whose release/process identity is recorded.
2. Use an isolated app config and SQLite database unless the human explicitly
   authorizes inspecting the human lane.
3. Point backend-backed surfaces at the lane under test and record whether that
   lane is `service`, `validation`, or task-specific.
4. Open the actual dashboard window and inspect the visible `Tasks` surface.
5. Build a reconciliation matrix with one row per visible claim:
   - visible claim
   - authoritative source
   - expected value
   - actual visible value
   - pass/fail
   - linked bug if failed
6. Reconcile task inventory:
   - every visible task row against `Tracking/Task-*/TASK-STATE.json` and
     backend `/api/v1/tasks`
   - summary counts against visible rows and backend-classified groups
   - selected-task detail labels against the selected task's durable state
7. Reconcile completed-task semantics:
   - `status: complete` and `phase: closure` must not appear as `Waiting on you`,
     `Needs you`, `Ready`, or dispatchable
   - completed work must be absent from active-work buckets or shown only in an
     explicit completed/history treatment
8. Reconcile human-wait semantics:
   - `Waiting on you` or `Needs you` must only appear when durable truth assigns
     the next action to the human, such as plan approval or an active run waiting
     for human input
   - backend-review or unmapped/internal states must not be translated into
     human-owned waiting copy
9. Reconcile Temporal/task-run state:
   - `Running`, `Paused`, `Sleeping`, `Poke`, `Pause`, and `Resume`/`Continue`
     must match backend run state and action availability
   - no active run means run-control actions are absent or disabled with a true
     reason
10. Reconcile provenance and action copy:
    - authored/promoted labels match durable provenance
    - unpromoted candidates are absent
    - no visible `Candidate`, `Prov: Candidate`, or backend-internal `Interrupt`
      copy appears on committed-work task rows
11. Capture a screenshot as supporting evidence, but do not treat the screenshot
    as the proof. The reconciliation matrix is the proof.
12. For every divergence, open or update a task-owned `BUG-<NNNN>.md` before
    calling the regression run complete.

Expected result:

- visible task counts equal the reconciled durable/backend state
- completed tasks are not presented as waiting on the human
- human-wait copy appears only for real human-owned waits
- run-control actions match Temporal/backend action availability
- provenance and action labels match the committed-work product contract
- all divergences have bug records with evidence, root-cause hypothesis, and
  required closure proof

Interpretation:

- this is the canonical semantic dashboard regression for the `Tasks` surface
- release/process proof can identify which build was inspected, but it does not
  replace semantic reconciliation
- API-only, parser-only, and screenshot-only checks do not satisfy this case

### REG-005 Usage Source Filter (Codex/Claude)

Goal:

Confirm the real desktop overlay exposes the Task-0013 source filter (Codex and
Claude checkboxes) over the displayed token aggregation, that toggling a source
includes/excludes its events from the totals and chart, and that applying the
filter does not block the Tk UI thread on a synchronous database read.

Steps:

1. Launch the real app from repo root on an isolated lane.
2. Point it at a task-owned fixture tree containing BOTH real-shaped Codex
   `token_count` events and real-shaped Claude `projects/.../*.jsonl`
   (`type:"assistant"`, `message.usage`, repeated `requestId`s). Do not use
   `C:\Users\gregs\.codex` or `~/.claude` unless the human explicitly authorizes
   that run.
3. Use an isolated config (`claude_root` pointed at the fixture) and an isolated
   SQLite database as documented in [TESTING.md](./TESTING.md).
4. Allow at least one ingest cycle to complete so both sources land in
   `token_events` with their `source` discriminator.
5. Trigger the real overlay path.
6. Open the source filter control in the running overlay and confirm it shows
   separate Codex and Claude checkboxes, styled consistently with the overlay.
7. With both checked, record the displayed 7d total / chart (merged value).
8. Uncheck Claude and confirm the total/chart drop to the Codex-only value
   without the overlay hitching or spawning extra windows.
9. Uncheck Codex (Claude only) and confirm the total/chart show the Claude-only
   value.
10. Uncheck both and confirm a clear empty/zero state with no crash.
11. Re-check both and confirm the merged value returns.
12. Capture an artifact from the running app surface showing the filter control
    and the before/after of toggling a source.
13. Exit cleanly.

Expected result:

- the overlay shows a visible source filter with Codex and Claude checkboxes
- toggling a source updates totals, projections, and chart immediately
- the displayed value for a selected subset equals the sum of only the selected
  source(s)
- unchecking both shows an empty/zero state and does not crash
- applying the filter does not trigger a synchronous SQLite read on the Tk UI
  thread (it filters the already-loaded in-memory snapshot and re-renders)
- no extra console or app windows are spawned by the interaction

Interpretation:

- this is the canonical regression for the Task-0013 Objective 4 source filter
- unit coverage of the source-filtered aggregation lives in
  `tests/test_task0013_obsidian.py`; that unit proof supports but does not
  replace this app-surface case

### REG-006 Tab-Aware Overlay Resize And Reposition

Goal:

Confirm the real desktop overlay repositions toward the top of the monitor on
every tab and grows to fill the usable height (the work area, taskbar excluded) on
the `Jobs` and `Tasks` tabs, with a configurable top/bottom padding, while never
covering the Windows taskbar, never widening past 980px, and never resizing the
`Usage` tab.

Steps:

1. Launch the real app from repo root on an isolated lane.
2. Point it at a task-owned fixture tree with real-shaped `token_count` events. Do
   not use `C:\Users\gregs\.codex` unless the human explicitly authorizes that run.
3. Use an isolated config and isolated SQLite database as documented in
   [TESTING.md](./TESTING.md). Optionally set a non-default `pad_fraction` in the
   task-owned config to verify configurability.
4. Trigger the real overlay path on the default `Usage` tab.
5. Observe the overlay opens near the top of the monitor (its top edge one `pad`
   below the top of the usable work area) at the current `980x660` size.
6. Click the `Tasks` tab, then the `Jobs` tab.
7. Observe that on each of `Jobs` and `Tasks` the same window grows tall — filling
   the usable height minus the top/bottom padding — at width 980, right-aligned, so
   many more rows are visible at once.
8. Observe a visible gap remains above the Windows taskbar on every tab.
9. Click back to `Usage` and observe the window returns to `980x660` at the same
   top position.
10. Verify the tab switches are immediate and do not trigger a data refresh,
    re-aggregation, or full re-render (the Task-0013 cheap show/hide behavior is
    preserved).
11. Capture an artifact from the running app surface (for example via
    `write_overlay_capture`) showing the tall `Jobs`/`Tasks` layout and the gap
    above the taskbar.
12. Exit cleanly.

Expected result:

- on every tab the overlay top edge sits one `pad` below the usable work-area top
- the `Jobs` and `Tasks` tabs fill the usable height minus top/bottom padding at
  width 980, right-aligned
- a positive `pad` gap always remains above the taskbar; the taskbar is never
  covered on any tab
- the `Usage` tab keeps its `980x660` size and is only repositioned
- a different `pad_fraction` visibly changes how far the window sits from the
  top/bottom edges after a restart
- switching tabs does not rebuild or refetch tab data and does not spawn extra
  windows

Interpretation:

- this is the canonical regression for the Task-0014 tab-aware overlay
  resize/reposition interaction
- unit coverage of the pure geometry math lives in
  `tests/test_overlay_geometry.py`; that unit proof supports but does not replace
  this app-surface case
- source edits plus passing unit tests do not change the live overlay until a new
  pinned release is published and the overlay restarted (see [TESTING.md](./TESTING.md)
  and `scripts/Publish-DashboardRelease.ps1`); that publish + restart is a separate
  human-gated step

### REG-007 Queue-Drain Consumer Dispatch From The GitHub Web Surface

Goal:

Confirm the always-on Temporal queue-drain consumer dispatches the mapped Task
when an issue's `Queue` field is flipped to `Ready` **at the real GitHub web
surface**. This case is not satisfied by flipping the field through the API or a
proxy; the flip under test must be performed by driving the GitHub web UI through
the Chrome debug session, because the issue PROVIDER is the human surface being
tested.

Surface:

The GitHub web interface (issue Fields panel) driven via the Chrome debug session
using the `github-operator` skill, plus the orchestration backend's queue-drain
consumer on the isolated validation / `reg007` lane.

Steps:

1. Run all queue-drain proof on the isolated validation / `reg007` lane against
   the throwaway `QueueDrainTestbed` repo. Never use the human's production
   repo (`Digital-Collective-Games/Obsidian`), the live `CodexDashboard` repo, or
   the real `default` Temporal namespace where the operator's scheduled cron jobs
   live. Run the consumer in the isolated `reg007` Temporal namespace only.
2. Ensure the throwaway test issue already has an issue **type** and an initial
   `Queue` field **value** so the org field renders in the UI (set those
   separately; see [TESTING.md](./TESTING.md) and the obsidian-operator sync
   scripts).
3. Start the always-on queue-drain consumer on the `reg007` lane with a poll
   interval `<= 60s` (use ~30s), pointed at the `QueueDrainTestbed` repo, and
   confirm it is running and reachable without disturbing the always-on service
   lane or activating any real scheduled cron job.
4. The **human authenticates** the debug Chrome profile
   (`C:\Agent\Orchestrator\Scripts\Start-ChromeDebugProfile.ps1`, port `9222`) and
   logs into GitHub; the human does not click through the test.
5. Record the starting `Queue` state of the test issue via
   `skills/github-operator/scripts/Get-IssueQueueState.ps1`.
6. The **agent flips** the issue's `Queue` field to `Ready` by driving the GitHub
   web UI with `skills/github-operator/scripts/Set-IssueFieldViaUi.ps1`
   (`-FieldName Queue -OptionName Ready`), and confirms the surface committed the
   value (observed control text reads `QueueReady`; optionally re-read via
   `-VerifyApi`).
7. Observe that the always-on consumer notices the `Queue=Ready` flip within
   `<= 1 minute` and dispatches the mapped Task into an owned worktree.
8. Observe that the dispatch launches the top-level `claude` agent in that
   worktree (not `codex`), and that the launched agent is discoverable (its
   session id and transcript resolve under `~/.claude/projects/<slug>/`).
9. Capture artifacts from the run: the GitHub UI flip result (committed control
   text), the consumer log/`GET` showing the within-1-minute dispatch, the
   created worktree, and the launched claude session id/transcript path.
10. Reset the test issue (`Queue` back to its starting value or `Never`) and tear
    down the worktree/lane so the proof can be re-run arbitrarily.

Expected result:

- the `Queue=Ready` flip is performed at the real GitHub web UI via the Chrome
  debug session, and the surface commits the value (`QueueReady`)
- the always-on consumer notices the flip within `<= 1 minute`
- the consumer dispatches the mapped Task into an owned worktree
- the dispatch launches a top-level `claude` agent in that worktree, discoverable
  by session id and transcript
- the run is performed on the isolated validation / `reg007` lane and the
  `QueueDrainTestbed` repo, never the human's production repo/lane and never the
  real `default` Temporal namespace
- no real scheduled cron job is triggered by the run

Disqualifiers:

- flipping `Queue` through the API, a proxy, or any path other than the real
  GitHub web UI via the Chrome debug session does NOT satisfy this case
- a consumer that dispatches but takes longer than 1 minute to notice the flip
- dispatching `codex` instead of a top-level `claude` agent
- running against the production repo, the live `CodexDashboard` repo, or the
  real `default` Temporal namespace

Interpretation:

- this is the canonical end-to-end regression for the Task-0015 queue-drain
  consumer measured FROM the GitHub web surface
- API-only or proxy proof of the flip is a Disqualifier, not supporting evidence
- the human authenticates the debug Chrome once; the agent drives the UI
  end-to-end via the `github-operator` skill
  ([skills/github-operator/SKILL.md](./skills/github-operator/SKILL.md))

Concurrency + worktree-lifecycle sub-scenarios (required):

- **Concurrency / queuing:** with `queue_workers = N`, flip MORE than N issues to
  `Queue=Ready` (via the real UI) and confirm the consumer dispatches EXACTLY N
  concurrently and DEFERS the rest (no over-dispatch); a deferred issue gets no
  worktree.
- **Release on close + dequeue:** close one dispatched issue (the human-approved
  `gh issue close` / obsidian-operator close path) and confirm the consumer
  RECLAIMS its worktree, frees the slot, and DEQUEUES a deferred Ready issue. A
  dispatched run holds its worktree until that close (the agent never self-closes).
- **Park retains:** set a dispatched issue `Human Needed=Yes` and confirm the
  consumer PARKS it (`run_gate_state = parked_*`): the worktree is RETAINED, the
  slot stays used, and it is NOT redispatched. Only a CLOSED issue deallocates.

Proof of all three (live, registry-driven, `queue_workers=2`):
[Tracking/Task-0015/Testing/PASS-0006/REG-007-CONCURRENCY-RELEASE-PROOF.md](./Tracking/Task-0015/Testing/PASS-0006/REG-007-CONCURRENCY-RELEASE-PROOF.md).

- **Full deallocate/reuse cycle (required):** with `queue_workers=N`, allocate ALL N
  slots, let the dispatched agents complete and ANNOUNCE done, simulate human closure
  approval, and confirm EVERY worktree DEALLOCATES (slots freed); then queue another
  task and confirm it REUSES a freed slot. The deallocate must actually free the
  worktree (a closed issue whose worktree is only pruned by hand does NOT satisfy this).
  To simulate the human closure approval in-test ONLY, set
  `OBSIDIAN_AUTO_CLOSE_QUEUED=true` (default OFF in production; the agent announces done
  by setting its `TASK-STATE.json current_gate="closure"` and never closes the issue
  itself). Proof: [Tracking/Task-0015/Testing/PASS-0008/REG-007-FULL-CYCLE-PROOF.md](./Tracking/Task-0015/Testing/PASS-0008/REG-007-FULL-CYCLE-PROOF.md)
  (resolves [BUG-0002](./Tracking/Task-0015/BUG-0002.md): reclaim now terminates the
  launched agent + is idempotent/self-healing, so a close ALWAYS deallocates).

- **Cap=1 serialization on a separate 1-slot repo (required):** register a SECOND repo with
  `queue_workers=1`, flip TWO of its issues to `Queue=Ready` at the real UI at ~the same
  time (auto-close enabled), and confirm the consumer dispatches EXACTLY ONE (one worktree,
  never two) and DEFERS the other with NO worktree; the first finishes → auto-close →
  DEALLOCATE, then the second is dispatched INTO the freed slot (slot REUSE, not a second
  concurrent slot) and finishes → deallocate → 0. Run on the isolated `reg007` lane against
  the throwaway `QueueDrainTestbed2` repo; never the production repo or the real `default`
  namespace. Proof:
  [Tracking/Task-0015/Testing/PASS-0009/REG-007-CAP1-SERIALIZATION-PROOF.md](./Tracking/Task-0015/Testing/PASS-0009/REG-007-CAP1-SERIALIZATION-PROOF.md).
  Note: a first attempt on a SHARED runs-root + multi-repo registry wedged (worktree torn
  down under a running agent → re-dispatch loop); it did not reproduce after isolating to a
  dedicated runs-root + single-repo registry. That candidate follow-up — a GLOBAL,
  non-repo-scoped runs-root scan letting one repo clobber another's lane — WAS the
  [BUG-0003](./Tracking/Task-0015/BUG-0003.md) root cause, fixed (repo-namespaced run ids +
  repo-scoped accounting) and re-validated by REG-009 below.
  **Re-run on the Landing-2 code (PASS):**
  [Tracking/Task-0015/Testing/PASS-0012/REG-007-008-009-LANDING2-LIVE.md](./Tracking/Task-0015/Testing/PASS-0012/REG-007-008-009-LANDING2-LIVE.md)
  — UI Ready-flip → dispatch → launched agent ran; `wt<=1` throughout; autoclose → reclaim →
  slot reuse → 0, all on a multi-repo registry with the BUG-0003 fix in place.

- **Pool-of-1 reinterpretation under the manual worktree-pool model (Task-0016):** once the
  manual worktree-pool model lands (Task-0016 removes the `queue_workers` cap; concurrency is
  bounded by the number of idle pool worktrees by construction), the "cap=1" sub-scenario above
  is exercised as a **pool of 1**: the testbed repo's pool is **seeded via the Create action to
  exactly one idle worktree** before the drain can dispatch, then TWO of its issues are flipped
  `Queue=Ready` at the real UI. Expected behavior is unchanged in human terms — the consumer
  dispatches EXACTLY ONE (the one idle worktree is drawn), DEFERS the other with NO worktree
  (empty pool ⇒ wait, no auto-create), and on close/Eject the freed worktree is REUSED for the
  deferred issue. The surface and Disqualifiers above still apply (real-UI Ready flip via the
  Chrome debug session; never the production repo or the real `default` namespace). This is a
  reinterpretation of the same case under the new model, not a new lane.

### REG-008 Queue-Drain Durable State In Temporal + Backend-Restart Survival

Goal:

Confirm the run/gate label (`running`/`parked_*`) and the worktree↔session binding
(session id, transcript, launched PID) are the per-run `TaskRunWorkflow`'s DURABLE state —
written by signal, read by query — and NOT the `owned-lane-bootstrap.json` side-store, which
is demoted to a recovery breadcrumb (its gate ossifies at `running`; it retains only the
launched PID for the reclaim path). After a backend crash/restart on the same Temporal
namespace, active lanes' gate + binding + per-repo watchdog supervision reconstruct from
durable Temporal state + the git worktree list — nothing is lost, no agent is orphaned.

Surface:

The queue-drain consumer + `GET /api/v1/worktrees` on the isolated `reg007` lane, driven
from the GitHub web surface (as REG-007). The human-perceived behavior under test is "kill
and restart the backend mid-run and nothing is lost."

Steps (in-app, against the Landing-2 code):

1. Via the REG-007 GitHub-web Ready-flip, dispatch a lane, then set the issue
   `Human Needed=Yes` so the consumer PARKS it (`run_gate_state=parked_*`).
2. Confirm `GET /api/v1/worktrees` reports the parked gate while the on-disk
   `owned-lane-bootstrap.json` still reads `running` (demotion: the binary trusts the
   workflow, not the JSON).
3. Kill the backend (simulated crash); restart on the same namespace.
4. Confirm `GET /api/v1/worktrees` still reports the parked lane (gate read from the durable
   workflow) and the watchdog supervisor is re-established for any launched lane.
5. Close the issue (human-approved path) and confirm reclaim still deallocates the worktree.

Expected result:

- gate/binding always reflect the workflow; the JSON breadcrumb is never authoritative
- a backend restart loses no active-lane state and orphans no agent/worktree

Disqualifiers:

- reading gate/binding from the JSON side-store, or a restart that drops a lane / orphans an agent
- CLI-signaling the workflow gate instead of driving the real consumer — that is a smoke
  (see [PASS-0011](./Tracking/Task-0015/Testing/PASS-0011/REG-007-LANDING2-LIVE-PROOF.md)),
  NOT this in-app regression

Status:

**PASSED on the Landing-2 code, live, in-app**
([PASS-0012](./Tracking/Task-0015/Testing/PASS-0012/REG-007-008-009-LANDING2-LIVE.md)): the
consumer parked on `Human Needed=Yes` (`parked [Task-0001]`), `/worktrees` reported
`parked_awaiting_closure` from the workflow while the on-disk breadcrumb stayed `running`
(demotion), a backend kill+restart reconstructed the parked lane from durable Temporal, and a
close reclaimed it. Mechanism also proven earlier by the gated real-Temporal smoke + manual
binary sequence ([PASS-0011](./Tracking/Task-0015/Testing/PASS-0011/REG-007-LANDING2-LIVE-PROOF.md)).
FOLLOW-UP (observed once, non-fatal): the FIRST poll after a restart can exceed the 2-minute poll
`StartToClose` timeout (self-corrects next tick); candidate cause is the first-build
`reconstructSupervision` + per-record workflow query on a cold worker. Tracked as a latency item.

### REG-009 Cross-Repo Registry Isolation (No Shared-#N Reclaim) [BUG-0003]

Goal:

With a multi-repo registry where two repos each have an issue `#N`, closing repo A's `#N`
must NOT reclaim repo B's live `Task-N` (no agent kill, no worktree deletion). Run identity
is repo-namespaced (`taskrun--<repoID>--Task-N--active`) and lane accounting is repo-scoped,
so one repo never sees or acts on another repo's lane.

Surface:

The queue-drain consumer over a TWO-repo registry on the isolated `reg007` lane (two
throwaway repos, each with an open `#1`).

Steps:

1. Register two throwaway repos, each with an open `#1` flipped `Queue=Ready`; dispatch both
   (each gets its own namespaced lane + a live launched agent).
2. Close repo A's `#1` (human-approved path) so its lane reclaims.
3. Confirm repo B's `Task-0001` worktree + launched agent SURVIVE (PID stays alive, worktree
   intact); only repo A's lane reclaimed.

Expected result:

- no cross-repo reclaim; repo B's live lane is untouched by repo A's close

Disqualifiers:

- a single-repo testbed (cannot exhibit the collision)
- proxy/log-only evidence without a live launched agent to prove it was not killed

Status:

**PASSED on the Landing-2 code, live, in-app**
([PASS-0012](./Tracking/Task-0015/Testing/PASS-0012/REG-007-008-009-LANDING2-LIVE.md)): with both
repos holding an open `#1 -> Task-0001`, both dispatched under DISTINCT repo-namespaced run ids
(`taskrun--QueueDrainTestbed--Task-0001--active` vs `taskrun--QueueDrainTestbed2--Task-0001--active`);
closing `QueueDrainTestbed#1` reclaimed ONLY its lane (`reclaimed [Task-0001]`, worktree gone) while
`QueueDrainTestbed2`'s live `Task-0001` lane SURVIVED (worktree intact, still listed). Also proven
on the Landing-1 code earlier
([PASS-0010](./Tracking/Task-0015/Testing/PASS-0010/REG-007-BUG-0003-FIX-PROOF.md), the
[BUG-0003](./Tracking/Task-0015/BUG-0003.md) fix).

### REG-010 WORKTREES Tab Pool View (Allocated/Idle Color + Repo + Path)

Goal:

Confirm the real desktop app exposes the Task-0016 `WORKTREES` tab (renamed from
`TASKS`, content replaced), that it shows the whole worktree pool from the backend
with allocated worktrees visibly distinguished from idle ones by background color, and
that each row shows the repo and the local directory path. This is the canonical pool
view case; it is not satisfied by a backend `/worktrees` JSON response or by a
screenshot of an empty tab.

Surface:

The real desktop overlay `WORKTREES` tab, backed by the orchestration backend's
`GET /api/v1/worktrees` on the isolated validation lane.

Steps:

1. Launch the real app from repo root on an isolated lane with a task-owned config and
   isolated SQLite database as documented in [TESTING.md](./TESTING.md). Do not use
   `C:\Users\gregs\.codex` or the human's dashboard config/database.
2. Start the separate validation-lane orchestration backend
   (`http://127.0.0.1:14318`) and point the app's worktrees readback at it
   (`CODEX_DASHBOARD_*_BACKEND_URL`), without disturbing the always-on service lane.
3. Seed the pool against the validation lane so it contains at least one ALLOCATED
   worktree (an assigned/dispatched task) and at least one IDLE worktree, in a repo
   registered in the validation-lane registry.
4. Trigger the real overlay path.
5. Confirm the nav shows a `WORKTREES` tab where `TASKS` used to be, and no `TASKS`
   tab remains.
6. Click the `WORKTREES` tab and verify the switch completes immediately and does not
   dispatch, assign, eject, destroy, dequeue, or otherwise change backend state by
   switching tabs.
7. Verify the surface lists every worktree the backend reports, each row showing the
   repo, the local directory path, a stable identifier, and — for allocated rows — the
   bound task/run/status. Verify each panel's HEADING is the short repo id in BOTH idle
   and allocated states (e.g. `obsidian`); a full filesystem path in the heading fails —
   the full bound checkout path belongs only in the Details reveal.
8. Verify allocated rows render with a visibly different background color than idle
   rows (the distinction is perceivable, not only a text label).
9. Stop the validation-lane backend and confirm the tab shows a clear human-facing
   backend-unavailable message rather than crashing or showing a blank surface; restart
   the backend and confirm the pool view recovers.
10. Capture an artifact from the running app surface showing the `WORKTREES` tab active
    with both an allocated (colored) and an idle row.
11. Exit cleanly.

Expected result:

- the nav shows `WORKTREES` (not `TASKS`); the old task stream/detail/dispatch content
  is gone from this tab
- switching to `WORKTREES` is read-only and does not mutate backend state
- the pool view renders every backend-reported worktree with repo + local dir +
  identifier, and allocated rows are a visibly different background color than idle rows
- each panel HEADING shows the short repo id (not a full filesystem path) in BOTH idle
  and allocated states
- a backend-unavailable state shows a clear message and does not crash
- no extra console or app windows are spawned by the interaction

Interpretation:

- this is the canonical regression for the Task-0016 `WORKTREES`-tab pool view
- unit coverage of the pure render/color helpers lives in
  `tests/test_worktrees_tab.py`; that unit proof supports but does not replace this
  app-surface case
- the desktop frontend does not change for the human until a new pinned release is
  published and the overlay restarted (see [TESTING.md](./TESTING.md) and
  `scripts/Publish-DashboardRelease.ps1`); that publish + restart is a separate
  human-gated step

### REG-011 WORKTREES Tab Copy-Path Control

Goal:

Confirm the `WORKTREES` tab's per-row copy control copies that worktree's exact local
directory path to the system clipboard (the mockup's copy icon).

Surface:

The real desktop overlay `WORKTREES` tab copy control, backed by
`GET /api/v1/worktrees` on the isolated validation lane.

Steps:

1. Launch the real app and validation-lane backend as in REG-010 (isolated lane,
   task-owned config/SQLite); seed at least one worktree.
2. Trigger the real overlay path and open the `WORKTREES` tab.
3. Click the copy-path control on a worktree row.
4. Read the system clipboard and compare it to the `worktree_path` the backend reports
   for that worktree (via `GET /api/v1/worktrees`).
5. Capture an artifact from the running app surface showing the copy control and the
   clipboard contents matching the row's path.
6. Exit cleanly.

Expected result:

- clicking the copy control places the exact `worktree_path` string for that row on the
  system clipboard
- the interaction does not mutate backend state or spawn extra windows

Interpretation:

- this is the canonical regression for the Task-0016 copy-path control
- comparing the clipboard to the backend-reported path is the proof; a screenshot of the
  icon alone does not satisfy this case

### REG-012 WORKTREES Tab Repo Filter (Registry-Sourced Dropdown)

Goal:

Confirm the `WORKTREES` tab's repo filter dropdown is populated from the repo registry
(`GET /api/v1/repos`), not a hardcoded list, and that selecting a repo narrows the pool
view to that repo while "All repos" restores the full view.

Surface:

The real desktop overlay `WORKTREES` tab repo filter, backed by `GET /api/v1/repos` and
`GET /api/v1/worktrees` on the isolated validation lane.

Steps:

1. Launch the real app and validation-lane backend as in REG-010, with a validation-lane
   registry that registers AT LEAST TWO repos, each holding at least one worktree.
2. Trigger the real overlay path and open the `WORKTREES` tab.
3. Open the repo filter dropdown and confirm its options match the repos returned by
   `GET /api/v1/repos` (plus an "All repos" option) — not a fixed/hardcoded set.
4. Select the first repo and confirm the pool view shows only that repo's worktrees.
5. Select the second repo and confirm the pool view shows only the second repo's
   worktrees.
6. Select "All repos" and confirm the full pool returns.
7. Capture an artifact from the running app surface showing the dropdown options and the
   filtered view for one repo.
8. Exit cleanly.

Expected result:

- the dropdown options equal the registry repos from `GET /api/v1/repos` plus "All repos"
- selecting a repo filters the pool view to that repo; "All repos" restores the full view
- a hardcoded repo list, or a dropdown that does not actually filter, fails this case
- the interaction does not mutate backend state or spawn extra windows

Interpretation:

- this is the canonical regression for the Task-0016 registry-sourced repo filter
- the registry-sourced requirement is load-bearing: the proof must show the options
  changing with the validation-lane registry, not a static list

### REG-013 WORKTREES Tab Create Control

Goal:

Confirm the `WORKTREES` tab's Create control provisions a new idle worktree into the
selected repo's pool via the backend and that the new idle worktree then appears in the
pool view.

Surface:

The real desktop overlay `WORKTREES` tab Create control, backed by
`POST /api/v1/worktrees/create` + `GET /api/v1/worktrees` on the isolated validation
lane.

Steps:

1. Launch the real app and validation-lane backend as in REG-010 (isolated lane,
   task-owned config/SQLite) against a validation-lane registry with at least one repo.
2. Trigger the real overlay path and open the `WORKTREES` tab. Note the current idle
   worktree count for the target repo.
3. Select the target repo (via the repo filter / create affordance) and click Create.
4. Confirm the surface refreshes and a NEW idle worktree for that repo now appears in
   the pool view (idle color, real local-dir path), increasing the repo's idle count by
   one.
5. Cross-check `GET /api/v1/worktrees` shows the same new idle worktree on a stable path
   (not an `os.MkdirTemp` random dir).
6. Capture an artifact from the running app surface showing the newly created idle
   worktree in the view.
7. Tear down the created worktree (Destroy from the UI, or backend cleanup) so the proof
   can be re-run.
8. Exit cleanly.

Expected result:

- clicking Create provisions one new idle worktree into the selected repo's pool and it
  appears in the view as idle with a real path
- the new worktree is at a stable pool path, persisted, with no task bound
- the interaction acts only through the backend endpoint and spawns no extra windows

Interpretation:

- this is the canonical regression for the Task-0016 Create control
- proof that the worktree appears in the running app's view is required; a 201 from the
  endpoint alone does not satisfy this case

### REG-014 WORKTREES Tab Assign Popup Binds An Open Task

Goal:

Confirm the `WORKTREES` tab's Assign control on an idle worktree opens a popup that
queries the open tasks (id + title + state, no progress bars) and that selecting a task
binds it onto the chosen idle worktree, which then flips to allocated in the view.

Surface:

The real desktop overlay `WORKTREES` tab Assign popup, backed by `GET /api/v1/tasks`
(open-task list) and `POST /api/v1/worktrees/assign` on the isolated validation lane.

Steps:

1. Launch the real app and validation-lane backend as in REG-010, with at least one IDLE
   worktree in the pool and at least one open task readable from `GET /api/v1/tasks`
   (isolated lane / task-owned fixture per [TESTING.md](./TESTING.md)).
2. Trigger the real overlay path and open the `WORKTREES` tab.
3. Click Assign on an idle worktree row.
4. Verify the popup lists open tasks with task id + title + state, and does NOT show
   per-task progress bars or file-ref metadata lines (mockup exclusion E6).
5. Select a task and confirm.
6. Verify the chosen worktree flips to ALLOCATED in the view (allocated color), bound to
   the selected task, with a running status, and that the SAME worktree path is reused
   (no new directory is created).
7. Cross-check `GET /api/v1/worktrees` shows that worktree allocated to the task.
8. Capture an artifact from the running app surface showing the populated Assign popup
   and the worktree allocated afterward.
9. Eject/clean up so the proof can be re-run.
10. Exit cleanly.

Expected result:

- the Assign popup lists open tasks (id + title + state) from `GET /api/v1/tasks`, with
  no progress bars or file-ref lines
- confirming a selection binds that task onto the chosen idle worktree, which flips to
  allocated in the view, reusing the same folder
- an empty/hardcoded popup list, or a popup that does not actually bind, fails this case
- the interaction acts only through backend endpoints and spawns no extra windows

Interpretation:

- this is the canonical regression for the Task-0016 Assign popup → bind interaction
- the open-task source is the existing `GET /api/v1/tasks`; the proof must show real
  open tasks in the popup and a real allocation result, not just the popup opening

### REG-015 WORKTREES Tab Eject Returns Idle And Dequeues

Goal:

Confirm the `WORKTREES` tab's Eject control on an allocated worktree stops the agent,
cleans the worktree back to baseline, returns the SAME folder to idle in the view, and
dequeues the freed task so it is not re-dispatched (the Task-0016 UPDATE 2 behavior).

Surface:

The real desktop overlay `WORKTREES` tab Eject control, backed by
`POST /api/v1/worktrees/eject` + `GET /api/v1/worktrees` on the isolated validation lane.

Steps:

1. Launch the real app and validation-lane backend as in REG-010, with an ALLOCATED
   worktree bound to a task whose provider queue state is `Ready` (use a throwaway
   testbed task on the isolated lane; never a production issue).
2. Trigger the real overlay path and open the `WORKTREES` tab.
3. Click Eject on the allocated worktree row.
4. Verify the worktree returns to IDLE in the view (idle color), the bound task is no
   longer shown, and the SAME local-dir path is still present (the folder was kept, not
   deleted).
5. Verify the freed task is dequeued: the pool read no longer re-allocates it on a
   subsequent refresh / consumer poll (its provider queue state is `Never`, not `Ready`),
   so it is not re-dispatched (no bounce-back). The task's issue remains open.
6. Capture an artifact from the running app surface showing the worktree idle after Eject
   and the task no longer bound/re-dispatched.
7. Exit cleanly.

Expected result:

- clicking Eject returns the allocated worktree to idle in the view with the same folder
  retained (Eject never deletes the folder)
- the freed task is dequeued (`Queue=Never`, issue still open) and is not re-dispatched
  on the next poll
- the interaction acts only through the backend endpoint and spawns no extra windows

Interpretation:

- this is the canonical regression for the Task-0016 Eject (keep-folder + return-idle +
  dequeue) interaction at the desktop surface
- the no-bounce-back consequence is exercised in-app here; the backend-seam no-redispatch
  unit proof supports but does not replace this case
- run only against throwaway testbed tasks on the isolated lane; the dequeue is a backend
  provider write and must never touch a production-owned queue
- **Incomplete pending [BUG-0005](./Tracking/Task-0016/BUG-0005.md):** this case currently
  asserts the worktree returns to idle, but the live smoke showed Eject leaves the run's
  Temporal workflow ACTIVE/orphaned (worktree freed, run not terminated). Once BUG-0005 is
  fixed, strengthen this case to also assert the run is terminated (no active run lingers
  with no worktree after Eject).

### REG-016 WORKTREES Tab Destroy (Idle Only) And Standalone Dequeue

Goal:

Confirm the `WORKTREES` tab's Destroy control removes an idle worktree from the view,
rejects Destroy on an allocated worktree with a clear human-facing message (removing
nothing), and that the standalone Dequeue control takes a task out of the queue without
ejecting (the run keeps going; the issue stays open).

Surface:

The real desktop overlay `WORKTREES` tab Destroy + Dequeue controls, backed by
`POST /api/v1/worktrees/destroy`, `POST /api/v1/worktrees/dequeue`, and
`GET /api/v1/worktrees` on the isolated validation lane.

Steps:

1. Launch the real app and validation-lane backend as in REG-010, with BOTH an idle
   worktree and an allocated worktree in the pool (the allocated one bound to a throwaway
   testbed task on the isolated lane).
2. Trigger the real overlay path and open the `WORKTREES` tab.
3. Click Destroy on the IDLE worktree and confirm it disappears from the view and from
   `GET /api/v1/worktrees`.
4. Click Destroy on the ALLOCATED worktree and confirm the surface shows a clear
   human-facing rejection message (operator must Eject first), the worktree is NOT
   removed, and it remains allocated.
5. Click the standalone Dequeue control for the allocated worktree's task and confirm:
   the worktree stays ALLOCATED (the run is not stopped, the worktree is not returned to
   idle), the task's provider queue state becomes `Never` (it will not be re-dispatched),
   and the issue stays OPEN (Dequeue is not Close).
6. Capture an artifact from the running app surface showing the idle worktree removed, the
   allocated-Destroy rejection message, and the post-Dequeue allocated-but-not-queued
   state.
7. Exit cleanly.

Expected result:

- Destroy removes an idle worktree from the view; Destroy on an allocated worktree is
  rejected with a clear message and removes nothing
- standalone Dequeue takes the task out of the queue without ejecting (worktree stays
  allocated, run continues) and does not close the issue
- the interactions act only through backend endpoints and spawn no extra windows

Interpretation:

- this is the canonical regression for the Task-0016 Destroy (idle-only) and standalone
  Dequeue controls at the desktop surface
- the allocated-Destroy rejection and the dequeue-is-not-close behavior are load-bearing;
  a Destroy that deletes an allocated worktree, or a Dequeue that closes the issue, fails
- run only against throwaway testbed tasks on the isolated lane

### REG-017 WORKTREES Assign Popup Loads With A State-less Task In The Tracking Set

Goal:

Confirm the `WORKTREES` tab's Assign popup still loads its open-task list
(`GET /api/v1/tasks`) when one or more `Tracking/Task-*` directories lack a
`TASK-STATE.json`. Regression must exercise this against a realistic Tracking set so a
single state-less task cannot 502 the endpoint and blank the Assign popup (BUG-0003 — found
on the live registry where Task-0014 had no TASK-STATE.json).

Surface:

The real `WORKTREES` tab Assign popup, backed by `GET /api/v1/tasks` on the isolated
validation lane, with the lane's Tracking set seeded to include at least one task that has
`TASK.md` but NO `TASK-STATE.json`.

Steps:

1. Launch the real app + validation-lane backend as in REG-010, with the lane's Tracking
   root containing several tasks AND at least one Task dir that has `TASK.md` but no
   `TASK-STATE.json`.
2. Open the `WORKTREES` tab and an idle worktree's Assign control.
3. Confirm the Assign popup POPULATES with the open tasks (it does not blank, error, or
   hang); `GET /api/v1/tasks` returns 200, not 502.
4. Confirm the state-less task still appears (with an unknown/default state, not fabricated)
   and that a normal task can be selected and bound.
5. Capture an artifact showing the populated popup.

Expected result:

- the Assign popup populates even with a state-less task present; `GET /api/v1/tasks` is
  200 (a single missing `TASK-STATE.json` must NOT 502 the whole list)
- the state-less task appears with a default/unknown state; no state is fabricated

Interpretation:

- this is the human-surface regression for **BUG-0003** (a state-less task 502'd the Assign
  popup on the live registry). Unit coverage: `TestListTasksToleratesMissingTaskState`
  (supports but does not replace this in-app case).
- lesson (why this case exists): the WORKTREES human-surface cases (REG-010…016) must be run
  against a REALISTIC Tracking/registry set — including a state-less task — not only a
  pristine throwaway set, or this class of bug reaches the human instead of regression.

## Supporting Smoke

### SMOKE-001 Ingest Core

Run:

```powershell
python -m app.codex_dashboard --scan-once --print-summary
```

Expected result:

- the app reads real-shaped telemetry
- SQLite persistence is updated
- a human-readable summary prints

Interpretation:

- this is supporting proof only
- it does not replace `REG-001`

### SMOKE-002 Service Lane Release Isolation

Goal:

Confirm the human service lane is pinned to a promoted release and cannot be
advanced by merely updating or editing the mutable repo checkout.

Precondition:

Only run against the human service lane after the human explicitly authorizes
that lane to be inspected or restarted for this specific run.

Steps:

1. Run the unit tests for service-lane scripts:
   `python -m unittest tests.test_service_lane_scripts -v`
2. Publish the intended service-lane release with
   `backend/orchestration/scripts/Publish-ServiceLaneRelease.ps1`.
3. Restart or install the service lane through the repo scripts only after the
   release has been pinned.
4. Run `backend/orchestration/scripts/Test-ServiceLaneIsolation.ps1`.
5. Run `backend/orchestration/scripts/Get-ServiceLaneStatus.ps1`.
6. Verify no live service-lane runner command line points at the repo-local
   `Run-OrchestrationLane.ps1`.

Expected result:

- the scheduled task uses the runtime-root launcher
- `current-release.json` exists and validates binary and compose-file hashes
- the running process path matches the pinned release binary path
- backend health is reachable
- any dirty-source promotion is explicitly visible in the release manifest

Interpretation:

- this is required operator proof for human-lane release claims
- this is not a substitute for task-level desktop-app regression cases

### SMOKE-003 Dashboard Frontend Release Isolation

Goal:

Confirm the human-facing desktop dashboard frontend is pinned to a promoted
release and cannot be advanced by merely updating or editing the mutable repo
checkout.

Precondition:

Only run against the human dashboard frontend after the human explicitly
authorizes that lane to be inspected or restarted for this specific run.

Steps:

1. Run the unit tests for dashboard release scripts:
   `python -m unittest tests.test_dashboard_release_scripts tests.test_desktop_support -v`
2. Publish the intended dashboard release with `scripts/Publish-DashboardRelease.ps1`.
3. Restart the dashboard through `scripts/Start-DashboardRelease.ps1` or the
   installed runtime launcher.
4. Run `scripts/Test-DashboardRelease.ps1`.
5. Verify no live dashboard command line is `pythonw -m app.codex_dashboard`
   from `C:\Agent\CodexDashboard` without a release id and release root.
6. For visible-surface claims, run an app smoke artifact capture against the
   pinned release and verify the expected tab is active in the generated
   `overlay-summary.txt`.

Expected result:

- `dashboard-current-release.json` exists and validates copied source hashes
- frontend `source_mode` is `git_commit` unless the human explicitly requested
  a dirty working-tree promotion
- the startup file points at the runtime-root dashboard launcher
- the running dashboard process includes the pinned release id and release root
- any dirty-source promotion is explicitly visible in the release manifest
- visible-surface claims cite a smoke artifact, not backend-only proof

Interpretation:

- this is required operator proof for human-facing dashboard frontend release
  claims
- this is not a substitute for task-level desktop-app regression cases
