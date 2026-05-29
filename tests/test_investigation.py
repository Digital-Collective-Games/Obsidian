from __future__ import annotations

import tempfile
import unittest
from datetime import UTC, datetime, timedelta, timezone
from pathlib import Path

from app.codex_dashboard.investigation import (
    build_bucket_investigation,
    build_codex_launch_command,
    report_path_for_brief,
    write_bucket_investigation,
)
from app.codex_dashboard.models import AggregationBucket, SessionContextMarker, TokenEvent
from app.codex_dashboard.paths import default_investigations_path


class InvestigationTests(unittest.TestCase):
    def test_build_bucket_investigation_recovers_repo_and_session_signals(self) -> None:
        est = timezone(timedelta(hours=-5), name="EST")
        bucket = AggregationBucket(
            start_at=datetime(2026, 4, 3, 20, 0, tzinfo=est),
            end_at=datetime(2026, 4, 3, 20, 15, tzinfo=est),
            total_tokens=450,
        )

        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            repo_alpha = root / "RepoAlpha"
            repo_beta = root / "RepoBeta"
            (repo_alpha / ".git").mkdir(parents=True, exist_ok=True)
            (repo_beta / ".git").mkdir(parents=True, exist_ok=True)
            session_a = root / "session-a.jsonl"
            session_b = root / "session-b.jsonl"
            session_a.write_text(
                "\n".join(
                    [
                        (
                            '{"timestamp":"2026-04-04T01:00:00+00:00","type":"session_meta",'
                            f'"payload":{{"cwd":"{repo_alpha.as_posix()}","agent_nickname":"Singer","agent_role":"worker"}}}}'
                        ),
                        (
                            '{"timestamp":"2026-04-04T01:03:00+00:00","type":"event_msg",'
                            '"payload":{"type":"agent_message","message":"Investigating widget bug in Crystallize task state."}}'
                        ),
                        (
                            '{"timestamp":"2026-04-04T01:04:00+00:00","type":"response_item",'
                            '"payload":{"type":"function_call","name":"shell_command"}}'
                        ),
                        (
                            '{"timestamp":"2026-04-04T01:05:00+00:00","type":"event_msg",'
                            '"payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":270,'
                            '"cached_input_tokens":240,"output_tokens":15,"reasoning_output_tokens":6,"total_tokens":300}}}}'
                        ),
                    ]
                )
                + "\n",
                encoding="utf-8",
            )
            session_b.write_text(
                "\n".join(
                    [
                        (
                            '{"timestamp":"2026-04-04T01:00:00+00:00","type":"session_meta",'
                            f'"payload":{{"cwd":"{repo_beta.as_posix()}","agent_nickname":"Volta","agent_role":"worker"}}}}'
                        ),
                        (
                            '{"timestamp":"2026-04-04T01:06:00+00:00","type":"event_msg",'
                            '"payload":{"type":"agent_message","message":"Reviewing the sidebar regression."}}'
                        ),
                        (
                            '{"timestamp":"2026-04-04T01:07:00+00:00","type":"response_item",'
                            '"payload":{"type":"function_call","name":"view_image"}}'
                        ),
                        (
                            '{"timestamp":"2026-04-04T01:07:30+00:00","type":"event_msg",'
                            '"payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":135,'
                            '"cached_input_tokens":120,"output_tokens":9,"reasoning_output_tokens":4,"total_tokens":150}}}}'
                        ),
                    ]
                )
                + "\n",
                encoding="utf-8",
            )

            events = [
                TokenEvent(
                    session_path=str(session_a),
                    line_offset=5,
                    event_timestamp=datetime(2026, 4, 4, 1, 5, tzinfo=UTC),
                    total_tokens=300,
                    input_tokens=270,
                    cached_input_tokens=240,
                    output_tokens=15,
                    reasoning_output_tokens=6,
                    cumulative_total_tokens=300,
                    weekly_used_percent=None,
                    weekly_window_minutes=None,
                    weekly_resets_at=None,
                    raw_json="{}",
                ),
                TokenEvent(
                    session_path=str(session_b),
                    line_offset=6,
                    event_timestamp=datetime(2026, 4, 4, 1, 7, tzinfo=UTC),
                    total_tokens=150,
                    input_tokens=135,
                    cached_input_tokens=120,
                    output_tokens=9,
                    reasoning_output_tokens=4,
                    cumulative_total_tokens=450,
                    weekly_used_percent=None,
                    weekly_window_minutes=None,
                    weekly_resets_at=None,
                    raw_json="{}",
                ),
            ]

            investigation = build_bucket_investigation(
                bucket,
                events,
                {},
                "15m",
                "repo",
                Path("C:/Users/gregs/.codex"),
            )

            self.assertIn("Bucket: 2026-04-03 08:00 PM EST to 08:15 PM EST", investigation.markdown)
            self.assertIn("## Bucket Composition", investigation.markdown)
            self.assertIn("- RepoAlpha: 300", investigation.markdown)
            self.assertIn("- RepoBeta: 150", investigation.markdown)
            self.assertIn("### session-a.jsonl | Singer worker", investigation.markdown)
            self.assertIn("Commentary: Investigating widget bug in Crystallize task state.", investigation.markdown)
            self.assertIn("Tool calls: shell_command", investigation.markdown)
            self.assertIn("session-a.jsonl | 300", investigation.markdown)
            self.assertIn("Workspace: ", investigation.markdown)
            self.assertEqual(investigation.workspace_root, Path("C:/Users/gregs/.codex"))

    def test_build_codex_launch_command_writes_report_via_exec(self) -> None:
        command = build_codex_launch_command(
            "C:/Tools/codex.exe",
            Path("C:/Agent/CodexDashboard/Tracking/Investigations/bucket-investigation-20260403-000000-brief.md"),
            Path("C:/Agent/CodexDashboard/Tracking/Investigations/bucket-investigation-20260403-000000-report.md"),
            Path("C:/Users/gregs/.codex"),
            [Path("C:/Agent/RepoAlpha"), Path("C:/Users/gregs/.codex")],
        )

        self.assertEqual(command[:3], ["powershell.exe", "-NoExit", "-Command"])
        self.assertIn("'C:/Tools/codex.exe' exec", command[3])
        self.assertIn("--dangerously-bypass-approvals-and-sandbox", command[3])
        self.assertIn("--skip-git-repo-check", command[3])
        self.assertIn("-o 'C:\\Agent\\CodexDashboard\\Tracking\\Investigations\\bucket-investigation-20260403-000000-report.md'", command[3])
        self.assertIn("--add-dir 'C:\\Agent\\RepoAlpha'", command[3])
        self.assertNotIn("--add-dir 'C:/Users/gregs/.codex' --add-dir 'C:/Users/gregs/.codex'", command[3])
        self.assertIn("Read the bucket investigation brief at C:\\Agent\\CodexDashboard\\Tracking\\Investigations\\bucket-investigation-20260403-000000-brief.md", command[3])
        self.assertIn("This is a root-cause analysis task, not a summary task.", command[3])
        self.assertIn("Root cause: <one sentence>", command[3])
        self.assertIn("sections titled Trigger, Mechanism, Evidence, Avoidance, and Confidence", command[3])
        self.assertIn("Investigation report:", command[3])

    def test_default_investigations_path_lives_in_repo_tracking(self) -> None:
        self.assertEqual(
            default_investigations_path(),
            Path("C:/Agent/CodexDashboard/Tracking/Investigations"),
        )

    def test_report_path_for_brief_uses_report_suffix(self) -> None:
        self.assertEqual(
            report_path_for_brief(
                Path("C:/Agent/CodexDashboard/Tracking/Investigations/bucket-investigation-20260403-000000-brief.md")
            ),
            Path("C:/Agent/CodexDashboard/Tracking/Investigations/bucket-investigation-20260403-000000-report.md"),
        )

    def test_write_bucket_investigation_creates_brief_markdown_file(self) -> None:
        investigation = build_bucket_investigation(
            AggregationBucket(
                start_at=datetime(2026, 4, 3, 20, 0, tzinfo=UTC),
                end_at=datetime(2026, 4, 3, 20, 15, tzinfo=UTC),
                total_tokens=0,
            ),
            [],
            {
                "session-a": [
                    SessionContextMarker(
                        "session-a",
                        0,
                        "C:/RepoAlpha",
                        "C:/RepoAlpha",
                        "RepoAlpha",
                        "repo",
                    )
                ]
            },
            "15m",
            "velocity",
            Path("C:/Users/gregs/.codex"),
        )
        with tempfile.TemporaryDirectory() as temp_dir:
            file_path = write_bucket_investigation(
                investigation,
                Path(temp_dir),
                datetime(2026, 4, 3, 20, 15),
            )

            self.assertTrue(file_path.exists())
            self.assertTrue(file_path.name.endswith("-brief.md"))
            self.assertIn("Obsidian Bucket Investigation", file_path.read_text(encoding="utf-8"))


if __name__ == "__main__":
    unittest.main()
