# CodexDashboard Data Handling

Shared data-handling rules come from
`C:\Users\gregs\.codex\Orchestration\Processes\DATA-HANDLING.md`.

That shared process defines lane classes, backup classes, task backup-impact
obligations, and restore expectations. This repo-local file owns only the
CodexDashboard-specific inventory, lane names, paths, and exceptions.

## Product Name vs. Data Identifiers (Task-0013 Decision A)

The desktop product is named **Obsidian** (Task-0013 rebrand). The
`%LOCALAPPDATA%\CodexDashboard` data root and the matching OS identifiers
intentionally **keep the literal name `CodexDashboard`** for data continuity:

- data root: `%LOCALAPPDATA%\CodexDashboard` (holds `dashboard.db`, `config.json`)
- Startup script: `CodexDashboard.cmd`; launcher: `Start-CodexDashboard.ps1`
- scheduled task: `CodexDashboard-Orchestration-ServiceLane`
- Go orchestration runs root literal: `...\CodexDashboard\orchestration-runs`
- repo manifest id: `CodexDashboard` (`CODEX-REPO-MANIFEST.json`)

This is Decision A from `Tracking/Task-0013/PLAN.md`: rename display strings only,
do not rename these identifiers. Renaming the data root or OS identifiers without
a flawless one-time migration would orphan the human's existing `dashboard.db`,
`config.json`, and enabled startup entry. The product can read "Obsidian" while
the data path stays "CodexDashboard"; the two are decoupled on purpose. If a full
rename is ever adopted (Decision B), it must ship a tested, idempotent startup
migration that moves the existing data and re-points the startup/scheduled-task
artifacts, and is a no-op on a fresh install.

## Human Lane

The human lane is the data and services the human actually uses. It is
default-off-limits for agent testing, smoke, regression, destructive repair, and
deployment experimentation unless the human explicitly authorizes that specific
operation.

All statements about the human lane must follow the shared evidence-bound claims
rule in `C:\Users\gregs\.codex\Orchestration\Processes\DATA-HANDLING.md`.

In this repo, do not claim the human lane is `safe`, `upgraded`, `deployed`,
`backed up`, `restored`, or `preserved` unless the proof for that exact claim is
provided. Health checks, command exit codes, and script names are not sufficient
evidence for stronger claims.

Human-lane reports should state:

- the claim
- the proof checked
- any remaining unknowns

## Human Lane Release Discipline

The service lane must not rebuild from the mutable repo checkout on scheduled
task restart. Human-lane code changes require an explicit service-lane release:

```powershell
cd C:\Agent\CodexDashboard\backend\orchestration
powershell -ExecutionPolicy Bypass -File .\scripts\Publish-ServiceLaneRelease.ps1
```

That release writes a pinned manifest under the service-lane runtime root and
copies the scheduled-task launcher into the runtime root. The service runner
must launch the binary and compose file named in that manifest, with matching
hashes, instead of building from `C:\Agent\CodexDashboard` at restart time.

Use this read-only proof command before making any claim about human-lane
release isolation:

```powershell
cd C:\Agent\CodexDashboard\backend\orchestration
powershell -ExecutionPolicy Bypass -File .\scripts\Test-ServiceLaneIsolation.ps1
```

`Start-ServiceLane.ps1` and `Install-ServiceLane.ps1` fail closed when the
service lane has no pinned release or when the scheduled task still points at a
repo-local launcher.

Human-lane promotion sequence:

1. Confirm the human has authorized touching the lane for this specific run.
2. Run unit/script validation before promotion.
3. Publish a pinned release with `Publish-ServiceLaneRelease.ps1`. If the repo
   is dirty and the human still wants that exact state promoted, use
   `-AllowDirty` and treat `source_dirty = true` in the manifest as part of the
   evidence.
4. Restart through `Stop-ServiceLane.ps1` and `Install-ServiceLane.ps1` so the
   scheduled task is rewritten to the runtime-root launcher.
