param(
    [Parameter(Mandatory = $true)]
    [string]$TaskPath,

    [string]$RepoId = "CodexDashboard",
    [string]$ManifestPath = "CODEX-REPO-MANIFEST.json",
    [string]$OutputBodyPath,
    [string]$ViewOutputPath,
    [string]$MetadataPath,
    [string]$QueueValue = "Never",
    [string]$PriorityValue = "P2",
    [string]$HumanNeededValue = "No",
    [switch]$SkipIssueFieldSync,
    [switch]$RepairMismatchedRemote,
    [switch]$ForceRemoteOverwrite,
    [switch]$DryRun
)

Set-StrictMode -Version 3.0
$ErrorActionPreference = "Stop"
# gh emits UTF-8 stdout and reads UTF-8 stdin. Under Windows PowerShell 5.1 the
# console defaults to a non-UTF-8 code page, which mojibakes smart quotes on
# native-command capture and produces false text_conflict readbacks. Force
# UTF-8 console encoding so gh output decodes/encodes correctly regardless of
# host code page.
try {
    [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
    [Console]::InputEncoding = [System.Text.UTF8Encoding]::new($false)
} catch {
}
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

function Get-MarkdownSection {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Content,

        [Parameter(Mandatory = $true)]
        [string]$Name
    )

    $escapedName = [regex]::Escape($Name)
    $match = [regex]::Match(
        $Content,
        "(?ms)^##\s+$escapedName\s*\r?\n(?<body>.*?)(?=^\s*##\s+|\z)"
    )

    if (-not $match.Success) {
        return $null
    }

    return $match.Groups["body"].Value.Trim()
}

function Get-FirstNonEmptyLine {
    param([string]$Text)

    foreach ($line in ($Text -split "\r?\n")) {
        $trimmed = $line.Trim()
        if ($trimmed.Length -gt 0) {
            return $trimmed
        }
    }

    return $null
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

function Assert-NoTaskIdMismatch {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Body,

        [Parameter(Mandatory = $true)]
        [string]$ExpectedTaskId,

        [Parameter(Mandatory = $true)]
        [string]$ExpectedTaskPath
    )

    $markerPattern = "<!--\s*task-sync:\s*repo=[^;]+;\s*task_id=(?<task>Task-\d{4});\s*task_path=(?<path>[^ ]+)\s*-->"
    $marker = [regex]::Match($Body, $markerPattern)
    if (-not $marker.Success) {
        throw "Generated body is missing the task-sync marker."
    }

    $markerTask = $marker.Groups["task"].Value
    if ($markerTask -ne $ExpectedTaskId) {
        throw "Generated marker task id '$markerTask' does not match '$ExpectedTaskId'."
    }

    $markerPath = $marker.Groups["path"].Value
    if ($markerPath -ne $ExpectedTaskPath) {
        throw "Generated marker task path '$markerPath' does not match '$ExpectedTaskPath'."
    }

    $h1 = [regex]::Match($Body, "(?m)^#\s+(?<task>Task-\d{4}):")
    if (-not $h1.Success) {
        throw "Generated body is missing the task-prefixed H1."
    }

    $h1Task = $h1.Groups["task"].Value
    if ($h1Task -ne $ExpectedTaskId) {
        throw "Generated H1 task id '$h1Task' does not match '$ExpectedTaskId'."
    }
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
    return [System.Uri]::UnescapeDataString($baseUri.MakeRelativeUri($targetUri).ToString())
}

function Get-LabelNames {
    param($Labels)

    return @($Labels) | ForEach-Object {
        if ($_ -is [string]) {
            $_
        } elseif ($_.PSObject.Properties.Name -contains "name") {
            [string]$_.name
        }
    }
}

