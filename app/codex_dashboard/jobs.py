from __future__ import annotations

import json
import re
import subprocess
import tempfile
from collections import Counter
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Any

from .paths import default_jobs_registry_path, default_jobs_schema_path, legacy_jobs_registry_path
from .startup import startup_command, startup_script_path

REGISTRY_SCHEMA_VERSION = 1
JOB_KIND_SCHEDULED_TASK = "scheduled_task"
JOB_KIND_STARTUP_LAUNCHER = "startup_launcher"
JOB_STATUS_BLOCKED = "blocked"
JOB_STATUS_DISABLED = "disabled"
JOB_STATUS_DRIFTED = "drifted"
JOB_STATUS_IN_SYNC = "in_sync"
JOB_STATUS_MISSING = "missing"
DESIRED_STATE_DISABLED = "disabled"
DESIRED_STATE_ENABLED = "enabled"
POWERSHELL_CREATION_FLAGS = getattr(subprocess, "CREATE_NO_WINDOW", 0)


def jobs_registry_schema_payload() -> dict[str, Any]:
    return {
        "$schema": "https://json-schema.org/draft/2020-12/schema",
        "title": "Declared Codex Jobs",
        "type": "object",
        "additionalProperties": False,
        "required": ["schema_version", "updated_at", "jobs"],
        "properties": {
            "schema_version": {"const": REGISTRY_SCHEMA_VERSION},
            "updated_at": {"type": "string"},
            "jobs": {
                "type": "array",
                "items": {
                    "type": "object",
                    "additionalProperties": True,
                    "required": ["job_id", "label", "kind", "desired_state", "definition"],
                    "properties": {
                        "job_id": {"type": "string"},
                        "label": {"type": "string"},
                        "kind": {"enum": [JOB_KIND_SCHEDULED_TASK, JOB_KIND_STARTUP_LAUNCHER]},
                        "desired_state": {"enum": [DESIRED_STATE_ENABLED, DESIRED_STATE_DISABLED]},
                        "definition": {"type": "object"},
                    },
                },
            },
        },
    }


@dataclass(slots=True)
class JobObservation:
    exists: bool
    enabled: bool | None
    fingerprint: str | None
    details: dict[str, Any] = field(default_factory=dict)
    blocked_reason: str | None = None


def _now_timestamp() -> str:
    return datetime.now().astimezone().isoformat(timespec="seconds")


def _normalize_text(value: str) -> str:
    return "\n".join(line.rstrip() for line in value.replace("\r\n", "\n").strip().split("\n"))


