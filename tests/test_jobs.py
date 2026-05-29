from __future__ import annotations

import subprocess
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

import app.codex_dashboard.jobs as jobs_module
from app.codex_dashboard.jobs import (
    DESIRED_STATE_DISABLED,
    DESIRED_STATE_ENABLED,
    JOB_KIND_SCHEDULED_TASK,
    JOB_KIND_STARTUP_LAUNCHER,
    JOB_STATUS_DISABLED,
    JOB_STATUS_DRIFTED,
    JOB_STATUS_IN_SYNC,
    JOB_STATUS_MISSING,
    JobObservation,
    apply_job,
    bootstrap_jobs_registry,
    default_jobs_registry_path,
    ensure_jobs_registry,
    reconcile_job,
    set_job_desired_state,
)
from app.codex_dashboard.paths import default_jobs_schema_path, legacy_jobs_registry_path


class JobsTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.root = Path(self.temp_dir.name)
        self.codex_root = self.root / ".codex"

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def test_default_jobs_registry_path_uses_orchestration_subtree(self) -> None:
        registry_path = default_jobs_registry_path(self.codex_root)

        self.assertEqual(
            registry_path,
            self.codex_root / "Orchestration" / "Jobs" / "declared-jobs.json",
        )

    def test_bootstrap_jobs_registry_creates_startup_and_scheduled_entries(self) -> None:
        startup_script = self.root / "Startup" / "CodexDashboard.cmd"
        with mock.patch(
            "app.codex_dashboard.jobs.startup_script_path",
            return_value=startup_script,
        ), mock.patch(
            "app.codex_dashboard.jobs.startup_command",
            return_value='@echo off\r\n"pythonw.exe" -m app.codex_dashboard\r\n',
        ), mock.patch(
            "app.codex_dashboard.jobs._discover_bootstrap_scheduled_task_definitions",
            return_value=[
                {
                    "job_id": "codex-daily-agentic-swe-digest",
                    "label": "Codex Daily Agentic SWE Digest",
                    "kind": JOB_KIND_SCHEDULED_TASK,
                    "desired_state": DESIRED_STATE_ENABLED,
                    "definition": {
                        "task_name": "Codex Daily Agentic SWE Digest",
                        "task_path": "\\",
                        "task_xml": "<Task />",
                    },
                }
            ],
        ):
            registry = bootstrap_jobs_registry(codex_root=self.codex_root)

        registry_path = default_jobs_registry_path(self.codex_root)
        self.assertTrue(registry_path.exists())
        self.assertTrue(default_jobs_schema_path(self.codex_root).exists())
        self.assertEqual(registry["schema_version"], 1)
        self.assertEqual(len(registry["jobs"]), 2)
        self.assertEqual(registry["jobs"][0]["kind"], JOB_KIND_STARTUP_LAUNCHER)
        self.assertEqual(registry["jobs"][1]["label"], "Codex Daily Agentic SWE Digest")

    def test_ensure_jobs_registry_returns_existing_registry_without_rebootstrap(self) -> None:
        registry_path = default_jobs_registry_path(self.codex_root)
        registry_path.parent.mkdir(parents=True, exist_ok=True)
        registry_path.write_text(
            '{"schema_version": 1, "updated_at": "2026-04-04T00:00:00-04:00", "jobs": []}\n',
            encoding="utf-8",
        )

        with mock.patch("app.codex_dashboard.jobs.bootstrap_jobs_registry") as bootstrap:
            registry = ensure_jobs_registry(codex_root=self.codex_root)

        bootstrap.assert_not_called()
        self.assertEqual(registry["jobs"], [])

    def test_ensure_jobs_registry_migrates_legacy_registry_path(self) -> None:
        legacy_path = legacy_jobs_registry_path(self.codex_root)
        legacy_path.parent.mkdir(parents=True, exist_ok=True)
        legacy_path.write_text(
            '{"schema_version": 1, "updated_at": "2026-04-04T00:00:00-04:00", "jobs": []}\n',
            encoding="utf-8",
        )

        registry = ensure_jobs_registry(codex_root=self.codex_root)

        self.assertEqual(registry["jobs"], [])
        self.assertTrue(default_jobs_registry_path(self.codex_root).exists())
        self.assertTrue(default_jobs_schema_path(self.codex_root).exists())

    def test_reconcile_job_reports_missing_startup_launcher(self) -> None:
        job = {
            "job_id": "codex-dashboard-startup",
            "label": "Obsidian overlay at sign-in",
            "kind": JOB_KIND_STARTUP_LAUNCHER,
            "desired_state": DESIRED_STATE_ENABLED,
            "definition": {
                "script_path": str(self.root / "Startup" / "CodexDashboard.cmd"),
                "command_text": "@echo off\r\nexpected\r\n",
            },
        }

        status = reconcile_job(job, JobObservation(False, None, None))

        self.assertEqual(status["status"], JOB_STATUS_MISSING)

    def test_reconcile_job_reports_drifted_startup_launcher(self) -> None:
        job = {
            "job_id": "codex-dashboard-startup",
            "label": "Obsidian overlay at sign-in",
            "kind": JOB_KIND_STARTUP_LAUNCHER,
            "desired_state": DESIRED_STATE_ENABLED,
            "definition": {
                "script_path": str(self.root / "Startup" / "CodexDashboard.cmd"),
                "command_text": "@echo off\r\nexpected\r\n",
            },
        }

        status = reconcile_job(
            job,
            JobObservation(
                True,
                True,
                "@echo off\nobserved",
                details={"script_path": job["definition"]["script_path"]},
            ),
        )

        self.assertEqual(status["status"], JOB_STATUS_DRIFTED)

    def test_reconcile_job_reports_disabled_scheduled_task(self) -> None:
        job = {
            "job_id": "codex-digest",
            "label": "Codex Digest",
            "kind": JOB_KIND_SCHEDULED_TASK,
            "desired_state": DESIRED_STATE_ENABLED,
            "definition": {
                "task_name": "Codex Digest",
                "task_path": "\\",
                "task_xml": "<Task />",
            },
        }

        status = reconcile_job(
            job,
            JobObservation(
                True,
                False,
                "<Task />",
                details={"task_name": "Codex Digest", "task_path": "\\"},
            ),
        )

        self.assertEqual(status["status"], JOB_STATUS_DISABLED)

    def test_reconcile_job_includes_managed_definition(self) -> None:
        job = {
            "job_id": "codex-dashboard-startup",
            "label": "Obsidian overlay at sign-in",
            "kind": JOB_KIND_STARTUP_LAUNCHER,
            "desired_state": DESIRED_STATE_ENABLED,
            "definition": {
                "script_path": str(self.root / "Startup" / "CodexDashboard.cmd"),
                "command_text": "@echo off\r\nexpected\r\n",
            },
        }

        status = reconcile_job(
            job,
            JobObservation(
                True,
                True,
                "@echo off\nexpected",
                details={"script_path": job["definition"]["script_path"]},
            ),
        )

        self.assertEqual(status["definition"], job["definition"])

    def test_apply_job_writes_startup_launcher_idempotently(self) -> None:
        startup_script = self.root / "Startup" / "CodexDashboard.cmd"
        job = {
            "job_id": "codex-dashboard-startup",
            "label": "Obsidian overlay at sign-in",
            "kind": JOB_KIND_STARTUP_LAUNCHER,
            "desired_state": DESIRED_STATE_ENABLED,
            "definition": {
                "script_path": str(startup_script),
                "command_text": "@echo off\r\nexpected\r\n",
            },
        }

        first_status = apply_job(job)
        second_status = apply_job(job)

        self.assertTrue(startup_script.exists())
        self.assertEqual(
            startup_script.read_text(encoding="utf-8").splitlines(),
            ["@echo off", "expected"],
        )
        self.assertEqual(first_status["status"], JOB_STATUS_IN_SYNC)
        self.assertEqual(second_status["status"], JOB_STATUS_IN_SYNC)

    def test_apply_job_registers_scheduled_task_from_xml(self) -> None:
        job = {
            "job_id": "codex-digest",
            "label": "Codex Digest",
            "kind": JOB_KIND_SCHEDULED_TASK,
            "desired_state": DESIRED_STATE_ENABLED,
            "definition": {
                "task_name": "Codex Digest",
                "task_path": "\\",
                "task_xml": "<Task />",
            },
        }

        with mock.patch("app.codex_dashboard.jobs._run_powershell") as run_powershell, mock.patch(
            "app.codex_dashboard.jobs.reconcile_job",
            return_value={"status": JOB_STATUS_IN_SYNC},
        ) as reconcile:
            result = apply_job(job)

        self.assertEqual(result["status"], JOB_STATUS_IN_SYNC)
        self.assertIn("Register-ScheduledTask", run_powershell.call_args.args[0])
        self.assertIn("Enable-ScheduledTask", run_powershell.call_args.args[0])
        reconcile.assert_called_once_with(job)

    def test_set_job_desired_state_updates_only_target_job(self) -> None:
        registry = {
            "schema_version": 1,
            "updated_at": "2026-04-04T00:00:00-04:00",
            "jobs": [
                {
                    "job_id": "codex-dashboard-startup",
                    "label": "Obsidian overlay at sign-in",
                    "kind": JOB_KIND_STARTUP_LAUNCHER,
                    "desired_state": DESIRED_STATE_ENABLED,
                    "definition": {
                        "script_path": str(self.root / "Startup" / "CodexDashboard.cmd"),
                        "command_text": "@echo off\r\nexpected\r\n",
                    },
                },
                {
                    "job_id": "codex-digest",
                    "label": "Codex Digest",
                    "kind": JOB_KIND_SCHEDULED_TASK,
                    "desired_state": DESIRED_STATE_ENABLED,
                    "definition": {
                        "task_name": "Codex Digest",
                        "task_path": "\\",
                        "task_xml": "<Task />",
                    },
                },
            ],
        }

        updated_registry = set_job_desired_state(
            registry,
            "codex-digest",
            DESIRED_STATE_DISABLED,
        )

        self.assertEqual(updated_registry["jobs"][0]["desired_state"], DESIRED_STATE_ENABLED)
        self.assertEqual(updated_registry["jobs"][1]["desired_state"], DESIRED_STATE_DISABLED)
        self.assertNotEqual(updated_registry["updated_at"], registry["updated_at"])

    def test_run_powershell_hides_child_windows(self) -> None:
        with mock.patch(
            "app.codex_dashboard.jobs.subprocess.run",
            return_value=SimpleNamespace(returncode=0, stderr="", stdout="ok"),
        ) as run_powershell:
            result = jobs_module._run_powershell("Write-Output 'ok'")

        self.assertEqual(result, "ok")
        self.assertEqual(
            run_powershell.call_args.kwargs["creationflags"],
            getattr(subprocess, "CREATE_NO_WINDOW", 0),
        )
        self.assertIs(run_powershell.call_args.kwargs["check"], False)


if __name__ == "__main__":
    unittest.main()
