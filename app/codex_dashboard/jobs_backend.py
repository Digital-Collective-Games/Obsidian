from __future__ import annotations

import json
import os
from collections import Counter
from datetime import datetime
from typing import Any
from urllib import error, parse, request

DEFAULT_JOBS_BACKEND_URL = "http://127.0.0.1:4318"
JOBS_BACKEND_URL_ENV = "CODEX_DASHBOARD_JOBS_BACKEND_URL"
REQUEST_TIMEOUT_SECONDS = 10.0


class JobsBackendError(RuntimeError):
    pass


def configured_jobs_backend_url() -> str:
    configured = os.environ.get(JOBS_BACKEND_URL_ENV, "").strip()
    if configured:
        return configured
    return DEFAULT_JOBS_BACKEND_URL


def fetch_jobs_snapshot(base_url: str = DEFAULT_JOBS_BACKEND_URL) -> dict[str, object]:
    payload = _request_json("GET", _join_url(base_url, "/api/v1/jobs"))
    return map_state_view_to_jobs_snapshot(payload)


def sync_jobs_snapshot(
    base_url: str = DEFAULT_JOBS_BACKEND_URL,
) -> tuple[dict[str, object], dict[str, Any]]:
    report = _request_json("POST", _join_url(base_url, "/sync"))
    state = report.get("state", {})
    if not isinstance(state, dict):
        raise JobsBackendError("Sync response did not include a valid state payload.")
    return map_state_view_to_jobs_snapshot(state), report


def start_job_run(job_id: str, base_url: str = DEFAULT_JOBS_BACKEND_URL) -> dict[str, Any]:
    encoded_job_id = parse.quote(job_id, safe="")
    payload = _request_json("POST", _join_url(base_url, f"/api/v1/jobs/{encoded_job_id}/run"))
    if not isinstance(payload, dict):
        raise JobsBackendError("Run-now response was not a JSON object.")
    return payload


def jobs_backend_error_snapshot(message: str) -> dict[str, object]:
    return {
        "last_reconciled_at": None,
        "summary": {"blocked": 1},
        "jobs": [
            {
                "job_id": "jobs-backend-blocked",
                "label": "Jobs backend",
                "kind": "orchestration_backend",
                "mechanism_label": "Backend",
                "desired_state": "enabled",
                "desired_label": "Enabled",
                "observed_label": "Blocked",
                "status": "blocked",
                "reason": message,
                "definition": {"error": message},
                "supports_run_now": False,
            }
        ],
    }


def map_state_view_to_jobs_snapshot(payload: dict[str, Any]) -> dict[str, object]:
    jobs_payload = payload.get("jobs", [])
    if not isinstance(jobs_payload, list):
        raise JobsBackendError("Jobs backend payload did not contain a jobs list.")

    jobs = [map_backend_job(job_payload) for job_payload in jobs_payload if isinstance(job_payload, dict)]
    summary = Counter(str(job.get("status", "unknown")) for job in jobs)
    last_sync = payload.get("last_sync", {})
    last_reconciled_at = None
    if isinstance(last_sync, dict):
        last_reconciled_at = _text(last_sync.get("last_success_at"))
    return {
        "generated_at": _text(payload.get("generated_at")),
        "last_reconciled_at": last_reconciled_at,
        "summary": dict(summary),
        "jobs": jobs,
    }


def map_backend_job(job_payload: dict[str, Any]) -> dict[str, object]:
    status = _text(job_payload.get("status"), default="unknown")
    desired_state = _text(job_payload.get("desired_state"), default="enabled")
    triggers = _dict_list(job_payload.get("triggers"))
    schedules = _dict_list(job_payload.get("schedules"))
    recent_runs = _dict_list(job_payload.get("recent_runs"))
    latest_run = recent_runs[0] if recent_runs else _latest_schedule_run(schedules)
    next_run_at = _next_schedule_time(schedules)
    supports_run_now = any(_text(trigger.get("type")) == "manual" for trigger in triggers)

    return {
        "job_id": _text(job_payload.get("job_id"), default="unknown-job"),
        "label": _text(job_payload.get("label"), default="Unnamed job"),
        "kind": "orchestration_backend",
        "mechanism_label": trigger_label(triggers),
        "desired_state": desired_state,
        "desired_label": "Enabled" if desired_state == "enabled" else "Disabled",
        "observed_label": observed_status_label(status),
        "status": status,
        "reason": reason_text(
            status=status,
            next_run_at=next_run_at,
            latest_run=latest_run,
            supports_run_now=supports_run_now,
            schedules=schedules,
        ),
        "definition": dict(job_payload),
        "supports_run_now": supports_run_now,
    }


