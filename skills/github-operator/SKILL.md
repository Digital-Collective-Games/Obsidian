---
name: github-operator
description: Drive the GitHub web UI through the human's debug Chrome session (CDP) for issue-provider integration testing. Use when Codex must exercise a GitHub issue surface at the REAL human surface - e.g. flip an issue's Queue field to Ready in the browser to prove the queue-drain consumer dispatches, set/read org issue field state via skills/github-operator/scripts/Set-IssueFieldViaUi.ps1 and skills/github-operator/scripts/Get-IssueQueueState.ps1, or any end-to-end proof against the GitHub web interface where API-only/proxy evidence does not satisfy the regression. The human only authenticates the debug Chrome profile; the agent drives the UI.
---

# GitHub Operator

## Purpose

Drive the GitHub **web UI** through the human's debug Chrome session (Chrome
DevTools Protocol over the debug endpoint at `http://127.0.0.1:9222`) so that
issue-provider integration tests are exercised at the **real human surface**.

The division of labor is fixed:

- the **human** only **authenticates** the debug Chrome profile (logs into
  GitHub once); they do not click through the test
- the **agent drives** the UI end-to-end via CDP

This exists because, for integration testing of an issue PROVIDER surface (the
GitHub web interface), there is NO pass or excuse to skip exercising it
end-to-end at that surface. Proxy/API-only evidence does not satisfy these
regressions. Reading state back through the API is fine for *verification*; the
WRITE/flip under test must happen at the real surface.

## Prerequisites

- The debug Chrome profile from
  `C:\Agent\Orchestrator\Scripts\Start-ChromeDebugProfile.ps1` must be running and
  listening on port `9222`:

  ```powershell
  powershell -NoProfile -ExecutionPolicy Bypass -File C:\Agent\Orchestrator\Scripts\Start-ChromeDebugProfile.ps1
  ```

- GitHub must be **logged in** in that debug Chrome profile (the human does this
  once; the profile persists the session).
- `gh` must be authenticated for the API readback used by `-VerifyApi` and by
  `Get-IssueQueueState.ps1` (`gh auth status -h github.com`).
- The issue's org **issue field** only renders in the UI once the issue has an
  issue **type** AND an initial field **value**. That setup is done separately
  (see `skills/obsidian-operator/SKILL.md` and its sync scripts); this skill only
  flips an already-renderable field.

## How the CDP session is selected

`GET /json/list` lists tabs; the scripts pick the `type==page` tab whose `url`
contains the target repo (or navigate it), so an operation never lands on the
wrong GitHub tab when several are open. If more than one tab matches, the scripts
stop and ask for an explicit `-TargetId`.

## Scripts

- `skills/github-operator/scripts/CdpCommon.ps1`
  - Reusable, dot-sourceable CDP helpers: `Get-CdpTargets`, `Select-CdpTarget`
    (`-UrlContains` / `-TargetId`), `New-CdpSocket`, `Invoke-CdpCommand`,
    `Invoke-CdpEval` (Runtime.evaluate, returnByValue + awaitPromise -> returns the
    value), `Invoke-CdpClick`, `Send-CdpKey`, and `Close-CdpSocket`.
  - Applies the PowerShell void-suppression fix: every async
    `ConnectAsync/SendAsync(...).GetAwaiter().GetResult()` is suppressed with
    `$null =` and the socket is returned with `return ,$ws`, so the socket is
    never silently turned into an array. Receives use a per-read
    `CancellationTokenSource` timeout so a missing response cannot hang.
- `skills/github-operator/scripts/Set-IssueFieldViaUi.ps1`
  - The WRITE/flip exercised at the real surface. Flips one issue's single-select
    org field (default `Queue`) to an option (e.g. `Ready` / `Never`) by driving
    the browser, then verifies by re-reading the control text.
  - Params: `-Repo owner/name`, `-IssueNumber`, `-FieldName` (default `Queue`),
    `-OptionName`, `-Endpoint` (default `http://127.0.0.1:9222`), `-TargetId`,
    `-VerifyApi`.
  - Implements the proven sequence: targets the right tab (by url contains repo)
    or navigates -> polls for `button[aria-label="Edit <FieldName>"]` to render ->
    clicks it -> clicks the `li[role="option"]` whose text starts with the option
    name -> sends Escape to commit -> re-reads the control text and expects
    `<FieldName><OptionName>`. With `-VerifyApi` it also reads back the stored
    value via the org issue-field-values API and confirms.
  - Returns a structured result (`Committed` true/false, `ObservedButtonText`,
    and the API readback when `-VerifyApi`).
  - Clear error if the field control never renders (hint: the issue needs an issue
    type plus an initial field value).

  ```powershell
  powershell -NoProfile -ExecutionPolicy Bypass -File skills/github-operator/scripts/Set-IssueFieldViaUi.ps1 -Repo <owner>/QueueDrainTestbed -IssueNumber 1 -FieldName Queue -OptionName Ready -VerifyApi
  ```

- `skills/github-operator/scripts/Get-IssueQueueState.ps1`
  - Reads the issue's field value via the org issue-field-values API
    (`GET /repos/<repo>/issues/<n>/issue-field-values`; the stored value is a
    numeric option id, the human-readable name is `single_select_option.name`,
    decoded the same way as `Reconcile-TaskGitHubState.ps1`). Prints
    `<FieldName> -> <option name>` (or `unset`). `-ValueOnly` emits just the name.
  - Params: `-Repo`, `-IssueNumber`, `-FieldName` (default `Queue`), `-ValueOnly`.
  - Reading via API is allowed; the UI-only rule applies to the WRITE/flip.

  ```powershell
  powershell -NoProfile -ExecutionPolicy Bypass -File skills/github-operator/scripts/Get-IssueQueueState.ps1 -Repo <owner>/QueueDrainTestbed -IssueNumber 1
  ```

## Guardrails

- Operate only the **intended tab**. Select by `-UrlContains <repo>` or
  `-TargetId`; if multiple tabs match, stop and disambiguate. Never drive a tab
  pointed at the wrong repo.
- Use **throwaway/test issues only for writes** (e.g. the `QueueDrainTestbed`
  repo). Never drive the human's **production** issues (`Digital-Collective-Games/Obsidian`
  and other live repos) without explicit authorization for that specific run.
- The flip is a real GitHub write. Confirm the repo, issue number, field, and
  option before running, and prefer a `Get-IssueQueueState.ps1` read first.
- Verifying via the API is fine, but it does not replace exercising the WRITE at
  the real surface; the surface flip is the proof (see repo-root `TESTING.md` and
  `REGRESSION.md` case `REG-007`).
- Link issue URLs and local artifacts when reporting results.
