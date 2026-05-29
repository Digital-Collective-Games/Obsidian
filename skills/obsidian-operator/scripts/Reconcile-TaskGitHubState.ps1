param(
    [string]$TaskRoot = "Tracking",
    [string]$RepoId = "CodexDashboard",
    [string]$ManifestPath = "CODEX-REPO-MANIFEST.json",
    [string]$SyncScriptPath = "skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1",
    [string]$OutputDir = "Tracking/Task-0012/Testing/TaskGitHubReconcile",
    [string]$OutputPath,
    [string]$DifferenceReportPath,
    [string]$DispatchReportPath,
    [int]$Limit = 1000,
    [switch]$DryRun,
    [switch]$DispatchActions,
    [switch]$NoOutputFile,
    [switch]$NoDifferenceReport,
    [switch]$NoDispatchReport
)

Set-StrictMode -Version 3.0
$ErrorActionPreference = "Stop"
# gh emits UTF-8 stdout. Under Windows PowerShell 5.1 the console defaults to a
# non-UTF-8 code page, which mojibakes smart quotes on native-command capture
# and produces false text_conflict readbacks. Force UTF-8 console encoding so
# gh output decodes/encodes correctly regardless of host code page.
try {
    [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
    [Console]::InputEncoding = [System.Text.UTF8Encoding]::new($false)
} catch {
}
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)
if (Get-Variable -Name PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue) {
    $PSNativeCommandUseErrorActionPreference = $false
}

function Write-Utf8NoBom {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,

        [Parameter(Mandatory = $true)]
        [string]$Value
    )

    $encoding = [System.Text.UTF8Encoding]::new($false)
    [System.IO.File]::WriteAllText($Path, $Value, $encoding)
}

function Invoke-GhJson {
    param(
        [Parameter(Mandatory = $true)]
        [string[]]$Arguments,

        [switch]$AllowNotFound
    )

    $oldErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        $output = & gh @Arguments 2>&1
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldErrorActionPreference
    }

    $text = ($output | Out-String)
    if ($exitCode -eq 0) {
        if (-not $text.Trim()) {
            return $null
        }

        return ($text | ConvertFrom-Json)
    }

    if ($AllowNotFound -and ($text -match "HTTP 404" -or $text -match "not found" -or $text -match "Not Found" -or $text -match "Could not resolve to an issue or pull request")) {
        return $null
    }

    throw "gh $($Arguments -join ' ') failed. $text"
}

function Get-TaskNumber {
    param([Parameter(Mandatory = $true)] [string]$TaskDirectoryName)

    if ($TaskDirectoryName -notmatch "^Task-(\d{4})$") {
        throw "Task directory '$TaskDirectoryName' is not formatted as Task-0000."
    }

    return [int]$Matches[1]
}

function Get-RelativePathCompat {
    param(
        [Parameter(Mandatory = $true)]
        [string]$BasePath,

        [Parameter(Mandatory = $true)]
        [string]$TargetPath
    )

    $baseWithSlash = $BasePath.TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
    $baseUri = [System.Uri]::new($baseWithSlash)
    $targetUri = [System.Uri]::new($TargetPath)
    return [System.Uri]::UnescapeDataString($baseUri.MakeRelativeUri($targetUri).ToString()) -replace "\\", "/"
}

function Get-JsonProperty {
    param(
        $Object,
        [Parameter(Mandatory = $true)]
        [string]$Name
    )

    if ($null -eq $Object) {
        return $null
    }

    if ($Object.PSObject.Properties.Name -contains $Name) {
        return $Object.$Name
    }

    return $null
}

function Normalize-IssueState {
    param([string]$State)

    if (-not $State) {
        return "UNKNOWN"
    }

    return $State.ToUpperInvariant()
}

function Get-ExpectedIssueStateFromLocalStatus {
    param([string]$Status)

    if (-not $Status) {
        return "UNKNOWN"
    }

    switch ($Status.ToLowerInvariant()) {
        { $_ -in @("complete", "completed", "done", "cancelled", "canceled", "rejected") } { return "CLOSED" }
        { $_ -in @("pending", "planning", "planned", "ready", "in_progress", "blocked", "active") } { return "OPEN" }
        default { return "UNKNOWN" }
    }
}

function Get-LocalHumanNeeded {
    param($TaskState)

    $explicit = Get-JsonProperty -Object $TaskState -Name "human_needed"
    if ($null -ne $explicit) {
        if ($explicit -is [bool]) {
            return $(if ($explicit) { "Yes" } else { "No" })
        }

        switch -Regex ([string]$explicit) {
            "^(yes|true|needed|human_needed)$" { return "Yes" }
            "^(no|false|none|not_needed)$" { return "No" }
        }
    }

    $gate = [string](Get-JsonProperty -Object $TaskState -Name "current_gate")
    if ($gate -match "(?i)human|approval|review") {
        return "Yes"
    }

    $blockers = @(Get-JsonProperty -Object $TaskState -Name "blockers")
    $blockerText = ($blockers -join "`n")
    if ($blockerText -match "(?i)human|approval|review|input") {
        return "Yes"
    }

    return "No"
}

function Normalize-BodyForCompare {
    param([string]$Body)

    if ($null -eq $Body) {
        return ""
    }

    $normalized = $Body -replace "`r`n", "`n"
    $normalized = $normalized -replace '(?m)^- Source commit: `[^`]*`$', '- Source commit: `<normalized>`'
    $normalized = $normalized -replace '(?m)^- Rendered at: `[^`]*`$', '- Rendered at: `<normalized>`'
    return $normalized.Trim()
}

