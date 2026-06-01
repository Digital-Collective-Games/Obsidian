package queue

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
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
	// worktree (one per allocated pool worktree). The consumer uses it to know which
	// issues already own a worktree to park/reclaim rather than redispatch.
	ActiveOwnedLaneTasks() ([]string, error)
	// ClosureRequested reports whether the task's dispatched agent has ANNOUNCED
	// completion by setting its worktree TASK-STATE.json current_gate to "closure".
	// It is read by the TEST-ONLY auto-close path only. A task with no active owned
	// lane (or no state file) reports false, nil — not an error.
	ClosureRequested(taskID string) (bool, error)
}

// PoolSizer reports how many IDLE worktrees the consumer's repo pool currently has —
// the admission budget that replaced the queue_workers cap (Task-0016). Concurrency is
// bounded by the count of idle pool worktrees by construction: the consumer admits a
// dispatch only while an idle worktree remains to draw, and an empty pool defers the
// Ready issue. Production reads it from the on-disk worktree pool; tests pin it directly.
type PoolSizer interface {
	IdleWorktreeCount() (int, error)
}

// Consumer is the GitHub queue-drain consumer core. DrainOnce is the single poll
// step: it reads the provider's issues, applies DecideQueueAction to each, and
// for eligible (open + Queue=Ready + not-parked) issues maps #N -> Tracking/Task-N
// and dispatches by drawing an IDLE pool worktree (Task-0016). It is pure orchestration
// over the Dispatcher + DecideQueueAction + PoolSizer seams (no Temporal, no GitHub
// writes), so it is fully unit-testable with a FAKE provider and a FAKE dispatcher.
type Consumer struct {
	repo       string
	provider   QueueProvider
	dispatcher Dispatcher
	sizer      PoolSizer
	// autoCloseEnabled gates the TEST-ONLY simulated-human auto-close: when true the
	// consumer closes the GitHub issue of any active dispatched task that announced
	// completion (current_gate == "closure"). Default false keeps it read-only.
	autoCloseEnabled bool
}

// NewConsumer builds a consumer for a provider repo.
func NewConsumer(repo string, provider QueueProvider, dispatcher Dispatcher, sizer PoolSizer) *Consumer {
	return &Consumer{repo: repo, provider: provider, dispatcher: dispatcher, sizer: sizer}
}

// SetAutoCloseEnabled toggles the TEST-ONLY simulated-human auto-close on the
// consumer. It is set from the OBSIDIAN_AUTO_CLOSE_QUEUED config flag at wiring time.
func (c *Consumer) SetAutoCloseEnabled(enabled bool) {
	c.autoCloseEnabled = enabled
}

// DrainResult summarizes one DrainOnce poll for logging/proof.
type DrainResult struct {
	Dispatched []string
	Parked     []string
	Reclaimed  []string
	Skipped    []string
	// AutoClosed lists the tasks whose GitHub issue the TEST-ONLY auto-close closed
	// this poll (an active task that announced completion via current_gate=="closure").
	// Empty whenever the auto-close flag is off.
	AutoClosed []string
}

// IssueNumberFromTaskID parses a Tracking task id ("Task-0008") back to its GitHub
// issue number (8), the inverse of TaskIDForIssue. It errors on a malformed id so a
// close is never attempted against an unparseable issue number.
func IssueNumberFromTaskID(taskID string) (int, error) {
	suffix := strings.TrimPrefix(taskID, "Task-")
	if suffix == taskID || suffix == "" {
		return 0, fmt.Errorf("malformed task id %q: want Task-<number>", taskID)
	}
	number, err := strconv.Atoi(suffix)
	if err != nil {
		return 0, fmt.Errorf("malformed task id %q: %w", taskID, err)
	}
	return number, nil
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
//   - ActionDispatch: only if an IDLE pool worktree remains to draw (Task-0016) AND the
//     task does not already own a lane. Skip Queue=Never.
//   - ActionPark (Human Needed=Yes): record the parked state in place; never
//     redispatch; never reclaim.
//   - ActionTerminal (closed): reclaim the worktree + free the slot (only if the
//     task still owns a lane).
//   - ActionNone: skip.
//
// The idle-worktree budget is read once at the start of the cycle and decremented as
// the consumer dispatches (each dispatch draws one idle worktree), so it never
// over-admits past the available idle pool within a single poll; an empty pool defers
// the Ready issue (it is re-picked once an Eject/close frees a worktree).
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
	idleAvailable, err := c.sizer.IdleWorktreeCount()
	if err != nil {
		return result, fmt.Errorf("count idle pool worktrees for %s: %w", c.repo, err)
	}

	// TEST-ONLY simulated-human auto-close (OBSIDIAN_AUTO_CLOSE_QUEUED): for each
	// active dispatched task that ANNOUNCED completion (current_gate == "closure"),
	// close its GitHub issue exactly as a human would. This is the ONLY GitHub-write
	// and never fires with the flag off. We deliberately do NOT reclaim here: the
	// next poll observes the now-closed issue and reclaims via the existing
	// ActionTerminal path (no second reclaim path). Errors are non-fatal (log +
	// continue) so one bad task never wedges the poll.
	if c.autoCloseEnabled {
		for _, taskID := range activeTasks {
			requested, err := c.dispatcher.ClosureRequested(taskID)
			if err != nil {
				log.Printf("queue-drain auto-close: closure check for %s failed: %v", taskID, err)
				continue
			}
			if !requested {
				continue
			}
			num, err := IssueNumberFromTaskID(taskID)
			if err != nil {
				log.Printf("queue-drain auto-close: skip %s: %v", taskID, err)
				continue
			}
			if err := c.provider.CloseIssue(c.repo, num); err != nil {
				log.Printf("queue-drain auto-close: close issue #%d (%s) on %s failed: %v", num, taskID, c.repo, err)
				continue
			}
			log.Printf("queue-drain auto-close: closed issue #%d (%s) on %s (simulated human closure approval); next poll reclaims its worktree", num, taskID, c.repo)
			result.AutoClosed = append(result.AutoClosed, taskID)
		}
	}

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
			// A terminal close frees the worktree back to the idle pool, so a later
			// Ready issue in the SAME poll can draw it.
			idleAvailable++
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
			if idleAvailable <= 0 {
				// Empty pool: re-picked on a later poll once an Eject/close frees an
				// idle worktree (no auto-create — capacity is operator-owned).
				result.Skipped = append(result.Skipped, taskID)
				continue
			}
			if err := c.dispatcher.Dispatch(ctx, taskID); err != nil {
				return result, fmt.Errorf("dispatch %s: %w", taskID, err)
			}
			active[taskID] = true
			// This dispatch drew one idle worktree from the pool.
			idleAvailable--
			result.Dispatched = append(result.Dispatched, taskID)
		default:
			result.Skipped = append(result.Skipped, taskID)
		}
	}
	return result, nil
}
