param(
    [string]$TaskRoot = "Tracking",
    [string]$RepoId = "CodexDashboard",
    [string]$ManifestPath = "CODEX-REPO-MANIFEST.json",
    [string]$SyncScriptPath = "skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1",
    [int]$StartTaskNumber = 2,
    [int]$EndTaskNumber = 0,
    [string]$OutputDir = "Tracking/Task-0012/Testing/BulkIssueBootstrap",
    [string]$ResultPath,
    [string]$QueueValue = "Never",
    [string]$PriorityValue = "P2",
    [string]$HumanNeededValue = "No",
    [string]$IssueType = "Task",
    [switch]$DryRun
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

function Get-TaskNumber {
    param([Parameter(Mandatory = $true)] [string]$TaskDirectoryName)

    if ($TaskDirectoryName -notmatch "^Task-(\d{4})$") {
        throw "Task directory '$TaskDirectoryName' is not formatted as Task-0000."
    }

    return [int]$Matches[1]
}

function Get-IssueOrPullRequest {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ProviderRepo,

        [Parameter(Mandatory = $true)]
        [int]$Number
    )

    $oldErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        $output = & gh api "/repos/$ProviderRepo/issues/$Number" 2>&1
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldErrorActionPreference
    }

    $text = ($output | Out-String)
    if ($exitCode -eq 0) {
        return ($text | ConvertFrom-Json)
    }

    if ($text -match "HTTP 404" -or $text -match '"status"\s*:\s*"404"') {
        return $null
    }

    throw "Failed to inspect $ProviderRepo#$Number. $text"
}

