"""Task-0013 tests: Obsidian rebrand, Claude token merge, and source filter.

Covers Objective 1 (rebrand display strings), Objective 2 (Claude Code token
ingest with per-request dedup, source tagging, idempotent re-scan, and merged
totals), and Objective 4 (source filter aggregation). Objective 3 (non-blocking
activation + timestamp index) is covered in ``tests/test_desktop_support.py``.

Fixtures are task-owned and written to a temp directory; no live ``~/.claude`` or
``%LOCALAPPDATA%`` data is read.
"""

from __future__ import annotations

import json
import tempfile
import unittest
from datetime import UTC, datetime, timedelta
from pathlib import Path

from app.codex_dashboard import APP_NAME
from app.codex_dashboard.aggregation import (
    KNOWN_SOURCES,
    build_buckets,
    filter_events_by_source,
)
from app.codex_dashboard.config import DashboardConfig, load_config, save_config
from app.codex_dashboard.models import TokenEvent
from app.codex_dashboard.scanner import (
    claude_jsonl_files,
    ingest_once,
    parse_claude_token_events,
)
from app.codex_dashboard.storage import (
    connect,
    count_events,
    initialize_db,
    load_events_since,
)
from app.codex_dashboard.ui import format_token_value


CODEX_TOKEN_EVENT_LINE = (
    b'{"timestamp":"2026-04-03T00:09:11.080Z","type":"event_msg","payload":'
    b'{"type":"token_count","info":{"total_token_usage":{"total_tokens":7242153},'
    b'"last_token_usage":{"input_tokens":100970,"cached_input_tokens":97920,'
    b'"output_tokens":398,"reasoning_output_tokens":30,"total_tokens":101368}},'
    b'"rate_limits":{"secondary":{"used_percent":42.0,"window_minutes":10080,'
    b'"resets_at":1775638824}}}}'
)


def _claude_assistant_line(
    request_id: str,
    timestamp: str,
    *,
    input_tokens: int,
    cache_creation: int,
    cache_read: int,
    output_tokens: int,
) -> bytes:
    payload = {
        "type": "assistant",
        "requestId": request_id,
        "timestamp": timestamp,
        "message": {
            "usage": {
                "input_tokens": input_tokens,
                "cache_creation_input_tokens": cache_creation,
                "cache_read_input_tokens": cache_read,
                "output_tokens": output_tokens,
            }
        },
    }
    return (json.dumps(payload) + "\n").encode("utf-8")


class RebrandTests(unittest.TestCase):
    """Objective 1: user-facing surfaces present 'Obsidian', not 'CodexDashboard'."""

    def test_app_name_is_obsidian(self) -> None:
        self.assertEqual(APP_NAME, "Obsidian")

    def test_no_codexdashboard_product_name_in_user_facing_surfaces(self) -> None:
        repo_root = Path(__file__).resolve().parents[1]
        checks = {
            repo_root / "app" / "codex_dashboard" / "__init__.py": ('APP_NAME = "Obsidian"',),
            repo_root / "app" / "codex_dashboard" / "__main__.py": ("Obsidian ingest utility",),
            repo_root / "app" / "codex_dashboard" / "jobs.py": ("Obsidian overlay at sign-in",),
        }
        for path, expected_strings in checks.items():
            text = path.read_text(encoding="utf-8")
            for expected in expected_strings:
                self.assertIn(expected, text, f"{path} missing {expected!r}")

    def test_ui_window_title_and_brand_render_obsidian(self) -> None:
        ui_text = (
            Path(__file__).resolve().parents[1]
            / "app"
            / "codex_dashboard"
            / "ui.py"
        ).read_text(encoding="utf-8")
        self.assertIn('self.root.title("OBSIDIAN")', ui_text)
        self.assertIn('text="OBSIDIAN", style="Brand.TLabel"', ui_text)
        self.assertNotIn('self.root.title("CODEX DASHBOARD")', ui_text)
        self.assertNotIn('text="CODEX_DASHBOARD"', ui_text)

    def test_readme_title_is_obsidian(self) -> None:
        readme = (Path(__file__).resolve().parents[1] / "README.md").read_text(
            encoding="utf-8"
        )
        first_line = readme.splitlines()[0]
        self.assertEqual(first_line.strip(), "# Obsidian")


