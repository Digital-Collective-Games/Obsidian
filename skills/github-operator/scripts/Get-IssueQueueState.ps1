# Read one issue's single-select field value (default "Queue") via the GitHub org
# issue-field-values API and print the field -> option name (or "unset").
#
# Reading state via the API is fine. The UI-only rule applies to the WRITE/flip
# being exercised at the real surface (see Set-IssueFieldViaUi.ps1 and TESTING.md);
# reading back the stored value through the API is the appropriate verification.
#
# The issue-field-values entry carries a numeric single-select option id; the
# human-readable name lives in single_select_option.name. This decode mirrors
# skills/obsidian-operator/scripts/Reconcile-TaskGitHubState.ps1.

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Repo,           # owner/name
    [Parameter(Mandatory = $true)][int]$IssueNumber,
    [string]$FieldName = 'Queue',
    [switch]$ValueOnly                                     # emit only the option name string
)

Set-StrictMode -Version 3.0
$ErrorActionPreference = 'Stop'

# gh emits UTF-8 stdout; force UTF-8 console encoding so option names decode
# correctly regardless of host code page (mirrors the obsidian-operator scripts).
try {
    [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
    [Console]::InputEncoding = [System.Text.UTF8Encoding]::new($false)
} catch {
}
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)
if (Get-Variable -Name PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue) {
    $PSNativeCommandUseErrorActionPreference = $false
}

function Invoke-GhJson {
    param([Parameter(Mandatory = $true)][string[]]$Arguments)

    $oldErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $output = & gh @Arguments 2>&1
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldErrorActionPreference
    }

    $text = ($output | Out-String)
    if ($exitCode -ne 0) {
        throw "gh $($Arguments -join ' ') failed. $text"
    }

    if (-not $text.Trim()) {
        return $null
    }

    return ($text | ConvertFrom-Json)
}

if ($Repo -notmatch '^[^/]+/[^/]+$') {
    throw "Repo '$Repo' must be formatted as owner/name."
}

$providerOwner = $Repo.Split('/')[0]

gh auth status -h github.com | Out-Null

# Map field name -> field id from the org issue-fields definition.
$issueFields = Invoke-GhJson -Arguments @('api', "/orgs/$providerOwner/issue-fields")
$fieldNameById = @{}
foreach ($field in @($issueFields)) {
    if ($field.PSObject.Properties.Name -contains 'id') {
        $fieldNameById[[int]$field.id] = [string]$field.name
    }
}

# Read the stored values for this issue and decode the requested field.
$values = Invoke-GhJson -Arguments @('api', "/repos/$Repo/issues/$IssueNumber/issue-field-values")

$optionName = 'unset'
foreach ($value in @($values)) {
    $fieldId = [int]$value.issue_field_id
    if (-not $fieldNameById.ContainsKey($fieldId)) {
        continue
    }
    if ($fieldNameById[$fieldId] -ne $FieldName) {
        continue
    }

    if ($value.PSObject.Properties.Name -contains 'single_select_option' -and $null -ne $value.single_select_option) {
        $optionName = [string]$value.single_select_option.name
    } elseif ($value.PSObject.Properties.Name -contains 'value' -and $null -ne $value.value) {
        $optionName = [string]$value.value
    }
    break
}

if ($ValueOnly) {
    if ($optionName -eq 'unset') {
        return $null
    }
    return $optionName
}

Write-Output "${FieldName} -> ${optionName}"
