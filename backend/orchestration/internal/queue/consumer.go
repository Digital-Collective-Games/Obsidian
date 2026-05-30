package queue

import (
	"context"
	"fmt"
	"sort"
)

// Dispatcher is the seam the queue-drain consumer uses to act on issue state. It
// keeps internal/queue a leaf package (it must NOT import taskrun, which already
// imports queue); production wires these methods to the existing taskrun.Service
// (Dispatch, SetRunGateState, the cleanup-on-close path) without duplicating that
// logic. The consumer NEVER calls a manual HTTP /dispatch route — it invokes
// Dispatch directly, which IS the autonomous queue-drain dispatch (A3.1).
type Dispatcher interface {
	// Dispatch dispatches taskID into an owned-lane worktree through the existing
	// taskrun dispatch path. It is the same entry the manual POST .../dispatch
	// route calls, invoked here WITHOUT a manual call (A3.1).
	Dispatch(ctx context.Context, taskID string) error
	// SetRunGateState records the parked run/gate state for a Human Needed=Yes task
	// (park in place; retain worktree+slot; never redispatch). A task with no active
	// owned lane is a no-op (nothing is parked) — implementations report that as a
	// non-fatal condition the consumer tolerates.
	SetRunGateState(taskID string, state string) error
	// Reclaim performs the terminal close handling for a closed issue: reclaim the
	// owned worktree and free the slot (cleanupOwnedLane). A task with no active
	// owned lane is a no-op.
	Reclaim(ctx context.Context, taskID string) error
	// ActiveOwnedLaneTasks returns the task ids that currently hold an owned-lane
	// worktree (one per used slot). The consumer uses it for slot accounting
	// (EvaluateSlot) and to know which issues already own a worktree to park/reclaim.
	ActiveOwnedLaneTasks() ([]string, error)
}

// SlotSizer resolves the per-repo queue_workers cap (max concurrent owned lanes)
// for the consumer's repo. Production reads it from REPO-MANIFEST.json; tests pin
// it directly.
type SlotSizer interface {
	RepoSlotLimit() int
}

// Consumer is the GitHub queue-drain consumer core. DrainOnce is the single poll
// step: it reads the provider's issues, applies DecideQueueAction to each, and
// for eligible (open + Queue=Ready + not-parked) issues maps #N -> Tracking/Task-N
// and dispatches into a free per-repo slot. It is pure orchestration over the
// Dispatcher + DecideQueueAction + EvaluateSlot seams (no Temporal, no GitHub
// writes), so it is fully unit-testable with a FAKE provider and a FAKE dispatcher.
type Consumer struct {
	repo       string
	provider   QueueProvider
	dispatcher Dispatcher
	sizer      SlotSizer
}

// NewConsumer builds a consumer for a provider repo.
func NewConsumer(repo string, provider QueueProvider, dispatcher Dispatcher, sizer SlotSizer) *Consumer {
	return &Consumer{repo: repo, provider: provider, dispatcher: dispatcher, sizer: sizer}
}

// DrainResult summarizes one DrainOnce poll for logging/proof.
type DrainResult struct {
	Dispatched []string
	Parked     []string
	Reclaimed  []string
	Skipped    []string
}

// TaskIDForIssue maps a GitHub issue #N to its Tracking/Task-N id EXACTLY, with no
// mapping layer (SKILL.md provider contract, A3.3): issue #12 -> "Task-0012". The
// number is zero-padded to four digits to match the on-disk Tracking/Task-NNNN
// directory convention.
func TaskIDForIssue(number int) string {
	return fmt.Sprintf("Task-%04d", number)
}

// DrainOnce performs one poll cycle. For each issue, DecideQueueAction yields the
// action; the consumer wires it to the Dispatcher:
//   - ActionDispatch: only if a per-repo slot is free (EvaluateSlot over the live
//     owned-lane count) AND the task does not already own a lane. Skip Queue=Never.
//   - ActionPark (Human Needed=Yes): record the parked state in place; never
//     redispatch; never reclaim.
//   - ActionTerminal (closed): reclaim the worktree + free the slot (only if the
//     task still owns a lane).
//   - ActionNone: skip.
//
// Used-slot accounting is taken once at the start of the cycle and incremented as
// the consumer dispatches, so it never over-admits past queue_workers within a
// single poll.
func (c *Consumer) DrainOnce(ctx context.Context) (DrainResult, error) {
	result := DrainResult{}
	issues, err := c.provider.ListReadyIssues(c.repo)
	if err != nil {
		return result, fmt.Errorf("list issues for %s: %w", c.repo, err)
	}

	activeTasks, err := c.dispatcher.ActiveOwnedLaneTasks()
	if err != nil {
		return result, fmt.Errorf("count active owned lanes: %w", err)
	}
	active := map[string]bool{}
	for _, taskID := range activeTasks {
		active[taskID] = true
	}
	limit := c.sizer.RepoSlotLimit()
	used := len(activeTasks)

	// Deterministic order: lowest issue number first, so proof and slot admission
	// are stable regardless of provider iteration order.
	sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })

	for _, issue := range issues {
		taskID := TaskIDForIssue(issue.Number)
		decision := DecideQueueAction(issue.State)
		switch decision.Action {
		case ActionTerminal:
			if !active[taskID] {
				result.Skipped = append(result.Skipped, taskID)
				continue
			}
			if err := c.dispatcher.Reclaim(ctx, taskID); err != nil {
				return result, fmt.Errorf("reclaim %s: %w", taskID, err)
			}
			delete(active, taskID)
			used--
			result.Reclaimed = append(result.Reclaimed, taskID)
		case ActionPark:
			if err := c.dispatcher.SetRunGateState(taskID, decision.ParkState); err != nil {
				return result, fmt.Errorf("park %s: %w", taskID, err)
			}
			result.Parked = append(result.Parked, taskID)
		case ActionDispatch:
			if active[taskID] {
				// Already running in a slot — not a new dispatch.
				result.Skipped = append(result.Skipped, taskID)
				continue
			}
			if !EvaluateSlot(limit, used).Admit {
				// Full: re-picked on a later poll once a close frees a slot.
				result.Skipped = append(result.Skipped, taskID)
				continue
			}
			if err := c.dispatcher.Dispatch(ctx, taskID); err != nil {
				return result, fmt.Errorf("dispatch %s: %w", taskID, err)
			}
			active[taskID] = true
			used++
			result.Dispatched = append(result.Dispatched, taskID)
		default:
			result.Skipped = append(result.Skipped, taskID)
		}
	}
	return result, nil
}