function Get-BodySyncMetadata {
    param([string]$Body)

    $result = [ordered]@{
        marker_task_id = $null
        marker_task_path = $null
        local_task_sha256 = $null
    }

    if (-not $Body) {
        return [pscustomobject]$result
    }

    $marker = [regex]::Match($Body, "<!--\s*task-sync:\s*repo=[^;]+;\s*task_id=(?<task>Task-\d{4});\s*task_path=(?<path>[^ ]+)\s*-->")
    if ($marker.Success) {
        $result.marker_task_id = $marker.Groups["task"].Value
        $result.marker_task_path = $marker.Groups["path"].Value
    }

    $hash = [regex]::Match($Body, '(?m)^- Local task SHA-256:\s*`(?<hash>[A-Fa-f0-9]+)`\s*$')
    if ($hash.Success) {
        $result.local_task_sha256 = $hash.Groups["hash"].Value.ToUpperInvariant()
    }

    return [pscustomobject]$result
}

function Get-IssueFieldsByName {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ProviderRepo,

        [Parameter(Mandatory = $true)]
        [int]$IssueNumber,

        [Parameter(Mandatory = $true)]
        [hashtable]$FieldNameById
    )

    $values = Invoke-GhJson -Arguments @("api", "/repos/$ProviderRepo/issues/$IssueNumber/issue-field-values")
    $result = @{}
    foreach ($value in @($values)) {
        $fieldId = [int]$value.issue_field_id
        if (-not $FieldNameById.ContainsKey($fieldId)) {
            continue
        }

        $fieldName = $FieldNameById[$fieldId]
        $optionName = $null
        if ($value.PSObject.Properties.Name -contains "single_select_option" -and $null -ne $value.single_select_option) {
            $optionName = [string]$value.single_select_option.name
        } elseif ($value.PSObject.Properties.Name -contains "value") {
            $optionName = [string]$value.value
        }

        $result[$fieldName] = $optionName
    }

    return $result
}

function Add-Action {
    param(
        [System.Collections.Generic.List[object]]$Actions,

        [Parameter(Mandatory = $true)]
        [string]$TaskId,

        [Parameter(Mandatory = $true)]
        [string]$Type,

        [Parameter(Mandatory = $true)]
        [string]$Summary,

        [string]$Authority = "reconcile_policy",
        [hashtable]$Details = @{}
    )

    $Actions.Add([pscustomobject]@{
        task_id = $TaskId
        type = $Type
        authority = $Authority
        summary = $Summary
        details = $Details
    })
}

function Test-ConflictActionType {
    param([string]$Type)

    return ($Type -in @(
        "manual_status_reconcile_required",
        "repair_or_review_issue_marker",
        "repair_task_meta_binding",
        "text_conflict"
    ))
}

function New-TextDiffDetails {
    param(
        [Parameter(Mandatory = $true)]
        [string]$TaskId,

        [Parameter(Mandatory = $true)]
        [string]$RemoteBodyPath,

        [Parameter(Mandatory = $true)]
        [string]$LocalRenderedBodyPath,

        [Parameter(Mandatory = $true)]
        [string]$OutputDirectory
    )

    $diffPath = Join-Path $OutputDirectory "$TaskId-TEXT-DIFF.patch"

    $oldErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        $diffOutput = & git diff --no-index --no-ext-diff -- $RemoteBodyPath $LocalRenderedBodyPath 2>&1
        $diffExitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldErrorActionPreference
    }

    if ($diffExitCode -gt 1) {
        throw "git diff --no-index failed for $TaskId. $($diffOutput | Out-String)"
    }

    Write-Utf8NoBom -Path $diffPath -Value ($diffOutput | Out-String)

    return @{
        local_rendered_body_path = $LocalRenderedBodyPath
        remote_body_path = $RemoteBodyPath
        diff_path = $diffPath
        diff_exit_code = $diffExitCode
    }
}

function Format-ActionDetailForMarkdown {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name,

        $Value,

        [Parameter(Mandatory = $true)]
        [string]$ReportDirectory
    )

    if ($null -eq $Value) {
        return '`<null>`'
    }

    if ($Name -match "_path$" -and $Value -is [string] -and (Test-Path -LiteralPath $Value)) {
        $resolvedPath = (Resolve-Path -LiteralPath $Value).Path
        $relativePath = Get-RelativePathCompat -BasePath $ReportDirectory -TargetPath $resolvedPath
        return "[$relativePath](./$relativePath)"
    }

    if ($Value -is [array] -or $Value -is [hashtable] -or $Value -is [pscustomobject]) {
        return ('`{0}`' -f ($Value | ConvertTo-Json -Depth 8 -Compress))
    }

    return ('`{0}`' -f $Value)
}

