# Task-0015 Research Synthesis

Planning-ready synthesis for the Temporal-backed GitHub queue-drain consumer.
Produced single-context by the TaskDispatch task leader (no nested
research-leader dispatch tool was available); same phase discipline applied. The
solution shape is fixed by [TASK.md](./TASK.md); this synthesis confirms the
seams from real code so the pass plan is evidence-based.

Companion: [RESEARCH-PLAN.md](./RESEARCH-PLAN.md) (problem list),
[Research/LIVENESS-SIGNAL.md](./Research/LIVENESS-SIGNAL.md) (the embedded O5
research item, COMPLETE).

## Verified Seam Map (from real code)

### §O1 — Manifest rename + `queue_workers` (P6)

- `CODEX-REPO-MANIFEST.json` today has `schema_version`, `manifest_type:
  codex_repo_registry`, and a single `repos[]` entry (`id: CodexDashboard`,
  `local_root`, `source_control_provider`, `task_provider`
  (`Digital-Collective-Games/Obsidian`), `task_proposal_provider`). No
  `queue_workers` field. (Confirmed by reading the file.)
- Live-code references to the FILENAME confirmed present:
  - `skills/obsidian-operator/SKILL.md` frontmatter `description` (line 3) and
    body grounding (line 10).
  - `skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1` default
    `-ManifestPath` (line 4).
  - `skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1` default
    `-ManifestPath` (line 6).
  - `skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1` default
    `-ManifestPath` (line 4).
  - `tests/test_obsidian_title_roundtrip.py` (fixture write line 110;
    `-ManifestPath` arg line 154).
  - `app/codex_dashboard/paths.py` (doc comment naming the file, ~line 52).
  - `DATA-HANDLING.md` (lines 20, 161, 239).
- Durable history that must NOT be rewritten (F-O1): anything under
  `Tracking/Task-0012/*`, `Tracking/Task-0013/*`, and
  `C:\Users\gregs\.codex\sessions\*`. The rename must be verified by a guard
  (e.g. `git diff --stat` / a search) showing only the live-code refs changed.
- Planning consequence: O1 is a small, independently shippable pass (rename +
  field + named-ref edits + the title-roundtrip unit test A1.3). It should be
  PASS-0000 so later passes can read `queue_workers`.

### §O2 — Real N>1 concurrency (P3)

- The hard 1:1 is real and lives at two points:
  - `deriveDispatchReadiness` appends the `active_run_exists` block reason
    whenever an active run exists
    (`backend/orchestration/internal/taskrun/service.go:777-781`), blocking a
    second dispatch.
  - `dispatchWithDirective` calls `releasePreviousOwnedLane`
    (`service.go:138`, impl `service.go:968-986`) which tears down the prior
    owned lane via `cleanupOwnedLane` before a new dispatch.
- `RepoLane` (`types.go:121-138`) is a single owned checkout; there is no
  per-repo slot or pending-queue concept anywhere.
- Owned-lane lifecycle to REUSE unchanged per slot:
  `provisionOwnedLane` runs `git worktree add --detach`
  (`service.go:934-966`); `bootstrapOwnedLane` writes
  `owned-lane-bootstrap.json` (`service.go:988-1026`); `cleanupOwnedLane` runs
  `git worktree remove --force` (`service.go:1028-1045`).
- Planning consequence: O2 must (a) make the dispatch gate "fewer than
  `queue_workers` active owned lanes for this repo" instead of "no active owned
  lane," and (b) stop `releasePreviousOwnedLane` from tearing down a same-repo
  SIBLING that legitimately holds its own slot. Proof is TWO concurrent
  worktrees + two live runs (A2.1), a refused/queued 5th (A2.2), and no
  `active_run_exists` while a free slot remains (A2.3).

### §O3 — GitHub queue-drain consumer

- Worker registration seam: `StartWorker` registers `jobexec` + `taskexec`
  (`temporalbackend/backend.go:134-142`); a new `internal/queue` register call
  slots in next to `taskexec.Register(w)`.
- HTTP route seam: all routes are registered in one place
  (`httpapi/mux.go:16-89`); the POST dispatch route pattern is
  `mux.go:122-145` (start/stop the consumer follows that shape).
- Provider read: the consumer polls the same org issue-field-values endpoint
  the skill uses (`/repos/Digital-Collective-Games/Obsidian/issues/<n>/issue-field-values`)
  for `Queue == Ready`, maps issue `#N` → `Tracking/Task-N` exactly (provider
  contract `skills/obsidian-operator/SKILL.md:19`), and dispatches into a free
  slot via the existing `taskrun` dispatch path. No separate queue DB.
- Planning consequence: O3 depends on O2 (needs slots to dispatch > 1). Provable
  against a task-owned fixture/test issue set on the isolated lane.

