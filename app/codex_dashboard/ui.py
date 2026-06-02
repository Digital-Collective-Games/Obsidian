from __future__ import annotations

import ctypes
import json
import os
import queue
import shutil
import subprocess
import threading
import tkinter as tk
import webbrowser
from datetime import UTC, datetime, timedelta, tzinfo
from pathlib import Path
from tkinter import ttk

from .aggregation import (
    INTERVAL_SECONDS,
    KNOWN_SOURCES,
    METRIC_MODES,
    SOURCE_LABELS,
    build_buckets,
    build_project_stacks,
    filter_events_by_source,
    is_over_redline,
    project_weekly_burn,
)
from .config import DashboardConfig, load_config, maybe_upgrade_weekly_budget, save_config
from .hotkey import GlobalHotkey
from .investigation import (
    build_bucket_investigation,
    build_codex_launch_command,
    report_path_for_brief,
    write_bucket_investigation,
)
from .jobs_backend import (
    configured_jobs_backend_url,
    fetch_jobs_snapshot,
    job_detail_text,
    job_is_running,
    job_last_run_display,
    job_next_run_display,
    job_status_chip,
    jobs_backend_error_snapshot,
    start_job_run,
    summarize_apply_report,
    sync_jobs_snapshot,
)
from .paths import default_config_path, default_investigations_path
from .scanner import ingest_once
from .storage import (
    connect,
    initialize_db,
    load_events_since,
    load_latest_weekly_advisory,
    load_session_context_markers,
    sum_total_tokens_by_source_since,
)
from .tasks_backend import fetch_tasks_snapshot
from .worktrees_backend import (
    assign_worktree,
    configured_worktrees_backend_url,
    create_worktree,
    dequeue_task,
    destroy_worktree,
    eject_worktree,
    fetch_pool_snapshot,
    fetch_repos,
    worktrees_backend_error_snapshot,
)
from .worktrees_tab import (
    ALL_REPOS_OPTION,
    claude_session_uri,
    filter_task_options,
    filter_worktrees_by_repo,
    first_assignable_task_id,
    is_allocated,
    open_task_options,
    repo_filter_options,
    sort_task_options,
    task_state_category,
    task_state_chip_label,
    worktree_detail_lines,
    worktree_heading_repo,
    worktree_issue_url,
    worktree_session_target,
    worktree_status_background,
    worktree_status_color,
    worktree_status_label,
    worktree_summary_counts,
    vscodium_uri,
)


INTERVAL_TITLES = {
    "1m": "1 Minute",
    "5m": "5 Minutes",
    "15m": "15 Minutes",
    "1h": "1 Hour",
    "1d": "1 Day",
}
CHART_MODES = {
    "velocity": "Velocity",
    "repo": "Repo",
}
FONT_ASSET_DIR = Path(__file__).resolve().parent / "assets" / "fonts"
USAGE_SUMMARY_WINDOW = timedelta(days=7)
DEFAULT_CHART_BUCKET_COUNT = 20
CHART_BUCKET_COUNT_BY_INTERVAL = {
    "1d": 35,
}
ROLLING_PROJECTION_BUCKETS = 4
REPO_STACK_COLORS = (
    "#5eb8ff",
    "#ff9b52",
    "#3fd49f",
    "#d785ff",
    "#ffd65c",
    "#6e7b8c",
)
REPO_STACK_OUTLINE = "#0d131b"
TASK_DETAIL_TEXT_BACKGROUND = "#10141a"
TAB_ACTIVE_FOREGROUND = "#c3f5ff"
TAB_INACTIVE_FOREGROUND = "#9fbdcc"
TAB_ACTIVE_UNDERLINE = "#00e5ff"
HEADER_BACKGROUND = "#181c22"


def load_private_font_assets() -> list[Path]:
    font_paths = [
        FONT_ASSET_DIR / "Inter[opsz,wght].ttf",
        FONT_ASSET_DIR / "SpaceGrotesk[wght].ttf",
    ]
    loaded_fonts: list[Path] = []
    add_font_resource = ctypes.windll.gdi32.AddFontResourceExW
    FR_PRIVATE = 0x10
    for font_path in font_paths:
        if font_path.exists() and add_font_resource(str(font_path), FR_PRIVATE, 0):
            loaded_fonts.append(font_path)
    return loaded_fonts


def unload_private_font_assets(font_paths: list[Path]) -> None:
    remove_font_resource = ctypes.windll.gdi32.RemoveFontResourceExW
    FR_PRIVATE = 0x10
    for font_path in font_paths:
        remove_font_resource(str(font_path), FR_PRIVATE, 0)


def format_tick_label(start_at: datetime, interval_key: str) -> str:
    if interval_key == "1d":
        return start_at.strftime("%m-%d")

    hour = start_at.strftime("%I").lstrip("0") or "12"
    meridiem = start_at.strftime("%p")
    if start_at.minute == 0:
        return f"{hour}{meridiem}"
    return f"{hour}:{start_at.minute:02d}{meridiem}"


def format_token_value(value: int) -> str:
    absolute_value = abs(value)
    for divisor, suffix in (
        (1_000_000_000, "B"),
        (1_000_000, "M"),
        (1_000, "K"),
    ):
        if absolute_value >= divisor:
            scaled = value / divisor
            text = f"{scaled:.1f}".rstrip("0").rstrip(".")
            return f"{text}{suffix}"
    return str(value)


def format_signed_token_value(value: int) -> str:
    if value == 0:
        return "0"
    prefix = "+" if value > 0 else "-"
    return f"{prefix}{format_token_value(abs(value))}"


def format_reset_remaining(reset_epoch: int, now: datetime | None = None) -> str:
    current_time = now or datetime.now(UTC)
    remaining_seconds = max(0, reset_epoch - int(current_time.timestamp()))
    if remaining_seconds >= 24 * 60 * 60:
        return f"{remaining_seconds / (24 * 60 * 60):.1f}d"
    if remaining_seconds >= 60 * 60:
        return f"{remaining_seconds / (60 * 60):.1f}h"
    if remaining_seconds >= 60:
        return f"{remaining_seconds / 60:.0f}m"
    return f"{remaining_seconds}s"


def format_budget_billions(weekly_budget_tokens: int) -> str:
    return f"{weekly_budget_tokens / 1_000_000_000:.1f}".rstrip("0").rstrip(".")


def parse_budget_billions(raw_value: str) -> int:
    normalized = raw_value.lower().replace(",", "").strip()
    if not normalized:
        raise ValueError("budget is required")
    if normalized.endswith("b"):
        normalized = normalized[:-1].strip()
    budget_value = float(normalized)
    if budget_value <= 0:
        raise ValueError("budget must be positive")
    if "." in normalized or budget_value < 10_000:
        return int(round(budget_value * 1_000_000_000))
    return int(round(budget_value))


def rolling_average_tokens(buckets, sample_size: int) -> int:
    if not buckets or sample_size <= 0:
        return 0
    recent_buckets = buckets[-sample_size:]
    return int(round(sum(bucket.total_tokens for bucket in recent_buckets) / len(recent_buckets)))


def chart_bucket_count(interval_key: str) -> int:
    return CHART_BUCKET_COUNT_BY_INTERVAL.get(interval_key, DEFAULT_CHART_BUCKET_COUNT)


def usage_history_lookback(interval_key: str, bucket_count: int | None = None) -> timedelta:
    effective_bucket_count = bucket_count or chart_bucket_count(interval_key)
    return max(
        USAGE_SUMMARY_WINDOW,
        timedelta(seconds=INTERVAL_SECONDS[interval_key] * effective_bucket_count),
    )


def format_chart_title(
    interval_key: str,
    chart_mode: str = "velocity",
    metric_mode: str = "total",
) -> str:
    prefix = "Normalized " if metric_mode == "norm" else ""
    if chart_mode == "repo":
        return f"{prefix}Repo Share per {INTERVAL_TITLES.get(interval_key, interval_key)}"
    return f"{prefix}Token Velocity per {INTERVAL_TITLES.get(interval_key, interval_key)}"


def format_velocity_tooltip(total_tokens: int) -> str:
    return format_token_value(total_tokens)


def format_repo_tooltip(
    bucket_totals: dict[str, int],
    repo_legend: list[tuple[str, str]],
) -> str:
    nonzero_segments = [
        (label, bucket_totals.get(project_key, 0))
        for project_key, label in repo_legend
        if bucket_totals.get(project_key, 0) > 0
    ]
    if not nonzero_segments:
        return "0"
    nonzero_segments.sort(key=lambda item: (-item[1], item[0].lower()))
    return "\n".join(f"{label}: {format_token_value(total_tokens)}" for label, total_tokens in nonzero_segments)


def interval_redline_tokens(weekly_budget_tokens: int, interval_seconds: int) -> int:
    return max(1, int(weekly_budget_tokens * interval_seconds / (7 * 24 * 60 * 60)))


def jobs_needs_attention_count(summary: dict[str, int]) -> int:
    return sum(count for status, count in summary.items() if status != "in_sync")


def format_jobs_timestamp(raw_value: str | None) -> str:
    if not raw_value:
        return "Not reconciled"
    parsed = datetime.fromisoformat(raw_value.replace("Z", "+00:00"))
    return parsed.astimezone().strftime("%I:%M %p").lstrip("0")


def format_tasks_timestamp(raw_value: str | None) -> str:
    if not raw_value:
        return "Not refreshed"
    parsed = datetime.fromisoformat(raw_value.replace("Z", "+00:00"))
    return parsed.astimezone().strftime("%b %d, %I:%M %p").replace(" 0", " ")


def write_overlay_capture(window: tk.Toplevel, output_path: Path) -> None:
    output_path.parent.mkdir(parents=True, exist_ok=True)
    x = window.winfo_rootx()
    y = window.winfo_rooty()
    width = window.winfo_width()
    height = window.winfo_height()
    if width <= 0 or height <= 0:
        raise ValueError("overlay capture requires a visible window with non-zero size")

    escaped_output = str(output_path).replace("'", "''")
    script = f"""
Add-Type -AssemblyName System.Drawing
$bounds = New-Object System.Drawing.Rectangle({x}, {y}, {width}, {height})
$bitmap = New-Object System.Drawing.Bitmap $bounds.Width, $bounds.Height
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
$graphics.CopyFromScreen($bounds.Location, [System.Drawing.Point]::Empty, $bounds.Size)
$bitmap.Save('{escaped_output}', [System.Drawing.Imaging.ImageFormat]::Png)
$graphics.Dispose()
$bitmap.Dispose()
"""
    kwargs: dict[str, object] = {"check": True}
    if hasattr(subprocess, "CREATE_NO_WINDOW"):
        kwargs["creationflags"] = subprocess.CREATE_NO_WINDOW
    subprocess.run(
        ["powershell", "-NoProfile", "-Command", script],
        **kwargs,
    )


def query_primary_work_area() -> tuple[int, int, int, int]:
    """Return the primary monitor's work area as (left, top, right, bottom).

    The work area excludes the Windows taskbar (on whichever edge it is docked),
    unlike Tk's ``winfo_screenwidth``/``winfo_screenheight``. Raises ``OSError`` if
    the query fails so the caller can fall back to full-screen dimensions.
    """
    SPI_GETWORKAREA = 0x0030

    class _RECT(ctypes.Structure):
        _fields_ = [
            ("left", ctypes.c_long),
            ("top", ctypes.c_long),
            ("right", ctypes.c_long),
            ("bottom", ctypes.c_long),
        ]

    rect = _RECT()
    if not ctypes.windll.user32.SystemParametersInfoW(
        SPI_GETWORKAREA, 0, ctypes.byref(rect), 0
    ):
        raise OSError("SystemParametersInfo(SPI_GETWORKAREA) failed")
    return (int(rect.left), int(rect.top), int(rect.right), int(rect.bottom))


def compute_overlay_geometry(
    screen_width: int,
    screen_height: int,
    work_area: tuple[int, int, int, int],
    tab_id: str,
    pad_fraction: float,
) -> str:
    """Pure, tab-aware overlay geometry. Returns a Tk ``"WxH+X+Y"`` string.

    Task-0014: every tab is repositioned into the usable work area (taskbar
    excluded) with its top one ``pad`` below the work-area top; the Jobs and Worktrees
    tabs additionally grow to fill the usable height minus ``pad`` at top and
    bottom, so the taskbar is never covered. The Usage (default) tab keeps its
    current size. Width stays the current 980 clamp (never widened) and is
    right-aligned within the work area. ``pad = round(pad_fraction * screen_height)``.
    Takes no Tk calls and reads no global state, so it is unit-testable with mocked
    inputs.
    """
    wa_left, wa_top, wa_right, wa_bottom = work_area
    pad = round(pad_fraction * screen_height)
    usable_width = wa_right - wa_left
    usable_height = wa_bottom - wa_top
    margin_x = 40

    width = min(980, max(860, usable_width - 80))
    if tab_id in ("jobs", "worktrees"):
        # Tall layout: fill the usable height minus top/bottom pad. The 620 floor
        # only triggers for a misconfigured (degenerate) pad_fraction; for any sane
        # value the canonical usable_height - 2*pad applies and bottom == wa_bottom
        # - pad, keeping a gap above the taskbar.
        height = max(620, usable_height - 2 * pad)
    else:
        height = min(660, max(620, usable_height - 80))

    x = max(wa_left, wa_right - width - margin_x)
    y = wa_top + pad
    return f"{width}x{height}+{x}+{y}"