function Get-RejectedLabelNames {
    param(
        [string[]]$LabelNames
    )

    return @($LabelNames | Where-Object {
        $_ -match "^codex" -or
        $_ -match "^task-\d{4}$" -or
        $_ -eq "pilot" -or
        $_ -match "^queue:" -or
        $_ -match "^priority:" -or
        $_ -eq "human-gate"
    })
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

function Normalize-TitleForCompare {
    # GitHub stores the title verbatim, but minor whitespace/line-ending drift
    # (and the historical Windows PowerShell 5.1 native-argument quote stripping
    # documented below) used to make a literal `-eq` readback compare fail for a
    # legitimate title (e.g. one containing a double-quote). Normalize both the
    # expected local title and the live GitHub readback identically before
    # comparing so a faithful round-trip never reports a false readback failure
    # or false text_conflict. This collapses CR/LF and runs of whitespace to a
    # single space and trims the ends. It does NOT strip quotes/colons/ampersands
    # -- those are content and must round-trip intact.
    param([string]$Title)

    if ($null -eq $Title) {
        return ""
    }

    $normalized = $Title -replace "`r`n", "`n"
    $normalized = $normalized -replace "\s+", " "
    return $normalized.Trim()
}

function Set-GitHubIssueTitleAndBody {
    # Robust send-side write. Windows PowerShell 5.1 mangles native-command
    # arguments that contain double-quotes (it strips embedded `"` and turns a
    # trailing `\` into `"`), so `gh issue edit --title $issueTitle` silently
    # corrupted any title with a double-quote before it ever reached GitHub. We
    # instead PATCH the issue through `gh api` with a JSON body delivered on
    # stdin (`--input -`), exactly like the create path in
    # Bootstrap-TaskGitHubIssues.ps1. JSON on stdin is never subject to native
    # argument quoting, so the literal title round-trips intact.
    param(
        [Parameter(Mandatory = $true)]
        [string]$ProviderRepo,

        [Parameter(Mandatory = $true)]
        [int]$IssueNumber,

        [Parameter(Mandatory = $true)]
        [string]$Title,

        [Parameter(Mandatory = $true)]
        [string]$Body
    )

    $payload = [pscustomobject]@{
        title = $Title
        body = $Body
    } | ConvertTo-Json -Depth 6

    $payload | gh api -X PATCH "/repos/$ProviderRepo/issues/$IssueNumber" --input - | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to update title/body on $ProviderRepo#$IssueNumber via gh api PATCH."
    }
}

function Sync-GitHubIssueFields {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ProviderRepo,

        [Parameter(Mandatory = $true)]
        [int]$IssueNumber,

        [Parameter(Mandatory = $true)]
        [System.Collections.IDictionary]$DesiredValues
    )

    $repoParts = $ProviderRepo.Split("/")
    if ($repoParts.Count -ne 2) {
        throw "Provider repo '$ProviderRepo' must be formatted as owner/name."
    }

    $owner = $repoParts[0]
    $fieldsJson = gh api "/orgs/$owner/issue-fields"
    if ($LASTEXITCODE -ne 0) {
        throw "Could not read organization issue fields for '$owner'. Accepted task repos must be organization-owned when issue fields are required."
    }

    $fields = $fieldsJson | ConvertFrom-Json
    $fieldValues = @()
    $expectedByFieldId = @{}

    foreach ($fieldName in $DesiredValues.Keys) {
        $field = @($fields | Where-Object { $_.name -eq $fieldName }) | Select-Object -First 1
        if (-not $field) {
            throw "GitHub issue field '$fieldName' was not found in organization '$owner'."
        }

        if ($field.data_type -ne "single_select") {
            throw "GitHub issue field '$fieldName' must be single_select, but is '$($field.data_type)'."
        }

        $desiredValue = [string]$DesiredValues[$fieldName]
        $option = @($field.options | Where-Object { $_.name -eq $desiredValue }) | Select-Object -First 1
        if (-not $option) {
            throw "GitHub issue field '$fieldName' does not have option '$desiredValue'."
        }

        $fieldValues += [pscustomobject]@{
            field_id = [int]$field.id
            value = $desiredValue
        }
        $expectedByFieldId[[int]$field.id] = [pscustomobject]@{
            name = $fieldName
            value = $desiredValue
        }
    }

    $payload = [pscustomobject]@{
        issue_field_values = $fieldValues
    } | ConvertTo-Json -Depth 6

    $payload | gh api -X POST "/repos/$ProviderRepo/issues/$IssueNumber/issue-field-values" --input - | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to set issue field values on $ProviderRepo#$IssueNumber."
    }

    $readbackJson = gh api "/repos/$ProviderRepo/issues/$IssueNumber/issue-field-values"
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to read back issue field values on $ProviderRepo#$IssueNumber."
    }

    $readback = $readbackJson | ConvertFrom-Json
    foreach ($fieldId in $expectedByFieldId.Keys) {
        $actual = @($readback | Where-Object { [int]$_.issue_field_id -eq [int]$fieldId }) | Select-Object -First 1
        if (-not $actual) {
            throw "Readback is missing issue field '$($expectedByFieldId[$fieldId].name)'."
        }

        $actualValue = [string]$actual.single_select_option.name
        if ($actualValue -ne $expectedByFieldId[$fieldId].value) {
            throw "Readback issue field '$($expectedByFieldId[$fieldId].name)' is '$actualValue', expected '$($expectedByFieldId[$fieldId].value)'."
        }
    }

    return $readbackJson
}

