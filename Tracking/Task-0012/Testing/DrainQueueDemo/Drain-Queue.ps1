<#
.SYNOPSIS
    Drain-queue consumer demonstration ("drain my queue pls").

.DESCRIPTION
    Pulls ready tasks from a local JSON queue and, for each one:
      1. allocates a dedicated git worktree on the target repo (one per task),
         recording the allocation in a registry that follows the
         WORKTREE-ALLOCATIONS.json schema from
         C:\Users\gregs\.codex\Orchestration\WORKTREES.md;
      2. dispatches a subagent (Invoke-Subagent.ps1) to perform the task's
         minor program modification inside that worktree;
      3. respects a configured concurrency limit on in-flight subagents /
         allocated worktrees;
      4. captures the per-task result (apply summary, tests-passed, commit SHA,
         diff stat);
      5. releases (removes) the worktree and marks the allocation released.

    Worktrees here are disposable demo worktrees (reusable=false), so they are
    named by slot index and removed on release. The registry, schema, and the
    allocate/bind/release lifecycle follow the shared WORKTREES.md conventions.

.PARAMETER QueuePath
    Path to queue.json.

.PARAMETER TargetRepo
    Path to the target git repo whose program the tasks modify.

.PARAMETER MaxConcurrency
    Maximum number of tasks (and therefore allocated worktrees / in-flight
    subagents) at once.

.PARAMETER OutDir
    Directory for results, logs, registry, and summary.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$QueuePath,
    [Parameter(Mandatory = $true)][string]$TargetRepo,
    [int]$MaxConcurrency = 2,
    [Parameter(Mandatory = $true)][string]$OutDir
)

$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()

$scriptDir   = Split-Path -Parent $MyInvocation.MyCommand.Path
$applierPath = Join-Path $scriptDir "apply_modification.py"
$subagent    = Join-Path $scriptDir "Invoke-Subagent.ps1"
$resultsDir  = Join-Path $OutDir "results"
$logsDir     = Join-Path $OutDir "logs"
$registryPath = Join-Path $OutDir "worktree-allocations-demo.json"
$logPath     = Join-Path $OutDir "DRAIN-RUN.log"

New-Item -ItemType Directory -Force -Path $resultsDir | Out-Null
New-Item -ItemType Directory -Force -Path $logsDir | Out-Null

$runLog = New-Object System.Collections.Generic.List[string]
function Log([string]$msg) {
    $line = "[{0}] {1}" -f (Get-Date).ToString("HH:mm:ss.fff"), $msg
    $runLog.Add($line)
    Write-Host $line
}

# --- Worktree allocation registry (WORKTREES.md schema) -------------------
$repoId = Split-Path -Leaf $TargetRepo
$registry = [ordered]@{
    schema_version = 1
    updated_at     = (Get-Date).ToString("o")
    note           = "Demo worktree allocation registry following WORKTREES.md schema. These are disposable per-task demo worktrees (reusable=false), kept separate from the shared cross-task WORKTREE-ALLOCATIONS.json so disposable demo state does not pollute coordination state."
    slots          = New-Object System.Collections.Generic.List[object]
}
function Save-Registry {
    $registry.updated_at = (Get-Date).ToString("o")
    $registry | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $registryPath -Encoding UTF8
}

function Add-Allocation($slotId, $path, $branch, $head, $taskId) {
    $rec = [ordered]@{
        slot_id          = $slotId
        repo_id          = $repoId
        base_repo_path   = $TargetRepo
        path             = $path
        intended_path    = $null
        state            = "bound"
        reusable         = $false
        owning_task_id   = $taskId
        branch           = $branch
        head             = $head
        dirty_state      = "clean at allocation"
        review_gate      = "none (disposable demo worktree)"
        release_condition = "remove worktree after subagent result is captured"
        allocated_at     = (Get-Date).ToString("o")
        released_at      = $null
        last_verified_at = (Get-Date).ToString("o")
        last_verified_by = "Drain-Queue.ps1"
        notes            = "Disposable demo worktree allocated by the drain-queue consumer."
    }
    $registry.slots.Add($rec)
    Save-Registry
    return $rec
}

function Set-Released($slotId, $finalHead, $dirtyState) {
    foreach ($s in $registry.slots) {
        if ($s.slot_id -eq $slotId) {
            $s.state = "released"
            $s.head = $finalHead
            $s.dirty_state = $dirtyState
            $s.released_at = (Get-Date).ToString("o")
            $s.last_verified_at = (Get-Date).ToString("o")
        }
    }
    Save-Registry
}

# --- Load queue -----------------------------------------------------------
$queue = Get-Content -LiteralPath $QueuePath -Raw -Encoding UTF8 | ConvertFrom-Json
$ready = @($queue.items | Where-Object { $_.state -eq "ready" } | Sort-Object priority)
Log "Loaded queue: $($queue.items.Count) item(s), $($ready.Count) ready."
Log "Target repo: $TargetRepo"
Log "Concurrency limit: $MaxConcurrency"

# Confirm target repo baseline.
Push-Location $TargetRepo
$baseHead = (& git rev-parse HEAD).Trim()
$baseBranch = (& git rev-parse --abbrev-ref HEAD).Trim()
Pop-Location
Log "Target baseline: branch=$baseBranch head=$baseHead"

# --- Drain loop with concurrency limit ------------------------------------
$inflight = @{}   # job.Id -> context hashtable
$slotIndex = 0
$completed = New-Object System.Collections.Generic.List[object]
$queueIndex = 0

