<!-- task-sync: repo=CodexDashboard; task_id=Task-0011; task_path=Tracking/Task-0011/TASK.md -->

# Task-0011: Design and build the dashboard `Review` tab as the canonical intake surface for incoming asks.

## Source Of Truth

Local `Tracking/Task-0011/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0011:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

## Summary

CodexDashboard now has two different human jobs to support:

- supervise committed work
- review incoming asks before they become committed work

`Task-0009` owns the first job through the `Tasks` tab.

This task owns the second job through a new `Review` tab.

The review job is not narrow Dream-email triage.

It includes multiple ask pipelines that want explicit human judgment:

- Dream-generated option tasks
- interface-review findings
- QA or adversarial bug-finding batches
- general-design or first-principles product asks
- approval requests
- runtime anomalies that need acknowledgement, routing, or intervention

Email can still notify the human that those asks exist.

Email should not be the canonical place where the human manages them.

The intended `Review` tab is not a raw inbox and not a second `Tasks` tab.

It is a trustworthy intake surface that lets the human answer:

- what came in
- why it arrived
- what evidence supports it
- what action is being asked for
- what happens if I promote, approve, route, defer, or dismiss it

The surface must keep provisional asks separate from committed work while still making the next step cheap and explicit.

Any `Promote to Task` action exposed on this surface must be a client of the single backend promotion contract owned by [Task-0010](../Task-0010/TASK.md).

This task does not define a second promotion mechanism.

## Goals

- Make `Review` the canonical human-facing intake surface for incoming asks in CodexDashboard.
- Keep `Review` and `Tasks` semantically separate:
  - `Review` for provisional asks
  - `Tasks` for committed work
- Support multiple ask sources without pretending they are all the same kind of item.
- Preserve provenance so the human can see where an ask came from and why it exists.
- Expose bounded dispositions that make the next action explicit.
- Keep email as notification and digest, not as the canonical state home.
- Produce task-local design and research artifacts strong enough to guide real implementation.

## Acceptance Criteria

- CodexDashboard renders a real `Review` tab that is selectable from the main dashboard shell.
- The `Review` tab shows a top summary strip for high-signal incoming-ask states.
- The `Review` tab shows a grouped incoming-ask stream and does not collapse all review material into one flat inbox.
- Selecting an ask updates a persistent detail pane that includes:
  - summary
  - source
  - requested action
  - provenance
  - bounded actions
- The surface can honestly represent at least these states:
  - empty
  - loading
  - populated
  - stale
  - backend unavailable
- The surface distinguishes incoming asks from committed work visually and semantically.
- `Promote to Task` appears only for review items that are true task candidates and only through the single promotion mechanism owned by [Task-0010](../Task-0010/TASK.md).
- The surface exposes other dispositions only when the relevant backing contract exists and the human-visible consequence is explicit.
- A real UI proof bundle exists showing mixed incoming asks, a high-attention review state, and at least one candidate-task detail view.
- Repo regression proof exists for the real dashboard surface after the tab lands.

## Non-Goals

- Replacing the `Tasks` tab as the home for committed work.
- Defining the Dream backend promotion contract here instead of in [Task-0010](../Task-0010/TASK.md).
- Defining dispatch-state semantics or runtime interruption rules here instead of in [Task-0008](../Task-0008/TASK.md).
- Building every future pipeline producer before the first humane intake surface exists.
- Turning `Review` into a generic inbox, BI dashboard, or transcript browser.
- Auto-promoting all candidate work into tasks.

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `11`
- Local task path: `Tracking/Task-0011/TASK.md`
- Source commit: `75177b6dee23399358ee66676791fb41dc01d51e`
- Local task SHA-256: `9F29119EA4E92C6BEA859C1741B836D9E8DD5453EF189973E3D4910C8D06FEDE`
- Rendered at: `2026-05-28T23:29:44.0286382-04:00`