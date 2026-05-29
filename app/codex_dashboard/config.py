from __future__ import annotations

import json
from dataclasses import asdict, dataclass
from pathlib import Path

from .paths import (
    default_claude_root,
    default_codex_root,
    default_config_path,
    default_db_path,
)

DEFAULT_WEEKLY_BUDGET_TOKENS = 4_000_000_000
LEGACY_WEEKLY_BUDGET_TOKENS = 8_000_000
ADVISORY_BUDGET_ROUNDING_TOKENS = 50_000_000


@dataclass(slots=True)
class DashboardConfig:
    codex_root: str
    db_path: str
    polling_seconds: int = 5
    weekly_budget_tokens: int = DEFAULT_WEEKLY_BUDGET_TOKENS
    startup_enabled: bool = False
    hotkey: str = "Ctrl+Alt+Space"
    # Task-0013 Objective 2: second ingest source. Empty string means "no Claude
    # source" and ingest skips it cleanly.
    claude_root: str = ""

    @classmethod
    def defaults(cls) -> "DashboardConfig":
        return cls(
            codex_root=str(default_codex_root()),
            db_path=str(default_db_path()),
            claude_root=str(default_claude_root()),
        )


def advisory_implied_weekly_budget_tokens(
    total_7d_tokens: int,
    weekly_used_percent: float | None,
) -> int | None:
    if total_7d_tokens <= 0 or weekly_used_percent is None or weekly_used_percent <= 0:
        return None
    implied_budget = int(round(total_7d_tokens / (weekly_used_percent / 100.0)))
    rounded_budget = int(
        round(implied_budget / ADVISORY_BUDGET_ROUNDING_TOKENS)
        * ADVISORY_BUDGET_ROUNDING_TOKENS
    )
    return max(ADVISORY_BUDGET_ROUNDING_TOKENS, rounded_budget)


def maybe_upgrade_weekly_budget(
    config: "DashboardConfig",
    total_7d_tokens: int,
    weekly_used_percent: float | None,
) -> bool:
    if config.weekly_budget_tokens != LEGACY_WEEKLY_BUDGET_TOKENS:
        return False
    config.weekly_budget_tokens = (
        advisory_implied_weekly_budget_tokens(total_7d_tokens, weekly_used_percent)
        or DEFAULT_WEEKLY_BUDGET_TOKENS
    )
    return True


def load_config(path: Path | None = None) -> DashboardConfig:
    config_path = path or default_config_path()
    if not config_path.exists():
        config = DashboardConfig.defaults()
        save_config(config, config_path)
        return config
    payload = json.loads(config_path.read_text(encoding="utf-8"))
    defaults = DashboardConfig.defaults()
    return DashboardConfig(
        codex_root=str(payload.get("codex_root", defaults.codex_root)),
        db_path=str(payload.get("db_path", defaults.db_path)),
        polling_seconds=int(payload.get("polling_seconds", defaults.polling_seconds)),
        weekly_budget_tokens=int(
            payload.get("weekly_budget_tokens", defaults.weekly_budget_tokens)
        ),
        startup_enabled=bool(payload.get("startup_enabled", defaults.startup_enabled)),
        hotkey=str(payload.get("hotkey", defaults.hotkey)),
        claude_root=str(payload.get("claude_root", defaults.claude_root)),
    )


def save_config(config: DashboardConfig, path: Path | None = None) -> None:
    config_path = path or default_config_path()
    config_path.parent.mkdir(parents=True, exist_ok=True)
    config_path.write_text(
        json.dumps(asdict(config), indent=2, sort_keys=True),
        encoding="utf-8",
    )
