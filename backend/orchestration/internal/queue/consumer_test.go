package queue

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// fakeProvider is a deterministic in-memory QueueProvider for the TIER-1 unit
// proof. No live GitHub access: the test supplies the exact issue set.
type fakeProvider struct {
	issues []IssueRef
	calls  int
}

func (p *fakeProvider) ListReadyIssues(string) ([]IssueRef, error) {
	p.calls++
	return append([]IssueRef(nil), p.issues...), nil
}

// fakeDispatcher records every action the consumer takes so the tests can assert
// EXACTLY which tasks were dispatched/parked/reclaimed — and, critically, that the
// consumer dispatched through the dispatch seam with NO manual dispatch call
// (A3.1). active models which tasks already own an owned-lane worktree (used slots).
type fakeDispatcher struct {
	active        []string
	dispatched    []string
	parked        map[string]string
	reclaimed     []string
	dispatchErr   error
	failOnSetGate bool
}

func newFakeDispatcher(active ...string) *fakeDispatcher {
	return &fakeDispatcher{active: append([]string(nil), active...), parked: map[string]string{}}
}

func (d *fakeDispatcher) Dispatch(_ context.Context, taskID string) error {
	if d.dispatchErr != nil {
		return d.dispatchErr
	}
	d.dispatched = append(d.dispatched, taskID)
	d.active = append(d.active, taskID)
	return nil
}

func (d *fakeDispatcher) SetRunGateState(taskID string, state string) error {
	if d.failOnSetGate {
		return errors.New("set gate state failed")
	}
	d.parked[taskID] = state
	return nil
}

func (d *fakeDispatcher) Reclaim(_ context.Context, taskID string) error {
	d.reclaimed = append(d.reclaimed, taskID)
	for i, t := range d.active {
		if t == taskID {
			d.active = append(d.active[:i], d.active[i+1:]...)
			break
		}
	}
	return nil
}

func (d *fakeDispatcher) ActiveOwnedLaneTasks() ([]string, error) {
	return append([]string(nil), d.active...), nil
}

// fixedSizer pins the per-repo queue_workers cap for deterministic slot tests.
type fixedSizer int

func (s fixedSizer) RepoSlotLimit() int { return int(s) }

const testRepo = "Digital-Collective-Games/QueueDrainTestbed"

// singleEntryRegistryConsumer wraps one fake provider/dispatcher/sizer in a
// registry-driven consumer over a single-entry fake registry, so tests that
// exercise the WORKFLOW poll path (which now drives a RegistryConsumer) can reuse
// the existing per-repo fakes. The fake registry yields exactly one repo whose
// task_provider.repo is providerRepo and whose local_root is a synthetic path.
func singleEntryRegistryConsumer(providerRepo string, provider QueueProvider, dispatcher Dispatcher, sizer SlotSizer) *RegistryConsumer {
	loadRegistry := func() (RepoManifest, error) {
		return RepoManifest{Repos: []RepoEntry{{
			ID:           "TestRepo",
			LocalRoot:    "C:\\Agent\\TestRepo",
			QueueWorkers: sizer.RepoSlotLimit(),
			TaskProvider: &TaskProvider{Kind: "github_issues", Repo: providerRepo},
		}}}, nil
	}
	dispatchFor := func(RegistryRepo) (RepoDispatch, error) {
		return RepoDispatch{Provider: provider, Dispatcher: dispatcher, Sizer: sizer}, nil
	}
	return NewRegistryConsumer(loadRegistry, dispatchFor)
}

// A3.1: a Queue==Ready issue causes a dispatch through the taskrun dispatch path
// with NO manual dispatch call — the consumer invokes Dispatch itself. We assert
// the dispatch seam was invoked exactly for the Ready issue's Task-N.
func TestDrainOnceReadyIssueDispatchesWithoutManualCall(t *testing.T) {
	provider := &fakeProvider{issues: []IssueRef{
		{Number: 7001, State: IssueState{Queue: QueueReady}},
	}}
	dispatcher := newFakeDispatcher()
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedSizer(4))

	result, err := consumer.DrainOnce(context.Background())
	if err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}
	if want := []string{"Task-7001"}; !reflect.DeepEqual(dispatcher.dispatched, want) {
		t.Fatalf("dispatched = %v, want %v (Ready must dispatch via the dispatch path, no manual call)", dispatcher.dispatched, want)
	}
	if !reflect.DeepEqual(result.Dispatched, []string{"Task-7001"}) {
		t.Fatalf("result.Dispatched = %v, want [Task-7001]", result.Dispatched)
	}
	if provider.calls != 1 {
		t.Fatalf("provider polled %d times, want 1", provider.calls)
	}
}

