<#
.SYNOPSIS
    Dispatched subagent runner for the drain-queue demonstration.

.DESCRIPTION
    This script is the unit of work the consumer dispatches per queued task. It
    is deliberately isolated: it receives only one task spec and one allocated
    git worktree, applies the requested minor modification to the program in
    that worktree, runs the program's tests, commits the change on the worktree
    branch, and writes a structured result JSON.

    Honesty note: in this demonstration the "subagent" is a deterministic local
    executor that stands in for a dispatched Codex subagent, because nested
    Codex subagent-spawn tooling is not exposed to this run. The execution
    model it proves (isolated per-task work inside an allocated worktree,
    structured result capture) is the same shape a real Codex subagent dispatch
    would use.

.PARAMETER TaskSpecPath
    Path to a JSON file containing one queue item's spec.

.PARAMETER WorktreePath
    Path to the git worktree allocated for this task.

.PARAMETER ResultPath
    Path to write the structured result JSON.

.PARAMETER ApplierPath
    Path to apply_modification.py.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$TaskSpecPath,
    [Parameter(Mandatory = $true)][string]$WorktreePath,
    [Parameter(Mandatory = $true)][string]$ResultPath,
    [Parameter(Mandatory = $true)][string]$ApplierPath
)

$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()

$spec = Get-Content -LiteralPath $TaskSpecPath -Raw -Encoding UTF8 | ConvertFrom-Json
$taskId = $spec.task_id

$result = [ordered]@{
    task_id        = $taskId
    title          = $spec.title
    worktree_path  = $WorktreePath
    status         = "unknown"
    apply_summary  = $null
    tests_passed   = $false
    commit_sha     = $null
    diff_stat      = $null
    error          = $null
    started_at     = (Get-Date).ToString("o")
    finished_at    = $null
}

try {
    # 1. Apply the modification (the real subagent edit work).
    $applyOut = & python $ApplierPath $WorktreePath $TaskSpecPath 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "applier failed (exit $LASTEXITCODE): $applyOut"
    }
    $result.apply_summary = ($applyOut | Out-String).Trim()

    # 2. Run the program's tests inside the worktree.
    Push-Location $WorktreePath
    try {
        $testOut = & python -m unittest test_calc -v 2>&1
        $testExit = $LASTEXITCODE
    } finally {
        Pop-Location
    }
    $result.tests_passed = ($testExit -eq 0)
    if ($testExit -ne 0) {
        throw "tests failed (exit $testExit): $(( $testOut | Out-String).Trim())"
    }

    # 3. Commit the change on the worktree branch.
    Push-Location $WorktreePath
    try {
        & git add -A 2>&1 | Out-Null
        $commitMsg = "$taskId`: $($spec.title)"
        & git commit -q -m $commitMsg 2>&1 | Out-Null
        $result.commit_sha = (& git rev-parse HEAD).Trim()
        $result.diff_stat = ((& git show --stat --oneline HEAD) | Out-String).Trim()
    } finally {
        Pop-Location
    }

    $result.status = "succeeded"
}
catch {
    $result.status = "failed"
    $result.error = $_.Exception.Message
}
finally {
    $result.finished_at = (Get-Date).ToString("o")
    $json = $result | ConvertTo-Json -Depth 6
    Set-Content -LiteralPath $ResultPath -Value $json -Encoding UTF8
}

if ($result.status -eq "succeeded") { exit 0 } else { exit 1 }
