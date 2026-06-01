# PASS-0009 LIVE REGRESSION-RUN-0001 — REG-007 / REG-008 / REG-009 + live REG-015 (BUG-0001) + clean reg014

Date: 2026-06-01. Run owner: TaskDispatch task leader (single-context; nested dispatch
unavailable — labeled honestly). This is the LIVE regression phase for Task-0016 under
the manual worktree-pool model, driven from the REAL GitHub web surface via the
human-authenticated Chrome debug session (port 9222).

## Lane / isolation (CONFIRMED before the consumer ran)

- Backend: control-plane binary built from the tree at HEAD `084effa` (contains the
  BUG-0001 dequeue fix) → `127.0.0.1:24320` (REG-007/008/009/015) and `127.0.0.1:24321`
  (reg014 clean capture). NEVER the service lane `:4318`, the validation lane `:14318`,
  the human dashboard config/db, or live Codex data.
- Temporal: validation server `127.0.0.1:17233`, **dedicated throwaway namespaces**
  `reg007live` / `reg014cap` (deleted at teardown). NEVER the real `default` namespace
  (the operator cron jobs) or the service Temporal `:7233`.
- Registry → throwaway repos ONLY: `RepoA → Digital-Collective-Games/QueueDrainTestbed`,
  `RepoB → Digital-Collective-Games/QueueDrainTestbed2`. NEVER the human production repo
  `Digital-Collective-Games/Obsidian`.
- Isolated owned-lane root via `TEMP` override into the lane (`…/reg007-live/temp/cdxow`),
  isolated runs/jobs/tracking roots. `/api/v1/repos` confirmed the registry and showed
  **no `queue_workers`** field.
- Post-run: production lanes `:4318` (pid 27680) and `:14318` (pid 77280) confirmed on
  their original pre-run pids — untouched. Both throwaway namespaces deleted; testbed
  issues reset to `Queue=Never` + closed; lane dirs + binaries removed.

## REG-007 — pool-of-1 dispatch FROM the real GitHub web surface — PARTIAL (core PROVEN; Create-seed + reuse BLOCKED by [BUG-0002](../../BUG-0002.md))

PROVEN (the human-perceived dispatch behavior):
- Real-GitHub-UI `Queue=Ready` flip via `Set-IssueFieldViaUi.ps1` on the 9222 session
  (`QueueDrainTestbed #1` and `#5`), each `Committed=True`,
  `ObservedButtonText=QueueReady`, `ApiMatches=True`. (API/proxy flip disqualifier ruled
  out — the WRITE under test was the real web UI.)
- The always-on consumer (poll 30s, `<=60s`) dispatched **exactly one** Ready issue into
  a pool worktree within ~30s of an idle worktree existing (`<=1 min`), launching a
  **top-level `claude` agent** (`agent_session_id=b171904e-…`, `launched_pid=65272`,
  discoverable transcript under `~/.claude/projects/…wt-0001/…jsonl`),
  `run_gate_state=running`. (codex-dispatch disqualifier ruled out — it launched claude.)
- The **SECOND Ready issue WAITED**: only one pool member existed, no `wt-0002` was
  auto-created, no `Task-0005` workflow started — empty pool defers, no auto-create.
- Proof: [reg007-live/consumer-backend.log](./reg007-live/consumer-backend.log),
  [reg007-live/api-repos.json](./reg007-live/api-repos.json),
  [reg007-live/REPO-MANIFEST.json](./reg007-live/REPO-MANIFEST.json).

BLOCKED (recorded honestly):
- **Seeding the consumer's pool via the product Create action** and the **close/eject →
  free → reuse** leg could NOT complete via the documented path. The dashboard
  `POST /api/v1/worktrees/create` segments the pool by a hash of the dashboard's single
  `WorktreeRoot`, while the registry-driven consumer segments by the registry repo id —
  so a Create-seeded worktree is invisible to the consumer's pool-draw. The core dispatch
  above was only reachable after **hand-seeding** an idle worktree under the consumer's
  repo segment (clearly labeled). Tracked as [BUG-0002](../../BUG-0002.md).

## REG-008 — durable-state survival across backend restart — PASS

- Drove the live consumer to dispatch `Task-0005` (RepoA), then set
  `Human Needed=Yes` at the **real GitHub UI** (`Committed=True`, `ApiMatches=True`) →
  consumer logged `parked [Task-0005 …]`; `/worktrees` reported
  `run_gate_state=parked_awaiting_closure` read from the workflow, while the on-disk
  `owned-lane-bootstrap.json` breadcrumb still read `running` — **demotion proven**.
- Killed the backend, restarted on the SAME `reg007live` namespace. `/worktrees` STILL
  reported the parked Task-0005 lane (`allocated / parked_awaiting_closure`,
  reconstructed from durable Temporal via discover-on-startup), and the idle worktree's
  idle classification reconstructed. Subsequent polls RETAINED the parked lane (never
  redispatched/reclaimed). No first-poll-after-restart timeout occurred this run (the
  documented REGRESSION.md latency caveat; it self-corrects when it appears).
