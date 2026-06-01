from __future__ import annotations

import io
import json
import os
import unittest
from unittest import mock
from urllib import error

from app.codex_dashboard.worktrees_backend import (
    WORKTREES_BACKEND_URL_ENV,
    WorktreesBackendError,
    assign_worktree,
    configured_worktrees_backend_url,
    create_worktree,
    dequeue_task,
    destroy_worktree,
    eject_worktree,
    map_backend_pool_snapshot,
    map_backend_repos,
    worktrees_backend_error_snapshot,
)
from app.codex_dashboard.worktrees_tab import (
    ALL_REPOS_OPTION,
    filter_worktrees_by_repo,
    is_allocated,
    open_task_options,
    repo_filter_options,
    shorten_path,
    sort_worktrees,
    worktree_detail_lines,
    worktree_face_lines,
    worktree_matches_repo,
    worktree_status_background,
    worktree_status_color,
    worktree_status_label,
    worktree_summary_counts,
)


# Backend-shaped fixtures taken from the live PASS-0006 server-only smoke. Idle members
# carry the registry id in `repo`; allocated members carry the bound checkout's repo path
# in `repo` — the worktrees_tab helpers must join on the stable worktree_id segment.
ALLOCATED_WORKTREE = {
    "worktree_id": "obsidian/wt-0001",
    "status": "allocated",
    "run_id": "taskrun--Task-0007--active",
    "repo": "C:\\Agent\\CodexDashboard\\Tracking\\Task-0016\\Testing\\Runtime\\smoke-testbed",
    "task_id": "Task-0007",
    "worktree_path": "C:\\owned\\obsidian\\wt-0001\\wt-0001",
    "run_gate_state": "running",
    "agent_session_id": "sess-abc",
    "session_transcript_path": "C:\\transcripts\\sess-abc.jsonl",
    "launched_pid": 12345,
}
IDLE_WORKTREE = {
    "worktree_id": "obsidian/wt-0002",
    "status": "idle",
    "repo": "obsidian",
    "task_id": "",
    "worktree_path": "C:\\owned\\obsidian\\wt-0002\\wt-0002",
    "run_gate_state": "",
}
OTHER_REPO_WORKTREE = {
    "worktree_id": "demo/wt-0001",
    "status": "idle",
    "repo": "demo",
    "worktree_path": "C:\\owned\\demo\\wt-0001\\wt-0001",
}
REPOS = [
    {
        "id": "obsidian",
        "local_root": "C:\\Agent\\CodexDashboard\\Tracking\\Task-0016\\Testing\\Runtime\\smoke-testbed",
        "task_provider_repo": "gregsemple2003/obsidian",
    },
    {"id": "demo", "local_root": "C:\\Agent\\Demo", "task_provider_repo": ""},
]