function Write-DifferenceReport {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,

        [Parameter(Mandatory = $true)]
        $Result
    )

    $reportDirectory = Split-Path -Parent $Path
    New-Item -ItemType Directory -Force -Path $reportDirectory | Out-Null

    $lines = [System.Collections.Generic.List[string]]::new()
    $lines.Add("# Task/GitHub Reconcile Differences")
    $lines.Add("")
    $lines.Add(('- Generated at: `{0}`' -f $Result.generated_at))
    $lines.Add(('- Repo: `{0}`' -f $Result.provider_repo))
    $lines.Add(('- Local tasks: `{0}`' -f $Result.summary.local_task_count))
    $lines.Add(('- Remote issues: `{0}`' -f $Result.summary.remote_issue_count))
    $lines.Add(('- Difference count: `{0}`' -f $Result.summary.difference_count))
    $lines.Add(('- Conflict count: `{0}`' -f $Result.summary.conflict_count))
    $lines.Add("")
    $lines.Add("## Policy")
    $lines.Add("")
    $lines.Add("- Issue state: $($Result.policy.issue_state)")
    $lines.Add("- Text: $($Result.policy.text)")
    $lines.Add("- Priority: $($Result.policy.priority)")
    $lines.Add("- Queue: $($Result.policy.queue)")
    $lines.Add("- Human Needed: $($Result.policy.human_needed)")
    $lines.Add("")
    $lines.Add("## Differences")
    $lines.Add("")

    if (@($Result.differences).Count -eq 0) {
        $lines.Add("No differences are currently reported.")
    }

    foreach ($difference in @($Result.differences)) {
        $lines.Add("### $($difference.task_id): $($difference.type)")
        $lines.Add("")
        $lines.Add(('- Authority: `{0}`' -f $difference.authority))
        $lines.Add("- Summary: $($difference.summary)")

        $details = $difference.details
        if ($details) {
            if ($details -is [hashtable]) {
                $detailItems = @($details.GetEnumerator() | Sort-Object Name)
            } else {
                $detailItems = @($details.PSObject.Properties | Sort-Object Name)
            }

            foreach ($detailItem in $detailItems) {
                $formatted = Format-ActionDetailForMarkdown -Name $detailItem.Name -Value $detailItem.Value -ReportDirectory $reportDirectory
                $lines.Add("- $($detailItem.Name): $formatted")
            }
        }

        $lines.Add("")
    }

    Write-Utf8NoBom -Path $Path -Value ($lines -join "`n")
}

function Quote-PowerShellArgument {
    param([string]$Value)

    if ($null -eq $Value -or $Value.Length -eq 0) {
        return "''"
    }

    if ($Value -match '^[A-Za-z0-9_\-./:\\#]+$') {
        return $Value
    }

    return "'$($Value.Replace("'", "''"))'"
}

function Join-CommandLine {
    param([string[]]$Parts)

    return (($Parts | ForEach-Object { Quote-PowerShellArgument -Value $_ }) -join " ")
}

function Get-ActionDetailValue {
    param(
        [Parameter(Mandatory = $true)]
        $Action,

        [Parameter(Mandatory = $true)]
        [string]$Name
    )

    $details = $Action.details
    if ($null -eq $details) {
        return $null
    }

    if ($details -is [hashtable]) {
        if ($details.ContainsKey($Name)) {
            return $details[$Name]
        }

        return $null
    }

    if ($details.PSObject.Properties.Name -contains $Name) {
        return $details.$Name
    }

    return $null
}

function Get-TaskNumberFromTaskId {
    param([Parameter(Mandatory = $true)] [string]$TaskId)

    if ($TaskId -notmatch "^Task-(\d{4})$") {
        throw "Task id '$TaskId' is not formatted as Task-0000."
    }

    return [int]$Matches[1]
}

