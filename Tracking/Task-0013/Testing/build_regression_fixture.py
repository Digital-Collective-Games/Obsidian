"""Build a task-owned isolated regression lane for Task-0013 REG-001 / REG-005.

Creates, under Tracking/Task-0013/Testing/Runtime/:
  - codex-fixture/.codex/sessions/.../*.jsonl   (real-shaped Codex token_count)
  - claude-fixture/.claude/projects/.../*.jsonl (real-shaped Claude assistant)
  - config.json pointing codex_root + claude_root at the fixtures and db_path at
    an isolated SQLite file

It reads NO live data (not C:\\Users\\gregs\\.codex, not ~/.claude, not the live
dashboard.db). Run with:
  python Tracking/Task-0013/Testing/build_regression_fixture.py
"""

from __future__ import annotations

import json
from datetime import UTC, datetime, timedelta
from pathlib import Path

RUNTIME = Path(__file__).resolve().parent / "Runtime"


def codex_line(ts: datetime, total: int, used_percent: float) -> bytes:
    payload = {
        "timestamp": ts.isoformat().replace("+00:00", "Z"),
        "type": "event_msg",
        "payload": {
            "type": "token_count",
            "info": {
                "total_token_usage": {"total_tokens": total * 10},
                "last_token_usage": {
                    "input_tokens": int(total * 0.6),
                    "cached_input_tokens": int(total * 0.2),
                    "output_tokens": int(total * 0.18),
                    "reasoning_output_tokens": int(total * 0.02),
                    "total_tokens": total,
                },
            },
            "rate_limits": {
                "secondary": {
                    "used_percent": used_percent,
                    "window_minutes": 10080,
                    "resets_at": int((ts + timedelta(days=3)).timestamp()),
                }
            },
        },
    }
    return (json.dumps(payload) + "\n").encode("utf-8")


def claude_assistant_line(request_id: str, ts: datetime, output: int) -> bytes:
    payload = {
        "type": "assistant",
        "requestId": request_id,
        "timestamp": ts.isoformat().replace("+00:00", "Z"),
        "message": {
            "usage": {
                "input_tokens": 5000,
                "cache_creation_input_tokens": 2000,
                "cache_read_input_tokens": 8000,
                "output_tokens": output,
            }
        },
    }
    return (json.dumps(payload) + "\n").encode("utf-8")


def main() -> None:
    RUNTIME.mkdir(parents=True, exist_ok=True)
    now = datetime.now(UTC)

    codex_root = RUNTIME / "codex-fixture" / ".codex"
    session_dir = codex_root / "sessions" / "2026" / "05" / "29"
    session_dir.mkdir(parents=True, exist_ok=True)
    codex_lines = []
    for i in range(60):
        ts = now - timedelta(hours=i * 2)
        codex_lines.append(codex_line(ts, 2_000_000 + i * 10_000, 30.0 + i * 0.2))
    (session_dir / "rollout-fixture.jsonl").write_bytes(b"".join(codex_lines))

    claude_root = RUNTIME / "claude-fixture" / ".claude"
    project_dir = claude_root / "projects" / "C--Agent-CodexDashboard"
    project_dir.mkdir(parents=True, exist_ok=True)
    claude_lines = []
    for i in range(40):
        ts = now - timedelta(hours=i * 3)
        request_id = f"req-{i:04d}"
        # two assistant lines per request (growing usage); last one wins
        claude_lines.append(claude_assistant_line(request_id, ts, 1000 + i * 10))
        claude_lines.append(
            claude_assistant_line(request_id, ts + timedelta(seconds=2), 5000 + i * 50)
        )
    (project_dir / "session-fixture.jsonl").write_bytes(b"".join(claude_lines))

    config = {
        "codex_root": str(codex_root),
        "claude_root": str(claude_root),
        "db_path": str(RUNTIME / "dashboard-regression.db"),
        "polling_seconds": 5,
        "weekly_budget_tokens": 4_000_000_000,
        "startup_enabled": False,
        "hotkey": "Ctrl+Alt+Space",
    }
    config_path = RUNTIME / "config.json"
    config_path.write_text(json.dumps(config, indent=2), encoding="utf-8")
    # Start from a clean DB so the smoke run reflects this fixture only.
    db_path = RUNTIME / "dashboard-regression.db"
    if db_path.exists():
        db_path.unlink()
    print(f"codex_root={codex_root}")
    print(f"claude_root={claude_root}")
    print(f"config={config_path}")
    print(f"db={db_path}")


if __name__ == "__main__":
    main()
