"""Task-0013 Objective 3 FOLLOW-UP: end-to-end activation latency harness.

The prior `activation_timing_harness.py` measured only the Tk UI-thread blocking
time of the synchronous DB read (~5 ms after the fix). The human reports the
PERCEIVED time from key press to a fully rendered, painted window is ~500 ms and
still feels clunky. That perceived latency is NOT the UI-thread DB block: it is

    poll latency  +  deiconify/lift/focus  +  window map  +  _render_dashboard
                  +  the OS actually compositing/painting the window.

This harness drives the REAL Tk DashboardApp in-process against a task-owned
SYNTHETIC SQLite database sized comparably to the human's large live DB, and
times each phase of the activation pipeline with perf_counter markers around the
real production methods (show_overlay -> deiconify/lift/focus -> _render_dashboard
-> a forced update()/update_idletasks() that makes the window actually paint).

It does NOT open or read the human's live dashboard.db, live config,
C:\\Users\\gregs\\.codex, or ~/.claude. It builds and measures its own DB.

Usage (headed; a real Tk window will flash on screen):
  python Tracking/Task-0013/Testing/activation_e2e_harness.py \
      --db Tracking/Task-0013/Testing/Runtime/e2e-timing.db \
      --events 1400000 \
      --out Tracking/Task-0013/Testing/E2E-TIMING-RESULT.json
"""

from __future__ import annotations

import argparse
import json
import sqlite3
import time
from datetime import UTC, datetime, timedelta
from pathlib import Path

import sys

REPO_ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(REPO_ROOT))

from app.codex_dashboard.storage import (  # noqa: E402
    connect,
    count_events,
    initialize_db,
    load_events_since,
)


