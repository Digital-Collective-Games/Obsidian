from __future__ import annotations

from pathlib import Path


def repo_root() -> Path:
    return Path(__file__).resolve().parents[2]


def default_codex_root() -> Path:
    return Path.home() / ".codex"


def default_claude_root() -> Path:
    # Task-0013 Objective 2: Claude Code stores per-message token usage under
    # ~/.claude/projects/<encoded-cwd>/*.jsonl.
    return Path.home() / ".claude"


def orchestration_root(codex_root: Path | None = None) -> Path:
    return (codex_root or default_codex_root()) / "Orchestration"


def jobs_root(codex_root: Path | None = None) -> Path:
    return orchestration_root(codex_root) / "Jobs"


def legacy_jobs_registry_path(codex_root: Path | None = None) -> Path:
    return orchestration_root(codex_root) / "codex-jobs-registry.json"


def default_jobs_registry_path(codex_root: Path | None = None) -> Path:
    return jobs_root(codex_root) / "declared-jobs.json"


def default_jobs_schema_path(codex_root: Path | None = None) -> Path:
    return jobs_root(codex_root) / "declared-jobs.schema.json"


def job_specs_root(codex_root: Path | None = None) -> Path:
    return jobs_root(codex_root) / "specs"


def job_spec_schema_path(codex_root: Path | None = None) -> Path:
    return jobs_root(codex_root) / "job-spec.schema.json"


def app_data_root() -> Path:
    # Task-0013 rebrand Decision A: the product name is "Obsidian" but this
    # %LOCALAPPDATA%\CodexDashboard data root (and the matching OS identifiers:
    # startup script name, launcher name, scheduled-task name, Go runs root,
    # REPO-MANIFEST.json "id") intentionally stay "CodexDashboard" for
    # data continuity. Renaming this path without a flawless migration would
    # orphan the human's existing dashboard.db / config.json / startup entry.
    # See DATA-HANDLING.md and Tracking/Task-0013/PLAN.md.
    return Path.home() / "AppData" / "Local" / "CodexDashboard"


def default_db_path() -> Path:
    return app_data_root() / "dashboard.db"


def default_config_path() -> Path:
    return app_data_root() / "config.json"


def default_investigations_path() -> Path:
    return repo_root() / "Tracking" / "Investigations"
