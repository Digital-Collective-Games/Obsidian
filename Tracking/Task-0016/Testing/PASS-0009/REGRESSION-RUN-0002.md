# PASS-0009 LIVE REGRESSION-RUN-0002 — REG-007 re-verify AFTER the BUG-0002 fix

Date: 2026-06-01. Run owner: context-blind fix + live-regression worker (single context;
nested `claude` agents WERE launched by the consumer — real nested dispatch, not a
single-context fallback). This run re-verifies the REG-007 legs that
[REGRESSION-RUN-0001.md](./REGRESSION-RUN-0001.md) recorded as BLOCKED by
[BUG-0002](../../BUG-0002.md), now that the fix is committed (`dddab0e`).

## Lane / isolation (CONFIRMED before + after)

- Backend: control-plane binary built from the fixed tree at HEAD `dddab0e` (the BUG-0002
  fix) → `127.0.0.1:24330`. NEVER the service lane `:4318`, the validation lane `:14318`,
  the human dashboard config/db, or live Codex data.
- Temporal: validation server `127.0.0.1:17233`, **dedicated throwaway namespace**
  `reg007fix` (created this run, deleted at teardown). NEVER the real `default` namespace
  or the service Temporal `:7233`.
- Registry → throwaway repos ONLY: `RepoA → Digital-Collective-Games/QueueDrainTestbed`,
  `RepoB → Digital-Collective-Games/QueueDrainTestbed2`. NEVER the human production repo
  `Digital-Collective-Games/Obsidian`.
- Isolated owned-lane root via `TEMP` override (`…/reg007fix/temp/cdxow`); isolated
  runs/jobs/tracking roots. `/api/v1/repos` confirmed the registry + **no `queue_workers`**.
  The dashboard `WorktreeRoot` = the lane root (NOT RepoA's `local_root`) — the faithful
  production condition that previously hashed to `repo-<hash>/`.
- Post-run: production lanes `:4318` (pid 27680) and `:14318` (pid 77280) confirmed on
  their original pids — untouched. The `reg007fix` namespace deleted; testbed issues reset
  to `Queue=Never` + closed; lane dir + binary removed; the two launched `claude` agents
  killed.

## REG-007 — pool-of-1 dispatch FROM the real GitHub web surface — PASS (all legs green)

The legs RUN-0001 PROVEN stand; this run closes the BLOCKED legs:

- **(a) Product Create seeds the consumer's segment (BUG-0002 fix).**
  `POST /api/v1/worktrees/create {repo:"RepoA"}` → `worktree_id=RepoA/wt-0001`, path
  `…/cdxow/RepoA/wt-0001/wt-0001` (the **RepoA registry-id segment**, NOT `repo-<hash>/`).
  The checkout is a genuine RepoA checkout (HEAD = RepoA commit `d9569ee`, contains RepoA's
  `Tracking/Task-0005`+`Task-0006`).
- **(c) Real-GitHub-UI Ready flips** (`Set-IssueFieldViaUi.ps1` on the 9222 session;
  API/proxy flip disqualifier ruled out): `QueueDrainTestbed #5` and `#6`, each
  `Committed=True`, `ObservedButtonText=QueueReady`, `ApiMatches=True`.
- **(d) Exactly one dispatched into the Created worktree; the second waits.**
  `14:59:52 poll acted: dispatched [Task-0005] parked [] reclaimed []`. `RepoA/wt-0001` →
  allocated (`taskrun--RepoA--Task-0005--active`), launching a **top-level `claude` agent**
  (`agent_session_id=01bd1a84-…`, `launched_pid=13244` confirmed running,
  transcript `…/wt-0001-wt-0001/01bd1a84-….jsonl`, `run_gate_state=running` — codex-dispatch
  disqualifier ruled out). The **SECOND** Ready issue (`#6`/`Task-0006`) **WAITED**: only
  `RepoA/wt-0001` existed (no `wt-0002`), only `Task-0005` dispatched; the next polls did
  not dispatch `#6` (empty pool defers, no auto-create).
- **(e) Close → freed worktree REUSED for the second.** Human-approved `gh issue close #5`
  → `15:02:14 poll acted: dispatched [Task-0006] parked [] reclaimed [Task-0005]`. In ONE
  poll the consumer reclaimed `Task-0005`'s worktree (returned to idle, **checkout kept** —
  the reclaim fix) and **reused the SAME `RepoA/wt-0001` member** to dispatch `Task-0006`
  (`taskrun--RepoA--Task-0006--active`, new `claude` agent `a2cd542d-…`, pid 61460). STILL
  only one member dir on disk — reused, not re-created.
- Proof: [reg007-live-v2/REG-007-LIVE-V2-RESULT.txt](./reg007-live-v2/REG-007-LIVE-V2-RESULT.txt),
  [reg007-live-v2/consumer-backend.log](./reg007-live-v2/consumer-backend.log),
  [reg007-live-v2/worktrees-reuse-task0006.json](./reg007-live-v2/worktrees-reuse-task0006.json),
  [reg007-live-v2/pool-record-reuse.json](./reg007-live-v2/pool-record-reuse.json),
  [reg007-live-v2/api-repos.json](./reg007-live-v2/api-repos.json),
  [reg007-live-v2/REPO-MANIFEST.json](./reg007-live-v2/REPO-MANIFEST.json), and the two
  launched-agent transcripts under `reg007-live-v2/claude-transcripts/`.

## Net verdict

- **REG-007: PASS** live, real-surface, end-to-end — the product-Create-seed path AND the
  close→reclaim→reuse leg both work after the BUG-0002 fix. The RUN-0001 core
  (real-UI flip → dispatch → claude launch → second-Ready-defers) is reconfirmed.
- REG-008, REG-009, live REG-015 (BUG-0001 fix): **PASS** stand from
  [REGRESSION-RUN-0001.md](./REGRESSION-RUN-0001.md) (not re-driven this run).
- [BUG-0002](../../BUG-0002.md): **FIXED + LIVE-VERIFIED**.
- NO production repo / lane / namespace / issue was touched.
