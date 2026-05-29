<!-- task-sync: repo=CodexDashboard; task_id=Task-0006; task_path=Tracking/Task-0006/TASK.md -->

# Task-0006: Capture human-facing orchestration incidents as durable evidence.

## Source Of Truth

Local `Tracking/Task-0006/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0006:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

## Summary

The higher-level orchestration goal is to catch dropped balls, misunderstandings, and process bumps before they have to be escalated manually.

This task is intentionally narrower than that larger goal. It is only about capturing the moments where the human had to step in and disagree with an AI-produced or AI-directed outcome in order to protect the intended human-facing result.

Longer-horizon, this capture work is meant to support a future counterpoint agent that can say some version of `I think the human will want X` as a check against the producer, with the goal of reducing unnecessary human escalation. This task does not build that agent yet; it builds the evidence base honestly enough that such an agent could later be grounded in real human interventions and real human-interest statements.

That future agent is not meant to be a default veto-heavy critic. The intended default stance is advisory:

- warn when a produced outcome appears to violate a protected human interest
- explain the likely conflict concretely
- let the producer decide in most cases
- reserve stronger intervention or escalation for later workflow rules and higher-confidence conflicts

For this task, an `incident` means:

- a divergence from the intended human-facing outcome
- where an AI-produced or AI-directed outcome existed before the intervention
- where the human explicitly intervened to reject, correct, or redirect that outcome
- and where that intervention should become durable evidence instead of disappearing into chat history

The seed research for this task shows that these divergences are not all the same. Some are perceptual or interface-read failures, some are human-world or dignity failures, some are taste failures, and some come from orchestration or prompt gaps that let the miss through. This task should capture the incident first so later work can learn from it honestly.

## Goals

- Define a durable incident contract for human course correction incidents.
- Preserve the human's correction path, not just the final corrected artifact.
- Preserve the concrete pre-correction state, including active course when the incident is about drift rather than just a static artifact snapshot.
- Capture the human-facing expected state, the actual state, and why the gap mattered.
- Trace each incident upward through one or more ordered `why_chains`, where each chain is internally linear and each later entry answers why the prior entry mattered to the human.
- Distinguish the main incident layers when useful, such as:
  - perception or interface-read failure
  - human-world or dignity failure
  - taste or stylistic mismatch
  - orchestration, prompt, or workflow failure
- Make incidents concrete enough that later prompts, skills, evals, or training work can learn from them.
- Keep incidents tied to tasks, artifacts, screenshots, prompts, or concrete outputs rather than vague complaints.
- Separate `fix the artifact now` from `encode the durable process or prompt repair for next time`.
- Allow a quick first-pass incident capture, followed by a second-pass root-cause refinement once the underlying mechanism is better grounded.
- Make accepted incident JSONs heavyweight enough to carry their own verbatim human timeline and transcript context.
- Preserve enough surrounding daily-message context that later work can learn not only from explicit corrections, but also from non-corrective statements that reveal durable human interests, human limits, or human-world constraints.
- Preserve enough evidence that later work can distinguish recurring protected-interest families such as:
  - state-story truth
  - real-world done
  - human-facing form factor
  - control-boundary ownership

## Acceptance Criteria

- `Task-0006` defines `incident` explicitly as a human-corrected divergence from the intended human-facing outcome.
- The task scope is clearly limited to capture and classification, not full autonomous resolution.
- The minimum durable incident record shape is defined, including:
  - event-level expected state
  - event-level actual state
  - grounded pre-correction state, including active course when relevant
  - human intervention summary and evidence
  - human cost or reason the divergence mattered
  - layer or classification
  - `why_chains` with linear entries that progressively generalize the target state being protected
  - schema-constrained principle categories for those entries
  - evidence references
- The task owns a concrete task-local incident schema and at least two validated example incidents.
- The task definition distinguishes incidents from:
  - ordinary code bugs
  - pass audits
  - regression runs
  - taste-library entries
  - later prompt or training assets
- The task framing preserves the concern families surfaced by the seed research and the named review prompts:
  - perceptual or interface-read failures
  - human-world and dignity failures
  - taste failures
  - orchestration or workflow failures
- The task framing requires incidents to preserve one or more progressively generalized `why_chains` rather than only a symptom description, and each chain must stop before unsupported inference.
- The task framing makes daily capture review explicit:
  - transcript-first intervention-pass artifacts may exist per day while a day is under review
  - one day folder per incident date
  - accepted incident instances stored in the reports folder matching their `source_date`
- The task framing also distinguishes:
  - strict incidents, which require explicit human correction
  - broader intervention or human-model evidence, which may stay in task-local pass artifacts even when it is not promoted into the accepted incident set
- The task framing also distinguishes first-pass incident capture from later root-cause refinement, so incident opening does not wait on perfect diagnosis.
- The implementation home makes the shared-versus-repo-local split explicit.
- The next honest phase after task creation is research on incident shape, storage, and workflow fit rather than immediate implementation guesswork.

## Non-Goals

- Building the full autonomous critic stack, reviewer swarm, or "Jarvis" loop in this task.
- Solving every captured incident automatically.
- Replacing bug tracking for defects that do not involve explicit human disagreement over an AI-produced or AI-directed outcome.
- Turning this task into a general taste-library or model-training task.
- Treating every user preference or local polish comment as an incident.
- Building the counterpoint agent itself in this task.

That said, do not flatten screenshot-visible mockup distortions into `mere polish` when the human is clearly correcting product-read quality such as typography, spacing, icon fidelity, hierarchy, clipping, or similar human-facing semantic distortion.

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `6`
- Local task path: `Tracking/Task-0006/TASK.md`
- Source commit: `75177b6dee23399358ee66676791fb41dc01d51e`
- Local task SHA-256: `2A9AAB0F86BCFD8959BC49DCBE08BC79AEA54E650AEA6A95C7415AC90D2BEE6E`
- Rendered at: `2026-05-28T23:29:39.5955110-04:00`