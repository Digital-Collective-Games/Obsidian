from __future__ import annotations

from typing import Iterable


# Allocated vs idle is the load-bearing visual distinction (Goal 13 / REG-010): the two
# row background colors must be perceivably different, reusing the app's dark cyan/navy
# palette. Allocated rows get a brighter navy-teal panel; idle rows stay the darker
# surface used elsewhere in the cockpit.
WORKTREE_STATUS_BACKGROUNDS = {
    "allocated": "#173a44",
    "idle": "#181c22",
}
WORKTREE_STATUS_DEFAULT_BACKGROUND = "#181c22"

# Status accent (left rail + chip foreground) per status.
WORKTREE_STATUS_COLORS = {
    "allocated": "#16d9f5",
    "idle": "#5ee69a",
}
WORKTREE_STATUS_DEFAULT_COLOR = "#8fa8bb"

# The synthetic "All repos" filter option key. The repo dropdown is otherwise sourced
# ENTIRELY from GET /api/v1/repos (the registry) — never a hardcoded repo list.
ALL_REPOS_OPTION = "All repos"


def is_allocated(worktree: dict[str, object]) -> bool:
    return str(worktree.get("status") or "").lower() == "allocated"


def worktree_status_background(worktree: dict[str, object]) -> str:
    status = str(worktree.get("status") or "").lower()
    return WORKTREE_STATUS_BACKGROUNDS.get(status, WORKTREE_STATUS_DEFAULT_BACKGROUND)


def worktree_status_color(worktree: dict[str, object]) -> str:
    status = str(worktree.get("status") or "").lower()
    return WORKTREE_STATUS_COLORS.get(status, WORKTREE_STATUS_DEFAULT_COLOR)


def worktree_status_label(worktree: dict[str, object]) -> str:
    status = str(worktree.get("status") or "").lower()
    if status == "allocated":
        gate = str(worktree.get("run_gate_state") or "").strip()
        if gate:
            return f"ALLOCATED - {gate.replace('_', ' ').upper()}"
        return "ALLOCATED"
    if status == "idle":
        return "IDLE"
    return status.upper() or "UNKNOWN"


def worktree_repo_segment(worktree: dict[str, object]) -> str:
    """Return the stable repo segment from a worktree id (`<repoSegment>/wt-NNNN`).

    The worktree id's segment is the most reliable repo join key because it is identical
    for idle and allocated members, unlike the flattened `repo` field (which is the
    registry id for an idle member but the bound checkout's repo path for an allocated
    one).
    """
    worktree_id = str(worktree.get("worktree_id") or "")
    segment, separator, _leaf = worktree_id.partition("/")
    if separator:
        return segment
    return ""


def worktree_matches_repo(worktree: dict[str, object], repo: dict[str, object]) -> bool:
    """Whether a worktree belongs to a registered repo.

    Matches on any stable identity the backend exposes: the worktree id's repo segment
    equal to the repo id (the canonical join), or the flattened `repo` field equal to the
    repo id or the repo's local_root (covering the idle-vs-allocated `repo`-field
    difference).
    """
    repo_id = str(repo.get("id") or "").strip()
    repo_root = str(repo.get("local_root") or "").strip()
    candidates = {value for value in (worktree_repo_segment(worktree), str(worktree.get("repo") or "").strip()) if value}
    if repo_id and repo_id in candidates:
        return True
    if repo_root and _same_path(repo_root, str(worktree.get("repo") or "").strip()):
        return True
    return False


def filter_worktrees_by_repo(
    worktrees: Iterable[dict[str, object]],
    selected_repo_id: str,
    repos: Iterable[dict[str, object]],
) -> list[dict[str, object]]:
    """Filter the pool to one repo, or return all for the All-repos selection.

    `selected_repo_id` is a registry repo id (or the ALL_REPOS_OPTION sentinel). The
    repo list is the registry projection used to resolve the id to its match keys.
    """
    worktree_list = list(worktrees)
    if not selected_repo_id or selected_repo_id == ALL_REPOS_OPTION:
        return sort_worktrees(worktree_list)
    repo = next((entry for entry in repos if str(entry.get("id") or "") == selected_repo_id), None)
    if repo is None:
        # An unknown selection (e.g. a repo that dropped out of the registry) shows
        # nothing rather than silently falling back to the full pool.
        return []
    return sort_worktrees([wt for wt in worktree_list if worktree_matches_repo(wt, repo)])


def sort_worktrees(worktrees: Iterable[dict[str, object]]) -> list[dict[str, object]]:
    """Stable display order: allocated rows first, then by worktree id."""
    def key(worktree: dict[str, object]) -> tuple[int, str]:
        return (0 if is_allocated(worktree) else 1, str(worktree.get("worktree_id") or ""))

    return sorted(worktrees, key=key)


def repo_filter_options(repos: Iterable[dict[str, object]]) -> list[str]:
    """The dropdown values: All repos + every registered repo id (registry-sourced)."""
    options = [ALL_REPOS_OPTION]
    options.extend(str(repo.get("id") or "") for repo in repos if str(repo.get("id") or ""))
    return options


def worktree_summary_counts(worktrees: Iterable[dict[str, object]]) -> dict[str, int]:
    allocated = 0
    idle = 0
    for worktree in worktrees:
        if is_allocated(worktree):
            allocated += 1
        elif str(worktree.get("status") or "").lower() == "idle":
            idle += 1
    return {"allocated": allocated, "idle": idle, "total": allocated + idle}


def worktree_detail_lines(worktree: dict[str, object]) -> list[tuple[str, str]]:
    """Per-row detail fields (repo + local dir + id, and the allocated binding)."""
    lines: list[tuple[str, str]] = [
        ("Repo", str(worktree.get("repo") or "")),
        ("Local dir", str(worktree.get("worktree_path") or "")),
        ("ID", str(worktree.get("worktree_id") or "")),
    ]
    if is_allocated(worktree):
        for label, key in (
            ("Task", "task_id"),
            ("Run", "run_id"),
            ("Gate", "run_gate_state"),
        ):
            value = str(worktree.get(key) or "")
            if value:
                lines.append((label, value))
    return [(label, value) for label, value in lines if value]


def open_task_options(tasks_snapshot: dict[str, object]) -> list[dict[str, str]]:
    """Project the open-task list (from GET /api/v1/tasks via tasks_backend) for the
    Assign popup: id + title + state ONLY (mockup exclusion E6 — no progress bars or
    file-ref lines).
    """
    options: list[dict[str, str]] = []
    for task in list(tasks_snapshot.get("tasks", [])):
        if not isinstance(task, dict):
            continue
        task_id = str(task.get("task_id") or "").strip()
        if not task_id:
            continue
        options.append(
            {
                "task_id": task_id,
                "title": str(task.get("title") or task_id),
                "state": str(task.get("state_label") or task.get("state") or ""),
            }
        )
    return options


def _same_path(left: str, right: str) -> bool:
    if not left or not right:
        return False
    return _normalize_path(left) == _normalize_path(right)


def _normalize_path(value: str) -> str:
    return value.replace("\\", "/").rstrip("/").lower()
