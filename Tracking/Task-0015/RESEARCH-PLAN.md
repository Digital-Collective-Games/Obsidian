# Task-0015 Research Plan

Single-context note: this research phase was executed directly by the
TaskDispatch task leader because no nested research-leader dispatch tool was
available in this runtime. The same phase discipline and durable artifacts were
applied. The chosen solution shape is fixed by `TASK.md` (concrete
implementation task with ONE embedded research item); research here is scoped to
the unknowns that materially change pass planning, NOT to redesign the solution.

## Decision-Shaping Problems (bounded)

| # | Problem | Why it changes pass planning | Where answered |
| --- | --- | --- | --- |
| P1 | O5 liveness signal (the embedded KNOWN UNKNOWN): what observable signal distinguishes "working" from "asleep" for a headless agent? | The whole watchdog (detect/poke/email) and its parked-suspension depend on it; a fixed timer is a disqualified proxy (F-O5-signal). | [Research/LIVENESS-SIGNAL.md](./Research/LIVENESS-SIGNAL.md) |
| P2 | Where does the worktree↔session binding (O6) actually live today, and is it durable? | Determines whether O6 extends `RepoLane`/`bootstrapOwnedLane` (durable record) vs only the in-memory `DeepContext` on the run view. | [RESEARCH.md](./RESEARCH.md) §O6 |
| P3 | Is the backend truly hard 1:1, and exactly which lines enforce it? | O2 must relax the real gate, not a config field. | [RESEARCH.md](./RESEARCH.md) §O2 |
| P4 | Does the current execution model actually launch a top-level coding agent, or only run backend activities? | O5's "top-level agent that spawns subagents" is net-new vs reuse; changes the size/risk of the O5 pass. | [RESEARCH.md](./RESEARCH.md) §O5 |
| P5 | What is the existing GitHub issue-field + close/reopen surface, and how is the human-only-closure tightening layered on it without a second write path? | O4 must reuse the obsidian-operator surface and must NOT auto-close from local terminal status. | [RESEARCH.md](./RESEARCH.md) §O4 |
| P6 | Exact live-code references for the manifest rename, and which artifacts are durable history that must NOT be rewritten. | O1 falsifier F-O1 fails if a historical artifact is rewritten or a live ref still resolves to the old name. | [RESEARCH.md](./RESEARCH.md) §O1 |
| P7 | What is the isolated proof lane and its isolation rule? | Every sub-objective's proof must run on the validation lane / task-owned fixtures, never the human live config/DB or ungated live GitHub writes. | [RESEARCH.md](./RESEARCH.md) §Proof |

## Out Of Scope For Research

- Re-opening the solution shape (fixed by `TASK.md`).
- The Obsidian review-surface UI (sibling task / Non-Goal).
- Voluntary worktree release (deferred Non-Goal).
- Picking the exact Temporal object for the slot manager (left as an
  implementation choice by `TASK.md` Open Questions; constrained by acceptance,
  not by research).

## Exit Bar

- P1 answered in a durable artifact that names the chosen signal, sampling, and
  why it separates the two states (DONE — `LIVENESS-SIGNAL.md`).
- P2–P7 answered from real repo code with file:line evidence, enough to write a
  pass plan whose passes map 1:1 to O1–O6 with per-pass falsifiers (DONE —
  `RESEARCH.md`).
- No solution-shape redesign; no scope narrowing.
