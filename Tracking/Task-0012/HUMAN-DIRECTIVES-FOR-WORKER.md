# Human Directives For TaskCreate Writer

Captured for Task-0012 from the current TaskCreate conversation.

## Directive

The desired rollout sequence is:

1. Push a single `TASK.md`.
2. Inspect the resulting GitHub-backed representation.
3. Agree that it meets the task quality bar.
4. Push all `TASK.md` files across all configured repos.
5. Wire in the `drain my tasks pls` workflow where a central coordinator allocates a worktree and dispatches work.

## Interpretation Constraint

Do not draft the first task as full queue draining or full autonomous dispatch.

The first task should create the smallest honest implementation slice that lets the human test one `TASK.md` publication to GitHub and decide whether the representation is good enough before bulk publication.

## Important Future Direction

The later workflow should support:

- central coordinator queue draining
- configured concurrency limit for active tasks
- worktree allocation and release
- cross-repo GitHub issue querying

Those future goals should remain visible as follow-on context, but they should not make the first task too broad.
