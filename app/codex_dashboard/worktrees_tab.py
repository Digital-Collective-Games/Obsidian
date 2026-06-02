from __future__ import annotations

from typing import Iterable
from urllib.parse import quote


# Allocated vs idle is the load-bearing visual distinction (Goal 13 / REG-010): the two
# row background colors must be perceivably different, reusing the app's dark cyan/navy
# palette. Allocated rows get a brighter navy-teal panel; idle rows stay the darker
# surface used elsewhere in the cockpit.
WORKTREE_STATUS_BACKGROUNDS = {
    "allocated": "#173a44",
    "idle": "#181c22",
}
WORKTREE_STATUS_DEFAULT_BACKGROUND = "#181c22"

# Status accent per status, matching the Stitch "Monolithic Terminal" mockup: idle = a
# subtle gray stripe, allocated (live/running) = bright cyan, needs-human/review (a parked
# run gate) = error red. The repo name reuses this color when allocated; the status chip
# fills with it for the bright states (the idle chip stays muted gray — see
# _build_worktree_status_chip).
WORKTREE_STATUS_COLORS = {
    "idle": "#3b494c",
    "allocated": "#00e5ff",
    "needs_human": "#ffb4ab",
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


def worktree_needs_human(worktree: dict[str, object]) -> bool:
    """Whether an allocated worktree's run is parked awaiting a human (closure / a research,
    plan, or regression gate, or an interrupt review) — i.e. it "needs human / review". Idle
    worktrees never need a human."""
    if str(worktree.get("status") or "").lower() != "allocated":
        return False
    gate = str(worktree.get("run_gate_state") or "").lower()
    return any(token in gate for token in ("parked", "await", "human", "review", "interrupt"))


def worktree_status_color(worktree: dict[str, object]) -> str:
    status = str(worktree.get("status") or "").lower()
    if status == "allocated":
        if worktree_needs_human(worktree):
            return WORKTREE_STATUS_COLORS["needs_human"]
        return WORKTREE_STATUS_COLORS["allocated"]
    return WORKTREE_STATUS_COLORS.get(status, WORKTREE_STATUS_DEFAULT_COLOR)


def worktree_status_label(worktree: dict[str, object]) -> str:
    status = str(worktree.get("status") or "").lower()
    if status == "idle":
        return "IDLE"
    if status == "allocated":
        return "REVIEW" if worktree_needs_human(worktree) else "ALLOCATED"
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


def worktree_heading_repo(worktree: dict[str, object]) -> str:
    """The short repo token for the panel HEADING in BOTH idle and allocated states.

    The flattened `repo` field is the registry id for an idle member but the bound
    checkout PATH for an allocated one; using it raw overflows the Space-Grotesk heading
    slot and reads as a different "kind" of heading. Prefer the stable worktree-id repo
    segment (identical for idle and allocated), falling back to `repo` only if empty.
    """
    segment = worktree_repo_segment(worktree)
    if segment:
        return segment
    return str(worktree.get("repo") or "")


# The chip carries a leading STATE MARK (UPDATE 5: "status via a chip, not text-only"):
# a filled mark for allocated, a hollow/outlined mark for idle, within the same cyan/green
# WORKTREE_STATUS_COLORS family. Drawn as a small Tk Canvas swatch (no emoji; 0px radius).
def worktree_chip_mark_filled(worktree: dict[str, object]) -> bool:
    """Whether the chip's state mark is filled (allocated) vs outlined (idle)."""
    return is_allocated(worktree)


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


def worktree_issue_url(worktree: dict[str, object], repos: Iterable[dict[str, object]]) -> str:
    """The GitHub issue URL for an allocated worktree's bound task, or "" if unresolvable.

    The bound task id (`Task-<n>`) maps to issue `#<n>` in the worktree repo's task provider
    repo (`owner/name`, resolved from the registry). Returns "" when there is no bound task,
    the id carries no number, or the repo has no `owner/name` provider — the caller then
    renders the task id as plain (non-link) text rather than a dead link.
    """
    task_id = str(worktree.get("task_id") or "").strip()
    digits = "".join(ch for ch in task_id if ch.isdigit())
    if not digits:
        return ""
    match = next((repo for repo in repos if worktree_matches_repo(worktree, repo)), None)
    provider = str((match or {}).get("task_provider_repo") or "").strip().strip("/")
    if provider.count("/") != 1 or not all(provider.split("/")):
        return ""
    return f"https://github.com/{provider}/issues/{int(digits)}"


def worktree_session_target(worktree: dict[str, object]) -> str:
    """The local path to open in VSCodium to watch / interact with an allocated worktree's
    running session.

    Prefer the worktree CHECKOUT folder: it is always present for an allocated member, it is
    where the agent's live edits land (you can watch the diff and open a terminal to interact),
    and for a Claude Code session it is the folder you resume the conversation from. Fall back
    to the captured session transcript path only when the checkout path is empty, and return
    "" when neither is known (e.g. an idle worktree, which has no running session).
    """
    path = str(worktree.get("worktree_path") or "").strip()
    if path:
        return path
    return str(worktree.get("session_transcript_path") or "").strip()


def vscodium_uri(path: str) -> str:
    """Build a ``vscodium://file/<path>`` URI for a local file or folder.

    The registered VSCodium protocol handler opens it (``VSCodium.exe --open-url``). The
    backend deliberately exposes the raw worktree/session fields and never emits this link
    itself — constructing it is the client's job. Backslashes become forward slashes and the
    path is percent-encoded except for the drive colon and separators, so spaces are handled.
    """
    forward = str(path or "").replace("\\", "/")
    return "vscodium://file/" + quote(forward, safe="/:")


def claude_session_uri(session_id: str) -> str:
    """Build the URI that opens a specific Claude Code session in VSCodium's Claude extension
    chat panel (extension id ``anthropic.claude-code``), so the operator can read the agent's
    thinking and continue the conversation.

    The extension resolves the session against the workspace currently OPEN in the editor, so
    the caller opens the worktree folder first (``vscodium_uri``) and then fires this. The
    registered editor scheme on this machine is ``vscodium://`` (plain VS Code would be
    ``vscode://``). Returns "" when there is no session id to open.
    """
    sid = str(session_id or "").strip()
    if not sid:
        return ""
    return "vscodium://anthropic.claude-code/open?session=" + quote(sid, safe="")


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


def shorten_path(path: str, max_len: int = 52) -> str:
    """A glanceable short form of a long local-dir path for the panel face: keep the
    leaf (and one parent) with a leading ellipsis when the full path is long. The full
    path stays available behind the Details reveal / tooltip and the copy control copies
    the EXACT full path.
    """
    path = str(path or "")
    if len(path) <= max_len:
        return path
    normalized = path.replace("/", "\\")
    parts = [p for p in normalized.split("\\") if p]
    if len(parts) >= 2:
        tail = "\\".join(parts[-2:])
    elif parts:
        tail = parts[-1]
    else:
        return path[-max_len:]
    return "...\\" + tail


def worktree_face_lines(worktree: dict[str, object]) -> list[tuple[str, str]]:
    """The GLANCEABLE on-face fields (INTERFACE-DESIGNER: default surface optimized for
    ordinary interpretation). Repo is the panel heading and the chip carries status, so
    the body face shows only the bound task (allocated) and a SHORT local dir; the full
    path / ids / session / pid / transcript move behind the Details reveal.
    """
    lines: list[tuple[str, str]] = []
    if is_allocated(worktree):
        task_id = str(worktree.get("task_id") or "")
        if task_id:
            lines.append(("Task", task_id))
    short = shorten_path(str(worktree.get("worktree_path") or ""))
    if short:
        lines.append(("Local dir", short))
    return lines


def worktree_detail_lines(worktree: dict[str, object]) -> list[tuple[str, str]]:
    """The full secondary/diagnostic fields for the per-panel Details reveal / tooltip:
    the full local dir, the stable id, and (allocated) the bound run/gate/session/pid/
    transcript. Empty fields are omitted truthfully (no fabricated agent-model chip — E4).
    """
    lines: list[tuple[str, str]] = [
        ("Repo", str(worktree.get("repo") or "")),
        ("Local dir", str(worktree.get("worktree_path") or "")),
        ("Worktree id", str(worktree.get("worktree_id") or "")),
    ]
    if is_allocated(worktree):
        for label, key in (
            ("Task", "task_id"),
            ("Run", "run_id"),
            ("Run gate", "run_gate_state"),
            ("Agent session", "agent_session_id"),
            ("Transcript", "session_transcript_path"),
        ):
            value = str(worktree.get(key) or "")
            if value:
                lines.append((label, value))
        pid = worktree.get("launched_pid")
        if isinstance(pid, int) and pid > 0:
            lines.append(("Launched PID", str(pid)))
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


def task_state_category(state: str) -> str:
    """Bucket a task state into the Assign-popup chip categories (mockup): blocked / ready /
    waiting (a human gate) / other. Substring-matched so it tolerates label variants."""
    s = str(state or "").lower()
    if "block" in s:
        return "blocked"
    if "ready" in s:
        return "ready"
    if any(token in s for token in ("wait", "review", "human", "needs", "park", "you")):
        return "waiting"
    return "other"


def task_state_chip_label(state: str) -> str:
    """The terminal-style chip text for a task state, e.g. "Waiting on you" -> WAITING_ON_YOU."""
    s = str(state or "").strip()
    if not s:
        return "UNKNOWN"
    return s.upper().replace(" ", "_").replace("-", "_")


def task_is_assignable(option: dict[str, object]) -> bool:
    """Whether a task may be bound to a worktree from the Assign popup. Blocked tasks are
    shown but not selectable (mockup: the radio is disabled)."""
    return task_state_category(str(option.get("state") or "")) != "blocked"


def filter_task_options(
    options: Iterable[dict[str, object]], query: str
) -> list[dict[str, object]]:
    """Case-insensitive substring filter over a task option's id + title (the popup's
    FILTER_TASKS_BY_ID_OR_DESC box). An empty query returns everything."""
    needle = str(query or "").strip().lower()
    option_list = list(options)
    if not needle:
        return option_list
    return [
        option
        for option in option_list
        if needle in f"{option.get('task_id', '')} {option.get('title', '')}".lower()
    ]


def sort_task_options(
    options: Iterable[dict[str, object]], ascending: bool = True
) -> list[dict[str, object]]:
    """Stable sort of task options by task id (the popup's SORT: ID control)."""
    return sorted(options, key=lambda option: str(option.get("task_id") or ""), reverse=not ascending)


def first_assignable_task_id(options: Iterable[dict[str, object]]) -> str:
    """The id of the first selectable (non-blocked) task, for the popup's default selection,
    or "" when none are selectable (so BIND_TASK prompts instead of binding a blocked task)."""
    for option in options:
        if task_is_assignable(option):
            return str(option.get("task_id") or "")
    return ""


def _same_path(left: str, right: str) -> bool:
    if not left or not right:
        return False
    return _normalize_path(left) == _normalize_path(right)


def _normalize_path(value: str) -> str:
    return value.replace("\\", "/").rstrip("/").lower()
