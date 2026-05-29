from __future__ import annotations

import json
from collections import Counter
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path

from .attribution import (
    UNKNOWN_PROJECT_KEY,
    UNKNOWN_PROJECT_LABEL,
    resolve_project_identity,
)
from .models import AggregationBucket, SessionContextMarker, TokenEvent


@dataclass(frozen=True, slots=True)
class BucketInvestigation:
    title: str
    markdown: str
    workspace_root: Path
    add_dirs: list[Path]
    session_paths: list[Path]


@dataclass(frozen=True, slots=True)
class InvestigationPaths:
    brief_path: Path
    report_path: Path


def _parse_timestamp(raw_value: object) -> datetime | None:
    if not isinstance(raw_value, str) or not raw_value:
        return None
    try:
        return datetime.fromisoformat(raw_value.replace("Z", "+00:00"))
    except ValueError:
        return None


def _normalize_excerpt(raw_text: str, limit: int = 180) -> str:
    collapsed = " ".join(raw_text.split())
    if len(collapsed) <= limit:
        return collapsed
    return f"{collapsed[: limit - 3].rstrip()}..."


def _format_token_value(value: int) -> str:
    absolute_value = abs(value)
    for divisor, suffix in (
        (1_000_000_000, "B"),
        (1_000_000, "M"),
        (1_000, "K"),
    ):
        if absolute_value >= divisor:
            scaled = value / divisor
            scaled_text = f"{scaled:.1f}".rstrip("0").rstrip(".")
            return f"{scaled_text}{suffix} ({value:,})"
    return f"{value:,}"


def _format_bucket_range(bucket: AggregationBucket) -> str:
    start_text = bucket.start_at.strftime("%Y-%m-%d %I:%M %p %Z").strip()
    end_text = bucket.end_at.strftime("%I:%M %p %Z").strip()
    return f"{start_text} to {end_text}"


def _extract_message_text(payload: dict[str, object]) -> str | None:
    message = payload.get("message")
    if isinstance(message, str) and message.strip():
        return _normalize_excerpt(message)
    content = payload.get("content")
    if not isinstance(content, list):
        return None
    fragments: list[str] = []
    for item in content:
        if not isinstance(item, dict):
            continue
        text = item.get("text")
        if isinstance(text, str) and text.strip():
            fragments.append(text.strip())
    if not fragments:
        return None
    return _normalize_excerpt(" ".join(fragments))


def _fallback_project_from_session_file(
    session_path: str,
    fallback_cache: dict[str, tuple[str, str]],
) -> tuple[str, str]:
    cached = fallback_cache.get(session_path)
    if cached is not None:
        return cached

    session_file = Path(session_path)
    if not session_file.exists():
        fallback_cache[session_path] = (UNKNOWN_PROJECT_KEY, UNKNOWN_PROJECT_LABEL)
        return fallback_cache[session_path]

    project_key = UNKNOWN_PROJECT_KEY
    project_label = UNKNOWN_PROJECT_LABEL
    try:
        with session_file.open("r", encoding="utf-8") as handle:
            for line in handle:
                try:
                    record = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if record.get("type") not in {"session_meta", "turn_context"}:
                    continue
                payload = record.get("payload")
                if not isinstance(payload, dict):
                    continue
                raw_cwd = payload.get("cwd")
                cwd, label, _source = resolve_project_identity(
                    str(raw_cwd) if raw_cwd is not None else None
                )
                project_key = cwd or UNKNOWN_PROJECT_KEY
                project_label = label
                break
    except OSError:
        pass

    fallback_cache[session_path] = (project_key, project_label)
    return fallback_cache[session_path]


def _resolve_event_project(
    event: TokenEvent,
    session_context_markers: dict[str, list[SessionContextMarker]],
    fallback_cache: dict[str, tuple[str, str]],
) -> tuple[str, str]:
    markers = session_context_markers.get(event.session_path, [])
    current_marker: SessionContextMarker | None = None
    for marker in markers:
        if marker.line_offset > event.line_offset:
            break
        current_marker = marker
    if current_marker is not None:
        return current_marker.project_key, current_marker.project_label
    return _fallback_project_from_session_file(event.session_path, fallback_cache)


