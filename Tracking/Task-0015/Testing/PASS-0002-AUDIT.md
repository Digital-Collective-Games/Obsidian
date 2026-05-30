# PASS-0002 Audit — O6: Worktree↔session binding + enumeration endpoint

Task: Task-0015. Pass: PASS-0002 (O6). Verdict: **READY** (O6 PASS, committed).
Scope: binding SCHEMA + endpoint + operator command (the PASS-0002 portion of O6).

## Independence

Implementer and QA were SEPARATE clean-context subagents. QA did NOT trust saved
evidence — it re-ran `go build`/`go test` and **re-derived the live endpoint proof
from scratch** with fresh task ids (Task-12/13, avoiding the stale validation
Temporal records), then cleaned up.

## Per-criterion verdict (independent re-derivation)

- **A6.1 (HARD) PASS** — after a fresh dispatch, the owned-lane record
  (`owned-lane-bootstrap.json`) and `RepoLane.Binding` carry all six fields
  populated: `repo` (resolved from `REPO-MANIFEST.json`), `task_id`,
  `worktree_path` (= `owned_repo_root`), `agent_session_id`,
  `session_transcript_path`, `run_gate_state=running`. The session/transcript
  values are dispatch-context (`captureDispatchContext`) placeholders, clearly
  labeled in `types.go` — not a fabricated real-agent value.
- **A6.2 (HARD) PASS** — `GET /api/v1/worktrees` → 200 with each active worktree's
  full binding; body contains NO `vscodium://` string (orchestrator boundary
  holds). Empty before dispatch.
- **A6.3 PASS** — `Get-ActiveWorktreeSessions.ps1 -BaseUrl http://127.0.0.1:14318`
  prints per-worktree Task/Issue, Repo, Run/Gate, Worktree, Session ID, Transcript.
- **AX.1 PASS** — `go build ./...` exit 0; `go test -count=1 ./...` all `ok`
  (incl. new `internal/httpapi/worktrees_test.go` + `internal/taskrun/binding_test.go`).
- **F-O6 NOT triggered** — endpoint enumerates active worktrees with session +
  transcript + run/gate state; binding is durable on the owned-lane record; an
  obsidian-operator command surfaces it.

## Honest deferrals (verified NOT faked)

- **A6.4** (parked needs-human worktree listed) → **PASS-0003 (O4)**: park state
  does not exist yet. `ListActiveWorktrees` returns whatever `run_gate_state` the
  record carries (all `running` today); it will list parked worktrees unchanged
  once O4 records them. No fake parked state injected.
- **A6.1 real launched-agent session** → **PASS-0005 (O5)**: the session/transcript
  fields are dispatch-context placeholders, explicitly marked. No pretend agent.

## Genuineness (file:line)

- `RepoBinding` + `RunGateStateRunning` — `internal/taskrun/types.go:144-175`;
  persisted via `ownedLaneBootstrapRecord.Binding` + `RepoLane.Binding`; populated
  by `bindingForLane`/`repoIdentity` at `service.go:1216-1252`. Repo id from
  `RepoManifest.RepoIDForRoot` (`internal/queue/manifest.go`).
- Endpoint `handleWorktreesList` — `internal/httpapi/mux.go:113-133` (route at
  `:39`), backed by `Service.ListActiveWorktrees` (`service.go:151-216`); active =
  checkout dir still on disk, so a reclaimed lane drops out. Tests substantive
  (assert each field, reclaimed-worktree drop-out, vscodium-absence guard, 405).

## Isolation, cleanup, churn

- Validation lane only (bind :14318, Temporal :17233) against throwaway
  `QueueDrainTestbed`; live lane (:4318/:7233/:5432) + live CodexDashboard worktree
  untouched; QA cleaned up (backend stopped, worktrees pruned, fixtures removed).
- Churn: additive; the only non-`-w` delta is a gofmt re-alignment of the
  `RepoLane` struct in `types.go` (whitespace-only, `git diff -w --numstat` = 35/0).
  No O3/O4/O5 scope creep (no park/close/launcher/watchdog code).

## Carry-forward note

QA observed that with `CODEX_ORCHESTRATION_JOBS_ROOT` unset, the backend's
`/healthz` read-reconciles the real `.codex/Orchestration/Jobs` (read-side only,
no writes). Future live proofs should also override `CODEX_ORCHESTRATION_JOBS_ROOT`
to a temp dir for full isolation. Harmless to O6; not a defect.