def seed_db(db_path: Path, event_count: int) -> None:
    """Seed a synthetic DB across 21 days, 50/50 codex/claude.

    Mirrors the seeding shape of the original harness so the two are comparable,
    but lets the caller request a much larger event_count to reach a ~270 MB DB.
    """
    db_path.parent.mkdir(parents=True, exist_ok=True)
    if db_path.exists():
        db_path.unlink()
        for suffix in ("-wal", "-shm"):
            extra = db_path.with_name(db_path.name + suffix)
            if extra.exists():
                extra.unlink()
    connection = connect(db_path)
    initialize_db(connection)
    now = datetime.now(UTC)
    span_seconds = 21 * 24 * 3600
    step = max(1, span_seconds // max(1, event_count))
    rows = []
    for i in range(event_count):
        ts = now - timedelta(seconds=span_seconds - i * step)
        source = "claude" if i % 2 == 0 else "codex"
        rows.append(
            (
                f"sess-{i % 4000}",
                i,
                ts.isoformat(),
                1000 + (i % 5000),
                500,
                100,
                300,
                10,
                1000 + (i % 5000),
                30.0 + (i % 50) * 0.1,
                10080,
                None,
                "{}",
                source,
                f"{source}-{i}",
            )
        )
        if len(rows) >= 20000:
            _insert_batch(connection, rows)
            rows = []
    if rows:
        _insert_batch(connection, rows)
    connection.commit()
    connection.close()


def _insert_batch(connection: sqlite3.Connection, rows: list) -> None:
    connection.executemany(
        """
        INSERT OR IGNORE INTO token_events(
            session_path, line_offset, event_timestamp, total_tokens,
            input_tokens, cached_input_tokens, output_tokens,
            reasoning_output_tokens, cumulative_total_tokens,
            weekly_used_percent, weekly_window_minutes, weekly_resets_at,
            raw_json, source, source_event_id
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        """,
        rows,
    )


def _write_config(runtime: Path, db_path: Path) -> Path:
    """Write an isolated config that points only at task-owned fixtures."""
    codex_root = runtime / "codex-empty"
    claude_root = runtime / "claude-empty"
    codex_root.mkdir(parents=True, exist_ok=True)
    claude_root.mkdir(parents=True, exist_ok=True)
    config = {
        "codex_root": str(codex_root),
        "claude_root": str(claude_root),
        "db_path": str(db_path),
        "polling_seconds": 999999,  # never auto-ingest during the measurement
        "weekly_budget_tokens": 4_000_000_000,
        "startup_enabled": False,
        # Use a distinct, unlikely-to-collide hotkey so we do NOT contend with the
        # human's live overlay (which already holds Ctrl+Alt+Space). The hotkey
        # registration is incidental to the activation timing we measure here.
        "hotkey": "Ctrl+Alt+Shift+F24",
    }
    config_path = runtime / "e2e-config.json"
    config_path.write_text(json.dumps(config, indent=2), encoding="utf-8")
    return config_path


def measure(db_path: Path, config_path: Path, repeats: int) -> dict:
    """Drive the real Tk DashboardApp activation pipeline and time each phase.

    We instrument the production methods directly:
      - DB read (off-UI-thread cost): _load_dashboard_data()
      - deiconify + lift + focus_force (window state change request)
      - first paint after deiconify (update_idletasks + update)
      - _render_dashboard (source filter + build_buckets + draw_chart + labels)
      - render paint (update_idletasks + update to flush the redraw)

    All Tk work runs on this (main) thread, exactly like the real UI thread.
    """
    # Import here so a missing display fails loudly with context.
    from app.codex_dashboard.ui import DashboardApp

    app = DashboardApp(config_path)
    # Let Tk finish building the (withdrawn) overlay so geometry/widgets exist.
    app.root.update_idletasks()

    def now_ms() -> float:
        return time.perf_counter() * 1000.0

    samples: list[dict] = []
    for _ in range(repeats + 1):  # one warmup + `repeats` measured
        # Ensure hidden between iterations (matches "press hotkey to show").
        if app.overlay_visible:
            app.hide_overlay()
            app.overlay.update_idletasks()

        t0 = now_ms()
        # Phase A: the DB read the cold-start path runs OFF the UI thread today.
        # We still time it because perceived latency on a cold activation waits
        # for it before any data is visible.
        events, markers = app._load_dashboard_data()
        t_db = now_ms()

        # Phase B: deiconify + lift + focus_force (what show_overlay does first).
        app.overlay.deiconify()
        app.overlay_visible = True
        app.overlay.lift()
        app.overlay.focus_force()
        t_deiconify = now_ms()

        # Phase C: force the OS to actually map+paint the just-shown window.
        app.overlay.update_idletasks()
        app.overlay.update()
        t_first_paint = now_ms()

        # Phase D: the render the activation path runs (filter+buckets+chart).
        app._render_dashboard(events, markers)
        t_render = now_ms()

        # Phase E: flush the redraw so the painted chart is actually on screen.
        app.overlay.update_idletasks()
        app.overlay.update()
        t_render_paint = now_ms()

        samples.append(
            {
                "db_read_ms": t_db - t0,
                "deiconify_lift_focus_ms": t_deiconify - t_db,
                "first_paint_ms": t_first_paint - t_deiconify,
                "render_dashboard_ms": t_render - t_first_paint,
                "render_paint_ms": t_render_paint - t_render,
                "show_to_painted_ms": t_render_paint - t_db,
                "end_to_end_ms": t_render_paint - t0,
            }
        )

    measured = samples[1:]  # drop warmup
    app.quit()

    def agg(key: str) -> dict:
        vals = sorted(s[key] for s in measured)
        n = len(vals)
        return {
            "min": round(vals[0], 3),
            "median": round(vals[n // 2], 3),
            "max": round(vals[-1], 3),
            "mean": round(sum(vals) / n, 3),
        }

    phases = [
        "db_read_ms",
        "deiconify_lift_focus_ms",
        "first_paint_ms",
        "render_dashboard_ms",
        "render_paint_ms",
        "show_to_painted_ms",
        "end_to_end_ms",
    ]
    return {key: agg(key) for key in phases}


def main() -> int:
    parser = argparse.ArgumentParser(description="Task-0013 end-to-end activation harness")
    parser.add_argument("--db", type=Path, required=True)
    parser.add_argument("--events", type=int, default=1_400_000)
    parser.add_argument("--repeats", type=int, default=7)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument(
        "--skip-seed",
        action="store_true",
        help="reuse an already-seeded DB at --db",
    )
    args = parser.parse_args()

    runtime = args.db.parent
    runtime.mkdir(parents=True, exist_ok=True)

    if not args.skip_seed:
        print(f"Seeding synthetic DB with {args.events} events at {args.db} ...")
        seed_db(args.db, args.events)

    connection = connect(args.db)
    total_rows = count_events(connection)
    connection.close()
    db_bytes = args.db.stat().st_size
    wal = args.db.with_name(args.db.name + "-wal")
    wal_bytes = wal.stat().st_size if wal.exists() else 0
    print(f"Seeded {total_rows} rows, db {db_bytes / 1_000_000:.1f} MB (+wal {wal_bytes / 1_000_000:.1f} MB)")

    config_path = _write_config(runtime, args.db)

    print("Driving real Tk activation pipeline (a window will flash) ...")
    phase_stats = measure(args.db, config_path, args.repeats)

    poll_interval_ms = 50.0  # ui.py: self.root.after(50, self._poll_hotkey)
    result = {
        "task": "Task-0013",
        "objective": "Objective 3 follow-up — end-to-end activation latency",
        "measured_at": datetime.now(UTC).isoformat(),
        "harness": "activation_e2e_harness.py",
        "synthetic_db": {
            "path": str(args.db),
            "event_count": total_rows,
            "db_bytes": db_bytes,
            "wal_bytes": wal_bytes,
            "note": (
                "Task-owned synthetic DB seeded across 21 days, 50/50 "
                "codex/claude, sized to the human's large-live-DB class. Live "
                "dashboard.db was NOT opened or sized."
            ),
        },
        "method": (
            "Drove the real Tk DashboardApp in-process against the synthetic DB. "
            "Timed each production phase with perf_counter and forced real paints "
            "with update_idletasks()+update(). All Tk work ran on the main "
            "(UI-equivalent) thread."
        ),
        "poll_interval_ms": poll_interval_ms,
        "poll_latency_ms_note": (
            f"_poll_hotkey runs every {poll_interval_ms:g} ms (root.after). The "
            "WM_HOTKEY is enqueued immediately by the Win32 thread but the "
            f"callback only fires on the next poll tick: 0..{poll_interval_ms:g} "
            f"ms added latency, ~{poll_interval_ms / 2:g} ms on average, that the "
            "in-process harness cannot include because it calls the methods "
            "directly."
        ),
        "phases_ms": phase_stats,
    }
    args.out.write_text(json.dumps(result, indent=2), encoding="utf-8")
    print(json.dumps(phase_stats, indent=2))
    print(f"Wrote {args.out}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
