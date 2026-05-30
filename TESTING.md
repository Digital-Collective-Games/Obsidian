# Testing Guide

CodexDashboard follows the shared lifecycle and glossary from:

- `C:\Users\gregs\.codex\AGENTS.md`
- `C:\Users\gregs\.codex\Orchestration\ORCHESTRATION.md`
- `C:\Users\gregs\.codex\Orchestration\Processes\TESTING.md`

This repo-local file only defines CodexDashboard-specific commands and evidence expectations.
Persistent data classes, human-lane backup scope, and backup-impact task obligations are defined in [DATA-HANDLING.md](./DATA-HANDLING.md).

## Unit Test Commands

From repo root:

```powershell
python -m unittest discover -s tests -p "test_*.py" -v
```

Use unit coverage for:

- line parsing
- cursor handling
- dedupe
- interval aggregation
- weekly redline math

## Supporting Scripted Smoke

From repo root:

```powershell
python -m app.codex_dashboard --scan-once --print-summary
```

Use this to prove the ingest core can read real-shaped `.jsonl` telemetry and persist token events into SQLite.

## Canonical Task Fixture Repos

Backend task-run unit and smoke tests should exercise real git behavior through
small generated fixture repos, not by preserving old task worktrees.

Unit tests may dispatch `Task-0008` against a fake runtime. That validates the
repo readback, dispatch-readiness, owned-worktree bootstrap, snapshot capture,
and action-exposure contracts without contacting Temporal, launching Codex, or
touching the human's lane. Live Temporal dispatch belongs in an isolated
validation or task-specific regression lane.

The source-controlled fixture builder is:

```powershell
cd C:\Agent\CodexDashboard\backend\orchestration
powershell -ExecutionPolicy Bypass -File .\scripts\New-TaskFixtureRepo.ps1
```

The script creates a disposable git repo with representative `Task-0008` and
`Task-0009` durable task artifacts plus a short commit history for state-change
and rollback tests. It prints the repo root, commit ids, and a cleanup command.

Generated task fixture repos are not durable product state. Delete them after a
smoke run, for example:

```powershell
Remove-Item -Recurse -Force -LiteralPath <fixture-repo-root>
```

Do not back up generated fixture repos, clean owned task worktrees, validation
lane clones, or other rebuildable git checkouts as part of the canonical backup
set. Back up the source-controlled fixture builder, real committed task
artifacts, backend persistence that cannot be regenerated, and any active dirty
worktree that contains unique uncommitted work.

## Backend Lane Discipline

Do not use the human's personal dashboard lane, app config, database, or live
Codex data for disposable backend proof, debugging, or regression work. Treat
the human's lane as off-limits unless the human explicitly authorizes touching
it for the current run.

Do not equate `default lane` with `human lane`. A default backend lane can be a
persistent product service lane; the human lane is whatever lane, config,
database, and data the human is actively using. Regression for task closure must
run on a separate isolated validation or task-specific regression lane.

Known lanes:

| Lane | Purpose | Backend URL | Temporal | Postgres | Data root | Default use |
| --- | --- | --- | --- | --- | --- | --- |
| service | persistent backend service lane | `http://127.0.0.1:4318` | `127.0.0.1:7233` | `5432` | `%LOCALAPPDATA%\CodexDashboard\orchestration-service-lane` | not a regression lane; do not use for task-closure regression unless explicitly authorized |
| validation | disposable agent proof lane | `http://127.0.0.1:14318` | `127.0.0.1:17233` | `15432` | `%LOCALAPPDATA%\CodexDashboard\orchestration-validation-lane` | default backend lane for agent smoke, debugging, and regression |

Any additional reusable lane must be documented here before relying on it for
task proof. The documentation must name its ports, runtime/data roots, start and
stop commands, and how it stays isolated from the human's service lane.

Use the validation lane under `backend/orchestration/scripts/` for backend smoke, regression, or debugging:

```powershell
cd C:\Agent\CodexDashboard\backend\orchestration
powershell -ExecutionPolicy Bypass -File .\scripts\Start-ValidationLane.ps1
```

Validation-lane startup may build from the local checkout because the lane is
disposable. The service lane must not use that path. Human-lane startup is
release-pinned and is covered by [DATA-HANDLING.md](./DATA-HANDLING.md).

Focused unit coverage for service-lane release isolation lives in
`tests/test_service_lane_scripts.py` and must include:

- service runner path resolves under the lane runtime root, not the repo
- missing pinned release manifests fail closed
- release binary and compose-file hashes are validated
- tampered release artifacts are rejected