class ConfigClaudeRootTests(unittest.TestCase):
    """Objective 2: claude_root round-trips and absent claude_root is benign."""

    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.root = Path(self.temp_dir.name)

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def test_claude_root_round_trips_through_save_and_load(self) -> None:
        config_path = self.root / "config.json"
        config = DashboardConfig.defaults()
        config.claude_root = str(self.root / "my-claude")
        save_config(config, config_path)
        reloaded = load_config(config_path)
        self.assertEqual(reloaded.claude_root, str(self.root / "my-claude"))

    def test_empty_claude_root_ingests_cleanly_with_no_claude_events(self) -> None:
        codex_root = self.root / ".codex"
        session_dir = codex_root / "sessions" / "2026" / "04" / "03"
        session_dir.mkdir(parents=True, exist_ok=True)
        (session_dir / "rollout.jsonl").write_bytes(CODEX_TOKEN_EVENT_LINE + b"\n")
        db_path = self.root / "dashboard.db"
        connection = connect(db_path)
        initialize_db(connection)
        config = DashboardConfig(
            codex_root=str(codex_root),
            db_path=str(db_path),
            claude_root="",
        )
        result = ingest_once(connection, config)
        self.assertEqual(result.events_ingested, 1)
        events = load_events_since(connection, datetime(2026, 1, 1, tzinfo=UTC))
        self.assertTrue(all(event.source == "codex" for event in events))
        connection.close()


class ClaudeParserTests(unittest.TestCase):
    """Objective 2: per-request dedup, canonical total, and column mapping."""

    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.root = Path(self.temp_dir.name)
        self.claude_root = self.root / ".claude"
        self.project_dir = self.claude_root / "projects" / "C--Agent-Repo"
        self.project_dir.mkdir(parents=True, exist_ok=True)

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def _write_transcript(self, name: str, lines: list[bytes]) -> Path:
        path = self.project_dir / name
        path.write_bytes(b"".join(lines))
        return path

    def test_claude_jsonl_files_discovers_project_transcripts(self) -> None:
        path = self._write_transcript(
            "session-a.jsonl",
            [
                _claude_assistant_line(
                    "req-1",
                    "2026-05-01T00:00:00Z",
                    input_tokens=10,
                    cache_creation=0,
                    cache_read=0,
                    output_tokens=5,
                )
            ],
        )
        files = claude_jsonl_files(self.claude_root)
        self.assertIn(path, files)

    def test_per_request_dedup_uses_last_event_not_line_sum(self) -> None:
        # One request streamed across three assistant lines with growing usage.
        # The final (last) usage block is the cumulative truth; summing all three
        # would massively over-count.
        lines = [
            _claude_assistant_line(
                "req-A",
                "2026-05-01T00:00:01Z",
                input_tokens=100,
                cache_creation=0,
                cache_read=0,
                output_tokens=10,
            ),
            _claude_assistant_line(
                "req-A",
                "2026-05-01T00:00:02Z",
                input_tokens=100,
                cache_creation=0,
                cache_read=0,
                output_tokens=40,
            ),
            _claude_assistant_line(
                "req-A",
                "2026-05-01T00:00:03Z",
                input_tokens=100,
                cache_creation=0,
                cache_read=0,
                output_tokens=90,
            ),
        ]
        path = self._write_transcript("dedup.jsonl", lines)
        events = parse_claude_token_events(path)
        self.assertEqual(len(events), 1, "expected one event per requestId")
        event = events[0]
        # Canonical total = input + cache_creation + cache_read + output for the
        # LAST line only: 100 + 0 + 0 + 90 = 190 (NOT the 100+10+100+40+100+90 sum).
        self.assertEqual(event.total_tokens, 190)
        self.assertEqual(event.output_tokens, 90)
        self.assertEqual(event.source, "claude")
        self.assertEqual(event.source_event_id, "req-A")

    def test_canonical_total_counts_cache_creation_as_input(self) -> None:
        path = self._write_transcript(
            "formula.jsonl",
            [
                _claude_assistant_line(
                    "req-B",
                    "2026-05-01T01:00:00Z",
                    input_tokens=200,
                    cache_creation=300,
                    cache_read=400,
                    output_tokens=50,
                )
            ],
        )
        events = parse_claude_token_events(path)
        event = events[0]
        # total = input + cache_creation + cache_read + output
        self.assertEqual(event.total_tokens, 200 + 300 + 400 + 50)
        # input_tokens column folds cache_creation in (billed input cost).
        self.assertEqual(event.input_tokens, 200 + 300)
        self.assertEqual(event.cached_input_tokens, 400)
        self.assertEqual(event.reasoning_output_tokens, 0)

    def test_claude_events_have_null_codex_advisory_fields(self) -> None:
        path = self._write_transcript(
            "advisory.jsonl",
            [
                _claude_assistant_line(
                    "req-C",
                    "2026-05-01T02:00:00Z",
                    input_tokens=10,
                    cache_creation=0,
                    cache_read=0,
                    output_tokens=5,
                )
            ],
        )
        event = parse_claude_token_events(path)[0]
        self.assertIsNone(event.weekly_used_percent)
        self.assertIsNone(event.weekly_window_minutes)
        self.assertIsNone(event.weekly_resets_at)

    def test_multiple_requests_each_counted_once(self) -> None:
        lines = []
        for request_id in ("r1", "r2", "r3"):
            # two assistant lines per request, last one wins
            lines.append(
                _claude_assistant_line(
                    request_id,
                    "2026-05-01T03:00:00Z",
                    input_tokens=10,
                    cache_creation=0,
                    cache_read=0,
                    output_tokens=1,
                )
            )
            lines.append(
                _claude_assistant_line(
                    request_id,
                    "2026-05-01T03:00:01Z",
                    input_tokens=10,
                    cache_creation=0,
                    cache_read=0,
                    output_tokens=20,
                )
            )
        path = self._write_transcript("multi.jsonl", lines)
        events = parse_claude_token_events(path)
        self.assertEqual(len(events), 3)
        # each request's last line: 10 + 20 = 30
        self.assertEqual(sum(e.total_tokens for e in events), 90)


