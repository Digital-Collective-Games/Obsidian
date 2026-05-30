package queue

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// workflowActivityRegistrar is the minimal slice of worker.Worker that Register
// uses. worker.Worker satisfies it structurally (so StartWorker passes its real
// worker unchanged), and a test can satisfy it with a tiny recorder rather than
// implementing the full worker.Worker interface.
type workflowActivityRegistrar interface {
	RegisterWorkflowWithOptions(w interface{}, options workflow.RegisterOptions)
	RegisterActivityWithOptions(a interface{}, options activity.RegisterOptions)
}

// Temporal identifiers for the O3 queue-drain consumer. The workflow is a sibling
// of taskexec.TaskRunWorkflow and is registered in StartWorker next to
// taskexec.Register (A3.4). It owns NO per-run execution — it only polls and
// dispatches; the per-run lifecycle stays in TaskRunWorkflow.
const (
	// QueueDrainWorkflowName is the registered workflow type for the consumer.
	QueueDrainWorkflowName = "codex.queue.drain"
	// QueueDrainWorkflowID is the singleton workflow id the start/stop endpoint uses
	// (one consumer per backend; a duplicate Start is rejected by Temporal).
	QueueDrainWorkflowID = "queue-drain--consumer"
	// QueueDrainStopSignalName signals the running consumer to stop draining and exit.
	QueueDrainStopSignalName = "queue.drain.stop"
	// DrainPollActivityName is the activity that performs one DrainOnce poll. The poll
	// touches GitHub (provider) + the taskrun dispatch path, so it MUST be an activity
	// (workflow code stays deterministic).
	DrainPollActivityName = "queue.drain.poll"
)

// DefaultPollInterval is the consumer's poll cadence. The human confirmed polling
// (pull) at ~2 minutes at the plan gate; webhooks/tunnels are out of scope.
const DefaultPollInterval = 2 * time.Minute

// QueueDrainConfig is the start argument for the consumer workflow. PollInterval
// and the provider repo are configurable (HUMAN-DIRECTIVES O3); a non-positive
// interval falls back to DefaultPollInterval.
type QueueDrainConfig struct {
	// Repo is the provider repo the consumer polls (owner/name).
	Repo string `json:"repo"`
	// PollInterval is the poll cadence; <=0 uses DefaultPollInterval.
	PollInterval time.Duration `json:"poll_interval"`
}

// QueueDrainActivities holds the live consumer the poll activity drives. It is
// constructed once at worker start (with the gh provider + the taskrun-backed
// dispatcher) and registered on the worker; the workflow stays deterministic by
// only scheduling the activity and a timer.
type QueueDrainActivities struct {
	consumer *Consumer
}

// NewQueueDrainActivities wires the poll activity to a live consumer.
func NewQueueDrainActivities(consumer *Consumer) *QueueDrainActivities {
	return &QueueDrainActivities{consumer: consumer}
}

// Poll runs exactly one DrainOnce cycle.
func (a *QueueDrainActivities) Poll(ctx context.Context, _ QueueDrainConfig) (DrainResult, error) {
	if a.consumer == nil {
		return DrainResult{}, nil
	}
	return a.consumer.DrainOnce(ctx)
}

// Register registers the queue-drain workflow and (when activities are provided)
// its poll activity on the worker, next to taskexec.Register in StartWorker
// (A3.4). Passing nil activities registers only the workflow (e.g. a worker that
// hosts the workflow but no live consumer); the poll then no-ops.
func Register(w workflowActivityRegistrar, activities *QueueDrainActivities) {
	w.RegisterWorkflowWithOptions(QueueDrainWorkflow, workflow.RegisterOptions{Name: QueueDrainWorkflowName})
	if activities != nil {
		w.RegisterActivityWithOptions(activities.Poll, activity.RegisterOptions{Name: DrainPollActivityName})
	}
}

// QueueDrainWorkflow is the long-running consumer: it polls the provider on the
// configured interval and exits when the stop signal arrives. Each poll runs the
// DrainPollActivityName activity (which calls Consumer.DrainOnce). A failed poll
// is logged and the loop continues on the next tick — a transient GitHub error
// must not wedge the queue. The loop uses ContinueAsNew after a bounded number of
// polls to keep history small (standard long-poll pattern).
func QueueDrainWorkflow(ctx workflow.Context, config QueueDrainConfig) error {
	interval := config.PollInterval
	if interval <= 0 {
		interval = DefaultPollInterval
	}

	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 1},
	})

	stopCh := workflow.GetSignalChannel(ctx, QueueDrainStopSignalName)
	logger := workflow.GetLogger(ctx)

	// pollsBeforeContinue bounds workflow history before ContinueAsNew.
	const pollsBeforeContinue = 200

	for poll := 0; poll < pollsBeforeContinue; poll++ {
		var result DrainResult
		if err := workflow.ExecuteActivity(activityCtx, DrainPollActivityName, config).Get(activityCtx, &result); err != nil {
			logger.Warn("queue-drain poll failed; continuing on next tick", "error", err.Error())
		} else if len(result.Dispatched) > 0 || len(result.Parked) > 0 || len(result.Reclaimed) > 0 {
			logger.Info("queue-drain poll acted",
				"dispatched", result.Dispatched,
				"parked", result.Parked,
				"reclaimed", result.Reclaimed)
		}

		stopped := false
		timer := workflow.NewTimer(ctx, interval)
		selector := workflow.NewSelector(ctx)
		selector.AddFuture(timer, func(workflow.Future) {})
		selector.AddReceive(stopCh, func(c workflow.ReceiveChannel, _ bool) {
			c.Receive(ctx, nil)
			stopped = true
		})
		selector.Select(ctx)
		if stopped {
			logger.Info("queue-drain consumer stop signal received; exiting")
			return nil
		}
	}

	return workflow.NewContinueAsNewError(ctx, QueueDrainWorkflowName, config)
}
