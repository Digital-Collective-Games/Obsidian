package temporalbackend

import (
	"context"
	"errors"
	"fmt"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/queue"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/taskrun"
)

// taskrunDispatcher adapts the existing taskrun.Service to the queue.Dispatcher
// seam the O3 consumer drives. It REUSES the existing dispatch / park / reclaim
// paths (Dispatch, SetRunGateState, ReclaimOwnedLane) — it does not reimplement
// dispatch or slot logic. A park/reclaim for a task that has no active owned lane
// is tolerated as a no-op (ErrNoActiveOwnedLane), so a poll that observes a
// Human Needed=Yes or closed issue with no live worktree does not wedge the loop.
type taskrunDispatcher struct {
	service *taskrun.Service
}

func (d taskrunDispatcher) Dispatch(ctx context.Context, taskID string) error {
	_, err := d.service.Dispatch(ctx, taskID)
	return err
}

func (d taskrunDispatcher) SetRunGateState(taskID string, state string) error {
	_, err := d.service.SetRunGateState(taskID, state)
	if errors.Is(err, taskrun.ErrNoActiveOwnedLane) {
		return nil
	}
	return err
}

func (d taskrunDispatcher) Reclaim(_ context.Context, taskID string) error {
	err := d.service.ReclaimOwnedLane(taskID)
	if errors.Is(err, taskrun.ErrNoActiveOwnedLane) {
		return nil
	}
	return err
}

func (d taskrunDispatcher) ActiveOwnedLaneTasks() ([]string, error) {
	return d.service.ActiveOwnedLaneTasks()
}

// newQueueDrainActivities builds the consumer (gh provider + taskrun-backed
// dispatcher + the service's manifest slot sizer) for the worker's poll activity.
// A nil/empty provider repo or an unparsable repo yields nil activities so the
// workflow can still be registered while the consumer stays dormant.
func newQueueDrainActivities(repo string, service *taskrun.Service) (*queue.QueueDrainActivities, error) {
	if repo == "" || service == nil {
		return nil, nil
	}
	provider, err := queue.NewGitHubQueueProvider(repo, 0)
	if err != nil {
		return nil, fmt.Errorf("build queue provider: %w", err)
	}
	consumer := queue.NewConsumer(repo, provider, taskrunDispatcher{service: service}, service)
	return queue.NewQueueDrainActivities(consumer), nil
}

// StartQueueDrainConsumer starts the singleton queue-drain consumer workflow
// (A3.4). A duplicate start (the consumer is already running) is reported as a
// non-fatal AlreadyStarted so the start/stop endpoint is idempotent.
func (b *Backend) StartQueueDrainConsumer(ctx context.Context, config queue.QueueDrainConfig) (string, error) {
	run, err := b.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                    queue.QueueDrainWorkflowID,
		TaskQueue:             b.cfg.TaskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	}, queue.QueueDrainWorkflowName, config)
	if err != nil {
		return "", fmt.Errorf("start queue-drain consumer: %w", err)
	}
	return run.GetID(), nil
}

// StopQueueDrainConsumer signals the running consumer to stop draining and exit.
// A not-found consumer (already stopped) is tolerated so stop is idempotent.
func (b *Backend) StopQueueDrainConsumer(ctx context.Context) error {
	if err := b.client.SignalWorkflow(ctx, queue.QueueDrainWorkflowID, "", queue.QueueDrainStopSignalName, nil); err != nil {
		if isTemporalNotFound(err) {
			return nil
		}
		return fmt.Errorf("stop queue-drain consumer: %w", err)
	}
	return nil
}