- Proof: [reg008-live/backend-restart.log](./reg008-live/backend-restart.log),
  [reg008-live/worktrees-post-restart.json](./reg008-live/worktrees-post-restart.json).
- (The final close→reclaim leg was not re-driven on this lane — see the REG-007 reuse
  blocker; the park/restart/reconstruct/retain core — the heart of REG-008 — is proven.)

## REG-009 — cross-repo pool isolation — PASS

- Two throwaway repos each with issue `#1` dispatched concurrently under **DISTINCT
  repo-namespaced run ids**: `taskrun--RepoA--Task-0001--active` and
  `taskrun--RepoB--Task-0001--active` (the BUG-0003 cutover property).
- Closed `QueueDrainTestbed2 #1` (human-approved `gh issue close`). The consumer
  reclaimed ONLY RepoB's lane (its checkout removed); RepoA's lanes (the parked Task-0005
  and the Task-0001 workflow) **SURVIVED** — worktrees intact, lane still listed. Repo A's
  identically-numbered `#1` was not touched by repo B's close.
- Proof: [reg009-live/REG-009-LIVE-RESULT.txt](./reg009-live/REG-009-LIVE-RESULT.txt),
  [reg009-live/consumer.log](./reg009-live/consumer.log). (single-repo / proxy-only
  disqualifiers ruled out — two repos, live launched agents/run ids.)

## Live REG-015 — verifies the [BUG-0001](../../BUG-0001.md) fix — PASS

- Real-GitHub-UI flip `QueueDrainTestbed #1 → Ready` (`Committed=True`,
  `ObservedButtonText=QueueReady`, `ApiMatches=True`).
- Dashboard `POST /api/v1/worktrees/assign` bound `Task-0001` onto an idle worktree
  (allocated, `run taskrun--Task-0001--active`); dashboard
  `POST /api/v1/worktrees/eject {run_id}` → worktree returned to **idle**, **folder kept**
  on disk.
- **BUG-0001 fix verified LIVE:** the Eject wrote **`Queue=Never`** to
  `QueueDrainTestbed #1` (was `Ready`) — the multi-repo registry-resolving dequeue
  provider correctly resolved `record.Repo="RepoA"` →
  `Digital-Collective-Games/QueueDrainTestbed` and wrote to the CORRECT repo. The issue
  stayed **OPEN** (dequeue ≠ close; human-only closure preserved).
- **No re-dispatch:** with the consumer running and an idle worktree available, `Task-0001`
  was NOT re-dispatched (skipped — `#1=Never`), while a genuinely-Ready issue
  (`#5`/`Task-0005`) WAS dispatched — proving the dequeue is load-bearing (no bounce-back).
- Proof: [reg015-live/REG-015-LIVE-RESULT.txt](./reg015-live/REG-015-LIVE-RESULT.txt),
  [reg015-live/backend.log](./reg015-live/backend.log).
- HONEST SCOPE NOTE: the Eject was exercised via the dashboard HTTP endpoint (the exact
  production code path the WORKTREES-tab Eject button calls), driven against the real
  throwaway GitHub queue — NOT via a literal Tk widget click. The Tk-widget Eject was
  already proven on the local-no-op lane in the prior PASS-0009 QA; the LIVE provider
  WRITE + no-re-dispatch (what BUG-0001 needed) is proven here. The product `Queue=Never`
  write against the THROWAWAY testbed is allowed and is NOT a violation of the REG-007
  Ready-flip-via-UI rule (a separate backend write).

## Clean reg014 re-capture — DONE (resolves the QA-REPORT path-string artifact)

- Fresh-EMPTY pool seed → Create exactly one worktree (`RepoX/wt-0001`) → harness drove
  the REAL Tk WORKTREES-tab Assign of `Task-9001` onto it + captured the app surface.
- Result: allocated panel shows `RepoX` short heading, `ALLOCATED - RUNNING` chip,
  `Task: Task-9001`, and `Local dir: …\wt-0001\wt-0001` — the displayed Local dir now
  matches the assigned worktree id (`RepoX/wt-0001`); POOL 01 / ALLOCATED 01 / IDLE 00.
  The prior `wt-0001\wt-0001` vs status-`wt-0002` mismatch is gone (it was a stale
  re-seed artifact; here a single fresh-empty worktree is used).
- Proof: [reg014-assign-bound-clean/overlay.png](./reg014-assign-bound-clean/overlay.png),
  [reg014-assign-bound-clean/overlay-summary.txt](./reg014-assign-bound-clean/overlay-summary.txt).

## Net verdict

- REG-008, REG-009, live REG-015 (BUG-0001 fix): **PASS** live, real-surface.
- REG-007: **PARTIAL** — the real-UI-flip → dispatch → claude-launch → second-Ready-defers
  core is PROVEN; the product-Create-seed and close/eject→reuse legs are **BLOCKED** by
  [BUG-0002](../../BUG-0002.md) (must be fixed + re-verified before REG-007 is fully green
  and before task closure).
- reg014 clean capture: **DONE**.
- NO production repo / lane / namespace / issue was touched.
