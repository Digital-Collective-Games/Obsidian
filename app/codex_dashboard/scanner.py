from __future__ import annotations

import json
from datetime import datetime
from pathlib import Path

from .attribution import UNKNOWN_PROJECT_KEY, UNKNOWN_PROJECT_LABEL, resolve_project_identity
from .config import DashboardConfig
from .models import IngestRunSummary, SessionContextMarker, TokenEvent
from .storage import (
    count_session_context_markers,
    initialize_db,
    insert_event,
    insert_session_context_marker,
    load_cursor,
    save_cursor,
    upsert_claude_event,
)


def session_jsonl_files(codex_root: Path) -> list[Path]:
    sessions_root = codex_root / "sessions"
    if not sessions_root.exists():
        return []
    return sorted(sessions_root.rglob("*.jsonl"))


def claude_jsonl_files(claude_root: Path) -> list[Path]:
    # Task-0013 Objective 2: Claude Code per-message token usage lives in
    # ~/.claude/projects/<encoded-cwd>/*.jsonl, not under a sessions/ tree.
    projects_root = claude_root / "projects"
    if not projects_root.exists():
        return []
    return sorted(projects_root.rglob("*.jsonl"))


def _parse_timestamp(raw_value: str) -> datetime:
    normalized = raw_value.replace("Z", "+00:00")
    return datetime.fromisoformat(normalized)


def parse_token_event(
    session_path: str,
    line_offset: int,
    raw_line: bytes,
) -> TokenEvent | None:
    payload = json.loads(raw_line.decode("utf-8"))
    if payload.get("type") != "event_msg":
        return None
    event_payload = payload.get("payload", {})
    if not isinstance(event_payload, dict):
        return None
    if event_payload.get("type") != "token_count":
        return None
    info = event_payload.get("info") or {}
    if not isinstance(info, dict):
        return None
    last_token_usage = info.get("last_token_usage") or {}
    total_token_usage = info.get("total_token_usage") or {}
    if not isinstance(last_token_usage, dict) or not isinstance(total_token_usage, dict):
        return None
    if "total_tokens" not in last_token_usage:
        return None
    rate_limits = event_payload.get("rate_limits") or {}
    if not isinstance(rate_limits, dict):
        rate_limits = {}
    secondary = rate_limits.get("secondary") or {}
    if not isinstance(secondary, dict):
        secondary = {}
    return TokenEvent(
        session_path=session_path,
        line_offset=line_offset,
        event_timestamp=_parse_timestamp(str(payload["timestamp"])),
        total_tokens=int(last_token_usage.get("total_tokens", 0)),
        input_tokens=int(last_token_usage.get("input_tokens", 0)),
        cached_input_tokens=int(last_token_usage.get("cached_input_tokens", 0)),
        output_tokens=int(last_token_usage.get("output_tokens", 0)),
        reasoning_output_tokens=int(last_token_usage.get("reasoning_output_tokens", 0)),
        cumulative_total_tokens=int(total_token_usage.get("total_tokens", 0)),
        weekly_used_percent=(
            float(secondary["used_percent"]) if "used_percent" in secondary else None
        ),
        weekly_window_minutes=(
            int(secondary["window_minutes"]) if "window_minutes" in secondary else None
        ),
        weekly_resets_at=(
            int(secondary["resets_at"]) if "resets_at" in secondary else None
        ),
        raw_json=raw_line.decode("utf-8").rstrip("\n"),
        source="codex",
        source_event_id=f"{session_path}:{line_offset}",
    )


def _claude_total_tokens(usage: dict) -> int:
    # Task-0013 Objective 2 canonical Claude total formula:
    # cache_creation_input_tokens is real, billed input cost, so it counts as
    # input. There is no single total_tokens field and no reasoning output.
    return (
        int(usage.get("input_tokens", 0) or 0)
        + int(usage.get("cache_creation_input_tokens", 0) or 0)
        + int(usage.get("cache_read_input_tokens", 0) or 0)
        + int(usage.get("output_tokens", 0) or 0)
    )