class ClaudeIngestIntegrationTests(unittest.TestCase):
    """Objective 2: merged totals, source tagging, and idempotent re-scan."""

    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.root = Path(self.temp_dir.name)
        self.codex_root = self.root / ".codex"
        self.session_dir = self.codex_root / "sessions" / "2026" / "04" / "03"
        self.session_dir.mkdir(parents=True, exist_ok=True)
        self.claude_root = self.root / ".claude"
        self.project_dir = self.claude_root / "projects" / "C--Agent-Repo"
        self.project_dir.mkdir(parents=True, exist_ok=True)
        self.db_path = self.root / "dashboard.db"
        self.connection = connect(self.db_path)
        initialize_db(self.connection)
        self.config = DashboardConfig(
            codex_root=str(self.codex_root),
            db_path=str(self.db_path),
            claude_root=str(self.claude_root),
        )

    def tearDown(self) -> None:
        self.connection.close()
        self.temp_dir.cleanup()

    def _seed(self) -> None:
        (self.session_dir / "rollout.jsonl").write_bytes(
            CODEX_TOKEN_EVENT_LINE + b"\n"
        )
        (self.project_dir / "claude.jsonl").write_bytes(
            _claude_assistant_line(
                "req-X",
                "2026-04-03T00:09:11Z",
                input_tokens=1000,
                cache_creation=0,
                cache_read=0,
                output_tokens=500,
            )
        )

    def test_merged_window_total_equals_codex_plus_claude(self) -> None:
        self._seed()
        ingest_once(self.connection, self.config)
        events = load_events_since(self.connection, datetime(2026, 1, 1, tzinfo=UTC))
        codex_total = sum(e.total_tokens for e in events if e.source == "codex")
        claude_total = sum(e.total_tokens for e in events if e.source == "claude")
        merged_total = sum(e.total_tokens for e in events)
        self.assertEqual(codex_total, 101368)  # Codex last_token_usage total
        self.assertEqual(claude_total, 1500)  # 1000 input + 500 output
        self.assertEqual(merged_total, codex_total + claude_total)

    def test_source_column_distinguishes_sources(self) -> None:
        self._seed()
        ingest_once(self.connection, self.config)
        events = load_events_since(self.connection, datetime(2026, 1, 1, tzinfo=UTC))
        sources = {e.source for e in events}
        self.assertEqual(sources, {"codex", "claude"})

    def test_re_ingest_is_idempotent(self) -> None:
        self._seed()
        ingest_once(self.connection, self.config)
        first_count = count_events(self.connection)
        ingest_once(self.connection, self.config)
        second_count = count_events(self.connection)
        self.assertEqual(first_count, second_count)
        self.assertEqual(first_count, 2)

    def test_claude_in_progress_request_updates_not_duplicates(self) -> None:
        # First scan sees an in-progress request; a later append carries the
        # request's final, larger usage. The row count must stay 1 (per
        # requestId) and the total must reflect the latest usage.
        path = self.project_dir / "streaming.jsonl"
        path.write_bytes(
            _claude_assistant_line(
                "req-S",
                "2026-04-03T05:00:00Z",
                input_tokens=100,
                cache_creation=0,
                cache_read=0,
                output_tokens=10,
            )
        )
        ingest_once(self.connection, self.config)
        with path.open("ab") as handle:
            handle.write(
                _claude_assistant_line(
                    "req-S",
                    "2026-04-03T05:00:05Z",
                    input_tokens=100,
                    cache_creation=0,
                    cache_read=0,
                    output_tokens=400,
                )
            )
        ingest_once(self.connection, self.config)
        events = load_events_since(self.connection, datetime(2026, 1, 1, tzinfo=UTC))
        claude_events = [e for e in events if e.source == "claude"]
        self.assertEqual(len(claude_events), 1)
        self.assertEqual(claude_events[0].total_tokens, 500)  # 100 + 400


