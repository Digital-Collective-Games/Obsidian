package taskrun

import (
	"context"
	"errors"
	"time"
)

const ExecutionFailureModeTask0008WorkloadFailureOnce = "task_0008_workload_execution_failure_once"

var ErrRunNotFound = errors.New("task run not found")

const (
	StateReady             = "ready"
	StateQueued            = "queued"
	StateDispatching       = "dispatching"
	StateRunning           = "running"
	StateWaitingForHuman   = "waiting_for_human"
	StateBlocked           = "blocked"
	StateSleepingOrStalled = "sleeping_or_stalled"
	StateInterrupted       = "interrupted"
	StateCompleted         = "completed"
	StateCancelled         = "cancelled"
	StateFailed            = "failed"
)

const (
	AttentionNone           = "none"
	AttentionWatch          = "watch"
	AttentionNeedsAttention = "needs_attention"
	AttentionUrgent         = "urgent"
)

const (
	ActionDispatch  = "dispatch"
	ActionPoke      = "poke"
	ActionInterrupt = "interrupt"
)

type StateEnvelope struct {
	State              string                         `json:"state"`
	ReasonCode         string                         `json:"reason_code"`
	StateSummary       string                         `json:"state_summary"`
	EvidenceRefs       []EvidenceRef                  `json:"evidence_refs,omitempty"`
	NextOwner          string                         `json:"next_owner,omitempty"`
	NextExpectedEvent  string                         `json:"next_expected_event,omitempty"`
	SuspiciousAfter    time.Time                      `json:"suspicious_after,omitempty"`
	ActionBlockReasons map[string][]ActionBlockReason `json:"action_block_reasons,omitempty"`
}

type EvidenceRef struct {
	Type      string `json:"type"`
	Label     string `json:"label"`
	URI       string `json:"uri"`
	LineRange string `json:"line_range,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type ActionBlockReason struct {
	Code    string `json:"code"`
	Summary string `json:"summary"`
}

type HumanActionTarget struct {
	Kind      string `json:"kind"`
	Label     string `json:"label"`
	URI       string `json:"uri"`
	LineRange string `json:"line_range,omitempty"`
}

type LaunchTarget struct {
	Kind      string   `json:"kind"`
	Label     string   `json:"label"`
	URI       string   `json:"uri"`
	Command   []string `json:"command,omitempty"`
	Preferred bool     `json:"preferred,omitempty"`
}

type DeepContext struct {
	SessionID             string         `json:"session_id,omitempty"`
	TranscriptPath        string         `json:"transcript_path,omitempty"`
	PreferredLaunchTarget *LaunchTarget  `json:"preferred_launch_target,omitempty"`
	LaunchTargets         []LaunchTarget `json:"launch_targets,omitempty"`
}

type WaitContract struct {
	WaitingOn           string             `json:"waiting_on,omitempty"`
	WhyBlocked          string             `json:"why_blocked,omitempty"`
	ResumeWhen          string             `json:"resume_when,omitempty"`
	HumanActionRequired bool               `json:"human_action_required,omitempty"`
	HumanActionTarget   *HumanActionTarget `json:"human_action_target,omitempty"`
	NextOwner           string             `json:"next_owner,omitempty"`
	StaleAfter          time.Time          `json:"stale_after,omitempty"`
	EvidenceRefs        []EvidenceRef      `json:"evidence_refs,omitempty"`
}

type DispatchReadiness struct {
	Ready                bool                `json:"ready"`
	BlockReasons         []ActionBlockReason `json:"block_reasons,omitempty"`
	ExpectedFirstSignal  string              `json:"expected_first_signal,omitempty"`
	FirstSuspiciousAfter time.Time           `json:"first_suspicious_after,omitempty"`
}

type AttentionPriority struct {
	Level   string `json:"attention_level"`
	Reason  string `json:"attention_reason,omitempty"`
	SortKey string `json:"attention_sort_key,omitempty"`
}

type ActionAvailability struct {
	Allowed      bool                `json:"allowed"`
	BlockReasons []ActionBlockReason `json:"block_reasons,omitempty"`
}

type StoryOwnership struct {
	OwnerRunID string `json:"owner_run_id,omitempty"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
}

