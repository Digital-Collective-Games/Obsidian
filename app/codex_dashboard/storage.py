from __future__ import annotations

import sqlite3
from datetime import datetime
from pathlib import Path

from .models import SessionContextMarker, TokenEvent


def connect(db_path: Path) -> sqlite3.Connection:
    db_path.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(db_path)
    connection.row_factory = sqlite3.Row
    return connection


def initialize_db(connection: sqlite3.Connection) -> None:
    connection.executescript(
        """
        PRAGMA journal_mode=WAL;

        CREATE TABLE IF NOT EXISTS file_cursors (
            path TEXT PRIMARY KEY,
            last_offset INTEGER NOT NULL,
            last_size INTEGER NOT NULL,
            updated_at TEXT NOT NULL
        );

        CREATE TABLE IF NOT EXISTS token_events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            session_path TEXT NOT NULL,
            line_offset INTEGER NOT NULL,
            event_timestamp TEXT NOT NULL,
            total_tokens INTEGER NOT NULL,
            input_tokens INTEGER NOT NULL,
            cached_input_tokens INTEGER NOT NULL,
            output_tokens INTEGER NOT NULL,
            reasoning_output_tokens INTEGER NOT NULL,
            cumulative_total_tokens INTEGER NOT NULL,
            weekly_used_percent REAL,
            weekly_window_minutes INTEGER,
            weekly_resets_at INTEGER,
            raw_json TEXT NOT NULL,
            source TEXT NOT NULL DEFAULT 'codex',
            source_event_id TEXT NOT NULL DEFAULT '',
            UNIQUE(session_path, line_offset)
        );

        CREATE TABLE IF NOT EXISTS session_context_markers (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            session_path TEXT NOT NULL,
            line_offset INTEGER NOT NULL,
            cwd TEXT,
            project_key TEXT NOT NULL,
            project_label TEXT NOT NULL,
            project_source TEXT NOT NULL,
            UNIQUE(session_path, line_offset)
        );
        """
    )
    _migrate_token_events_source_columns(connection)
    # Task-0013 Objective 3: index the activation-time window query
    # (load_events_since: WHERE event_timestamp >= ? ORDER BY event_timestamp ASC)
    # so it stops being a full-table scan that grows with DB size. Idempotent.
    connection.executescript(
        """
        CREATE INDEX IF NOT EXISTS idx_token_events_event_timestamp
            ON token_events(event_timestamp);
        CREATE UNIQUE INDEX IF NOT EXISTS idx_token_events_source_event
            ON token_events(source, source_event_id);
        -- Task-0013 activation-fix follow-up (Fix B): the background poll fetches
        -- the latest weekly advisory with an indexed lookback instead of scanning
        -- the in-memory window. Partial index keeps it tiny (advisory rows only).
        CREATE INDEX IF NOT EXISTS idx_token_events_advisory_ts
            ON token_events(event_timestamp)
            WHERE weekly_used_percent IS NOT NULL;
        -- Task-0013 activation-fix follow-up (Fix B): covering index for the cheap
        -- rolling per-source 7-day SUM. With (event_timestamp, source,
        -- total_tokens) SQLite can satisfy
        --   WHERE event_timestamp >= ? GROUP BY source -> SUM(total_tokens)
        -- as an index-only range scan (only the ~7-day slice, not the whole
        -- table), so background freshness stays cheap as the DB grows.
        CREATE INDEX IF NOT EXISTS idx_token_events_ts_source_total
            ON token_events(event_timestamp, source, total_tokens);
        """
    )
    connection.commit()


def _migrate_token_events_source_columns(connection: sqlite3.Connection) -> None:
    """Additively migrate pre-Task-0013 token_events rows.

    Older databases created the table without the `source` / `source_event_id`
    columns. Add them when missing and backfill legacy Codex rows so the
    per-source idempotency key (UNIQUE(source, source_event_id)) is satisfiable
    without orphaning or rewriting existing data.
    """
    existing_columns = {
        str(row["name"])
        for row in connection.execute("PRAGMA table_info(token_events)").fetchall()
    }
    if "source" not in existing_columns:
        connection.execute(
            "ALTER TABLE token_events ADD COLUMN source TEXT NOT NULL DEFAULT 'codex'"
        )
    if "source_event_id" not in existing_columns:
        connection.execute(
            "ALTER TABLE token_events ADD COLUMN source_event_id TEXT NOT NULL DEFAULT ''"
        )
        # Backfill the legacy Codex identity so the unique index below is
        # consistent for rows ingested before this column existed.
        connection.execute(
            """
            UPDATE token_events
            SET source_event_id = session_path || ':' || line_offset
            WHERE source_event_id = ''
            """
        )


def load_cursor(connection: sqlite3.Connection, path: str) -> tuple[int, int]:
    row = connection.execute(
        "SELECT last_offset, last_size FROM file_cursors WHERE path = ?",
        (path,),
    ).fetchone()
    if row is None:
        return 0, 0
    return int(row["last_offset"]), int(row["last_size"])