def trigger_label(triggers: list[dict[str, Any]]) -> str:
    labels: list[str] = []
    trigger_types = {_text(trigger.get("type")) for trigger in triggers}
    if "schedule" in trigger_types:
        labels.append("Schedule")
    if "manual" in trigger_types:
        labels.append("Manual")
    if "webhook" in trigger_types:
        labels.append("Webhook")
    if not labels:
        return "Backend"
    return " + ".join(labels)


def observed_status_label(status: str) -> str:
    labels = {
        "in_sync": "In sync",
        "drifted": "Drifted",
        "missing": "Missing",
        "disabled": "Disabled",
        "blocked": "Blocked",
        "unknown": "Unknown",
    }
    return labels.get(status, status.replace("_", " ").title())


def reason_text(
    *,
    status: str,
    next_run_at: str | None,
    latest_run: dict[str, Any] | None,
    supports_run_now: bool,
    schedules: list[dict[str, Any]],
) -> str:
    if status == "blocked":
        note = _first_schedule_note(schedules)
        return note or "The orchestration backend reported a blocked state."
    if status == "missing":
        return "Desired job is missing from the orchestration runtime."
    if status == "drifted":
        note = _first_schedule_note(schedules)
        if note:
            return f"Git spec and Temporal differ. {note}"
        return "Git spec and the Temporal schedule differ; UPDATE re-applies the Git spec."
    if status == "disabled":
        return "Desired state is disabled."

    parts: list[str] = []
    if next_run_at:
        parts.append(f"Next run {local_clock_label(next_run_at)}")
    latest_run_at = _text((latest_run or {}).get("actual_time")) or _text((latest_run or {}).get("schedule_time"))
    if latest_run_at:
        parts.append(f"Last run {local_clock_label(latest_run_at)}")
    if supports_run_now:
        parts.append("Run now available")
    if parts:
        return ". ".join(parts) + "."
    return "Backend state is current."


def local_clock_label(raw_value: str) -> str:
    parsed = _parse_timestamp(raw_value)
    return parsed.astimezone().strftime("%I:%M %p").lstrip("0")


def relative_time_label(raw_value: str, now: datetime | None = None) -> str:
    """A compact "2h 14m ago" / "just now" / "in 5m" label for a timestamp (jobs LAST_RUN /
    next-run). `now` is injectable for tests."""
    parsed = _parse_timestamp(raw_value)
    if now is None:
        now = datetime.now(parsed.tzinfo) if parsed.tzinfo else datetime.now()
    seconds = int((now - parsed).total_seconds())
    future = seconds < 0
    seconds = abs(seconds)
    if seconds < 45:
        return "just now"
    if seconds >= 86400:
        label = f"{seconds // 86400}d {(seconds % 86400) // 3600}h"
    elif seconds >= 3600:
        label = f"{seconds // 3600}h {(seconds % 3600) // 60}m"
    else:
        label = f"{max(1, seconds // 60)}m"
    return f"in {label}" if future else f"{label} ago"


def cron_human(cron: str) -> str:
    """Plain-English cadence for a standard 5-field cron expression, or "" if unrecognized."""
    parts = str(cron or "").split()
    if len(parts) != 5:
        return ""
    minute, hour, dom, mon, dow = parts
    every_day = dom == "*" and mon == "*" and dow == "*"
    if every_day and hour == "*" and minute.startswith("*/") and minute[2:].isdigit():
        return f"Every {minute[2:]} minutes"
    if every_day and hour == "*" and minute == "0":
        return "Hourly"
    if every_day and minute.isdigit() and hour.isdigit():
        h, m = int(hour), int(minute)
        return f"Daily at {(h % 12) or 12}:{m:02d} {'AM' if h < 12 else 'PM'}"
    return ""


def job_schedule_display(job: dict[str, Any]) -> tuple[str, str]:
    """(primary, detail) for the SCHEDULE column: the cron expression with a plain-English
    cadence beneath it when available, falling back to the trigger mechanism + next run."""
    definition = job.get("definition", {})
    definition = definition if isinstance(definition, dict) else {}
    mechanism = _text(job.get("mechanism_label")) or "Backend"
    schedules = _dict_list(definition.get("schedules"))
    cron = ""
    for schedule in schedules:
        for key in ("cron", "spec", "calendar", "expression", "interval"):
            value = _text(schedule.get(key))
            if value:
                cron = value
                break
        if cron:
            break
    if cron:
        return cron, (cron_human(cron) or mechanism)
    next_run = _next_schedule_time(schedules)
    if next_run:
        return mechanism, f"next {local_clock_label(next_run)}"
    return mechanism, ""