class DashboardApp:
    def __init__(
        self,
        config_path: Path | None = None,
        smoke_artifact_dir: Path | None = None,
        smoke_tab: str | None = None,
    ) -> None:
        self.config_path = config_path or default_config_path()
        self.config = load_config(self.config_path)
        self.active_tab = "usage"
        self.selected_interval = "15m"
        self.selected_chart_mode = "velocity"
        self.selected_metric_mode = "total"
        # Task-0013 Objective 4: source filter selection. Default = all known
        # sources checked (today's merged Codex+Claude behavior). Held as a set
        # of source keys; the filter operates on the in-memory snapshot only.
        self.selected_sources: set[str] = set(KNOWN_SOURCES)
        self.ingest_queue: queue.Queue[tuple[str, object]] = queue.Queue()
        self.ingest_in_flight = False
        self.last_ingest_error: str | None = None
        self.hotkey_registered = False
        self.smoke_artifact_dir = smoke_artifact_dir
        self.smoke_tab = smoke_tab
        self._quitting = False
        self.smoke_hotkey_triggered = False
        self.smoke_overlay_fallback = False
        self.display_timezone = self._resolve_display_timezone()
        self.loaded_font_assets = load_private_font_assets()
        self.chart_hover_regions: list[dict[str, object]] = []
        self.chart_context_region: dict[str, object] | None = None
        self.latest_events = []
        self.latest_session_context_markers: dict[str, list[object]] = {}
        self.latest_repo_legend: list[tuple[str, str]] = []
        self.latest_repo_totals: list[dict[str, int]] = []
        # Task-0013 activation-fix follow-up (Fix B): per-source rolling 7-day
        # totals and the latest advisory are computed cheaply by the background
        # poll (indexed SQL SUM + lookback), not by re-scanning the in-memory
        # window. Kept here so a source-filter toggle can recompute the displayed
        # 7-day total in memory (sum the selected sources) with no DB read.
        self.latest_source_totals_7d: dict[str, int] = {}
        self.latest_weekly_advisory = None
        # Task-0013 Objective 3: guards a single off-thread cold-start load.
        self._activation_load_in_flight = False
        self.jobs_backend_url = configured_jobs_backend_url()
        self.jobs_snapshot: dict[str, object] = {
            "generated_at": None,
            "last_reconciled_at": None,
            "summary": {},
            "jobs": [],
        }
        self.jobs_status_message = (
            "REFRESH rereads backend state. UPDATE applies Git desired state to Temporal."
        )
        self.worktrees_backend_url = configured_worktrees_backend_url()
        self.worktrees_snapshot: dict[str, object] = {
            "status": "loading",
            "worktrees": [],
            "message": "Open Worktrees to load the backend worktree pool.",
        }
        self.worktrees_repos: list[dict[str, object]] = []
        self.worktrees_repo_filter = ALL_REPOS_OPTION
        self.worktrees_status_message = "Refresh rereads the worktree pool from the orchestration backend."
        self.debug_log_path = self.config_path.parent / "dashboard-debug.log"
        self._append_debug_log("dashboard_started")

        self.root = tk.Tk()
        self.root.withdraw()
        self.root.title("OBSIDIAN")
        self.root.protocol("WM_DELETE_WINDOW", self.quit)

        self.overlay = tk.Toplevel(self.root)
        self.overlay.withdraw()
        self.overlay_visible = False
        self.overlay.overrideredirect(True)
        self.overlay.attributes("-topmost", True)
        self.overlay.geometry(self._overlay_geometry())
        self.overlay.configure(bg="#0a0e14")
        self.overlay.bind("<Escape>", lambda _event: self.hide_overlay())

        self._configure_style()
        self._build_overlay()

        self.hotkey = GlobalHotkey(self.config.hotkey, self.toggle_overlay)
        try:
            self.hotkey.register()
            self.hotkey_registered = True
        except OSError:
            if self.smoke_artifact_dir is None:
                raise

        self.root.after(50, self._poll_hotkey)
        self.root.after(100, self._poll_ingest_results)
        # Task-0013 activation-fix follow-up: pre-render the persistent overlay at
        # startup so the FIRST hotkey toggle is already fast (no first-show
        # rebuild). Run the initial load OFF the UI thread so startup itself does
        # not block on the (potentially large) DB read; the snapshot lands via the
        # ingest queue and renders the withdrawn overlay before the user toggles.
        self.root.after(100, self._start_activation_load)
        self.root.after(250, self.schedule_ingest)
        if self.smoke_artifact_dir is not None:
            self.root.after(350, self._trigger_smoke_hotkey)
            self.root.after(1200, self._run_smoke_capture)

    def _overlay_geometry(self) -> str:
        screen_width = self.root.winfo_screenwidth()
        screen_height = self.root.winfo_screenheight()
        try:
            work_area = query_primary_work_area()
        except OSError:
            work_area = (0, 0, screen_width, screen_height)
        return compute_overlay_geometry(
            screen_width,
            screen_height,
            work_area,
            self.active_tab,
            self.config.pad_fraction,
        )

    def _apply_overlay_geometry(self) -> None:
        # Task-0014: cheap geometry-only re-apply (no data rebuild). Safe whether
        # or not the overlay is currently visible.
        self.overlay.geometry(self._overlay_geometry())

    def _configure_style(self) -> None:
        style = ttk.Style()
        style.theme_use("clam")
        style.configure("Overlay.TFrame", background="#0a0e14")
        style.configure("Shell.TFrame", background="#11151b")
        style.configure("Header.TFrame", background="#181c22")
        style.configure("BodyPanel.TFrame", background="#11151b")
        style.configure("Card.TFrame", background="#181c22")
        style.configure(
            "Brand.TLabel",
            background="#181c22",
            foreground="#bff4ff",
            font=("Space Grotesk", 16, "bold"),
        )
        style.configure(
            "Badge.TLabel",
            background="#31353c",
            foreground="#8fa8bb",
            font=("Inter", 8, "bold"),
        )
        style.configure(
            "Status.TLabel",
            background="#11151b",
            foreground="#8fa8bb",
            font=("Inter", 9),
        )
        style.configure(
            "MetricTitle.TLabel",
            background="#181c22",
            foreground="#8fa8bb",
            font=("Inter", 8, "bold"),
        )
        style.configure(
            "MetricValue.TLabel",
            background="#181c22",
            foreground="#dfe2eb",
            font=("Space Grotesk", 20, "bold"),
        )
        style.configure(
            "MetricUnit.TLabel",
            background="#181c22",
            foreground="#8fa8bb",
            font=("Inter", 9),
        )
        style.configure(
            "MetricDetail.TLabel",
            background="#181c22",
            foreground="#8fa8bb",
            font=("Inter", 9),
        )
        style.configure(
            "ChartTitle.TLabel",
            background="#11151b",
            foreground="#dfe2eb",
            font=("Space Grotesk", 10, "bold"),
        )
        style.configure(
            "Tiny.TLabel",
            background="#11151b",
            foreground="#6e8598",
            font=("Inter", 8),
        )
        style.configure(
            "Accent.TButton",
            background="#16d9f5",
            foreground="#10141a",
            font=("Inter", 9, "bold"),
            borderwidth=0,
            focusthickness=0,
        )
        style.map(
            "Accent.TButton",
            background=[("active", "#2ee8ff")],
        )
        style.configure(
            "Quiet.TButton",
            background="#303743",
            foreground="#dfe2eb",
            font=("Inter", 9, "bold"),
            borderwidth=0,
            focusthickness=0,
        )
        style.map(
            "Quiet.TButton",
            background=[("active", "#3b4450")],
        )
        # Destructive action (Eject) — the mockup's ghost/tinted error button: a subtle red
        # tint with red text at rest, going to a solid red fill on hover.
        style.configure(
            "Danger.TButton",
            background="#2a1719",
            foreground="#ffb4ab",
            font=("Inter", 9, "bold"),
            borderwidth=0,
            focusthickness=0,
        )
        style.map(
            "Danger.TButton",
            background=[("active", "#5a2327")],
        )
        style.configure(
            "HeaderQuiet.TButton",
            background="#303743",
            foreground="#dfe2eb",
            font=("Inter", 8, "bold"),
            borderwidth=0,
            focusthickness=0,
        )
        style.map(
            "HeaderQuiet.TButton",
            background=[("active", "#3b4450")],
        )
        style.configure(
            "HeaderAccent.TButton",
            background="#16d9f5",
            foreground="#10141a",
            font=("Inter", 8, "bold"),
            borderwidth=0,
            focusthickness=0,
        )
        style.map(
            "HeaderAccent.TButton",
            background=[("active", "#2ee8ff")],
        )
        style.configure(
            "ToolbarQuiet.TButton",
            background="#303743",
            foreground="#dfe2eb",
            font=("Inter", 8, "bold"),
            borderwidth=0,
            focusthickness=0,
        )
        style.map(
            "ToolbarQuiet.TButton",
            background=[("active", "#3b4450")],
        )
        style.configure(
            "ToolbarAccent.TButton",
            background="#16d9f5",
            foreground="#10141a",
            font=("Inter", 8, "bold"),
            borderwidth=0,
            focusthickness=0,
        )
        style.map(
            "ToolbarAccent.TButton",
            background=[("active", "#2ee8ff")],
        )
        style.configure(
            "StatusValue.TLabel",
            background="#181c22",
            foreground="#bff4ff",
            font=("Inter", 12, "bold"),
        )
        style.configure(
            "StatusDetail.TLabel",
            background="#181c22",
            foreground="#8fa8bb",
            font=("Inter", 9),
        )
        style.configure(
            "Vertical.TScrollbar",
            background="#2b3440",
            troughcolor="#10141a",
            bordercolor="#10141a",
            arrowcolor="#9fbdcc",
            darkcolor="#1c2026",
            lightcolor="#2b3440",
        )
        style.map(
            "Vertical.TScrollbar",
            background=[("active", "#374555")],
            arrowcolor=[("active", "#c3f5ff")],
        )
        # Task-0016 WORKTREES tab: a dark-palette repo-filter dropdown matching the cockpit.
        style.configure(
            "Worktrees.TCombobox",
            fieldbackground="#10141a",
            background="#303743",
            foreground="#dfe2eb",
            arrowcolor="#9fbdcc",
            bordercolor="#39424d",
            lightcolor="#39424d",
            darkcolor="#10141a",
            selectbackground="#173a44",
            selectforeground="#c3f5ff",
            font=("Inter", 9),
        )
        style.map(
            "Worktrees.TCombobox",
            fieldbackground=[("readonly", "#10141a")],
            foreground=[("readonly", "#dfe2eb")],
            arrowcolor=[("active", "#c3f5ff")],
        )

    def _build_overlay(self) -> None:
        self.container = ttk.Frame(self.overlay, style="Overlay.TFrame", padding=28)
        self.container.pack(fill="both", expand=True)

        self.shell = ttk.Frame(self.container, style="Shell.TFrame")
        self.shell.pack(fill="both", expand=True)

        header = ttk.Frame(self.shell, style="Header.TFrame", padding=(16, 12))
        header.pack(fill="x")
        header.columnconfigure(0, weight=1)
        brand_row = ttk.Frame(header, style="Header.TFrame")
        brand_row.grid(row=0, column=0, sticky="w")
        ttk.Label(brand_row, text="OBSIDIAN", style="Brand.TLabel").pack(side="left")
        self.tab_buttons: dict[str, tk.Label] = {}
        self.tab_underlines: dict[str, tk.Frame] = {}
        nav_row = ttk.Frame(brand_row, style="Header.TFrame")
        nav_row.pack(side="left", padx=(24, 0))
        for tab_id, label in (("usage", "Usage"), ("jobs", "Jobs"), ("worktrees", "Worktrees")):
            tab_shell = tk.Frame(nav_row, bg=HEADER_BACKGROUND)
            tab_shell.pack(side="left", padx=(0, 20))
            tab_label = tk.Label(
                tab_shell,
                text=label.upper(),
                bg=HEADER_BACKGROUND,
                fg=TAB_INACTIVE_FOREGROUND,
                font=("Space Grotesk", 10, "bold"),
                cursor="hand2",
            )
            tab_label.pack(anchor="w")
            underline = tk.Frame(
                tab_shell,
                bg=HEADER_BACKGROUND,
                height=2,
                width=30,
            )
            underline.pack(anchor="w", pady=(5, 0))
            for widget in (tab_shell, tab_label, underline):
                widget.bind("<Button-1>", lambda _event, key=tab_id: self.select_tab(key))
            self.tab_buttons[tab_id] = tab_label
            self.tab_underlines[tab_id] = underline

        ttk.Button(
            header,
            text="X",
            style="HeaderQuiet.TButton",
            command=self.hide_overlay,
            width=3,
        ).grid(row=0, column=1, sticky="e")

        tk.Frame(self.shell, bg="#39424d", height=1).pack(fill="x")

        self.content_stack = ttk.Frame(self.shell, style="BodyPanel.TFrame")
        self.content_stack.pack(fill="both", expand=True)

        body = ttk.Frame(self.content_stack, style="BodyPanel.TFrame", padding=(16, 14))
        body.pack(fill="both", expand=True)
        self.usage_body = body

        usage_toolbar = ttk.Frame(body, style="BodyPanel.TFrame")
        usage_toolbar.pack(fill="x", pady=(0, 12))
        self.usage_header_controls = ttk.Frame(usage_toolbar, style="BodyPanel.TFrame")
        self.usage_header_controls.pack(side="left")
        self.usage_budget_controls = ttk.Frame(usage_toolbar, style="BodyPanel.TFrame")
        self.usage_budget_controls.pack(side="right")

        interval_shell = ttk.Frame(self.usage_header_controls, style="Shell.TFrame", padding=(8, 6))
        interval_shell.pack(side="left", padx=(0, 8))
        self.interval_buttons: dict[str, ttk.Button] = {}
        for interval_key in ("1m", "5m", "15m", "1h", "1d"):
            button = ttk.Button(
                interval_shell,
                text=interval_key,
                style="ToolbarQuiet.TButton",
                command=lambda key=interval_key: self.select_interval(key),
                width=4,
            )
            button.pack(side="left", padx=(0, 6))
            self.interval_buttons[interval_key] = button

        chart_mode_shell = ttk.Frame(self.usage_header_controls, style="Shell.TFrame", padding=(8, 6))
        chart_mode_shell.pack(side="left", padx=(0, 8))
        self.chart_mode_buttons: dict[str, ttk.Button] = {}
        for chart_mode, label in CHART_MODES.items():
            button = ttk.Button(
                chart_mode_shell,
                text=label,
                style="ToolbarQuiet.TButton",
                command=lambda mode=chart_mode: self.select_chart_mode(mode),
                width=6,
            )
            button.pack(side="left", padx=(0, 6))
            self.chart_mode_buttons[chart_mode] = button

        metric_mode_shell = ttk.Frame(self.usage_header_controls, style="Shell.TFrame", padding=(8, 6))
        metric_mode_shell.pack(side="left")
        self.metric_mode_buttons: dict[str, ttk.Button] = {}
        for metric_mode, label in METRIC_MODES.items():
            button = ttk.Button(
                metric_mode_shell,
                text=label,
                style="ToolbarQuiet.TButton",
                command=lambda mode=metric_mode: self.select_metric_mode(mode),
                width=6,
            )
            button.pack(side="left", padx=(0, 6))
            self.metric_mode_buttons[metric_mode] = button

        self._build_source_filter_control()

        self.status_label = ttk.Label(
            body,
            text="Waiting for first ingest...",
            style="Status.TLabel",
        )
        ttk.Label(self.usage_budget_controls, text="Budget (B)", style="Status.TLabel").pack(side="left")
        self.weekly_budget_var = tk.StringVar(
            value=format_budget_billions(self.config.weekly_budget_tokens)
        )
        self.weekly_budget_entry = tk.Entry(
            self.usage_budget_controls,
            textvariable=self.weekly_budget_var,
            width=5,
            bg="#121820",
            fg="#dfe2eb",
            insertbackground="#dfe2eb",
            relief="flat",
            highlightthickness=1,
            highlightbackground="#2b323b",
            highlightcolor="#16d9f5",
            font=("Inter", 10),
        )
        self.weekly_budget_entry.pack(side="left", padx=(10, 8), ipady=4)
        ttk.Button(
            self.usage_budget_controls,
            text="Save",
            style="Accent.TButton",
            command=self.save_budget,
        ).pack(side="left", padx=(0, 10))

        metrics_row = ttk.Frame(body, style="BodyPanel.TFrame")
        metrics_row.pack(fill="x", pady=(0, 14))
        for column in range(4):
            metrics_row.columnconfigure(column, weight=1)

        card_7d = ttk.Frame(metrics_row, style="Card.TFrame", padding=(12, 10))
        card_7d.grid(row=0, column=0, sticky="nsew", padx=(0, 10))
        ttk.Label(card_7d, text="7D TOTAL TOKENS", style="MetricTitle.TLabel").pack(anchor="w")
        value_row = ttk.Frame(card_7d, style="Card.TFrame")
        value_row.pack(anchor="w", pady=(10, 4))
        self.local_total_value = ttk.Label(value_row, text="0", style="MetricValue.TLabel")
        self.local_total_value.pack(side="left")
        ttk.Label(value_row, text="TKN", style="MetricUnit.TLabel").pack(side="left", padx=(6, 0), pady=(9, 0))
        self.local_total_detail = ttk.Label(card_7d, text="", style="MetricDetail.TLabel")
        self.local_total_detail.pack(anchor="w")

        card_projected = ttk.Frame(metrics_row, style="Card.TFrame", padding=(12, 10))
        card_projected.grid(row=0, column=1, sticky="nsew", padx=(0, 10))
        ttk.Label(card_projected, text="PROJECTED WEEKLY BURN", style="MetricTitle.TLabel").pack(anchor="w")
        projected_row = ttk.Frame(card_projected, style="Card.TFrame")
        projected_row.pack(anchor="w", pady=(10, 4))
        self.projected_value = ttk.Label(projected_row, text="0", style="MetricValue.TLabel")
        self.projected_value.pack(side="left")
        ttk.Label(projected_row, text="TKN", style="MetricUnit.TLabel").pack(side="left", padx=(6, 0), pady=(9, 0))
        self.projected_detail = ttk.Label(card_projected, text="", style="MetricDetail.TLabel")
        self.projected_detail.pack(anchor="w")

        card_headroom = ttk.Frame(metrics_row, style="Card.TFrame", padding=(12, 10))
        card_headroom.grid(row=0, column=2, sticky="nsew", padx=(0, 10))
        ttk.Label(card_headroom, text="HEADROOM", style="MetricTitle.TLabel").pack(anchor="w")
        headroom_row = ttk.Frame(card_headroom, style="Card.TFrame")
        headroom_row.pack(anchor="w", pady=(10, 4))
        self.headroom_value = ttk.Label(headroom_row, text="0", style="MetricValue.TLabel")
        self.headroom_value.pack(side="left")
        ttk.Label(headroom_row, text="TKN", style="MetricUnit.TLabel").pack(side="left", padx=(6, 0), pady=(9, 0))
        self.headroom_detail = ttk.Label(card_headroom, text="", style="MetricDetail.TLabel")
        self.headroom_detail.pack(anchor="w")

        self.status_card = ttk.Frame(metrics_row, style="Card.TFrame", padding=(0, 10))
        self.status_card.grid(row=0, column=3, sticky="nsew")
        self.status_card.columnconfigure(1, weight=1)
        self.status_accent = tk.Frame(self.status_card, bg="#16d9f5", width=2)
        self.status_accent.grid(row=0, column=0, rowspan=3, sticky="ns", padx=(0, 10))
        ttk.Label(self.status_card, text="STATUS", style="MetricTitle.TLabel").grid(row=0, column=1, sticky="w", padx=(2, 0))
        status_value_row = ttk.Frame(self.status_card, style="Card.TFrame")
        status_value_row.grid(row=1, column=1, sticky="w", padx=(2, 0), pady=(10, 0))
        self.status_dot = tk.Frame(status_value_row, bg="#16d9f5", width=7, height=7)
        self.status_dot.pack(side="left", padx=(0, 8), pady=(3, 0))
        self.status_metric_value = ttk.Label(status_value_row, text="Awaiting data", style="StatusValue.TLabel")
        self.status_metric_value.pack(side="left")
        self.status_metric_detail = ttk.Label(self.status_card, text="No ingest cycle completed yet.", style="StatusDetail.TLabel")
        self.status_metric_detail.grid(row=2, column=1, sticky="w", padx=(19, 0), pady=(4, 0))

        chart_header = ttk.Frame(body, style="BodyPanel.TFrame")
        chart_header.pack(fill="x", pady=(0, 8))
        self.chart_header_title = ttk.Label(
            chart_header,
            text=format_chart_title(
                self.selected_interval,
                self.selected_chart_mode,
                self.selected_metric_mode,
            ),
            style="ChartTitle.TLabel",
        )
        self.chart_header_title.pack(side="left")
        self.chart_header_context = ttk.Label(chart_header, text=self._timezone_label(), style="Tiny.TLabel")
        self.chart_header_context.pack(side="right")

        chart_shell = ttk.Frame(body, style="Shell.TFrame", padding=(10, 10))
        chart_shell.pack(fill="both", expand=True)
        self.canvas = tk.Canvas(
            chart_shell,
            width=880,
            height=270,
            bg="#10141a",
            highlightthickness=0,
        )
        self.canvas.pack(fill="both", expand=True)
        self.canvas.bind("<Motion>", self._on_chart_motion)
        self.canvas.bind("<Leave>", self._on_chart_leave)
        self.canvas.bind("<Button-3>", self._on_chart_right_click)
        self.chart_context_menu = tk.Menu(self.overlay, tearoff=0)
        self.chart_context_menu.add_command(
            label="Investigate with Codex",
            command=self._investigate_selected_bucket,
        )

        info_row = ttk.Frame(body, style="BodyPanel.TFrame")
        info_row.pack(fill="x", pady=(12, 0))
        info_row.columnconfigure(0, weight=1)
        self.advisory_label = ttk.Label(
            info_row,
            text="No weekly advisory yet.",
            style="Status.TLabel",
            wraplength=720,
            justify="left",
        )
        self.advisory_label.grid(row=0, column=0, sticky="w")
        self.hotkey_label = ttk.Label(
            info_row,
            text=f"Toggle: {self.config.hotkey}",
            style="Tiny.TLabel",
        )
        self.hotkey_label.grid(row=0, column=1, sticky="e", padx=(12, 0))

        self._refresh_interval_buttons()
        self._refresh_chart_mode_buttons()
        self._refresh_metric_mode_buttons()
        self._build_jobs_lane()
        self._build_worktrees_lane()
        self._refresh_tab_buttons()
        self._render_active_tab()

    def _build_jobs_lane(self) -> None:
        self.jobs_body = ttk.Frame(self.content_stack, style="BodyPanel.TFrame", padding=(16, 14))

        # Action row: REFRESH (re-read) + UPDATE (apply Git desired state to Temporal),
        # right-aligned above the summary cards (mirrors the WORKTREES toolbar button row).
        action_row = ttk.Frame(self.jobs_body, style="BodyPanel.TFrame")
        action_row.pack(fill="x", pady=(0, 12))
        actions = tk.Frame(action_row, bg="#11151b")
        actions.pack(side="right")
        self._cta_button(
            actions, "REFRESH", self.refresh_jobs_data, bg="#11151b",
            top="#3a3f47", bottom="#23272d", fg="#cdd6da",
            hover_top="#454b54", hover_bottom="#2d323a", icon=None,
        ).pack(side="left", padx=(0, 10))
        self._cta_button(
            actions, "UPDATE", lambda: self.refresh_jobs_data(apply_changes=True), bg="#11151b",
            top="#c3f5ff", bottom="#00e5ff", fg="#00363d",
            hover_top="#d8faff", hover_bottom="#2ee8ff", icon="sync", icon_side="left",
        ).pack(side="left")

        # Summary cards in the WORKTREES format (three equal cards): TOTAL_JOBS / IN_SYNC /
        # NEEDS_ATTENTION. No tab title — the active tab already names the surface.
        summary_row = ttk.Frame(self.jobs_body, style="BodyPanel.TFrame")
        summary_row.pack(fill="x", pady=(0, 12))
        self.jobs_metric_values: dict[str, ttk.Label] = {}
        for column, (key, title) in enumerate(
            (("total", "TOTAL_JOBS"), ("in_sync", "IN_SYNC"), ("attention", "NEEDS_ATTENTION"))
        ):
            summary_row.columnconfigure(column, weight=1)
            self.jobs_metric_values[key] = self._build_worktrees_summary_card(summary_row, column, title, "0")

        # Full-width jobs table (NEXT shows the next scheduled run time).
        self._jobs_columns = (("JOB_ID", 312), ("NEXT", 90), ("STATUS", 110), ("LAST_RUN", 130))
        table_head = tk.Frame(self.jobs_body, bg="#262a31")
        table_head.pack(fill="x")
        head_inner = tk.Frame(table_head, bg="#262a31")
        head_inner.pack(fill="x", padx=14, pady=9)
        tk.Label(head_inner, text="ACTIONS", bg="#262a31", fg="#adcbda", font=("Inter", -11)).pack(side="right")
        for text, width in self._jobs_columns:
            cell = tk.Frame(head_inner, bg="#262a31", width=width, height=14)
            cell.pack(side="left")
            cell.pack_propagate(False)
            tk.Label(cell, text=text, bg="#262a31", fg="#adcbda", font=("Inter", -11)).pack(side="left")

        # Scrollable rows.
        rows_shell = tk.Frame(self.jobs_body, bg="#0a0e14")
        rows_shell.pack(fill="both", expand=True)
        self.jobs_scroll_canvas = tk.Canvas(rows_shell, bg="#0a0e14", highlightthickness=0, borderwidth=0)
        self.jobs_scrollbar = ttk.Scrollbar(rows_shell, orient="vertical", command=self.jobs_scroll_canvas.yview)
        self.jobs_scrollbar.pack(side="right", fill="y")
        self.jobs_scroll_canvas.pack(side="left", fill="both", expand=True)
        self.jobs_scroll_canvas.configure(yscrollcommand=self.jobs_scrollbar.set)
        self.jobs_rows_container = tk.Frame(self.jobs_scroll_canvas, bg="#0a0e14")
        self.jobs_scroll_window = self.jobs_scroll_canvas.create_window(
            (0, 0), window=self.jobs_rows_container, anchor="nw"
        )
        self.jobs_rows_container.bind("<Configure>", self._refresh_jobs_scroll_region)
        self.jobs_scroll_canvas.bind("<Configure>", self._resize_jobs_scroll_content)
        self.jobs_scroll_canvas.bind("<MouseWheel>", self._on_jobs_mousewheel)
        self.jobs_rows_container.bind("<MouseWheel>", self._on_jobs_mousewheel)

    def _build_worktrees_lane(self) -> None:
        # Task-0016 (D1=replace): this lane replaces the old TASKS tab's task
        # stream/detail/dispatch-pause-poke surface with the worktree-pool management
        # surface. The task lifecycle now lives on the GitHub Issues queue surface; this
        # tab is the backend worktree pool the operator drives.
        self.worktrees_body = ttk.Frame(self.content_stack, style="BodyPanel.TFrame", padding=(16, 14))

        header = ttk.Frame(self.worktrees_body, style="BodyPanel.TFrame")
        header.pack(fill="x", pady=(0, 12))
        header.columnconfigure(0, weight=1)
        title_copy = ttk.Frame(header, style="BodyPanel.TFrame")
        title_copy.grid(row=0, column=0, sticky="w")
        ttk.Label(title_copy, text="Worktrees", style="ChartTitle.TLabel").pack(anchor="w")
        self.worktrees_freshness_label = ttk.Label(
            title_copy,
            text="The worktree pool has not been refreshed yet.",
            style="Status.TLabel",
        )
        self.worktrees_freshness_label.pack(anchor="w", pady=(4, 0))
        # Action outcomes (Assign / Eject / Destroy rejection / Dequeue) are surfaced here
        # in-frame so the operator sees the result on the WORKTREES tab itself; the global
        # status_label is not laid out on this tab. Re-rendering the pool does NOT clear it.
        self.worktrees_action_status_label = ttk.Label(
            title_copy,
            text="",
            style="Status.TLabel",
            wraplength=760,
            justify="left",
        )
        self.worktrees_action_status_label.pack(anchor="w", pady=(2, 0))
        self.worktrees_refresh_button = self._cta_button(
            header, "REFRESH", self.refresh_worktrees_data, bg="#1c2026",
            top="#3a3f47", bottom="#23272d", fg="#cdd6da",
            hover_top="#454b54", hover_bottom="#2d323a", icon=None,
        )
        self.worktrees_refresh_button.grid(row=0, column=1, sticky="e")

        toolbar = ttk.Frame(self.worktrees_body, style="Shell.TFrame", padding=(10, 10))
        toolbar.pack(fill="x", pady=(0, 12))
        ttk.Label(toolbar, text="REPO", style="Tiny.TLabel").pack(side="left", padx=(0, 6))
        self.worktrees_repo_filter_var = tk.StringVar(value=ALL_REPOS_OPTION)
        self.worktrees_repo_combo = ttk.Combobox(
            toolbar,
            textvariable=self.worktrees_repo_filter_var,
            state="readonly",
            width=28,
            style="Worktrees.TCombobox",
            values=[ALL_REPOS_OPTION],
        )
        self.worktrees_repo_combo.pack(side="left", padx=(0, 12))
        self.worktrees_repo_combo.bind("<<ComboboxSelected>>", self._on_worktrees_repo_filter_changed)
        self.worktrees_create_button = self._cta_button(
            toolbar, "NEW WORKTREE", self.create_worktree_for_selected_repo, bg="#1c2026",
            top="#c3f5ff", bottom="#00e5ff", fg="#00363d",
            hover_top="#d8faff", hover_bottom="#2ee8ff", icon="plus", icon_side="left",
        )
        self.worktrees_create_button.pack(side="right")

        summary_row = ttk.Frame(self.worktrees_body, style="BodyPanel.TFrame")
        summary_row.pack(fill="x", pady=(0, 12))
        self.worktrees_summary_values: dict[str, ttk.Label] = {}
        for column, (key, title) in enumerate(
            (("total", "POOL SIZE"), ("allocated", "ALLOCATED"), ("idle", "IDLE"))
        ):
            summary_row.columnconfigure(column, weight=1)
            self.worktrees_summary_values[key] = self._build_worktrees_summary_card(
                summary_row, column, title, "0"
            )

        stream_shell = ttk.Frame(self.worktrees_body, style="Shell.TFrame", padding=(10, 10))
        stream_shell.pack(fill="both", expand=True)
        self.worktrees_scroll_canvas = tk.Canvas(
            stream_shell,
            bg="#11151b",
            highlightthickness=0,
            borderwidth=0,
        )
        self.worktrees_scroll_canvas.pack(side="left", fill="both", expand=True)
        self.worktrees_scrollbar = ttk.Scrollbar(
            stream_shell,
            orient="vertical",
            command=self.worktrees_scroll_canvas.yview,
        )
        self.worktrees_scrollbar.pack(side="right", fill="y")
        self.worktrees_scroll_canvas.configure(yscrollcommand=self.worktrees_scrollbar.set)
        self.worktrees_rows_content = ttk.Frame(self.worktrees_scroll_canvas, style="Shell.TFrame")
        self.worktrees_scroll_window = self.worktrees_scroll_canvas.create_window(
            (0, 0),
            window=self.worktrees_rows_content,
            anchor="nw",
        )
        self.worktrees_rows_content.bind("<Configure>", self._refresh_worktrees_scroll_region)
        self.worktrees_scroll_canvas.bind("<Configure>", self._resize_worktrees_scroll_content)
        self.worktrees_scroll_canvas.bind("<MouseWheel>", self._on_worktrees_mousewheel)
        self.worktrees_rows_content.bind("<MouseWheel>", self._on_worktrees_mousewheel)

    def _build_worktrees_summary_card(
        self,
        parent: ttk.Frame,
        column: int,
        title: str,
        value: str,
    ) -> ttk.Label:
        card = ttk.Frame(parent, style="Card.TFrame", padding=(12, 10))
        card.grid(row=0, column=column, sticky="nsew", padx=(0, 10) if column < 2 else (0, 0))
        ttk.Label(card, text=title, style="MetricTitle.TLabel").pack(anchor="w")
        value_label = ttk.Label(card, text=value, style="MetricValue.TLabel")
        value_label.pack(anchor="w", pady=(8, 0))
        return value_label

    def select_tab(self, tab_id: str) -> None:
        self.active_tab = tab_id
        if tab_id == "jobs":
            self._prime_jobs_snapshot()
        if tab_id == "worktrees":
            self._prime_worktrees_snapshot()
        # Task-0014: re-apply the tab-aware geometry on every tab change. This is a
        # cheap geometry-only call and must NOT rebuild, re-aggregate, or re-fetch
        # tab data (Task-0013 cheap show/hide behavior is preserved).
        self._apply_overlay_geometry()
        self._render_active_tab()

    def _render_active_tab(self) -> None:
        self._refresh_tab_buttons()
        if self.active_tab == "jobs":
            self.usage_body.pack_forget()
            self.worktrees_body.pack_forget()
            self.jobs_body.pack(fill="both", expand=True)
            return
        if self.active_tab == "worktrees":
            self.usage_body.pack_forget()
            self.jobs_body.pack_forget()
            self.worktrees_body.pack(fill="both", expand=True)
            return
        self.jobs_body.pack_forget()
        self.worktrees_body.pack_forget()
        self.usage_body.pack(fill="both", expand=True)

    def _refresh_tab_buttons(self) -> None:
        for tab_id, label in self.tab_buttons.items():
            is_active = tab_id == self.active_tab
            label.configure(
                fg=TAB_ACTIVE_FOREGROUND if is_active else TAB_INACTIVE_FOREGROUND,
            )
            self.tab_underlines[tab_id].configure(
                bg=TAB_ACTIVE_UNDERLINE if is_active else HEADER_BACKGROUND,
            )

    def refresh_worktrees_data(self) -> None:
        self._reload_worktrees_snapshot(refreshed=True)
        self._render_worktrees_snapshot()

    def _prime_worktrees_snapshot(self) -> None:
        if list(self.worktrees_snapshot.get("worktrees", [])):
            self._render_worktrees_snapshot()
            return
        self._reload_worktrees_snapshot(refreshed=False)
        self._render_worktrees_snapshot()

    def _reload_worktrees_snapshot(self, refreshed: bool, keep_status: bool = False) -> None:
        # keep_status=True is used after a mutating action so the action's own outcome
        # message (e.g. an allocated-Destroy rejection) is NOT overwritten by a generic
        # "pool refreshed" line; the action sets the status itself after the reload.
        try:
            self.worktrees_snapshot = fetch_pool_snapshot(self.worktrees_backend_url)
            if not keep_status:
                self.worktrees_status_message = (
                    "Worktree pool refreshed from orchestration backend."
                    if refreshed
                    else "Worktree pool loaded from orchestration backend."
                )
        except Exception as exc:
            self.worktrees_snapshot = worktrees_backend_error_snapshot(str(exc))
            if not keep_status:
                self.worktrees_status_message = f"Worktrees error: {exc}"
        # The repo filter is sourced from the registry; reload it alongside the pool. A
        # repo-list failure leaves the pool readable with an All-repos-only dropdown.
        try:
            self.worktrees_repos = fetch_repos(self.worktrees_backend_url)
        except Exception:
            self.worktrees_repos = []
        self._refresh_repo_filter_options()
        if self.active_tab == "worktrees" and not keep_status:
            self.status_label.configure(text=self.worktrees_status_message)

    def _refresh_repo_filter_options(self) -> None:
        options = repo_filter_options(self.worktrees_repos)
        self.worktrees_repo_combo.configure(values=options)
        if self.worktrees_repo_filter not in options:
            self.worktrees_repo_filter = ALL_REPOS_OPTION
        self.worktrees_repo_filter_var.set(self.worktrees_repo_filter)

    def _on_worktrees_repo_filter_changed(self, _event=None) -> None:
        self.worktrees_repo_filter = self.worktrees_repo_filter_var.get()
        # Filtering is a view-only re-render of the already-loaded snapshot; it performs
        # no backend mutation.
        self._render_worktrees_snapshot()

    def _visible_worktrees(self) -> list[dict[str, object]]:
        worktrees = [
            worktree
            for worktree in list(self.worktrees_snapshot.get("worktrees", []))
            if isinstance(worktree, dict)
        ]
        return filter_worktrees_by_repo(worktrees, self.worktrees_repo_filter, self.worktrees_repos)

    def _render_worktrees_snapshot(self) -> None:
        visible = self._visible_worktrees()
        counts = worktree_summary_counts(visible)
        for key in ("total", "allocated", "idle"):
            self.worktrees_summary_values[key].configure(text=f"{int(counts.get(key, 0)):02d}")
        self.worktrees_freshness_label.configure(
            text=str(self.worktrees_snapshot.get("message", self.worktrees_status_message))
        )

        for child in self.worktrees_rows_content.winfo_children():
            child.destroy()

        if not visible:
            # When the backend is reachable but the (filtered) pool is empty, point the
            # operator at CREATE WORKTREE rather than the generic refresh line.
            if str(self.worktrees_snapshot.get("status") or "") == "ok":
                empty_text = "No worktrees in this repo yet - use NEW WORKTREE to add one."
            else:
                empty_text = self.worktrees_status_message
            ttk.Label(
                self.worktrees_rows_content,
                text=empty_text,
                style="Status.TLabel",
                wraplength=760,
                justify="left",
            ).pack(anchor="w", padx=12, pady=(12, 0))
            return

        for worktree in visible:
            self._build_worktree_row(worktree)

    def _build_worktree_row(self, worktree: dict[str, object]) -> None:
        # The worktree panel matches the Stitch "Monolithic Terminal" mockup: ONE horizontal
        # row — a short accent stripe, then the repo (UPPERCASE) + status chip over the full
        # monospace path with an inline copy control, and the action(s) right-justified
        # (Assign for idle; Eject + Dequeue for allocated, with the bound task id linking to
        # its GitHub issue). Allocated/idle/needs-review reads from the stripe + chip color
        # (cyan / gray / red). No bottom button drawer.
        row_bg = worktree_status_background(worktree)
        accent_color = worktree_status_color(worktree)
        row = tk.Frame(self.worktrees_rows_content, bg=row_bg, padx=12, pady=10)
        row.pack(fill="x", pady=(0, 8))
        row.bind("<MouseWheel>", self._on_worktrees_mousewheel)

        # A wider (2x), shorter (66% height), vertically-centered accent stripe in a fixed
        # gutter — a status bar, not a full-height hairline.
        gutter = tk.Frame(row, bg=row_bg, width=6)
        gutter.pack(side="left", fill="y", padx=(0, 12))
        gutter.pack_propagate(False)
        stripe = tk.Frame(gutter, bg=accent_color)
        stripe.place(relx=0.0, rely=0.5, anchor="w", relheight=0.66, relwidth=1.0)

        # Right-justified action cluster (packed before content so content fills the middle).
        actions = tk.Frame(row, bg=row_bg)
        actions.pack(side="right")
        self._build_worktree_row_actions(actions, worktree, row_bg)

        content = tk.Frame(row, bg=row_bg)
        content.pack(side="left", fill="x", expand=True)

        head = tk.Frame(content, bg=row_bg)
        head.pack(anchor="w", fill="x")
        # Repo name FIRST, UPPERCASE (mockup): the accent color when allocated, white when idle.
        repo_label = tk.Label(
            head,
            text=(worktree_heading_repo(worktree) or "unknown repo").upper(),
            bg=row_bg,
            fg=accent_color if is_allocated(worktree) else "#dfe2eb",
            font=("Space Grotesk", 12, "bold"),
            anchor="w",
        )
        repo_label.pack(side="left")
        chip = self._build_worktree_status_chip(head, worktree)
        chip.pack(side="left", padx=(10, 0))
        # The bound-task id rides the heading line as a link to its running GitHub issue
        # (keeping the full path + copy control unobstructed on the line below).
        if is_allocated(worktree) and str(worktree.get("task_id") or ""):
            self._build_worktree_task_link(head, worktree, row_bg).pack(side="left", padx=(12, 0))

        # The FULL local dir in monospace white, with the copy control inline at the END of
        # the path (mockup), plus a full-path hover tooltip.
        path_row = tk.Frame(content, bg=row_bg)
        path_row.pack(anchor="w", fill="x", pady=(5, 0))
        full_path = str(worktree.get("worktree_path") or "")
        path_label = tk.Label(
            path_row,
            text=full_path or "(no path)",
            bg=row_bg,
            fg="#dfe2eb",
            font=("Consolas", -12),
            anchor="w",
        )
        path_label.pack(side="left")
        path_label.bind("<MouseWheel>", self._on_worktrees_mousewheel)
        if full_path:
            self._bind_tooltip(path_label, full_path)
            self._worktree_icon_button(
                path_row,
                "copy",
                lambda path=full_path: self.copy_worktree_path(path),
                "Copy path",
                row_bg,
            ).pack(side="left", padx=(8, 0))

        for widget in (content, head, chip, repo_label, gutter, stripe, actions):
            widget.bind("<MouseWheel>", self._on_worktrees_mousewheel)

    def _build_worktree_status_chip(self, parent: tk.Misc, worktree: dict[str, object]) -> tk.Label:
        # A solid 0px-radius status pill (mockup): the bright states (allocated cyan,
        # needs-review red) fill the chip with dark uppercase text; the idle chip stays a
        # muted gray-on-gray so idle reads as quiet next to an active slot.
        if is_allocated(worktree):
            chip_bg = worktree_status_color(worktree)
            chip_fg = "#10141a"
        else:
            chip_bg = "#31353c"
            chip_fg = "#bac9cc"
        return tk.Label(
            parent,
            text=worktree_status_label(worktree),
            bg=chip_bg,
            fg=chip_fg,
            font=("Space Grotesk", 8, "bold"),
            padx=6,
            pady=1,
        )

    def _build_worktree_task_link(self, parent: tk.Misc, worktree: dict[str, object], row_bg: str) -> tk.Label:
        # The bound-task id as a clickable link to its running GitHub issue (cyan +
        # underline). Falls back to plain white text when the issue URL can't be resolved.
        task_id = str(worktree.get("task_id") or "")
        issue_url = worktree_issue_url(worktree, self.worktrees_repos)
        label = tk.Label(
            parent,
            text=task_id,
            bg=row_bg,
            fg="#00e5ff" if issue_url else "#dfe2eb",
            font=("Consolas", -12, "underline") if issue_url else ("Consolas", -12),
            cursor="hand2" if issue_url else "",
        )
        label.bind("<MouseWheel>", self._on_worktrees_mousewheel)
        if issue_url:
            label.bind("<Button-1>", lambda _e, url=issue_url: webbrowser.open(url))
            self._bind_tooltip(label, f"Open issue: {issue_url}")
        return label

    def _build_worktree_row_actions(self, parent: tk.Frame, worktree: dict[str, object], row_bg: str) -> None:
        # The right-justified action cluster (mockup): no bottom drawer. Allocated shows a
        # Details icon + DEQUEUE + a red EJECT button; idle shows a Details icon + a Destroy
        # (trash) icon + the ASSIGN button. (The bound-task link lives on the heading line.)
        worktree_id = str(worktree.get("worktree_id") or "")
        if is_allocated(worktree):
            run_id = str(worktree.get("run_id") or "")
            task_id = str(worktree.get("task_id") or "")
            self._worktree_icon_button(
                parent, "details", lambda wt=dict(worktree): self.open_worktree_details(wt),
                "Details", row_bg,
            ).pack(side="left", padx=(0, 10))
            self._worktree_icon_button(
                parent, "launch", lambda wt=dict(worktree): self.open_worktree_session_action(wt),
                "Open session in VSCodium", row_bg,
            ).pack(side="left", padx=(0, 10))
            self._cta_button(
                parent, "DEQUEUE", lambda tid=task_id: self.dequeue_task_action(tid), bg=row_bg,
                top="#3a3f47", bottom="#23272d", fg="#dfe2eb",
                hover_top="#454b54", hover_bottom="#2d323a", icon=None,
                disabled=not task_id,
            ).pack(side="left", padx=(0, 8))
            self._cta_button(
                parent, "EJECT", lambda rid=run_id, wid=worktree_id: self.eject_worktree_action(rid, wid),
                bg=row_bg, top="#b3474a", bottom="#5a1f22", fg="#ffe7e4",
                hover_top="#c85457", hover_bottom="#742a2e", icon="eject", icon_side="left",
            ).pack(side="left")
        else:
            self._worktree_icon_button(
                parent, "details", lambda wt=dict(worktree): self.open_worktree_details(wt),
                "Details", row_bg,
            ).pack(side="left", padx=(0, 10))
            self._worktree_icon_button(
                parent, "destroy", lambda wid=worktree_id: self.destroy_worktree_action(wid),
                "Destroy", row_bg,
            ).pack(side="left", padx=(0, 12))
            self._cta_button(
                parent, "ASSIGN", lambda wt=dict(worktree): self.open_assign_popup(wt), bg=row_bg,
                top="#c3f5ff", bottom="#00e5ff", fg="#00363d",
                hover_top="#d8faff", hover_bottom="#2ee8ff", icon=None,
            ).pack(side="left")

    def _icon_photo(self, kind: str, color: str, size: int):
        # Cache + retain crisp PIL-supersampled glyph images (the cache dict keeps the
        # PhotoImage refs alive so Tk does not garbage-collect them). Returns None when
        # Pillow is unavailable, so callers can fall back to Canvas line art.
        cache = getattr(self, "_icon_image_cache", None)
        if cache is None:
            cache = {}
            self._icon_image_cache = cache
        key = (kind, color, int(size))
        if key not in cache:
            try:
                from PIL import ImageTk

                from .glyphs import render_glyph

                cache[key] = ImageTk.PhotoImage(render_glyph(kind, color, int(size)))
            except Exception:
                cache[key] = None
        return cache[key]

    def _worktree_icon_button(self, parent, kind, command, tooltip, row_bg, size: int = 17):
        # A borderless 0px-radius icon button: a crisp PIL-supersampled glyph (no Material
        # font / emoji), muted at rest and accent-on-hover (red for destroy). The tooltip
        # names the action so the glyph is never ambiguous. Falls back to Canvas line art if
        # Pillow is unavailable.
        rest = "#849396"
        hot = "#ffb4ab" if kind == "destroy" else "#00e5ff"
        rest_img = self._icon_photo(kind, rest, size)
        hot_img = self._icon_photo(kind, hot, size)
        if rest_img is not None:
            button = tk.Label(parent, image=rest_img, bg=row_bg, bd=0, cursor="hand2")
            button.bind("<Enter>", lambda _e: button.configure(image=hot_img))
            button.bind("<Leave>", lambda _e: button.configure(image=rest_img))
        else:
            button = tk.Canvas(
                parent, width=size, height=size, bg=row_bg, highlightthickness=0, bd=0, cursor="hand2"
            )
            self._draw_worktree_icon(button, kind, rest)
            button.bind("<Enter>", lambda _e: (button.delete("all"), self._draw_worktree_icon(button, kind, hot)))
            button.bind("<Leave>", lambda _e: (button.delete("all"), self._draw_worktree_icon(button, kind, rest)))
        button.bind("<Button-1>", lambda _e: command())
        button.bind("<MouseWheel>", self._on_worktrees_mousewheel)
        self._bind_tooltip(button, tooltip)
        return button

    def _draw_worktree_icon(self, canvas: tk.Canvas, kind: str, color: str) -> None:
        if kind == "copy":
            # content_copy: a back sheet peeking behind a front sheet.
            canvas.create_rectangle(6, 6, 15, 15, outline=color, width=1)
            canvas.create_rectangle(3, 3, 12, 12, outline=color, width=1)
        elif kind == "details":
            # information "i" in a circle.
            canvas.create_oval(2, 2, 16, 16, outline=color, width=1)
            canvas.create_oval(8, 4, 10, 6, fill=color, outline=color)
            canvas.create_line(9, 8, 9, 13, fill=color, width=1)
        elif kind == "destroy":
            # a trash can: lid + handle + tapered body + ribs.
            canvas.create_line(3, 5, 15, 5, fill=color, width=1)
            canvas.create_line(7, 3, 11, 3, fill=color, width=2)
            canvas.create_line(5, 5, 6, 15, fill=color, width=1)
            canvas.create_line(13, 5, 12, 15, fill=color, width=1)
            canvas.create_line(6, 15, 12, 15, fill=color, width=1)
            canvas.create_line(9, 7, 9, 13, fill=color, width=1)
        elif kind == "launch":
            # open_in_new: a box opened at the top-right with a NE arrow.
            canvas.create_line(3, 7, 3, 15, 13, 15, 13, 9, fill=color, width=1)
            canvas.create_line(3, 7, 7, 7, fill=color, width=1)
            canvas.create_line(8, 9, 15, 3, fill=color, width=1)
            canvas.create_line(11, 3, 15, 3, 15, 7, fill=color, width=1)

    def copy_worktree_path(self, path: str) -> None:
        if not path:
            self._set_worktrees_status("No worktree path to copy.")
            return
        self.overlay.clipboard_clear()
        self.overlay.clipboard_append(path)
        self._set_worktrees_status(f"Copied worktree path to clipboard: {path}")

    def _bind_tooltip(self, widget: tk.Widget, text: str) -> None:
        # A lightweight hover tooltip: the short on-face path stays glanceable while the
        # full path is revealed on mouseover (UPDATE 5 information hierarchy).
        tip: dict[str, tk.Toplevel | None] = {"win": None}

        def show(_event=None) -> None:
            if tip["win"] is not None:
                return
            win = tk.Toplevel(self.overlay)
            win.wm_overrideredirect(True)
            win.configure(bg="#353940")
            tk.Label(
                win,
                text=text,
                bg="#353940",
                fg="#dfe2eb",
                font=("Inter", 9),
                justify="left",
                padx=8,
                pady=4,
            ).pack()
            win.wm_geometry(f"+{widget.winfo_rootx()}+{widget.winfo_rooty() + widget.winfo_height() + 2}")
            tip["win"] = win

        def hide(_event=None) -> None:
            if tip["win"] is not None:
                tip["win"].destroy()
                tip["win"] = None

        widget.bind("<Enter>", show)
        widget.bind("<Leave>", hide)

    def open_worktree_session_action(self, worktree: dict[str, object]) -> None:
        # Open the allocated worktree's running CLAUDE SESSION in VSCodium's Claude extension
        # (anthropic.claude-code) so the operator can read the agent's chat/thinking and
        # continue it. The extension resolves a session against the OPEN workspace, so open the
        # worktree folder first, then fire vscodium://anthropic.claude-code/open?session=<id>.
        # If the backend captured no session id for this worktree (e.g. it was assigned without
        # an agent ever being launched into it), say so honestly — do NOT open a weaker proxy.
        session_id = str(worktree.get("agent_session_id") or "").strip()
        workspace = worktree_session_target(worktree)
        label = str(worktree.get("task_id") or worktree.get("worktree_id") or "session")
        if not session_id:
            self._set_worktrees_status(
                f"No Claude session is bound to {label} yet — nothing to open. "
                "(A worktree gets a session only once an agent is launched into it.)"
            )
            return
        try:
            if workspace:
                webbrowser.open(vscodium_uri(workspace))
            webbrowser.open(claude_session_uri(session_id))
            self._set_worktrees_status(f"Opening {label}'s Claude session in VSCodium…")
        except Exception as exc:
            self._set_worktrees_status(f"Could not open VSCodium: {exc}")

    def open_worktree_details(self, worktree: dict[str, object]) -> None:
        # The explicit Details reveal: the full secondary/diagnostic fields the panel face
        # intentionally omits (full path, ids, run/gate, agent session, transcript, PID).
        popup = tk.Toplevel(self.overlay)
        popup.title("Worktree details")
        popup.configure(bg="#1c2026")
        popup.transient(self.overlay)
        popup.attributes("-topmost", True)
        popup.geometry("560x360")

        head = tk.Frame(popup, bg="#1c2026")
        head.pack(fill="x", padx=16, pady=(16, 8))
        self._build_worktree_status_chip(head, worktree).pack(side="left")
        tk.Label(
            head,
            text=worktree_heading_repo(worktree) or "unknown repo",
            bg="#1c2026",
            fg="#dfe2eb",
            font=("Space Grotesk", 11, "bold"),
        ).pack(side="left", padx=(10, 0))

        body = tk.Frame(popup, bg="#10141a")
        body.pack(fill="both", expand=True, padx=16, pady=(0, 8))
        for label, value in worktree_detail_lines(worktree):
            line = tk.Frame(body, bg="#10141a")
            line.pack(anchor="w", fill="x", padx=10, pady=3)
            tk.Label(line, text=f"{label}:", bg="#10141a", fg="#6e8598", font=("Inter", 9), width=14, anchor="w").pack(side="left")
            tk.Label(
                line,
                text=value,
                bg="#10141a",
                fg="#dfe2eb",
                font=("Inter", 9),
                anchor="w",
                justify="left",
                wraplength=400,
            ).pack(side="left", fill="x", expand=True)

        ttk.Button(popup, text="CLOSE", style="Quiet.TButton", command=popup.destroy).pack(anchor="e", padx=16, pady=(0, 14))

    def create_worktree_for_selected_repo(self) -> None:
        repo_id = self._selected_repo_id()
        if not repo_id:
            self._set_worktrees_status("Select a repo in the filter before creating a worktree.")
            return
        try:
            create_worktree(repo_id, self.worktrees_backend_url)
            self._reload_worktrees_snapshot(refreshed=True, keep_status=True)
            self._set_worktrees_status(f"Created a new idle worktree in {repo_id}.")
        except Exception as exc:
            self._reload_worktrees_snapshot(refreshed=True, keep_status=True)
            self._set_worktrees_status(f"Create failed: {exc}")
        self._render_worktrees_snapshot()

    def eject_worktree_action(self, run_id: str, worktree_id: str) -> None:
        try:
            eject_worktree(run_id, worktree_id, self.worktrees_backend_url)
            self._reload_worktrees_snapshot(refreshed=True, keep_status=True)
            self._set_worktrees_status(f"Ejected {worktree_id}; it is idle and the task is dequeued.")
        except Exception as exc:
            self._reload_worktrees_snapshot(refreshed=True, keep_status=True)
            self._set_worktrees_status(f"Eject failed: {exc}")
        self._render_worktrees_snapshot()

    def destroy_worktree_action(self, worktree_id: str) -> None:
        try:
            destroy_worktree(worktree_id, self.worktrees_backend_url)
            self._reload_worktrees_snapshot(refreshed=True, keep_status=True)
            self._set_worktrees_status(f"Destroyed idle worktree {worktree_id}.")
        except Exception as exc:
            # The backend rejects destroying an allocated worktree (409); surface it.
            self._reload_worktrees_snapshot(refreshed=True, keep_status=True)
            self._set_worktrees_status(f"Destroy failed: {exc}")
        self._render_worktrees_snapshot()

    def dequeue_task_action(self, task_id: str) -> None:
        if not task_id:
            self._set_worktrees_status("No task is bound to dequeue.")
            return
        repo_id = self._selected_repo_id()
        try:
            dequeue_task(repo_id, task_id, self.worktrees_backend_url)
            self._reload_worktrees_snapshot(refreshed=True, keep_status=True)
            self._set_worktrees_status(f"Dequeued {task_id} (Queue=Never); the issue stays open.")
        except Exception as exc:
            self._reload_worktrees_snapshot(refreshed=True, keep_status=True)
            self._set_worktrees_status(f"Dequeue failed: {exc}")
        self._render_worktrees_snapshot()

    def open_assign_popup(self, worktree: dict[str, object]) -> None:
        worktree_id = str(worktree.get("worktree_id") or "")
        repo_for_assign = self._selected_repo_id() or str(worktree.get("repo") or "")
        try:
            tasks_snapshot = fetch_tasks_snapshot(self.worktrees_backend_url)
        except Exception as exc:
            self._set_worktrees_status(f"Could not load open tasks: {exc}")
            return
        options = open_task_options(tasks_snapshot)

        # Modal conforming to the ASSIGN_TASK_OPERATOR Stitch mockup: a cyan top border, a
        # surface-container-high header (assign icon + title + Target-Worktree subtitle +
        # close X), a filter/sort toolbar, a scrollable list of rich task cards (id + status
        # chip + description; selected = cyan border/radio, blocked = disabled/orange), and a
        # pinned footer (CANCEL + cyan BIND_TASK). Tonal layering, 0px radius, no dividers.
        popup = tk.Toplevel(self.overlay)
        popup.title("Assign Task")
        popup.configure(bg="#1c2026")
        popup.transient(self.overlay)
        popup.attributes("-topmost", True)
        # Borderless modal: drop the OS title bar so the cyan top border is the only chrome
        # (the in-modal close X / CANCEL dismiss it), matching the mockup and the overlay.
        popup.overrideredirect(True)
        self.overlay.update_idletasks()
        ow, oh = self.overlay.winfo_width(), self.overlay.winfo_height()
        popup_w = min(820, max(560, ow - 80))
        popup_h = min(640, max(440, oh - 80))
        ox, oy = self.overlay.winfo_rootx(), self.overlay.winfo_rooty()
        cx = ox + max(0, (ow - popup_w) // 2)
        cy = oy + max(0, (oh - popup_h) // 2)
        popup.geometry(f"{popup_w}x{popup_h}+{cx}+{cy}")
        popup.update_idletasks()
        # NOT grab_set(): a modal grab on this borderless topmost popup froze every click in
        # the overlay (the popup can sit behind the topmost overlay and swallow all input).
        # The popup is non-modal; re-assert topmost + lift so it rides above the overlay, and
        # Escape (plus the close X / CANCEL) dismiss it.
        popup.attributes("-topmost", True)
        popup.lift()
        popup.focus_force()
        popup.bind("<Escape>", lambda _e: popup.destroy())

        # Cyan top border (mockup: border-t-2 border-primary-container).
        tk.Frame(popup, bg="#00e5ff", height=2).pack(side="top", fill="x")

        # Header (surface-container-high): icon + title, target-worktree subtitle, close X.
        header = tk.Frame(popup, bg="#262a31")
        header.pack(side="top", fill="x")
        self._icon_only_button(
            header, "close", popup.destroy, "Close", "#262a31", rest="#adcbda", hot="#ffb4ab", size=20
        ).pack(side="right", padx=(0, 18), pady=14)
        header_left = tk.Frame(header, bg="#262a31")
        header_left.pack(side="left", padx=24, pady=(18, 16))
        title_row = tk.Frame(header_left, bg="#262a31")
        title_row.pack(anchor="w")
        assign_icon = self._icon_photo("assign", "#c3f5ff", 18)
        if assign_icon is not None:
            tk.Label(title_row, image=assign_icon, bg="#262a31").pack(side="left", padx=(0, 8))
        tk.Label(
            title_row, text="ASSIGN_TASK", bg="#262a31", fg="#c3f5ff",
            font=("Space Grotesk", -17, "bold"),
        ).pack(side="left")
        tk.Label(
            header_left, text=f"Target Worktree: {worktree_id}", bg="#262a31", fg="#adcbda",
            font=("Consolas", -12),
        ).pack(anchor="w", pady=(2, 0))

        # Footer (pinned bottom FIRST so a long list can never push it off-screen — BUG-0007).
        selection = tk.StringVar(value=first_assignable_task_id(options))
        footer = tk.Frame(popup, bg="#262a31")
        footer.pack(side="bottom", fill="x")
        # Full-width footer band with a top divider separating it from the scrollable list.
        tk.Frame(popup, bg="#31353c", height=1).pack(side="bottom", fill="x")
        bind_command = lambda: self._confirm_assign(popup, selection.get(), repo_for_assign, worktree_id)
        bind_button = self._cta_button(
            footer, "ASSIGN", bind_command, bg="#262a31",
            top="#c3f5ff", bottom="#00e5ff", fg="#00363d",
            hover_top="#d8faff", hover_bottom="#2ee8ff", icon="check",
        )
        bind_button.pack(side="right", padx=(0, 24), pady=12)
        self._flat_button(
            footer, "CANCEL", popup.destroy,
            bg="#181c22", fg="#adcbda", hover_bg="#353940", hover_fg="#dfe2eb",
            font=("Space Grotesk", -14, "bold"), padx=18, pady=9,
        ).pack(side="right", padx=(0, 12), pady=12)

        if not options:
            tk.Label(
                popup, text="No open tasks were returned by the backend (GET /api/v1/tasks).",
                bg="#0a0e14", fg="#adcbda", font=("Inter", -13), wraplength=popup_w - 80,
                justify="left", anchor="nw",
            ).pack(side="top", fill="both", expand=True, padx=24, pady=24)
            return

        # Scrollable list (built now so the render closure can bind it; packed below the
        # toolbar). Pinned footer means a long list never hides the actions (BUG-0007).
        list_shell = tk.Frame(popup, bg="#0a0e14")
        canvas = tk.Canvas(list_shell, bg="#0a0e14", highlightthickness=0, bd=0)
        scrollbar = ttk.Scrollbar(list_shell, orient="vertical", command=canvas.yview)
        canvas.configure(yscrollcommand=scrollbar.set)
        scrollbar.pack(side="right", fill="y")
        canvas.pack(side="left", fill="both", expand=True)
        inner = tk.Frame(canvas, bg="#0a0e14")
        inner_id = canvas.create_window((0, 0), window=inner, anchor="nw")

        def _on_mousewheel(event):
            canvas.yview_scroll(-1 if event.delta > 0 else 1, "units")

        canvas.bind("<Configure>", lambda e: canvas.itemconfigure(inner_id, width=e.width))
        inner.bind("<Configure>", lambda _e: canvas.configure(scrollregion=canvas.bbox("all")))
        for wheel_target in (canvas, inner):
            wheel_target.bind("<MouseWheel>", _on_mousewheel)

        sort_state = {"ascending": True}
        wrap = popup_w - 130

        def render() -> None:
            top_fraction = canvas.yview()[0] if inner.winfo_children() else 0.0
            for child in inner.winfo_children():
                child.destroy()
            tk.Frame(inner, bg="#0a0e14", height=16).pack(fill="x")  # list p-6 top inset
            visible = sort_task_options(
                filter_task_options(options, current_query()), sort_state["ascending"]
            )
            if not visible:
                tk.Label(
                    inner, text="No tasks match the filter.", bg="#0a0e14", fg="#849396",
                    font=("Inter", -13), anchor="w",
                ).pack(anchor="w", padx=24, pady=12)
            for option in visible:
                self._build_assign_task_card(inner, option, selection, render, wrap, _on_mousewheel)
            tk.Frame(inner, bg="#0a0e14", height=8).pack(fill="x")  # list bottom inset
            inner.update_idletasks()
            canvas.configure(scrollregion=canvas.bbox("all"))
            canvas.yview_moveto(top_fraction)

        def toggle_sort() -> None:
            sort_state["ascending"] = not sort_state["ascending"]
            render()

        # Toolbar (filter input + SORT button), above the list.
        toolbar = tk.Frame(popup, bg="#1c2026")
        toolbar.pack(side="top", fill="x")
        tk.Frame(popup, bg="#31353c", height=1).pack(side="top", fill="x")  # toolbar/list divider
        toolbar_inner = tk.Frame(toolbar, bg="#1c2026")
        toolbar_inner.pack(fill="x", padx=24, pady=12)
        filter_wrap = tk.Frame(toolbar_inner, bg="#181c22")
        filter_wrap.pack(side="left", fill="x", expand=True)
        filter_icon = self._icon_photo("filter", "#adcbda", 16)
        if filter_icon is not None:
            tk.Label(filter_wrap, image=filter_icon, bg="#181c22").pack(side="left", padx=(10, 6))
        filter_entry = tk.Entry(
            filter_wrap, bg="#181c22", fg="#5d6b6e", insertbackground="#00e5ff",
            relief="flat", bd=0, font=("Consolas", -14),
        )
        filter_entry.pack(side="left", fill="x", expand=True, ipady=7, padx=(0, 10))

        placeholder = "FILTER_TASKS_BY_ID_OR_DESC..."
        ph_state = {"on": True}

        def set_placeholder() -> None:
            filter_entry.delete(0, "end")
            filter_entry.insert(0, placeholder)
            filter_entry.configure(fg="#5d6b6e")
            ph_state["on"] = True

        def current_query() -> str:
            return "" if ph_state["on"] else filter_entry.get()

        def on_filter_focus_in(_e) -> None:
            if ph_state["on"]:
                filter_entry.delete(0, "end")
                filter_entry.configure(fg="#dfe2eb")
                ph_state["on"] = False

        def on_filter_focus_out(_e) -> None:
            if not filter_entry.get():
                set_placeholder()

        filter_entry.bind("<FocusIn>", on_filter_focus_in)
        filter_entry.bind("<FocusOut>", on_filter_focus_out)
        filter_entry.bind("<KeyRelease>", lambda _e: render())
        set_placeholder()

        self._flat_button(
            toolbar_inner, "SORT: ID", toggle_sort,
            bg="#353940", fg="#dfe2eb", hover_bg="#3f444c",
            font=("Space Grotesk", -12, "bold"), icon="sort", icon_color="#dfe2eb",
            icon_side="left", padx=12, pady=8,
        ).pack(side="left", padx=(12, 0))

        # The list fills the remaining space between the toolbar and the pinned footer.
        list_shell.pack(side="top", fill="both", expand=True)
        render()

    def _flat_button(
        self, parent, text, command, *, bg, fg, hover_bg, hover_fg=None, font,
        icon=None, icon_color=None, icon_side="right", padx=14, pady=8,
        border=None, disabled=False,
    ):
        # A flat 0px-radius button (mockup CTAs): solid fill + hover, optional crisp glyph
        # and a thin border (ghost buttons). Built from tk widgets (not ttk) so the fill,
        # padding, font, and icon match the mockup exactly. `disabled` mutes it + drops input.
        hover_fg = hover_fg or fg
        eff_fg = "#566066" if disabled else fg
        eff_icon_color = "#566066" if disabled else (icon_color or fg)
        frame_kwargs = {"bg": bg, "cursor": "" if disabled else "hand2"}
        if border is not None:
            frame_kwargs.update(highlightthickness=1, highlightbackground=border, highlightcolor=border)
        frame = tk.Frame(parent, **frame_kwargs)
        inner = tk.Frame(frame, bg=bg)
        inner.pack(padx=padx, pady=pady)
        text_label = tk.Label(inner, text=text, bg=bg, fg=eff_fg, font=font)
        widgets = [frame, inner, text_label]
        icon_label = None
        if icon is not None:
            image = self._icon_photo(icon, eff_icon_color, 16)
            if image is not None:
                icon_label = tk.Label(inner, image=image, bg=bg)
                widgets.append(icon_label)
        if icon_label is not None and icon_side == "left":
            icon_label.pack(side="left", padx=(0, 8))
            text_label.pack(side="left")
        else:
            text_label.pack(side="left")
            if icon_label is not None:
                icon_label.pack(side="left", padx=(8, 0))
        if disabled:
            return frame

        def enter(_e):
            for widget in widgets:
                widget.configure(bg=hover_bg)
            text_label.configure(fg=hover_fg)

        def leave(_e):
            for widget in widgets:
                widget.configure(bg=bg)
            text_label.configure(fg=fg)

        for widget in widgets:
            widget.bind("<Enter>", enter)
            widget.bind("<Leave>", leave)
            widget.bind("<Button-1>", lambda _e: command())
        return frame

    def _icon_only_button(self, parent, kind, command, tooltip, bg, *, rest, hot, size):
        # An icon-only button (e.g. the modal close X): crisp glyph, hover recolor, tooltip.
        rest_img = self._icon_photo(kind, rest, size)
        hot_img = self._icon_photo(kind, hot, size)
        if rest_img is not None:
            button = tk.Label(parent, image=rest_img, bg=bg, bd=0, cursor="hand2")
            button.bind("<Enter>", lambda _e: button.configure(image=hot_img))
            button.bind("<Leave>", lambda _e: button.configure(image=rest_img))
        else:
            button = tk.Label(parent, text="X", bg=bg, fg=rest, cursor="hand2",
                              font=("Space Grotesk", -16, "bold"))
        button.bind("<Button-1>", lambda _e: command())
        if tooltip:
            self._bind_tooltip(button, tooltip)
        return button

    def _cta_button(
        self, parent, text, command, *, bg, top, bottom, fg, hover_top, hover_bottom,
        icon="check", icon_side="right", disabled=False, font_px=15, side_pad=24,
    ):
        # The gradient "glow" button style (the ASSIGN/CTA look) baked as one image, so every
        # worktree action button shares one size/font/treatment; only the colors vary by role
        # (cyan create/assign, error-red eject, neutral refresh/dequeue). Falls back to a flat
        # solid button (the gradient's base color) if Pillow is unavailable. Retains image
        # refs on the widget so Tk does not garbage-collect them.
        try:
            from PIL import ImageTk

            from .glyphs import render_cta

            font_path = FONT_ASSET_DIR / "SpaceGrotesk[wght].ttf"
            if disabled:
                muted = ImageTk.PhotoImage(
                    render_cta(text, font_path, top_color="#2e333a", bottom_color="#23272d",
                               fg="#566066", icon=icon, icon_side=icon_side, font_px=font_px, side_pad=side_pad)
                )
                button = tk.Label(parent, image=muted, bg=bg, bd=0)
                button._cta_rest = muted
                return button
            rest = ImageTk.PhotoImage(
                render_cta(text, font_path, top_color=top, bottom_color=bottom, fg=fg,
                           icon=icon, icon_side=icon_side, font_px=font_px, side_pad=side_pad)
            )
            hot = ImageTk.PhotoImage(
                render_cta(text, font_path, top_color=hover_top, bottom_color=hover_bottom, fg=fg,
                           icon=icon, icon_side=icon_side, font_px=font_px, side_pad=side_pad)
            )
        except Exception:
            font = ("Space Grotesk", -max(12, font_px - 1), "bold")
            if disabled:
                return self._flat_button(parent, text, command, bg=bottom, fg=fg, hover_bg=bottom,
                                         font=font, icon=icon, icon_color=fg, icon_side=icon_side,
                                         padx=18, pady=9, disabled=True)
            return self._flat_button(parent, text, command, bg=bottom, fg=fg, hover_bg=hover_bottom,
                                     font=font, icon=icon, icon_color=fg, icon_side=icon_side,
                                     padx=18, pady=9)
        button = tk.Label(parent, image=rest, bg=bg, bd=0, cursor="hand2")
        button._cta_rest = rest
        button._cta_hot = hot
        button.bind("<Enter>", lambda _e: button.configure(image=hot))
        button.bind("<Leave>", lambda _e: button.configure(image=rest))
        button.bind("<Button-1>", lambda _e: command())
        return button

    def _build_assign_task_card(self, parent, option, selection, rerender, wrap, on_wheel):
        # One task row in the Assign popup (mockup): a left status border, a radio, the task
        # id + a color-coded status chip on one line, and the description below. Selected =
        # cyan border + filled cyan radio + cyan id; blocked = orange border, muted, disabled.
        task_id = str(option.get("task_id") or "")
        title = str(option.get("title") or task_id)
        state = str(option.get("state") or "")
        category = task_state_category(state)
        assignable = category != "blocked"
        selected = selection.get() == task_id

        chip_palette = {
            "waiting": ("#0e2e34", "#7fdfe8"),
            "ready": ("#262a31", "#c3f5ff"),
            "blocked": ("#2a2125", "#ffc1bd"),
            "other": ("#262a31", "#adcbda"),
        }
        card_bg = "#1c2026"
        # Blocked rows recede (mockup opacity-80): dimmer id + heavily-muted description,
        # with a clearly-orange left border distinct from the normal gray.
        border_color = "#00e5ff" if selected else ("#5a3a3e" if not assignable else "#31353c")
        id_fg = "#c3f5ff" if selected else ("#6b757a" if not assignable else "#bac9cc")
        desc_fg = "#dfe2eb" if selected else ("#4e565b" if not assignable else "#adcbda")

        card = tk.Frame(parent, bg=card_bg)
        card.pack(fill="x", padx=24, pady=(0, 8))
        tk.Frame(card, bg=border_color, width=2).pack(side="left", fill="y")
        body = tk.Frame(card, bg=card_bg)
        body.pack(side="left", fill="x", expand=True, padx=16, pady=12)

        top = tk.Frame(body, bg=card_bg)
        top.pack(fill="x")
        radio_kind = "radio_on" if selected else "radio_off"
        radio_color = "#00e5ff" if selected else ("#566066" if not assignable else "#849396")
        radio_img = self._icon_photo(radio_kind, radio_color, 16)
        radio_label = None
        if radio_img is not None:
            radio_label = tk.Label(top, image=radio_img, bg=card_bg)
            radio_label.pack(side="left", padx=(0, 12))
        chip_bg, chip_fg = chip_palette[category]
        tk.Label(
            top, text=task_state_chip_label(state), bg=chip_bg, fg=chip_fg,
            font=("Inter", -10), padx=8, pady=2,
        ).pack(side="right")
        id_label = tk.Label(top, text=task_id, bg=card_bg, fg=id_fg, font=("Consolas", -14, "bold"))
        id_label.pack(side="left")
        desc_label = tk.Label(
            body, text=title, bg=card_bg, fg=desc_fg, font=("Inter", -13),
            anchor="w", justify="left", wraplength=wrap,
        )
        desc_label.pack(anchor="w", fill="x", pady=(4, 0))

        hoverable = [card, body, top, id_label, desc_label]
        if radio_label is not None:
            hoverable.append(radio_label)
        for widget in hoverable:
            widget.bind("<MouseWheel>", on_wheel)

        if assignable:
            def select(_e):
                selection.set(task_id)
                rerender()

            for widget in hoverable:
                widget.configure(cursor="hand2")
                widget.bind("<Button-1>", select)

            if not selected:
                def enter(_e):
                    for widget in (card, body, top, id_label, desc_label):
                        widget.configure(bg="#353940")
                    if radio_label is not None:
                        radio_label.configure(bg="#353940")
                    id_label.configure(fg="#c3f5ff")
                    desc_label.configure(fg="#dfe2eb")

                def leave(_e):
                    for widget in (card, body, top, id_label, desc_label):
                        widget.configure(bg=card_bg)
                    if radio_label is not None:
                        radio_label.configure(bg=card_bg)
                    id_label.configure(fg=id_fg)
                    desc_label.configure(fg=desc_fg)

                for widget in hoverable:
                    widget.bind("<Enter>", enter)
                    widget.bind("<Leave>", leave)

    def _confirm_assign(self, popup: tk.Toplevel, task_id: str, repo: str, worktree_id: str) -> None:
        popup.destroy()
        if not task_id:
            self._set_worktrees_status("Select a task to assign.")
            return
        try:
            assign_worktree(task_id, repo, worktree_id, self.worktrees_backend_url)
            self._reload_worktrees_snapshot(refreshed=True, keep_status=True)
            self._set_worktrees_status(f"Assigned {task_id} to {worktree_id}; it is now allocated.")
        except Exception as exc:
            self._reload_worktrees_snapshot(refreshed=True, keep_status=True)
            self._set_worktrees_status(f"Assign failed: {exc}")
        self._render_worktrees_snapshot()

    def _selected_repo_id(self) -> str:
        selected = self.worktrees_repo_filter
        if selected and selected != ALL_REPOS_OPTION:
            return selected
        if len(self.worktrees_repos) == 1:
            return str(self.worktrees_repos[0].get("id") or "")
        return ""

    def _set_worktrees_status(self, message: str) -> None:
        self.worktrees_status_message = message
        self.status_label.configure(text=message)
        self.worktrees_action_status_label.configure(text=message)

    def _refresh_worktrees_scroll_region(self, _event=None) -> None:
        self.worktrees_scroll_canvas.configure(scrollregion=self.worktrees_scroll_canvas.bbox("all"))

    def _resize_worktrees_scroll_content(self, event) -> None:
        self.worktrees_scroll_canvas.itemconfigure(self.worktrees_scroll_window, width=event.width)

    def _on_worktrees_mousewheel(self, event) -> str:
        if self.active_tab != "worktrees":
            return "break"
        delta = event.delta
        if delta == 0:
            return "break"
        self.worktrees_scroll_canvas.yview_scroll(int(-delta / 120), "units")
        return "break"

    def _event_from_widget(self, widget, ancestor) -> bool:
        current = widget
        while current is not None:
            if current == ancestor:
                return True
            current = getattr(current, "master", None)
        return False

    def refresh_jobs_data(self, apply_changes: bool = False) -> None:
        try:
            if apply_changes:
                self.jobs_snapshot, report = sync_jobs_snapshot(self.jobs_backend_url)
                changes = summarize_apply_report(report)
                self.jobs_status_message = (
                    f"UPDATE applied Git desired state to Temporal: {changes['total']} schedule changes "
                    f"(+{changes['created']} ~{changes['updated']} -{changes['deleted']})."
                )
            else:
                self.jobs_snapshot = fetch_jobs_snapshot(self.jobs_backend_url)
                self.jobs_status_message = "Jobs state refreshed from orchestration backend."
            if self.active_tab == "jobs":
                self.status_label.configure(text=self.jobs_status_message)
        except Exception as exc:
            self.jobs_snapshot = jobs_backend_error_snapshot(str(exc))
            self.jobs_status_message = f"Jobs error: {exc}"
            self.status_label.configure(text=self.jobs_status_message)
        self._render_jobs_snapshot()

    def _prime_jobs_snapshot(self) -> None:
        existing_jobs = list(self.jobs_snapshot.get("jobs", []))
        if existing_jobs:
            return
        try:
            self.jobs_snapshot = fetch_jobs_snapshot(self.jobs_backend_url)
            self.jobs_status_message = "Jobs state loaded from orchestration backend."
        except Exception as exc:
            self.jobs_snapshot = jobs_backend_error_snapshot(str(exc))
            self.jobs_status_message = f"Jobs error: {exc}"
        self._render_jobs_snapshot()

    def _render_jobs_snapshot(self) -> None:
        snapshot = self.jobs_snapshot
        summary = dict(snapshot.get("summary", {}))
        jobs = list(snapshot.get("jobs", []))
        self.jobs_metric_values["total"].configure(text=f"{len(jobs):02d}")
        self.jobs_metric_values["in_sync"].configure(text=f"{int(summary.get('in_sync', 0)):02d}")
        self.jobs_metric_values["attention"].configure(text=f"{jobs_needs_attention_count(summary):02d}")

        for child in self.jobs_rows_container.winfo_children():
            child.destroy()

        if not jobs:
            tk.Label(
                self.jobs_rows_container,
                text=self.jobs_status_message,
                bg="#0a0e14", fg="#adcbda", font=("Inter", -12),
                wraplength=520, justify="left", anchor="w",
            ).pack(anchor="w", padx=14, pady=14)
        else:
            for job in jobs:
                self._build_jobs_row(job)

    def _build_jobs_row(self, job: dict[str, object]) -> None:
        # One table row aligned under the fixed-width header columns (JOB_ID / NEXT /
        # STATUS / LAST_RUN) with right-justified actions (Details icon + Run now).
        status = str(job.get("status", "unknown"))
        attention = status != "in_sync"
        row_bg = "#221a1c" if attention else "#1c2026"
        widths = dict(self._jobs_columns)

        row = tk.Frame(self.jobs_rows_container, bg=row_bg)
        row.pack(fill="x", pady=(0, 1))
        inner = tk.Frame(row, bg=row_bg, height=48)
        inner.pack(fill="x", padx=14, pady=1)
        inner.pack_propagate(False)

        actions = tk.Frame(inner, bg=row_bg)
        actions.pack(side="right")
        if job_is_running(job):
            self._cta_button(
                actions, "RUNNING", lambda: None, bg=row_bg,
                top="#2e333a", bottom="#23272d", fg="#566066",
                hover_top="#2e333a", hover_bottom="#23272d", icon=None, disabled=True,
                font_px=12, side_pad=14,
            ).pack(side="right")
        elif bool(job.get("supports_run_now")):
            self._cta_button(
                actions, "RUN NOW", lambda payload=dict(job): self.run_job_now(payload), bg=row_bg,
                top="#c3f5ff", bottom="#00e5ff", fg="#00363d",
                hover_top="#d8faff", hover_bottom="#2ee8ff", icon=None, font_px=12, side_pad=14,
            ).pack(side="right")
        self._icon_only_button(
            actions, "details", lambda payload=dict(job): self.open_job_details_popup(payload),
            "Details", row_bg, rest="#849396", hot="#00e5ff", size=16,
        ).pack(side="right", padx=(0, 12))

        def cell(width: int) -> tk.Frame:
            frame = tk.Frame(inner, bg=row_bg, width=width)
            frame.pack(side="left", fill="y")
            frame.pack_propagate(False)
            content = tk.Frame(frame, bg=row_bg)
            content.pack(expand=True, anchor="w")  # vertically centered, left-aligned
            return content

        id_content = cell(widths["JOB_ID"])
        icon_color = "#ffb4ab" if status in ("blocked", "missing") else ("#ffc1bd" if attention else "#849396")
        icon_kind = "sync" if job_is_running(job) else ("warning" if attention else "clock")
        id_icon = self._icon_photo(icon_kind, icon_color, 14)
        if id_icon is not None:
            tk.Label(id_content, image=id_icon, bg=row_bg).pack(side="left", padx=(0, 8))
        tk.Label(id_content, text=str(job.get("job_id", "")), bg=row_bg, fg="#dfe2eb",
                 font=("Consolas", -13, "bold")).pack(side="left")

        next_content = cell(widths["NEXT"])
        tk.Label(next_content, text=job_next_run_display(job), bg=row_bg, fg="#dfe2eb",
                 font=("Space Grotesk", -13, "bold"), anchor="w").pack(anchor="w")

        status_content = cell(widths["STATUS"])
        chip_label, chip_bg, chip_fg = job_status_chip(status)
        tk.Label(status_content, text=chip_label, bg=chip_bg, fg=chip_fg,
                 font=("Inter", -10), padx=8, pady=2).pack(anchor="w")

        last_primary, last_detail = job_last_run_display(job)
        last_content = cell(widths["LAST_RUN"])
        tk.Label(last_content, text=last_primary, bg=row_bg, fg="#dfe2eb",
                 font=("Space Grotesk", -13, "bold"), anchor="w").pack(anchor="w")
        if last_detail:
            detail_color = "#ffb4ab" if "fail" in last_detail.lower() else "#849396"
            tk.Label(last_content, text=last_detail, bg=row_bg, fg=detail_color,
                     font=("Inter", -11), anchor="w").pack(anchor="w")

        for widget in (row, inner, actions):
            widget.bind("<MouseWheel>", self._on_jobs_mousewheel)

    def run_job_now(self, job: dict[str, object]) -> None:
        try:
            started = start_job_run(str(job.get("job_id", "")), self.jobs_backend_url)
            workflow_id = str(started.get("workflow_id", "")).strip()
            if workflow_id:
                self.jobs_status_message = f"Run now started for {job.get('label', 'job')}: {workflow_id}"
            else:
                self.jobs_status_message = f"Run now started for {job.get('label', 'job')}."
            self.jobs_snapshot = fetch_jobs_snapshot(self.jobs_backend_url)
        except Exception as exc:
            self.jobs_status_message = f"Run now failed: {exc}"
            self.jobs_snapshot = jobs_backend_error_snapshot(str(exc))
        if self.active_tab == "jobs":
            self.status_label.configure(text=self.jobs_status_message)
        self._render_jobs_snapshot()

    def open_job_details_popup(self, job: dict[str, object]) -> None:
        # Per-job diagnostic detail behind a reveal (the row info button), in the shared
        # borderless-modal chrome: key facts + the raw backend definition. Non-modal (no
        # grab) + Escape/close, matching the Assign popup.
        popup = tk.Toplevel(self.overlay)
        popup.configure(bg="#1c2026")
        popup.transient(self.overlay)
        popup.attributes("-topmost", True)
        popup.overrideredirect(True)
        self.overlay.update_idletasks()
        ow, oh = self.overlay.winfo_width(), self.overlay.winfo_height()
        pw, ph = min(560, max(420, ow - 120)), min(560, max(360, oh - 120))
        ox, oy = self.overlay.winfo_rootx(), self.overlay.winfo_rooty()
        popup.geometry(f"{pw}x{ph}+{ox + max(0, (ow - pw) // 2)}+{oy + max(0, (oh - ph) // 2)}")
        popup.update_idletasks()
        popup.lift()
        popup.focus_force()
        popup.bind("<Escape>", lambda _e: popup.destroy())

        tk.Frame(popup, bg="#00e5ff", height=2).pack(side="top", fill="x")
        header = tk.Frame(popup, bg="#262a31")
        header.pack(side="top", fill="x")
        self._icon_only_button(
            header, "close", popup.destroy, "Close", "#262a31", rest="#adcbda", hot="#ffb4ab", size=20
        ).pack(side="right", padx=(0, 18), pady=14)
        header_left = tk.Frame(header, bg="#262a31")
        header_left.pack(side="left", padx=24, pady=(16, 14))
        title_row = tk.Frame(header_left, bg="#262a31")
        title_row.pack(anchor="w")
        detail_icon = self._icon_photo("details", "#c3f5ff", 18)
        if detail_icon is not None:
            tk.Label(title_row, image=detail_icon, bg="#262a31").pack(side="left", padx=(0, 8))
        tk.Label(title_row, text="JOB_DETAIL", bg="#262a31", fg="#c3f5ff",
                 font=("Space Grotesk", -17, "bold")).pack(side="left")
        tk.Label(header_left, text=str(job.get("job_id", "")), bg="#262a31", fg="#adcbda",
                 font=("Consolas", -12)).pack(anchor="w", pady=(2, 0))

        body = tk.Frame(popup, bg="#0a0e14")
        body.pack(side="top", fill="both", expand=True)
        detail_scroll = ttk.Scrollbar(body, orient="vertical")
        detail_scroll.pack(side="right", fill="y")
        text = tk.Text(body, bg="#0a0e14", fg="#bac9cc", relief="flat", bd=0, wrap="word",
                       font=("Consolas", -12), padx=16, pady=14, highlightthickness=0,
                       yscrollcommand=detail_scroll.set)
        detail_scroll.configure(command=text.yview)
        text.pack(side="left", fill="both", expand=True)
        text.insert("end", job_detail_text(job))
        text.configure(state="disabled")

    def _refresh_jobs_scroll_region(self, _event=None) -> None:
        self.jobs_scroll_canvas.configure(scrollregion=self.jobs_scroll_canvas.bbox("all"))

    def _resize_jobs_scroll_content(self, event) -> None:
        self.jobs_scroll_canvas.itemconfigure(self.jobs_scroll_window, width=event.width)

    def _on_jobs_mousewheel(self, event) -> str:
        if self.active_tab != "jobs":
            return "break"
        delta = event.delta
        if delta == 0:
            return "break"
        self.jobs_scroll_canvas.yview_scroll(int(-delta / 120), "units")
        return "break"

    def _poll_hotkey(self) -> None:
        if self.hotkey_registered:
            self.hotkey.poll()
        self.root.after(50, self._poll_hotkey)

    def _poll_ingest_results(self) -> None:
        try:
            while True:
                event_type, payload = self.ingest_queue.get_nowait()
                if event_type == "summary":
                    self.ingest_in_flight = False
                    self.last_ingest_error = None
                    files_scanned, files_updated, events_ingested = payload
                    self.status_label.configure(
                        text=(
                            f"Last ingest {datetime.now().strftime('%H:%M:%S')} | "
                            f"files {files_updated}/{files_scanned} | "
                            f"events +{events_ingested}"
                        )
                    )
                    self.refresh_data()
                elif event_type == "error":
                    self.ingest_in_flight = False
                    self.last_ingest_error = str(payload)
                    self.status_label.configure(text=f"Ingest error: {payload}")
                    self._refresh_status_surfaces(False)
                elif event_type == "dashboard_data":
                    # Task-0013 activation-fix follow-up: the off-thread startup
                    # pre-render (or cold-start safety net) finished; render the
                    # persistent (withdrawn) overlay now so the first hotkey
                    # toggle is fast and never has to rebuild on show.
                    self._activation_load_in_flight = False
                    self._render_dashboard(*payload)
                elif event_type == "dashboard_data_error":
                    self._activation_load_in_flight = False
                    self.last_ingest_error = str(payload)
                    self.status_label.configure(text=f"Load error: {payload}")
                    self._refresh_status_surfaces(False)
        except queue.Empty:
            pass
        self.root.after(100, self._poll_ingest_results)

    def _start_activation_load(self) -> None:
        """Pre-render the persistent overlay by loading its data off the UI thread.

        Task-0013 activation-fix follow-up: this is the STARTUP pre-render (and a
        cold-start safety net). It runs the DB read on a worker thread and posts
        the snapshot to the ingest queue so the (withdrawn) overlay is rendered
        before the user toggles it. The hotkey show/hide path itself never calls
        this; it only reveals the already-rendered window.
        """
        if self._activation_load_in_flight:
            return
        self._activation_load_in_flight = True

        def worker() -> None:
            try:
                snapshot = self._load_dashboard_data()
                self.ingest_queue.put(("dashboard_data", snapshot))
            except Exception as exc:  # pragma: no cover - GUI error surfacing
                self.ingest_queue.put(("dashboard_data_error", str(exc)))

        threading.Thread(target=worker, daemon=True).start()

    def schedule_ingest(self) -> None:
        if self._quitting:
            return
        self.root.after(self.config.polling_seconds * 1000, self.schedule_ingest)
        if self.ingest_in_flight:
            return
        self.ingest_in_flight = True

        def worker() -> None:
            try:
                connection = connect(Path(self.config.db_path))
                initialize_db(connection)
                summary = ingest_once(connection, self.config)
                connection.close()
                self.ingest_queue.put(
                    (
                        "summary",
                        (
                            summary.files_scanned,
                            summary.files_updated,
                            summary.events_ingested,
                        ),
                    )
                )
            except Exception as exc:  # pragma: no cover - GUI error surfacing
                self.ingest_queue.put(("error", str(exc)))

        thread = threading.Thread(target=worker, daemon=True)
        thread.start()

    def _load_dashboard_data(self):
        """Read the dashboard window data from SQLite.

        Task-0013 activation-fix follow-up (Fix B): this is the only blocking
        database work in the refresh path. It runs on the background poll's worker
        thread, performs no Tk calls, and is deliberately cheap:

        - It loads ONLY the charted window's events (interval x bucket_count),
          not the full 7-day window, so bucketing loops a few thousand events
          instead of the ~467k a 7-day window holds on a large DB.
        - It computes the rolling 7-day total with an indexed SQL SUM grouped by
          source (no per-event materialization), so the displayed 7-day total
          stays correct and the source filter can include/exclude a source from
          it purely in memory.
        - It fetches the latest weekly advisory with a cheap indexed lookback so
          it is not missed when the chart window is shorter than 7 days.

        Returns (events, session_context_markers, source_totals_7d,
        latest_weekly_advisory).
        """
        connection = connect(Path(self.config.db_path))
        initialize_db(connection)
        now = datetime.now(self.display_timezone)
        bucket_count = chart_bucket_count(self.selected_interval)
        # Only the charted span needs per-event rows. The 7-day total no longer
        # requires loading the 7-day window here: it is computed by the indexed
        # aggregate query below, so this load is bounded to the chart window.
        chart_window = timedelta(
            seconds=INTERVAL_SECONDS[self.selected_interval] * bucket_count
        )
        chart_since = now.astimezone(UTC) - chart_window
        summary_since = now.astimezone(UTC) - USAGE_SUMMARY_WINDOW
        events = load_events_since(connection, chart_since)
        source_totals_7d = sum_total_tokens_by_source_since(connection, summary_since)
        latest_weekly_advisory = load_latest_weekly_advisory(connection, summary_since)
        session_context_markers = (
            load_session_context_markers(
                connection,
                sorted({event.session_path for event in events}),
            )
            if self.selected_chart_mode == "repo"
            else {}
        )
        connection.close()
        return events, session_context_markers, source_totals_7d, latest_weekly_advisory

    def refresh_data(self) -> None:
        snapshot = self._load_dashboard_data()
        self._render_dashboard(*snapshot)

    def _render_dashboard(
        self,
        events,
        session_context_markers,
        source_totals_7d=None,
        latest_weekly_advisory=None,
    ) -> None:
        """Render the dashboard window from an in-memory snapshot.

        Pure Tk rendering, no blocking database read, so it runs on the UI thread.
        This is invoked by the background poll (with a freshly loaded snapshot) and
        by a source-filter toggle (Objective 4, reusing the stored snapshot). It is
        NOT on the hotkey show/hide path: the persistent overlay keeps the most
        recent render, and the hotkey only reveals/hides it.

        Task-0013 activation-fix follow-up (Fix B): `source_totals_7d` is the
        per-source rolling 7-day total computed cheaply by `_load_dashboard_data`;
        the displayed 7-day total is the sum over the SELECTED sources, so the
        source filter adjusts it in memory with no DB read. When omitted (e.g. an
        older test call), the stored per-source totals are reused.
        """
        now = datetime.now(self.display_timezone)
        bucket_count = chart_bucket_count(self.selected_interval)
        # Keep the full snapshot so a source-filter toggle can re-render without a
        # database read (Task-0013 Objective 4 preserves Objective 3).
        self.latest_events = events
        self.latest_session_context_markers = session_context_markers
        if source_totals_7d is not None:
            self.latest_source_totals_7d = source_totals_7d
        if latest_weekly_advisory is not None:
            self.latest_weekly_advisory = latest_weekly_advisory
        self.latest_repo_legend = []
        self.latest_repo_totals = []

        # Task-0013 Objective 4: restrict the displayed aggregation to the
        # selected sources. This filters the already-loaded snapshot in memory;
        # it does not read SQLite, so it is safe on the UI thread.
        events = filter_events_by_source(events, self.selected_sources)

        raw_buckets = build_buckets(
            events,
            self.selected_interval,
            bucket_count=bucket_count,
            now=now,
            display_tz=self.display_timezone,
            metric_mode="total",
        )
        display_buckets = raw_buckets
        if self.selected_metric_mode != "total":
            display_buckets = build_buckets(
                events,
                self.selected_interval,
                bucket_count=bucket_count,
                now=now,
                display_tz=self.display_timezone,
                metric_mode=self.selected_metric_mode,
            )
        interval_seconds = INTERVAL_SECONDS[self.selected_interval]
        # Task-0013 activation-fix follow-up (Fix B): the 7-day total is the sum
        # over the selected sources of the cheap per-source rolling totals; the
        # source filter excludes a source by dropping its precomputed total. No
        # per-event 7-day scan happens here. None selection means all sources.
        selected = (
            self.selected_sources
            if self.selected_sources is not None
            else set(self.latest_source_totals_7d)
        )
        total_7d = sum(
            tokens
            for source, tokens in self.latest_source_totals_7d.items()
            if (source or "codex") in selected
        )
        # The weekly advisory is a Codex rate-limit artifact, so it only applies
        # while Codex is in the selected sources (matches the prior filter, which
        # recomputed the advisory from the source-filtered events).
        latest_advisory = (
            self.latest_weekly_advisory if "codex" in selected else None
        )
        if maybe_upgrade_weekly_budget(
            self.config,
            total_7d,
            latest_advisory.weekly_used_percent if latest_advisory else None,
        ):
            save_config(self.config, self.config_path)
            self.weekly_budget_var.set(format_budget_billions(self.config.weekly_budget_tokens))

        pace_sample_size = min(ROLLING_PROJECTION_BUCKETS, len(raw_buckets))
        pace_tokens = rolling_average_tokens(raw_buckets, pace_sample_size)
        projected = project_weekly_burn(pace_tokens, interval_seconds)
        redline = projected > self.config.weekly_budget_tokens
        budget_line_tokens = interval_redline_tokens(
            self.config.weekly_budget_tokens,
            interval_seconds,
        )
        headroom_tokens = budget_line_tokens - pace_tokens

        self.local_total_value.configure(text=format_token_value(total_7d))
        self.local_total_detail.configure(text="in the last 7d")
        self.projected_value.configure(text=format_token_value(projected))
        self.projected_detail.configure(text=f"based on the last {pace_sample_size} bars")
        self.headroom_value.configure(text=format_signed_token_value(headroom_tokens))
        self.headroom_detail.configure(text="until exceeding budget")
        if latest_advisory is None:
            self.advisory_label.configure(text="No weekly advisory yet.")
        else:
            advisory = latest_advisory.weekly_used_percent
            reset_text = (
                f"reset in {format_reset_remaining(latest_advisory.weekly_resets_at)}"
                if latest_advisory.weekly_resets_at is not None
                else "reset time unavailable"
            )
            self.advisory_label.configure(
                text=(
                    f"Codex advisory window: {advisory:.1f}% used | "
                    f"{reset_text}"
                )
            )
        self.chart_header_title.configure(
            text=format_chart_title(
                self.selected_interval,
                self.selected_chart_mode,
                self.selected_metric_mode,
            )
        )
        self._refresh_status_surfaces(redline)
        chart_context_bits: list[str] = []
        if self.selected_chart_mode == "repo":
            repo_buckets, repo_legend, repo_totals = build_project_stacks(
                events,
                session_context_markers,
                self.selected_interval,
                bucket_count=bucket_count,
                now=now,
                display_tz=self.display_timezone,
                top_n=5,
                metric_mode=self.selected_metric_mode,
            )
            self.latest_repo_legend = repo_legend
            self.latest_repo_totals = repo_totals
            chart_context_bits.append("Top 5 repos")
            if self.selected_metric_mode == "norm":
                chart_context_bits.append("Norm")
            chart_context_bits.append(self._timezone_label())
            self.chart_header_context.configure(
                text=" | ".join(chart_context_bits)
            )
            self.draw_chart(
                repo_buckets,
                repo_legend=repo_legend,
                repo_totals=repo_totals,
                raw_buckets=raw_buckets,
                show_budget_line=self.selected_metric_mode == "total",
            )
            return
        if self.selected_metric_mode == "norm":
            chart_context_bits.append("Norm")
        chart_context_bits.append(self._timezone_label())
        self.chart_header_context.configure(text=" | ".join(chart_context_bits))
        self.draw_chart(
            display_buckets,
            raw_buckets=raw_buckets,
            show_budget_line=self.selected_metric_mode == "total",
        )

    def _refresh_status_surfaces(self, redline: bool) -> None:
        if self.last_ingest_error is not None:
            accent = "#ff5a52"
            value = "Attention"
            detail = "Ingest error detected."
        elif redline:
            accent = "#ff5a52"
            value = "Redline"
            detail = "Projected weekly burn exceeds budget."
        else:
            accent = "#bff4ff"
            value = "Operational"
            detail = "Within weekly budget."

        self.status_accent.configure(bg=accent)
        self.status_dot.configure(bg=accent)
        self.status_metric_value.configure(text=value, foreground=accent)
        self.status_metric_detail.configure(text=detail)

    def draw_chart(
        self,
        buckets,
        repo_legend: list[tuple[str, str]] | None = None,
        repo_totals: list[dict[str, int]] | None = None,
        raw_buckets=None,
        show_budget_line: bool = True,
    ) -> None:
        self.canvas.delete("all")
        self.chart_hover_regions = []
        self.chart_context_region = None
        self._hide_chart_tooltip()
        width = max(int(self.canvas.winfo_width()), int(self.canvas["width"]))
        height = max(int(self.canvas.winfo_height()), int(self.canvas["height"]))
        left = 56
        right = width - 24
        top = 18
        if repo_legend:
            top += 24
        bottom = height - 28
        chart_height = bottom - top
        chart_width = right - left

        self.canvas.create_rectangle(
            left,
            top,
            right,
            bottom,
            outline="#39424d",
            fill="#10141a",
        )

        if not buckets:
            self.canvas.create_text(
                width / 2,
                height / 2,
                text="No token data yet.",
                fill="#dfe2eb",
                font=("Segoe UI Semibold", 14),
            )
            return

        if repo_legend:
            legend_x = left
            legend_y = top - 15
            for index, (_project_key, label) in enumerate(repo_legend):
                color = REPO_STACK_COLORS[index % len(REPO_STACK_COLORS)]
                self.canvas.create_rectangle(
                    legend_x,
                    legend_y - 5,
                    legend_x + 8,
                    legend_y + 3,
                    fill=color,
                    outline=REPO_STACK_OUTLINE,
                    width=1,
                )
                self.canvas.create_text(
                    legend_x + 14,
                    legend_y,
                    anchor="w",
                    text=label,
                    fill="#8fa8bb",
                    font=("Inter", 8),
                )
                legend_x += 14 + min(120, len(label) * 6 + 18)
                if legend_x > right - 120:
                    legend_x = left
                    legend_y += 14

        threshold_tokens = 0
        if show_budget_line:
            threshold_tokens = interval_redline_tokens(
                self.config.weekly_budget_tokens,
                INTERVAL_SECONDS[self.selected_interval],
            )
        max_tokens = max(
            max(bucket.total_tokens for bucket in buckets),
            threshold_tokens if show_budget_line else 0,
            1,
        )
        grid_steps = 4
        for row in range(grid_steps + 1):
            y = top + row * chart_height / grid_steps
            self.canvas.create_line(left, y, right, y, fill="#31353c")
            label_value = int(round(max_tokens * (grid_steps - row) / grid_steps))
            self.canvas.create_text(
                left - 8,
                y,
                anchor="e",
                text=format_token_value(label_value),
                fill="#6e8598",
                font=("Inter", 8),
            )

        if show_budget_line:
            threshold_y = bottom - ((threshold_tokens / max_tokens) * chart_height)
            threshold_color = "#8ec5ff"
            self.canvas.create_line(
                left,
                threshold_y,
                right,
                threshold_y,
                fill=threshold_color,
                width=2,
                dash=(2, 4),
            )
            label_left = left + 12
            label_right = label_left + 54
            self.canvas.create_rectangle(
                label_left,
                threshold_y - 9,
                label_right,
                threshold_y + 5,
                fill="#10141a",
                outline="",
            )
            self.canvas.create_text(
                (label_left + label_right) / 2,
                threshold_y - 2,
                text="BUDGET",
                fill=threshold_color,
                font=("Inter", 7, "bold"),
            )

        gap = 8
        bar_width = max(12, int((chart_width - gap * (len(buckets) - 1)) / len(buckets)))
        for index, bucket in enumerate(buckets):
            x0 = left + index * (bar_width + gap)
            x1 = x0 + bar_width
            hover_text = format_velocity_tooltip(bucket.total_tokens)
            raw_bucket = raw_buckets[index] if raw_buckets is not None and index < len(raw_buckets) else bucket
            if repo_legend and repo_totals is not None:
                segment_bottom = bottom
                bucket_segments = repo_totals[index] if index < len(repo_totals) else {}
                hover_text = format_repo_tooltip(bucket_segments, repo_legend)
                if bucket.total_tokens == 0:
                    self.canvas.create_rectangle(
                        x0,
                        bottom - 1,
                        x1,
                        bottom,
                        fill="#17314c",
                        outline=REPO_STACK_OUTLINE,
                        width=1,
                    )
                else:
                    for color_index, (project_key, _label) in enumerate(repo_legend):
                        segment_tokens = bucket_segments.get(project_key, 0)
                        if segment_tokens <= 0:
                            continue
                        segment_height = (segment_tokens / max_tokens) * (chart_height - 8)
                        y0 = segment_bottom - segment_height
                        self.canvas.create_rectangle(
                            x0,
                            y0,
                            x1,
                            segment_bottom,
                            fill=REPO_STACK_COLORS[color_index % len(REPO_STACK_COLORS)],
                            outline=REPO_STACK_OUTLINE,
                            width=1,
                        )
                        segment_bottom = y0
            else:
                bar_height = (bucket.total_tokens / max_tokens) * (chart_height - 8)
                y0 = bottom - bar_height
                if show_budget_line and bucket.total_tokens >= threshold_tokens and bucket.total_tokens > 0:
                    fill = "#ff7a6e"
                elif index == len(buckets) - 1:
                    fill = "#58a8ff"
                elif bucket.total_tokens == 0:
                    fill = "#17314c"
                elif index % 2 == 0:
                    fill = "#2f6fa3"
                else:
                    fill = "#265d8a"
                self.canvas.create_rectangle(x0, y0, x1, bottom, fill=fill, outline="")
            if index % 3 == 0 or index == len(buckets) - 1:
                label = format_tick_label(bucket.start_at, self.selected_interval)
                self.canvas.create_text(
                    (x0 + x1) / 2,
                    bottom + 14,
                    text=label,
                    fill="#6e8598",
                    font=("Inter", 8),
                )
                self.chart_hover_regions.append(
                {
                    "x0": x0,
                    "x1": x1,
                    "y0": top,
                    "y1": bottom,
                    "text": hover_text,
                    "bucket": raw_bucket,
                    "display_total": bucket.total_tokens,
                    "repo_totals": dict(repo_totals[index]) if repo_totals is not None and index < len(repo_totals) else {},
                }
            )

    def select_interval(self, interval_key: str) -> None:
        self.selected_interval = interval_key
        self._refresh_interval_buttons()
        self.refresh_data()

    def select_chart_mode(self, chart_mode: str) -> None:
        self.selected_chart_mode = chart_mode
        self._refresh_chart_mode_buttons()
        self.refresh_data()

    def select_metric_mode(self, metric_mode: str) -> None:
        self.selected_metric_mode = metric_mode
        self._refresh_metric_mode_buttons()
        self.refresh_data()

    def _build_source_filter_control(self) -> None:
        """Build the Task-0013 Objective 4 source filter dropdown.

        A Menubutton with one checkbutton per known source (Codex, Claude),
        styled to match the overlay toolbar. Toggling a checkbox includes or
        excludes that source from the displayed aggregation by re-rendering the
        already-loaded snapshot (no synchronous database read).
        """
        source_shell = ttk.Frame(
            self.usage_header_controls, style="Shell.TFrame", padding=(8, 6)
        )
        source_shell.pack(side="left", padx=(8, 0))
        self.source_filter_button = ttk.Menubutton(
            source_shell,
            style="ToolbarQuiet.TButton",
            direction="below",
        )
        self.source_filter_button.pack(side="left")
        menu = tk.Menu(
            self.source_filter_button,
            tearoff=0,
            bg="#121820",
            fg="#dfe2eb",
            activebackground="#16d9f5",
            activeforeground="#10141a",
            relief="flat",
        )
        self.source_filter_vars: dict[str, tk.BooleanVar] = {}
        for source in KNOWN_SOURCES:
            var = tk.BooleanVar(value=source in self.selected_sources)
            self.source_filter_vars[source] = var
            menu.add_checkbutton(
                label=SOURCE_LABELS.get(source, source.title()),
                variable=var,
                command=lambda src=source: self._toggle_source(src),
            )
        self.source_filter_button["menu"] = menu
        self._refresh_source_filter_label()

    def _toggle_source(self, source: str) -> None:
        """Include/exclude one source and re-render from the in-memory snapshot.

        Task-0013 Objective 4: this never reads SQLite. It mutates the selection
        and re-renders the already-loaded snapshot, so it cannot reintroduce a
        synchronous UI-thread database read (Objective 3 stays intact).
        """
        var = self.source_filter_vars.get(source)
        if var is not None and var.get():
            self.selected_sources.add(source)
        else:
            self.selected_sources.discard(source)
        self._refresh_source_filter_label()
        self._render_dashboard(
            self.latest_events,
            self.latest_session_context_markers,
        )

    def _refresh_source_filter_label(self) -> None:
        selected = [s for s in KNOWN_SOURCES if s in self.selected_sources]
        if len(selected) == len(KNOWN_SOURCES):
            text = "Source: All"
        elif not selected:
            text = "Source: None"
        else:
            text = "Source: " + ", ".join(
                SOURCE_LABELS.get(s, s.title()) for s in selected
            )
        if getattr(self, "source_filter_button", None) is not None:
            self.source_filter_button.configure(text=text)

    def _refresh_interval_buttons(self) -> None:
        for key, button in self.interval_buttons.items():
            button.configure(
                style="ToolbarAccent.TButton" if key == self.selected_interval else "ToolbarQuiet.TButton"
            )

    def _refresh_chart_mode_buttons(self) -> None:
        for key, button in self.chart_mode_buttons.items():
            button.configure(
                style="ToolbarAccent.TButton" if key == self.selected_chart_mode else "ToolbarQuiet.TButton"
            )

    def _refresh_metric_mode_buttons(self) -> None:
        for key, button in self.metric_mode_buttons.items():
            button.configure(
                style="ToolbarAccent.TButton" if key == self.selected_metric_mode else "ToolbarQuiet.TButton"
            )

    def _chart_region_at(self, x: int, y: int) -> dict[str, object] | None:
        for region in self.chart_hover_regions:
            if (
                region["x0"] <= x <= region["x1"]
                and region["y0"] <= y <= region["y1"]
            ):
                return region
        return None

    def _on_chart_motion(self, event) -> None:
        region = self._chart_region_at(event.x, event.y)
        if region is not None:
            self._show_chart_tooltip(event.x, event.y, str(region["text"]))
            return
        self._hide_chart_tooltip()

    def _on_chart_leave(self, _event) -> None:
        self._hide_chart_tooltip()

    def _on_chart_right_click(self, event) -> None:
        region = self._chart_region_at(event.x, event.y)
        if region is None:
            self._append_debug_log(
                f"right_click_miss x={event.x} y={event.y} mode={self.selected_chart_mode} metric={self.selected_metric_mode} interval={self.selected_interval}"
            )
            return
        self.chart_context_region = region
        bucket = region.get("bucket")
        if bucket is not None:
            display_total = int(region.get("display_total") or 0)
            self._append_debug_log(
                "right_click_bucket "
                f"bucket_start={bucket.start_at.isoformat()} "
                f"bucket_end={bucket.end_at.isoformat()} "
                f"bucket_total={bucket.total_tokens} "
                f"display_total={display_total} "
                f"mode={self.selected_chart_mode} "
                f"metric={self.selected_metric_mode} "
                f"interval={self.selected_interval} "
                f"x={event.x} y={event.y}"
            )
        self._show_chart_tooltip(event.x, event.y, str(region["text"]))
        try:
            self.chart_context_menu.tk_popup(event.x_root, event.y_root)
        finally:
            self.chart_context_menu.grab_release()

    def _investigate_selected_bucket(self) -> None:
        region = self.chart_context_region
        if region is None:
            self.status_label.configure(text="Select a bucket before launching Codex investigation.")
            return
        codex_executable = shutil.which("codex")
        if codex_executable is None:
            self.status_label.configure(text="Codex CLI was not found in PATH.")
            return
        bucket = region.get("bucket")
        if bucket is None:
            self.status_label.configure(text="No bucket context is available for investigation.")
            return
        investigation = build_bucket_investigation(
            bucket,
            self.latest_events,
            self.latest_session_context_markers,
            self.selected_interval,
            self.selected_chart_mode,
            Path(self.config.codex_root),
        )
        brief_path = write_bucket_investigation(
            investigation,
            default_investigations_path(),
            datetime.now(),
        )
        report_path = report_path_for_brief(brief_path)
        launch_command = build_codex_launch_command(
            codex_executable,
            brief_path,
            report_path,
            investigation.workspace_root,
            investigation.add_dirs,
        )
        self._append_debug_log(f"investigation_brief path={brief_path}")
        self._append_debug_log(f"investigation_report path={report_path}")
        self._append_debug_log(
            f"investigation_command {subprocess.list2cmdline(launch_command)}"
        )
        try:
            subprocess.Popen(
                launch_command,
                cwd=str(investigation.workspace_root),
                creationflags=getattr(subprocess, "CREATE_NEW_CONSOLE", 0),
            )
        except OSError as exc:
            self._append_debug_log(f"investigation_launch_failed error={exc}")
            self.status_label.configure(text=f"Failed to launch Codex investigation: {exc}")
            return
        self._append_debug_log("investigation_launch_started")
        self.status_label.configure(
            text=(
                "Codex investigation launched for "
                f"{bucket.start_at.strftime('%I:%M %p').lstrip('0')}. "
                f"Report: {report_path.name}"
            )
        )

    def _show_chart_tooltip(self, x: int, y: int, text: str) -> None:
        self.canvas.delete("chart-tooltip")
        tooltip_text = self.canvas.create_text(
            x + 14,
            y + 14,
            anchor="nw",
            text=text,
            justify="left",
            fill="#dfe2eb",
            font=("Inter", 8),
            tags="chart-tooltip",
        )
        bbox = self.canvas.bbox(tooltip_text)
        if bbox is None:
            return
        left, top, right, bottom = bbox
        shift_x = 0
        shift_y = 0
        canvas_width = max(int(self.canvas.winfo_width()), int(self.canvas["width"]))
        canvas_height = max(int(self.canvas.winfo_height()), int(self.canvas["height"]))
        if right + 8 > canvas_width:
            shift_x = canvas_width - right - 8
        if bottom + 8 > canvas_height:
            shift_y = canvas_height - bottom - 8
        if shift_x or shift_y:
            self.canvas.move(tooltip_text, shift_x, shift_y)
            bbox = self.canvas.bbox(tooltip_text)
            if bbox is None:
                return
            left, top, right, bottom = bbox
        background = self.canvas.create_rectangle(
            left - 6,
            top - 5,
            right + 6,
            bottom + 5,
            fill="#0d131b",
            outline="#39424d",
            width=1,
            tags="chart-tooltip",
        )
        self.canvas.tag_lower(background, tooltip_text)

    def _hide_chart_tooltip(self) -> None:
        self.canvas.delete("chart-tooltip")

    def _append_debug_log(self, message: str) -> None:
        self.debug_log_path.parent.mkdir(parents=True, exist_ok=True)
        timestamp = datetime.now().isoformat(timespec="seconds")
        with self.debug_log_path.open("a", encoding="utf-8") as handle:
            handle.write(f"{timestamp} {message}\n")

    def save_budget(self) -> None:
        raw_value = self.weekly_budget_var.get().strip()
        try:
            budget = parse_budget_billions(raw_value)
        except ValueError:
            self.status_label.configure(text="Weekly budget must be a number of billions, for example 3.5.")
            return
        self.config.weekly_budget_tokens = max(1, budget)
        save_config(self.config, self.config_path)
        self.weekly_budget_var.set(format_budget_billions(self.config.weekly_budget_tokens))
        self.status_label.configure(text=f"Saved weekly budget: {format_budget_billions(self.config.weekly_budget_tokens)}B")
        self.refresh_data()

    def toggle_overlay(self) -> None:
        if self.smoke_artifact_dir is not None:
            self.smoke_hotkey_triggered = True
        if self.overlay_visible:
            self.hide_overlay()
        else:
            self.show_overlay()

    def show_overlay(self) -> None:
        # Task-0013 Objective 3 (activation-fix follow-up): the global hotkey
        # TOGGLES VISIBILITY ONLY of a persistent, already-rendered overlay. The
        # window is built once and kept current by the background ingest poll
        # (`refresh_data`), so showing it performs NO re-aggregation, NO bucket
        # rebuild, NO DB read, and NO full re-render. It only reveals the window
        # the OS already has laid out, so the toggle costs only the window
        # map/paint + the hotkey detection, not per-event work.
        #
        # If the very first hotkey press happens before the startup pre-render
        # snapshot has landed, the overlay shows its pre-built empty state and the
        # next background poll fills it in (freshness within one poll interval is
        # acceptable; an instant show is the priority). The startup pre-render is
        # dispatched off-thread in __init__ so the first toggle is still fast and
        # this path never has to rebuild state on show.
        self.overlay.deiconify()
        self.overlay_visible = True
        self.overlay.lift()
        self.overlay.focus_force()

    def hide_overlay(self) -> None:
        self.chart_context_region = None
        self._hide_chart_tooltip()
        self.overlay.withdraw()
        self.overlay_visible = False

    def quit(self) -> None:
        if self._quitting:
            return
        self._quitting = True
        if self.hotkey_registered:
            self.hotkey.unregister()
        unload_private_font_assets(self.loaded_font_assets)
        self.root.quit()
        self.overlay.destroy()
        self.root.destroy()

    def _run_smoke_capture(self) -> None:
        artifact_dir = self.smoke_artifact_dir
        if artifact_dir is None:
            return
        artifact_dir.mkdir(parents=True, exist_ok=True)
        if self.smoke_tab in {"usage", "jobs", "worktrees"}:
            self.select_tab(self.smoke_tab)
        if self.smoke_tab == "jobs":
            self.refresh_jobs_data(apply_changes=True)
        if not self.overlay_visible:
            self.smoke_overlay_fallback = True
            self.show_overlay()
        # The hotkey show/hide path deliberately does NOT render (the persistent
        # overlay is kept current by the background poll). For a DETERMINISTIC
        # capture, force a render here so the artifact always shows real data
        # regardless of whether the off-thread startup pre-render has landed yet.
        # This is capture-only and does not affect the production hotkey path.
        if self.active_tab == "usage":
            self.refresh_data()
        self.overlay.update_idletasks()
        self.overlay.update()
        write_overlay_capture(self.overlay, artifact_dir / "overlay.png")
        if self.active_tab == "usage":
            self.canvas.postscript(
                file=str(artifact_dir / "overlay-chart.ps"),
                colormode="color",
            )
        summary_lines = [
            f"active_tab={self.active_tab}",
            self.status_label.cget("text"),
            f"hotkey_triggered={self.smoke_hotkey_triggered}",
            f"overlay_fallback={self.smoke_overlay_fallback}",
        ]
        if self.active_tab == "usage":
            summary_lines.extend(
                [
                    f"interval={self.selected_interval}",
                    f"metric_mode={self.selected_metric_mode}",
                    f"weekly_budget={self.config.weekly_budget_tokens}",
                    f"7d_total={self.local_total_value.cget('text')}",
                    f"projected={self.projected_value.cget('text')}",
                    f"headroom={self.headroom_value.cget('text')}",
                    (
                        f"budget_line={format_token_value(interval_redline_tokens(self.config.weekly_budget_tokens, INTERVAL_SECONDS[self.selected_interval]))}"
                        if self.selected_metric_mode == "total"
                        else "budget_line=hidden_in_norm_mode"
                    ),
                    f"status={self.status_metric_value.cget('text')}",
                    self.advisory_label.cget("text"),
                ]
            )
        elif self.active_tab == "jobs":
            summary_lines.extend(
                [
                    f"jobs_backend={self.jobs_backend_url}",
                    f"jobs_declared={self.jobs_metric_values['total'].cget('text')}",
                    f"jobs_in_sync={self.jobs_metric_values['in_sync'].cget('text')}",
                    f"jobs_needs_attention={self.jobs_metric_values['attention'].cget('text')}",
                    f"jobs_last_reconciled={format_jobs_timestamp(self.jobs_snapshot.get('last_reconciled_at'))}",
                ]
            )
        else:
            visible = self._visible_worktrees()
            worktree_ids = [str(worktree.get("worktree_id") or "") for worktree in visible]
            allocated_ids = [
                str(worktree.get("worktree_id") or "") for worktree in visible if is_allocated(worktree)
            ]
            idle_ids = [
                str(worktree.get("worktree_id") or "")
                for worktree in visible
                if str(worktree.get("status") or "").lower() == "idle"
            ]
            summary_lines.extend(
                [
                    f"worktrees_backend={self.worktrees_backend_url}",
                    f"worktrees_repo_filter={self.worktrees_repo_filter}",
                    f"worktrees_repo_options={','.join(repo_filter_options(self.worktrees_repos))}",
                    f"worktrees_total={self.worktrees_summary_values['total'].cget('text')}",
                    f"worktrees_allocated={self.worktrees_summary_values['allocated'].cget('text')}",
                    f"worktrees_idle={self.worktrees_summary_values['idle'].cget('text')}",
                    f"worktrees_ids={','.join(worktree_ids)}",
                    f"worktrees_allocated_ids={','.join(allocated_ids)}",
                    f"worktrees_idle_ids={','.join(idle_ids)}",
                ]
            )
        summary = "\n".join(summary_lines)
        (artifact_dir / "overlay-summary.txt").write_text(summary, encoding="utf-8")
        os._exit(0)

    def _trigger_smoke_hotkey(self) -> None:
        if not self.hotkey_registered:
            self.toggle_overlay()
            return
        user32 = ctypes.windll.user32
        keybd_event = user32.keybd_event
        KEYEVENTF_KEYUP = 0x0002
        VK_CONTROL = 0x11
        VK_MENU = 0x12
        VK_SPACE = 0x20
        keybd_event(VK_CONTROL, 0, 0, 0)
        keybd_event(VK_MENU, 0, 0, 0)
        keybd_event(VK_SPACE, 0, 0, 0)
        keybd_event(VK_SPACE, 0, KEYEVENTF_KEYUP, 0)
        keybd_event(VK_MENU, 0, KEYEVENTF_KEYUP, 0)
        keybd_event(VK_CONTROL, 0, KEYEVENTF_KEYUP, 0)

    def run(self) -> None:
        self.root.mainloop()

    def _resolve_display_timezone(self) -> tzinfo:
        return datetime.now().astimezone().tzinfo or UTC

    def _timezone_label(self) -> str:
        return datetime.now(self.display_timezone).strftime("%Z") or "local"