def parse_claude_token_events(file_path: Path) -> list[TokenEvent]:
    """Parse one Claude transcript into de-duplicated per-request TokenEvents.

    Claude streams multiple `type:"assistant"` lines per request whose `usage`
    blocks overlap (confirmed: one transcript had 219 assistant lines across 56
    distinct requestIds). Naive per-line summation over-counts severely. This
    records exactly one usage record per `requestId`, using the usage from the
    LAST assistant event seen for that requestId (the final cumulative block),
    keyed for idempotency by `source_event_id = requestId`.
    """
    session_path = str(file_path)
    # requestId -> (line_offset, timestamp, usage) for the latest assistant line.
    latest_by_request: dict[str, tuple[int, str, dict]] = {}
    order: list[str] = []
    line_offset = 0
    with file_path.open("rb") as handle:
        for raw_line in handle:
            current_offset = line_offset
            line_offset += len(raw_line)
            stripped = raw_line.strip()
            if not stripped:
                continue
            try:
                payload = json.loads(stripped.decode("utf-8"))
            except (json.JSONDecodeError, UnicodeDecodeError, ValueError):
                continue
            if not isinstance(payload, dict) or payload.get("type") != "assistant":
                continue
            request_id = payload.get("requestId")
            if not request_id:
                continue
            message = payload.get("message") or {}
            if not isinstance(message, dict):
                continue
            usage = message.get("usage") or {}
            if not isinstance(usage, dict) or not usage:
                continue
            timestamp = payload.get("timestamp")
            if not timestamp:
                continue
            request_id = str(request_id)
            if request_id not in latest_by_request:
                order.append(request_id)
            # Keep the LAST assistant event for each requestId.
            latest_by_request[request_id] = (current_offset, str(timestamp), usage)

    events: list[TokenEvent] = []
    for request_id in order:
        offset, timestamp, usage = latest_by_request[request_id]
        input_tokens = int(usage.get("input_tokens", 0) or 0)
        cache_creation = int(usage.get("cache_creation_input_tokens", 0) or 0)
        cache_read = int(usage.get("cache_read_input_tokens", 0) or 0)
        output_tokens = int(usage.get("output_tokens", 0) or 0)
        total_tokens = input_tokens + cache_creation + cache_read + output_tokens
        events.append(
            TokenEvent(
                session_path=session_path,
                line_offset=offset,
                event_timestamp=_parse_timestamp(timestamp),
                total_tokens=total_tokens,
                input_tokens=input_tokens + cache_creation,
                cached_input_tokens=cache_read,
                output_tokens=output_tokens,
                reasoning_output_tokens=0,
                cumulative_total_tokens=total_tokens,
                weekly_used_percent=None,
                weekly_window_minutes=None,
                weekly_resets_at=None,
                raw_json=json.dumps({"requestId": request_id, "usage": usage}),
                source="claude",
                source_event_id=request_id,
            )
        )
    return events


def parse_session_context_marker(
    session_path: str,
    line_offset: int,
    raw_line: bytes,
) -> SessionContextMarker | None:
    payload = json.loads(raw_line.decode("utf-8"))
    payload_type = payload.get("type")
    if payload_type not in {"session_meta", "turn_context"}:
        return None
    payload_body = payload.get("payload", {})
    if not isinstance(payload_body, dict):
        return None
    raw_cwd = payload_body.get("cwd")
    cwd, project_label, project_source = resolve_project_identity(
        str(raw_cwd) if raw_cwd is not None else None
    )
    project_key = cwd or UNKNOWN_PROJECT_KEY
    return SessionContextMarker(
        session_path=session_path,
        line_offset=line_offset,
        cwd=cwd,
        project_key=project_key,
        project_label=project_label,
        project_source=project_source,
    )


