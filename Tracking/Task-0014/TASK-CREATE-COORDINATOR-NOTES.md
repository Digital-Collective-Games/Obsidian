# Task-0014 TaskCreate Coordinator Notes

## Process model

TaskCreate as of 2026-05-29: coordinator + blind writer-worker, with a
**coordinator review** in place of a separate blind agent-auditor lane. The
coordinator is the keeper of common sense and human intention and reviews the
draft to **increase concreteness without narrowing scope**. See
`C:\Users\gregs\.codex\Orchestration\Processes\TaskCreate\README.md`.

## Grounding / recon

Before drafting, the coordinator ran a read-only recon fan-out over the repo to
ground the objective and context manifest in `file:line` evidence (tab structure,
current window geometry, the absence of any work-area/taskbar query plus the
reusable `hotkey.py` ctypes pattern, and the pinned-release deploy + test path).
That evidence is captured in `TASK-CREATE-OBJECTIVE.md` and
`TASK-CREATE-CONTEXT-MANIFEST.md`.

## Clarifying answers (load-bearing) surfaced to the human BEFORE drafting

Three ambiguities in the original request changed the acceptance bar and were
surfaced to the human, who answered authoritatively (recorded verbatim in
`HUMAN-DIRECTIVES-FOR-WORKER.md`):

1. Tall-mode width → keep current width (980px); only height/vertical position
   change.
2. Reconcile "95% height" + "50px padding" + "don't cover taskbar" → use the area
   above the taskbar as usable space; pad top and bottom by 5% of screen height
   (configurable); that defines the height and the implied y. This SUPERSEDES the
   literal "95%/50px."
3. Usage-tab size → keep current 980×660 size, but reposition by the same
   padding/usable-space rule (moved up). So "move up" is global; tall sizing is
   Jobs/Tasks-only.

## Writer draft

The blind writer-worker produced [TASK.md](./TASK.md) as a `concrete
implementation` task: a pure, unit-testable tab-aware geometry function + a
configurable `pad_fraction` config field + a ctypes `SPI_GETWORKAREA` work-area
query + a `select_tab()` hook. The math is pinned to the human's answers and every
acceptance criterion is pass/fail.

## Coordinator review — concreteness change made (scope preserved)

One concreteness gap pinned (the review's function — not a scope change):

1. **Degenerate-padding guard (ambiguous edge → pinned).** Because `pad_fraction`
   is configurable, a pathologically large value could make the tall height
   `usable_height − 2·pad` non-positive, leaving an implementer to guess. Pinned a
   safety floor: clamp the Jobs/Tasks height to the existing `620` minimum if the
   formula would fall below it. For any sane `pad_fraction` (incl. the `0.05`
   default) the canonical `usable_height − 2·pad` value applies and the bottom
   stays at `wa_bottom − pad`, so this does not weaken AC 5 (taskbar-never-covered)
   for normal/tested configurations. Edit applied directly to
   `## Proposed Changes` → step 3 (re-launching the writer would add no value).

Everything else in the draft already satisfied the concreteness/specificity lens
(`TASK-AUDIT.md`): named home, pinned mechanism, pass/fail acceptance, dedicated
falsifiers for the two human-flagged hard constraints (taskbar-never-covered,
padding-configurable), explicit `What Does Not Count`, and honest pinned-release
deployment caveat.

## Scope integrity

No part of the human-selected scope was split, narrowed, re-sequenced, or removed.
All four human decisions are preserved: width stays 980, Usage keeps its size,
Jobs/Tasks go tall, and "move up" is global across tabs. The padding fraction is
configurable as the human required.

## Provider-binding gate (SATISFIED — 2026-05-29)

Per `TASK-CREATE.md` Task-Provider Binding Gate and the `obsidian-operator` skill.
The human approved the draft and authorized the outward-facing GitHub write.
Verified the next issue number was actually 14 (highest issue was #13, zero PRs in
the repo → no numbering hole), then created issue-first via
`Bootstrap-TaskGitHubIssues.ps1 -StartTaskNumber 14 -EndTaskNumber 14` (the
script's safety stop for a number ≠ task number did not fire). Result:

- GitHub issue #14 created: https://github.com/Digital-Collective-Games/Obsidian/issues/14
- `Tracking/Task-0014/TASK-META.json` written after readback (issue_number 14).
- Reconcile dry-run: Task-0014 ↔ #14 `text_state: in_sync`,
  `remote_hash_matches_local: true`, `difference_count: 0`, no conflicts.
- Issue state OPEN (created, not executed). Issue fields default Queue=Never,
  Priority=P2, Human Needed=No. Artifacts under `Tracking/Task-0014/Testing/IssueBootstrap/`.

`TASK-STATE.json` is intentionally NOT created at task-creation time; it is owned
by the dispatch lifecycle (TaskDispatch), and the reconcile treats the empty local
status as a non-difference for a freshly created task.

## Status

HUMAN-APPROVED and provider-bound (issue #14). Task-0014 is created and
enqueue-ready. Under the 2026-05-29 TaskCreate model there is no separate
agent-auditor lane.