### §O4 — Done-contract + parked needs-human + human-only closure (P5)

- Issue-field writes already exist in the skill:
  `Sync-TaskToGitHubIssue.ps1` writes field values (`:300,455-461`, and per
  TASK.md `:548-553`); reconcile reads them
  (`Reconcile-TaskGitHubState.ps1`). `Human Needed` is the existing org field
  (No/Yes) and is owned by Codex/local task state per the skill Authority Model
  (`SKILL.md:54`).
- Close/reopen builders already exist: reconcile builds
  `gh issue close --reason completed|not planned --comment ...`
  (`Reconcile-TaskGitHubState.ps1:557-567`) and `gh issue reopen` (`:571-575`);
  terminal local statuses map to CLOSED (`:140-141`).
- TIGHTENING required (the human's closure directive): in the autonomous queue
  flow the agent NEVER self-closes (abandon OR perceived completion), and a
  local "complete"/terminal status MUST NOT auto-close the issue. Close happens
  ONLY on explicit human closure approval, via the existing human-gated
  obsidian-operator close path. Approvals for research/plan/pass/regression
  gates are NOT closure approval.
- No second GitHub-write path (A4.6): all issue-state/field writes go through
  the obsidian-operator surface. SKILL.md guardrail
  `SKILL.md:166` ("Do not create backend orchestration endpoints for this
  workflow unless a later task explicitly asks for them") — THIS task is that
  explicit later task for the consumer/endpoint, but the GitHub WRITES still
  route through the skill, not a parallel backend writer.
- Park semantics (the revision): `Human Needed=Yes` (incl. "awaiting closure
  approval") = PARK in place — keep worktree + slot, do NOT redispatch, do NOT
  `cleanupOwnedLane`; resumable in the SAME `OwnedRepoRoot`. ONLY a
  human-approved CLOSED issue calls `cleanupOwnedLane` and frees the slot.
- Run/gate state: the parked-vs-running distinction and which gate
  (research/plan/regression/awaiting-closure) must be recorded on the owned-lane
  record (shared with O6's binding) so the watchdog (O5) and endpoint (O6) read
  it. Existing states (`StateWaitingForHuman`, `types.go:18`) are related but
  the task needs the GitHub `Human Needed` state to be the queryable source of
  truth; the local state is an internal aid only (D2).
- Planning consequence: O4 is partly agent-side (skill done-contract writes +
  SKILL.md "Queue Done-Contract" doc) and partly consumer-side (read issue
  state to drive cleanup/park/dequeue; never auto-close). Provable by driving
  each transition on the isolated lane with dry-run-first GitHub interaction.

### §O5 — Liveness watchdog + top-level agent + incident email (P1, P4)

- The embedded research item is COMPLETE: the signal is bound-transcript append
  growth (size+mtime → `last_active_signal_at`), corroborated by process state;
  detection deadline anchored to last activity, never dispatch time. See
  [Research/LIVENESS-SIGNAL.md](./Research/LIVENESS-SIGNAL.md).
- Reusable primitives: `PokeRun` `poke_worker_check` follow-up
  (`service.go:226-255`); `staleRunUpdate` → `sleeping_or_stalled`
  (`service.go:1254-1268`); overdue-follow-up escalation to urgent
  (`service.go:1214-1222`). Current `SuspiciousAfter` = dispatch + 15 min
  (`taskexec.go:81,165,296`; `service.go:805`) — this is the fixed-timer the
  task narrows to last-activity-anchored ~5 min and refreshes on each append.
- IMPORTANT FINDING (P4): the CURRENT execution model does NOT launch a coding
  agent. `runOwnedLaneExecution` (`taskexec.go:120-293`) runs backend
  ACTIVITIES (preflight → workload step → execute step) inside the owned lane;
  TESTING.md confirms unit dispatch runs "without ... launching Codex." So O5's
  requirement to launch a TOP-LEVEL headless `codex`/`claude` agent that can
  spawn its OWN subagents (A5.1 / F-O5-toplevel) is genuinely NET-NEW dispatch
  work, not a tweak. Evidence the runtimes CAN spawn subagents: Claude already
  writes per-subagent transcripts under `.../<session>/subagents/agent-*.jsonl`
  (observed on this machine), so a top-level agent spawning subagents is the
  normal mode — the requirement is to launch it as a top-level process, not
  nested.
- `DeepContext` (`types.go:79-84`) already carries `SessionID`,
  `TranscriptPath`, and `LaunchTargets`, and `buildDeepContext`
  (`taskexec.go:388-462`) builds it from `request.ContextSnapshot`. This is the
  bridge to O6 and to the watchdog's transcript path — but today it is built on
  the in-memory run view, not persisted on the durable owned-lane record.
- Email: incident → `gmail-digest-email` skill / local gmail MCP to
  `admin@digitalcollective.games`, containing observed state + transcript
  (A5.3). On the isolated lane the send may be captured/mocked but the artifact
  must show full email contents.
- Parked suspension (A5.5 / F-O5-parked): skip `staleRunUpdate` + poke + email
  for any run whose issue is `Human Needed=Yes` / owned-lane gate-parked.
- Planning consequence: O5 is the largest pass and has two halves — (5a)
  top-level agent dispatch (net-new launcher) + transcript binding, and (5b) the
  external watchdog (signal-anchored detect, parked-suspend, one poke, incident
  email). Both provable with controlled repros (stall repro; gate-parked-idle
  repro) on the isolated lane.

### §O6 — Worktree↔session binding registry + enumeration endpoint (P2)

- Today the binding fields do NOT exist on the durable record. `RepoLane`
  (`types.go:121-138`) carries `OwnedRepoRoot` + artifact paths but NO agent
  session id, transcript path, or run/gate state. `bootstrapOwnedLane`
  (`service.go:988-1026`) writes `owned-lane-bootstrap.json` without them. The
  session/transcript only appear on the IN-MEMORY `DeepContext` of the run view
  (built by `buildDeepContext`), which is not the durable owned-lane record.
- Planning consequence: O6 must extend `RepoLane` + the bootstrap record with
  `{ repo, issue #N / Task-N, worktree path (OwnedRepoRoot), agent session id,
  session transcript path, run/gate state }` and populate them at dispatch
  (durable). Then add a read endpoint `GET /api/v1/worktrees` in `httpapi/mux.go`
  (following the `handleTasksList` read pattern `mux.go:91-108`) returning every
  active worktree (running AND parked) with its binding. Then add an
  `obsidian-operator` command (new script e.g.
  `Get-ActiveWorktreeSessions.ps1`) that calls the endpoint and prints, per
  worktree, the worktree path, issue/Task, run/gate state, and transcript path.
- Orchestrator boundary (TASK.md O6): the endpoint SUPPLIES the fields to
  CONSTRUCT a VSCodium link (worktree path/cwd, session id, transcript path) but
  does NOT itself emit a `vscodium://` link — link generation is a downstream
  concern.
- O6 binding is a PREREQUISITE the O5 watchdog reads (the transcript path it
  samples). Plan O6 binding alongside / before the O5 watchdog half.

## Cross-Cutting / Proof (P7)

- Go test command: `go test ./...` from `backend/orchestration` (confirmed
  convention, `Tracking/Task-0008/Testing/PASS-0002-BACKEND-SMOKE-0001.md:24-26`;
  AX.1).
- Isolated lane (REGRESSION.md:11-19, TESTING.md "Known lanes"): validation lane
  backend `http://127.0.0.1:14318`, Temporal `127.0.0.1:17233`, Postgres
  `15432`, runtime root
  `%LOCALAPPDATA%\CodexDashboard\orchestration-validation-lane`; start/stop via
  `backend/orchestration/scripts/Start-ValidationLane.ps1` /
  `Stop-ValidationLane.ps1`. NEVER the human service lane (`:4318`/`:7233`/`5432`)
  or live Codex/dashboard data unless the human authorizes.
- GitHub interaction stays dry-run-first and human-gated
  (`SKILL.md:96-108,161-168`); proof uses task-owned fixtures / an explicitly
  gated test issue, never ungated live GitHub writes (F-X).
- A named REGRESSION.md case must cover the new operator behavior incl. the
  simulated-stall incident email (AX.2 / REGRESSION.md:23). NOTE: this repo's
  canonical regression lane is the DESKTOP APP surface (REGRESSION.md Canonical
  Rule). The new behavior is backend/operator-facing, not a desktop screen — so
  the named case will be a backend/operator-lane case; flag at the plan gate
  whether the human wants it added to REGRESSION.md as a new case class
  (operator lane) vs. only an isolated backend regression run, since the
  existing matrix is app-surface-centric.

## Planning-Readiness Verdict

- All decision-shaping problems P1–P7 are answered from real code/evidence.
- The embedded research item (P1 / O5 signal) is pinned in a durable artifact.
- No solution-shape redesign and no scope narrowing occurred.
- Remaining unknowns are bounded implementation choices the task explicitly
  leaves open (slot Temporal object, poll cadence, watchdog placement) and are
  constrained by acceptance criteria, not by missing research.
- One human-gate-worthy clarification surfaced for the plan gate: how the new
  operator-facing behavior is represented in the app-surface-centric
  `REGRESSION.md` (new operator-lane case class vs isolated backend run). This
  does not block planning; it is raised in the plan review package.

READY FOR PLANNING.