$repoRoot = (Get-Location).Path
$manifestFullPath = (Resolve-Path -LiteralPath $ManifestPath).Path
$taskFullPath = (Resolve-Path -LiteralPath $TaskPath).Path
$taskItem = Get-Item -LiteralPath $taskFullPath

if ($taskItem.Name -ne "TASK.md") {
    throw "TaskPath must point to a TASK.md file."
}

if ($taskItem.Directory.Name -notmatch "^Task-(\d{4})$") {
    throw "TASK.md must live under a Tracking/Task-0000 style folder."
}

$taskDigits = $Matches[1]
$taskId = "Task-$taskDigits"
$issueNumber = [int]$taskDigits
$relativeTaskPath = (Get-RelativePathCompat -BasePath $repoRoot -TargetPath $taskFullPath) -replace "\\", "/"

$content = Get-Content -Raw -Encoding UTF8 -LiteralPath $taskFullPath
$header = [regex]::Match($content, "(?m)^#\s*Task(?:[-\s])(?<digits>\d{4})\s*$")
if (-not $header.Success) {
    throw "TASK.md must contain a top-level '# Task 0000' or '# Task-0000' heading."
}

$headerDigits = $header.Groups["digits"].Value
if ($headerDigits -ne $taskDigits) {
    throw "TASK.md heading Task-$headerDigits does not match folder $taskId."
}

$titleSection = Get-MarkdownSection -Content $content -Name "Title"
$taskTitle = Get-FirstNonEmptyLine -Text $titleSection
if (-not $taskTitle) {
    throw "TASK.md must contain a non-empty '## Title' section."
}

$manifest = Get-Content -Raw -Encoding UTF8 -LiteralPath $manifestFullPath | ConvertFrom-Json
$repoEntry = @($manifest.repos) | Where-Object { $_.id -eq $RepoId } | Select-Object -First 1
if (-not $repoEntry) {
    throw "Repo id '$RepoId' was not found in $ManifestPath."
}

if ($repoEntry.task_provider.kind -ne "github_issues") {
    throw "Repo '$RepoId' task_provider must be github_issues."
}

$providerRepo = [string]$repoEntry.task_provider.repo
$sourceCommit = (& git rev-parse HEAD 2>$null).Trim()
if ($LASTEXITCODE -ne 0 -or -not $sourceCommit) {
    $sourceCommit = "unknown"
}

$taskHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $taskFullPath).Hash
$renderedAt = (Get-Date -Format o)
$issueTitle = "${taskId}: $taskTitle"

$body = @"
<!-- task-sync: repo=$RepoId; task_id=$taskId; task_path=$relativeTaskPath -->

# ${taskId}: $taskTitle

## Source Of Truth

Local ``$relativeTaskPath`` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for ${taskId}:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with ``gh``.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through ``gh``, and
writes local task metadata only after successful readback.
"@

foreach ($sectionName in @("Summary", "Human Outcome", "Goals", "Acceptance Criteria", "Non-Goals")) {
    $sectionBody = Get-MarkdownSection -Content $content -Name $sectionName
    if ($sectionBody) {
        $body += "`n`n## $sectionName`n`n$sectionBody"
    }
}

$body += "`n"
$body += @"

## Sync Metadata

- GitHub repo: ``$providerRepo``
- Issue number: ``$issueNumber``
- Local task path: ``$relativeTaskPath``
- Source commit: ``$sourceCommit``
- Local task SHA-256: ``$taskHash``
- Rendered at: ``$renderedAt``
"@