5. Run `Test-ServiceLaneIsolation.ps1` and `Get-ServiceLaneStatus.ps1`.
6. Claim only what the proof shows: release id, git commit, dirty-source flag,
   scheduled-task launcher path, running process path, health, and any unknowns.

The desktop dashboard frontend follows the same release discipline. The startup
entry must invoke the runtime-root dashboard launcher, not `pythonw -m
app.codex_dashboard` from `C:\Agent\CodexDashboard`.

By default, dashboard frontend releases copy the committed Git tree at `HEAD`,
not uncommitted working-tree files. The manifest records both `source_mode` and
whether the surrounding repository was dirty at publish time. Use
`-FromWorkingTree -AllowDirty` only when the human explicitly asks to promote an
uncommitted frontend state.

Publish the frontend release from repo root:

```powershell
cd C:\Agent\CodexDashboard
powershell -ExecutionPolicy Bypass -File .\scripts\Publish-DashboardRelease.ps1
```

Use this read-only proof command before claiming the visible dashboard frontend
is pinned or current:

```powershell
cd C:\Agent\CodexDashboard
powershell -ExecutionPolicy Bypass -File .\scripts\Test-DashboardRelease.ps1
```

Frontend claims require frontend proof. Backend release isolation does not prove
the dashboard process, startup entry, or visible tab surface.

## Backup Scripts

Estimate the current human-lane backup set:

```powershell
cd C:\Agent\CodexDashboard\backend\orchestration
powershell -ExecutionPolicy Bypass -File .\scripts\Get-HumanLaneBackupPlan.ps1
```

Create a human-lane backup:

```powershell
cd C:\Agent\CodexDashboard\backend\orchestration
powershell -ExecutionPolicy Bypass -File .\scripts\New-HumanLaneBackup.ps1 -DestinationRoot D:\CodexDashboardBackups
```

By default, `New-HumanLaneBackup.ps1` uses `pg_dumpall` through Docker Compose
for the service-lane Temporal/Postgres database, archives the must-backup
filesystem roots, exports the service-lane scheduled task, and records the repo
commit plus uncommitted/untracked delta. Add `-IncludeRepoBundle` when upstream
git should not be treated as sufficient for repo recovery.

Use `-PlanOnly` to print the backup plan without writing a backup:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\New-HumanLaneBackup.ps1 -PlanOnly
```

Current human-lane must-backup set:

| Data | Current location | Backup class | Notes |
| --- | --- | --- | --- |
| Temporal/Postgres state | Docker volume `codex-orchestration-service_temporal-postgres-data` | must backup | Contains service-lane Temporal state. Use a coherent DB dump or stopped-lane volume snapshot. |
| service-lane runtime root | `%LOCALAPPDATA%\CodexDashboard\orchestration-service-lane` | must backup | Includes service-lane binary/log layout and local runtime metadata. |
| service-lane pinned release manifest and releases | `%LOCALAPPDATA%\CodexDashboard\orchestration-service-lane\current-release.json` and `%LOCALAPPDATA%\CodexDashboard\orchestration-service-lane\releases\` | must backup | Defines the exact human-lane binary and compose file that restarts are allowed to launch. |
| service-lane run artifacts | `%LOCALAPPDATA%\CodexDashboard\orchestration-runs\service-lane` | must backup | Small and useful for recovery/postmortem. |
| dashboard frontend pinned releases | `%LOCALAPPDATA%\CodexDashboard\dashboard-current-release.json`, `%LOCALAPPDATA%\CodexDashboard\dashboard-releases\`, and `%LOCALAPPDATA%\CodexDashboard\dashboard-launcher\` | must backup | Defines the exact human-facing dashboard source tree and launcher that restarts are allowed to load. |
| dashboard config | `%LOCALAPPDATA%\CodexDashboard\config.json` | must backup | Human app settings. |
| dashboard SQLite DB | `%LOCALAPPDATA%\CodexDashboard\dashboard.db` | must backup | Kept in backup even if parts may be reconstructable from session telemetry. |
| job specs | `C:\Users\gregs\.codex\Orchestration\Jobs\specs\` | must backup | Durable desired job declarations. |
| repo binding manifest | `C:\Agent\CodexDashboard\CODEX-REPO-MANIFEST.json` | must backup delta | Tracked manifest for repo-local task, proposal, backlog, and review-surface bindings. Restored with repo state. |
| repo state delta | `C:\Agent\CodexDashboard` plus upstream git | must backup delta | If upstream is current, record commit/branch and back up only unpushed commits, uncommitted diffs, and important untracked files. |

## Service Lane

The service lane is the persistent backend lane:

- backend URL: `http://127.0.0.1:4318`
- Temporal: `127.0.0.1:7233`
- Postgres: `5432`
- runtime root: `%LOCALAPPDATA%\CodexDashboard\orchestration-service-lane`
- runs root: `%LOCALAPPDATA%\CodexDashboard\orchestration-runs\service-lane`
- scheduled task: `CodexDashboard-Orchestration-ServiceLane`

