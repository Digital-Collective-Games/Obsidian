# Flip a single-select GitHub org issue field (default "Queue") on one issue by
# driving the real GitHub web UI through the human's debug Chrome session (CDP).
#
# This is the WRITE/flip exercised at the real human surface -- the human only
# authenticates the debug Chrome profile; this script drives the UI end-to-end.
# It implements the proven flip sequence (poll for the editable control, click it,
# pick the option from the picker, Escape to commit, then re-read the control's
# text to verify) and returns a structured result.
#
# WRITE GUARDRAIL: only run this against throwaway/test issues (e.g. the
# QueueDrainTestbed repo). Never drive the human's production issues without
# explicit authorization.

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Repo,            # owner/name
    [Parameter(Mandatory = $true)][int]$IssueNumber,
    [string]$FieldName = 'Queue',
    [Parameter(Mandatory = $true)][string]$OptionName,      # e.g. Ready / Never
    [string]$Endpoint = 'http://127.0.0.1:9222',
    [string]$TargetId,
    [switch]$VerifyApi,
    [int]$ControlTimeoutSeconds = 30
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

. "$PSScriptRoot\CdpCommon.ps1"

if ($Repo -notmatch '^[^/]+/[^/]+$') {
    throw "Repo '$Repo' must be formatted as owner/name."
}

$issueUrl = "https://github.com/$Repo/issues/$IssueNumber"

$targets = Get-CdpTargets -Endpoint $Endpoint

# Prefer an already-open tab for the target repo so we never operate on the wrong
# GitHub tab; fall back to any open page tab we can navigate.
$target = $null
if ($TargetId) {
    $target = Select-CdpTarget -Targets $targets -TargetId $TargetId
} else {
    $target = Select-CdpTarget -Targets $targets -UrlContains $Repo
}

if (-not $target) {
    $anyPage = @($targets | Where-Object { $_.url -notlike 'devtools://*' })
    if ($anyPage.Count -eq 0) {
        throw "No usable Chrome page tab is open. Open a GitHub tab in the debug Chrome (port from $Endpoint) and retry."
    }
    $target = $anyPage[0]
}

$socket = $null
$nextId = 1
try {
    $socket = New-CdpSocket -WebSocketUrl $target.webSocketDebuggerUrl

    $null = Invoke-CdpCommand -Socket $socket -Id ($nextId++) -Method 'Page.enable' -Params @{}
    $null = Invoke-CdpCommand -Socket $socket -Id ($nextId++) -Method 'Runtime.enable' -Params @{}
    $null = Invoke-CdpCommand -Socket $socket -Id ($nextId++) -Method 'Page.bringToFront' -Params @{}
    $null = Invoke-CdpCommand -Socket $socket -Id ($nextId++) -Method 'Page.navigate' -Params @{ url = $issueUrl }

    # All JS sent to the page is built as PowerShell SINGLE-quoted literals so
    # PowerShell never interprets the JS as PowerShell. Dynamic values are spliced
    # in by PowerShell string concatenation OUTSIDE the JS quotes: the field name
    # is concatenated into the aria-label literal, and the option name is injected
    # as a safe JS string literal produced by ConvertTo-Json. Doubled single quotes
    # ('') inside the literals represent a single quote in the emitted JS.

    # Probe JS: returns the trimmed text of button[aria-label="Edit <FieldName>"],
    # or null when the control has not rendered yet.
    $probeJs = '(()=>{const b=document.querySelector(''button[aria-label="Edit ' + $FieldName + '"]''); return b ? (b.textContent||'''').trim() : null;})()'

    # Poll for the editable field control to render. The org issue field only
    # renders once the issue has a TYPE and an initial field VALUE; that setup is
    # done separately (not this script's job).
    $deadline = (Get-Date).AddSeconds($ControlTimeoutSeconds)
    $controlText = $null
    while ((Get-Date) -lt $deadline) {
        $controlText = Invoke-CdpEval -Socket $socket -Id ($nextId++) -Expression $probeJs
        if ($null -ne $controlText) {
            break
        }
        Start-Sleep -Milliseconds 2500
    }

    if ($null -eq $controlText) {
        throw "The 'Edit $FieldName' control never rendered on $issueUrl within $ControlTimeoutSeconds seconds. The org issue field only appears once the issue has an issue TYPE and an initial field VALUE -- set those first (see the obsidian-operator sync scripts), then retry."
    }

    # Click the editable control to open the picker.
    $editJs = '(()=>{const b=document.querySelector(''button[aria-label="Edit ' + $FieldName + '"]''); if(!b)return false; b.click(); return true;})()'
    $clickedEdit = Invoke-CdpEval -Socket $socket -Id ($nextId++) -Expression $editJs
    if (-not $clickedEdit) {
        throw "Failed to click the 'Edit $FieldName' control after it was found."
    }
    Start-Sleep -Seconds 2

    # Pick the matching option from the item picker. Options render as
    # li[role="option"] whose text starts with the option name (the trailing text
    # is the option description, e.g. "ReadyEligible for automatic dispatch.").
    # ConvertTo-Json turns the option name into a safe double-quoted JS string
    # literal, concatenated into the single-quoted JS body.
    $optionLiteral = ConvertTo-Json -InputObject $OptionName -Compress
    $optJs = '(()=>{const o=[...document.querySelectorAll(''li[role="option"]'')].find(e=>(e.textContent||'''').trim().startsWith(' + $optionLiteral + ')); if(!o)return false; o.click(); return true;})()'
    $clickedOption = Invoke-CdpEval -Socket $socket -Id ($nextId++) -Expression $optJs
    if (-not $clickedOption) {
        throw "Option '$OptionName' was not found in the '$FieldName' picker on $issueUrl. Confirm the option name matches an org single-select option."
    }
    Start-Sleep -Seconds 2

    # Escape to close/commit the picker.
    Send-CdpKey -Socket $socket -Id ($nextId++) -Key 'Escape' -Code 'Escape' -WindowsVirtualKeyCode 27
    $nextId += 2
    Start-Sleep -Seconds 3

    # Verify: the control text should now read "<FieldName><OptionName>".
    $observed = Invoke-CdpEval -Socket $socket -Id ($nextId++) -Expression $probeJs
    $expected = "$FieldName$OptionName"
    $committed = ($null -ne $observed -and $observed.Replace(' ', '') -eq $expected.Replace(' ', ''))

    $apiOptionName = $null
    $apiMatches = $null
    if ($VerifyApi) {
        $apiOptionName = & "$PSScriptRoot\Get-IssueQueueState.ps1" -Repo $Repo -IssueNumber $IssueNumber -FieldName $FieldName -ValueOnly
        $apiMatches = ($apiOptionName -eq $OptionName)
    }

    return [pscustomobject]@{
        Repo                = $Repo
        IssueNumber         = $IssueNumber
        IssueUrl            = $issueUrl
        FieldName           = $FieldName
        RequestedOption     = $OptionName
        Committed           = $committed
        ObservedButtonText  = $observed
        ExpectedButtonText  = $expected
        TargetId            = $target.id
        ApiVerified         = [bool]$VerifyApi
        ApiOptionName       = $apiOptionName
        ApiMatches          = $apiMatches
    }
} finally {
    Close-CdpSocket -Socket $socket
}
