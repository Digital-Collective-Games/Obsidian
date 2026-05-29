"""Task-0013 Objective 3 activation fix: end-to-end TOGGLE latency harness.

The investigation (ACTIVATION-LATENCY-INVESTIGATION.md) found the perceived
clunkiness was dominated by `_render_dashboard` re-aggregating the full 7-day
window (~230 ms) on EVERY activation, plus window first-paint (~76 ms) and the
hotkey poll lag (~25 ms) — a ~350 ms warm path. The authorized fix makes the
hotkey TOGGLE VISIBILITY ONLY of a persistent, already-rendered overlay: no
re-aggregation, no bucket rebuild, no DB read, no render on the toggle path.

This harness drives the REAL Tk DashboardApp in-process against a task-owned
SYNTHETIC SQLite DB sized comparably to the human's large live DB, and times,
per iteration:

  BEFORE  — the old render-on-show warm path (deiconify + _render_dashboard over
            the full 7-day window + paint).
  AFTER   — the REAL production show_overlay()/hide_overlay() on the persistent
            window (deiconify + lift + focus + paint, NO render).
  POLL    — the real Fix-B _load_dashboard_data() (chart window + indexed
            per-source 7-day SUM) + one render, to show background freshness is
            cheap.

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


def _agg(values: list[float]) -> dict:
    vals = sorted(values)
    n = len(vals)
    return {
        "min": round(vals[0], 3),
        "median": round(vals[n // 2], 3),
        "max": round(vals[-1], 3),
        "mean": round(sum(vals) / n, 3),
    }


def measure(db_path: Path, config_path: Path, repeats: int) -> dict:
    """Drive the real Tk DashboardApp and time BEFORE vs AFTER the activation fix.

    The activation fix makes the global hotkey TOGGLE VISIBILITY ONLY of a
    persistent, already-rendered overlay. To produce an honest before/after on the
    same machine/DB, each iteration measures all three of:

      BEFORE (old warm path): show_overlay used to deiconify + render the dashboard
        (filter_events_by_source + build_buckets over the full 7-day window) on the
        UI thread on every activation. We reproduce that here as
        deiconify + first paint + _render_dashboard(full 7-day snapshot) + paint.

      AFTER (new toggle path): the REAL production show_overlay() / hide_overlay()
        on the persistent window — deiconify + lift + focus + paint, NO render.
        This is what the hotkey now costs.

      BACKGROUND POLL (Fix B): the real _load_dashboard_data() (now bounded to the
        chart window + an indexed per-source 7-day SUM) plus one _render_dashboard.
        This runs off the hotkey path on a timer; we measure it to show keeping the
        overlay fresh in the background is cheap (no full-window per-event scan).

    All Tk work runs on this (main) thread, exactly like the real UI thread.
    """
    # Import here so a missing display fails loudly with context.
    from app.codex_dashboard.ui import DashboardApp
    from app.codex_dashboard.storage import connect, initialize_db, load_events_since

    app = DashboardApp(config_path)
    # Let Tk finish building the (withdrawn) overlay so geometry/widgets exist.
    app.root.update_idletasks()

    # Build the FULL 7-day snapshot once, the way the OLD code loaded it on every
    # activation, so the BEFORE path renders the same heavy ~467k-event window the
    # human was experiencing. (The new code never loads this on the hotkey path.)
    now = datetime.now(app.display_timezone)
    summary_since = now.astimezone(UTC) - timedelta(days=7)
    connection = connect(db_path)
    initialize_db(connection)
    full_7d_events = load_events_since(connection, summary_since)
    connection.close()
    full_markers: dict = {}

    # Prime the persistent overlay's snapshot via the REAL Fix-B load+render once,
    # so the AFTER toggle path reveals an already-rendered window (the production
    # startup pre-render does this off-thread).
    snapshot = app._load_dashboard_data()
    app._render_dashboard(*snapshot)
    app.overlay.update_idletasks()

    def now_ms() -> float:
        return time.perf_counter() * 1000.0

    before_samples: list[dict] = []
    after_samples: list[dict] = []
    poll_samples: list[dict] = []

    for _ in range(repeats + 1):  # one warmup + `repeats` measured
        if app.overlay_visible:
            app.hide_overlay()
            app.overlay.update_idletasks()

        # ---- BEFORE: old warm path (render the full 7-day window on show) ----
        b0 = now_ms()
        app.overlay.deiconify()
        app.overlay_visible = True
        app.overlay.lift()
        app.overlay.focus_force()
        b_deiconify = now_ms()
        app.overlay.update_idletasks()
        app.overlay.update()
        b_first_paint = now_ms()
        # The expensive per-event render the OLD show_overlay ran every time.
        app._render_dashboard(full_7d_events, full_markers)
        b_render = now_ms()
        app.overlay.update_idletasks()
        app.overlay.update()
        b_paint = now_ms()
        before_samples.append(
            {
                "deiconify_lift_focus_ms": b_deiconify - b0,
                "first_paint_ms": b_first_paint - b_deiconify,
                "render_dashboard_ms": b_render - b_first_paint,
                "render_paint_ms": b_paint - b_render,
                "show_to_painted_ms": b_paint - b0,
            }
        )

        # Hide before the AFTER measurement so AFTER is a real show from hidden.
        app.hide_overlay()
        app.overlay.update_idletasks()
        app.overlay.update()

        # ---- AFTER: the REAL production toggle path (show/hide only) ----
        a0 = now_ms()
        app.show_overlay()  # production method: deiconify + lift + focus, NO render
        a_show = now_ms()
        app.overlay.update_idletasks()
        app.overlay.update()  # OS maps + paints the already-rendered window
        a_paint = now_ms()
        app.hide_overlay()  # production hide
        a_hide = now_ms()
        app.overlay.update_idletasks()
        app.overlay.update()
        a_hide_paint = now_ms()
        after_samples.append(
            {
                "show_call_ms": a_show - a0,
                "show_paint_ms": a_paint - a_show,
                "show_to_painted_ms": a_paint - a0,
                "hide_call_ms": a_hide - a_paint,
                "hide_to_painted_ms": a_hide_paint - a_paint,
                "toggle_round_trip_ms": a_hide_paint - a0,
            }
        )

        # ---- BACKGROUND POLL (Fix B): cheap load + one render, off hotkey path ----
        p0 = now_ms()
        snap = app._load_dashboard_data()
        p_load = now_ms()
        app._render_dashboard(*snap)
        p_render = now_ms()
        poll_samples.append(
            {
                "load_dashboard_data_ms": p_load - p0,
                "render_dashboard_ms": p_render - p_load,
                "poll_total_ms": p_render - p0,
            }
        )

    app.quit()

    def aggregate(samples: list[dict]) -> dict:
        measured = samples[1:]  # drop warmup
        keys = measured[0].keys()
        return {k: _agg([s[k] for s in measured]) for k in keys}

    return {
        "before_old_render_on_show": aggregate(before_samples),
        "after_toggle_show_hide_only": aggregate(after_samples),
        "background_poll_fixb": aggregate(poll_samples),
    }


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
    stats = measure(args.db, config_path, args.repeats)

    before = stats["before_old_render_on_show"]
    after = stats["after_toggle_show_hide_only"]
    poll_interval_ms = 50.0  # ui.py: self.root.after(50, self._poll_hotkey)
    poll_avg = poll_interval_ms / 2

    before_show = before["show_to_painted_ms"]["median"]
    after_show = after["show_to_painted_ms"]["median"]
    result = {
        "task": "Task-0013",
        "objective": (
            "Objective 3 activation fix — show/hide only, no rebuild on toggle"
        ),
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
            "BEFORE reproduces the old render-on-show warm path (deiconify + "
            "_render_dashboard over the full 7-day window + paint). AFTER calls "
            "the REAL production show_overlay()/hide_overlay() on the persistent "
            "window (deiconify + lift + focus + paint, NO render). BACKGROUND POLL "
            "times the real Fix-B _load_dashboard_data() (chart window + indexed "
            "per-source 7-day SUM) plus one render. All Tk work ran on the main "
            "(UI-equivalent) thread; real paints forced with update_idletasks()+"
            "update()."
        ),
        "poll_interval_ms": poll_interval_ms,
        "poll_latency_ms_note": (
            f"_poll_hotkey runs every {poll_interval_ms:g} ms (root.after). The "
            "WM_HOTKEY is enqueued immediately by the Win32 thread but the "
            f"callback only fires on the next poll tick: 0..{poll_interval_ms:g} "
            f"ms added latency, ~{poll_avg:g} ms on average, added to BOTH before "
            "and after (the in-process harness calls methods directly so it does "
            "not include this; add it to both for perceived latency)."
        ),
        "summary": {
            "before_show_to_painted_median_ms": before_show,
            "after_show_to_painted_median_ms": after_show,
            "before_perceived_median_ms": round(before_show + poll_avg, 3),
            "after_perceived_median_ms": round(after_show + poll_avg, 3),
            "speedup_x": (
                round(before_show / after_show, 2) if after_show > 0 else None
            ),
            "after_render_on_toggle": False,
            "interpretation": (
                "AFTER toggle show latency is dominated only by the OS window "
                "map/paint (no per-event work), versus the BEFORE warm path that "
                "re-aggregated the full 7-day window on every activation."
            ),
        },
        "phases_ms": stats,
    }
    args.out.write_text(json.dumps(result, indent=2), encoding="utf-8")
    print(json.dumps(result["summary"], indent=2))
    print(json.dumps(stats, indent=2))
    print(f"Wrote {args.out}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
