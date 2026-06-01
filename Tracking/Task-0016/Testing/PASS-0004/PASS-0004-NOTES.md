# PASS-0004 — Provider dequeue write + standalone dequeue endpoint

Backend pass 5 of the Task-0016 backend batch (single-context implementation).

## Objective (TASK.md §10, §11; Goals 10, 11; AC12, AC14, AC15)

The **first task-provider queue WRITE**: set the issue's `Queue` single-select to `Never`
**through the `QueueProvider` abstraction** (symmetric to the `Queue` read and to
`CloseIssue`), idempotent, **never closes the issue**. Exposed standalone so it can run
without ejecting.

## Changes

- [provider.go](../../../../backend/orchestration/internal/queue/provider.go):
  - Added `DequeueIssue(repo, number) error` to the `QueueProvider` interface (the
    symmetric sibling of the `Queue` read and the existing `CloseIssue` write), documented
    as: sets `Queue=Never`, **never** closes, idempotent.
  - Implemented on `ghQueueProvider`: `queueField()` resolves the org `Queue` single-select
    field id (and verifies the `Never` option exists, reusing the
    [`QueueNever`](../../../../backend/orchestration/internal/queue/decision.go) constant);
    `DequeueIssue` POSTs `{issue_field_values:[{field_id, value:"Never"}]}` to
    `/repos/<repo>/issues/<n>/issue-field-values` — the **same** write the
    obsidian-operator sync performs — through the **injectable `run`** func via `--input`,
    so a test never touches real GitHub. Extended `ghIssueField` to decode `data_type` +
    `options`.
- [pool.go](../../../../backend/orchestration/internal/taskrun/pool.go) +
  [service.go](../../../../backend/orchestration/internal/taskrun/service.go):
  - New `DequeueProvider` seam on the `Service` (`SetDequeueProvider`); a nil provider
    makes dequeue a safe no-op.
  - `Service.DequeueTask(repo, taskID)` resolves the issue number via
    [`IssueNumberFromTaskID`](../../../../backend/orchestration/internal/queue/consumer.go)
    and calls the provider dequeue **without** ejecting/cleaning/unbinding; a task with no
    parseable issue number (or no provider) is a safe no-op; it never closes the issue.
- [queuedrain.go](../../../../backend/orchestration/internal/temporalbackend/queuedrain.go):
  the per-repo Service is given the write capability via `service.SetDequeueProvider(provider)`
  — the **same** gh provider that polls `Queue=Ready` performs the `Queue=Never` write, so
  the dequeue goes **through the provider**, never an inline gh call.
- [mux.go](../../../../backend/orchestration/internal/httpapi/mux.go): `POST
  /api/v1/worktrees/dequeue {repo, task_id}` on the method/path-guarded worktree sub-router
  (200; does not close).

## Proof — Go unit tests (no real GitHub)

- [provider_test.go](../../../../backend/orchestration/internal/queue/provider_test.go)
  `TestGitHubQueueProviderDequeueSetsQueueNeverViaProvider` (AC12/AC14): with a fake `run`,
  `DequeueIssue` resolves the `Queue` field id and POSTs the issue-field-value with
  `value:"Never"` (body read back from the `--input` temp file), and the fake **fails the
  test if `issue close` / PATCH / DELETE is ever called** — proving it never closes and is
  a provider write, not an inline gh call.
- [pool_test.go](../../../../backend/orchestration/internal/taskrun/pool_test.go):
  `TestDequeueTaskCallsProviderWithIssueNumber` (AC12 — resolves `Task-0007` → issue #7,
  correct repo, through the provider), and the two safe-no-op cases (unparseable task id;
  no provider).
- [worktrees_pool_test.go](../../../../backend/orchestration/internal/httpapi/worktrees_pool_test.go):
  `TestWorktreeDequeueEndpointInvokesProviderWithoutClosing` (AC15 — 200, invokes the
  injected provider for repo/issue #7, no eject), and the dequeue route method guard (405).

### Falsifier guards covered
- A dequeue implemented as an inline hardcoded `gh` call instead of on the provider would
  not satisfy the provider-write test (the Service calls `dequeueProvider.DequeueIssue`,
  the gh provider owns the write) — AC12.
- A dequeue that **closes** the issue fails the provider test (the fake fatals on
  `issue close`) — AC14.

### Commands

```
cd C:\Agent\CodexDashboard\backend\orchestration
gofmt -l internal/   # (no output — clean)
go build ./...        # exit 0
go test ./...         # all ok
```

Producer testing; independent clean-context QA is coordinator-arranged after the backend
batch. The Eject coupling (Eject calls this dequeue) and the consumer+service
no-bounce-back seam are PASS-0005.
