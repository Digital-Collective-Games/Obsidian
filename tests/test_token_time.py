from __future__ import annotations

import csv
import json
import tempfile
import unittest
from pathlib import Path

from app.codex_dashboard.token_time import (
    add_total_time_to_token_usage_csvs,
    insert_field_after,
    title_with_task_id,
    working_time_buckets_for_session,
)


def write_jsonl(path: Path, records: list[dict]) -> None:
    with path.open("w", encoding="utf-8", newline="\n") as handle:
        for record in records:
            handle.write(json.dumps(record) + "\n")


def write_csv(path: Path, fieldnames: list[str], rows: list[dict[str, str]]) -> None:
    with path.open("w", encoding="utf-8", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames, lineterminator="\n")
        writer.writeheader()
        writer.writerows(rows)


def read_csv(path: Path) -> tuple[list[str], list[dict[str, str]]]:
    with path.open("r", encoding="utf-8", newline="") as handle:
        reader = csv.DictReader(handle)
        return list(reader.fieldnames or []), list(reader)


class TokenTimeTests(unittest.TestCase):
    def test_working_time_buckets_count_chat_only(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            session_path = Path(temp_dir) / "session.jsonl"
            write_jsonl(
                session_path,
                [
                    {
                        "timestamp": "2026-04-26T12:01:00Z",
                        "type": "event_msg",
                        "payload": {"type": "user_message", "message": "start"},
                    },
                    {
                        "timestamp": "2026-04-26T12:09:59Z",
                        "type": "event_msg",
                        "payload": {"type": "agent_message", "message": "same bucket"},
                    },
                    {
                        "timestamp": "2026-04-26T12:10:00Z",
                        "type": "response_item",
                        "payload": {"type": "message", "role": "assistant"},
                    },
                    {
                        "timestamp": "2026-04-26T12:20:00Z",
                        "type": "response_item",
                        "payload": {"type": "function_call"},
                    },
                ],
            )

            self.assertEqual(
                working_time_buckets_for_session(session_path),
                {"2026-04-26T12:00:00Z", "2026-04-26T12:10:00Z"},
            )

    def test_add_total_time_to_csvs_deduplicates_aggregate_buckets(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            session_one = root / "one.jsonl"
            session_two = root / "two.jsonl"
            write_jsonl(
                session_one,
                [
                    {
                        "timestamp": "2026-04-26T12:01:00Z",
                        "type": "event_msg",
                        "payload": {"type": "user_message"},
                    },
                    {
                        "timestamp": "2026-04-26T12:11:00Z",
                        "type": "event_msg",
                        "payload": {"type": "agent_message"},
                    },
                ],
            )
            write_jsonl(
                session_two,
                [
                    {
                        "timestamp": "2026-04-26T12:02:00Z",
                        "type": "event_msg",
                        "payload": {"type": "agent_message"},
                    }
                ],
            )

            sessions_csv = root / "sessions.csv"
            aggregate_csv = root / "aggregate.csv"
            write_csv(
                sessions_csv,
                ["repo", "task_id", "session_path", "total_tokens", "token_event_count"],
                [
                    {
                        "repo": r"C:\Agent\CodexDashboard",
                        "task_id": "Task-0008",
                        "session_path": str(session_one),
                        "total_tokens": "100",
                        "token_event_count": "2",
                    },
                    {
                        "repo": r"c:\agent\codexdashboard",
                        "task_id": "Task-0008",
                        "session_path": str(session_two),
                        "total_tokens": "50",
                        "token_event_count": "1",
                    },
                ],
            )
            write_csv(
                aggregate_csv,
                ["repo", "task_id", "title", "total_tokens"],
                [
                    {
                        "repo": r"C:\Agent\CodexDashboard",
                        "task_id": "Task-0008",
                        "title": "Build the backend task dispatch layer",
                        "total_tokens": "150",
                    }
                ],
            )

            summary = add_total_time_to_token_usage_csvs(aggregate_csv, sessions_csv)

            aggregate_fieldnames, aggregate_rows = read_csv(aggregate_csv)
            session_fieldnames, session_rows = read_csv(sessions_csv)
            self.assertEqual(summary["aggregate_rows"], 1)
            self.assertEqual(summary["session_rows"], 2)
            self.assertEqual(summary["missing_session_files"], 0)
            self.assertEqual(insert_field_after(["total_tokens"], "total_time", "total_tokens"), ["total_tokens", "total_time"])
            self.assertEqual(aggregate_fieldnames, ["repo", "task_id", "title", "total_tokens", "total_time"])
            self.assertEqual(aggregate_rows[0]["total_time"], "20")
            self.assertEqual(
                aggregate_rows[0]["title"],
                "TASK-0008: Build the backend task dispatch layer",
            )
            self.assertEqual(session_fieldnames[4], "total_time")
            self.assertEqual([row["total_time"] for row in session_rows], ["20", "10"])

    def test_title_with_task_id_skips_unattributed_and_existing_prefix(self) -> None:
        self.assertEqual(
            title_with_task_id({"task_id": "Task-0001", "title": "First task"}),
            "TASK-0001: First task",
        )
        self.assertEqual(
            title_with_task_id({"task_id": "Task-0001", "title": "TASK-0001: First task"}),
            "TASK-0001: First task",
        )
        self.assertEqual(
            title_with_task_id({"task_id": "Unattributed", "title": "Unattributed"}),
            "Unattributed",
        )


if __name__ == "__main__":
    unittest.main()
