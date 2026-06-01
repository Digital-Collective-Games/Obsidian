# Task-0016 PASS-0009 — UI surface review (INTERFACE-DESIGNER re-check + human-surface QA)

Two independent clean-context reviews of the FINAL WORKTREES-tab surface (commit
`4967cf9`, captures `e18be01`).

## INTERFACE-DESIGNER re-check — fidelity bar MET

- **B1 RESOLVED** — heading shows the short repo token (`RepoX`) in both allocated and
  idle states; full bound path only in the DETAILS reveal.
- **B2 RESOLVED** — chip carries a state mark (filled cyan square allocated / hollow green
  idle), cyan/green family, 0px-radius, no emoji.
- **B3 RESOLVED** — fidelity-bearing panels re-captured on the shipped v2 surface.
- **No new blocking fidelity discrepancies.** Only optional N1 (a leading copy glyph) remains.

## Human-surface QA — FAIL (evidence only, not product behavior)

| Case | Verdict | Note |
| --- | --- | --- |
| REG-010 (FE16/17) | **PASS** | WORKTREES tab (no TASKS), color distinction, short heading, chip mark, Details, honest backend-unavailable |
| REG-011 (FE18) | **PASS** | copy control + recorded `clipboard==backend_path` |
| REG-012 (FE19) | **PASS** | registry-sourced filter narrows; reachable-empty message shown |
| REG-013 (FE20) | **PASS** | Create adds idle `RepoX/wt-0003` to the view |
| REG-014 (FE21) | **FAIL** | post-Assign `overlay.png` is the wrong window (Claude Code IDE), not the app — "flips to allocated in the view" not visible |
| REG-015 (FE22 Eject) | **FAIL** | post-Eject `overlay.png` is the same wrong window — "returned to idle in the view" not visible (folder-kept recorded ok; live dequeue correctly deferred) |
| REG-016 (FE22 Destroy/Dequeue) | **PARTIAL** | destroy-idle PASS; allocated-Destroy **rejection message** and post-Dequeue **confirmation** are not rendered in their (real-app) screenshots, only in the text summary |

**Root cause (not a product defect):** a capture-harness wrong-window/timing race — the
same surface renders correctly in the passing cases; the text summaries record the correct
outcomes. QA cannot certify a surface absent from the image.

## Required re-captures (route-back) — on the same isolated throwaway lane, real WORKTREES-tab surface only

1. **REG-014** — post-Assign: the chosen worktree (`RepoX/wt-0002`) rendered **ALLOCATED**
   (allocated color + bound `Task-9001` + running chip) in the pool view.
2. **REG-015** — post-Eject: the previously-allocated worktree (`RepoX/wt-0001`) rendered
   **IDLE** (idle color, no bound task), row still present (folder kept).
3. **REG-016** — the allocated-Destroy **rejection message** rendered in the app, and the
   post-Dequeue **confirmation** rendered in the app (worktree still allocated).

Delete/replace the two contaminated wrong-window PNGs (`reg014-assign-bound/overlay.png`,
`reg015-eject/overlay.png`). REG-010/011/012/013 and REG-016 destroy-idle PASS — no rework.
The LIVE GitHub `Queue=Never` dequeue + no-re-dispatch (REG-015 consequence; REG-007/008/009)
remain a separate human-Chrome-session step, not part of this re-capture.
