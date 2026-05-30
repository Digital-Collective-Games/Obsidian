from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

from app.codex_dashboard.config import DashboardConfig, load_config, save_config
from app.codex_dashboard.ui import DashboardApp, compute_overlay_geometry

# Representative monitor: 1920x1080 with a 40px bottom taskbar (work area bottom
# 1040 < full screen height 1080, so "taskbar present").
SCREEN_W = 1920
SCREEN_H = 1080
WORK_AREA = (0, 0, 1920, 1040)


def _parse(geometry: str) -> tuple[int, int, int, int]:
    """Parse a Tk "WxH+X+Y" string (non-negative offsets) into (w, h, x, y)."""
    size, x_str, y_str = geometry.split("+")
    width_str, height_str = size.split("x")
    return int(width_str), int(height_str), int(x_str), int(y_str)


class PadFractionConfigTests(unittest.TestCase):
    def test_pad_fraction_round_trips_through_config(self) -> None:
        # AC1: a non-default pad_fraction survives save -> load.
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "config.json"
            config = DashboardConfig.defaults()
            config.pad_fraction = 0.10
            save_config(config, path)
            self.assertEqual(load_config(path).pad_fraction, 0.10)

    def test_missing_pad_fraction_loads_default(self) -> None:
        # AC1: a config payload without the field loads the 0.05 default cleanly.
        defaults = DashboardConfig.defaults()
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "config.json"
            path.write_text(
                json.dumps({"codex_root": defaults.codex_root, "db_path": defaults.db_path}),
                encoding="utf-8",
            )
            self.assertEqual(load_config(path).pad_fraction, 0.05)

    def test_default_pad_fraction_is_five_percent(self) -> None:
        self.assertEqual(DashboardConfig.defaults().pad_fraction, 0.05)


class ComputeOverlayGeometryTests(unittest.TestCase):
    def test_returns_geometry_string_with_no_tk(self) -> None:
        # AC2: pure function, callable with mocked inputs, returns "WxH+X+Y".
        geometry = compute_overlay_geometry(SCREEN_W, SCREEN_H, WORK_AREA, "usage", 0.05)
        self.assertRegex(geometry, r"^\d+x\d+\+\d+\+\d+$")

    def test_jobs_tasks_height_is_usable_minus_two_pad(self) -> None:
        # AC3
        pad = round(0.05 * SCREEN_H)
        expected = (WORK_AREA[3] - WORK_AREA[1]) - 2 * pad
        for tab in ("jobs", "tasks"):
            _, height, _, _ = _parse(
                compute_overlay_geometry(SCREEN_W, SCREEN_H, WORK_AREA, tab, 0.05)
            )
            self.assertEqual(height, expected, tab)

    def test_top_y_is_wa_top_plus_pad_for_all_tabs(self) -> None:
        # AC4: use a top-docked taskbar so wa_top != 0 (catches ignoring wa_top).
        work_area = (0, 48, 1920, 1080)
        pad = round(0.05 * SCREEN_H)
        for tab in ("usage", "jobs", "tasks"):
            _, _, _, y = _parse(
                compute_overlay_geometry(SCREEN_W, SCREEN_H, work_area, tab, 0.05)
            )
            self.assertEqual(y, work_area[1] + pad, tab)

    def test_taskbar_never_covered(self) -> None:
        # AC5: jobs/tasks bottom == wa_bottom - pad <= wa_bottom (positive gap).
        pad = round(0.05 * SCREEN_H)
        wa_bottom = WORK_AREA[3]
        for tab in ("jobs", "tasks"):
            _, height, _, y = _parse(
                compute_overlay_geometry(SCREEN_W, SCREEN_H, WORK_AREA, tab, 0.05)
            )
            bottom = y + height
            self.assertLessEqual(bottom, wa_bottom, tab)
            self.assertEqual(bottom, wa_bottom - pad, tab)
            self.assertGreater(wa_bottom - bottom, 0, tab)
        # usage also stays within the work area.
        _, height, _, y = _parse(
            compute_overlay_geometry(SCREEN_W, SCREEN_H, WORK_AREA, "usage", 0.05)
        )
        self.assertLessEqual(y + height, wa_bottom)

    def test_usage_keeps_current_size(self) -> None:
        # AC6
        width, height, _, _ = _parse(
            compute_overlay_geometry(SCREEN_W, SCREEN_H, WORK_AREA, "usage", 0.05)
        )
        self.assertEqual(width, 980)
        self.assertEqual(height, 660)
        pad = round(0.05 * SCREEN_H)
        self.assertNotEqual(height, (WORK_AREA[3] - WORK_AREA[1]) - 2 * pad)

    def test_width_stays_980_for_all_tabs(self) -> None:
        # AC7: width is the 980 clamp on a wide screen and is never widened past it.
        for tab in ("usage", "jobs", "tasks"):
            width, _, _, _ = _parse(
                compute_overlay_geometry(SCREEN_W, SCREEN_H, WORK_AREA, tab, 0.05)
            )
            self.assertEqual(width, 980, tab)
        # On a narrow work area the width drops to the clamp value (min(980, ...)),
        # proving 980 is a ceiling and not a hardcoded constant.
        narrow = (0, 0, 900, 1080)
        for tab in ("usage", "jobs", "tasks"):
            width, _, _, _ = _parse(
                compute_overlay_geometry(900, SCREEN_H, narrow, tab, 0.05)
            )
            self.assertLess(width, 980, tab)
            self.assertEqual(width, max(860, narrow[2] - 80), tab)

    def test_padding_is_configurable(self) -> None:
        # AC8: larger pad_fraction -> smaller jobs/tasks height and larger top y.
        _, height_5, _, y_5 = _parse(
            compute_overlay_geometry(SCREEN_W, SCREEN_H, WORK_AREA, "jobs", 0.05)
        )
        _, height_10, _, y_10 = _parse(
            compute_overlay_geometry(SCREEN_W, SCREEN_H, WORK_AREA, "jobs", 0.10)
        )
        self.assertLess(height_10, height_5)
        self.assertGreater(y_10, y_5)

    def test_x_right_aligned_within_work_area(self) -> None:
        # AC9: with a right-docked taskbar (wa_right strictly inside screen_width by
        # more than margin_x), x is right-aligned within the WORK-AREA width, not the
        # full screen width. The exact-x assertion fails a buggy impl that fed
        # screen_width (1920) into the x computation: that would yield x=900, not 780.
        work_area = (0, 0, 1800, 1080)
        margin_x = 40
        for tab in ("usage", "jobs", "tasks"):
            width, _, x, _ = _parse(
                compute_overlay_geometry(SCREEN_W, SCREEN_H, work_area, tab, 0.05)
            )
            self.assertEqual(x, work_area[2] - width - margin_x, tab)
            self.assertLessEqual(x + width, work_area[2], tab)


