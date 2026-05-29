from __future__ import annotations

from datetime import UTC, datetime, timedelta, tzinfo

from .attribution import UNKNOWN_PROJECT_KEY, UNKNOWN_PROJECT_LABEL
from .models import AggregationBucket, SessionContextMarker, TokenEvent


INTERVAL_SECONDS = {
    "1m": 60,
    "5m": 300,
    "15m": 900,
    "1h": 3600,
    "1d": 86400,
}
METRIC_MODES = {
    "total": "Total",
    "norm": "Norm",
}
# Task-0013 Objective 4: the ingest sources the overlay can filter by. Order is
# the display order in the source filter control. "codex" is the legacy default
# for rows written before the source column existed.
KNOWN_SOURCES = ("codex", "claude")
SOURCE_LABELS = {
    "codex": "Codex",
    "claude": "Claude",
}
NORMALIZED_CACHED_INPUT_WEIGHT = 0.1
NORMALIZED_OUTPUT_WEIGHT = 6.0


def filter_events_by_source(
    events: list[TokenEvent],
    selected_sources,
) -> list[TokenEvent]:
    """Return only events whose `source` is in `selected_sources`.

    Task-0013 Objective 4: the source filter operates purely on the already
    stored `source` column of the in-memory snapshot. It performs no database
    read, so it is safe to call on the Tk UI thread when a filter checkbox
    toggles (preserving Objective 3's non-blocking activation). When
    `selected_sources` is None, all events pass through (today's merged
    behavior). When it is empty, nothing passes (a clear zero state, not a
    crash).
    """
    if selected_sources is None:
        return list(events)
    allowed = set(selected_sources)
    return [event for event in events if (event.source or "codex") in allowed]


def _resolve_display_timezone(
    now: datetime | None,
    display_tz: tzinfo | None,
) -> tzinfo:
    if display_tz is not None:
        return display_tz
    if now is not None and now.tzinfo is not None:
        return now.tzinfo
    return datetime.now().astimezone().tzinfo or UTC


def _align_bucket_start(dt_local: datetime, interval_key: str) -> datetime:
    if interval_key == "1d":
        return dt_local.replace(hour=0, minute=0, second=0, microsecond=0)
    if interval_key == "1h":
        return dt_local.replace(minute=0, second=0, microsecond=0)

    minute_step = INTERVAL_SECONDS[interval_key] // 60
    aligned_minute = dt_local.minute - (dt_local.minute % minute_step)
    return dt_local.replace(minute=aligned_minute, second=0, microsecond=0)


def event_metric_tokens(event: TokenEvent, metric_mode: str = "total") -> int:
    if metric_mode == "total":
        return event.total_tokens
    if metric_mode == "norm":
        uncached_input_tokens = max(0, event.input_tokens - event.cached_input_tokens)
        weighted_output_tokens = event.output_tokens + event.reasoning_output_tokens
        normalized_total = (
            uncached_input_tokens
            + (event.cached_input_tokens * NORMALIZED_CACHED_INPUT_WEIGHT)
            + (weighted_output_tokens * NORMALIZED_OUTPUT_WEIGHT)
        )
        return int(round(normalized_total))
    raise ValueError(f"Unsupported metric mode: {metric_mode}")


def build_buckets(
    events: list[TokenEvent],
    interval_key: str,
    bucket_count: int,
    now: datetime | None = None,
    display_tz: tzinfo | None = None,
    metric_mode: str = "total",
) -> list[AggregationBucket]:
    if interval_key not in INTERVAL_SECONDS:
        raise ValueError(f"Unsupported interval: {interval_key}")
    if metric_mode not in METRIC_MODES:
        raise ValueError(f"Unsupported metric mode: {metric_mode}")
    timezone = _resolve_display_timezone(now, display_tz)
    now_local = (now or datetime.now(timezone)).astimezone(timezone)
    interval_seconds = INTERVAL_SECONDS[interval_key]
    aligned_end = _align_bucket_start(now_local, interval_key)
    buckets: list[AggregationBucket] = []
    totals: dict[datetime, int] = {}
    for event in events:
        bucket_start = _align_bucket_start(event.event_timestamp.astimezone(timezone), interval_key)
        totals[bucket_start] = totals.get(bucket_start, 0) + event_metric_tokens(event, metric_mode)

    for offset in range(bucket_count - 1, -1, -1):
        start_at = aligned_end - timedelta(seconds=interval_seconds * offset)
        end_at = start_at + timedelta(seconds=interval_seconds)
        buckets.append(
            AggregationBucket(
                start_at=start_at,
                end_at=end_at,
                total_tokens=totals.get(start_at, 0),
            )
        )
    return buckets


