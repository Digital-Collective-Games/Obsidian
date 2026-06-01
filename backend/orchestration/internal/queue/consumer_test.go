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
	// closed records every (repo, number) the consumer asked CloseIssue to close, so
	// the auto-close tests can assert the exact issue closed (and that none closed
	// when the flag is off).
	closed []closedIssue
	// dequeued records every (repo, number) DequeueIssue (Queue -> Never) was called
	// for, so dequeue/eject tests can assert the write without touching real GitHub.
	dequeued []closedIssue
}

type closedIssue struct {
	repo   string
	number int
}

func (p *fakeProvider) ListReadyIssues(string) ([]IssueRef, error) {
	p.calls++
	return append([]IssueRef(nil), p.issues...), nil
}

func (p *fakeProvider) CloseIssue(repo string, number int) error {
	p.closed = append(p.closed, closedIssue{repo: repo, number: number})
	return nil
}

func (p *fakeProvider) DequeueIssue(repo string, number int) error {
	p.dequeued = append(p.dequeued, closedIssue{repo: repo, number: number})
	// Model the real provider write: flip the issue's observed Queue to Never so the next
	// poll's ListReadyIssues reflects the dequeue (the no-bounce-back behavior).
	for i := range p.issues {
		if p.issues[i].Number == number {
			p.issues[i].State.Queue = QueueNever
		}
	}
	return nil
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
	// closureRequested[taskID]=true models a dispatched agent that has ANNOUNCED
	// completion (current_gate=="closure") so the auto-close path can act on it.
	closureRequested map[string]bool
}

func newFakeDispatcher(active ...string) *fakeDispatcher {
	return &fakeDispatcher{active: append([]string(nil), active...), parked: map[string]string{}, closureRequested: map[string]bool{}}
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

func (d *fakeDispatcher) ClosureRequested(taskID string) (bool, error) {
	return d.closureRequested[taskID], nil
}

// fixedIdleSizer pins the per-repo IDLE worktree count for deterministic pool-admission
// tests (Task-0016): the consumer admits a dispatch only while an idle worktree remains
// to draw, so fixedIdleSizer(N) models a pool of N idle worktrees.
type fixedIdleSizer int

func (s fixedIdleSizer) IdleWorktreeCount() (int, error) { return int(s), nil }

const testRepo = "Digital-Collective-Games/QueueDrainTestbed"

// singleEntryRegistryConsumer wraps one fake provider/dispatcher/sizer in a
// registry-driven consumer over a single-entry fake registry, so tests that
// exercise the WORKFLOW poll path (which now drives a RegistryConsumer) can reuse
// the existing per-repo fakes. The fake registry yields exactly one repo whose
// task_provider.repo is providerRepo and whose local_root is a synthetic path.
func singleEntryRegistryConsumer(providerRepo string, provider QueueProvider, dispatcher Dispatcher, sizer PoolSizer) *RegistryConsumer {
	loadRegistry := func() (RepoManifest, error) {
		return RepoManifest{Repos: []RepoEntry{{
			ID:           "TestRepo",
			LocalRoot:    "C:\\Agent\\TestRepo",
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
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedIdleSizer(4))

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
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedIdleSizer(4))

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
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedIdleSizer(4))
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
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedIdleSizer(4))

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
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedIdleSizer(4))

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

// Pool admission (Task-0016): with a pool of 2 idle worktrees and 3 Ready issues, the
// consumer draws both idle worktrees (dispatches 2) and the 3rd Ready issue is NOT
// dispatched — it waits for an idle worktree (re-picked once an Eject/close frees one).
func TestDrainOnceDispatchesUpToIdlePoolThenWaits(t *testing.T) {
	provider := &fakeProvider{issues: []IssueRef{
		{Number: 7020, State: IssueState{Queue: QueueReady}},
		{Number: 7021, State: IssueState{Queue: QueueReady}},
		{Number: 7022, State: IssueState{Queue: QueueReady}},
	}}
	dispatcher := newFakeDispatcher()
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedIdleSizer(2))

	result, err := consumer.DrainOnce(context.Background())
	if err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}
	if want := []string{"Task-7020", "Task-7021"}; !reflect.DeepEqual(dispatcher.dispatched, want) {
		t.Fatalf("dispatched = %v, want %v (pool of 2 idle worktrees)", dispatcher.dispatched, want)
	}
	if !reflect.DeepEqual(result.Skipped, []string{"Task-7022"}) {
		t.Fatalf("skipped = %v, want [Task-7022] (waits for an idle worktree)", result.Skipped)
	}
}