class WorktreesTabHelperTests(unittest.TestCase):
    def test_allocated_and_idle_have_distinct_background_colors(self) -> None:
        allocated_bg = worktree_status_background(ALLOCATED_WORKTREE)
        idle_bg = worktree_status_background(IDLE_WORKTREE)
        self.assertNotEqual(allocated_bg, idle_bg)
        self.assertTrue(is_allocated(ALLOCATED_WORKTREE))
        self.assertFalse(is_allocated(IDLE_WORKTREE))

    def test_status_color_and_label_reflect_state(self) -> None:
        self.assertNotEqual(
            worktree_status_color(ALLOCATED_WORKTREE), worktree_status_color(IDLE_WORKTREE)
        )
        self.assertIn("ALLOCATED", worktree_status_label(ALLOCATED_WORKTREE))
        self.assertIn("RUNNING", worktree_status_label(ALLOCATED_WORKTREE))
        self.assertEqual(worktree_status_label(IDLE_WORKTREE), "IDLE")

    def test_detail_lines_include_repo_path_id_and_allocated_binding(self) -> None:
        idle_lines = dict(worktree_detail_lines(IDLE_WORKTREE))
        self.assertEqual(idle_lines["Local dir"], IDLE_WORKTREE["worktree_path"])
        self.assertEqual(idle_lines["Worktree id"], IDLE_WORKTREE["worktree_id"])
        self.assertNotIn("Task", idle_lines)

        # The Details reveal carries the full secondary/diagnostic fields (UPDATE 5).
        allocated_lines = dict(worktree_detail_lines(ALLOCATED_WORKTREE))
        self.assertEqual(allocated_lines["Task"], "Task-0007")
        self.assertEqual(allocated_lines["Run gate"], "running")
        self.assertEqual(allocated_lines["Agent session"], "sess-abc")
        self.assertEqual(allocated_lines["Transcript"], "C:\\transcripts\\sess-abc.jsonl")
        self.assertEqual(allocated_lines["Launched PID"], "12345")

    def test_face_lines_are_glanceable_only(self) -> None:
        # The panel face shows only the bound task (allocated) + a SHORT path; the long
        # ids / session / pid / transcript are NOT on the face (UPDATE 5).
        allocated_face = dict(worktree_face_lines(ALLOCATED_WORKTREE))
        self.assertEqual(allocated_face.get("Task"), "Task-0007")
        self.assertIn("Local dir", allocated_face)
        self.assertNotIn("Run", allocated_face)
        self.assertNotIn("Agent session", allocated_face)
        self.assertNotIn("Worktree id", allocated_face)
        idle_face = dict(worktree_face_lines(IDLE_WORKTREE))
        self.assertNotIn("Task", idle_face)
        self.assertIn("Local dir", idle_face)

    def test_shorten_path_keeps_leaf_with_ellipsis(self) -> None:
        short = shorten_path("C:\\Users\\gregs\\AppData\\Local\\Temp\\cdxow\\repo-x\\wt-0001\\wt-0001")
        self.assertTrue(short.startswith("..."))
        self.assertIn("wt-0001", short)
        self.assertEqual(shorten_path("C:\\short\\p"), "C:\\short\\p")  # short path unchanged

    def test_repo_filter_options_are_registry_sourced_not_hardcoded(self) -> None:
        # The dropdown is All repos + EXACTLY the registry ids, in registry order.
        self.assertEqual(repo_filter_options(REPOS), [ALL_REPOS_OPTION, "obsidian", "demo"])
        # An empty registry leaves only the All-repos sentinel — never a hardcoded fallback.
        self.assertEqual(repo_filter_options([]), [ALL_REPOS_OPTION])

    def test_worktree_matches_repo_joins_on_id_segment_for_allocated(self) -> None:
        obsidian = REPOS[0]
        # Allocated member's `repo` is the local path, but the worktree_id segment is the id.
        self.assertTrue(worktree_matches_repo(ALLOCATED_WORKTREE, obsidian))
        self.assertTrue(worktree_matches_repo(IDLE_WORKTREE, obsidian))
        self.assertFalse(worktree_matches_repo(OTHER_REPO_WORKTREE, obsidian))

    def test_filter_by_repo_narrows_and_all_repos_restores(self) -> None:
        pool = [ALLOCATED_WORKTREE, IDLE_WORKTREE, OTHER_REPO_WORKTREE]
        all_view = filter_worktrees_by_repo(pool, ALL_REPOS_OPTION, REPOS)
        self.assertEqual(len(all_view), 3)
        obsidian_view = filter_worktrees_by_repo(pool, "obsidian", REPOS)
        self.assertEqual(
            {wt["worktree_id"] for wt in obsidian_view},
            {"obsidian/wt-0001", "obsidian/wt-0002"},
        )
        demo_view = filter_worktrees_by_repo(pool, "demo", REPOS)
        self.assertEqual([wt["worktree_id"] for wt in demo_view], ["demo/wt-0001"])

    def test_filter_unknown_repo_shows_nothing(self) -> None:
        pool = [ALLOCATED_WORKTREE, IDLE_WORKTREE]
        self.assertEqual(filter_worktrees_by_repo(pool, "ghost", REPOS), [])

    def test_sort_places_allocated_first_then_by_id(self) -> None:
        ordered = sort_worktrees([IDLE_WORKTREE, OTHER_REPO_WORKTREE, ALLOCATED_WORKTREE])
        self.assertEqual(ordered[0]["worktree_id"], "obsidian/wt-0001")  # allocated first

    def test_summary_counts(self) -> None:
        counts = worktree_summary_counts([ALLOCATED_WORKTREE, IDLE_WORKTREE, OTHER_REPO_WORKTREE])
        self.assertEqual(counts, {"allocated": 1, "idle": 2, "total": 3})

    def test_open_task_options_id_title_state_no_progress(self) -> None:
        snapshot = {
            "tasks": [
                {"task_id": "Task-0007", "title": "Pool work", "state_label": "Ready", "progress": 0.5},
                {"task_id": "", "title": "skip me"},
                {"task_id": "Task-0009", "title": "Other", "state": "running"},
            ]
        }
        options = open_task_options(snapshot)
        self.assertEqual([o["task_id"] for o in options], ["Task-0007", "Task-0009"])
        self.assertEqual(options[0], {"task_id": "Task-0007", "title": "Pool work", "state": "Ready"})
        # No progress key leaks into the popup projection (mockup exclusion E6).
        self.assertNotIn("progress", options[0])


