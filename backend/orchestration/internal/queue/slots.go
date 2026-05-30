// Package queue holds the per-repo concurrency primitives for the Temporal-backed
// GitHub queue-drain consumer. O2 introduces real N>1 per-repo worktree slots: a
// repo may hold up to queue_workers concurrent owned lanes at once, replacing the
// backend's historical hard 1:1 dispatch gate.
package queue

// DefaultQueueWorkers is the per-repo concurrency used when a repo entry omits
// queue_workers. It mirrors the manifest default documented for REPO-MANIFEST.json.
const DefaultQueueWorkers = 4

// SlotDecision is the outcome of asking the slot manager whether a same-repo
// dispatch may proceed. A dispatch is admitted only while fewer than the repo's
// limit of owned lanes are currently active; otherwise it is refused so the
// consumer re-picks the Ready issue once a slot frees.
type SlotDecision struct {
	// Admit is true when a free slot remains for the repo.
	Admit bool
	// Limit is the repo's queue_workers (max concurrent owned lanes).
	Limit int
	// Used is the number of owned lanes currently active for the repo.
	Used int
	// Reason explains a refusal; empty when Admit is true.
	Reason string
}

// Available reports the number of free slots (never negative).
func (d SlotDecision) Available() int {
	free := d.Limit - d.Used
	if free < 0 {
		return 0
	}
	return free
}

// EvaluateSlot decides whether a same-repo dispatch may proceed given the repo's
// limit and the number of owned lanes already active for that repo. A non-positive
// limit is normalized to DefaultQueueWorkers so a misconfigured manifest never
// silently pins concurrency to zero.
func EvaluateSlot(limit int, used int) SlotDecision {
	if limit <= 0 {
		limit = DefaultQueueWorkers
	}
	if used < 0 {
		used = 0
	}
	decision := SlotDecision{Limit: limit, Used: used}
	if used >= limit {
		decision.Reason = "all per-repo worktree slots are occupied"
		return decision
	}
	decision.Admit = true
	return decision
}