def _summarize_session_activity(
    session_path: str,
    bucket: AggregationBucket,
) -> dict[str, object]:
    summary: dict[str, object] = {
        "cwd": None,
        "project_key": UNKNOWN_PROJECT_KEY,
        "project_label": UNKNOWN_PROJECT_LABEL,
        "agent_nickname": None,
        "agent_role": None,
        "spawn_depth": None,
        "commentary": [],
        "tool_names": [],
        "tool_counts": {},
        "token_bursts": [],
        "token_event_count": 0,
        "token_total": 0,
        "input_total": 0,
        "cached_total": 0,
        "output_total": 0,
        "reasoning_total": 0,
        "first_timestamp": None,
        "last_timestamp": None,
    }
    session_file = Path(session_path)
    if not session_file.exists():
        return summary

    tool_counts: Counter[str] = Counter()
    try:
        with session_file.open("r", encoding="utf-8") as handle:
            for line in handle:
                try:
                    record = json.loads(line)
                except json.JSONDecodeError:
                    continue

                record_type = record.get("type")
                payload = record.get("payload")
                if not isinstance(payload, dict):
                    continue

                if record_type in {"session_meta", "turn_context"}:
                    raw_cwd = payload.get("cwd")
                    cwd, label, _source = resolve_project_identity(
                        str(raw_cwd) if raw_cwd is not None else None
                    )
                    summary["cwd"] = cwd
                    summary["project_key"] = cwd or UNKNOWN_PROJECT_KEY
                    summary["project_label"] = label
                    if record_type == "session_meta":
                        nickname = payload.get("agent_nickname")
                        role = payload.get("agent_role")
                        if isinstance(nickname, str) and nickname.strip():
                            summary["agent_nickname"] = nickname.strip()
                        if isinstance(role, str) and role.strip():
                            summary["agent_role"] = role.strip()
                        source = payload.get("source")
                        if isinstance(source, dict):
                            subagent = source.get("subagent")
                            if isinstance(subagent, dict):
                                thread_spawn = subagent.get("thread_spawn")
                                if isinstance(thread_spawn, dict):
                                    depth = thread_spawn.get("depth")
                                    if isinstance(depth, int):
                                        summary["spawn_depth"] = depth

                timestamp = _parse_timestamp(record.get("timestamp"))
                if timestamp is None:
                    continue
                local_timestamp = timestamp.astimezone(bucket.start_at.tzinfo)
                if not (bucket.start_at <= local_timestamp < bucket.end_at):
                    continue
                if summary["first_timestamp"] is None:
                    summary["first_timestamp"] = local_timestamp
                summary["last_timestamp"] = local_timestamp

                if record_type == "event_msg":
                    payload_type = payload.get("type")
                    if payload_type == "agent_message":
                        text = _extract_message_text(payload)
                        commentary = summary["commentary"]
                        if (
                            isinstance(text, str)
                            and isinstance(commentary, list)
                            and text not in commentary
                            and len(commentary) < 3
                        ):
                            commentary.append(text)
                    if payload_type == "token_count":
                        info = payload.get("info")
                        if not isinstance(info, dict):
                            continue
                        last_usage = info.get("last_token_usage")
                        if not isinstance(last_usage, dict):
                            continue
                        total_tokens = int(last_usage.get("total_tokens") or 0)
                        summary["token_event_count"] = int(summary["token_event_count"]) + 1
                        summary["token_total"] = int(summary["token_total"]) + total_tokens
                        summary["input_total"] = int(summary["input_total"]) + int(last_usage.get("input_tokens") or 0)
                        summary["cached_total"] = int(summary["cached_total"]) + int(last_usage.get("cached_input_tokens") or 0)
                        summary["output_total"] = int(summary["output_total"]) + int(last_usage.get("output_tokens") or 0)
                        summary["reasoning_total"] = int(summary["reasoning_total"]) + int(
                            last_usage.get("reasoning_output_tokens") or 0
                        )
                        token_bursts = summary["token_bursts"]
                        if isinstance(token_bursts, list):
                            token_bursts.append(
                                {
                                    "timestamp": local_timestamp,
                                    "total_tokens": total_tokens,
                                    "input_tokens": int(last_usage.get("input_tokens") or 0),
                                    "cached_input_tokens": int(last_usage.get("cached_input_tokens") or 0),
                                    "output_tokens": int(last_usage.get("output_tokens") or 0),
                                    "reasoning_output_tokens": int(
                                        last_usage.get("reasoning_output_tokens") or 0
                                    ),
                                }
                            )
                if record_type == "response_item":
                    payload_type = payload.get("type")
                    if payload_type == "function_call":
                        tool_name = payload.get("name")
                        tool_names = summary["tool_names"]
                        if (
                            isinstance(tool_name, str)
                            and isinstance(tool_names, list)
                            and tool_name not in tool_names
                            and len(tool_names) < 6
                        ):
                            tool_names.append(tool_name)
                            tool_counts[tool_name] += 1
                        elif isinstance(tool_name, str):
                            tool_counts[tool_name] += 1
    except OSError:
        return summary

    token_bursts = summary["token_bursts"]
    if isinstance(token_bursts, list):
        token_bursts.sort(
            key=lambda item: (
                -int(item["total_tokens"]),
                str(item["timestamp"]),
            )
        )
        summary["token_bursts"] = token_bursts[:3]
    summary["tool_counts"] = dict(tool_counts.most_common(10))
    return summary