class WorktreesBackendTests(unittest.TestCase):
    def test_configured_url_prefers_worktrees_env_then_shared_tasks_env(self) -> None:
        with mock.patch.dict(os.environ, {WORKTREES_BACKEND_URL_ENV: "http://127.0.0.1:14318"}, clear=False):
            self.assertEqual(configured_worktrees_backend_url(), "http://127.0.0.1:14318")
        with mock.patch.dict(
            os.environ,
            {"CODEX_DASHBOARD_TASKS_BACKEND_URL": "http://127.0.0.1:24318"},
            clear=True,
        ):
            self.assertEqual(configured_worktrees_backend_url(), "http://127.0.0.1:24318")
        with mock.patch.dict(os.environ, {}, clear=True):
            self.assertEqual(configured_worktrees_backend_url(), "http://127.0.0.1:4318")

    def test_map_pool_snapshot_flattens_fields(self) -> None:
        snapshot = map_backend_pool_snapshot({"worktrees": [ALLOCATED_WORKTREE, IDLE_WORKTREE]})
        self.assertEqual(snapshot["status"], "ok")
        self.assertEqual(len(snapshot["worktrees"]), 2)
        allocated = snapshot["worktrees"][0]
        self.assertEqual(allocated["status"], "allocated")
        self.assertEqual(allocated["task_id"], "Task-0007")
        self.assertEqual(allocated["run_id"], "taskrun--Task-0007--active")
        self.assertEqual(allocated["launched_pid"], 12345)
        self.assertEqual(allocated["session_transcript_path"], "C:\\transcripts\\sess-abc.jsonl")
        idle = snapshot["worktrees"][1]
        self.assertEqual(idle["status"], "idle")
        self.assertEqual(idle["run_id"], "")
        self.assertEqual(idle["launched_pid"], 0)

    def test_map_pool_snapshot_rejects_non_list(self) -> None:
        with self.assertRaises(WorktreesBackendError):
            map_backend_pool_snapshot({"worktrees": "nope"})

    def test_map_repos_drops_idless_entries(self) -> None:
        repos = map_backend_repos(
            {"repos": [{"id": "obsidian", "local_root": "C:\\x"}, {"local_root": "C:\\y"}, "bad"]}
        )
        self.assertEqual([r["id"] for r in repos], ["obsidian"])

    def test_error_snapshot_shape(self) -> None:
        snap = worktrees_backend_error_snapshot("backend down")
        self.assertEqual(snap["status"], "backend_unavailable")
        self.assertEqual(snap["worktrees"], [])
        self.assertEqual(snap["message"], "backend down")

    def test_create_posts_repo_body(self) -> None:
        captured = {}

        def fake_urlopen(req, timeout=None):
            captured["url"] = req.full_url
            captured["method"] = req.get_method()
            captured["body"] = req.data.decode("utf-8") if req.data else ""
            return _FakeResponse({"worktree_id": "obsidian/wt-0001", "status": "idle"})

        with mock.patch("app.codex_dashboard.worktrees_backend.request.urlopen", fake_urlopen):
            result = create_worktree("obsidian", "http://127.0.0.1:14318")
        self.assertEqual(captured["url"], "http://127.0.0.1:14318/api/v1/worktrees/create")
        self.assertEqual(captured["method"], "POST")
        self.assertEqual(json.loads(captured["body"]), {"repo": "obsidian"})
        self.assertEqual(result["worktree_id"], "obsidian/wt-0001")

    def test_assign_eject_destroy_dequeue_post_expected_bodies(self) -> None:
        calls = []

        def fake_urlopen(req, timeout=None):
            calls.append((req.full_url, json.loads(req.data.decode("utf-8")) if req.data else {}))
            return _FakeResponse({"status": "ok"})

        with mock.patch("app.codex_dashboard.worktrees_backend.request.urlopen", fake_urlopen):
            assign_worktree("Task-0007", "obsidian", "obsidian/wt-0001", "http://b")
            eject_worktree("taskrun--Task-0007--active", "obsidian/wt-0001", "http://b")
            destroy_worktree("obsidian/wt-0002", "http://b")
            dequeue_task("obsidian", "Task-0007", "http://b")

        self.assertEqual(calls[0][0], "http://b/api/v1/worktrees/assign")
        self.assertEqual(
            calls[0][1], {"task_id": "Task-0007", "repo": "obsidian", "worktree_id": "obsidian/wt-0001"}
        )
        self.assertEqual(calls[1][0], "http://b/api/v1/worktrees/eject")
        self.assertEqual(calls[1][1], {"run_id": "taskrun--Task-0007--active", "worktree_id": "obsidian/wt-0001"})
        self.assertEqual(calls[2][0], "http://b/api/v1/worktrees/destroy")
        self.assertEqual(calls[2][1], {"worktree_id": "obsidian/wt-0002"})
        self.assertEqual(calls[3][0], "http://b/api/v1/worktrees/dequeue")
        self.assertEqual(calls[3][1], {"repo": "obsidian", "task_id": "Task-0007"})

    def test_destroy_surfaces_backend_409_message(self) -> None:
        def fake_urlopen(req, timeout=None):
            raise error.HTTPError(
                req.full_url,
                409,
                "Conflict",
                hdrs=None,
                fp=io.BytesIO(json.dumps({"error": "worktree is allocated; eject it before destroy"}).encode("utf-8")),
            )

        with mock.patch("app.codex_dashboard.worktrees_backend.request.urlopen", fake_urlopen):
            with self.assertRaises(WorktreesBackendError) as ctx:
                destroy_worktree("obsidian/wt-0001", "http://b")
        self.assertIn("allocated", str(ctx.exception))
        self.assertIn("409", str(ctx.exception))


class _FakeResponse:
    def __init__(self, payload: dict) -> None:
        self._raw = json.dumps(payload).encode("utf-8")

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False

    def read(self) -> bytes:
        return self._raw


if __name__ == "__main__":
    unittest.main()
