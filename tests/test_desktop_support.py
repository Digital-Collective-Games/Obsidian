from __future__ import annotations

import tempfile
import unittest
from datetime import datetime
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

from app.codex_dashboard.hotkey import MOD_ALT, MOD_CONTROL, GlobalHotkey, parse_hotkey
from app.codex_dashboard.config import (
    DashboardConfig,
    advisory_implied_weekly_budget_tokens,
    maybe_upgrade_weekly_budget,
)
from app.codex_dashboard.startup import startup_command
from app.codex_dashboard.ui import (
    DEFAULT_CHART_BUCKET_COUNT,
    DashboardApp,
    chart_bucket_count,
    format_budget_billions,
    format_chart_title,
    format_jobs_timestamp,
    format_tasks_timestamp,
    format_repo_tooltip,
    format_reset_remaining,
    format_tick_label,
    format_signed_token_value,
    format_token_value,
    format_velocity_tooltip,
    interval_redline_tokens,
    jobs_needs_attention_count,
    parse_budget_billions,
    rolling_average_tokens,
    usage_history_lookback,
    write_overlay_capture,
)


class DesktopSupportTests(unittest.TestCase):
    def test_parse_hotkey_supports_ctrl_alt_space(self) -> None:
        modifiers, virtual_key = parse_hotkey("Ctrl+Alt+Space")
        self.assertEqual(modifiers, MOD_CONTROL | MOD_ALT)
        self.assertEqual(virtual_key, 0x20)

    def test_startup_command_uses_pinned_dashboard_launcher(self) -> None:
        fake_app_data = Path("C:/Users/gregs/AppData/Local/CodexDashboard")
        fake_powershell = Path("C:/Windows/System32/WindowsPowerShell/v1.0/powershell.exe")
        with mock.patch("app.codex_dashboard.startup.app_data_root", return_value=fake_app_data):
            with mock.patch(
                "app.codex_dashboard.startup.startup_powershell_executable",
                return_value=fake_powershell,
            ):
                command = startup_command()
        self.assertNotIn("C:\\Agent\\CodexDashboard", command)
        self.assertIn(
            '"C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe" -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File',
            command,
        )
        self.assertIn(
            "C:\\Users\\gregs\\AppData\\Local\\CodexDashboard\\dashboard-launcher\\Start-CodexDashboard.ps1",
            command,
        )

    def test_global_hotkey_poll_drains_pending_callbacks(self) -> None:
        callback = mock.Mock()
        hotkey = GlobalHotkey("Ctrl+Alt+Space", callback)
        hotkey._pending_callbacks.put(None)
        hotkey._pending_callbacks.put(None)

        hotkey.poll()

        self.assertEqual(callback.call_count, 2)

    def test_format_tick_label_uses_compact_ampm_hours(self) -> None:
        self.assertEqual(format_tick_label(datetime(2026, 4, 2, 19, 0), "1h"), "7PM")
        self.assertEqual(format_tick_label(datetime(2026, 4, 2, 5, 0), "1h"), "5AM")
        self.assertEqual(format_tick_label(datetime(2026, 4, 2, 17, 15), "15m"), "5:15PM")

    def test_format_token_value_uses_human_suffixes(self) -> None:
        self.assertEqual(format_token_value(999), "999")
        self.assertEqual(format_token_value(100_000), "100K")
        self.assertEqual(format_token_value(2_500_000), "2.5M")

    def test_format_signed_token_value_shows_direction(self) -> None:
        self.assertEqual(format_signed_token_value(250_000), "+250K")
        self.assertEqual(format_signed_token_value(-2_500_000), "-2.5M")
        self.assertEqual(format_signed_token_value(0), "0")

    def test_format_chart_title_uses_interval_name(self) -> None:
        self.assertEqual(format_chart_title("1h"), "Token Velocity per 1 Hour")
        self.assertEqual(format_chart_title("5m"), "Token Velocity per 5 Minutes")
        self.assertEqual(format_chart_title("15m", "repo"), "Repo Share per 15 Minutes")
        self.assertEqual(
            format_chart_title("15m", "velocity", "norm"),
            "Normalized Token Velocity per 15 Minutes",
        )
        self.assertEqual(
            format_chart_title("15m", "repo", "norm"),
            "Normalized Repo Share per 15 Minutes",
        )

    def test_velocity_tooltip_uses_scalar_human_format(self) -> None:
        self.assertEqual(format_velocity_tooltip(2_500_000), "2.5M")

    def test_repo_tooltip_lists_nonzero_repo_breakdown(self) -> None:
        tooltip = format_repo_tooltip(
            {
                "repo-a": 2_500_000,
                "repo-b": 0,
                "__other__": 1_200_000,
            },
            [
                ("repo-a", "RepoA"),
                ("repo-b", "RepoB"),
                ("__other__", "Other"),
            ],
        )
        self.assertEqual(tooltip, "RepoA: 2.5M\nOther: 1.2M")

    def test_budget_billions_helpers_format_and_parse(self) -> None:
        self.assertEqual(format_budget_billions(3_550_000_000), "3.5")
        self.assertEqual(parse_budget_billions("3.5"), 3_500_000_000)
        self.assertEqual(parse_budget_billions("3.5B"), 3_500_000_000)
        self.assertEqual(parse_budget_billions("3550000000"), 3_550_000_000)

    def test_format_reset_remaining_prefers_days_then_hours(self) -> None:
        now = datetime(2026, 4, 2, 22, 0)
        self.assertEqual(
            format_reset_remaining(int(datetime(2026, 4, 8, 0, 0).timestamp()), now=now),
            "5.1d",
        )
        self.assertEqual(
            format_reset_remaining(int(datetime(2026, 4, 3, 1, 0).timestamp()), now=now),
            "3.0h",
        )

    def test_interval_redline_tokens_scales_budget_to_bucket_size(self) -> None:
        self.assertEqual(interval_redline_tokens(8_000_000, 3600), 47_619)

    def test_rolling_average_tokens_uses_last_n_buckets(self) -> None:
        buckets = [
            SimpleNamespace(total_tokens=5),
            SimpleNamespace(total_tokens=10),
            SimpleNamespace(total_tokens=15),
            SimpleNamespace(total_tokens=20),
            SimpleNamespace(total_tokens=30),
        ]
        self.assertEqual(rolling_average_tokens(buckets, 4), 19)

    def test_chart_bucket_count_expands_daily_view(self) -> None:
        self.assertEqual(chart_bucket_count("1d"), 35)
        self.assertEqual(chart_bucket_count("1h"), DEFAULT_CHART_BUCKET_COUNT)

    def test_usage_history_lookback_expands_daily_chart_beyond_summary_window(self) -> None:
        self.assertEqual(
            usage_history_lookback("1d").days,
            35,
        )

    def test_usage_history_lookback_keeps_shorter_intervals_at_seven_days(self) -> None:
        self.assertEqual(usage_history_lookback("1h").days, 7)

    def test_advisory_implied_weekly_budget_rounds_to_nearest_50m(self) -> None:
        self.assertEqual(
            advisory_implied_weekly_budget_tokens(1_838_350_974, 52.0),
            3_550_000_000,
        )

    def test_maybe_upgrade_weekly_budget_replaces_legacy_default(self) -> None:
        config = DashboardConfig(
            codex_root="C:/Users/gregs/.codex",
            db_path="C:/Users/gregs/AppData/Local/CodexDashboard/dashboard.db",
            weekly_budget_tokens=8_000_000,
        )

        updated = maybe_upgrade_weekly_budget(config, 1_838_350_974, 52.0)

        self.assertTrue(updated)
        self.assertEqual(config.weekly_budget_tokens, 3_550_000_000)

    def test_jobs_needs_attention_count_excludes_in_sync(self) -> None:
        self.assertEqual(
            jobs_needs_attention_count(
                {
                    "in_sync": 2,
                    "drifted": 1,
                    "missing": 1,
                    "blocked": 1,
                }
            ),
            3,
        )

    def test_format_jobs_timestamp_uses_local_clock_label(self) -> None:
        self.assertEqual(
            format_jobs_timestamp("2026-04-04T12:01:55-04:00"),
            "12:01 PM",
        )

    def test_format_tasks_timestamp_uses_date_and_local_clock(self) -> None:
        self.assertEqual(
            format_tasks_timestamp("2026-04-26T21:30:00-04:00"),
            "Apr 26, 9:30 PM",
        )

    def test_select_tab_does_not_trigger_jobs_refresh(self) -> None:
        app = SimpleNamespace(
            active_tab="usage",
            _prime_jobs_snapshot=mock.Mock(),
            _prime_tasks_snapshot=mock.Mock(),
            _render_active_tab=mock.Mock(),
            refresh_jobs_data=mock.Mock(),
        )

        DashboardApp.select_tab(app, "jobs")

        self.assertEqual(app.active_tab, "jobs")
        app._prime_jobs_snapshot.assert_called_once_with()
        app._render_active_tab.assert_called_once_with()
        app.refresh_jobs_data.assert_not_called()

    def test_select_tab_primes_tasks_snapshot_without_control_actions(self) -> None:
        app = SimpleNamespace(
            active_tab="usage",
            _prime_jobs_snapshot=mock.Mock(),
            _prime_tasks_snapshot=mock.Mock(),
            _render_active_tab=mock.Mock(),
        )

        DashboardApp.select_tab(app, "tasks")

        self.assertEqual(app.active_tab, "tasks")
        app._prime_tasks_snapshot.assert_called_once_with()
        app._prime_jobs_snapshot.assert_not_called()
        app._render_active_tab.assert_called_once_with()

    def test_prime_tasks_snapshot_uses_backend_snapshot(self) -> None:
        snapshot = {
            "status": "ok",
            "summary": {"needs_you": 1},
            "tasks": [{"task_id": "Task-0009"}],
        }
        app = SimpleNamespace(
            tasks_snapshot={"tasks": []},
            tasks_status_message="",
            tasks_backend_url="http://127.0.0.1:14318",
            _render_tasks_snapshot=mock.Mock(),
        )

        with mock.patch("app.codex_dashboard.ui.fetch_tasks_snapshot", return_value=snapshot):
            DashboardApp._prime_tasks_snapshot(app)

        self.assertEqual(app.tasks_snapshot, snapshot)
        self.assertEqual(app.tasks_status_message, "Tasks state loaded from orchestration backend.")
        app._render_tasks_snapshot.assert_called_once_with()

    def test_prime_jobs_snapshot_uses_backend_snapshot(self) -> None:
        snapshot = {
            "last_reconciled_at": "2026-04-07T14:49:45Z",
            "summary": {"in_sync": 1},
            "jobs": [{"job_id": "codex-daily-agentic-swe-digest"}],
        }
        app = SimpleNamespace(
            jobs_snapshot={"jobs": []},
            jobs_status_message="",
            jobs_backend_url="http://127.0.0.1:4318",
            _render_jobs_snapshot=mock.Mock(),
        )

        with mock.patch("app.codex_dashboard.ui.fetch_jobs_snapshot", return_value=snapshot):
            DashboardApp._prime_jobs_snapshot(app)

        self.assertEqual(app.jobs_snapshot, snapshot)
        self.assertEqual(app.jobs_status_message, "Jobs state loaded from orchestration backend.")
        app._render_jobs_snapshot.assert_called_once_with()

    def test_show_job_details_includes_backend_payload(self) -> None:
        jobs_detail_title = mock.Mock()
        jobs_detail_text = mock.Mock()
        jobs_detail_shell = mock.Mock()
        jobs_detail_shell.winfo_manager.return_value = False
        app = SimpleNamespace(
            jobs_detail_title=jobs_detail_title,
            jobs_detail_text=jobs_detail_text,
            jobs_detail_shell=jobs_detail_shell,
            jobs_rows_shell=object(),
        )

        DashboardApp._show_job_details(
            app,
            {
                "job_id": "codex-dashboard-startup",
                "label": "Obsidian overlay at sign-in",
                "kind": "orchestration_backend",
                "desired_state": "enabled",
                "desired_label": "Enabled",
                "observed_label": "Enabled",
                "status": "in_sync",
                "reason": "Backend state is current.",
                "definition": {
                    "executor": {"entrypoint": "agentic-swe-digest"},
                    "recent_runs": [{"workflow_id": "job/example"}],
                },
            },
        )

        inserted_text = jobs_detail_text.insert.call_args.args[1]
        self.assertIn('"executor"', inserted_text)
        self.assertIn('"recent_runs"', inserted_text)
        jobs_detail_shell.pack.assert_called_once()

    def test_refresh_jobs_data_syncs_backend_when_apply_changes(self) -> None:
        app = SimpleNamespace(
            jobs_snapshot={},
            jobs_backend_url="http://127.0.0.1:4318",
            jobs_status_message="",
            active_tab="jobs",
            status_label=mock.Mock(),
            _render_jobs_snapshot=mock.Mock(),
        )
        snapshot = {
            "last_reconciled_at": "2026-04-07T14:49:45Z",
            "summary": {"in_sync": 1},
            "jobs": [],
        }
        report = {"created_schedule_ids": ["schedule-a"], "updated_schedule_ids": [], "deleted_schedule_ids": []}

        with mock.patch("app.codex_dashboard.ui.sync_jobs_snapshot", return_value=(snapshot, report)):
            DashboardApp.refresh_jobs_data(app, apply_changes=True)

        self.assertEqual(app.jobs_snapshot, snapshot)
        self.assertEqual(app.jobs_status_message, "Jobs sync completed. 1 schedule changes.")
        app.status_label.configure.assert_called_once_with(text="Jobs sync completed. 1 schedule changes.")
        app._render_jobs_snapshot.assert_called_once_with()

    def test_run_job_now_starts_manual_run_and_refreshes_snapshot(self) -> None:
        app = SimpleNamespace(
            jobs_backend_url="http://127.0.0.1:4318",
            jobs_snapshot={},
            jobs_status_message="",
            active_tab="jobs",
            status_label=mock.Mock(),
            _render_jobs_snapshot=mock.Mock(),
        )
        refreshed_snapshot = {"summary": {"in_sync": 1}, "jobs": []}

        with mock.patch(
            "app.codex_dashboard.ui.start_job_run",
            return_value={"workflow_id": "job/codex-daily-agentic-swe-digest/manual/example"},
        ), mock.patch(
            "app.codex_dashboard.ui.fetch_jobs_snapshot",
            return_value=refreshed_snapshot,
        ):
            DashboardApp.run_job_now(
                app,
                {
                    "job_id": "codex-daily-agentic-swe-digest",
                    "label": "Codex Daily Agentic SWE Digest",
                    "supports_run_now": True,
                },
            )

        self.assertEqual(app.jobs_snapshot, refreshed_snapshot)
        self.assertIn("Run now started for Codex Daily Agentic SWE Digest", app.jobs_status_message)
        app.status_label.configure.assert_called_once()
        app._render_jobs_snapshot.assert_called_once_with()

    def test_execute_task_pause_calls_backend_interrupt_and_refreshes_snapshot(self) -> None:
        refreshed_snapshot = {"summary": {"needs_you": 1}, "tasks": []}
        app = SimpleNamespace(
            tasks_backend_url="http://127.0.0.1:14318",
            tasks_snapshot={},
            tasks_status_message="",
            status_label=mock.Mock(),
            _render_tasks_snapshot=mock.Mock(),
        )

        with mock.patch("app.codex_dashboard.ui.pause_task_run") as pause_run, mock.patch(
            "app.codex_dashboard.ui.fetch_tasks_snapshot",
            return_value=refreshed_snapshot,
        ):
            DashboardApp.execute_task_action(
                app,
                {
                    "label": "Pause",
                    "backend_action": "interrupt",
                    "run_id": "taskrun--Task-0009--active",
                    "allowed": True,
                },
            )

        pause_run.assert_called_once_with("taskrun--Task-0009--active", "http://127.0.0.1:14318")
        self.assertEqual(app.tasks_snapshot, refreshed_snapshot)
        self.assertEqual(app.tasks_status_message, "Pause requested for taskrun--Task-0009--active.")
        app.status_label.configure.assert_called_once_with(text=app.tasks_status_message)
        app._render_tasks_snapshot.assert_called_once_with()

    def test_execute_task_open_uses_backend_launch_target(self) -> None:
        app = SimpleNamespace(
            tasks_backend_url="http://127.0.0.1:14318",
            tasks_snapshot={},
            tasks_status_message="",
            status_label=mock.Mock(),
            _render_tasks_snapshot=mock.Mock(),
            _open_task_launch_target=mock.Mock(),
        )
        target = {"kind": "working_context", "uri": "C:/Agent/CodexDashboard"}

        DashboardApp.execute_task_action(
            app,
            {
                "label": "Open Working Context",
                "backend_action": "open",
                "target": target,
                "allowed": True,
            },
        )

        app._open_task_launch_target.assert_called_once_with(target)
        self.assertEqual(app.tasks_status_message, "Open Working Context opened.")
        app.status_label.configure.assert_called_once_with(text=app.tasks_status_message)
        app._render_tasks_snapshot.assert_called_once_with()

    def test_launch_target_allows_vscodium_commands_only(self) -> None:
        self.assertTrue(DashboardApp._is_allowed_launch_command("codium.cmd"))
        self.assertTrue(DashboardApp._is_allowed_launch_command("C:/Program Files/VSCodium/VSCodium.exe"))
        self.assertFalse(DashboardApp._is_allowed_launch_command("powershell.exe"))

    def test_open_task_launch_target_falls_back_from_code_to_vscodium(self) -> None:
        app = SimpleNamespace()

        def fake_which(name: str) -> str | None:
            if name == "codium":
                return "C:/Program Files/VSCodium/bin/codium.cmd"
            return None

        with mock.patch("app.codex_dashboard.ui.shutil.which", side_effect=fake_which), mock.patch(
            "app.codex_dashboard.ui.subprocess.Popen"
        ) as popen:
            DashboardApp._open_task_launch_target(
                app,
                {
                    "command": ["code", "C:\\Agent\\CodexDashboard\\Tracking\\Task-0011\\PLAN.md"],
                    "uri": "file:///C:/Agent/CodexDashboard/Tracking/Task-0011/PLAN.md",
                },
            )

        popen.assert_called_once()
        self.assertEqual(
            popen.call_args.args[0],
            ["C:/Program Files/VSCodium/bin/codium.cmd", "C:\\Agent\\CodexDashboard\\Tracking\\Task-0011\\PLAN.md"],
        )

    def test_open_task_launch_target_falls_back_to_normalized_file_uri(self) -> None:
        app = SimpleNamespace()

        with mock.patch("app.codex_dashboard.ui.shutil.which", return_value=None), mock.patch(
            "app.codex_dashboard.ui.subprocess.Popen"
        ) as popen, mock.patch("app.codex_dashboard.ui.os.startfile", create=True) as startfile:
            DashboardApp._open_task_launch_target(
                app,
                {
                    "command": ["code", "C:\\Agent\\CodexDashboard\\Tracking\\Task-0011\\PLAN.md"],
                    "uri": "file:///C:/Agent/CodexDashboard/Tracking/Task-0011/PLAN.md",
                },
            )

        popen.assert_not_called()
        startfile.assert_called_once()
        opened = startfile.call_args.args[0].replace("\\", "/")
        self.assertEqual(opened, "C:/Agent/CodexDashboard/Tracking/Task-0011/PLAN.md")

    def test_write_overlay_capture_uses_window_bounds(self) -> None:
        overlay = SimpleNamespace(
            winfo_rootx=lambda: 40,
            winfo_rooty=lambda: 60,
            winfo_width=lambda: 980,
            winfo_height=lambda: 660,
        )

        with tempfile.TemporaryDirectory() as tmpdir, mock.patch("app.codex_dashboard.ui.subprocess.run") as run:
            write_overlay_capture(overlay, Path(tmpdir) / "overlay.png")

        command = run.call_args.args[0]
        self.assertEqual(command[:3], ["powershell", "-NoProfile", "-Command"])
        self.assertIn("Rectangle(40, 60, 980, 660)", command[3])
        self.assertIn("overlay.png", command[3])

    def test_jobs_mousewheel_scrolls_canvas_when_jobs_tab_active(self) -> None:
        jobs_scroll_canvas = mock.Mock()
        app = SimpleNamespace(
            active_tab="jobs",
            jobs_scroll_canvas=jobs_scroll_canvas,
        )
        event = SimpleNamespace(delta=-120)

        result = DashboardApp._on_jobs_mousewheel(app, event)

        jobs_scroll_canvas.yview_scroll.assert_called_once_with(1, "units")
        self.assertEqual(result, "break")

    def test_toggle_overlay_hides_visible_overlay_from_explicit_flag(self) -> None:
        app = SimpleNamespace(
            smoke_artifact_dir=None,
            smoke_hotkey_triggered=False,
            overlay_visible=True,
            show_overlay=mock.Mock(),
            hide_overlay=mock.Mock(),
        )

        DashboardApp.toggle_overlay(app)

        app.hide_overlay.assert_called_once_with()
        app.show_overlay.assert_not_called()

    def test_show_overlay_only_reveals_persistent_window_no_rebuild(self) -> None:
        # Task-0013 activation-fix follow-up: the hotkey TOGGLES VISIBILITY ONLY
        # of a persistent, already-rendered overlay. show_overlay must NOT
        # re-aggregate, rebuild buckets, read the DB, or re-render on the toggle
        # path. It only reveals the window the OS already laid out.
        overlay = mock.Mock()
        markers = {"a": []}
        app = SimpleNamespace(
            overlay=overlay,
            overlay_visible=False,
            latest_events=["snapshot-event"],
            latest_session_context_markers=markers,
            _activation_load_in_flight=False,
            _load_dashboard_data=mock.Mock(),
            refresh_data=mock.Mock(),
            _render_dashboard=mock.Mock(),
            _start_activation_load=mock.Mock(),
            chart_context_region="selected",
            _hide_chart_tooltip=mock.Mock(),
        )

        DashboardApp.show_overlay(app)

        self.assertTrue(app.overlay_visible)
        # No DB read, no off-thread load, and NO re-render on the toggle path.
        app._load_dashboard_data.assert_not_called()
        app.refresh_data.assert_not_called()
        app._start_activation_load.assert_not_called()
        app._render_dashboard.assert_not_called()
        # It only reveals the persistent window.
        overlay.deiconify.assert_called_once_with()
        overlay.lift.assert_called_once_with()
        overlay.focus_force.assert_called_once_with()

        DashboardApp.hide_overlay(app)

        self.assertFalse(app.overlay_visible)
        self.assertIsNone(app.chart_context_region)
        app._hide_chart_tooltip.assert_called_once_with()
        overlay.withdraw.assert_called_once_with()

    def test_show_overlay_cold_snapshot_still_only_reveals_no_load(self) -> None:
        # Task-0013 activation-fix follow-up: even if the startup pre-render has
        # not landed yet (empty snapshot), the hotkey toggle still only reveals
        # the (pre-built empty-state) window. It does NOT dispatch a load or
        # render on the toggle path; the startup pre-render and background poll
        # own loading. This keeps the show instant.
        overlay = mock.Mock()
        app = SimpleNamespace(
            overlay=overlay,
            overlay_visible=False,
            latest_events=[],
            latest_session_context_markers={},
            _activation_load_in_flight=False,
            _load_dashboard_data=mock.Mock(),
            refresh_data=mock.Mock(),
            _render_dashboard=mock.Mock(),
            _start_activation_load=mock.Mock(),
        )

        DashboardApp.show_overlay(app)

        self.assertTrue(app.overlay_visible)
        overlay.deiconify.assert_called_once_with()
        app._load_dashboard_data.assert_not_called()
        app.refresh_data.assert_not_called()
        app._render_dashboard.assert_not_called()
        app._start_activation_load.assert_not_called()

    def test_start_activation_load_runs_load_off_thread(self) -> None:
        # Task-0013 Objective 3: the cold-start loader runs _load_dashboard_data
        # on a worker thread and posts the result to the ingest queue for the UI
        # thread to render.
        import queue as queue_module

        result_queue: queue_module.Queue = queue_module.Queue()
        app = SimpleNamespace(
            _activation_load_in_flight=False,
            _load_dashboard_data=mock.Mock(return_value=(["e"], {"a": []})),
            ingest_queue=result_queue,
        )

        captured = {}

        class _ImmediateThread:
            def __init__(self, target, daemon=None):
                captured["target"] = target

            def start(self):
                captured["target"]()

        with mock.patch("app.codex_dashboard.ui.threading.Thread", _ImmediateThread):
            DashboardApp._start_activation_load(app)

        self.assertTrue(app._activation_load_in_flight)
        app._load_dashboard_data.assert_called_once_with()
        event_type, payload = result_queue.get_nowait()
        self.assertEqual(event_type, "dashboard_data")
        self.assertEqual(payload, (["e"], {"a": []}))

    def test_schedule_ingest_requeues_even_when_previous_scan_is_still_running(self) -> None:
        root = mock.Mock()
        app = SimpleNamespace(
            _quitting=False,
            root=root,
            config=SimpleNamespace(polling_seconds=5),
            ingest_in_flight=True,
            schedule_ingest=mock.Mock(),
        )

        with mock.patch("app.codex_dashboard.ui.threading.Thread") as thread_ctor:
            DashboardApp.schedule_ingest(app)

        root.after.assert_called_once_with(5000, app.schedule_ingest)
        thread_ctor.assert_not_called()

    def test_schedule_ingest_starts_worker_and_queues_next_poll(self) -> None:
        root = mock.Mock()
        thread = mock.Mock()
        app = SimpleNamespace(
            _quitting=False,
            root=root,
            config=SimpleNamespace(
                polling_seconds=5,
                db_path="C:/Users/gregs/AppData/Local/CodexDashboard/dashboard.db",
            ),
            ingest_in_flight=False,
            ingest_queue=mock.Mock(),
            schedule_ingest=mock.Mock(),
        )

        with mock.patch("app.codex_dashboard.ui.threading.Thread", return_value=thread) as thread_ctor:
            DashboardApp.schedule_ingest(app)

        self.assertTrue(app.ingest_in_flight)
        root.after.assert_called_once_with(5000, app.schedule_ingest)
        thread_ctor.assert_called_once()
        thread.start.assert_called_once_with()


if __name__ == "__main__":
    unittest.main()