function New-DispatchPlanItem {
    param(
        [Parameter(Mandatory = $true)]
        $Action,

        [Parameter(Mandatory = $true)]
        [string]$ProviderRepo,

        [Parameter(Mandatory = $true)]
        [string]$RepoId,

        [Parameter(Mandatory = $true)]
        [string]$ManifestPath,

        [Parameter(Mandatory = $true)]
        [string]$SyncScriptPath,

        [Parameter(Mandatory = $true)]
        [hashtable]$FieldIdByName,

        [hashtable]$BlockingActionsByTask = @{}
    )

    $taskId = [string]$Action.task_id
    $issueNumber = Get-ActionDetailValue -Action $Action -Name "issue_number"
    if ($null -eq $issueNumber) {
        $issueNumber = Get-TaskNumberFromTaskId -TaskId $taskId
    }

    $taskPath = "Tracking/$taskId/TASK.md"
    $taskStatePath = "Tracking/$taskId/TASK-STATE.json"
    $mode = "dry_run"
    $executor = "codex"
    $command = $null
    $codexStep = $null
    $writes = @()
    $blockedBy = @()

    if ($BlockingActionsByTask.ContainsKey($taskId)) {
        $blockedBy = @($BlockingActionsByTask[$taskId])
    }

    $isBlockingAction = ([string]$Action.type -in @($blockedBy))
    if (@($blockedBy).Count -gt 0 -and -not $isBlockingAction) {
        return [pscustomobject]@{
            task_id = $taskId
            difference_type = [string]$Action.type
            mode = $mode
            executor = "blocked"
            command = $null
            codex_step = "Do not dispatch this difference until these conflicts are resolved: $($blockedBy -join ', ')."
            writes = @("blocked")
            blocked_by = @($blockedBy)
        }
    }

    $dispatchBlockedBy = @()
    if (-not $isBlockingAction) {
        $dispatchBlockedBy = @($blockedBy)
    }

    switch ([string]$Action.type) {
        "close_github_issue_to_match_local_task_state" {
            $localStatus = Get-ActionDetailValue -Action $Action -Name "local_status"
            if ($localStatus -in @("cancelled", "canceled", "rejected")) {
                $closeReason = "not planned"
            } else {
                $closeReason = "completed"
            }

            $comment = "Closed by task reconcile: local $taskStatePath status is '$localStatus'."
            $executor = "gh"
            $command = Join-CommandLine -Parts @("gh", "issue", "close", ([string]$issueNumber), "--repo", $ProviderRepo, "--reason", $closeReason, "--comment", $comment)
            $codexStep = "After the command succeeds, read back $ProviderRepo#$issueNumber and refresh Tracking/$taskId/TASK-META.json from the live provider binding readback."
            $writes = @("github_issue_state", "github_issue_comment", "local_task_meta")
        }
        "reopen_github_issue_to_match_local_task_state" {
            $localStatus = Get-ActionDetailValue -Action $Action -Name "local_status"
            $comment = "Reopened by task reconcile: local $taskStatePath status is '$localStatus'."
            $executor = "gh"
            $command = Join-CommandLine -Parts @("gh", "issue", "reopen", ([string]$issueNumber), "--repo", $ProviderRepo, "--comment", $comment)
            $codexStep = "After the command succeeds, read back $ProviderRepo#$issueNumber and refresh Tracking/$taskId/TASK-META.json from the live provider binding readback."
            $writes = @("github_issue_state", "github_issue_comment", "local_task_meta")
        }
        "create_github_issue" {
            $executor = "script"
            $command = Join-CommandLine -Parts @("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "skills/obsidian-operator/scripts/Bootstrap-TaskGitHubIssues.ps1", "-StartTaskNumber", ([string]$issueNumber), "-EndTaskNumber", ([string]$issueNumber))
            $writes = @("github_issue", "local_task_meta")
        }
        "sync_github_text_from_local_task" {
            $executor = "script"
            $command = Join-CommandLine -Parts @("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $SyncScriptPath, "-TaskPath", $taskPath, "-RepoId", $RepoId, "-ManifestPath", $ManifestPath)
            $writes = @("github_issue_text", "local_task_meta")
        }
        "remove_unsupported_issue_labels" {
            $labels = @(Get-ActionDetailValue -Action $Action -Name "labels")
            $executor = "gh"
            $command = Join-CommandLine -Parts @("gh", "issue", "edit", ([string]$issueNumber), "--repo", $ProviderRepo, "--remove-label", ($labels -join ","))
            $writes = @("github_issue_labels")
        }
        "set_github_human_needed_from_codex" {
            $fieldValue = [string](Get-ActionDetailValue -Action $Action -Name "local_human_needed")
            $fieldId = $FieldIdByName["Human Needed"]
            if ($fieldId) {
                $payload = "@{ issue_field_values = @(@{ field_id = $fieldId; value = '$fieldValue' }) } | ConvertTo-Json -Depth 5"
                $apiPath = "/repos/$ProviderRepo/issues/$issueNumber/issue-field-values"
                $executor = "gh"
                $command = "$payload | gh api -X POST $(Quote-PowerShellArgument -Value $apiPath) --input -"
                $writes = @("github_issue_field:Human Needed")
            } else {
                $codexStep = "Resolve the organization issue field id for Human Needed, then set $ProviderRepo#$issueNumber to '$fieldValue' through the GitHub issue-fields API."
                $writes = @("github_issue_field:Human Needed")
            }
        }
        "update_local_priority_from_github" {
            $githubPriority = Get-ActionDetailValue -Action $Action -Name "github_priority"
            $codexStep = "Edit $taskStatePath so local priority matches GitHub Issue Field Priority value '$githubPriority'."
            $writes = @("local_task_state")
        }
        "update_local_queue_from_github" {
            $githubQueue = Get-ActionDetailValue -Action $Action -Name "github_queue"
            $codexStep = "Edit $taskStatePath so local queue matches GitHub Issue Field Queue value '$githubQueue'."
            $writes = @("local_task_state")
        }
        "write_task_meta_from_github_readback" {
            $codexStep = "Write Tracking/$taskId/TASK-META.json from a fresh gh issue view $issueNumber --repo $ProviderRepo readback after verifying the issue marker still matches $taskPath."
            $writes = @("local_task_meta")
        }
        "repair_task_meta_binding" {
            $codexStep = "Manually inspect Tracking/$taskId/TASK-META.json and GitHub issue $ProviderRepo#$issueNumber before editing either side; provider binding mismatch can corrupt the task identity."
            $writes = @("manual_review")
        }
        "repair_or_review_issue_marker" {
            $codexStep = "Manually inspect $ProviderRepo#$issueNumber and $taskPath; do not rewrite the issue marker until the identity mismatch is understood."
            $writes = @("manual_review")
        }
        "manual_status_reconcile_required" {
            $codexStep = "Review $taskStatePath and $ProviderRepo#$issueNumber because the live GitHub state differs from the local task state; choose close, reopen, or local state edit explicitly."
            $writes = @("manual_review")
        }
        "text_conflict" {
            $diffPath = Get-ActionDetailValue -Action $Action -Name "diff_path"
            $codexStep = "Review $diffPath and resolve the text conflict before any GitHub write runs for this task."
            $writes = @("conflict_resolution")
        }
        "detach_issue_from_projects" {
            $codexStep = "Use GitHub GraphQL to delete the listed project items from $ProviderRepo#$issueNumber after confirming issue fields are still the intended queue surface."
            $writes = @("github_project_items")
        }
        "materialize_or_triage_remote_issue_without_local_task" {
            $codexStep = "Create or triage Tracking/$taskId/TASK.md for $ProviderRepo#$issueNumber; preserve the issue-number-to-folder-number identity convention."
            $writes = @("local_task_document")
        }
        default {
            $codexStep = "No dispatcher is defined for difference type '$($Action.type)'; handle manually."
            $writes = @("manual_review")
        }
    }

    return [pscustomobject]@{
        task_id = $taskId
        difference_type = [string]$Action.type
        mode = $mode
        executor = $executor
        command = $command
        codex_step = $codexStep
        writes = $writes
        blocked_by = [object[]]$dispatchBlockedBy
    }
}

function Write-DispatchReport {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,

        [Parameter(Mandatory = $true)]
        $Result
    )

    $reportDirectory = Split-Path -Parent $Path
    New-Item -ItemType Directory -Force -Path $reportDirectory | Out-Null

    $lines = [System.Collections.Generic.List[string]]::new()
    $lines.Add("# Task/GitHub Reconcile Dispatch Dry Run")
    $lines.Add("")
    $lines.Add(('- Generated at: `{0}`' -f $Result.generated_at))
    $lines.Add(('- Repo: `{0}`' -f $Result.provider_repo))
    $lines.Add(('- Dispatch item count: `{0}`' -f $Result.dispatch.item_count))
    $lines.Add("")
    $lines.Add("This is a dry run. It does not edit local files and does not edit GitHub.")
    $lines.Add("")

    if (@($Result.dispatch.items).Count -eq 0) {
        $lines.Add("No dispatch items are currently required.")
    }

    foreach ($item in @($Result.dispatch.items)) {
        $lines.Add("## $($item.task_id): $($item.difference_type)")
        $lines.Add("")
        $lines.Add(('- Executor: `{0}`' -f $item.executor))
        $lines.Add(('- Writes: `{0}`' -f (@($item.writes) -join ", ")))
        if (@($item.blocked_by).Count -gt 0) {
            $lines.Add(('- Blocked by: `{0}`' -f (@($item.blocked_by) -join ", ")))
        }

        if ($item.command) {
            $lines.Add("")
            $lines.Add("Command dry run:")
            $lines.Add("")
            $lines.Add('```powershell')
            $lines.Add($item.command)
            $lines.Add('```')
        }

        if ($item.codex_step) {
            $lines.Add("")
            $lines.Add("Codex step dry run:")
            $lines.Add("")
            $lines.Add($item.codex_step)
        }

        $lines.Add("")
    }

    Write-Utf8NoBom -Path $Path -Value ($lines -join "`n")
}

$repoRoot = (Get-Location).Path
$manifestFullPath = (Resolve-Path -LiteralPath $ManifestPath).Path
$syncScriptFullPath = (Resolve-Path -LiteralPath $SyncScriptPath).Path
$taskRootFullPath = (Resolve-Path -LiteralPath $TaskRoot).Path
$outputFullPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputDir)
New-Item -ItemType Directory -Force -Path $outputFullPath | Out-Null