def job_next_run_display(job: dict[str, Any]) -> str:
    """The next scheduled run as a clock time ("4:00 AM"), or "—" when there is no upcoming
    scheduled run (e.g. a manual-only job)."""
    definition = job.get("definition", {})
    definition = definition if isinstance(definition, dict) else {}
    next_run = _next_schedule_time(_dict_list(definition.get("schedules")))
    return local_clock_label(next_run) if next_run else "—"


def job_is_running(job: dict[str, Any]) -> bool:
    """Whether the job's most recent run is still in flight (so Run-now should be disabled)."""
    definition = job.get("definition", {})
    definition = definition if isinstance(definition, dict) else {}
    recent = _dict_list(definition.get("recent_runs"))
    if not recent:
        latest = _latest_schedule_run(_dict_list(definition.get("schedules")))
        recent = [latest] if latest else []
    if not recent:
        return False
    status = (_text(recent[0].get("status")) or _text(recent[0].get("result")) or "").lower()
    return status == "running"


def job_last_run_display(job: dict[str, Any], now: datetime | None = None) -> tuple[str, str]:
    """(when, detail) for the LAST_RUN column from the backend's recent runs; ("Never run",
    "") when the job has no recorded run."""
    definition = job.get("definition", {})
    definition = definition if isinstance(definition, dict) else {}
    recent = _dict_list(definition.get("recent_runs"))
    if not recent:
        latest = _latest_schedule_run(_dict_list(definition.get("schedules")))
        recent = [latest] if latest else []
    if not recent:
        return "Never run", ""
    run = recent[0]
    stamp = _text(run.get("actual_time")) or _text(run.get("schedule_time"))
    when = relative_time_label(stamp, now=now) if stamp else "—"
    result = _text(run.get("status")) or _text(run.get("result"))
    detail = result.replace("_", " ").title() if result else ""
    return when, detail


JOB_STATUS_CHIPS = {
    # in-sync = cyan; drifted = amber/warning (diverged but present); missing/blocked = hard
    # error-red (absent / broken) — the two attention states are tellable apart by color.
    "in_sync": ("IN SYNC", "#16323a", "#7fdfe8"),
    "drifted": ("DRIFTED", "#3a1f22", "#ffc1bd"),
    "missing": ("MISSING", "#4a1418", "#ffb4ab"),
    "blocked": ("BLOCKED", "#4a1418", "#ffb4ab"),
    "disabled": ("DISABLED", "#31353c", "#adcbda"),
    "unknown": ("UNKNOWN", "#31353c", "#8fa8bb"),
}


def job_status_chip(status: str) -> tuple[str, str, str]:
    """(label, background, foreground) for a job's drift-status chip."""
    key = str(status or "").lower()
    if key in JOB_STATUS_CHIPS:
        return JOB_STATUS_CHIPS[key]
    return (key.replace("_", " ").replace("-", " ").upper() or "UNKNOWN", "#31353c", "#8fa8bb")


def schedule_owner_map(snapshot: dict[str, Any]) -> dict[str, str]:
    """Map each schedule id to its owning job id, so the SYNC_AUDIT "last UPDATE" lines can
    name jobs instead of raw schedule ids. Best-effort: only ids the backend exposes."""
    mapping: dict[str, str] = {}
    jobs = snapshot.get("jobs", [])
    for job in jobs if isinstance(jobs, list) else []:
        if not isinstance(job, dict):
            continue
        job_id = str(job.get("job_id") or "")
        definition = job.get("definition", {})
        definition = definition if isinstance(definition, dict) else {}
        for schedule in _dict_list(definition.get("schedules")):
            for key in ("id", "schedule_id", "schedule_handle", "name"):
                schedule_id = _text(schedule.get(key))
                if schedule_id:
                    mapping[schedule_id] = job_id
    return mapping


def jobs_attention_jobs(snapshot: dict[str, Any]) -> list[dict[str, Any]]:
    """The jobs that are NOT in sync (drift / missing / blocked / disabled) — i.e. exactly
    what an UPDATE (git -> Temporal apply) would reconcile. The SYNC_AUDIT panel lists these."""
    jobs = snapshot.get("jobs", [])
    jobs = jobs if isinstance(jobs, list) else []
    return [job for job in jobs if isinstance(job, dict) and str(job.get("status")) != "in_sync"]


