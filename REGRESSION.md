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