class SelectTabGeometryTests(unittest.TestCase):
    def test_select_tab_reapplies_geometry_without_rebuild(self) -> None:
        # AC10: tab switch issues the geometry re-apply and does NOT rebuild data.
        app = SimpleNamespace(
            active_tab="usage",
            _prime_jobs_snapshot=mock.Mock(),
            _prime_tasks_snapshot=mock.Mock(),
            _render_active_tab=mock.Mock(),
            _apply_overlay_geometry=mock.Mock(),
            refresh_data=mock.Mock(),
        )

        DashboardApp.select_tab(app, "jobs")

        self.assertEqual(app.active_tab, "jobs")
        app._apply_overlay_geometry.assert_called_once()
        app._render_active_tab.assert_called_once()
        app._prime_jobs_snapshot.assert_called_once()
        app.refresh_data.assert_not_called()


class OverlayGeometryDelegationTests(unittest.TestCase):
    """Cover the non-pure glue: _overlay_geometry's delegation, tab selection, and
    the work-area-query fallback (the construction-path clause of AC10)."""

    def _app(self, tab: str = "usage", pad_fraction: float = 0.05) -> SimpleNamespace:
        return SimpleNamespace(
            root=SimpleNamespace(
                winfo_screenwidth=lambda: SCREEN_W,
                winfo_screenheight=lambda: SCREEN_H,
            ),
            active_tab=tab,
            config=SimpleNamespace(pad_fraction=pad_fraction),
        )

    def test_overlay_geometry_delegates_with_work_area_and_active_tab(self) -> None:
        app = self._app(tab="usage")
        with mock.patch(
            "app.codex_dashboard.ui.query_primary_work_area", return_value=WORK_AREA
        ):
            geometry = DashboardApp._overlay_geometry(app)
        self.assertEqual(
            geometry,
            compute_overlay_geometry(SCREEN_W, SCREEN_H, WORK_AREA, "usage", 0.05),
        )

    def test_overlay_geometry_falls_back_to_full_screen_on_oserror(self) -> None:
        app = self._app(tab="jobs")
        with mock.patch(
            "app.codex_dashboard.ui.query_primary_work_area",
            side_effect=OSError("no display"),
        ):
            geometry = DashboardApp._overlay_geometry(app)
        self.assertEqual(
            geometry,
            compute_overlay_geometry(
                SCREEN_W, SCREEN_H, (0, 0, SCREEN_W, SCREEN_H), "jobs", 0.05
            ),
        )


if __name__ == "__main__":
    unittest.main()