Assert-NoTaskIdMismatch -Body $body -ExpectedTaskId $taskId -ExpectedTaskPath $relativeTaskPath

if (-not $OutputBodyPath) {
    $OutputBodyPath = Join-Path ([System.IO.Path]::GetTempPath()) "codex-task-gh-body-$taskId.md"
}

$outputBodyFullPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputBodyPath)
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $outputBodyFullPath) | Out-Null
Write-Utf8NoBom -Path $outputBodyFullPath -Value $body

if (-not $MetadataPath) {
    $MetadataPath = Join-Path $taskItem.Directory.FullName "TASK-META.json"
}

$metadataFullPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($MetadataPath)

if ($DryRun) {
    # Expose the exact send-side title payload (JSON, the way it is delivered to
    # `gh api --input -`) and the normalized-compare form so the title
    # encode/round-trip/compare logic can be verified deterministically without a
    # live GitHub write. `send_payload_title` is what the JSON parser on the
    # GitHub side reconstructs; for a faithful round-trip it equals $issueTitle.
    $sendPayloadJson = [pscustomobject]@{ title = $issueTitle } | ConvertTo-Json -Depth 4
    $sendPayloadTitle = ($sendPayloadJson | ConvertFrom-Json).title

    [pscustomobject]@{
        status = "dry_run"
        task_id = $taskId
        issue_number = $issueNumber
        provider_repo = $providerRepo
        title = $issueTitle
        title_normalized = (Normalize-TitleForCompare -Title $issueTitle)
        send_payload_title = $sendPayloadTitle
        send_payload_round_trips = ($sendPayloadTitle -eq $issueTitle)
        send_payload_compares_equal = ((Normalize-TitleForCompare -Title $sendPayloadTitle) -eq (Normalize-TitleForCompare -Title $issueTitle))
        issue_fields = [ordered]@{
            Queue = $QueueValue
            Priority = $PriorityValue
            "Human Needed" = $HumanNeededValue
        }
        body_path = $outputBodyFullPath
        metadata_path = $metadataFullPath
    } | ConvertTo-Json -Depth 4
    exit 0
}

gh auth status -h github.com | Out-Null

$existingJson = gh issue view $issueNumber --repo $providerRepo --json number,title,body,state,labels,url
if ($LASTEXITCODE -ne 0) {
    throw "Issue #$issueNumber does not exist in $providerRepo. Create accepted tasks through TaskCreate first so GitHub assigns the task number."
}

$existing = $existingJson | ConvertFrom-Json
if ([int]$existing.number -ne $issueNumber) {
    throw "Read issue number '$($existing.number)' but expected '$issueNumber'."
}

if (Test-Path -LiteralPath $metadataFullPath) {
    $metadata = Get-Content -Raw -Encoding UTF8 -LiteralPath $metadataFullPath | ConvertFrom-Json
    if ([string]$metadata.provider_repo -ne $providerRepo) {
        throw "TASK-META.json provider_repo '$($metadata.provider_repo)' does not match '$providerRepo'."
    }
    if ([int]$metadata.issue_number -ne $issueNumber) {
        throw "TASK-META.json issue_number '$($metadata.issue_number)' does not match '$issueNumber'."
    }
}

$existingMarkerPattern = "<!--\s*task-sync:\s*repo=[^;]+;\s*task_id=(?<task>Task-\d{4});\s*task_path=(?<path>[^ ]+)\s*-->"
$existingMarker = [regex]::Match([string]$existing.body, $existingMarkerPattern)
if ($existingMarker.Success) {
    $existingTask = $existingMarker.Groups["task"].Value
    $existingPath = $existingMarker.Groups["path"].Value
    if (($existingTask -ne $taskId -or $existingPath -ne $relativeTaskPath) -and -not $RepairMismatchedRemote) {
        throw "Issue #$issueNumber already has marker task_id=$existingTask task_path=$existingPath. Refusing to overwrite with $taskId $relativeTaskPath without -RepairMismatchedRemote."
    }
} elseif (-not $RepairMismatchedRemote -and -not $ForceRemoteOverwrite) {
    throw "Issue #$issueNumber has no task-sync marker. Refusing to overwrite live GitHub issue text without -RepairMismatchedRemote or -ForceRemoteOverwrite."
}

