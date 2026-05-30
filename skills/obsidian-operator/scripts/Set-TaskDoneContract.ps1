param(
    [Parameter(Mandatory = $true)]
    [string]$TaskPath,

    # The done outcome the dispatched agent reached:
    #   abandon    - the agent decided mid-work the task should be abandoned / is a
    #                bad idea, OR it hit a research/plan/regression human gate.
    #   completion - the agent believes the work is complete (perceived completion).
    # BOTH set Human Needed=Yes and PARK in place. Neither closes the issue.
    [Parameter(Mandatory = $true)]
    [ValidateSet("abandon", "completion")]
    [string]$Outcome,

    # When -Outcome abandon, which human gate the agent parked on (informational;
    # records the run/gate state the consumer would set). awaiting_closure is the
    # implied gate for -Outcome completion.
    [ValidateSet("awaiting_closure", "research", "plan", "regression")]
    [string]$Gate,

    [string]$RepoId = "CodexDashboard",
    [string]$ManifestPath = "REPO-MANIFEST.json",
    [string]$SyncScriptPath = "skills/obsidian-operator/scripts/Sync-TaskToGitHubIssue.ps1",
    [switch]$DryRun
)

# Queue Done-Contract executor (O4). The dispatched agent's "done" path — on BOTH
# abandon and perceived completion — sets the GitHub issue's Human Needed=Yes and
# PARKS in place. The agent NEVER self-closes: this script has NO `gh issue close`
# path at all. Closing the issue is a distinct, final human action performed via
# Reconcile-TaskGitHubState.ps1 (the human-gated close builder), never here.
#
# It writes through the EXISTING field-value write in Sync-TaskToGitHubIssue.ps1
# (-HumanNeededValue Yes) so there is NO second GitHub-write path (A4.6). It only
# decides the run/gate state to record and asserts the never-self-close invariant.

Set-StrictMode -Version 3.0
$ErrorActionPreference = "Stop"

# Resolve the run/gate state the consumer would record for this park. Perceived
# completion is always the awaiting-closure park; an abandon defaults to the
# awaiting-closure park unless a specific human gate is named.
if ($Outcome -eq "completion") {
    if ($Gate -and $Gate -ne "awaiting_closure") {
        throw "Perceived completion parks awaiting closure approval; -Gate '$Gate' is only valid with -Outcome abandon."
    }
    $gateHint = "awaiting_closure"
} else {
    $gateHint = if ($Gate) { $Gate } else { "awaiting_closure" }
}

$runGateState = switch ($gateHint) {
    "awaiting_closure" { "parked_awaiting_closure" }
    "research"         { "parked_research" }
    "plan"             { "parked_plan" }
    "regression"       { "parked_regression" }
}

# The ONLY GitHub write: flip Human Needed=Yes through the existing sync field
# write. No close command is ever constructed here.
$syncArgs = @(
    "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $SyncScriptPath,
    "-TaskPath", $TaskPath,
    "-RepoId", $RepoId,
    "-ManifestPath", $ManifestPath,
    "-HumanNeededValue", "Yes"
)
if ($DryRun) {
    $syncArgs += "-DryRun"
}

$result = [ordered]@{
    contract        = "queue_done_contract"
    outcome         = $Outcome
    gate_hint       = $gateHint
    human_needed    = "Yes"
    run_gate_state  = $runGateState
    closes_issue    = $false
    self_closes     = $false
    write_path      = "Sync-TaskToGitHubIssue.ps1 issue-field-values (single GitHub-write path)"
    closure_note    = "Closure is a distinct, final human gate. A pass/regression/plan/research approval is NOT closure approval. The human closes via Reconcile-TaskGitHubState.ps1, never this script."
    dry_run         = [bool]$DryRun
    sync_command    = (@("powershell") + $syncArgs) -join " "
}

if ($DryRun) {
    $result["status"] = "dry_run"
    [pscustomobject]$result | ConvertTo-Json -Depth 5
    exit 0
}

# Real write delegates entirely to the existing field-value write path.
& powershell @syncArgs | Out-Null
$result["status"] = "written"
[pscustomobject]$result | ConvertTo-Json -Depth 5
