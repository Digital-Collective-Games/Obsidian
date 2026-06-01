// Package queue holds the per-repo dispatch primitives for the Temporal-backed GitHub
// queue-drain consumer. Concurrency is bounded by the count of IDLE worktrees in each
// repo's manually-managed pool, by construction (Task-0016): the consumer draws an idle
// pool worktree per dispatch and an empty pool defers the Ready issue. The historical
// numeric queue_workers cap (EvaluateSlot / SlotDecision / DefaultQueueWorkers) was
// removed when the pool's idle count became the cap.
package queue