$manifest = Get-Content -Raw -Encoding UTF8 -LiteralPath $manifestFullPath | ConvertFrom-Json
$repoEntry = @($manifest.repos) | Where-Object { $_.id -eq $RepoId } | Select-Object -First 1
if (-not $repoEntry) {
    throw "Repo id '$RepoId' was not found in $ManifestPath."
}

if ($repoEntry.task_provider.kind -ne "github_issues") {
    throw "Repo '$RepoId' task_provider must be github_issues."
}

$providerRepo = [string]$repoEntry.task_provider.repo
$repoParts = $providerRepo.Split("/")
if ($repoParts.Count -ne 2) {
    throw "Provider repo '$providerRepo' must be formatted as owner/name."
}

$providerOwner = $repoParts[0]
gh auth status -h github.com | Out-Null

$issueFields = Invoke-GhJson -Arguments @("api", "/orgs/$providerOwner/issue-fields")
$fieldNameById = @{}
$fieldIdByName = @{}
foreach ($field in @($issueFields)) {
    if ($field.PSObject.Properties.Name -contains "id") {
        $fieldNameById[[int]$field.id] = [string]$field.name
        $fieldIdByName[[string]$field.name] = [int]$field.id
    }
}

$remoteIssueList = Invoke-GhJson -Arguments @(
    "issue", "list",
    "--repo", $providerRepo,
    "--state", "all",
    "--limit", ([string]$Limit),
    "--json", "number,title,state,url"
)

$remoteIssueNumbers = @{}
foreach ($issue in @($remoteIssueList)) {
    $remoteIssueNumbers[[int]$issue.number] = $issue
}

$localTasks = Get-ChildItem -LiteralPath $taskRootFullPath -Directory -Filter "Task-*" |
    ForEach-Object {
        $number = Get-TaskNumber -TaskDirectoryName $_.Name
        $taskPath = Join-Path $_.FullName "TASK.md"
        if (Test-Path -LiteralPath $taskPath) {
            [pscustomobject]@{
                number = $number
                task_id = ("Task-{0:D4}" -f $number)
                task_path = $taskPath
                relative_task_path = (Get-RelativePathCompat -BasePath $repoRoot -TargetPath (Resolve-Path -LiteralPath $taskPath).Path)
                directory = $_.FullName
                task_state_path = Join-Path $_.FullName "TASK-STATE.json"
                task_meta_path = Join-Path $_.FullName "TASK-META.json"
            }
        }
    } |
    Sort-Object number

$localNumbers = @{}
foreach ($task in @($localTasks)) {
    $localNumbers[[int]$task.number] = $task
}

$actions = [System.Collections.Generic.List[object]]::new()
$taskReports = @()