function Assert-ExistingIssueMatchesTask {
    param(
        [Parameter(Mandatory = $true)]
        $Existing,

        [Parameter(Mandatory = $true)]
        [string]$TaskId,

        [Parameter(Mandatory = $true)]
        [string]$RelativeTaskPath
    )

    if ($Existing.PSObject.Properties.Name -contains "pull_request") {
        throw "Number #$($Existing.number) is already a pull request, not an issue. Cannot map $TaskId to it."
    }

    $body = [string]$Existing.body
    $markerPattern = "<!--\s*task-sync:\s*repo=[^;]+;\s*task_id=(?<task>Task-\d{4});\s*task_path=(?<path>[^ ]+)\s*-->"
    $marker = [regex]::Match($body, $markerPattern)
    if ($marker.Success) {
        $existingTask = $marker.Groups["task"].Value
        $existingPath = $marker.Groups["path"].Value
        if ($existingTask -ne $TaskId -or $existingPath -ne $RelativeTaskPath) {
            throw "Issue #$($Existing.number) already has marker task_id=$existingTask task_path=$existingPath, not $TaskId $RelativeTaskPath."
        }

        return
    }

    if ([string]$Existing.title -notlike "${TaskId}:*") {
        throw "Issue #$($Existing.number) already exists without a matching marker or '${TaskId}:' title."
    }
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
gh auth status -h github.com | Out-Null

$taskItems = Get-ChildItem -LiteralPath $taskRootFullPath -Directory -Filter "Task-*" |
    ForEach-Object {
        $number = Get-TaskNumber -TaskDirectoryName $_.Name
        $taskPath = Join-Path $_.FullName "TASK.md"
        if ((Test-Path -LiteralPath $taskPath) -and $number -ge $StartTaskNumber -and ($EndTaskNumber -eq 0 -or $number -le $EndTaskNumber)) {
            [pscustomobject]@{
                Number = $number
                TaskId = ("Task-{0:D4}" -f $number)
                TaskPath = $taskPath
                Directory = $_.FullName
                RelativeTaskPath = (Resolve-Path -LiteralPath $taskPath -Relative).TrimStart(".", "\", "/") -replace "\\", "/"
            }
        }
    } |
    Sort-Object Number

if (@($taskItems).Count -eq 0) {
    throw "No TASK.md files found under '$TaskRoot' for the selected range."
}

$results = @()

foreach ($task in $taskItems) {
    $bodyPath = Join-Path $outputFullPath "$($task.TaskId)-BODY.md"
    $viewPath = Join-Path $outputFullPath "$($task.TaskId)-VIEW.json"

    $renderJson = & $syncScriptFullPath `
        -TaskPath $task.TaskPath `
        -RepoId $RepoId `
        -ManifestPath $manifestFullPath `
        -OutputBodyPath $bodyPath `
        -QueueValue $QueueValue `
        -PriorityValue $PriorityValue `
        -HumanNeededValue $HumanNeededValue `
        -IssueType $IssueType `
        -DryRun

    if ($LASTEXITCODE -ne 0) {
        throw "Dry-run render failed for $($task.TaskId)."
    }

    $render = $renderJson | ConvertFrom-Json
    if ([int]$render.issue_number -ne [int]$task.Number) {
        throw "Rendered issue number '$($render.issue_number)' does not match $($task.TaskId)."
    }

    $existing = Get-IssueOrPullRequest -ProviderRepo $providerRepo -Number $task.Number
    if ($existing) {
        Assert-ExistingIssueMatchesTask -Existing $existing -TaskId $task.TaskId -RelativeTaskPath $task.RelativeTaskPath
        $action = "sync_existing"
    } else {
        $action = "create"
    }

    if ($DryRun) {
        $results += [pscustomobject]@{
            task_id = $task.TaskId
            expected_issue_number = $task.Number
            action = "dry_run_$action"
            provider_repo = $providerRepo
            title = $render.title
            body_path = $bodyPath
        }
        continue
    }

    if ($action -eq "create") {
        [string]$issueBody = Get-Content -Raw -Encoding UTF8 -LiteralPath $bodyPath
        $payloadObj = [ordered]@{
            title = [string]$render.title
            body = $issueBody
        }
        # Give the new issue its type at creation so GitHub renders the Fields
        # panel (Priority/Queue/Human Needed) immediately; a typeless issue shows
        # "No fields configured for issues without a type". The post-create sync
        # below re-asserts the same type.
        if ($IssueType) { $payloadObj["type"] = $IssueType }
        $payload = [pscustomobject]$payloadObj | ConvertTo-Json -Depth 6

        $createdJson = $payload | gh api -X POST "/repos/$providerRepo/issues" --input -
        if ($LASTEXITCODE -ne 0) {
            throw "gh issue create failed for $($task.TaskId)."
        }

        $created = $createdJson | ConvertFrom-Json
        if ([int]$created.number -ne [int]$task.Number) {
            $mismatchPath = Join-Path $outputFullPath "$($task.TaskId)-NUMBER-MISMATCH.json"
            Write-Utf8NoBom -Path $mismatchPath -Value $createdJson
            throw "GitHub returned issue #$($created.number) for $($task.TaskId), expected #$($task.Number). Stopping immediately. Details: $mismatchPath"
        }
    }

    $syncJson = & $syncScriptFullPath `
        -TaskPath $task.TaskPath `
        -RepoId $RepoId `
        -ManifestPath $manifestFullPath `
        -OutputBodyPath $bodyPath `
        -ViewOutputPath $viewPath `
        -QueueValue $QueueValue `
        -PriorityValue $PriorityValue `
        -HumanNeededValue $HumanNeededValue `
        -IssueType $IssueType

    if ($LASTEXITCODE -ne 0) {
        throw "Post-create sync failed for $($task.TaskId)."
    }

    $sync = $syncJson | ConvertFrom-Json
    if ([int]$sync.issue_number -ne [int]$task.Number) {
        throw "Sync readback issue #$($sync.issue_number) does not match $($task.TaskId)."
    }

    $results += [pscustomobject]@{
        task_id = $task.TaskId
        expected_issue_number = $task.Number
        action = $action
        provider_repo = $providerRepo
        issue_url = $sync.issue_url
        title = $sync.title
        labels = @($sync.labels)
        body_path = $bodyPath
        view_path = $viewPath
        metadata_path = $sync.metadata_path
    }
}

$summary = [pscustomobject]@{
    status = if ($DryRun) { "dry_run" } else { "bootstrapped" }
    provider_repo = $providerRepo
    task_count = @($taskItems).Count
    start_task_number = $StartTaskNumber
    end_task_number = if ($EndTaskNumber -eq 0) { $null } else { $EndTaskNumber }
    output_dir = $outputFullPath
    results = $results
}

$summaryJson = $summary | ConvertTo-Json -Depth 8
if ($ResultPath) {
    $summaryPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($ResultPath)
} elseif ($DryRun) {
    $summaryPath = Join-Path $outputFullPath "BOOTSTRAP-ISSUES-DRY-RUN.json"
} else {
    $summaryPath = Join-Path $outputFullPath "BOOTSTRAP-ISSUES-RESULT.json"
}

New-Item -ItemType Directory -Force -Path (Split-Path -Parent $summaryPath) | Out-Null
Write-Utf8NoBom -Path $summaryPath -Value $summaryJson
$summaryJson