def summarize_apply_report(report: dict[str, Any]) -> dict[str, int]:
    """Counts of schedule changes from a /sync apply report (what the last UPDATE changed)."""
    def count(field: str) -> int:
        value = report.get(field)
        return len(value) if isinstance(value, list) else 0

    created, updated, deleted = count("created_schedule_ids"), count("updated_schedule_ids"), count("deleted_schedule_ids")
    return {"created": created, "updated": updated, "deleted": deleted, "total": created + updated + deleted}


def job_detail_text(job: dict[str, Any]) -> str:
    """The per-job detail readout (key facts + the raw backend definition) shown in the
    Jobs row's Details popup."""
    lines = []
    for label, key in (
        ("Label", "label"),
        ("Status", "observed_label"),
        ("Desired", "desired_label"),
        ("Trigger", "mechanism_label"),
        ("Reason", "reason"),
    ):
        value = _text(job.get(key))
        if value:
            lines.append(f"{label}: {value}")
    definition = job.get("definition", {})
    definition = definition if isinstance(definition, dict) else {}
    return "\n".join(lines) + "\n\nDEFINITION\n" + json.dumps(definition, indent=2, sort_keys=True)


def _latest_schedule_run(schedules: list[dict[str, Any]]) -> dict[str, Any] | None:
    for schedule in schedules:
        recent_runs = _dict_list(schedule.get("recent_runs"))
        if recent_runs:
            return recent_runs[0]
    return None


def _next_schedule_time(schedules: list[dict[str, Any]]) -> str | None:
    for schedule in schedules:
        next_action_times = schedule.get("next_action_times", [])
        if isinstance(next_action_times, list) and next_action_times:
            return _text(next_action_times[0])
    return None


def _first_schedule_note(schedules: list[dict[str, Any]]) -> str | None:
    for schedule in schedules:
        note = _text(schedule.get("note"))
        if note:
            return note
    return None


def _request_json(method: str, url: str) -> dict[str, Any]:
    req = request.Request(
        url,
        method=method,
        headers={"Accept": "application/json"},
    )
    try:
        with request.urlopen(req, timeout=REQUEST_TIMEOUT_SECONDS) as response:
            raw = response.read().decode("utf-8")
    except error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace").strip()
        raise JobsBackendError(f"{method} {url} failed with HTTP {exc.code}: {body or exc.reason}") from exc
    except OSError as exc:
        raise JobsBackendError(_format_request_os_error(method, url, exc)) from exc

    try:
        payload = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise JobsBackendError(f"{method} {url} returned invalid JSON: {exc}") from exc
    if not isinstance(payload, dict):
        raise JobsBackendError(f"{method} {url} returned an unexpected JSON payload.")
    return payload


def _join_url(base_url: str, path: str) -> str:
    return base_url.rstrip("/") + path


def _format_request_os_error(method: str, url: str, exc: OSError) -> str:
    detail = str(exc)
    normalized = detail.lower()
    origin = _request_origin(url)

    if _is_connection_refused(detail):
        return (
            f"{method} {url} failed: the jobs backend is not reachable at {origin}. "
            f"{_jobs_backend_recovery_hint(origin)}"
        )
    if "timed out" in normalized:
        return f"{method} {url} failed: the jobs backend at {origin} timed out."
    return f"{method} {url} failed: {exc}"


def _request_origin(url: str) -> str:
    parsed = parse.urlsplit(url)
    if parsed.scheme and parsed.netloc:
        return f"{parsed.scheme}://{parsed.netloc}"
    return url


def _is_connection_refused(detail: str) -> bool:
    normalized = detail.lower()
    return (
        "10061" in detail
        or "actively refused it" in normalized
        or "connection refused" in normalized
    )


def _jobs_backend_recovery_hint(origin: str) -> str:
    parsed = parse.urlsplit(origin)
    host = (parsed.hostname or "").lower()
    port = parsed.port
    if host in {"127.0.0.1", "localhost", "::1"} and port == 4318:
        return (
            "Start the orchestration service lane or set "
            f"{JOBS_BACKEND_URL_ENV} to a running backend."
        )
    if host in {"127.0.0.1", "localhost", "::1"}:
        return f"Start the backend bound to {origin} or set {JOBS_BACKEND_URL_ENV} to a running backend."
    return "Check that the configured jobs backend is running and reachable."


def _dict_list(value: Any) -> list[dict[str, Any]]:
    if not isinstance(value, list):
        return []
    return [item for item in value if isinstance(item, dict)]


def _parse_timestamp(raw_value: str):
    return datetime.fromisoformat(raw_value.replace("Z", "+00:00"))


def _text(value: Any, default: str | None = None) -> str | None:
    if value is None:
        return default
    text = str(value).strip()
    if text == "":
        return default
    return text
