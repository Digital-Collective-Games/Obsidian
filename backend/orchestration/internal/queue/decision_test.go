package queue

import "testing"

// O4 done-contract: the decision function MUST encode closed-only-deallocates and
// needs-human-never-frees/redispatches.
func TestDecideQueueActionEncodesDoneContract(t *testing.T) {
	cases := []struct {
		name      string
		issue     IssueState
		wantAct   QueueAction
		wantState string
	}{
		{
			name:    "closed is terminal even when queue still reads ready",
			issue:   IssueState{Closed: true, Queue: QueueReady},
			wantAct: ActionTerminal,
		},
		{
			name:    "closed is terminal even when human needed",
			issue:   IssueState{Closed: true, HumanNeeded: true},
			wantAct: ActionTerminal,
		},
		{
			name:      "human needed parks and never redispatches even if ready",
			issue:     IssueState{HumanNeeded: true, Queue: QueueReady},
			wantAct:   ActionPark,
			wantState: runGateStateParkedAwaitingClosure,
		},
		{
			name:      "human needed with research hint parks at research gate",
			issue:     IssueState{HumanNeeded: true, GateHint: GateHintResearch},
			wantAct:   ActionPark,
			wantState: runGateStateParkedResearch,
		},
		{
			name:      "human needed with plan hint parks at plan gate",
			issue:     IssueState{HumanNeeded: true, GateHint: GateHintPlan},
			wantAct:   ActionPark,
			wantState: runGateStateParkedPlan,
		},
		{
			name:      "human needed with regression hint parks at regression gate",
			issue:     IssueState{HumanNeeded: true, GateHint: GateHintRegression},
			wantAct:   ActionPark,
			wantState: runGateStateParkedRegression,
		},
		{
			name:      "human needed with unknown hint defaults to awaiting closure",
			issue:     IssueState{HumanNeeded: true, GateHint: GateHint("bogus")},
			wantAct:   ActionPark,
			wantState: runGateStateParkedAwaitingClosure,
		},
		{
			name:    "open ready not parked is eligible to dispatch",
			issue:   IssueState{Queue: QueueReady},
			wantAct: ActionDispatch,
		},
		{
			name:    "open never is no action",
			issue:   IssueState{Queue: QueueNever},
			wantAct: ActionNone,
		},
		{
			name:    "open unset queue is no action",
			issue:   IssueState{},
			wantAct: ActionNone,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecideQueueAction(tc.issue)
			if got.Action != tc.wantAct {
				t.Fatalf("action = %q, want %q (decision = %#v)", got.Action, tc.wantAct, got)
			}
			if got.ParkState != tc.wantState {
				t.Fatalf("park state = %q, want %q", got.ParkState, tc.wantState)
			}
			// Only a park carries a run/gate state; only terminal deallocates.
			if got.Action != ActionPark && got.ParkState != "" {
				t.Fatalf("non-park decision should carry no park state, got %q", got.ParkState)
			}
		})
	}
}

// The park action must never be produced for a non-Human-Needed open issue, and
// the terminal (deallocating) action must require a closed issue. This is the
// closed-only-deallocates / needs-human-never-frees invariant restated as a guard.
func TestDecideQueueActionInvariants(t *testing.T) {
	// An open issue (not human-needed) never yields a terminal/deallocating action.
	for _, q := range []QueueFieldValue{QueueReady, QueueNever, ""} {
		d := DecideQueueAction(IssueState{Queue: q})
		if d.Action == ActionTerminal {
			t.Fatalf("open issue (queue %q) must not be terminal", q)
		}
		if d.Action == ActionPark {
			t.Fatalf("open issue without human needed must not park, got %#v", d)
		}
	}
	// A human-needed open issue never deallocates and never dispatches.
	d := DecideQueueAction(IssueState{HumanNeeded: true, Queue: QueueReady})
	if d.Action == ActionTerminal || d.Action == ActionDispatch {
		t.Fatalf("human-needed issue must park, not free/redispatch, got %#v", d)
	}
}

func TestEffectiveFreeConcurrencyIsLimitMinusParked(t *testing.T) {
	cases := []struct {
		limit  int
		parked int
		want   int
	}{
		{limit: 4, parked: 0, want: 4},
		{limit: 4, parked: 1, want: 3}, // A4.5: 4-slot repo with 1 parked admits 3
		{limit: 4, parked: 4, want: 0},
		{limit: 4, parked: 6, want: 0},  // never negative
		{limit: 0, parked: 1, want: 3},  // non-positive limit -> default 4
		{limit: -2, parked: 0, want: 4}, // non-positive limit -> default 4
		{limit: 4, parked: -3, want: 4}, // negative parked normalized to zero
	}
	for _, tc := range cases {
		if got := EffectiveFreeConcurrency(tc.limit, tc.parked); got != tc.want {
			t.Fatalf("EffectiveFreeConcurrency(%d,%d) = %d, want %d", tc.limit, tc.parked, got, tc.want)
		}
	}
}