Treat it as production-like state for this repo. Do not use it for task-closure
regression unless the human explicitly authorizes that run.

## Validation Lanes

Validation lanes are disposable agent/test lanes unless a task explicitly
documents otherwise.

The default validation lane is documented in [TESTING.md](./TESTING.md). It is
not part of the human-lane disaster backup set.

Known disposable validation or manual validation roots currently include:

- `%LOCALAPPDATA%\CodexDashboard\orchestration-validation-lane`
- `%LOCALAPPDATA%\CodexDashboard\orchestration-validation-manual`
- `%LOCALAPPDATA%\CodexDashboard\orchestration-validation-manual-15318`
- `%LOCALAPPDATA%\CodexDashboard\orchestration-runs\validation-lane`
- `%LOCALAPPDATA%\CodexDashboard\orchestration-runs-validation`

## Disposable Data

Do not back up these by default:

- generated fixture repos
- clean owned task worktrees
- validation-lane clones and generated data
- `%TEMP%\cdxow`
- Docker images that can be pulled again
- rebuilt binaries under lane `bin` directories
- Python/Go caches and other generated build artifacts

Back up a disposable-looking path only if it currently contains unique
uncommitted work, task-owned evidence, or an active investigation artifact.

## Task Closeout

Any CodexDashboard task that touches persistent data must follow the shared
backup-impact labels from
`C:\Users\gregs\.codex\Orchestration\Processes\DATA-HANDLING.md`.

Update this file before closure when a task changes:

- service-lane or validation-lane roots, ports, or scheduled-task names
- Docker volumes or database files
- dashboard config or SQLite DB behavior
- job spec locations
- task state storage
- restore requirements
- whether a CodexDashboard path is must-backup, conditional, or disposable

## Restore Notes

A CodexDashboard human-lane restore should restore or verify:

- repo commit/branch plus any unpushed or uncommitted delta
- Docker volume or DB dump for `codex-orchestration-service_temporal-postgres-data`
- `%LOCALAPPDATA%\CodexDashboard\orchestration-service-lane`
- `%LOCALAPPDATA%\CodexDashboard\orchestration-runs\service-lane`
- `%LOCALAPPDATA%\CodexDashboard\dashboard-current-release.json`
- `%LOCALAPPDATA%\CodexDashboard\dashboard-releases\`
- `%LOCALAPPDATA%\CodexDashboard\dashboard-launcher\`
- `%LOCALAPPDATA%\CodexDashboard\config.json`
- `%LOCALAPPDATA%\CodexDashboard\dashboard.db`
- `C:\Users\gregs\.codex\Orchestration\Jobs\specs\`
- `C:\Agent\CodexDashboard\CODEX-REPO-MANIFEST.json`
- scheduled task `CodexDashboard-Orchestration-ServiceLane`, if it is not recreated by the service-lane installer

Validation/temp data should be intentionally discarded unless a task records a
specific exception.
