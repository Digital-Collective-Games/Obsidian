from __future__ import annotations

import unittest
from datetime import datetime, timezone

from app.codex_dashboard.jobs_backend import (
    cron_human,
    job_is_running,
    job_last_run_display,
    job_next_run_display,
    job_schedule_display,
    job_status_chip,
    jobs_attention_jobs,
    relative_time_label,
    schedule_owner_map,
    summarize_apply_report,
)

NOW = datetime(2026, 6, 2, 14, 2, 45, tzinfo=timezone.utc)


class JobsDisplayHelperTests(unittest.TestCase):
    def test_relative_time_label(self) -> None:
        self.assertEqual(relative_time_label("2026-06-02T14:02:30Z", now=NOW), "just now")
        self.assertEqual(relative_time_label("2026-06-02T13:48:33Z", now=NOW), "14m ago")
        self.assertEqual(relative_time_label("2026-06-02T11:48:33Z", now=NOW), "2h 14m ago")
        self.assertTrue(relative_time_label("2026-06-03T14:02:45Z", now=NOW).startswith("in "))

    def test_schedule_display(self) -> None:
        self.assertEqual(
            job_schedule_display({"mechanism_label": "Schedule", "definition": {"schedules": [{"cron": "0 2 * * *"}]}}),
            ("0 2 * * *", "Daily at 2:00 AM"),
        )
        self.assertEqual(job_schedule_display({"mechanism_label": "Manual", "definition": {}}), ("Manual", ""))

    def test_cron_human(self) -> None:
        self.assertEqual(cron_human("0 2 * * *"), "Daily at 2:00 AM")
        self.assertEqual(cron_human("*/15 * * * *"), "Every 15 minutes")
        self.assertEqual(cron_human("0 * * * *"), "Hourly")
        self.assertEqual(cron_human("30 14 * * *"), "Daily at 2:30 PM")
        self.assertEqual(cron_human("nonsense"), "")

    def test_job_next_run_display(self) -> None:
        job = {"definition": {"schedules": [{"next_action_times": ["2026-06-02T13:00:00Z"]}]}}
        result = job_next_run_display(job)
        self.assertNotEqual(result, "—")
        self.assertTrue(result)
        self.assertEqual(job_next_run_display({"definition": {}}), "—")

    def test_job_is_running(self) -> None:
        self.assertTrue(job_is_running({"definition": {"recent_runs": [{"status": "running"}]}}))
        self.assertFalse(job_is_running({"definition": {"recent_runs": [{"status": "succeeded"}]}}))
        self.assertFalse(job_is_running({"definition": {}}))

    def test_last_run_display(self) -> None:
        job = {"definition": {"recent_runs": [{"actual_time": "2026-06-02T11:48:33Z", "status": "succeeded"}]}}
        self.assertEqual(job_last_run_display(job, now=NOW), ("2h 14m ago", "Succeeded"))
        self.assertEqual(job_last_run_display({"definition": {}}, now=NOW), ("Never run", ""))

    def test_status_chip(self) -> None:
        self.assertEqual(job_status_chip("in_sync")[0], "IN SYNC")
        self.assertEqual(job_status_chip("drifted")[0], "DRIFTED")
        self.assertEqual(job_status_chip("weird-status")[0], "WEIRD STATUS")
        # bg + fg are returned for every status
        self.assertEqual(len(job_status_chip("blocked")), 3)

    def test_attention_jobs_excludes_in_sync(self) -> None:
        snapshot = {"jobs": [
            {"job_id": "a", "status": "in_sync"},
            {"job_id": "b", "status": "drifted"},
            {"job_id": "c", "status": "missing"},
        ]}
        self.assertEqual([j["job_id"] for j in jobs_attention_jobs(snapshot)], ["b", "c"])

    def test_schedule_owner_map(self) -> None:
        snapshot = {"jobs": [
            {"job_id": "JOB-A", "definition": {"schedules": [{"id": "sched-a"}]}},
            {"job_id": "JOB-B", "definition": {"schedules": [{"schedule_id": "sched-b"}]}},
            {"job_id": "JOB-C", "definition": {"schedules": [{}]}},
        ]}
        mapping = schedule_owner_map(snapshot)
        self.assertEqual(mapping.get("sched-a"), "JOB-A")
        self.assertEqual(mapping.get("sched-b"), "JOB-B")

    def test_summarize_apply_report(self) -> None:
        report = {"created_schedule_ids": ["x"], "updated_schedule_ids": ["y", "z"], "deleted_schedule_ids": []}
        self.assertEqual(
            summarize_apply_report(report),
            {"created": 1, "updated": 2, "deleted": 0, "total": 3},
        )
        self.assertEqual(summarize_apply_report({}), {"created": 0, "updated": 0, "deleted": 0, "total": 0})


if __name__ == "__main__":
    unittest.main()