def save_cursor(
    connection: sqlite3.Connection,
    path: str,
    last_offset: int,
    last_size: int,
) -> None:
    connection.execute(
        """
        INSERT INTO file_cursors(path, last_offset, last_size, updated_at)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(path) DO UPDATE SET
            last_offset = excluded.last_offset,
            last_size = excluded.last_size,
            updated_at = excluded.updated_at
        """,
        (path, last_offset, last_size, datetime.now().isoformat()),
    )


def insert_event(connection: sqlite3.Connection, event: TokenEvent) -> bool:
    source_event_id = event.source_event_id or f"{event.session_path}:{event.line_offset}"
    cursor = connection.execute(
        """
        INSERT OR IGNORE INTO token_events(
            session_path,
            line_offset,
            event_timestamp,
            total_tokens,
            input_tokens,
            cached_input_tokens,
            output_tokens,
            reasoning_output_tokens,
            cumulative_total_tokens,
            weekly_used_percent,
            weekly_window_minutes,
            weekly_resets_at,
            raw_json,
            source,
            source_event_id
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        """,
        (
            event.session_path,
            event.line_offset,
            event.event_timestamp.isoformat(),
            event.total_tokens,
            event.input_tokens,
            event.cached_input_tokens,
            event.output_tokens,
            event.reasoning_output_tokens,
            event.cumulative_total_tokens,
            event.weekly_used_percent,
            event.weekly_window_minutes,
            event.weekly_resets_at,
            event.raw_json,
            event.source,
            source_event_id,
        ),
    )
    return cursor.rowcount > 0


def upsert_claude_event(connection: sqlite3.Connection, event: TokenEvent) -> bool:
    """Insert-or-update a Claude event keyed by (source, source_event_id).

    Unlike Codex append-only events, a Claude request can still be streaming when
    a poll runs, so a later re-parse of the same transcript may carry the request's
    final (larger) cumulative usage for the same requestId. Replacing on the
    per-source key lets the latest parse win instead of being ignored, while
    keeping exactly one row per request (no double-count). Returns True when a new
    request row was created (not merely updated).
    """
    source_event_id = event.source_event_id or f"{event.session_path}:{event.line_offset}"
    existing = connection.execute(
        "SELECT id FROM token_events WHERE source = ? AND source_event_id = ?",
        (event.source, source_event_id),
    ).fetchone()
    if existing is None:
        connection.execute(
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
                event.session_path,
                event.line_offset,
                event.event_timestamp.isoformat(),
                event.total_tokens,
                event.input_tokens,
                event.cached_input_tokens,
                event.output_tokens,
                event.reasoning_output_tokens,
                event.cumulative_total_tokens,
                event.weekly_used_percent,
                event.weekly_window_minutes,
                event.weekly_resets_at,
                event.raw_json,
                event.source,
                source_event_id,
            ),
        )
        return True
    connection.execute(
        """
        UPDATE token_events SET
            session_path = ?, line_offset = ?, event_timestamp = ?,
            total_tokens = ?, input_tokens = ?, cached_input_tokens = ?,
            output_tokens = ?, reasoning_output_tokens = ?,
            cumulative_total_tokens = ?, weekly_used_percent = ?,
            weekly_window_minutes = ?, weekly_resets_at = ?, raw_json = ?
        WHERE source = ? AND source_event_id = ?
        """,
        (
            event.session_path,
            event.line_offset,
            event.event_timestamp.isoformat(),
            event.total_tokens,
            event.input_tokens,
            event.cached_input_tokens,
            event.output_tokens,
            event.reasoning_output_tokens,
            event.cumulative_total_tokens,
            event.weekly_used_percent,
            event.weekly_window_minutes,
            event.weekly_resets_at,
            event.raw_json,
            event.source,
            source_event_id,
        ),
    )
    return False


def insert_session_context_marker(
    connection: sqlite3.Connection,
    marker: SessionContextMarker,
) -> bool:
    cursor = connection.execute(
        """
        INSERT OR IGNORE INTO session_context_markers(
            session_path,
            line_offset,
            cwd,
            project_key,
            project_label,
            project_source
        ) VALUES (?, ?, ?, ?, ?, ?)
        """,
        (
            marker.session_path,
            marker.line_offset,
            marker.cwd,
            marker.project_key,
            marker.project_label,
            marker.project_source,
        ),
    )
    return cursor.rowcount > 0


def load_events_since(
    connection: sqlite3.Connection,
    since: datetime,
) -> list[TokenEvent]:
    rows = connection.execute(
        """
        SELECT *
        FROM token_events
        WHERE event_timestamp >= ?
        ORDER BY event_timestamp ASC
        """,
        (since.isoformat(),),
    ).fetchall()
    return [_row_to_token_event(row) for row in rows]