foreach ($task in @($localTasks)) {
    $taskState = $null
    if (Test-Path -LiteralPath $task.task_state_path) {
        $taskState = Get-Content -Raw -Encoding UTF8 -LiteralPath $task.task_state_path | ConvertFrom-Json
    }

    $taskMeta = $null
    if (Test-Path -LiteralPath $task.task_meta_path) {
        $taskMeta = Get-Content -Raw -Encoding UTF8 -LiteralPath $task.task_meta_path | ConvertFrom-Json
    }

    $localStatus = [string](Get-JsonProperty -Object $taskState -Name "status")
    $expectedIssueState = Get-ExpectedIssueStateFromLocalStatus -Status $localStatus
    $localHumanNeeded = Get-LocalHumanNeeded -TaskState $taskState
    $localHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $task.task_path).Hash.ToUpperInvariant()

    $renderedBodyPath = Join-Path $outputFullPath "$($task.task_id)-LOCAL-RENDERED-BODY.md"
    $renderJson = & $syncScriptFullPath `
        -TaskPath $task.task_path `
        -RepoId $RepoId `
        -ManifestPath $manifestFullPath `
        -OutputBodyPath $renderedBodyPath `
        -DryRun

    if ($LASTEXITCODE -ne 0) {
        throw "Dry-run render failed for $($task.task_id)."
    }

    $render = $renderJson | ConvertFrom-Json
    $renderedBody = Get-Content -Raw -Encoding UTF8 -LiteralPath $renderedBodyPath

    $remoteIssue = Invoke-GhJson -Arguments @(
        "issue", "view", ([string]$task.number),
        "--repo", $providerRepo,
        "--json", "number,title,state,url,body,labels,projectItems"
    ) -AllowNotFound

    if (-not $remoteIssue) {
        Add-Action -Actions $actions -TaskId $task.task_id -Type "create_github_issue" -Authority "local_task_document" -Summary "Local task has no matching GitHub issue #$($task.number)." -Details @{
            expected_issue_number = $task.number
            local_task_path = $task.relative_task_path
            stop_if_created_number_differs = $true
        }

        $taskReports += [pscustomobject]@{
            task_id = $task.task_id
            issue_number = $task.number
            local_status = $localStatus
            expected_issue_state = $expectedIssueState
            remote_state = $null
            queue = $null
            priority = $null
            human_needed_local = $localHumanNeeded
            human_needed_remote = $null
            text_state = "missing_remote_issue"
            difference_count = 1
        }
        continue
    }

    $remoteIssueState = Normalize-IssueState -State ([string]$remoteIssue.state)
    $fieldValues = Get-IssueFieldsByName -ProviderRepo $providerRepo -IssueNumber $task.number -FieldNameById $fieldNameById
    $remoteQueue = $fieldValues["Queue"]
    $remotePriority = $fieldValues["Priority"]
    $remoteHumanNeeded = $fieldValues["Human Needed"]

    $remoteBodyPath = Join-Path $outputFullPath "$($task.task_id)-REMOTE-BODY.md"
    $remoteViewPath = Join-Path $outputFullPath "$($task.task_id)-REMOTE-VIEW.json"
    Write-Utf8NoBom -Path $remoteBodyPath -Value ([string]$remoteIssue.body)
    Write-Utf8NoBom -Path $remoteViewPath -Value ($remoteIssue | ConvertTo-Json -Depth 8)

    $remoteSync = Get-BodySyncMetadata -Body ([string]$remoteIssue.body)
    if ($remoteSync.marker_task_id -ne $task.task_id -or $remoteSync.marker_task_path -ne $task.relative_task_path) {
        Add-Action -Actions $actions -TaskId $task.task_id -Type "repair_or_review_issue_marker" -Authority "identity_policy" -Summary "GitHub issue marker does not match local task identity." -Details @{
            issue_number = $task.number
            expected_task_id = $task.task_id
            actual_task_id = $remoteSync.marker_task_id
            expected_task_path = $task.relative_task_path
            actual_task_path = $remoteSync.marker_task_path
        }
    }

    if (-not $taskMeta) {
        Add-Action -Actions $actions -TaskId $task.task_id -Type "write_task_meta_from_github_readback" -Authority "github_readback" -Summary "Local TASK-META.json is missing." -Details @{
            issue_number = $task.number
            issue_url = [string]$remoteIssue.url
        }
    } else {
        if ([string]$taskMeta.provider_repo -ne $providerRepo -or [int]$taskMeta.issue_number -ne [int]$task.number -or [string]$taskMeta.issue_url -ne [string]$remoteIssue.url) {
            Add-Action -Actions $actions -TaskId $task.task_id -Type "repair_task_meta_binding" -Authority "github_readback" -Summary "TASK-META.json provider binding does not match GitHub readback." -Details @{
                expected_provider_repo = $providerRepo
                actual_provider_repo = [string]$taskMeta.provider_repo
                expected_issue_number = $task.number
                actual_issue_number = [int]$taskMeta.issue_number
                expected_issue_url = [string]$remoteIssue.url
                actual_issue_url = [string]$taskMeta.issue_url
            }
        }
    }

    if ($expectedIssueState -ne "UNKNOWN" -and $remoteIssueState -ne $expectedIssueState) {
        if ($expectedIssueState -eq "CLOSED") {
            Add-Action -Actions $actions -TaskId $task.task_id -Type "close_github_issue_to_match_local_task_state" -Authority "local_task_state" -Summary "Local task status '$localStatus' is terminal, but GitHub issue is open." -Details @{
                issue_number = $task.number
                local_status = $localStatus
                remote_issue_state = $remoteIssueState
            }
        } elseif ($expectedIssueState -eq "OPEN") {
            Add-Action -Actions $actions -TaskId $task.task_id -Type "reopen_github_issue_to_match_local_task_state" -Authority "local_task_state" -Summary "Local task status '$localStatus' is nonterminal, but GitHub issue is closed." -Details @{
                issue_number = $task.number
                local_status = $localStatus
                remote_issue_state = $remoteIssueState
            }
        }
    }

    $localPriority = Get-JsonProperty -Object $taskState -Name "priority"
    if ($null -ne $localPriority -and [string]$localPriority -ne [string]$remotePriority) {
        Add-Action -Actions $actions -TaskId $task.task_id -Type "update_local_priority_from_github" -Authority "github_issue_field" -Summary "Priority differs; GitHub Priority is authoritative." -Details @{
            issue_number = $task.number
            local_priority = [string]$localPriority
            github_priority = [string]$remotePriority
        }
    }

    $localQueue = Get-JsonProperty -Object $taskState -Name "queue"
    if ($null -ne $localQueue -and [string]$localQueue -ne [string]$remoteQueue) {
        Add-Action -Actions $actions -TaskId $task.task_id -Type "update_local_queue_from_github" -Authority "github_issue_field" -Summary "Queue differs; GitHub Queue is authoritative." -Details @{
            issue_number = $task.number
            local_queue = [string]$localQueue
            github_queue = [string]$remoteQueue
        }
    }

    if ([string]$remoteHumanNeeded -ne [string]$localHumanNeeded) {
        Add-Action -Actions $actions -TaskId $task.task_id -Type "set_github_human_needed_from_codex" -Authority "codex_task_state" -Summary "Human Needed differs; Codex/local task state is authoritative." -Details @{
            issue_number = $task.number
            local_human_needed = $localHumanNeeded
            github_human_needed = [string]$remoteHumanNeeded
            current_gate = [string](Get-JsonProperty -Object $taskState -Name "current_gate")
        }
    }

    $labelNames = @($remoteIssue.labels | ForEach-Object { $_.name })
    if (@($labelNames).Count -gt 0) {
        Add-Action -Actions $actions -TaskId $task.task_id -Type "remove_unsupported_issue_labels" -Authority "provider_policy" -Summary "Accepted task issue has labels; current provider policy keeps identity/queue state out of labels." -Details @{
            labels = @($labelNames)
        }
    }

    if (@($remoteIssue.projectItems).Count -gt 0) {
        Add-Action -Actions $actions -TaskId $task.task_id -Type "detach_issue_from_projects" -Authority "provider_policy" -Summary "Accepted task issue is attached to GitHub Projects; current provider policy uses Issue Fields instead." -Details @{
            project_items = @($remoteIssue.projectItems)
        }
    }

    $canonicalLocalBody = Normalize-BodyForCompare -Body $renderedBody
    $canonicalRemoteBody = Normalize-BodyForCompare -Body ([string]$remoteIssue.body)
    $bodyMatches = ($canonicalLocalBody -eq $canonicalRemoteBody)
    $titleMatches = ([string]$render.title -eq [string]$remoteIssue.title)
    $remoteHashMatchesLocal = ($remoteSync.local_task_sha256 -and $remoteSync.local_task_sha256 -eq $localHash)

    $textState = "in_sync"
    if (-not $bodyMatches -or -not $titleMatches) {
        $textDiffDetails = New-TextDiffDetails -TaskId $task.task_id -RemoteBodyPath $remoteBodyPath -LocalRenderedBodyPath $renderedBodyPath -OutputDirectory $outputFullPath
        $textState = "text_conflict"
        $details = $textDiffDetails.Clone()
        $details["title_matches"] = $titleMatches
        $details["body_matches"] = $bodyMatches
        $details["remote_embedded_task_sha256"] = $remoteSync.local_task_sha256
        $details["local_task_sha256"] = $localHash
        $details["remote_hash_matches_local"] = [bool]$remoteHashMatchesLocal
        $details["task_meta_last_synced_at"] = [string](Get-JsonProperty -Object $taskMeta -Name "last_synced_at")
        Add-Action -Actions $actions -TaskId $task.task_id -Type "text_conflict" -Authority "coowned_text" -Summary "Rendered local issue text and live GitHub issue text differ; resolve this conflict before any GitHub write runs for this task." -Details @{
            local_rendered_body_path = $details["local_rendered_body_path"]
            remote_body_path = $details["remote_body_path"]
            diff_path = $details["diff_path"]
            diff_exit_code = $details["diff_exit_code"]
            title_matches = $details["title_matches"]
            body_matches = $details["body_matches"]
            remote_embedded_task_sha256 = $details["remote_embedded_task_sha256"]
            local_task_sha256 = $details["local_task_sha256"]
            remote_hash_matches_local = $details["remote_hash_matches_local"]
            task_meta_last_synced_at = $details["task_meta_last_synced_at"]
        }
    }

    $taskReports += [pscustomobject]@{
        task_id = $task.task_id
        issue_number = $task.number
        issue_url = [string]$remoteIssue.url
        task_meta_last_synced_at = [string](Get-JsonProperty -Object $taskMeta -Name "last_synced_at")
        local_status = $localStatus
        expected_issue_state = $expectedIssueState
        remote_state = $remoteIssueState
        queue = [string]$remoteQueue
        priority = [string]$remotePriority
        human_needed_local = $localHumanNeeded
        human_needed_remote = [string]$remoteHumanNeeded
        text_state = $textState
        remote_hash_matches_local = [bool]$remoteHashMatchesLocal
        local_task_sha256 = $localHash
        remote_embedded_task_sha256 = $remoteSync.local_task_sha256
        difference_count = @($actions | Where-Object { $_.task_id -eq $task.task_id }).Count
    }
}

foreach ($remoteNumber in @($remoteIssueNumbers.Keys | Sort-Object)) {
    if (-not $localNumbers.ContainsKey([int]$remoteNumber)) {
        $remoteIssue = $remoteIssueNumbers[[int]$remoteNumber]
        Add-Action -Actions $actions -TaskId ("Task-{0:D4}" -f [int]$remoteNumber) -Type "materialize_or_triage_remote_issue_without_local_task" -Authority "github_issue" -Summary "GitHub issue #$remoteNumber has no matching local Tracking/Task folder." -Details @{
            issue_number = [int]$remoteNumber
            issue_url = [string]$remoteIssue.url
            title = [string]$remoteIssue.title
            state = Normalize-IssueState -State ([string]$remoteIssue.state)
        }
    }
}

$actionsByType = @{}
$conflictsByType = @{}
foreach ($action in @($actions)) {
    if (-not $actionsByType.ContainsKey($action.type)) {
        $actionsByType[$action.type] = 0
    }
    $actionsByType[$action.type] += 1

    if (Test-ConflictActionType -Type ([string]$action.type)) {
        if (-not $conflictsByType.ContainsKey($action.type)) {
            $conflictsByType[$action.type] = 0
        }
        $conflictsByType[$action.type] += 1
    }
}

$dispatchPlan = @()
if ($DispatchActions) {
    $blockingActionTypes = @(
        "manual_status_reconcile_required",
        "repair_or_review_issue_marker",
        "repair_task_meta_binding",
        "text_conflict"
    )
    $blockingActionsByTask = @{}
    foreach ($action in @($actions)) {
        if (Test-ConflictActionType -Type ([string]$action.type)) {
            if (-not $blockingActionsByTask.ContainsKey([string]$action.task_id)) {
                $blockingActionsByTask[[string]$action.task_id] = @()
            }

            $blockingActionsByTask[[string]$action.task_id] = @($blockingActionsByTask[[string]$action.task_id]) + [string]$action.type
        }
    }

    $dispatchPlan = @($actions | ForEach-Object {
        New-DispatchPlanItem `
            -Action $_ `
            -ProviderRepo $providerRepo `
            -RepoId $RepoId `
            -ManifestPath $ManifestPath `
            -SyncScriptPath $SyncScriptPath `
            -FieldIdByName $fieldIdByName `
            -BlockingActionsByTask $blockingActionsByTask
    })
}