class SourceFilterTests(unittest.TestCase):
    """Objective 4: source-filtered aggregation over the in-memory snapshot."""

    def _event(self, source: str, total: int, when: datetime) -> TokenEvent:
        return TokenEvent(
            session_path=f"{source}-path",
            line_offset=0,
            event_timestamp=when,
            total_tokens=total,
            input_tokens=total,
            cached_input_tokens=0,
            output_tokens=0,
            reasoning_output_tokens=0,
            cumulative_total_tokens=total,
            weekly_used_percent=None,
            weekly_window_minutes=None,
            weekly_resets_at=None,
            raw_json="{}",
            source=source,
            source_event_id=f"{source}-0",
        )

    def setUp(self) -> None:
        base = datetime(2026, 5, 1, 12, 0, tzinfo=UTC)
        self.events = [
            self._event("codex", 100, base),
            self._event("claude", 25, base + timedelta(seconds=1)),
            self._event("codex", 50, base + timedelta(seconds=2)),
            self._event("claude", 75, base + timedelta(seconds=3)),
        ]

    def test_known_sources_are_codex_and_claude(self) -> None:
        self.assertEqual(set(KNOWN_SOURCES), {"codex", "claude"})

    def test_none_selection_passes_all_events(self) -> None:
        filtered = filter_events_by_source(self.events, None)
        self.assertEqual(len(filtered), 4)

    def test_both_sources_equal_merged_total(self) -> None:
        filtered = filter_events_by_source(self.events, {"codex", "claude"})
        self.assertEqual(sum(e.total_tokens for e in filtered), 250)

    def test_codex_only_excludes_claude(self) -> None:
        filtered = filter_events_by_source(self.events, {"codex"})
        self.assertEqual(sum(e.total_tokens for e in filtered), 150)
        self.assertTrue(all(e.source == "codex" for e in filtered))

    def test_claude_only_excludes_codex(self) -> None:
        filtered = filter_events_by_source(self.events, {"claude"})
        self.assertEqual(sum(e.total_tokens for e in filtered), 100)
        self.assertTrue(all(e.source == "claude" for e in filtered))

    def test_empty_selection_yields_zero_state_not_crash(self) -> None:
        filtered = filter_events_by_source(self.events, set())
        self.assertEqual(filtered, [])

    def test_legacy_null_source_treated_as_codex(self) -> None:
        legacy = self._event("", 999, datetime(2026, 5, 1, 12, 0, tzinfo=UTC))
        filtered = filter_events_by_source([legacy], {"codex"})
        self.assertEqual(len(filtered), 1)

    def test_filtered_buckets_reflect_only_selected_source(self) -> None:
        # Aggregation over the filtered snapshot equals the selected-source sum.
        now = datetime(2026, 5, 1, 12, 1, tzinfo=UTC)
        codex_only = filter_events_by_source(self.events, {"codex"})
        buckets = build_buckets(codex_only, "15m", bucket_count=4, now=now)
        self.assertEqual(sum(b.total_tokens for b in buckets), 150)


