<!-- task-sync: repo=CodexDashboard; task_id=Task-0009; task_path=Tracking/Task-0009/TASK.md -->

# Task-0009: Design and build the dashboard `Tasks` tab as the primary human surface for committed task dispatch and monitoring.

## Source Of Truth

Local `Tracking/Task-0009/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0009:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

## Summary

CodexDashboard needs a high-level committed-work surface that tells a human, at a glance:

- what matters now
- what is running
- what is stuck
- what is waiting on the human
- which real tasks were recently authored or promoted
- what can safely be dispatched next

The current dashboard has useful but fragmented surfaces:

- the overlay explains token burn
- the `Jobs` tab explains backend job state

What is missing is the product heart for committed work:

- a humane task-dispatch and task-monitoring surface

This task owns that surface.

The intended `Tasks` tab is not a raw database browser, not a transcript explorer, and not the canonical intake surface for new asks.
That intake surface belongs to [Task-0011](../Task-0011/TASK.md).

It is a trustworthy command surface for long-running committed work:

- queue review
- dispatch intent
- active run monitoring
- stuck-run recovery
- task readiness
- task provenance
- task-detail drilldown
- thread launch for deeper work when needed

The tab should make the repo feel more like one system with durable memory and supervision, and less like a loose pile of tasks, session transcripts, job specs, and review artifacts.

When the tab surfaces work promoted out of `Review`, it must show durable provenance from the single promotion contract owned by [Task-0010](../Task-0010/TASK.md) and the intake split owned by [Task-0011](../Task-0011/TASK.md).

This task does not define promotion semantics or intake review.

## Goals

- Make `Tasks` the high-level, always-useful home for task dispatch and task monitoring in CodexDashboard.
- Give the human one place to answer:
  - what should happen next
  - what is actively happening now
  - where intervention is needed
  - whether a task is safe to dispatch
- Keep the surface humane:
  - low interpretation cost
  - obvious next actions
  - calm failure handling
  - minimal hidden state
- Show the relationship between:
  - repo tasks
  - active agent work
  - durable execution state
  - promoted-task provenance
- Support fast task triage without forcing the human to read raw markdown first.
- Let the human click into a task and open the deeper working context when needed.
- Keep the first version legible on desktop without turning the app into a sprawling multi-pane IDE clone.
- Ground the surface in [GENERAL-DESIGNER.md](../../../../Users/gregs/.codex/Orchestration/Prompts/GENERAL-DESIGNER.md) and [INTERFACE-DESIGNER.md](../../../../Users/gregs/.codex/Orchestration/Prompts/INTERFACE-DESIGNER.md), not only in backend convenience.
- Produce a durable task-local design brief, a reusable [Stitch prompt](./Design/STITCH-PROMPT.md), and a generated [Stitch mockup](./Mockup/stitch_task_tab/screen.png) so the UI direction can be iterated visually before or during implementation.

## Acceptance Criteria

- CodexDashboard renders a real `Tasks` tab that is selectable from the main dashboard shell.
- The `Tasks` tab shows a top summary strip for:
  - needs attention
  - sleeping
  - running
  - ready
  - blocked
- The `Tasks` tab shows a grouped task stream and does not collapse all work into one flat table.
- Selecting a task updates a persistent detail pane that includes:
  - summary
  - current state
  - next expected step
  - artifact links
  - bounded actions
- The surface can honestly represent at least these states:
  - loading
  - populated
  - stale
  - backend unavailable
  - empty-but-healthy
- The surface distinguishes authored, promoted, and actively running tasks visually and semantically without showing still-pending review items as committed work.
- The surface can link back to upstream review provenance for promoted tasks without redefining the promotion flow owned by [Task-0010](../Task-0010/TASK.md).
- The surface exposes `Dispatch`, `Poke`, `Interrupt`, and deep-context launch only when the backing contract from [Task-0008](../Task-0008/TASK.md) says those actions are valid.
- A real UI proof bundle exists showing the populated screen, an attention state, and committed-task provenance.
- Repo regression proof exists for the real dashboard surface after the tab lands.

## Non-Goals

- Building the dispatch runtime itself in this task.
- Defining Temporal orchestration internals here instead of in [Task-0008](../Task-0008/TASK.md).
- Implementing the daily Dream job or digest generation here instead of in [Task-0010](../Task-0010/TASK.md).
- Owning the canonical intake and review surface for new asks; that belongs in [Task-0011](../Task-0011/TASK.md).
- Replacing task-owned markdown artifacts with only UI state.
- Turning the `Tasks` tab into a raw transcript browser, log tail, or arbitrary file explorer.
- Solving every future task-flow need before the first humane high-level surface exists.
- Using task dispatch as a pretext to leak raw backend or agent jargon into the default UI.

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `9`
- Local task path: `Tracking/Task-0009/TASK.md`
- Source commit: `75177b6dee23399358ee66676791fb41dc01d51e`
- Local task SHA-256: `E6A380E136D885B76A71C6198B43C90016A922A731CB2FEDB8F603744A75C76E`
- Rendered at: `2026-05-28T17:20:42.6805796-04:00`