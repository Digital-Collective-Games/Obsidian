"""Task-0013 Objective 4 source-filter before/after demonstration.

Loads the task-owned regression DB snapshot (built by build_regression_fixture.py
+ one ingest), then shows the displayed 7d total and bucket totals for each filter
selection: All, Codex-only, Claude-only, and None. This mirrors exactly what the
overlay's _render_dashboard does (filter the in-memory snapshot, then aggregate),
with NO additional database read per toggle — proving the filter operates on the
snapshot.

Run after building + ingesting the fixture:
  python Tracking/Task-0013/Testing/build_regression_fixture.py
  python -m app.codex_dashboard --config-path \
      Tracking/Task-0013/Testing/Runtime/config.json --scan-once
  python Tracking/Task-0013/Testing/source_filter_demo.py
"""

from __future__ import annotations

import json
import sys
from datetime import UTC, datetime, timedelta
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(REPO_ROOT))

from app.codex_dashboard.aggregation import (  # noqa: E402
    KNOWN_SOURCES,
    build_buckets,
    filter_events_by_source,
)
from app.codex_dashboard.storage import (  # noqa: E402
    connect,
    initialize_db,
    load_events_since,
)

RUNTIME = Path(__file__).resolve().parent / "Runtime"
DB = RUNTIME / "dashboard-regression.db"


def displayed_totals(events) -> dict:
    now = datetime.now(UTC)
    window_total = sum(
        e.total_tokens for e in events if e.event_timestamp >= now - timedelta(days=7)
    )
    buckets = build_buckets(events, "15m", bucket_count=20, now=now)
    return {
        "event_count": len(events),
        "7d_total_tokens": window_total,
        "bucket_total_tokens": sum(b.total_tokens for b in buckets),
    }


def main() -> int:
    # Single DB read (the snapshot the background ingest would hold in memory).
    connection = connect(DB)
    initialize_db(connection)
    snapshot = load_events_since(connection, datetime.now(UTC) - timedelta(days=7))
    connection.close()

    selections = {
        "All (codex+claude)": set(KNOWN_SOURCES),
        "Codex only": {"codex"},
        "Claude only": {"claude"},
        "None (unchecked all)": set(),
    }
    result = {
        "task": "Task-0013",
        "objective": "Objective 4 — source filter (Codex/Claude)",
        "measured_at": datetime.now(UTC).isoformat(),
        "snapshot_db": str(DB),
        "snapshot_loaded_once": True,
        "note": (
            "Each selection below is computed by filtering the SAME in-memory "
            "snapshot (filter_events_by_source) then aggregating. No per-toggle "
            "database read occurs, mirroring the overlay's _toggle_source path."
        ),
        "selections": {},
    }
    for label, selection in selections.items():
        filtered = filter_events_by_source(snapshot, selection)
        result["selections"][label] = displayed_totals(filtered)

    # Cross-check: All == Codex + Claude.
    all_total = result["selections"]["All (codex+claude)"]["7d_total_tokens"]
    codex_total = result["selections"]["Codex only"]["7d_total_tokens"]
    claude_total = result["selections"]["Claude only"]["7d_total_tokens"]
    result["merged_equals_sum_of_parts"] = all_total == codex_total + claude_total
    result["none_is_zero"] = (
        result["selections"]["None (unchecked all)"]["7d_total_tokens"] == 0
    )

    out = RUNTIME.parent / "SOURCE-FILTER-RESULT.json"
    out.write_text(json.dumps(result, indent=2), encoding="utf-8")
    for label, totals in result["selections"].items():
        print(f"{label:24} -> {totals}")
    print(f"merged_equals_sum_of_parts={result['merged_equals_sum_of_parts']}")
    print(f"none_is_zero={result['none_is_zero']}")
    print(f"Wrote {out}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