function Start-OneTask($item) {
    $script:slotIndex++
    $slotId  = "{0}-drainslot{1}" -f $repoId.ToLower(), $script:slotIndex
    $wtPath  = "{0}_drainslot{1}" -f $TargetRepo, $script:slotIndex
    $branch  = "drain/$($item.task_id)"
    $specPath = Join-Path $logsDir "$($item.task_id)-spec.json"
    $resultPath = Join-Path $resultsDir "$($item.task_id)-result.json"
    $jobLog  = Join-Path $logsDir "$($item.task_id)-subagent.log"

    # Persist this item's isolated spec for the subagent.
    $item | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $specPath -Encoding UTF8

    # Allocate the worktree.
    Push-Location $TargetRepo
    & git worktree add -b $branch $wtPath $baseHead 2>&1 | Out-Null
    $wtAddExit = $LASTEXITCODE
    Pop-Location
    if ($wtAddExit -ne 0) { throw "git worktree add failed for $($item.task_id)" }
    Log "ALLOCATE  $($item.task_id) -> slot=$slotId path=$wtPath branch=$branch"
    Add-Allocation -slotId $slotId -path $wtPath -branch $branch -head $baseHead -taskId $item.task_id | Out-Null

    # Dispatch the subagent as a background job.
    $job = Start-Job -ScriptBlock {
        param($subagent, $specPath, $wtPath, $resultPath, $applierPath, $jobLog)
        & pwsh -NoProfile -File $subagent -TaskSpecPath $specPath -WorktreePath $wtPath -ResultPath $resultPath -ApplierPath $applierPath *> $jobLog
        exit $LASTEXITCODE
    } -ArgumentList $subagent, $specPath, $wtPath, $resultPath, $applierPath, $jobLog

    Log "DISPATCH  $($item.task_id) -> subagent job #$($job.Id)"
    $script:inflight[$job.Id] = @{
        job = $job; item = $item; slotId = $slotId; wtPath = $wtPath
        branch = $branch; resultPath = $resultPath
    }
}

function Complete-OneJob($jobId) {
    $ctx = $script:inflight[$jobId]
    $item = $ctx.item
    Receive-Job -Job $ctx.job -ErrorAction SilentlyContinue | Out-Null
    $jobState = $ctx.job.State
    Remove-Job -Job $ctx.job -Force -ErrorAction SilentlyContinue

    $result = $null
    if (Test-Path -LiteralPath $ctx.resultPath) {
        $result = Get-Content -LiteralPath $ctx.resultPath -Raw -Encoding UTF8 | ConvertFrom-Json
    }
    $status = if ($result) { $result.status } else { "no_result" }
    $commit = if ($result) { $result.commit_sha } else { $null }
    Log "RESULT    $($item.task_id) -> status=$status commit=$commit (job=$jobState)"

    # Capture the diff against baseline before releasing the worktree.
    $finalHead = $baseHead
    $dirtyState = "no change"
    if (Test-Path -LiteralPath $ctx.wtPath) {
        Push-Location $ctx.wtPath
        $finalHead = (& git rev-parse HEAD).Trim()
        $diff = (& git diff $baseHead HEAD) | Out-String
        Pop-Location
        if ($finalHead -ne $baseHead) {
            $diffPath = Join-Path $script:resultsDir "$($item.task_id).diff"
            Set-Content -LiteralPath $diffPath -Value $diff -Encoding UTF8
            $dirtyState = "committed on branch $($ctx.branch)"
        }
    }

    # Release the worktree.
    Push-Location $TargetRepo
    & git worktree remove $ctx.wtPath --force 2>&1 | Out-Null
    $rmExit = $LASTEXITCODE
    Pop-Location
    if ($rmExit -eq 0) {
        Log "RELEASE   $($item.task_id) -> removed worktree $($ctx.wtPath)"
    } else {
        Log "RELEASE   $($item.task_id) -> WARNING git worktree remove exit $rmExit"
    }
    Set-Released -slotId $ctx.slotId -finalHead $finalHead -dirtyState $dirtyState

    $script:completed.Add([ordered]@{
        task_id = $item.task_id; title = $item.title; status = $status
        commit_sha = $commit; branch = $ctx.branch; slot_id = $ctx.slotId
        tests_passed = if ($result) { $result.tests_passed } else { $false }
    })
    $script:inflight.Remove($jobId)
}

# Main scheduling loop honoring MaxConcurrency.
while ($queueIndex -lt $ready.Count -or $inflight.Count -gt 0) {
    while ($inflight.Count -lt $MaxConcurrency -and $queueIndex -lt $ready.Count) {
        Start-OneTask $ready[$queueIndex]
        $queueIndex++
    }
    if ($inflight.Count -gt 0) {
        $jobs = $inflight.Values | ForEach-Object { $_.job }
        $done = Wait-Job -Job $jobs -Any -Timeout 120
        if ($done) {
            foreach ($d in @($done)) { Complete-OneJob $d.Id }
        } else {
            Log "WARN no job completed within timeout window; re-polling"
        }
    }
}

# --- Summary --------------------------------------------------------------
$succeeded = @($completed | Where-Object { $_.status -eq "succeeded" }).Count
Log "DONE drained $($completed.Count) task(s): $succeeded succeeded."

$summary = [ordered]@{
    generated_at    = (Get-Date).ToString("o")
    target_repo     = $TargetRepo
    baseline_head   = $baseHead
    max_concurrency = $MaxConcurrency
    queue_path      = $QueuePath
    drained_count   = $completed.Count
    succeeded_count = $succeeded
    tasks           = $completed
}
$summary | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $OutDir "DRAIN-RUN-SUMMARY.json") -Encoding UTF8
Set-Content -LiteralPath $logPath -Value ($runLog -join "`n") -Encoding UTF8

Write-Host ""
Write-Host "Summary JSON: $(Join-Path $OutDir 'DRAIN-RUN-SUMMARY.json')"
Write-Host "Registry:     $registryPath"
Write-Host "Run log:      $logPath"
