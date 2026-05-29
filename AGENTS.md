# CodexDashboard Repo Guide

## Project Goal

Build a Windows-first desktop utility that reads local Codex session telemetry and shows recent token velocity through a hotkey-first overlay.

Current implementation target:

- Python 3.13
- Tkinter desktop UI
- SQLite persistence
- Win32 integration through `ctypes`

## Documentation Split

Use the shared workflow from:

- `C:\Users\gregs\.codex\AGENTS.md`
- `C:\Users\gregs\.codex\Orchestration\ORCHESTRATION.md`

The directory-purpose split for shared orchestration homes such as `Reports`, `Processes`, and `Prompts` is defined in `C:\Users\gregs\.codex\AGENTS.md`.

Use repo-root docs here only for CodexDashboard-specific truth such as:

- implementation layout
- run commands
- regression lane definition
- persistent data, backup, restore, and lane data handling in `DATA-HANDLING.md`
- the canonical intervention packet output home under `C:\Users\gregs\.codex\Orchestration\Reports\Interventions\`
- the shared intervention packet process and prompt homes under `C:\Users\gregs\.codex\Orchestration\Processes\INTERVENTION-REPORTS.md` and `C:\Users\gregs\.codex\Orchestration\Prompts\Interventions\`

Keep task history and evidence under `Tracking/Task-<id>/`.
The explicit exception is:

- `C:\Users\gregs\.codex\Orchestration\Reports\Interventions\`
  - promoted canonical intervention packet outputs only

Task bootstrap contracts, day-pass artifacts, and pass history should still start under the owning task until promoted.

## Implementation Layout

- `app/codex_dashboard/`
  - Python package for config, storage, ingest, aggregation, and later UI
- `C:\Users\gregs\.codex\Orchestration\Reports\Interventions\`
  - promoted canonical intervention packet outputs
- `C:\Users\gregs\.codex\Orchestration\Processes\INTERVENTION-REPORTS.md`
  - shared intervention packet structure and promotion rules
- `C:\Users\gregs\.codex\Orchestration\Prompts\Interventions\`
  - shared intervention packet prompt bundles
- `tests/`
  - focused unit coverage
- `skills/obsidian-operator/`
  - repo-local Codex skill and bundled scripts for operating task/GitHub
    provider sync, reconcile, bootstrap, and conflict-resolution workflows
- `Tracking/Task-0001/`
  - task-owned artifacts for the current workstream

## Build And Verification Commands

From repo root:

```powershell
python -m unittest discover -s tests -p "test_*.py" -v
python -m app.codex_dashboard --scan-once --print-summary
```

## Guardrails

- do not infer token usage from file counts or file sizes
- parse real `token_count` events from `C:\Users\gregs\.codex\sessions\`
- keep the first version Windows-only and hotkey-first
- before changing task/GitHub provider sync, `TASK-META.json`, issue fields,
  bootstrap, reconcile, or text-conflict handling, use
  `skills/obsidian-operator/SKILL.md`
- treat repo-root `REGRESSION.md` as canonical for task-level regression in this repo
- treat repo-root `DATA-HANDLING.md` as canonical for persistent data classes, backup inventory, restore expectations, and backup-impact task obligations in this repo
- for agent-run validation and regression, treat the human's personal dashboard lane, config, database, and live Codex data as off-limits unless the human explicitly authorizes that run; task-closure regression must use the isolated lane/data flow in `TESTING.md`

## Jobs And Scheduling

- In this repo, if the human asks to create, change, repair, or verify a recurring job or scheduled task on this machine, default to the backend-controlled Temporal path that powers the dashboard.
- The expected implementation path is: update tracked v1 job specs under `C:\Users\gregs\.codex\Orchestration\Jobs\specs\`, reconcile through the orchestration backend, and verify the job appears in `GET /api/v1/jobs` and the dashboard Jobs tab.
- Do not satisfy those requests by creating or editing standalone Windows Scheduled Tasks, `declared-jobs.json`, or other legacy scheduler state unless the human explicitly asks for a legacy Windows-task path or the work is specifically backend bootstrap infrastructure such as `CodexDashboard-Orchestration-ServiceLane`.
- If legacy Windows scheduler state and backend or Temporal state disagree, treat the backend or Temporal path as the requested product behavior and call out the mismatch explicitly instead of silently using the legacy path.