def _slugify(raw_value: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", raw_value.lower()).strip("-")
    return slug or "job"


def _powershell_literal(raw_value: str) -> str:
    return raw_value.replace("'", "''")


def _powershell_startupinfo():
    startupinfo_ctor = getattr(subprocess, "STARTUPINFO", None)
    if startupinfo_ctor is None:
        return None
    startupinfo = startupinfo_ctor()
    startupinfo.dwFlags |= getattr(subprocess, "STARTF_USESHOWWINDOW", 0)
    startupinfo.wShowWindow = 0
    return startupinfo


def _run_powershell(script: str) -> str:
    completed = subprocess.run(
        ["powershell", "-NoProfile", "-Command", script],
        capture_output=True,
        text=True,
        encoding="utf-8",
        creationflags=POWERSHELL_CREATION_FLAGS,
        startupinfo=_powershell_startupinfo(),
        check=False,
    )
    if completed.returncode != 0:
        stderr = completed.stderr.strip() or completed.stdout.strip() or "unknown PowerShell failure"
        raise RuntimeError(stderr)
    return completed.stdout


def _run_powershell_json(script: str) -> list[dict[str, Any]]:
    raw_output = _run_powershell(script).strip()
    if not raw_output:
        return []
    payload = json.loads(raw_output)
    if isinstance(payload, list):
        return payload
    if isinstance(payload, dict):
        return [payload]
    raise ValueError("PowerShell JSON payload must be an object or array")


def schema_path_for_registry(registry_path: Path) -> Path:
    return registry_path.with_name(f"{registry_path.stem}.schema.json")


def jobs_registry_payload(jobs: list[dict[str, Any]], updated_at: str | None = None) -> dict[str, Any]:
    return {
        "schema_version": REGISTRY_SCHEMA_VERSION,
        "updated_at": updated_at or _now_timestamp(),
        "jobs": jobs,
    }


def load_jobs_registry(
    path: Path | None = None,
    codex_root: Path | None = None,
) -> dict[str, Any]:
    registry_path = path or default_jobs_registry_path(codex_root)
    return json.loads(registry_path.read_text(encoding="utf-8"))


def save_jobs_registry_schema(
    path: Path | None = None,
    codex_root: Path | None = None,
) -> Path:
    schema_path = path or default_jobs_schema_path(codex_root)
    schema_path.parent.mkdir(parents=True, exist_ok=True)
    schema_path.write_text(
        json.dumps(jobs_registry_schema_payload(), indent=2) + "\n",
        encoding="utf-8",
    )
    return schema_path


def save_jobs_registry(
    registry: dict[str, Any],
    path: Path | None = None,
    codex_root: Path | None = None,
) -> Path:
    registry_path = path or default_jobs_registry_path(codex_root)
    registry_path.parent.mkdir(parents=True, exist_ok=True)
    save_jobs_registry_schema(schema_path_for_registry(registry_path))
    registry_path.write_text(
        json.dumps(registry, indent=2) + "\n",
        encoding="utf-8",
    )
    return registry_path


def dashboard_startup_job() -> dict[str, Any]:
    return {
        "job_id": "codex-dashboard-startup",
        "label": "Obsidian overlay at sign-in",
        "kind": JOB_KIND_STARTUP_LAUNCHER,
        "desired_state": DESIRED_STATE_ENABLED,
        "definition": {
            "script_path": str(startup_script_path()),
            "command_text": startup_command(),
        },
    }


def _scheduled_task_job_id(task_name: str, task_path: str) -> str:
    normalized_path = task_path.replace("\\", "-").replace("/", "-")
    return _slugify(f"{normalized_path}-{task_name}")


def _discover_bootstrap_scheduled_task_definitions() -> list[dict[str, Any]]:
    script = """
$tasks = Get-ScheduledTask | Where-Object { $_.TaskName -like 'Codex*' }
$items = foreach ($task in $tasks) {
  $info = Get-ScheduledTaskInfo -TaskName $task.TaskName -TaskPath $task.TaskPath
  [pscustomobject]@{
    task_name = $task.TaskName
    task_path = $task.TaskPath
    enabled = [bool]$task.Settings.Enabled
    state = [string]$task.State
    last_task_result = [int]$info.LastTaskResult
    last_run_time = if ($info.LastRunTime -and $info.LastRunTime -ne [datetime]::MinValue) { $info.LastRunTime.ToString('o') } else { $null }
    next_run_time = if ($info.NextRunTime -and $info.NextRunTime -ne [datetime]::MinValue) { $info.NextRunTime.ToString('o') } else { $null }
    description = $task.Description
    author = $task.Author
    task_xml = (Export-ScheduledTask -TaskName $task.TaskName -TaskPath $task.TaskPath | Out-String)
  }
}
if (@($items).Count -eq 0) {
  '[]'
} else {
  @($items) | ConvertTo-Json -Depth 6 -Compress
}
"""
    scheduled_tasks = _run_powershell_json(script)
    jobs: list[dict[str, Any]] = []
    for item in scheduled_tasks:
        task_name = str(item["task_name"])
        task_path = str(item.get("task_path") or "\\")
        jobs.append(
            {
                "job_id": _scheduled_task_job_id(task_name, task_path),
                "label": task_name,
                "kind": JOB_KIND_SCHEDULED_TASK,
                "desired_state": DESIRED_STATE_ENABLED,
                "definition": {
                    "task_name": task_name,
                    "task_path": task_path,
                    "task_xml": str(item.get("task_xml") or ""),
                },
            }
        )
    return jobs


def bootstrap_jobs_registry(
    path: Path | None = None,
    codex_root: Path | None = None,
) -> dict[str, Any]:
    jobs = [dashboard_startup_job()]
    jobs.extend(_discover_bootstrap_scheduled_task_definitions())
    registry = jobs_registry_payload(jobs)
    save_jobs_registry(registry, path=path, codex_root=codex_root)
    return registry


def ensure_jobs_registry(
    path: Path | None = None,
    codex_root: Path | None = None,
) -> dict[str, Any]:
    registry_path = path or default_jobs_registry_path(codex_root)
    save_jobs_registry_schema(schema_path_for_registry(registry_path))
    if registry_path.exists():
        return load_jobs_registry(registry_path)
    legacy_path = legacy_jobs_registry_path(codex_root)
    if legacy_path.exists():
        registry = load_jobs_registry(legacy_path)
        save_jobs_registry(registry, path=registry_path)
        return registry
    return bootstrap_jobs_registry(registry_path)


def _startup_observation(job: dict[str, Any]) -> JobObservation:
    definition = job["definition"]
    script_path = Path(definition["script_path"])
    if not script_path.exists():
        return JobObservation(
            exists=False,
            enabled=None,
            fingerprint=None,
            details={"script_path": str(script_path)},
        )
    try:
        command_text = script_path.read_text(encoding="utf-8")
    except OSError as exc:
        return JobObservation(
            exists=True,
            enabled=None,
            fingerprint=None,
            details={"script_path": str(script_path)},
            blocked_reason=f"Startup launcher could not be read: {exc}",
        )
    return JobObservation(
        exists=True,
        enabled=True,
        fingerprint=_normalize_text(command_text),
        details={
            "script_path": str(script_path),
            "command_text": command_text,
        },
    )


def _scheduled_task_observation(job: dict[str, Any]) -> JobObservation:
    definition = job["definition"]
    task_name = str(definition["task_name"])
    task_path = str(definition.get("task_path") or "\\")
    script = f"""
$task = Get-ScheduledTask -TaskName '{_powershell_literal(task_name)}' -TaskPath '{_powershell_literal(task_path)}' -ErrorAction SilentlyContinue
if ($null -eq $task) {{
  '[]'
  exit 0
}}
$info = Get-ScheduledTaskInfo -TaskName $task.TaskName -TaskPath $task.TaskPath
$item = [pscustomobject]@{{
  task_name = $task.TaskName
  task_path = $task.TaskPath
  enabled = [bool]$task.Settings.Enabled
  state = [string]$task.State
  last_task_result = [int]$info.LastTaskResult
  last_run_time = if ($info.LastRunTime -and $info.LastRunTime -ne [datetime]::MinValue) {{ $info.LastRunTime.ToString('o') }} else {{ $null }}
  next_run_time = if ($info.NextRunTime -and $info.NextRunTime -ne [datetime]::MinValue) {{ $info.NextRunTime.ToString('o') }} else {{ $null }}
  task_xml = (Export-ScheduledTask -TaskName $task.TaskName -TaskPath $task.TaskPath | Out-String)
}}
@($item) | ConvertTo-Json -Depth 6 -Compress
"""
    try:
        items = _run_powershell_json(script)
    except (RuntimeError, ValueError, json.JSONDecodeError) as exc:
        return JobObservation(
            exists=False,
            enabled=None,
            fingerprint=None,
            details={
                "task_name": task_name,
                "task_path": task_path,
            },
            blocked_reason=f"Scheduled Task state could not be read: {exc}",
        )
    if not items:
        return JobObservation(
            exists=False,
            enabled=None,
            fingerprint=None,
            details={
                "task_name": task_name,
                "task_path": task_path,
            },
        )
    item = items[0]
    task_xml = str(item.get("task_xml") or "")
    return JobObservation(
        exists=True,
        enabled=bool(item.get("enabled")),
        fingerprint=_normalize_text(task_xml),
        details={
            "task_name": task_name,
            "task_path": task_path,
            "state": item.get("state"),
            "last_task_result": item.get("last_task_result"),
            "last_run_time": item.get("last_run_time"),
            "next_run_time": item.get("next_run_time"),
            "task_xml": task_xml,
        },
    )


def observe_job(job: dict[str, Any]) -> JobObservation:
    kind = job["kind"]
    if kind == JOB_KIND_STARTUP_LAUNCHER:
        return _startup_observation(job)
    if kind == JOB_KIND_SCHEDULED_TASK:
        return _scheduled_task_observation(job)
    return JobObservation(
        exists=False,
        enabled=None,
        fingerprint=None,
        details={},
        blocked_reason=f"Unsupported job kind: {kind}",
    )


def reconcile_job(job: dict[str, Any], observation: JobObservation | None = None) -> dict[str, Any]:
    observed = observation or observe_job(job)
    desired_state = job["desired_state"]
    definition = job["definition"]
    expected_fingerprint = None
    if job["kind"] == JOB_KIND_STARTUP_LAUNCHER:
        expected_fingerprint = _normalize_text(str(definition["command_text"]))
    elif job["kind"] == JOB_KIND_SCHEDULED_TASK:
        expected_fingerprint = _normalize_text(str(definition["task_xml"]))

    if observed.blocked_reason:
        status = JOB_STATUS_BLOCKED
        reason = observed.blocked_reason
    elif desired_state == DESIRED_STATE_ENABLED:
        if not observed.exists:
            status = JOB_STATUS_MISSING
            reason = "Job is missing from durable Windows state."
        elif observed.enabled is False:
            status = JOB_STATUS_DISABLED
            reason = "Job exists but is disabled."
        elif observed.fingerprint != expected_fingerprint:
            status = JOB_STATUS_DRIFTED
            reason = "Observed durable state differs from the managed definition."
        else:
            status = JOB_STATUS_IN_SYNC
            reason = "Job matches the managed definition."
    else:
        if not observed.exists:
            status = JOB_STATUS_IN_SYNC
            reason = "Job is absent as desired."
        elif observed.enabled is False:
            status = JOB_STATUS_DISABLED
            reason = "Job exists in a disabled state."
        else:
            status = JOB_STATUS_DRIFTED
            reason = "Job remains enabled even though the desired state is disabled."

    observed_label = "Missing"
    if observed.blocked_reason:
        observed_label = "Blocked"
    elif observed.exists and observed.enabled is False:
        observed_label = "Disabled"
    elif observed.exists:
        observed_label = "Enabled"

    mechanism_label = "Startup launcher"
    if job["kind"] == JOB_KIND_SCHEDULED_TASK:
        mechanism_label = "Scheduled Task"

    return {
        "job_id": job["job_id"],
        "label": job["label"],
        "kind": job["kind"],
        "mechanism_label": mechanism_label,
        "desired_state": desired_state,
        "desired_label": "Enabled" if desired_state == DESIRED_STATE_ENABLED else "Disabled",
        "observed_label": observed_label,
        "status": status,
        "reason": reason,
        "definition": dict(definition),
        "details": dict(observed.details),
    }


def reconcile_registry(registry: dict[str, Any]) -> dict[str, Any]:
    jobs = [reconcile_job(job) for job in registry.get("jobs", [])]
    counts = Counter(job["status"] for job in jobs)
    return {
        "last_reconciled_at": _now_timestamp(),
        "summary": dict(counts),
        "jobs": jobs,
    }


def _apply_startup_job(job: dict[str, Any]) -> None:
    definition = job["definition"]
    script_path = Path(definition["script_path"])
    desired_state = job["desired_state"]
    if desired_state == DESIRED_STATE_DISABLED:
        if script_path.exists():
            script_path.unlink()
        return
    script_path.parent.mkdir(parents=True, exist_ok=True)
    with script_path.open("w", encoding="utf-8", newline="") as handle:
        handle.write(str(definition["command_text"]))


def _apply_scheduled_task_job(job: dict[str, Any]) -> None:
    definition = job["definition"]
    task_name = str(definition["task_name"])
    task_path = str(definition.get("task_path") or "\\")
    if job["desired_state"] == DESIRED_STATE_DISABLED:
        script = f"""
$task = Get-ScheduledTask -TaskName '{_powershell_literal(task_name)}' -TaskPath '{_powershell_literal(task_path)}' -ErrorAction SilentlyContinue
if ($null -ne $task) {{
  Disable-ScheduledTask -TaskName $task.TaskName -TaskPath $task.TaskPath | Out-Null
}}
"""
        _run_powershell(script)
        return

    with tempfile.NamedTemporaryFile("w", encoding="utf-8", delete=False, suffix=".xml") as handle:
        handle.write(str(definition["task_xml"]))
        xml_path = Path(handle.name)
    try:
        script = f"""
$xml = Get-Content -Raw -LiteralPath '{_powershell_literal(str(xml_path))}'
Register-ScheduledTask -TaskName '{_powershell_literal(task_name)}' -TaskPath '{_powershell_literal(task_path)}' -Xml $xml -Force | Out-Null
Enable-ScheduledTask -TaskName '{_powershell_literal(task_name)}' -TaskPath '{_powershell_literal(task_path)}' | Out-Null
"""
        _run_powershell(script)
    finally:
        xml_path.unlink(missing_ok=True)


def apply_job(job: dict[str, Any]) -> dict[str, Any]:
    if job["kind"] == JOB_KIND_STARTUP_LAUNCHER:
        _apply_startup_job(job)
    elif job["kind"] == JOB_KIND_SCHEDULED_TASK:
        _apply_scheduled_task_job(job)
    else:
        raise ValueError(f"Unsupported job kind: {job['kind']}")
    return reconcile_job(job)


def apply_registry(registry: dict[str, Any]) -> dict[str, Any]:
    jobs = [apply_job(job) for job in registry.get("jobs", [])]
    counts = Counter(job["status"] for job in jobs)
    return {
        "last_reconciled_at": _now_timestamp(),
        "summary": dict(counts),
        "jobs": jobs,
    }


def set_job_desired_state(
    registry: dict[str, Any],
    job_id: str,
    desired_state: str,
) -> dict[str, Any]:
    if desired_state not in {DESIRED_STATE_DISABLED, DESIRED_STATE_ENABLED}:
        raise ValueError(f"Unsupported desired state: {desired_state}")

    updated_jobs: list[dict[str, Any]] = []
    found = False
    for job in registry.get("jobs", []):
        updated_job = dict(job)
        if updated_job.get("job_id") == job_id:
            updated_job["desired_state"] = desired_state
            found = True
        updated_jobs.append(updated_job)

    if not found:
        raise KeyError(f"Unknown job id: {job_id}")

    updated_registry = dict(registry)
    updated_registry["updated_at"] = _now_timestamp()
    updated_registry["jobs"] = updated_jobs
    return updated_registry
