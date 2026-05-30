param(
    # Backend base URL. Defaults to the local service-lane bind. Point this at the
    # isolated validation lane (e.g. http://127.0.0.1:14318) when proving on a
    # throwaway test repo rather than the live service.
    [string]$BaseUrl = "http://127.0.0.1:4318",
    [switch]$AsJson
)

Set-StrictMode -Version 3.0
$ErrorActionPreference = "Stop"

# O6 (Task-0015): enumerate the active owned-lane worktrees bound to the backend's
# GET /api/v1/worktrees endpoint and print, per worktree, the worktree path,
# issue/Task, run/gate state, and the session transcript path the operator would
# open in VSCodium to kick a parked or slow agent along.
#
# This command SURFACES the raw fields the endpoint supplies (worktree path,
# agent session id, transcript path); constructing the actual vscodium:// resume
# link is a consumer concern and is intentionally NOT done by the backend.

$uri = ($BaseUrl.TrimEnd("/")) + "/api/v1/worktrees"
try {
    $response = Invoke-RestMethod -Method Get -Uri $uri -TimeoutSec 15
} catch {
    Write-Error "Failed to query active worktrees at ${uri}: $($_.Exception.Message)"
    exit 1
}

$worktrees = @()
if ($null -ne $response -and $null -ne $response.worktrees) {
    $worktrees = @($response.worktrees)
}

if ($AsJson) {
    $worktrees | ConvertTo-Json -Depth 6
    return
}

if ($worktrees.Count -eq 0) {
    Write-Host "No active owned-lane worktrees." -ForegroundColor Yellow
    return
}

Write-Host ("Active owned-lane worktrees ({0}):" -f $worktrees.Count)
foreach ($wt in $worktrees) {
    Write-Host ""
    Write-Host ("  Task/Issue : {0}" -f $wt.task_id)
    Write-Host ("  Repo       : {0}" -f $wt.repo)
    Write-Host ("  Run/Gate   : {0}" -f $wt.run_gate_state)
    Write-Host ("  Worktree   : {0}" -f $wt.worktree_path)
    Write-Host ("  Session ID : {0}" -f $wt.agent_session_id)
    Write-Host ("  Transcript : {0}" -f $wt.session_transcript_path)
}
