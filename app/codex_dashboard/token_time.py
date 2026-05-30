from __future__ import annotations

import csv
import json
from collections.abc import Iterable
from datetime import datetime, timezone
from pathlib import Path


WORKING_TIME_BUCKET_MINUTES = 10
_WORKING_TIME_BUCKET_SECONDS = WORKING_TIME_BUCKET_MINUTES * 60
CHAT_EVENT_TYPES = {"user_message", "agent_message"}
CHAT_RESPONSE_ROLES = {"user", "assistant"}


def parse_timestamp(raw_value: str) -> datetime:
    normalized = raw_value.replace("Z", "+00:00")
    value = datetime.fromisoformat(normalized)
    if value.tzinfo is None:
        return value.replace(tzinfo=timezone.utc)
    return value.astimezone(timezone.utc)


def working_time_bucket(timestamp: datetime) -> str:
    utc_timestamp = timestamp.astimezone(timezone.utc)
    bucket_epoch = int(utc_timestamp.timestamp()) // _WORKING_TIME_BUCKET_SECONDS
    bucket_start = datetime.fromtimestamp(
        bucket_epoch * _WORKING_TIME_BUCKET_SECONDS,
        tz=timezone.utc,
    )
    return bucket_start.isoformat().replace("+00:00", "Z")


def is_chat_record(record: dict) -> bool:
    payload = record.get("payload")
    if not isinstance(payload, dict):
        return False
    record_type = record.get("type")
    payload_type = payload.get("type")
    if record_type == "event_msg" and payload_type in CHAT_EVENT_TYPES:
        return True
    if (
        record_type == "response_item"
        and payload_type == "message"
        and payload.get("role") in CHAT_RESPONSE_ROLES
    ):
        return True
    return False


def working_time_buckets_for_session(session_path: Path) -> set[str]:
    buckets: set[str] = set()
    with session_path.open("rb") as handle:
        for raw_line in handle:
            stripped = raw_line.strip()
            if not stripped:
                continue
            try:
                record = json.loads(stripped)
            except (json.JSONDecodeError, UnicodeDecodeError):
                continue
            if not isinstance(record, dict) or not is_chat_record(record):
                continue
            raw_timestamp = record.get("timestamp")
            if not raw_timestamp:
                continue
            try:
                buckets.add(working_time_bucket(parse_timestamp(str(raw_timestamp))))
            except ValueError:
                continue
    return buckets


def total_time_minutes_from_buckets(buckets: Iterable[str]) -> int:
    return len(set(buckets)) * WORKING_TIME_BUCKET_MINUTES


def normalized_repo_key(raw_value: str | None) -> str:
    value = (raw_value or "").strip().replace("/", "\\")
    while value.endswith("\\"):
        value = value[:-1]
    return value.lower()


def task_row_key(row: dict[str, str]) -> tuple[str, str]:
    return normalized_repo_key(row.get("repo")), (row.get("task_id") or "").strip()


def insert_field_after(
    fieldnames: list[str],
    fieldname: str,
    after_fieldname: str,
) -> list[str]:
    existing = [name for name in fieldnames if name != fieldname]
    if after_fieldname in existing:
        index = existing.index(after_fieldname) + 1
        return existing[:index] + [fieldname] + existing[index:]
    return existing + [fieldname]


def title_with_task_id(row: dict[str, str]) -> str:
    task_id = (row.get("task_id") or "").strip()
    title = (row.get("title") or "").strip()
    if not task_id.lower().startswith("task-"):
        return title
    title_prefix = f"{task_id.upper()}: "
    if title.upper().startswith(title_prefix):
        return title
    return f"{title_prefix}{title}" if title else task_id.upper()


def load_csv_rows(path: Path) -> tuple[list[str], list[dict[str, str]]]:
    with path.open("r", newline="", encoding="utf-8-sig") as handle:
        reader = csv.DictReader(handle)
        fieldnames = list(reader.fieldnames or [])
        return fieldnames, list(reader)


def write_csv_rows(
    path: Path,
    fieldnames: list[str],
    rows: Iterable[dict[str, str]],
) -> None:
    with path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames, lineterminator="\n")
        writer.writeheader()
        for row in rows:
            writer.writerow(row)


def add_total_time_to_token_usage_csvs(
    aggregate_csv: Path,
    sessions_csv: Path,
) -> dict[str, int]:
    session_fieldnames, session_rows = load_csv_rows(sessions_csv)
    aggregate_fieldnames, aggregate_rows = load_csv_rows(aggregate_csv)

    aggregate_buckets: dict[tuple[str, str], set[str]] = {}
    missing_session_files = 0

    for row in session_rows:
        raw_session_path = row.get("session_path") or ""
        session_path = Path(raw_session_path)
        if not raw_session_path or not session_path.exists():
            buckets: set[str] = set()
            missing_session_files += 1
        else:
            buckets = working_time_buckets_for_session(session_path)

        row["total_time"] = str(total_time_minutes_from_buckets(buckets))
        aggregate_buckets.setdefault(task_row_key(row), set()).update(buckets)

    for row in aggregate_rows:
        row["total_time"] = str(
            total_time_minutes_from_buckets(aggregate_buckets.get(task_row_key(row), set()))
        )
        if "title" in row:
            row["title"] = title_with_task_id(row)

    session_fieldnames = insert_field_after(
        session_fieldnames,
        "total_time",
        "total_tokens",
    )
    aggregate_fieldnames = insert_field_after(
        aggregate_fieldnames,
        "total_time",
        "total_tokens",
    )

    write_csv_rows(sessions_csv, session_fieldnames, session_rows)
    write_csv_rows(aggregate_csv, aggregate_fieldnames, aggregate_rows)

    return {
        "aggregate_rows": len(aggregate_rows),
        "session_rows": len(session_rows),
        "missing_session_files": missing_session_files,
    }
