# PASS-0003 Audit — O4: Done-contract (park-in-place, human-only closure)

Task: Task-0015. Pass: PASS-0003 (O4). Verdict: **READY** (O4 mechanism PASS, committed).
Scope: the done-contract MECHANISM + unit/dry-run proof + the O6 A6.4 parked-listing
re-proof. Integrated consumer-driven (O3) and agent-driven (O5) end-to-end A4 proofs
are honestly DEFERRED (see below).

## Independence

Implementer and QA were SEPARATE clean-context subagents. QA re-ran `go build`/`go
test` itself, re-ran the skill dry-runs with a fake `gh` tripwire on PATH
(confirmed `gh` was NEVER invoked), independently grepped for any backend GitHub
write, and grep-confirmed that no consumer loop / agent launcher was built.

## Per-criterion verdict (independent)

- **A4.5 PROVEN** — `EffectiveFreeConcurrency(limit, parked) = limit − parked`,
  clamped ≥0, default-on-nonpositive-limit (`internal/queue/decision.go`) + table test.
- **A4.6 PROVEN** — independent grep of `backend/orchestration` for
  `api.github.com|gh issue|issue-field-values|gh api|github.com/repos` → no matches;
  every `github.com` string is a Go import. All GitHub writes route through the
  skill's single `Sync-TaskToGitHubIssue.ps1 -HumanNeededValue` path.
  **F-O4-source not triggered**: `DecideQueueAction` reads `IssueState{Closed,
  HumanNeeded, Queue}` (the issue), not a local enum.
- **A4.1/A4.2/A4.3/A4.4/A4.7 — MECHANISM PROVEN now; integrated end-to-end DEFERRED
  (honest, not faked).** The pure decision (`closed⇒terminal` precedence even over
  Queue=Ready; `Human Needed=Yes⇒park` even over Ready, never redispatch;
  open+Ready+not-parked⇒dispatch), `SetRunGateState` (parks without deallocating,
  persists to `owned-lane-bootstrap.json`, rejects unknown states), and the skill
  never-self-close contract are proven by re-run unit tests + dry-run. The
  consumer-loop proofs (A4.3/A4.4 in-loop) are DEFERRED to PASS-0004 (O3); the
  dispatched-agent proofs (A4.1 abandon, A4.2 perceived-completion park, A4.7 never
  self-close) are DEFERRED to PASS-0005 (O5). QA grep-confirmed
  `DecideQueueAction`/`SetRunGateState`/`EffectiveFreeConcurrency` are NOT yet wired
  into `temporalbackend`/`taskexec`/`mux.go` and `internal/queue` has no polling loop
  or launcher — so the deferral is real.
- **O6 A6.4 PROVEN now** at both layers: `TestSetRunGateStateParksAndStaysListed`
  (service `ListActiveWorktrees`) and `TestWorktreesEndpointListsParkedWorktree`
  (httptest against the real mux — a dispatched lane parked via `SetRunGateState`
  is still returned by `GET /api/v1/worktrees` with `parked_awaiting_closure`).
- **AX.1 PASS** — `go build ./...` exit 0; `go test -count=1 ./...` all `ok`; gofmt
  + `go vet` clean.

## Skill done-contract (verified)

`Set-TaskDoneContract.ps1`: on BOTH `-Outcome abandon` and `-Outcome completion`
sets `Human Needed=Yes` (completion ⇒ `parked_awaiting_closure`) via the existing
Sync write path; contains NO executable `gh issue close` (only a comment). The
human close remains the separate, human-gated `Reconcile-TaskGitHubState.ps1`
builder (unmodified). `SKILL.md` "Queue Done-Contract" accurately documents: agent
never self-closes; closure is a distinct, final human gate (pass/regression/plan/
research ≠ closure); `Human Needed=Yes` (incl. awaiting-closure) parks and retains
worktree+slot; only the human-approved close deallocates.

## Scope / churn / isolation

No scope creep (no O3 consumer loop, no O5 launcher, no backend GitHub write).
Churn surgical — raw `git diff --numstat` == `-w` for `service.go` (81/4) and
`types.go` (35/5), gofmt-clean. No live lane (:4318/:7233/:5432), live CodexDashboard
worktree, or live GitHub touched; the dry-run used a throwaway fixture (deleted),
`gh` never invoked.

## Note

A6.4 was proven via `httptest` against the real `NewMux` rather than booting the
control-plane lane (the `/api/v1/worktrees` handler only reads `ListActiveWorktrees`
from disk, so the httptest exercises the identical code the live lane serves).