Focused unit coverage for dashboard frontend release isolation lives in
`tests/test_dashboard_release_scripts.py` and must include:

- release planning uses `%LOCALAPPDATA%\CodexDashboard\dashboard-releases`
  and the runtime launcher under
  `%LOCALAPPDATA%\CodexDashboard\dashboard-launcher`
- default release publish uses the committed Git tree, not untracked working-tree files
- startup registration points at the runtime launcher, not the repo checkout
- current release manifests validate all copied source hashes
- tampered release files are reported as hash mismatches

Startup-command unit coverage in `tests/test_desktop_support.py` must assert
that the dashboard startup file invokes the pinned launcher and does not launch
`C:\Agent\CodexDashboard` directly.

When a desktop run should target that validation lane instead of any persistent service lane:

```powershell
$env:CODEX_DASHBOARD_JOBS_BACKEND_URL = "http://127.0.0.1:14318"
python -m app.codex_dashboard
Remove-Item Env:CODEX_DASHBOARD_JOBS_BACKEND_URL -ErrorAction SilentlyContinue
```

Do not run desktop regression against the human's dashboard config or database.
Create a task-owned or temp config that points at fixture telemetry and an
isolated SQLite file, then launch the app with that config:

```powershell
$root = "C:\Agent\CodexDashboard\Tracking\Task-<id>\Testing\Runtime"
New-Item -ItemType Directory -Force $root | Out-Null
@{
  codex_root = "$root\codex-fixture"
  db_path = "$root\dashboard-regression.db"
  polling_seconds = 5
  weekly_budget_tokens = 4000000000
  startup_enabled = $false
  hotkey = "Ctrl+Alt+Space"
} | ConvertTo-Json | Set-Content -Encoding UTF8 "$root\config.json"
$env:CODEX_DASHBOARD_JOBS_BACKEND_URL = "http://127.0.0.1:14318"
python -m app.codex_dashboard --config-path "$root\config.json"
Remove-Item Env:CODEX_DASHBOARD_JOBS_BACKEND_URL -ErrorAction SilentlyContinue
```

The fixture `codex_root` must contain only synthetic or task-owned
real-shaped telemetry. Do not point regression at `C:\Users\gregs\.codex`, the
human's active Codex data, or the human's active dashboard database unless the
human explicitly authorizes that run.

Tear the validation lane down after the proof run:

```powershell
cd C:\Agent\CodexDashboard\backend\orchestration
powershell -ExecutionPolicy Bypass -File .\scripts\Stop-ValidationLane.ps1
```

## Issue-Provider Integration Testing (GitHub web surface)

Integration tests against an issue PROVIDER (e.g. the GitHub web interface) are
exercised at the **real human surface** — the GitHub web UI — via the Chrome
debug session, using the `github-operator` skill
([skills/github-operator/SKILL.md](./skills/github-operator/SKILL.md)).

The division of labor is fixed: the **human only authenticates** the debug
Chrome profile (started with
[Start-ChromeDebugProfile.ps1](../Orchestrator/Scripts/Start-ChromeDebugProfile.ps1)
on port `9222`), and the **agent drives the UI end-to-end** through the Chrome
DevTools Protocol.

There is **NO pass or excuse to skip exercising it end-to-end** at that surface.
API-only or proxy evidence does **not** satisfy these regressions. Reading state
back through the issue-field-values API is acceptable as verification, but the
WRITE/flip under test must happen at the real GitHub web UI via the
`github-operator` scripts (for example flipping an issue's `Queue` field to
`Ready` with
[Set-IssueFieldViaUi.ps1](./skills/github-operator/scripts/Set-IssueFieldViaUi.ps1)).

Writes are exercised only against throwaway/test issues (the `QueueDrainTestbed`
repo), never the human's production issues without explicit authorization.

## Regression Adapter

Task-level regression for this repo is defined in repo-root `REGRESSION.md`.

The regression lane must start the real desktop app surface and exercise the real overlay behavior. Parser-only proof does not satisfy regression closure by itself.

For `REG-004 Semantic Dashboard State Reconciliation`, the required artifact is
a visible-claim reconciliation matrix, not just a screenshot. Each checked UI
claim must cite the durable source that makes it true or false. Divergences must
open or update a task-owned `BUG-<NNNN>.md` before the run is considered
complete.

For new CodexDashboard feature work, tester coverage review must include the
owning task docs, task-local design docs, repo-root
`Design/GENERAL-DESIGN.md`, interface mockups or screenshots, and any prompt
deltas used as product references. If that review finds behavior or states not
named in repo-root `REGRESSION.md`, update the matrix or record the gap before
calling the feature adequately tested.
