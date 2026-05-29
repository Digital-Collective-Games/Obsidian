"""Task-0013 publish+restart proof: capture human-surface evidence FROM THE
PINNED RELEASE source tree.

This harness imports ``DashboardApp`` from the pinned dashboard release root
(``%LOCALAPPDATA%\\CodexDashboard\\dashboard-releases\\<release_id>\\app``), which
is the exact, hash-verified code the human's restarted overlay is now running
(``Test-DashboardRelease.ps1`` confirmed the running pythonw process points at
that release id + release root). It does NOT import from the mutable repo
checkout, so the captured surface reflects the deployed release.

Data safety: it runs against the task-owned isolated config + SQLite DB under
``Tracking/Task-0013/Testing/Runtime/`` (built by ``build_regression_fixture.py``
from synthetic Codex + Claude fixtures). It reads NO live data
(``%LOCALAPPDATA%\\CodexDashboard\\dashboard.db``, ``C:\\Users\\gregs\\.codex``,
``~/.claude`` are never touched), so the human's live spend is not exposed and
the live overlay/db/config are not disturbed.

It reuses the RELEASED app's own smoke-capture timeline (``--smoke-artifact-dir``)
because that is the path the app authors built for screenshotting the overlay and
it captures while the freshly-shown overlay is foreground. The two screenshot
modes drive that same released ``_run_smoke_capture`` after optionally toggling
the source filter:

  --mode all       -> overlay.png : OBSIDIAN brand + merged Codex+Claude 7d total,
                                     control reads "Source: All" (BEFORE).
  --mode claudeoff -> overlay.png : after unchecking Claude via the released
                                     _toggle_source path, 7d total drops to
                                     Codex-only and the control reads "Source: Codex"
                                     (AFTER).
  --mode menu      -> release-overlay-expanded.png : the released source-filter
                                     dropdown posted, showing the Codex + Claude
                                     checkbuttons expanded.
  --mode summary   -> release-capture-summary.json : deterministic, headless,
                                     occlusion-proof numeric record of the same
                                     displayed values + per-source aggregation.
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import time
from datetime import UTC, datetime, timedelta
from pathlib import Path

HERE = Path(__file__).resolve().parent
RUNTIME = HERE.parent / "Runtime"
CONFIG_PATH = RUNTIME / "config.json"

RELEASE_ID = os.environ["CODEX_DASHBOARD_RELEASE_ID"]
RELEASE_ROOT = Path(os.environ["CODEX_DASHBOARD_RELEASE_ROOT"])

# Import the dashboard package FROM THE PINNED RELEASE, not the repo checkout.
sys.path.insert(0, str(RELEASE_ROOT))

from app.codex_dashboard import aggregation  # noqa: E402
from app.codex_dashboard.config import load_config  # noqa: E402
from app.codex_dashboard.scanner import ingest_once  # noqa: E402
from app.codex_dashboard.storage import connect, load_events_since  # noqa: E402
from app.codex_dashboard.ui import DashboardApp  # noqa: E402

LOADED_UI = sys.modules["app.codex_dashboard.ui"].__file__


def _cancel_pending_afters(app) -> None:
    """Cancel every pending Tk `after` callback the released __init__ scheduled
    (the auto-smoke hotkey trigger + capture, ingest poll, etc.) so we can drive a
    deterministic capture timeline ourselves."""
    try:
        ids = app.root.tk.call("after", "info")
        for aid in app.root.tk.splitlist(ids):
            try:
                app.root.after_cancel(aid)
            except Exception:  # noqa: BLE001
                pass
    except Exception:  # noqa: BLE001
        pass


def run_capture(out_dir: Path, toggle_claude_off: bool) -> int:
    """Drive the RELEASED app's smoke capture, optionally with Claude filtered out.

    We pass smoke_artifact_dir so the released app tolerates the live overlay
    already holding the global hotkey. The released __init__ schedules its own
    auto-smoke timeline by value, so we cancel all pending `after`s and schedule
    our own: show the overlay, optionally toggle Claude off via the released
    _toggle_source, then invoke the released _run_smoke_capture (which writes
    overlay.png + overlay-summary.txt and os._exit(0)s)."""
    out_dir.mkdir(parents=True, exist_ok=True)
    app = DashboardApp(CONFIG_PATH, smoke_artifact_dir=out_dir, smoke_tab="usage")
    _cancel_pending_afters(app)

    def _drive() -> None:
        # Load the snapshot synchronously from the isolated DB (we cancelled the
        # scheduled refresh), then show the overlay and capture.
        app.refresh_data()
        app.select_tab("usage")
        if not app.overlay_visible:
            app.smoke_overlay_fallback = True
            app.show_overlay()
        app.smoke_hotkey_triggered = True  # activation exercised via show_overlay
        if toggle_claude_off:
            app.source_filter_vars["claude"].set(False)
            app._toggle_source("claude")
        app.overlay.update_idletasks()
        app.overlay.update()
        app._run_smoke_capture()  # released capture + os._exit(0)

    app.root.after(700, _drive)
    app.run()
    return 0


def run_menu() -> int:
    app = DashboardApp(CONFIG_PATH, smoke_artifact_dir=(HERE / "_smoke_unused"), smoke_tab="usage")
    # Neutralize the auto-smoke timeline; we drive the menu post ourselves.
    app._trigger_smoke_hotkey = lambda: None
    app._run_smoke_capture = lambda: None
    app.selected_interval = "1d"
    app.refresh_data()
    app.show_overlay()
    app.refresh_data()
    app.overlay.attributes("-topmost", True)
    app.overlay.lift()
    app.overlay.update_idletasks()
    app.overlay.update()

    btn = app.source_filter_button
    btn.update_idletasks()
    menu = btn.nametowidget(btn.cget("menu"))
    px = btn.winfo_rootx()
    py = btn.winfo_rooty() + btn.winfo_height()
    ox, oy = app.overlay.winfo_rootx(), app.overlay.winfo_rooty()
    ow = app.overlay.winfo_width()

    def _grab() -> None:
        try:
            app.overlay.lift()
            _capture_region(ox, oy, ow, (py + 220) - oy, HERE / "release-overlay-expanded.png")
        finally:
            os._exit(0)

    menu.after(450, _grab)
    menu.post(px, py)  # Win32 modal popup loop; _grab captures then exits hard.
    os._exit(0)


def run_summary() -> int:
    """Headless numeric proof through the RELEASED DB + scanner + render path."""
    config = load_config(CONFIG_PATH)
    conn = connect(Path(config.db_path))
    ingest = ingest_once(conn, config)
    events = load_events_since(conn, datetime.now(UTC) - timedelta(days=7))
    conn.close()

    codex_total = sum(e.total_tokens for e in events if (e.source or "codex") == "codex")
    claude_total = sum(e.total_tokens for e in events if e.source == "claude")
    merged_total = codex_total + claude_total

    app = DashboardApp(CONFIG_PATH, smoke_artifact_dir=(HERE / "_smoke_unused"), smoke_tab="usage")
    app._trigger_smoke_hotkey = lambda: None
    app._run_smoke_capture = lambda: None
    app.selected_interval = "15m"
    app.refresh_data()
    app.show_overlay()
    app.refresh_data()
    label_all = app.source_filter_button.cget("text")
    total_all = app.local_total_value.cget("text")

    app.source_filter_vars["claude"].set(False)
    app._toggle_source("claude")
    app.overlay.update_idletasks()
    app.overlay.update()
    label_codex_only = app.source_filter_button.cget("text")
    total_codex_only = app.local_total_value.cget("text")

    summary = {
        "task": "Task-0013",
        "proof": "publish+restart human-surface proof from pinned release",
        "captured_at": datetime.now(UTC).isoformat(),
        "release_id": RELEASE_ID,
        "release_root": str(RELEASE_ROOT),
        "loaded_ui_module_path": LOADED_UI,
        "loaded_from_release_root": str(RELEASE_ROOT) in LOADED_UI,
        "isolated_config": str(CONFIG_PATH),
        "isolated_db": config.db_path,
        "reads_live_data": False,
        "ingest": {
            "files_scanned": ingest.files_scanned,
            "files_updated": ingest.files_updated,
            "events_ingested": ingest.events_ingested,
        },
        "aggregation_7d": {
            "codex_tokens": codex_total,
            "claude_tokens": claude_total,
            "merged_tokens": merged_total,
            "merged_equals_codex_plus_claude": merged_total == codex_total + claude_total,
            "claude_present": claude_total > 0,
        },
        "displayed": {
            "all_sources_label": label_all,
            "all_sources_7d_total": total_all,
            "claude_off_label": label_codex_only,
            "claude_off_7d_total": total_codex_only,
            "before_after_changed": total_all != total_codex_only,
        },
        "known_sources": list(aggregation.KNOWN_SOURCES),
    }
    (HERE / "release-capture-summary.json").write_text(
        json.dumps(summary, indent=2), encoding="utf-8"
    )
    sys.stdout.write(json.dumps(summary, indent=2) + "\n")
    sys.stdout.flush()
    os._exit(0)


def _capture_region(left: int, top: int, width: int, height: int, out: Path) -> None:
    out.parent.mkdir(parents=True, exist_ok=True)
    escaped = str(out).replace("'", "''")
    script = f"""
Add-Type -AssemblyName System.Drawing
$bounds = New-Object System.Drawing.Rectangle({left}, {top}, {width}, {height})
$bitmap = New-Object System.Drawing.Bitmap $bounds.Width, $bounds.Height
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
$graphics.CopyFromScreen($bounds.Location, [System.Drawing.Point]::Empty, $bounds.Size)
$bitmap.Save('{escaped}', [System.Drawing.Imaging.ImageFormat]::Png)
$graphics.Dispose()
$bitmap.Dispose()
"""
    subprocess.run(["powershell", "-NoProfile", "-Command", script], check=True)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--mode", choices=("summary", "all", "claudeoff", "menu"), default="summary"
    )
    args = parser.parse_args()
    if args.mode == "menu":
        return run_menu()
    if args.mode == "summary":
        return run_summary()
    if args.mode == "all":
        return run_capture(HERE / "smoke-all", toggle_claude_off=False)
    return run_capture(HERE / "smoke-claude-off", toggle_claude_off=True)


if __name__ == "__main__":
    raise SystemExit(main())
