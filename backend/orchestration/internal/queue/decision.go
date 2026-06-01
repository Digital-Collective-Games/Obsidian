package queue

// O4 done-contract decision logic. This is the PURE core the O3 queue-drain
// consumer (PASS-0004) calls to turn an observed GitHub issue state into a queue
// action. Keeping it pure (no I/O, no GitHub, no worktree side effects) lets the
// done-contract be proven deterministically here; the consumer wires the actions
// to taskrun.Service (cleanup/free/dequeue, SetRunGateState, dispatch) in O3.
//
// The done-contract invariants this function MUST encode (HUMAN-DIRECTIVES O4):
//   - ONLY a CLOSED issue deallocates (reclaim worktree + free slot + dequeue).
//   - Human Needed=Yes PARKS in place: retain worktree + slot, never redispatch,
//     never cleanup. The agent NEVER self-closes; closure is a distinct human gate.
//   - open + Queue=Ready + not parked => eligible to dispatch.
//   - anything else => no action.

// QueueFieldValue is the GitHub org "Queue" issue-field value.
type QueueFieldValue string

const (
	// QueueReady marks an issue eligible to dispatch.
	QueueReady QueueFieldValue = "Ready"
	// QueueNever (or any non-Ready value) is not eligible.
	QueueNever QueueFieldValue = "Never"
)

// GateHint optionally tells the decision which human gate a parked
// (Human Needed=Yes) run is waiting on, so the recorded run/gate state names the
// gate. It is a HINT: it never changes whether the run parks (Human Needed=Yes is
// the sole park trigger), only which parked state is recorded. An empty/unknown
// hint defaults to the awaiting-closure park.
type GateHint string

const (
	// GateHintNone leaves the parked state as the default awaiting-closure park.
	GateHintNone GateHint = ""
	// GateHintAwaitingClosure is the perceived-completion park.
	GateHintAwaitingClosure GateHint = "awaiting_closure"
	// GateHintResearch / GateHintPlan / GateHintRegression name the three
	// TaskDispatch human gates that must not bump the task.
	GateHintResearch   GateHint = "research"
	GateHintPlan       GateHint = "plan"
	GateHintRegression GateHint = "regression"
)

// IssueState is the observed GitHub issue state the consumer reads (the GitHub
// issue is the queryable source of truth, D2). Closed is terminal; HumanNeeded
// parks; Queue gates eligibility for an open, non-parked issue.
type IssueState struct {
	// Closed is true when the GitHub issue is closed (the ONLY terminal state).
	Closed bool
	// HumanNeeded is true when the issue's Human Needed field is Yes.
	HumanNeeded bool
	// Queue is the issue's Queue field value.
	Queue QueueFieldValue
	// GateHint optionally names which gate a Human Needed=Yes run is parked on.
	GateHint GateHint
}

// QueueAction is the action the consumer should take for an issue.
type QueueAction string

const (
	// ActionTerminal: the issue is closed => reclaim the worktree (cleanupOwnedLane),
	// free the slot, and dequeue the next Ready issue. The ONLY deallocating action.
	ActionTerminal QueueAction = "terminal"
	// ActionPark: Human Needed=Yes => retain worktree + slot, record the parked
	// run/gate state, do NOT redispatch, do NOT cleanup.
	ActionPark QueueAction = "park"
	// ActionDispatch: open + Queue=Ready + not parked => eligible to dispatch into
	// a free slot (subject to slot availability, which the consumer checks).
	ActionDispatch QueueAction = "dispatch"
	// ActionNone: nothing to do (e.g. open + not Ready + not parked).
	ActionNone QueueAction = "none"
)

// QueueDecision is the outcome of DecideQueueAction. When Action is ParkState
// names the run/gate state the consumer records via taskrun SetRunGateState; it is
// empty for non-park actions.
type QueueDecision struct {
	Action    QueueAction
	ParkState string
	Reason    string
}

// Park run/gate state values mirrored from the taskrun package. They are duplicated
// here as plain strings so internal/queue stays a leaf package (it must not import
// taskrun, which already imports queue); a unit test asserts they match the taskrun
// constants so the two never drift.
const (
	runGateStateRunning               = "running"
	runGateStateParkedAwaitingClosure = "parked_awaiting_closure"
	runGateStateParkedResearch        = "parked_research"
	runGateStateParkedPlan            = "parked_plan"
	runGateStateParkedRegression      = "parked_regression"
)

// DecideQueueAction maps an observed issue state to the queue action, encoding the
// O4 done-contract: closed-only-deallocates and needs-human-never-frees/redispatches.
// Precedence is deliberate: CLOSED is checked first (terminal wins even if the
// Queue field still reads Ready), then Human Needed (park wins over Ready so a
// parked-but-still-Ready issue is never redispatched), then Ready (dispatch).
func DecideQueueAction(issue IssueState) QueueDecision {
	if issue.Closed {
		return QueueDecision{
			Action: ActionTerminal,
			Reason: "issue closed: reclaim worktree, free slot, dequeue next",
		}
	}
	if issue.HumanNeeded {
		return QueueDecision{
			Action:    ActionPark,
			ParkState: parkStateForHint(issue.GateHint),
			Reason:    "human needed: park in place, retain worktree+slot, do not redispatch",
		}
	}
	if issue.Queue == QueueReady {
		return QueueDecision{
			Action: ActionDispatch,
			Reason: "open + queue ready + not parked: eligible to dispatch",
		}
	}
	return QueueDecision{Action: ActionNone, Reason: "open, not ready, not parked: no action"}
}

// parkStateForHint resolves which parked run/gate state to record for a
// Human Needed=Yes run. An unknown/empty hint defaults to awaiting-closure.
func parkStateForHint(hint GateHint) string {
	switch hint {
	case GateHintResearch:
		return runGateStateParkedResearch
	case GateHintPlan:
		return runGateStateParkedPlan
	case GateHintRegression:
		return runGateStateParkedRegression
	default:
		return runGateStateParkedAwaitingClosure
	}
}
