"""Obsidian-operator skill: GitHub issue-title round-trip hardening.

Background (Finding B from Tracking/Task-0013/Testing/GITHUB-SYNC-FAILURE.md):
``Sync-TaskToGitHubIssue.ps1`` used to write the issue title with
``gh issue edit --title $issueTitle`` and then assert an exact-match readback.
Under Windows PowerShell 5.1 (the edition the skill scripts target), native
command-argument quoting silently *strips embedded double-quotes* before the
argument ever reaches ``gh`` -- e.g. ``Harden "quoted" sync`` is delivered to the
child process as ``Harden quoted sync``. GitHub therefore stored a title that
differed from the local title, so the literal ``$view.title -ne $issueTitle``
readback could never pass for a quote-containing title: a permanent
mismatch / false ``text_conflict``.

The hardening has two parts, both exercised here without any live GitHub call:

1. Send-side: the title is now PATCHed via ``gh api --input -`` with a JSON body
   on stdin (like the create path in ``Bootstrap-TaskGitHubIssues.ps1``), which
   is never subject to native-argument quoting, so the literal title round-trips.
2. Compare-side: both the expected title and the live readback are passed through
   ``Normalize-TitleForCompare`` before comparison, so faithful titles (quotes,
   colons, ampersands) never produce a false readback failure / text_conflict.

These tests run the script's deterministic ``-DryRun`` title encode/compare seam
under Windows PowerShell 5.1 and assert the round-trip. They also include a
negative control that reproduces the original native-arg quote-stripping bug, to
prove the regression is real and that the JSON-stdin path avoids it.
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[1]
SYNC_SCRIPT = (
    REPO_ROOT / "skills" / "obsidian-operator" / "scripts" / "Sync-TaskToGitHubIssue.ps1"
)

# Titles that must survive a faithful round-trip. The double-quote case is the
# Task-0013 bug; colon and ampersand are the "ideally also" special chars.
TITLE_CASES = {
    "double_quote": 'Harden the "quoted" round-trip',
    "colon": "Sync subsystem: deeper coverage",
    "ampersand": "Bootstrap & reconcile parity",
    "all_special": 'Title with "quotes", colon: and & ampersand',
}


def _windows_powershell() -> str | None:
    """Resolve Windows PowerShell 5.1 (powershell.exe), the edition the skill
    scripts target and where the native-argument quote-stripping bug lives."""
    return shutil.which("powershell") or shutil.which("powershell.exe")


def _clean_powershell_env() -> dict[str, str]:
    """Return an environment whose ``PSModulePath`` lets Windows PowerShell 5.1
    autoload its own modules (``Microsoft.PowerShell.Utility`` -> ``Get-FileHash``).

    When these tests run nested inside a PowerShell 7 host, the inherited
    ``PSModulePath`` front-loads pwsh7 module directories and breaks 5.1's
    module autoload, so a 5.1 subprocess fails on ``Get-FileHash`` even though it
    ships the cmdlet. Production never launches the script with a pwsh7-poisoned
    ``PSModulePath``; sanitizing it here keeps the test faithful to the script's
    real runtime instead of testing the harness's environment quirk.
    """
    env = os.environ.copy()
    system_root = env.get("SystemRoot", r"C:\Windows")
    env["PSModulePath"] = os.pathsep.join(
        [
            str(Path(system_root) / "system32" / "WindowsPowerShell" / "v1.0" / "Modules"),
            str(Path(env.get("ProgramFiles", r"C:\Program Files")) / "WindowsPowerShell" / "Modules"),
        ]
    )
    return env


class _SyncDryRunFixture:
    """A throwaway git repo containing a manifest and a single TASK.md whose
    ``## Title`` is the supplied title."""

    MANIFEST = {
        "schema_version": 1,
        "manifest_type": "codex_repo_registry",
        "repos": [
            {
                "id": "CodexDashboard",
                "local_root": "fixture",
                "task_provider": {
                    "kind": "github_issues",
                    "host": "github.com",
                    "repo": "Digital-Collective-Games/Obsidian",
                },
            }
        ],
    }

    def __init__(self, title: str, task_number: int = 99) -> None:
        self.title = title
        self.task_number = task_number
        self._tmp = tempfile.TemporaryDirectory()
        self.root = Path(self._tmp.name)

    def __enter__(self) -> "_SyncDryRunFixture":
        (self.root / "CODEX-REPO-MANIFEST.json").write_text(
            json.dumps(self.MANIFEST), encoding="utf-8"
        )
        task_dir = self.root / "Tracking" / f"Task-{self.task_number:04d}"
        task_dir.mkdir(parents=True, exist_ok=True)
        (task_dir / "TASK.md").write_text(
            f"# Task-{self.task_number:04d}\n\n## Title\n\n{self.title}\n\n"
            "## Summary\n\nFixture for title round-trip tests.\n",
            encoding="utf-8",
        )
        # The script reads `git rev-parse HEAD`; make the fixture a real repo.
        env = {**os.environ, "GIT_CONFIG_NOSYSTEM": "1"}
        for args in (
            ["git", "init", "-q"],
            ["git", "-c", "user.email=t@t.test", "-c", "user.name=t", "add", "-A"],
            ["git", "-c", "user.email=t@t.test", "-c", "user.name=t", "commit", "-q", "-m", "fixture"],
        ):
            subprocess.run(args, cwd=self.root, check=True, capture_output=True, env=env)
        return self

    def __exit__(self, *exc: object) -> None:
        self._tmp.cleanup()