def sum_total_tokens_by_source_since(
    connection: sqlite3.Connection,
    since: datetime,
) -> dict[str, int]:
    """Return {source: SUM(total_tokens)} for events at/after `since`.

    Task-0013 activation-fix follow-up (Fix B): the background poll needs the
    rolling 7-day total but must NOT materialize every event in the window just
    to sum it. This pushes the sum into SQLite (covered by the
    idx_token_events_ts_source_total index) and groups by `source` so the source
    filter can include/exclude a source from the displayed 7-day total purely in
    memory, without re-reading the database. Legacy NULL/empty sources are folded
    into "codex".
    """
    # Force the covering (event_timestamp, source, total_tokens) index: the
    # planner otherwise prefers the (source, source_event_id) index because the
    # GROUP BY avoids a temp B-tree, but that FULL-scans the table (~385 ms on a
    # 1.4M-row DB). The covering index range-scans only the 7-day slice (~44 ms).
    # initialize_db (called before this) guarantees the index exists.
    rows = connection.execute(
        """
        SELECT source, COALESCE(SUM(total_tokens), 0) AS total
        FROM token_events INDEXED BY idx_token_events_ts_source_total
        WHERE event_timestamp >= ?
        GROUP BY source
        """,
        (since.isoformat(),),
    ).fetchall()
    totals: dict[str, int] = {}
    for row in rows:
        source = str(row["source"]) if row["source"] else "codex"
        totals[source] = totals.get(source, 0) + int(row["total"] or 0)
    return totals


def load_latest_weekly_advisory(
    connection: sqlite3.Connection,
    since: datetime,
) -> TokenEvent | None:
    """Return the most recent event carrying a weekly advisory, or None.

    Task-0013 activation-fix follow-up (Fix B): the dashboard shows the latest
    Codex weekly advisory. When the chart window is short (Fix B loads only the
    charted span), the latest advisory can fall outside that span, so fetch it
    with a cheap indexed lookback instead of scanning the full window in memory.
    """
    row = connection.execute(
        """
        SELECT *
        FROM token_events
        WHERE event_timestamp >= ? AND weekly_used_percent IS NOT NULL
        ORDER BY event_timestamp DESC
        LIMIT 1
        """,
        (since.isoformat(),),
    ).fetchone()
    if row is None:
        return None
    return _row_to_token_event(row)


def _row_to_token_event(row: sqlite3.Row) -> TokenEvent:
    return TokenEvent(
        session_path=str(row["session_path"]),
        line_offset=int(row["line_offset"]),
        event_timestamp=datetime.fromisoformat(str(row["event_timestamp"])),
        total_tokens=int(row["total_tokens"]),
        input_tokens=int(row["input_tokens"]),
        cached_input_tokens=int(row["cached_input_tokens"]),
        output_tokens=int(row["output_tokens"]),
        reasoning_output_tokens=int(row["reasoning_output_tokens"]),
        cumulative_total_tokens=int(row["cumulative_total_tokens"]),
        weekly_used_percent=(
            float(row["weekly_used_percent"])
            if row["weekly_used_percent"] is not None
            else None
        ),
        weekly_window_minutes=(
            int(row["weekly_window_minutes"])
            if row["weekly_window_minutes"] is not None
            else None
        ),
        weekly_resets_at=(
            int(row["weekly_resets_at"])
            if row["weekly_resets_at"] is not None
            else None
        ),
        raw_json=str(row["raw_json"]),
        source=str(row["source"]) if row["source"] is not None else "codex",
        source_event_id=(
            str(row["source_event_id"])
            if row["source_event_id"] is not None
            else ""
        ),
    )


def count_session_context_markers(connection: sqlite3.Connection, session_path: str) -> int:
    row = connection.execute(
        """
        SELECT COUNT(*) AS total
        FROM session_context_markers
        WHERE session_path = ?
        """,
        (session_path,),
    ).fetchone()
    return int(row["total"])


def load_session_context_markers(
    connection: sqlite3.Connection,
    session_paths: list[str],
) -> dict[str, list[SessionContextMarker]]:
    if not session_paths:
        return {}

    markers_by_session: dict[str, list[SessionContextMarker]] = {}
    batch_size = 500
    for start in range(0, len(session_paths), batch_size):
        batch = session_paths[start : start + batch_size]
        placeholders = ",".join("?" for _ in batch)
        rows = connection.execute(
            f"""
            SELECT session_path, line_offset, cwd, project_key, project_label, project_source
            FROM session_context_markers
            WHERE session_path IN ({placeholders})
            ORDER BY session_path ASC, line_offset ASC
            """,
            batch,
        ).fetchall()
        for row in rows:
            marker = SessionContextMarker(
                session_path=str(row["session_path"]),
                line_offset=int(row["line_offset"]),
                cwd=str(row["cwd"]) if row["cwd"] is not None else None,
                project_key=str(row["project_key"]),
                project_label=str(row["project_label"]),
                project_source=str(row["project_source"]),
            )
            markers_by_session.setdefault(marker.session_path, []).append(marker)
    return markers_by_session


def count_events(connection: sqlite3.Connection) -> int:
    row = connection.execute("SELECT COUNT(*) AS total FROM token_events").fetchone()
    return int(row["total"])