// REG-007 "pool of 1": with a single idle worktree and two Ready issues, the consumer
// dispatches exactly ONE and the second waits (no idle worktree to draw). This is the
// pool-model reinterpretation of the former cap=1 sub-scenario.
func TestDrainOncePoolOfOneDispatchesExactlyOne(t *testing.T) {
	provider := &fakeProvider{issues: []IssueRef{
		{Number: 7050, State: IssueState{Queue: QueueReady}},
		{Number: 7051, State: IssueState{Queue: QueueReady}},
	}}
	dispatcher := newFakeDispatcher()
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedIdleSizer(1))

	result, err := consumer.DrainOnce(context.Background())
	if err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}
	if want := []string{"Task-7050"}; !reflect.DeepEqual(dispatcher.dispatched, want) {
		t.Fatalf("dispatched = %v, want %v (pool of 1)", dispatcher.dispatched, want)
	}
	if !reflect.DeepEqual(result.Skipped, []string{"Task-7051"}) {
		t.Fatalf("skipped = %v, want [Task-7051] (second waits for an idle worktree)", result.Skipped)
	}
}

// Empty pool: with zero idle worktrees, a Ready issue is deferred (no auto-create).
func TestDrainOnceEmptyPoolDefersReadyIssue(t *testing.T) {
	provider := &fakeProvider{issues: []IssueRef{
		{Number: 7060, State: IssueState{Queue: QueueReady}},
	}}
	dispatcher := newFakeDispatcher()
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedIdleSizer(0))

	result, err := consumer.DrainOnce(context.Background())
	if err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}
	if len(dispatcher.dispatched) != 0 {
		t.Fatalf("dispatched = %v, want none (empty pool defers)", dispatcher.dispatched)
	}
	if !reflect.DeepEqual(result.Skipped, []string{"Task-7060"}) {
		t.Fatalf("skipped = %v, want [Task-7060] (empty pool, no auto-create)", result.Skipped)
	}
}

// An already-running Ready issue is not re-dispatched (it already owns its slot).
func TestDrainOnceDoesNotRedispatchAlreadyRunningReadyTask(t *testing.T) {
	provider := &fakeProvider{issues: []IssueRef{
		{Number: 7030, State: IssueState{Queue: QueueReady}},
	}}
	dispatcher := newFakeDispatcher("Task-7030")
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedIdleSizer(4))

	if _, err := consumer.DrainOnce(context.Background()); err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}
	if len(dispatcher.dispatched) != 0 {
		t.Fatalf("dispatched = %v, want none (already running)", dispatcher.dispatched)
	}
}

// IssueNumberFromTaskID parses Task-N back to N (inverse of TaskIDForIssue) and
// errors on a malformed id so a close is never attempted against a bad number.
func TestIssueNumberFromTaskID(t *testing.T) {
	ok := map[string]int{
		"Task-0008":  8,
		"Task-0012":  12,
		"Task-7001":  7001,
		"Task-10000": 10000,
	}
	for taskID, want := range ok {
		got, err := IssueNumberFromTaskID(taskID)
		if err != nil {
			t.Fatalf("IssueNumberFromTaskID(%q) error: %v", taskID, err)
		}
		if got != want {
			t.Fatalf("IssueNumberFromTaskID(%q) = %d, want %d", taskID, got, want)
		}
	}
	for _, bad := range []string{"", "Task-", "0008", "Task-00x8", "Issue-8"} {
		if _, err := IssueNumberFromTaskID(bad); err == nil {
			t.Fatalf("IssueNumberFromTaskID(%q) = nil error, want error on malformed id", bad)
		}
	}
}

// TEST-ONLY auto-close (ENABLED): an active dispatched task that announced completion
// (ClosureRequested=true) gets its GitHub issue closed with the correct repo + parsed
// issue number, and DrainResult.AutoClosed records it. The consumer does NOT reclaim
// here — the next poll observes the closed issue and reclaims via ActionTerminal.
func TestDrainOnceAutoCloseClosesAnnouncedTaskWhenEnabled(t *testing.T) {
	// #7040 is open + Ready and already owns its slot (an active dispatched lane). It
	// announced completion via current_gate=="closure".
	provider := &fakeProvider{issues: []IssueRef{
		{Number: 7040, State: IssueState{Queue: QueueReady}},
	}}
	dispatcher := newFakeDispatcher("Task-7040")
	dispatcher.closureRequested["Task-7040"] = true
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedIdleSizer(4))
	consumer.SetAutoCloseEnabled(true)

	result, err := consumer.DrainOnce(context.Background())
	if err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}
	if want := []closedIssue{{repo: testRepo, number: 7040}}; !reflect.DeepEqual(provider.closed, want) {
		t.Fatalf("closed = %v, want %v (close issue #7040 on the repo, like a human)", provider.closed, want)
	}
	if !reflect.DeepEqual(result.AutoClosed, []string{"Task-7040"}) {
		t.Fatalf("result.AutoClosed = %v, want [Task-7040]", result.AutoClosed)
	}
	// Auto-close must NOT also reclaim: deallocation stays the next-poll ActionTerminal
	// path (the issue is still open this poll).
	if len(dispatcher.reclaimed) != 0 {
		t.Fatalf("reclaimed = %v, want none (auto-close closes only; next poll reclaims)", dispatcher.reclaimed)
	}
}

