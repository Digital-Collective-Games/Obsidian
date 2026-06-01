from __future__ import annotations

import json
import os
from typing import Any
from urllib import error, parse, request


DEFAULT_WORKTREES_BACKEND_URL = "http://127.0.0.1:4318"
WORKTREES_BACKEND_URL_ENV = "CODEX_DASHBOARD_WORKTREES_BACKEND_URL"
# The worktrees tab also reads the open-task list and the repo registry off the same
# backend. When only the shared tasks-backend override is set (e.g. a regression lane
# pointing CODEX_DASHBOARD_TASKS_BACKEND_URL at the validation backend), follow it so a
# single override moves the whole WORKTREES tab onto the isolated lane.
TASKS_BACKEND_URL_ENV = "CODEX_DASHBOARD_TASKS_BACKEND_URL"
REQUEST_TIMEOUT_SECONDS = 30.0


class WorktreesBackendError(RuntimeError):
    pass


def configured_worktrees_backend_url() -> str:
    configured = os.environ.get(WORKTREES_BACKEND_URL_ENV, "").strip()
    if configured:
        return configured
    shared = os.environ.get(TASKS_BACKEND_URL_ENV, "").strip()
    if shared:
        return shared
    return DEFAULT_WORKTREES_BACKEND_URL


def fetch_pool_snapshot(base_url: str = DEFAULT_WORKTREES_BACKEND_URL) -> dict[str, object]:
    payload = _request_json("GET", _join_url(base_url, "/api/v1/worktrees"))
    return map_backend_pool_snapshot(payload)


def fetch_repos(base_url: str = DEFAULT_WORKTREES_BACKEND_URL) -> list[dict[str, object]]:
    payload = _request_json("GET", _join_url(base_url, "/api/v1/repos"))
    return map_backend_repos(payload)


def create_worktree(repo: str, base_url: str = DEFAULT_WORKTREES_BACKEND_URL) -> dict[str, Any]:
    return _request_json(
        "POST",
        _join_url(base_url, "/api/v1/worktrees/create"),
        body={"repo": repo},
    )


def assign_worktree(
    task_id: str,
    repo: str,
    worktree_id: str,
    base_url: str = DEFAULT_WORKTREES_BACKEND_URL,
) -> dict[str, Any]:
    return _request_json(
        "POST",
        _join_url(base_url, "/api/v1/worktrees/assign"),
        body={"task_id": task_id, "repo": repo, "worktree_id": worktree_id},
    )


def eject_worktree(
    run_id: str,
    worktree_id: str = "",
    base_url: str = DEFAULT_WORKTREES_BACKEND_URL,
) -> dict[str, Any]:
    body: dict[str, str] = {"run_id": run_id}
    if worktree_id:
        body["worktree_id"] = worktree_id
    return _request_json("POST", _join_url(base_url, "/api/v1/worktrees/eject"), body=body)


def destroy_worktree(worktree_id: str, base_url: str = DEFAULT_WORKTREES_BACKEND_URL) -> dict[str, Any]:
    return _request_json(
        "POST",
        _join_url(base_url, "/api/v1/worktrees/destroy"),
        body={"worktree_id": worktree_id},
    )


def dequeue_task(repo: str, task_id: str, base_url: str = DEFAULT_WORKTREES_BACKEND_URL) -> dict[str, Any]:
    return _request_json(
        "POST",
        _join_url(base_url, "/api/v1/worktrees/dequeue"),
        body={"repo": repo, "task_id": task_id},
    )


def worktrees_backend_error_snapshot(message: str) -> dict[str, object]:
    return {
        "status": "backend_unavailable",
        "worktrees": [],
        "message": message,
    }


def map_backend_pool_snapshot(payload: dict[str, Any]) -> dict[str, object]:
    worktrees_payload = payload.get("worktrees", [])
    if not isinstance(worktrees_payload, list):
        raise WorktreesBackendError("Worktrees backend payload did not contain a worktrees list.")
    worktrees = [
        map_backend_worktree(item)
        for item in worktrees_payload
        if isinstance(item, dict)
    ]
    return {
        "status": _text(payload.get("status"), default="ok"),
        "worktrees": worktrees,
        "message": _text(
            payload.get("message"),
            default="Worktree pool loaded from orchestration backend.",
        ),
    }