class TimestampIndexTests(unittest.TestCase):
    """Objective 3: initialize_db creates the event_timestamp index."""

    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.db_path = Path(self.temp_dir.name) / "dashboard.db"

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def test_event_timestamp_index_exists_after_init(self) -> None:
        connection = connect(self.db_path)
        initialize_db(connection)
        rows = connection.execute(
            "SELECT name FROM sqlite_master WHERE type='index' "
            "AND tbl_name='token_events'"
        ).fetchall()
        index_names = {str(row["name"]) for row in rows}
        self.assertIn("idx_token_events_event_timestamp", index_names)
        connection.close()

    def test_event_timestamp_index_covers_window_query(self) -> None:
        connection = connect(self.db_path)
        initialize_db(connection)
        plan = connection.execute(
            "EXPLAIN QUERY PLAN SELECT * FROM token_events "
            "WHERE event_timestamp >= ? ORDER BY event_timestamp ASC",
            ("2026-01-01T00:00:00+00:00",),
        ).fetchall()
        plan_text = " ".join(str(tuple(row)) for row in plan)
        self.assertIn("idx_token_events_event_timestamp", plan_text)
        connection.close()


class SourceFilterRenderPathTests(unittest.TestCase):
    """Objective 4: _toggle_source re-renders from snapshot, no DB read."""

    def test_toggle_source_renders_from_snapshot_without_db_read(self) -> None:
        from types import SimpleNamespace
        from unittest import mock

        from app.codex_dashboard.ui import DashboardApp

        snapshot = ["snapshot-event"]
        markers = {"a": []}
        app = SimpleNamespace(
            selected_sources={"codex", "claude"},
            source_filter_vars={"claude": SimpleNamespace(get=lambda: False)},
            source_filter_button=None,
            latest_events=snapshot,
            latest_session_context_markers=markers,
            _render_dashboard=mock.Mock(),
            _refresh_source_filter_label=mock.Mock(),
            _load_dashboard_data=mock.Mock(),
            refresh_data=mock.Mock(),
        )
        DashboardApp._toggle_source(app, "claude")
        # Claude was unchecked, so it is removed from the selection.
        self.assertNotIn("claude", app.selected_sources)
        self.assertIn("codex", app.selected_sources)
        # Re-render from the in-memory snapshot, with no blocking DB read.
        app._render_dashboard.assert_called_once_with(snapshot, markers)
        app._load_dashboard_data.assert_not_called()
        app.refresh_data.assert_not_called()