// TEST-ONLY auto-close (DISABLED, the default): even an announced task is NOT closed —
// the consumer stays read-only against GitHub (A4.6).
func TestDrainOnceAutoCloseDoesNotCloseWhenDisabled(t *testing.T) {
	provider := &fakeProvider{issues: []IssueRef{
		{Number: 7041, State: IssueState{Queue: QueueReady}},
	}}
	dispatcher := newFakeDispatcher("Task-7041")
	dispatcher.closureRequested["Task-7041"] = true
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedIdleSizer(4)) // auto-close left OFF (default)

	result, err := consumer.DrainOnce(context.Background())
	if err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}
	if len(provider.closed) != 0 {
		t.Fatalf("closed = %v, want none (auto-close off => read-only against GitHub)", provider.closed)
	}
	if len(result.AutoClosed) != 0 {
		t.Fatalf("result.AutoClosed = %v, want none (auto-close off)", result.AutoClosed)
	}
}

// Task-0016 PASS-0005 / AC13: the no-bounce-back seam. After an Eject of a task whose
// issue is Queue=Ready, the Eject dequeue (Queue -> Never via the provider) means a
// subsequent consumer poll does NOT re-dispatch the freed task. The load-bearing variant
// (Eject that SKIPS the dequeue, leaving the issue Ready) shows the task IS re-dispatched,
// proving the dequeue is what prevents the bounce-back.
func TestEjectThenNoBounceBackOnNextPoll(t *testing.T) {
	// Pool of 1 idle worktree; #7070 is Ready and dispatches on the first poll.
	provider := &fakeProvider{issues: []IssueRef{
		{Number: 7070, State: IssueState{Queue: QueueReady}},
	}}
	dispatcher := newFakeDispatcher()
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedIdleSizer(1))

	first, err := consumer.DrainOnce(context.Background())
	if err != nil {
		t.Fatalf("first DrainOnce: %v", err)
	}
	if want := []string{"Task-7070"}; !reflect.DeepEqual(first.Dispatched, want) {
		t.Fatalf("first poll dispatched = %v, want %v", first.Dispatched, want)
	}

	// Eject: free the dispatcher's lane AND dequeue the issue through the provider
	// (Queue -> Never), exactly as Service.EjectWorktree does.
	dispatcher.active = nil
	if err := provider.DequeueIssue(testRepo, 7070); err != nil {
		t.Fatalf("provider dequeue: %v", err)
	}

	// Next poll: the issue now reads Never, so it is NOT re-dispatched (no bounce-back).
	second, err := consumer.DrainOnce(context.Background())
	if err != nil {
		t.Fatalf("second DrainOnce: %v", err)
	}
	if len(second.Dispatched) != 0 {
		t.Fatalf("second poll dispatched = %v, want none (Eject dequeued it -> no bounce-back)", second.Dispatched)
	}

	// Load-bearing variant: an Eject that SKIPS the dequeue leaves the issue Ready, so the
	// next poll DOES re-dispatch it (the bounce-back the dequeue prevents).
	provider2 := &fakeProvider{issues: []IssueRef{
		{Number: 7071, State: IssueState{Queue: QueueReady}},
	}}
	dispatcher2 := newFakeDispatcher()
	consumer2 := NewConsumer(testRepo, provider2, dispatcher2, fixedIdleSizer(1))
	if _, err := consumer2.DrainOnce(context.Background()); err != nil {
		t.Fatalf("variant first DrainOnce: %v", err)
	}
	dispatcher2.active = nil // eject frees the lane, but DO NOT dequeue (issue stays Ready)
	bounce, err := consumer2.DrainOnce(context.Background())
	if err != nil {
		t.Fatalf("variant second DrainOnce: %v", err)
	}
	if want := []string{"Task-7071"}; !reflect.DeepEqual(bounce.Dispatched, want) {
		t.Fatalf("variant second poll dispatched = %v, want %v (skipping the dequeue bounces it back)", bounce.Dispatched, want)
	}
}

// TEST-ONLY auto-close (ENABLED but NOT announced): an active task that has NOT set
// current_gate=="closure" (ClosureRequested=false) is not closed.
func TestDrainOnceAutoCloseDoesNotCloseWhenNotAnnounced(t *testing.T) {
	provider := &fakeProvider{issues: []IssueRef{
		{Number: 7042, State: IssueState{Queue: QueueReady}},
	}}
	dispatcher := newFakeDispatcher("Task-7042") // active, but closureRequested stays false
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedIdleSizer(4))
	consumer.SetAutoCloseEnabled(true)

	result, err := consumer.DrainOnce(context.Background())
	if err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}
	if len(provider.closed) != 0 {
		t.Fatalf("closed = %v, want none (task has not announced completion)", provider.closed)
	}
	if len(result.AutoClosed) != 0 {
		t.Fatalf("result.AutoClosed = %v, want none (not announced)", result.AutoClosed)
	}
}