def _format_session_mechanics(summary: dict[str, object]) -> list[str]:
    mechanics: list[str] = []
    first_timestamp = summary.get("first_timestamp")
    last_timestamp = summary.get("last_timestamp")
    if isinstance(first_timestamp, datetime) and isinstance(last_timestamp, datetime):
        mechanics.append(
            "- Active in bucket: "
            f"{first_timestamp.strftime('%I:%M:%S %p').lstrip('0')} to "
            f"{last_timestamp.strftime('%I:%M:%S %p').lstrip('0')}"
        )
    token_event_count = int(summary.get("token_event_count") or 0)
    token_total = int(summary.get("token_total") or 0)
    if token_event_count > 0:
        average_total = int(round(token_total / token_event_count))
        mechanics.append(
            "- Model turns in bucket: "
            f"{token_event_count} token-count events, avg {_format_token_value(average_total)} per turn"
        )
        average_input = int(round(int(summary.get("input_total") or 0) / token_event_count))
        average_cached = int(round(int(summary.get("cached_total") or 0) / token_event_count))
        average_output = int(round(int(summary.get("output_total") or 0) / token_event_count))
        mechanics.append(
            "- Turn profile: "
            f"avg input {_format_token_value(average_input)}, "
            f"avg cached {_format_token_value(average_cached)}, "
            f"avg output {_format_token_value(average_output)}"
        )
    tool_counts = summary.get("tool_counts")
    if isinstance(tool_counts, dict) and tool_counts:
        formatted = ", ".join(f"{name} x{count}" for name, count in tool_counts.items())
        mechanics.append(f"- Tool loop counts: {formatted}")
    spawn_depth = summary.get("spawn_depth")
    if isinstance(spawn_depth, int):
        mechanics.append(f"- Spawn depth: {spawn_depth}")
    return mechanics


def _infer_bucket_root_cause_hints(session_diagnostics: list[dict[str, object]]) -> list[str]:
    if not session_diagnostics:
        return ["- No root-cause hints were inferred from the session mechanics."]

    bucket_total = sum(int(item["session_total"]) for item in session_diagnostics)
    diagnostics_sorted = sorted(
        session_diagnostics,
        key=lambda item: (-int(item["session_total"]), str(item["session_path"]).lower()),
    )
    dominant_sessions = diagnostics_sorted[:2]
    dominant_total = sum(int(item["session_total"]) for item in dominant_sessions)
    hints = [
        "- Dominant cost source: repeated input-context reload, not generation."
    ]
    if bucket_total > 0 and dominant_sessions:
        share = (dominant_total / bucket_total) * 100
        session_names = ", ".join(Path(str(item["session_path"])).name for item in dominant_sessions)
        hints.append(
            "- Dominant sessions: "
            f"{session_names} account for {_format_token_value(dominant_total)} "
            f"({share:.1f}% of this bucket)."
        )

    total_wait = 0
    total_spawn = 0
    total_send = 0
    total_resume = 0
    total_shell = 0
    max_depth = 0
    for item in dominant_sessions:
        tool_counts = item.get("tool_counts")
        if isinstance(tool_counts, dict):
            total_wait += int(tool_counts.get("wait_agent") or 0)
            total_spawn += int(tool_counts.get("spawn_agent") or 0)
            total_send += int(tool_counts.get("send_input") or 0)
            total_resume += int(tool_counts.get("resume_agent") or 0)
            total_shell += int(tool_counts.get("shell_command") or 0)
        spawn_depth = item.get("spawn_depth")
        if isinstance(spawn_depth, int):
            max_depth = max(max_depth, spawn_depth)
    if total_wait or total_spawn or total_send or total_resume:
        hints.append(
            "- Orchestration churn: "
            f"wait_agent x{total_wait}, spawn_agent x{total_spawn}, "
            f"send_input x{total_send}, resume_agent x{total_resume}, "
            f"shell_command x{total_shell} across the top sessions."
        )
    if max_depth > 0:
        hints.append(
            "- Nested delegation: the dominant sessions were already spawned workers, "
            f"and they still delegated again (observed spawn depth {max_depth})."
        )

    total_output = sum(int(item["output_total"]) for item in dominant_sessions)
    total_input = sum(int(item["input_total"]) for item in dominant_sessions)
    if total_input > 0:
        output_share = (total_output / total_input) * 100
        hints.append(
            "- Productivity signal: output was tiny relative to input, "
            f"about {output_share:.2f}% of input on the dominant sessions."
        )

    hints.append(
        "- Likely user-triggered root cause: launching the heavy task-leader orchestration flow "
        "with continuous supervision, then repeatedly polling or nudging the same delegated leaders "
        "while those workers also spawned more agents."
    )
    hints.append(
        "- Avoidance: use one local agent or one delegated leader, avoid nested spawn_agent inside "
        "worker threads, and do not hammer wait_agent/send_input status checks on short intervals."
    )
    return hints