type RepoLane struct {
	OwnedRepoRoot         string       `json:"owned_repo_root,omitempty"`
	CheckoutMode          string       `json:"checkout_mode,omitempty"`
	BaselineCommit        string       `json:"baseline_commit,omitempty"`
	CurrentCommit         string       `json:"current_commit,omitempty"`
	ApprovedRestoreCommit string       `json:"approved_restore_commit,omitempty"`
	RunArtifactRoot       string       `json:"run_artifact_root,omitempty"`
	BootstrapArtifactPath string       `json:"bootstrap_artifact_path,omitempty"`
	PreflightArtifactPath string       `json:"preflight_artifact_path,omitempty"`
	WorkloadStepPath      string       `json:"workload_step_path,omitempty"`
	WorkloadResultPath    string       `json:"workload_result_path,omitempty"`
	WorkloadOutputPath    string       `json:"workload_output_path,omitempty"`
	WorkloadCodePath      string       `json:"workload_code_path,omitempty"`
	ResetStatus           string       `json:"reset_status,omitempty"`
	LastResetTargetCommit string       `json:"last_reset_target_commit,omitempty"`
	ResetFailureSummary   string       `json:"reset_failure_summary,omitempty"`
	LastResetAt           time.Time    `json:"last_reset_at,omitempty"`
	Binding               *RepoBinding `json:"binding,omitempty"`
}

// RunGateState values recorded on the owned-lane record. A dispatched lane is
// "running" until the done-contract (O4/PASS-0003) parks it. Every parked state
// corresponds to GitHub issue Human Needed=Yes and RETAINS the worktree+slot;
// only a CLOSED issue deallocates (D2 / HUMAN-DIRECTIVES O4). The parked enum
// distinguishes which gate the run is waiting on so the watchdog (O5) can stay
// suspended and the enumeration endpoint (O6) can report the gate.
const (
	// RunGateStateRunning is the default state of a freshly dispatched owned lane.
	RunGateStateRunning = "running"
	// RunGateStateParkedAwaitingClosure is set when the agent perceives the work
	// complete: it sets Human Needed=Yes and PARKS awaiting an explicit human
	// closure approval. The agent NEVER self-closes (A4.2/A4.7).
	RunGateStateParkedAwaitingClosure = "parked_awaiting_closure"
	// RunGateStateParkedResearch is the research-gate park.
	RunGateStateParkedResearch = "parked_research"
	// RunGateStateParkedPlan is the plan-gate park.
	RunGateStateParkedPlan = "parked_plan"
	// RunGateStateParkedRegression is the regression-gate park.
	RunGateStateParkedRegression = "parked_regression"
)

// IsParkedRunGateState reports whether a run/gate state is one of the parked
// (Human Needed=Yes) states. A parked run retains its worktree and slot and is
// never redispatched; the watchdog is suspended while parked.
func IsParkedRunGateState(state string) bool {
	switch state {
	case RunGateStateParkedAwaitingClosure,
		RunGateStateParkedResearch,
		RunGateStateParkedPlan,
		RunGateStateParkedRegression:
		return true
	default:
		return false
	}
}

// RepoBinding is the O6 worktree<->session binding recorded on the owned-lane
// record at dispatch. It supplies the raw fields an operator (or a downstream
// review surface) needs to CONSTRUCT a VSCodium link to the bound session — the
// worktree path, the agent session id, and the session transcript path — but it
// never itself emits a vscodium:// link (the orchestrator boundary, per O6).
type RepoBinding struct {
	// Repo is the declared-repo id/root the owned lane belongs to.
	Repo string `json:"repo"`
	// TaskID is the issue #N / Task-N identifier (issue #N maps 1:1 to Task-N).
	TaskID string `json:"task_id"`
	// WorktreePath is the owned worktree checkout path (mirrors OwnedRepoRoot).
	WorktreePath string `json:"worktree_path"`
	// AgentSessionID is the session id bound to this worktree.
	//
	// PASS-0002 PLACEHOLDER: this is currently the BACKEND dispatch process's
	// session id (captured from its env at dispatch), NOT a launched agent's own
	// session. Real launched-agent session capture is PASS-0005 (O5) work.
	AgentSessionID string `json:"agent_session_id,omitempty"`
	// SessionTranscriptPath is the path to the bound session transcript.
	//
	// PASS-0002 PLACEHOLDER: dispatch-context value (see AgentSessionID);
	// real launched-agent transcript path is PASS-0005 (O5) work.
	SessionTranscriptPath string `json:"session_transcript_path,omitempty"`
	// RunGateState is the run/gate state. Defaults to RunGateStateRunning at
	// dispatch; the parked-needs-human / which-gate enum is O4/PASS-0003 work.
	RunGateState string `json:"run_gate_state"`
	// LaunchedPID is the launched agent's OS process id, persisted so reclaim can
	// terminate the agent before removing the worktree (BUG-0002): on Windows a
	// live agent's open handle on the checkout makes git worktree remove --force
	// partially fail and leave a residual directory.
	LaunchedPID int `json:"launched_pid,omitempty"`
}