def map_backend_worktree(worktree_payload: dict[str, Any]) -> dict[str, object]:
    status = _text(worktree_payload.get("status"), default="idle") or "idle"
    return {
        "worktree_id": _text(worktree_payload.get("worktree_id"), default="") or "",
        "status": status,
        "repo": _text(worktree_payload.get("repo"), default="") or "",
        "worktree_path": _text(worktree_payload.get("worktree_path"), default="") or "",
        "run_id": _text(worktree_payload.get("run_id"), default="") or "",
        "task_id": _text(worktree_payload.get("task_id"), default="") or "",
        "run_gate_state": _text(worktree_payload.get("run_gate_state"), default="") or "",
        "agent_session_id": _text(worktree_payload.get("agent_session_id"), default="") or "",
        "session_transcript_path": _text(worktree_payload.get("session_transcript_path"), default="") or "",
        "launched_pid": _int(worktree_payload.get("launched_pid")),
    }


def map_backend_repos(payload: dict[str, Any]) -> list[dict[str, object]]:
    repos_payload = payload.get("repos", [])
    if not isinstance(repos_payload, list):
        raise WorktreesBackendError("Repos backend payload did not contain a repos list.")
    repos: list[dict[str, object]] = []
    for item in repos_payload:
        if not isinstance(item, dict):
            continue
        repo_id = _text(item.get("id"), default="") or ""
        if not repo_id:
            continue
        repos.append(
            {
                "id": repo_id,
                "local_root": _text(item.get("local_root"), default="") or "",
                "task_provider_repo": _text(item.get("task_provider_repo"), default="") or "",
            }
        )
    return repos


def _request_json(method: str, url: str, body: dict[str, Any] | None = None) -> dict[str, Any]:
    data: bytes | None = None
    headers = {"Accept": "application/json"}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = request.Request(url, data=data, method=method, headers=headers)
    try:
        with request.urlopen(req, timeout=REQUEST_TIMEOUT_SECONDS) as response:
            raw = response.read().decode("utf-8")
    except error.HTTPError as exc:
        raw_body = exc.read().decode("utf-8", errors="replace").strip()
        raise WorktreesBackendError(
            f"{method} {url} failed with HTTP {exc.code}: {_extract_error_message(raw_body) or exc.reason}"
        ) from exc
    except OSError as exc:
        raise WorktreesBackendError(_format_request_os_error(method, url, exc)) from exc

    if not raw.strip():
        return {}
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise WorktreesBackendError(f"{method} {url} returned invalid JSON: {exc}") from exc
    if not isinstance(payload, dict):
        raise WorktreesBackendError(f"{method} {url} returned an unexpected JSON payload.")
    return payload


def _extract_error_message(raw_body: str) -> str:
    if not raw_body:
        return ""
    try:
        parsed = json.loads(raw_body)
    except json.JSONDecodeError:
        return raw_body
    if isinstance(parsed, dict):
        message = parsed.get("error") or parsed.get("message")
        if isinstance(message, str) and message.strip():
            return message.strip()
    return raw_body


def _format_request_os_error(method: str, url: str, exc: OSError) -> str:
    detail = str(exc)
    normalized = detail.lower()
    origin = _request_origin(url)
    if "10061" in detail or "actively refused it" in normalized or "connection refused" in normalized:
        return (
            f"{method} {url} failed: the worktrees backend is not reachable at {origin}. "
            f"Start the orchestration backend or set {WORKTREES_BACKEND_URL_ENV} to a running isolated lane."
        )
    if "timed out" in normalized:
        return f"{method} {url} failed: the worktrees backend at {origin} timed out."
    return f"{method} {url} failed: {exc}"


def _join_url(base_url: str, path: str) -> str:
    return base_url.rstrip("/") + path


def _request_origin(url: str) -> str:
    parsed = parse.urlsplit(url)
    if parsed.scheme and parsed.netloc:
        return f"{parsed.scheme}://{parsed.netloc}"
    return url


def _text(value: Any, default: str | None = None) -> str | None:
    if value is None:
        return default
    text = str(value).strip()
    if text == "":
        return default
    return text


def _int(value: Any) -> int:
    try:
        return int(value)
    except (TypeError, ValueError):
        return 0