def build_bucket_investigation(
    bucket: AggregationBucket,
    events: list[TokenEvent],
    session_context_markers: dict[str, list[SessionContextMarker]],
    interval_key: str,
    chart_mode: str,
    codex_root: Path,
) -> BucketInvestigation:
    bucket_events: list[TokenEvent] = []
    for event in events:
        event_local = event.event_timestamp.astimezone(bucket.start_at.tzinfo)
        if bucket.start_at <= event_local < bucket.end_at:
            bucket_events.append(event)

    fallback_cache: dict[str, tuple[str, str]] = {}
    project_totals: dict[str, int] = {}
    project_labels: dict[str, str] = {}
    session_totals: dict[str, int] = {}
    session_projects: dict[str, str] = {}

    for event in bucket_events:
        project_key, project_label = _resolve_event_project(
            event,
            session_context_markers,
            fallback_cache,
        )
        project_totals[project_key] = project_totals.get(project_key, 0) + event.total_tokens
        project_labels[project_key] = project_label
        session_totals[event.session_path] = session_totals.get(event.session_path, 0) + event.total_tokens
        session_projects.setdefault(event.session_path, project_label)

    sorted_projects = sorted(
        project_totals.items(),
        key=lambda item: (-item[1], project_labels.get(item[0], item[0]).lower()),
    )
    sorted_sessions = sorted(
        session_totals.items(),
        key=lambda item: (-item[1], item[0].lower()),
    )

    project_lines = [
        f"- {project_labels.get(project_key, project_key)}: {_format_token_value(total_tokens)}"
        for project_key, total_tokens in sorted_projects
    ] or ["- No repo attribution for this bucket."]

    top_session_lines = [
        f"- {Path(session_path).name}: {_format_token_value(total_tokens)}"
        f" [{session_projects.get(session_path, UNKNOWN_PROJECT_LABEL)}]"
        for session_path, total_tokens in sorted_sessions[:12]
    ] or ["- No session files contributed to this bucket."]

    session_path_lines = [
        f"- {session_path}"
        for session_path, _total_tokens in sorted_sessions[:20]
    ] or ["- No session files contributed to this bucket."]

    project_dirs: list[Path] = []
    seen_dirs: set[str] = set()
    for project_key, _total_tokens in sorted_projects:
        if project_key == UNKNOWN_PROJECT_KEY:
            continue
        project_path = Path(project_key)
        if not project_path.exists():
            continue
        normalized = str(project_path.resolve(strict=False)).lower()
        if normalized in seen_dirs:
            continue
        seen_dirs.add(normalized)
        project_dirs.append(project_path)

    bucket_input_tokens = sum(event.input_tokens for event in bucket_events)
    bucket_cached_tokens = sum(event.cached_input_tokens for event in bucket_events)
    bucket_output_tokens = sum(event.output_tokens for event in bucket_events)
    bucket_reasoning_tokens = sum(event.reasoning_output_tokens for event in bucket_events)

    token_burst_lines: list[str] = []
    session_signal_blocks: list[str] = []
    session_diagnostics: list[dict[str, object]] = []
    for session_path, session_total in sorted_sessions[:5]:
        session_summary = _summarize_session_activity(session_path, bucket)
        session_label = (
            str(session_summary.get("project_label"))
            if session_summary.get("project_label")
            else session_projects.get(session_path, UNKNOWN_PROJECT_LABEL)
        )
        agent_bits = [
            bit
            for bit in (
                session_summary.get("agent_nickname"),
                session_summary.get("agent_role"),
            )
            if isinstance(bit, str) and bit
        ]
        heading = f"### {Path(session_path).name}"
        if agent_bits:
            heading += f" | {' '.join(agent_bits)}"
        session_signal_blocks.append(heading)
        session_signal_blocks.append(f"- Bucket spend: {_format_token_value(session_total)}")
        session_signal_blocks.append(f"- Repo/CWD: {session_label}")
        cwd = session_summary.get("cwd")
        if isinstance(cwd, str) and cwd:
            session_signal_blocks.append(f"- Workspace: {cwd}")
        commentary = session_summary.get("commentary")
        if isinstance(commentary, list) and commentary:
            session_signal_blocks.append(f"- Commentary: {commentary[0]}")
        tool_names = session_summary.get("tool_names")
        if isinstance(tool_names, list) and tool_names:
            session_signal_blocks.append(f"- Tool calls: {', '.join(tool_names)}")
        session_signal_blocks.extend(_format_session_mechanics(session_summary))
        token_bursts = session_summary.get("token_bursts")
        if isinstance(token_bursts, list) and token_bursts:
            largest_burst = token_bursts[0]
            if isinstance(largest_burst, dict):
                timestamp = largest_burst["timestamp"]
                if isinstance(timestamp, datetime):
                    burst_time = timestamp.strftime("%I:%M:%S %p").lstrip("0")
                else:
                    burst_time = "unknown time"
                token_burst_lines.append(
                    "- "
                    f"{burst_time} | {Path(session_path).name} | "
                    f"{_format_token_value(int(largest_burst['total_tokens']))} | "
                    f"in {int(largest_burst['input_tokens']):,} | "
                    f"cached {int(largest_burst['cached_input_tokens']):,} | "
                    f"out {int(largest_burst['output_tokens']):,} | "
                    f"reasoning {int(largest_burst['reasoning_output_tokens']):,}"
                )
        session_diagnostics.append(
            {
                "session_path": session_path,
                "session_total": session_total,
                "tool_counts": session_summary.get("tool_counts"),
                "spawn_depth": session_summary.get("spawn_depth"),
                "output_total": session_summary.get("output_total"),
                "input_total": session_summary.get("input_total"),
            }
        )

    if not token_burst_lines:
        token_burst_lines = ["- No token burst detail was found inside the session files for this bucket."]
    if not session_signal_blocks:
        session_signal_blocks = ["- No session signal detail was found for this bucket."]
    root_cause_hints = _infer_bucket_root_cause_hints(session_diagnostics)

    title = f"{interval_key} bucket {_format_bucket_range(bucket)}"
    markdown = "\n".join(
        [
            "# Obsidian Bucket Investigation",
            "",
            f"- Bucket: {_format_bucket_range(bucket)}",
            f"- Interval: {interval_key}",
            f"- Chart mode: {chart_mode}",
            f"- Total tokens: {_format_token_value(bucket.total_tokens)}",
            f"- Contributing sessions: {len(sorted_sessions)}",
            "",
            "## Bucket Composition",
            f"- Input: {_format_token_value(bucket_input_tokens)}",
            f"- Cached input: {_format_token_value(bucket_cached_tokens)}",
            f"- Output: {_format_token_value(bucket_output_tokens)}",
            f"- Reasoning: {_format_token_value(bucket_reasoning_tokens)}",
            "",
            "## Repo Breakdown",
            *project_lines,
            "",
            "## Top Sessions",
            *top_session_lines,
            "",
            "## Top Token Bursts",
            *token_burst_lines,
            "",
            "## Root Cause Hints",
            *root_cause_hints,
            "",
            "## Session Signals",
            *session_signal_blocks,
            "",
            "## Session Paths",
            *session_path_lines,
            "",
            "## Investigation Ask",
            "- Produce a root-cause analysis, not a bucket summary.",
            "- Explain what actually happened in this bucket using the session files, not just this summary.",
            "- Identify the concrete operator or workflow action that triggered the spend spike.",
            "- Distinguish whether the spike came from one expensive task or from repeated context/tool-loop churn.",
            "- Quantify the top drivers of spend and name the avoidable ones.",
            "- Decide whether the spend looks productive, wasteful, or anomalous.",
            "- Recommend the most important next action to prevent a repeat.",
        ]
    )
    return BucketInvestigation(
        title=title,
        markdown=markdown,
        workspace_root=codex_root,
        add_dirs=project_dirs,
        session_paths=[Path(session_path) for session_path, _total_tokens in sorted_sessions],
    )