$outputResultPath = $null
if (-not $NoOutputFile) {
    if ($OutputPath) {
        $outputResultPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputPath)
    } else {
        $outputResultPath = Join-Path $outputFullPath "RECONCILE-RESULT.json"
    }
}

$differenceReportFullPath = $null
if (-not $NoDifferenceReport) {
    if ($DifferenceReportPath) {
        $differenceReportFullPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($DifferenceReportPath)
    } else {
        $differenceReportFullPath = Join-Path $outputFullPath "RECONCILE-DIFFERENCES.md"
    }
}

$dispatchReportFullPath = $null
if ($DispatchActions -and -not $NoDispatchReport) {
    if ($DispatchReportPath) {
        $dispatchReportFullPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($DispatchReportPath)
    } else {
        $dispatchReportFullPath = Join-Path $outputFullPath "RECONCILE-DISPATCH-DRYRUN.md"
    }
}

$result = [pscustomobject]@{
    schema_version = 1
    mode = "dry_run"
    generated_at = (Get-Date -Format o)
    repo_id = $RepoId
    provider_repo = $providerRepo
    policy = [ordered]@{
        issue_state = "Local TASK-STATE status maps to GitHub issue open/closed state; conflicting remote changes require manual review."
        text = "Local TASK.md and GitHub issue text are co-owned; any live title/body mismatch is a text_conflict difference and blocks GitHub writes for that task until resolved."
        priority = "GitHub Issue Field 'Priority' is authoritative."
        queue = "GitHub Issue Field 'Queue' is authoritative."
        human_needed = "Codex/local task state is authoritative for GitHub Issue Field 'Human Needed'."
    }
    summary = [ordered]@{
        local_task_count = @($localTasks).Count
        remote_issue_count = @($remoteIssueList).Count
        difference_count = @($actions).Count
        differences_by_type = $actionsByType
        conflict_count = @($actions | Where-Object { Test-ConflictActionType -Type ([string]$_.type) }).Count
        conflicts_by_type = $conflictsByType
    }
    artifacts = [ordered]@{
        result_path = $outputResultPath
        difference_report_path = $differenceReportFullPath
        dispatch_report_path = $dispatchReportFullPath
    }
    dispatch = [ordered]@{
        requested = [bool]$DispatchActions
        mode = "dry_run"
        item_count = @($dispatchPlan).Count
        items = @($dispatchPlan)
    }
    tasks = $taskReports
    differences = @($actions)
}

$resultJson = $result | ConvertTo-Json -Depth 12
if (-not $NoOutputFile) {
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $outputResultPath) | Out-Null
    Write-Utf8NoBom -Path $outputResultPath -Value $resultJson
}

if (-not $NoDifferenceReport) {
    Write-DifferenceReport -Path $differenceReportFullPath -Result $result
}

if ($DispatchActions -and -not $NoDispatchReport) {
    Write-DispatchReport -Path $dispatchReportFullPath -Result $result
}

$resultJson
