from __future__ import annotations

import argparse
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
if str(REPO_ROOT) not in sys.path:
    sys.path.insert(0, str(REPO_ROOT))

from app.codex_dashboard.token_time import add_total_time_to_token_usage_csvs


def main() -> int:
    parser = argparse.ArgumentParser(
        description=(
            "Add or refresh total_time columns in token usage aggregate and session CSVs. "
            "total_time is minutes in occupied 10-minute chat buckets. "
            "Aggregate task titles are prefixed with TASK-0000 style ids."
        )
    )
    parser.add_argument(
        "--aggregate-csv",
        type=Path,
        default=Path("Tracking/token-usage-by-task-since-2026-03-26.csv"),
    )
    parser.add_argument(
        "--sessions-csv",
        type=Path,
        default=Path("Tracking/token-usage-by-task-since-2026-03-26.sessions.csv"),
    )
    args = parser.parse_args()

    summary = add_total_time_to_token_usage_csvs(args.aggregate_csv, args.sessions_csv)
    print(
        "Updated token usage CSVs: "
        f"{summary['aggregate_rows']} aggregate rows, "
        f"{summary['session_rows']} session rows, "
        f"{summary['missing_session_files']} missing session files."
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
