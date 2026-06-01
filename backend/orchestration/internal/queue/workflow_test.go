package queue

import (
	"context"
	"testing"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// TestQueueDrainWorkflowPollsThenDispatchesAndStops runs the registered queue-drain
// workflow + its poll activity in the Temporal test environment, proving the
// consumer dispatches a Queue==Ready issue THROUGH THE WORKFLOW (the poll activity
// calls Consumer.DrainOnce, which invokes the dispatch seam — no manual call,
// A3.1), and that the stop signal exits the loop (A3.4 start/stop behavior).
func TestQueueDrainWorkflowPollsThenDispatchesAndStops(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()

	provider := &fakeProvider{issues: []IssueRef{
		{Number: 7001, State: IssueState{Queue: QueueReady}},
	}}
	dispatcher := newFakeDispatcher()
	consumer := singleEntryRegistryConsumer(testRepo, provider, dispatcher, fixedIdleSizer(4))
	activities := NewQueueDrainActivities(consumer)

	env.RegisterWorkflowWithOptions(QueueDrainWorkflow, workflow.RegisterOptions{Name: QueueDrainWorkflowName})
	env.RegisterActivityWithOptions(activities.Poll, activity.RegisterOptions{Name: DrainPollActivityName})

	// After the first poll fires, signal stop so the loop exits deterministically.
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(QueueDrainStopSignalName, nil)
	}, 30*time.Second)

	env.ExecuteWorkflow(QueueDrainWorkflow, QueueDrainConfig{Repo: testRepo, PollInterval: time.Minute})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}
	if len(dispatcher.dispatched) != 1 || dispatcher.dispatched[0] != "Task-7001" {
		t.Fatalf("dispatched = %v, want [Task-7001] (Ready dispatched through the workflow poll, no manual call)", dispatcher.dispatched)
	}
	if provider.calls == 0 {
		t.Fatal("provider was never polled by the workflow activity")
	}
}

// registryRecorder records workflow/activity registrations so a test can assert
// StartWorker-style registration WITHOUT a live Temporal server. It satisfies the
// minimal workflowActivityRegistrar interface queue.Register depends on.
type registryRecorder struct {
	workflows  []string
	activities int
}

func (w *registryRecorder) RegisterWorkflowWithOptions(_ interface{}, opts workflow.RegisterOptions) {
	w.workflows = append(w.workflows, opts.Name)
}
func (w *registryRecorder) RegisterActivityWithOptions(_ interface{}, _ activity.RegisterOptions) {
	w.activities++
}

// TestRegisterRegistersWorkflowAndActivity proves queue.Register wires the
// queue-drain workflow (and, with a live consumer, its poll activity) onto a
// worker — the registration StartWorker performs next to taskexec.Register (A3.4).
func TestRegisterRegistersWorkflowAndActivity(t *testing.T) {
	provider := &fakeProvider{}
	dispatcher := newFakeDispatcher()
	consumer := singleEntryRegistryConsumer(testRepo, provider, dispatcher, fixedIdleSizer(4))
	activities := NewQueueDrainActivities(consumer)

	w := &registryRecorder{}
	Register(w, activities)

	if len(w.workflows) != 1 || w.workflows[0] != QueueDrainWorkflowName {
		t.Fatalf("registered workflows = %v, want [%s]", w.workflows, QueueDrainWorkflowName)
	}
	if w.activities != 1 {
		t.Fatalf("registered %d activities, want 1 (the poll activity)", w.activities)
	}

	// With nil activities the workflow is still registered but no activity is.
	w2 := &registryRecorder{}
	Register(w2, nil)
	if len(w2.workflows) != 1 {
		t.Fatalf("nil-activities Register should still register the workflow, got %v", w2.workflows)
	}
	if w2.activities != 0 {
		t.Fatalf("nil-activities Register should register no activity, got %d", w2.activities)
	}

	// Touch the consumer's context-using path so the import stays meaningful.
	_, _ = consumer.DrainOnce(context.Background())
}