// A3.2: an issue with Queue=Never (or unset) is NOT dispatched by the consumer.
func TestDrainOnceNeverOrUnsetIsNotDispatched(t *testing.T) {
	provider := &fakeProvider{issues: []IssueRef{
		{Number: 7002, State: IssueState{Queue: QueueNever}},
		{Number: 7003, State: IssueState{}}, // unset Queue
	}}
	dispatcher := newFakeDispatcher()
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedSizer(4))

	result, err := consumer.DrainOnce(context.Background())
	if err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}
	if len(dispatcher.dispatched) != 0 {
		t.Fatalf("dispatched = %v, want none (Never/unset must not dispatch)", dispatcher.dispatched)
	}
	if len(result.Skipped) != 2 {
		t.Fatalf("skipped = %v, want both Never and unset skipped", result.Skipped)
	}
}

// A3.3: the consumer maps issue #N to Tracking/Task-N exactly (no mapping layer).
func TestTaskIDForIssueMapsExactly(t *testing.T) {
	cases := map[int]string{
		12:    "Task-0012",
		7:     "Task-0007",
		7001:  "Task-7001",
		15:    "Task-0015",
		10000: "Task-10000",
	}
	for number, want := range cases {
		if got := TaskIDForIssue(number); got != want {
			t.Fatalf("TaskIDForIssue(%d) = %q, want %q", number, got, want)
		}
	}
}

// A3.3 (end-to-end through the loop): the dispatched task id equals the exact
// Task-N for the Ready issue number.
func TestDrainOnceDispatchesExactTaskIDForIssueNumber(t *testing.T) {
	provider := &fakeProvider{issues: []IssueRef{
		{Number: 12, State: IssueState{Queue: QueueReady}},
	}}
	dispatcher := newFakeDispatcher()
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedSizer(4))
	if _, err := consumer.DrainOnce(context.Background()); err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}
	if want := []string{"Task-0012"}; !reflect.DeepEqual(dispatcher.dispatched, want) {
		t.Fatalf("dispatched = %v, want %v (#12 -> Task-0012 exact)", dispatcher.dispatched, want)
	}
}

// A4.3 (consumer-driven, in-loop via DecideQueueAction): on closed the consumer
// invokes the terminal/reclaim path; on Human Needed=Yes it parks (SetRunGateState)
// and does NOT redispatch and does NOT reclaim — the slot is retained.
func TestDrainOnceClosedReclaimsAndHumanNeededParks(t *testing.T) {
	// Both tasks already own a worktree (used slots). #7010 is closed (terminal),
	// #7011 is Human Needed=Yes even though Queue still reads Ready (park wins).
	provider := &fakeProvider{issues: []IssueRef{
		{Number: 7010, State: IssueState{Closed: true, Queue: QueueReady}},
		{Number: 7011, State: IssueState{HumanNeeded: true, Queue: QueueReady}},
	}}
	dispatcher := newFakeDispatcher("Task-7010", "Task-7011")
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedSizer(4))

	result, err := consumer.DrainOnce(context.Background())
	if err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}
	// Closed => reclaimed (the ONLY deallocating action).
	if want := []string{"Task-7010"}; !reflect.DeepEqual(dispatcher.reclaimed, want) {
		t.Fatalf("reclaimed = %v, want %v (closed is terminal)", dispatcher.reclaimed, want)
	}
	// Human Needed=Yes => parked awaiting closure, never redispatched.
	if state := dispatcher.parked["Task-7011"]; state != runGateStateParkedAwaitingClosure {
		t.Fatalf("Task-7011 parked state = %q, want %q", state, runGateStateParkedAwaitingClosure)
	}
	if len(dispatcher.dispatched) != 0 {
		t.Fatalf("dispatched = %v, want none (parked issue must NOT redispatch even though Queue=Ready)", dispatcher.dispatched)
	}
	// Parked task's worktree/slot is retained: it is NOT in the reclaimed set.
	for _, t7011 := range dispatcher.reclaimed {
		if t7011 == "Task-7011" {
			t.Fatal("parked Task-7011 must NOT be reclaimed; the slot is retained")
		}
	}
	if !reflect.DeepEqual(result.Reclaimed, []string{"Task-7010"}) {
		t.Fatalf("result.Reclaimed = %v, want [Task-7010]", result.Reclaimed)
	}
	if !reflect.DeepEqual(result.Parked, []string{"Task-7011"}) {
		t.Fatalf("result.Parked = %v, want [Task-7011]", result.Parked)
	}
}