def build_investigation_paths(
    output_root: Path,
    timestamp: datetime,
) -> InvestigationPaths:
    output_root.mkdir(parents=True, exist_ok=True)
    stem = timestamp.strftime("%Y%m%d-%H%M%S")
    return InvestigationPaths(
        brief_path=output_root / f"bucket-investigation-{stem}-brief.md",
        report_path=output_root / f"bucket-investigation-{stem}-report.md",
    )


def write_bucket_investigation(
    investigation: BucketInvestigation,
    output_root: Path,
    timestamp: datetime,
) -> Path:
    output_paths = build_investigation_paths(output_root, timestamp)
    output_paths.brief_path.write_text(investigation.markdown, encoding="utf-8")
    return output_paths.brief_path


def report_path_for_brief(brief_path: Path) -> Path:
    brief_name = brief_path.name
    if brief_name.endswith("-brief.md"):
        return brief_path.with_name(brief_name.replace("-brief.md", "-report.md"))
    return brief_path.with_name(f"{brief_path.stem}-report.md")


def _powershell_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def build_codex_launch_command(
    codex_executable: str,
    brief_path: Path,
    report_path: Path,
    workspace_root: Path,
    add_dirs: list[Path],
) -> list[str]:
    unique_dirs: list[Path] = []
    seen_dirs: set[str] = set()
    workspace_normalized = str(workspace_root.resolve(strict=False)).lower()
    for directory in add_dirs:
        normalized = str(directory.resolve(strict=False)).lower()
        if normalized == workspace_normalized or normalized in seen_dirs:
            continue
        seen_dirs.add(normalized)
        unique_dirs.append(directory)

    prompt = (
        f"Read the bucket investigation brief at {brief_path}. "
        "Then inspect the referenced session files directly. This is a root-cause analysis task, "
        "not a summary task. Do not just restate the brief and do not answer with only session "
        "names. Recover repo or cwd attribution from session_meta or turn_context when needed. "
        "Reconstruct the causal timeline that led to the spike, identify the exact operator action, "
        "instruction, or workflow pattern that triggered it, and explain the mechanism of token "
        "burn. If the spike came from orchestration churn such as repeated wait_agent, send_input, "
        "spawn_agent, or context reload loops, say that explicitly. Start the report with a line "
        "in the form 'Root cause: <one sentence>'. Then write markdown sections titled Trigger, "
        "Mechanism, Evidence, Avoidance, and Confidence. Under Trigger, name the triggering action "
        "or say 'Root cause not proven' if the evidence is insufficient. Under Evidence, cite the "
        "session path(s), timestamps, and the key counts that support the conclusion. Under "
        "Avoidance, give the concrete operator behavior change that would have prevented the spike."
    )
    script_parts = [
        f"Set-Location -LiteralPath {_powershell_quote(str(workspace_root))}",
        "&",
        _powershell_quote(codex_executable),
        "exec",
        "--dangerously-bypass-approvals-and-sandbox",
        "--skip-git-repo-check",
        "-C",
        _powershell_quote(str(workspace_root)),
        "-o",
        _powershell_quote(str(report_path)),
    ]
    for directory in unique_dirs:
        script_parts.extend(["--add-dir", _powershell_quote(str(directory))])
    script_parts.append(_powershell_quote(prompt))
    script = "; ".join(
        [
            script_parts[0],
            " ".join(script_parts[1:]),
            f"Write-Host ''",
            f"Write-Host 'Investigation report:' {_powershell_quote(str(report_path))}",
        ]
    )
    return ["powershell.exe", "-NoExit", "-Command", script]
