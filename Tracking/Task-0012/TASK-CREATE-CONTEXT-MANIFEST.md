# TaskCreate Context Manifest

This manifest lists worker-safe context for drafting `Tracking/Task-0012/TASK.md`.

## Shared Standards

- `C:\Users\gregs\.codex\AGENTS.md`
- `C:\Users\gregs\.codex\Orchestration\Processes\TASK-CREATE.md`
- `C:\Users\gregs\.codex\Orchestration\Processes\TASK-AUDIT.md`
- `C:\Users\gregs\.codex\Orchestration\Processes\TaskCreate\README.md`
- `C:\Users\gregs\.codex\Orchestration\Processes\TaskCreate\TASK-CREATE-WRITER-WORKER.md`
- `C:\Users\gregs\.codex\Orchestration\Processes\TaskCreate\TASK-CREATE-AUDITOR-WORKER.md`
- `C:\Users\gregs\.codex\Orchestration\TASK-STATE.md`
- `C:\Users\gregs\.codex\Orchestration\WORKTREES.md`

## Repo Standards And Existing Design

- `C:\Agent\CodexDashboard\AGENTS.md`
- `C:\Agent\CodexDashboard\DATA-HANDLING.md`
- `C:\Agent\CodexDashboard\backend\orchestration\README.md`
- `C:\Agent\CodexDashboard\Tracking\Task-0008\TASK.md`
- `C:\Agent\CodexDashboard\Tracking\Task-0008\PLAN.md`
- `C:\Agent\CodexDashboard\Tracking\Task-0008\Design\DURABLE-EXECUTION-STATE-CONTRACT.md`
- `C:\Agent\CodexDashboard\Tracking\Task-0009\TASK.md`

## Implementation Anchors

- `C:\Agent\CodexDashboard\backend\orchestration\internal\config\config.go`
- `C:\Agent\CodexDashboard\backend\orchestration\internal\httpapi\mux.go`
- `C:\Agent\CodexDashboard\backend\orchestration\internal\taskrun\types.go`
- `C:\Agent\CodexDashboard\backend\orchestration\internal\taskrun\service.go`

## Task-0012 Context

- `C:\Agent\CodexDashboard\Tracking\Task-0012\TASK-CREATE-OBJECTIVE.md`
- `C:\Agent\CodexDashboard\Tracking\Task-0012\HUMAN-DIRECTIVES-FOR-WORKER.md`
- `C:\Agent\CodexDashboard\Tracking\Task-0012\Research\2026-05-27-GITHUB-ISSUES-AS-CROSS-REPO-TASK-STATE.md`

## Local CLI Facts

The coordinator checked:

- `gh version 2.88.1 (2026-03-12)` is installed.
- `gh issue list`, `gh search issues`, `gh issue view`, `gh issue create`, `gh issue edit`, and `gh api` are available.
- `gh auth status -h github.com` reports no logged-in GitHub hosts.

## Exclusions

Do not rely on hidden chat context beyond the durable human directives and research artifact listed above.

Do not treat this manifest as authorization to edit shared `.codex` docs, product code, or Task-0008/Task-0009 artifacts. The writer's required output is `Tracking/Task-0012/TASK.md`.