class TitleRoundTripDryRunTests(unittest.TestCase):
    """The script's title encode/compare seam round-trips special-char titles
    under Windows PowerShell 5.1, with no live GitHub call."""

    def _run_dry_run(self, title: str) -> dict:
        executable = _windows_powershell()
        if executable is None:
            self.skipTest("Windows PowerShell is not available")
        with _SyncDryRunFixture(title) as fx:
            completed = subprocess.run(
                [
                    executable,
                    "-NoProfile",
                    "-ExecutionPolicy",
                    "Bypass",
                    "-File",
                    str(SYNC_SCRIPT),
                    "-TaskPath",
                    "Tracking/Task-0099/TASK.md",
                    "-ManifestPath",
                    "CODEX-REPO-MANIFEST.json",
                    "-OutputBodyPath",
                    str(fx.root / "out-body.md"),
                    "-MetadataPath",
                    str(fx.root / "out-meta.json"),
                    "-DryRun",
                ],
                cwd=str(fx.root),
                check=False,
                capture_output=True,
                text=True,
                timeout=60,
                env=_clean_powershell_env(),
            )
        self.assertEqual(
            completed.returncode,
            0,
            f"dry-run failed:\nSTDOUT:\n{completed.stdout}\nSTDERR:\n{completed.stderr}",
        )
        return json.loads(completed.stdout)

    def test_quote_colon_ampersand_titles_round_trip_and_compare_equal(self) -> None:
        for name, title in TITLE_CASES.items():
            with self.subTest(case=name):
                data = self._run_dry_run(title)
                expected = f"Task-0099: {title}"
                # The rendered issue title preserves the special chars verbatim.
                self.assertEqual(data["title"], expected)
                # The JSON-stdin send payload reconstructs the identical title
                # (this is what GitHub's JSON parser receives) -- the old
                # `--title $issueTitle` native-arg path would have lost the quotes.
                self.assertEqual(data["send_payload_title"], expected)
                self.assertTrue(
                    data["send_payload_round_trips"],
                    f"send payload did not round-trip for {name!r}: {data}",
                )
                # The normalized comparison used by the readback gate also matches.
                self.assertTrue(
                    data["send_payload_compares_equal"],
                    f"normalized compare failed for {name!r}: {data}",
                )

    def test_double_quote_is_not_stripped_from_rendered_title(self) -> None:
        data = self._run_dry_run(TITLE_CASES["double_quote"])
        self.assertIn('"quoted"', data["title"])
        # Normalized form keeps the quotes (it only collapses whitespace).
        self.assertIn('"quoted"', data["title_normalized"])