// A4.4 (consumer-driven): a parked Human Needed=Yes task that still owns its
// worktree is NOT redispatched on a later poll while it remains parked — the slot
// stays occupied and the same worktree is retained (no re-provision/eviction).
func TestDrainOnceParkedTaskIsNotRedispatchedAndRetainsSlot(t *testing.T) {
	provider := &fakeProvider{issues: []IssueRef{
		{Number: 7011, State: IssueState{HumanNeeded: true, GateHint: GateHintPlan, Queue: QueueReady}},
	}}
	dispatcher := newFakeDispatcher("Task-7011")
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedSizer(4))

	if _, err := consumer.DrainOnce(context.Background()); err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}
	if len(dispatcher.dispatched) != 0 {
		t.Fatalf("dispatched = %v, want none (parked task must not be redispatched)", dispatcher.dispatched)
	}
	if state := dispatcher.parked["Task-7011"]; state != runGateStateParkedPlan {
		t.Fatalf("parked state = %q, want %q (plan-gate park)", state, runGateStateParkedPlan)
	}
	// Slot retained: still active, never reclaimed.
	active, _ := dispatcher.ActiveOwnedLaneTasks()
	if !reflect.DeepEqual(active, []string{"Task-7011"}) {
		t.Fatalf("active owned lanes = %v, want [Task-7011] retained", active)
	}
	if len(dispatcher.reclaimed) != 0 {
		t.Fatalf("reclaimed = %v, want none (park retains the worktree)", dispatcher.reclaimed)
	}
}

// Slot accounting: with queue_workers=2 and 2 Ready issues already filling the
// slots, a 3rd Ready issue is NOT dispatched (re-picked once a slot frees).
func TestDrainOnceRespectsPerRepoSlotCap(t *testing.T) {
	provider := &fakeProvider{issues: []IssueRef{
		{Number: 7020, State: IssueState{Queue: QueueReady}},
		{Number: 7021, State: IssueState{Queue: QueueReady}},
		{Number: 7022, State: IssueState{Queue: QueueReady}},
	}}
	dispatcher := newFakeDispatcher()
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedSizer(2))

	result, err := consumer.DrainOnce(context.Background())
	if err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}
	if want := []string{"Task-7020", "Task-7021"}; !reflect.DeepEqual(dispatcher.dispatched, want) {
		t.Fatalf("dispatched = %v, want %v (cap of 2)", dispatcher.dispatched, want)
	}
	if !reflect.DeepEqual(result.Skipped, []string{"Task-7022"}) {
		t.Fatalf("skipped = %v, want [Task-7022] (refused while full)", result.Skipped)
	}
}

// An already-running Ready issue is not re-dispatched (it already owns its slot).
func TestDrainOnceDoesNotRedispatchAlreadyRunningReadyTask(t *testing.T) {
	provider := &fakeProvider{issues: []IssueRef{
		{Number: 7030, State: IssueState{Queue: QueueReady}},
	}}
	dispatcher := newFakeDispatcher("Task-7030")
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedSizer(4))

	if _, err := consumer.DrainOnce(context.Background()); err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}
	if len(dispatcher.dispatched) != 0 {
		t.Fatalf("dispatched = %v, want none (already running)", dispatcher.dispatched)
	}
}
