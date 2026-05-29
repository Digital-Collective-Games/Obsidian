from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime


@dataclass(frozen=True, slots=True)
class TokenEvent:
    session_path: str
    line_offset: int
    event_timestamp: datetime
    total_tokens: int
    input_tokens: int
    cached_input_tokens: int
    output_tokens: int
    reasoning_output_tokens: int
    cumulative_total_tokens: int
    weekly_used_percent: float | None
    weekly_window_minutes: int | None
    weekly_resets_at: int | None
    raw_json: str
    # Task-0013 Objective 2: source discriminator ("codex" | "claude") and a
    # per-source idempotency key. source_event_id is the Claude requestId for
    # Claude events and "<session_path>:<line_offset>" for Codex events.
    source: str = "codex"
    source_event_id: str = ""


@dataclass(frozen=True, slots=True)
class SessionContextMarker:
    session_path: str
    line_offset: int
    cwd: str | None
    project_key: str
    project_label: str
    project_source: str


@dataclass(frozen=True, slots=True)
class AggregationBucket:
    start_at: datetime
    end_at: datetime
    total_tokens: int


@dataclass(frozen=True, slots=True)
class IngestRunSummary:
    files_scanned: int
    files_updated: int
    events_ingested: int