$bodyMatches = ((Normalize-BodyForCompare -Body ([string]$existing.body)) -eq (Normalize-BodyForCompare -Body $body))
$titleMatches = ((Normalize-TitleForCompare -Title ([string]$existing.title)) -eq (Normalize-TitleForCompare -Title $issueTitle))
if ((-not $bodyMatches -or -not $titleMatches) -and -not $ForceRemoteOverwrite) {
    throw "Live GitHub issue #$issueNumber title/body differs from the rendered local task. Run reconcile and merge/review the diff, or use -ForceRemoteOverwrite after deciding the local render should replace the remote issue text."
}

$labelsToRemove = Get-RejectedLabelNames -LabelNames (Get-LabelNames -Labels $existing.labels)

Set-GitHubIssueTitleAndBody -ProviderRepo $providerRepo -IssueNumber $issueNumber -Title $issueTitle -Body $body

foreach ($labelName in $labelsToRemove) {
    gh issue edit $issueNumber --repo $providerRepo --remove-label $labelName | Out-Null
}

$viewJson = gh issue view $issueNumber --repo $providerRepo --json number,title,body,state,labels,url
$view = $viewJson | ConvertFrom-Json

if ([int]$view.number -ne $issueNumber) {
    throw "Readback issue number '$($view.number)' does not match '$issueNumber'."
}

if ((Normalize-TitleForCompare -Title ([string]$view.title)) -ne (Normalize-TitleForCompare -Title $issueTitle)) {
    throw "Readback title '$($view.title)' does not match '$issueTitle'."
}

Assert-NoTaskIdMismatch -Body ([string]$view.body) -ExpectedTaskId $taskId -ExpectedTaskPath $relativeTaskPath

$labelNames = Get-LabelNames -Labels $view.labels
$rejectedLabels = Get-RejectedLabelNames -LabelNames $labelNames
if (@($rejectedLabels).Count -gt 0) {
    throw "Issue still has rejected workflow/identity labels: $(($rejectedLabels -join ', ')). Queue state is owned by provider issue fields, not issue labels."
}

if (-not $SkipIssueFieldSync) {
    $issueFieldValuesJson = Sync-GitHubIssueFields -ProviderRepo $providerRepo -IssueNumber $issueNumber -DesiredValues ([ordered]@{
        Queue = $QueueValue
        Priority = $PriorityValue
        "Human Needed" = $HumanNeededValue
    })
}

$viewJson = gh issue view $issueNumber --repo $providerRepo --json number,title,body,state,labels,url
$view = $viewJson | ConvertFrom-Json

if ([int]$view.number -ne $issueNumber) {
    throw "Final readback issue number '$($view.number)' does not match '$issueNumber'."
}

if ((Normalize-TitleForCompare -Title ([string]$view.title)) -ne (Normalize-TitleForCompare -Title $issueTitle)) {
    throw "Final readback title '$($view.title)' does not match '$issueTitle'."
}

Assert-NoTaskIdMismatch -Body ([string]$view.body) -ExpectedTaskId $taskId -ExpectedTaskPath $relativeTaskPath

if ($ViewOutputPath) {
    $viewOutputFullPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($ViewOutputPath)
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $viewOutputFullPath) | Out-Null
    Write-Utf8NoBom -Path $viewOutputFullPath -Value $viewJson
}

New-Item -ItemType Directory -Force -Path (Split-Path -Parent $metadataFullPath) | Out-Null

[pscustomobject]@{
    schema_version = 1
    provider_kind = "github_issues"
    provider_repo = $providerRepo
    issue_number = $issueNumber
    issue_url = $view.url
    last_synced_at = (Get-Date -Format o)
} | ConvertTo-Json -Depth 4 | ForEach-Object {
    Write-Utf8NoBom -Path $metadataFullPath -Value $_
}

[pscustomobject]@{
    status = "synced"
    task_id = $taskId
    issue_number = $issueNumber
    provider_repo = $providerRepo
    issue_url = $view.url
    title = $view.title
    labels = @($labelNames)
    issue_fields = if ($SkipIssueFieldSync) { $null } else { ($issueFieldValuesJson | ConvertFrom-Json) }
    body_path = $outputBodyFullPath
    view_path = $ViewOutputPath
    metadata_path = $metadataFullPath
} | ConvertTo-Json -Depth 8