def backfill_session_context_markers(connection, file_path: Path, session_path: str) -> None:
    if count_session_context_markers(connection, session_path) > 0:
        return
    line_offset = 0
    inserted_any = False
    with file_path.open("rb") as handle:
        for raw_line in handle:
            stripped_line = raw_line.strip()
            if stripped_line:
                try:
                    marker = parse_session_context_marker(session_path, line_offset, stripped_line)
                except (json.JSONDecodeError, UnicodeDecodeError, ValueError):
                    marker = None
                if marker is not None:
                    insert_session_context_marker(connection, marker)
                    inserted_any = True
            line_offset += len(raw_line)
    if not inserted_any:
        insert_session_context_marker(
            connection,
            SessionContextMarker(
                session_path=session_path,
                line_offset=0,
                cwd=None,
                project_key=UNKNOWN_PROJECT_KEY,
                project_label=UNKNOWN_PROJECT_LABEL,
                project_source="unknown",
            ),
        )


def ingest_once(connection, config: DashboardConfig) -> IngestRunSummary:
    initialize_db(connection)
    files_scanned = 0
    files_updated = 0
    events_ingested = 0
    for file_path in session_jsonl_files(Path(config.codex_root)):
        files_scanned += 1
        session_path = str(file_path)
        stat = file_path.stat()
        backfill_session_context_markers(connection, file_path, session_path)
        last_offset, last_size = load_cursor(connection, session_path)
        if stat.st_size == last_size and stat.st_size == last_offset:
            continue
        if stat.st_size < last_offset:
            last_offset = 0
        with file_path.open("rb") as handle:
            handle.seek(last_offset)
            payload = handle.read()
        if not payload:
            save_cursor(connection, session_path, last_offset, stat.st_size)
            continue
        cursor_offset = last_offset
        complete_lines = payload.splitlines(keepends=True)
        for line in complete_lines:
            if not line.endswith(b"\n"):
                break
            line_offset = cursor_offset
            cursor_offset += len(line)
            stripped_line = line.strip()
            if not stripped_line:
                continue
            try:
                marker = parse_session_context_marker(session_path, line_offset, stripped_line)
            except (json.JSONDecodeError, UnicodeDecodeError, ValueError):
                marker = None
            if marker is not None:
                insert_session_context_marker(connection, marker)
            try:
                event = parse_token_event(session_path, line_offset, stripped_line)
            except (json.JSONDecodeError, UnicodeDecodeError, ValueError):
                continue
            if event is None:
                continue
            if insert_event(connection, event):
                events_ingested += 1
        if cursor_offset != last_offset or stat.st_size != last_size:
            files_updated += 1
        save_cursor(connection, session_path, cursor_offset, stat.st_size)

    # Task-0013 Objective 2: ingest Claude Code transcripts into the same
    # token_events pool, de-duplicated per requestId. Claude files are parsed
    # whole (not by byte tail) on size change because a request's final usage
    # block can arrive in a later append; the upsert keyed on
    # (source='claude', source_event_id=requestId) keeps exactly one row per
    # request and lets a later re-parse update an in-progress request's total.
    claude_root = (config.claude_root or "").strip()
    if claude_root:
        for file_path in claude_jsonl_files(Path(claude_root)):
            files_scanned += 1
            session_path = str(file_path)
            stat = file_path.stat()
            last_offset, last_size = load_cursor(connection, session_path)
            if stat.st_size == last_size:
                continue
            try:
                claude_events = parse_claude_token_events(file_path)
            except OSError:
                continue
            file_changed = False
            for event in claude_events:
                if upsert_claude_event(connection, event):
                    events_ingested += 1
                file_changed = True
            if file_changed:
                files_updated += 1
            # Cursor offset is unused for Claude (we always re-parse whole files
            # on size change); store size as the change sentinel.
            save_cursor(connection, session_path, stat.st_size, stat.st_size)

    connection.commit()
    return IngestRunSummary(
        files_scanned=files_scanned,
        files_updated=files_updated,
        events_ingested=events_ingested,
    )
