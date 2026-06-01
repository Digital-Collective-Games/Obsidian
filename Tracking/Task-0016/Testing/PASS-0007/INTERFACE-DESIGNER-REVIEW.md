# Task-0016 — INTERFACE-DESIGNER review of the WORKTREES tab

- **Reviewer:** clean-context, read-only interface-designer subagent (per
  `.codex-pushable-temp/Orchestration/Prompts/INTERFACE-DESIGNER.md`)
- **Date:** 2026-06-01
- **Surface:** desktop `WORKTREES` tab (Task-0016), vs the approved Stitch "Monolithic
  Terminal" mockup (right-hand worktree cards; E2–E7 exclusions honored)
- **Bar:** TASK.md [FE] AC16–22; HUMAN-DIRECTIVES UPDATE 5 (visual fidelity + info
  hierarchy); REGRESSION REG-010
- **Authoritative final-layout artifact:** [`reg010-pool-view-v2/overlay.png`](./reg010-pool-view-v2/overlay.png)
  + [`reg010-details-reveal/overlay.png`](./reg010-details-reveal/overlay.png)
  + [`reg010-backend-unavailable/overlay.png`](./reg010-backend-unavailable/overlay.png)

## Verdict

**Substantially achieves the concept + the info-hierarchy directive** (card form,
same-family tonal allocated/idle distinction, glanceable face, real DETAILS/hover reveal,
honest empty/unavailable state). **Not yet closable on visual fidelity** due to B1 + B2,
plus the stale-evidence gap B3.

## Blocking (must fix before closure)

- **B1 — allocated heading shows a full filesystem path, not the short repo token.** The
  heading binds to `worktree.get("repo")` ([ui.py L1306-1314](../../../backend/orchestration/../../app/codex_dashboard/ui.py#L1306)),
  which for an allocated worktree is the bound checkout path → overflows/clips in the
  Space-Grotesk display slot, and reads as a different "kind" of heading than idle panels.
  **Fix (one line):** use the existing
  [`worktree_repo_segment()`](../../../app/codex_dashboard/worktrees_tab.py#L54) (already
  written for this exact divergence) for the heading; the full bound path stays in DETAILS.
- **B2 — status chip is text-only (no state mark).** UPDATE 5 requires "status via a chip
  (not text-only)." Chip at [ui.py L1296-1305](../../../app/codex_dashboard/ui.py#L1296).
  **Fix:** add a small leading state mark (filled square/dot for allocated, hollow for
  idle) within the existing `WORKTREE_STATUS_COLORS` cyan/green family.
- **B3 — re-capture reg013–reg016 on the refined v2 panel.** The PASS-0008 functional
  captures are the pre-UPDATE-5 v1 layout (full path + ids crammed on-face, no DETAILS).
  They prove the interactions but not the shipped panel; the fidelity gate and the
  functional gate must agree on the same surface. (Evidence/process gap, not a code defect.)

## Non-blocking (polish)

- **N1** — copy/Details/Eject are text-only ttk buttons (no icon family). All-caps text is
  an acceptable Tk substitute (Q1=a); a leading copy glyph on COPY PATH would strengthen the
  one affordance UPDATE 5 calls out. Optional.
- **Empty-pool wording** — reachable-backend/zero-worktrees state shows a generic refresh
  line; should say "No worktrees in this repo yet — use CREATE WORKTREE to add one."
- N2 accent rail is in-system (boundary via tonal fill, not a divider) — acceptable. N3
  optional error-tint on EJECT.

## On-face vs reveal — correct

On-face: status chip, repo heading, bound task (allocated), shortened path + copy, action
buttons. Behind DETAILS ([`worktree_detail_lines`](../../../app/codex_dashboard/worktrees_tab.py#L171))
+ path mouseover: full path, worktree id, run id, run gate, agent session, transcript, pid
— truthfully omitted when empty (no fabricated agent-model chip; E4 honored). Matches
UPDATE 5.

## Process gaps (for the coordinator / .codex)

1. REG-010 says "shows the repo" but does not forbid a full path in the heading slot → B1
   passed functional QA. REG-010's in-app assertion should require the heading to be the
   short repo id in BOTH states.
2. The fidelity gate (UPDATE 5) and the functional gate (PASS-0008) were captured on
   different layout versions. Once a visual-fidelity directive lands, all human-surface
   regression captures should be re-taken on the post-directive layout before either gate is
   called green.