class ActivationFixStorageTests(unittest.TestCase):
    """Activation-fix follow-up (Fix B): cheap background aggregation helpers.

    The background poll must keep the overlay fresh without re-materializing the
    whole 7-day window every cycle. These cover the indexed SQL helpers that make
    that cheap: a per-source 7-day SUM (no per-event scan) and an indexed latest
    advisory lookback.
    """

    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.db_path = Path(self.temp_dir.name) / "dashboard.db"
        self.connection = connect(self.db_path)
        initialize_db(self.connection)
        self._row_seq = 0

    def tearDown(self) -> None:
        self.connection.close()
        self.temp_dir.cleanup()

    def _insert(
        self,
        *,
        source: str,
        total: int,
        when: datetime,
        advisory: float | None = None,
        resets_at: int | None = None,
    ) -> None:
        self._row_seq += 1
        self.connection.execute(
            """
            INSERT INTO token_events(
                session_path, line_offset, event_timestamp, total_tokens,
                input_tokens, cached_input_tokens, output_tokens,
                reasoning_output_tokens, cumulative_total_tokens,
                weekly_used_percent, weekly_window_minutes, weekly_resets_at,
                raw_json, source, source_event_id
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                f"{source}-path",
                self._row_seq,
                when.isoformat(),
                total,
                total,
                0,
                0,
                0,
                total,
                advisory,
                10080 if advisory is not None else None,
                resets_at,
                "{}",
                source,
                f"{source}-{self._row_seq}",
            ),
        )
        self.connection.commit()

    def test_sum_by_source_excludes_events_before_since(self) -> None:
        from app.codex_dashboard.storage import sum_total_tokens_by_source_since

        now = datetime.now(UTC)
        # Inside the 7-day window.
        self._insert(source="codex", total=100, when=now - timedelta(days=1))
        self._insert(source="claude", total=40, when=now - timedelta(days=2))
        # Outside the 7-day window: must NOT be counted.
        self._insert(source="codex", total=999, when=now - timedelta(days=10))
        totals = sum_total_tokens_by_source_since(
            self.connection, now - timedelta(days=7)
        )
        self.assertEqual(totals.get("codex"), 100)
        self.assertEqual(totals.get("claude"), 40)

    def test_sum_by_source_lets_filter_subtract_a_source_in_memory(self) -> None:
        from app.codex_dashboard.storage import sum_total_tokens_by_source_since

        now = datetime.now(UTC)
        self._insert(source="codex", total=150, when=now - timedelta(hours=1))
        self._insert(source="claude", total=25, when=now - timedelta(hours=2))
        totals = sum_total_tokens_by_source_since(
            self.connection, now - timedelta(days=7)
        )
        # Merged (today's default) = sum of both.
        self.assertEqual(sum(totals.values()), 175)
        # Codex-only = drop claude's precomputed total, no DB re-read.
        codex_only = sum(v for s, v in totals.items() if s in {"codex"})
        self.assertEqual(codex_only, 150)

    def test_sum_by_source_uses_covering_index_range_scan(self) -> None:
        # Fix B: the 7-day total must be a covering-index RANGE scan over just the
        # 7-day slice, not a full-table scan. The production helper forces the
        # (event_timestamp, source, total_tokens) covering index because the
        # planner otherwise full-scans via the source index.
        now = datetime.now(UTC)
        self._insert(source="codex", total=5, when=now - timedelta(hours=1))
        plan = self.connection.execute(
            "EXPLAIN QUERY PLAN SELECT source, SUM(total_tokens) "
            "FROM token_events INDEXED BY idx_token_events_ts_source_total "
            "WHERE event_timestamp >= ? GROUP BY source",
            ((now - timedelta(days=7)).isoformat(),),
        ).fetchall()
        plan_text = " ".join(str(tuple(row)) for row in plan)
        self.assertIn("COVERING INDEX idx_token_events_ts_source_total", plan_text)
        self.assertIn("event_timestamp>", plan_text)  # a range scan, not full scan

    def test_latest_advisory_returns_most_recent_advisory_row(self) -> None:
        from app.codex_dashboard.storage import load_latest_weekly_advisory

        now = datetime.now(UTC)
        self._insert(
            source="codex", total=10, when=now - timedelta(hours=5), advisory=11.0
        )
        self._insert(
            source="codex", total=10, when=now - timedelta(hours=1), advisory=42.0
        )
        # A later non-advisory event must not shadow the advisory.
        self._insert(source="claude", total=10, when=now)
        advisory = load_latest_weekly_advisory(
            self.connection, now - timedelta(days=7)
        )
        self.assertIsNotNone(advisory)
        self.assertEqual(advisory.weekly_used_percent, 42.0)

    def test_latest_advisory_none_when_no_advisory_in_window(self) -> None:
        from app.codex_dashboard.storage import load_latest_weekly_advisory

        now = datetime.now(UTC)
        self._insert(source="claude", total=10, when=now - timedelta(hours=1))
        advisory = load_latest_weekly_advisory(
            self.connection, now - timedelta(days=7)
        )
        self.assertIsNone(advisory)


class ActivationFixRenderPathTests(unittest.TestCase):
    """Activation-fix follow-up: the render path uses precomputed 7-day totals.

    These prove the show/hide decoupling at the data layer without a real Tk
    window: the displayed 7-day total comes from the cheap per-source totals (no
    per-event 7-day scan in _render_dashboard), and the source filter excludes a
    source's precomputed total in memory.
    """

    def test_load_dashboard_data_loads_only_chart_window(self) -> None:
        # Fix B: the per-event load is bounded to the charted span, not 7 days,
        # so bucketing does not loop the whole window on a large DB.
        from types import SimpleNamespace
        from unittest import mock

        from app.codex_dashboard.ui import DashboardApp

        captured = {}

        def fake_load_events_since(_connection, since):
            captured["events_since"] = since
            return []

        def fake_sum(_connection, since):
            captured["sum_since"] = since
            return {"codex": 7}

        app = SimpleNamespace(
            config=SimpleNamespace(db_path=":memory:"),
            display_timezone=UTC,
            selected_interval="15m",  # 15m x 20 buckets = 5h chart window
            selected_chart_mode="velocity",
        )
        with mock.patch("app.codex_dashboard.ui.connect"), mock.patch(
            "app.codex_dashboard.ui.initialize_db"
        ), mock.patch(
            "app.codex_dashboard.ui.load_events_since",
            side_effect=fake_load_events_since,
        ), mock.patch(
            "app.codex_dashboard.ui.sum_total_tokens_by_source_since",
            side_effect=fake_sum,
        ), mock.patch(
            "app.codex_dashboard.ui.load_latest_weekly_advisory",
            return_value=None,
        ):
            events, markers, totals, advisory = DashboardApp._load_dashboard_data(app)

        self.assertEqual(totals, {"codex": 7})
        self.assertIsNone(advisory)
        # The per-event load window (chart) is much shorter than the 7-day SUM
        # window: the chart-window cutoff is LATER (more recent) than the 7-day
        # cutoff.
        self.assertGreater(captured["events_since"], captured["sum_since"])
        # Specifically, the chart window is the default 5 hours.
        window = captured["events_since"] - captured["sum_since"]
        self.assertAlmostEqual(
            window.total_seconds(),
            (timedelta(days=7) - timedelta(hours=5)).total_seconds(),
            delta=5,
        )

    def test_render_total_7d_from_precomputed_per_source_totals(self) -> None:
        from types import SimpleNamespace
        from unittest import mock

        from app.codex_dashboard.ui import DashboardApp

        configured = {}

        def make_label(key):
            return SimpleNamespace(
                configure=lambda **kw: configured.__setitem__(key, kw.get("text"))
            )

        app = SimpleNamespace(
            display_timezone=UTC,
            selected_interval="15m",
            selected_metric_mode="total",
            selected_chart_mode="velocity",
            selected_sources={"codex", "claude"},
            latest_events=[],
            latest_session_context_markers={},
            latest_repo_legend=[],
            latest_repo_totals=[],
            latest_source_totals_7d={},
            latest_weekly_advisory=None,
            config=SimpleNamespace(weekly_budget_tokens=4_000_000_000),
            local_total_value=make_label("total"),
            local_total_detail=make_label("total_detail"),
            projected_value=make_label("proj"),
            projected_detail=make_label("proj_detail"),
            headroom_value=make_label("head"),
            headroom_detail=make_label("head_detail"),
            advisory_label=make_label("advisory"),
            chart_header_title=make_label("title"),
            chart_header_context=make_label("context"),
            _refresh_status_surfaces=mock.Mock(),
            _timezone_label=mock.Mock(return_value="UTC"),
            draw_chart=mock.Mock(),
        )

        # Merged: both sources counted.
        DashboardApp._render_dashboard(
            app, [], {}, {"codex": 100, "claude": 40}, None
        )
        self.assertEqual(configured["total"], format_token_value(140))

        # Source filter excludes claude: 7d total drops by claude's precomputed
        # total, with NO database read (we pass nothing; it sums in memory).
        app.selected_sources = {"codex"}
        DashboardApp._render_dashboard(app, [], {})
        self.assertEqual(configured["total"], format_token_value(100))


if __name__ == "__main__":
    unittest.main()