def build_project_stacks(
    events: list[TokenEvent],
    session_context_markers: dict[str, list[SessionContextMarker]],
    interval_key: str,
    bucket_count: int,
    now: datetime | None = None,
    display_tz: tzinfo | None = None,
    top_n: int = 5,
    metric_mode: str = "total",
) -> tuple[list[AggregationBucket], list[tuple[str, str]], list[dict[str, int]]]:
    buckets = build_buckets(
        events,
        interval_key,
        bucket_count,
        now=now,
        display_tz=display_tz,
        metric_mode=metric_mode,
    )
    if not buckets:
        return [], [], []

    timezone = _resolve_display_timezone(now, display_tz)
    top_keys_by_session: dict[str, list[SessionContextMarker]] = {
        session_path: sorted(markers, key=lambda marker: marker.line_offset)
        for session_path, markers in session_context_markers.items()
    }
    window_totals: dict[str, int] = {}
    label_by_key: dict[str, str] = {}
    bucket_totals: dict[datetime, dict[str, int]] = {bucket.start_at: {} for bucket in buckets}
    events_by_session: dict[str, list[TokenEvent]] = {}
    for event in events:
        events_by_session.setdefault(event.session_path, []).append(event)

    for session_path, session_events in events_by_session.items():
        markers = top_keys_by_session.get(session_path, [])
        marker_index = 0
        current_marker = markers[0] if markers else None
        sorted_events = sorted(session_events, key=lambda event: event.line_offset)
        for event in sorted_events:
            while marker_index + 1 < len(markers) and markers[marker_index + 1].line_offset <= event.line_offset:
                marker_index += 1
                current_marker = markers[marker_index]
            bucket_start = _align_bucket_start(event.event_timestamp.astimezone(timezone), interval_key)
            if bucket_start not in bucket_totals:
                continue
            project_key = current_marker.project_key if current_marker is not None else UNKNOWN_PROJECT_KEY
            project_label = current_marker.project_label if current_marker is not None else UNKNOWN_PROJECT_LABEL
            metric_tokens = event_metric_tokens(event, metric_mode)
            label_by_key[project_key] = project_label
            bucket_totals[bucket_start][project_key] = bucket_totals[bucket_start].get(project_key, 0) + metric_tokens
            window_totals[project_key] = window_totals.get(project_key, 0) + metric_tokens

    top_keys = [
        key
        for key, _total in sorted(
            window_totals.items(),
            key=lambda item: (-item[1], label_by_key.get(item[0], item[0]).lower()),
        )[:top_n]
    ]
    legend_entries = [(key, label_by_key[key]) for key in top_keys]
    other_total = sum(total for key, total in window_totals.items() if key not in top_keys)
    if other_total > 0:
        legend_entries.append(("__other__", "Other"))

    stacked_totals: list[dict[str, int]] = []
    for bucket in buckets:
        project_totals = dict(bucket_totals[bucket.start_at])
        reduced_totals: dict[str, int] = {}
        other_bucket_total = 0
        for project_key, total_tokens in project_totals.items():
            if project_key in top_keys:
                reduced_totals[project_key] = total_tokens
            else:
                other_bucket_total += total_tokens
        if other_bucket_total > 0:
            reduced_totals["__other__"] = other_bucket_total
        stacked_totals.append(reduced_totals)

    return buckets, legend_entries, stacked_totals


def project_weekly_burn(tokens_in_interval: int, interval_seconds: int) -> int:
    if interval_seconds <= 0:
        raise ValueError("interval_seconds must be positive")
    return int(tokens_in_interval * (7 * 24 * 60 * 60 / interval_seconds))


def is_over_redline(
    tokens_in_interval: int,
    interval_seconds: int,
    weekly_budget_tokens: int,
) -> bool:
    return project_weekly_burn(tokens_in_interval, interval_seconds) > weekly_budget_tokens