type RunFollowUp struct {
	Kind        string    `json:"kind"`
	Owner       string    `json:"owner"`
	Status      string    `json:"status"`
	Summary     string    `json:"summary"`
	RequestedAt time.Time `json:"requested_at,omitempty"`
	DueAt       time.Time `json:"due_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

type RunResolution struct {
	Kind       string    `json:"kind"`
	Decision   string    `json:"decision"`
	Summary    string    `json:"summary"`
	ResolvedBy string    `json:"resolved_by,omitempty"`
	ResolvedAt time.Time `json:"resolved_at,omitempty"`
}

type TaskDefinitionSnapshot struct {
	DeclaredWorktreeRoot string               `json:"declared_worktree_root"`
	DeclaredTaskRoot     string               `json:"declared_task_root"`
	DeclaredTaskRevision string               `json:"declared_task_revision"`
	DeclaredGitRevision  string               `json:"declared_git_revision,omitempty"`
	CapturedAt           time.Time            `json:"captured_at"`
	Files                []TaskArtifactDigest `json:"files,omitempty"`
}

type TaskArtifactDigest struct {
	RelativePath string `json:"relative_path"`
	SHA256       string `json:"sha256"`
}

type TaskRunView struct {
	RunID                       string                        `json:"run_id"`
	TaskID                      string                        `json:"task_id"`
	WorkflowID                  string                        `json:"workflow_id,omitempty"`
	TemporalExecutionRunID      string                        `json:"temporal_execution_run_id,omitempty"`
	Status                      string                        `json:"status"`
	StateEnvelope               StateEnvelope                 `json:"state_envelope"`
	MeaningSummary              string                        `json:"meaning_summary,omitempty"`
	WaitContract                *WaitContract                 `json:"wait_contract,omitempty"`
	Attention                   AttentionPriority             `json:"attention"`
	Actions                     map[string]ActionAvailability `json:"actions,omitempty"`
	FollowUp                    *RunFollowUp                  `json:"follow_up,omitempty"`
	Resolution                  *RunResolution                `json:"resolution,omitempty"`
	RepoLane                    RepoLane                      `json:"repo_lane"`
	LastProgressAt              time.Time                     `json:"last_progress_at,omitempty"`
	LastProgressSummary         string                        `json:"last_progress_summary,omitempty"`
	FailureSummary              string                        `json:"failure_summary,omitempty"`
	CapturedTaskSnapshot        TaskDefinitionSnapshot        `json:"captured_task_snapshot"`
	DocRuntimeDivergenceStatus  string                        `json:"doc_runtime_divergence_status,omitempty"`
	DocRuntimeDivergenceSummary string                        `json:"doc_runtime_divergence_summary,omitempty"`
	DeepContext                 *DeepContext                  `json:"deep_context,omitempty"`
}

type TaskView struct {
	TaskID               string                        `json:"task_id"`
	Title                string                        `json:"title"`
	MeaningSummary       string                        `json:"meaning_summary"`
	StateEnvelope        StateEnvelope                 `json:"state_envelope"`
	DispatchReadiness    DispatchReadiness             `json:"dispatch_readiness"`
	Attention            AttentionPriority             `json:"attention"`
	Actions              map[string]ActionAvailability `json:"actions"`
	DeclaredWorktreeRoot string                        `json:"declared_worktree_root"`
	DeclaredTaskRoot     string                        `json:"declared_task_root"`
	DeclaredTaskRevision string                        `json:"declared_task_revision"`
	DeclaredGitRevision  string                        `json:"declared_git_revision,omitempty"`
	CurrentStory         StoryOwnership                `json:"current_story"`
	LatestRun            *TaskRunView                  `json:"latest_run,omitempty"`
	CurrentGate          string                        `json:"current_gate,omitempty"`
	CurrentPass          string                        `json:"current_pass,omitempty"`
	Phase                string                        `json:"phase,omitempty"`
	PlanApproved         bool                          `json:"plan_approved"`
	Blockers             []string                      `json:"blockers,omitempty"`
	UpdatedAt            string                        `json:"updated_at,omitempty"`
	DeepContext          *DeepContext                  `json:"deep_context,omitempty"`
}

type Runtime interface {
	StartTaskRun(ctx context.Context, request StartTaskRunRequest) (TaskRunView, error)
	GetTaskRun(ctx context.Context, runID string) (TaskRunView, error)
	// GetActiveTaskRun returns the run for an already-constructed (repo-namespaced)
	// active run id. The Service owns namespace construction (s.runID) so dispatch
	// and lookup never diverge; the runtime is a dumb id-keyed query.
	GetActiveTaskRun(ctx context.Context, runID string) (TaskRunView, error)
	// SetRunGateState / BindLaunchedSession (Landing 2) move the run/gate label and the
	// worktree<->session binding into the per-run workflow's durable state (signal +
	// read-back), keyed by an already-namespaced runID. They never change run lifecycle
	// or closure. ErrRunNotFound when the run is gone.
	SetRunGateState(ctx context.Context, runID string, state string) (TaskRunView, error)
	BindLaunchedSession(ctx context.Context, runID string, sessionID string, transcriptPath string, pid int) (TaskRunView, error)
	ReconcileTaskSnapshot(ctx context.Context, runID string, snapshot TaskDefinitionSnapshot) (TaskRunView, error)
	UpdateTaskRun(ctx context.Context, runID string, update TaskRunUpdate) (TaskRunView, error)
	RetryTaskRunWorkload(ctx context.Context, runID string, request WorkloadRetryRequest) (TaskRunView, error)
	// TerminateTaskRun terminates the per-run TaskRunWorkflow so an operator Eject does not
	// leave the run's long-running workflow ACTIVE/orphaned after the worktree is freed
	// (BUG-0005). The TaskRunWorkflow loops on signals and only exits on a terminal status,
	// so nothing ends it when a slot is reclaimed; Eject must terminate it explicitly. It is
	// idempotent: a run that is already gone (ErrRunNotFound) is treated as success.
	TerminateTaskRun(ctx context.Context, runID string, reason string) error
}

type StartTaskRunRequest struct {
	RunID                string                 `json:"run_id"`
	TaskID               string                 `json:"task_id"`
	Title                string                 `json:"title"`
	MeaningSummary       string                 `json:"meaning_summary"`
	CapturedTaskSnapshot TaskDefinitionSnapshot `json:"captured_task_snapshot"`
	ExecutionDirective   *ExecutionDirective    `json:"execution_directive,omitempty"`
	ContextSnapshot      *DeepContext           `json:"context_snapshot,omitempty"`
	RepoLane             RepoLane               `json:"repo_lane"`
	DispatchRequestedAt  time.Time              `json:"dispatch_requested_at"`
}

type ExecutionDirective struct {
	FailureMode string `json:"failure_mode,omitempty"`
}

type InterruptReviewResolution struct {
	Decision   string `json:"decision"`
	Summary    string `json:"summary,omitempty"`
	ResolvedBy string `json:"resolved_by,omitempty"`
}

type WorkloadRetryRequest struct {
	CapturedTaskSnapshot TaskDefinitionSnapshot `json:"captured_task_snapshot"`
	RepoLane             RepoLane               `json:"repo_lane"`
	RetryRequestedAt     time.Time              `json:"retry_requested_at"`
}

// SetGateStateRequest is the payload of the taskrun.set_gate_state signal (Landing 2):
// it moves the run/gate label (running/parked_*) into the per-run workflow's durable
// state as the SOLE live writer, instead of the owned-lane-bootstrap.json side-store.
// It carries ONLY the gate label and never changes StateEnvelope/Status (gate state is
// orthogonal to run lifecycle, exactly as the legacy Service.SetRunGateState was).
type SetGateStateRequest struct {
	State string `json:"state"`
}

// BindSessionRequest is the payload of the taskrun.bind_session signal (Landing 2): it
// records the launched agent's discovered session/transcript/PID onto the per-run
// workflow's durable binding (the live authority), replacing the dispatch-time
// placeholders. It never changes run lifecycle/closure.
type BindSessionRequest struct {
	AgentSessionID        string `json:"agent_session_id"`
	SessionTranscriptPath string `json:"session_transcript_path"`
	LaunchedPID           int    `json:"launched_pid"`
}

type TaskRunUpdate struct {
	State               string                        `json:"state"`
	ReasonCode          string                        `json:"reason_code"`
	StateSummary        string                        `json:"state_summary"`
	NextOwner           string                        `json:"next_owner,omitempty"`
	NextExpectedEvent   string                        `json:"next_expected_event,omitempty"`
	SuspiciousAfter     time.Time                     `json:"suspicious_after,omitempty"`
	LastProgressSummary string                        `json:"last_progress_summary,omitempty"`
	WaitContract        *WaitContract                 `json:"wait_contract,omitempty"`
	Attention           *AttentionPriority            `json:"attention,omitempty"`
	RepoLane            *RepoLane                     `json:"repo_lane,omitempty"`
	Actions             map[string]ActionAvailability `json:"actions,omitempty"`
	FollowUp            *RunFollowUp                  `json:"follow_up,omitempty"`
	Resolution          *RunResolution                `json:"resolution,omitempty"`
	CompletedAt         time.Time                     `json:"completed_at,omitempty"`
	FailureSummary      string                        `json:"failure_summary,omitempty"`
}