class NativeArgRegressionControlTests(unittest.TestCase):
    """Negative control: prove the original bug (native `--title $x` strips
    embedded double-quotes under Windows PowerShell 5.1) is real, and that the
    JSON-stdin encoding the fix uses does NOT lose the quotes."""

    def _argv_via_native(self, value: str) -> list[str]:
        """Return argv exactly as Windows PowerShell 5.1 delivers it to a native
        executable, the way the OLD ``gh ... --title $issueTitle`` call did.

        We use the Python interpreter (a real native exe) as the callee and have
        it dump ``sys.argv`` as JSON, invoked through the 5.1 engine so the
        argument is subject to 5.1's native-command quoting -- the exact path
        that dropped the embedded double-quotes.
        """
        executable = _windows_powershell()
        if executable is None:
            self.skipTest("Windows PowerShell is not available")
        python_exe = shutil.which("python") or shutil.which("python.exe")
        if python_exe is None:
            self.skipTest("python is not available as a native callee")
        script = (
            f"& '{python_exe}' -c "
            "\"import sys,json;print(json.dumps(sys.argv[1:]))\" "
            f"--title {self._ps_single_quote(value)}"
        )
        completed = subprocess.run(
            [executable, "-NoProfile", "-Command", script],
            check=True,
            capture_output=True,
            text=True,
            timeout=30,
            env=_clean_powershell_env(),
        )
        return json.loads(completed.stdout.strip().splitlines()[-1])

    @staticmethod
    def _ps_single_quote(value: str) -> str:
        return "'" + value.replace("'", "''") + "'"

    def test_native_arg_strips_embedded_double_quotes(self) -> None:
        # This documents WHY the old send path was broken: the embedded quotes
        # vanish when passed as a native argument under Windows PowerShell 5.1.
        argv = self._argv_via_native('Harden "quoted" sync')
        self.assertEqual(argv[0], "--title")
        delivered = argv[1]
        # The bug: the embedded double-quotes are gone.
        self.assertNotIn('"', delivered)
        self.assertEqual(delivered, "Harden quoted sync")

    def test_json_stdin_payload_preserves_double_quotes(self) -> None:
        # The fix path encodes the title into JSON delivered on stdin, which is
        # immune to native-arg quoting. Round-trip through json proves fidelity.
        title = 'Harden "quoted" sync'
        encoded = json.dumps({"title": title})
        self.assertIn('\\"quoted\\"', encoded)
        self.assertEqual(json.loads(encoded)["title"], title)


class NormalizeTitleFunctionTests(unittest.TestCase):
    """Unit-test the Normalize-TitleForCompare helper directly: it must collapse
    whitespace/line-endings but never strip content characters."""

    def _normalize(self, value: str) -> str:
        executable = _windows_powershell()
        if executable is None:
            self.skipTest("Windows PowerShell is not available")
        # Dot-source the script's functions without executing its body by parsing
        # out the function and invoking it. Simpler: define the same logic inline
        # is NOT allowed (it must be the script's function), so we dot-source the
        # script with a guard. The script runs top-to-bottom, so instead we
        # extract and invoke the function via the AST.
        script = (
            "$ErrorActionPreference='Stop';"
            "$src = Get-Content -Raw -LiteralPath "
            f"'{SYNC_SCRIPT}';"
            "$ast = [System.Management.Automation.Language.Parser]::ParseInput("
            "$src,[ref]$null,[ref]$null);"
            "$fn = $ast.Find({param($n) "
            "$n -is [System.Management.Automation.Language.FunctionDefinitionAst] "
            "-and $n.Name -eq 'Normalize-TitleForCompare'}, $true);"
            "Invoke-Expression $fn.Extent.Text;"
            f"Normalize-TitleForCompare -Title {self._ps_single_quote(value)}"
        )
        completed = subprocess.run(
            [executable, "-NoProfile", "-Command", script],
            check=True,
            capture_output=True,
            text=True,
            timeout=30,
            env=_clean_powershell_env(),
        )
        # PowerShell appends a trailing newline.
        return completed.stdout.rstrip("\r\n")

    @staticmethod
    def _ps_single_quote(value: str) -> str:
        return "'" + value.replace("'", "''") + "'"

    def test_collapses_internal_whitespace_runs(self) -> None:
        self.assertEqual(self._normalize("Task-0099:   spaced    out"), "Task-0099: spaced out")

    def test_trims_leading_and_trailing_whitespace(self) -> None:
        self.assertEqual(self._normalize("  trimmed  "), "trimmed")

    def test_preserves_quotes_colons_ampersands(self) -> None:
        self.assertEqual(
            self._normalize('a "q" : b & c'),
            'a "q" : b & c',
        )


if __name__ == "__main__":
    unittest.main()
