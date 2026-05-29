# Task-0013 TaskCreate Context Manifest

Durable files the writer-worker may read to draft `TASK.md`. Paths are relative to
the repo root `c:\Agent\CodexDashboard` unless absolute. These are pointers from a
codebase map; the writer should read what it needs and may read neighbors.

## Task-owned coordination artifacts

- [HUMAN-DIRECTIVES-FOR-WORKER.md](./HUMAN-DIRECTIVES-FOR-WORKER.md) — authoritative human scope + auditor-override directive
- [TASK-CREATE-OBJECTIVE.md](./TASK-CREATE-OBJECTIVE.md) — objective and known seams

## Rebrand surface ("CodexDashboard" → "Obsidian")

- `app/codex_dashboard/__init__.py` (line 1: `APP_NAME = "CodexDashboard"`)
- `app/codex_dashboard/ui.py` (line 350 window title; line 567 overlay header label)
- `app/codex_dashboard/__main__.py` (line 16 CLI help)
- `app/codex_dashboard/jobs.py` (line 173 job label)
- `app/codex_dashboard/investigation.py` (line 552 report title)
- `app/codex_dashboard/paths.py` (line 43 `%LOCALAPPDATA%\CodexDashboard` root — DATA MIGRATION RISK)
- `app/codex_dashboard/startup.py` (lines 20, 34 startup artifact names)
- `backend/orchestration/internal/config/config.go` (lines 63, 65 path literal)
- `CODEX-REPO-MANIFEST.json` (line 6 `"id": "CodexDashboard"`)
- `README.md` (line 1 title)
- `DATA-HANDLING.md` (persistent paths + scheduled task name `CodexDashboard-Orchestration-ServiceLane`)
- `AGENTS.md` (documented config/data paths)

## Token sourcing / Claude merge

- `app/codex_dashboard/scanner.py` (`session_jsonl_files` 20-24; `parse_token_event` 32-80; `ingest_once` 141-194; `backfill_session_context_markers` 110-139)
- `app/codex_dashboard/storage.py` (`initialize_db` 17-59; `token_events` schema 29-45; unique key line 44)
- `app/codex_dashboard/models.py` (`TokenEvent` dataclass 7-21)
- `app/codex_dashboard/aggregation.py` (`event_metric_tokens` 46-58; `build_buckets` 61-93)
- `app/codex_dashboard/config.py` (`DashboardConfig` 14-28; `load_config` 59-76)
- `app/codex_dashboard/token_time.py` (CSV augmentation + working-time buckets)
- `scripts/add_total_time_to_token_usage_csvs.py` (CSV default paths)
- Tracking/`token-usage-by-task-since-2026-03-26.csv` (+ `.sessions.csv`) — sample token data shape
- External Claude data: `C:\Users\gregs\.claude` (Claude Code session/project transcripts; confirm the actual session/token JSON shape before designing the parser)

## Overlay-activation / hotkey performance

- `app/codex_dashboard/hotkey.py` (`parse_hotkey` 38-69; `GlobalHotkey` 72-157; message loop 104-138; `poll` 140-146)
- `app/codex_dashboard/config.py` (line 21 default hotkey `Ctrl+Alt+Space` — note mismatch with human-reported `Ctrl+Alt+Shift`)
- `app/codex_dashboard/ui.py` (line 365 hotkey registration; `_poll_hotkey` 1611-1614; `toggle_overlay` 2227-2233; `show_overlay` 2235-2240; `refresh_data` 1671-1749 synchronous on UI thread; `schedule_ingest` 1641-1669)
- The user's actual config: `%LOCALAPPDATA%\CodexDashboard\config.json` (to confirm the real bound hotkey)

## Architecture / tests

- Entry: `app/codex_dashboard/__main__.py` (`main` 74-108); run UI via `python -m app.codex_dashboard`
- DB: SQLite at `%LOCALAPPDATA%\CodexDashboard\dashboard.db` (`paths.py:default_db_path`)
- Tests: `tests/test_*.py` (e.g. `test_ingest_core.py`, `test_token_time.py`); run `python -m unittest discover -s tests -p "test_*.py" -v`
