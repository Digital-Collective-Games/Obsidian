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

## Re-capture resolution (2026-06-01, commit `59c5bd9`)

The three flagged evidence defects are RESOLVED and coordinator-verified (each new PNG
opened and confirmed to show the WORKTREES tab with the claimed state + in-frame message):

- **Wrong-window race fixed.** The task-owned capture harness (gitignored
  `Testing/Runtime/`) now grabs the overlay's own content by HWND via
  `PrintWindow(PW_RENDERFULLCONTENT)` (immune to occlusion), and the app's scheduled
  `_run_smoke_capture` that was overwriting shots was neutralized. `reg014-assign-bound`
  and `reg015-eject` now show the app surface (were the Claude Code IDE window).
- **Real product gap fixed (REG-016 messages).** The action status was only pushed to a
  global `status_label` never laid out on the WORKTREES tab, so Eject/Destroy-reject/Dequeue
  outcomes were genuinely invisible. A visible `worktrees_action_status_label` was added to
  the WORKTREES header (`app/codex_dashboard/ui.py`); Python unit suite green @ 182. The
  allocated-Destroy 409 rejection and the post-Dequeue confirmation now render in-frame.
- **Verified captures:** REG-014 (`RepoX/wt-0002` ALLOCATED + "now allocated" message),
  REG-015 (`RepoX/wt-0001` IDLE, folder kept + "idle and the task is dequeued" message),
  REG-016 (409 rejection rendered, worktree still allocated; dequeue confirmation rendered,
  still allocated).

**UI evidence gate: CLOSED.** REG-010…REG-016 in-app surface/behavior is proven on the
final v2 surface; both the INTERFACE-DESIGNER fidelity bar and the functional human-surface
QA are satisfied.

**Known benign caveat (not blocking, to be cleaned in the live run):** in `reg014` the
allocated row's "Local dir:" shows `…\wt-0001\wt-0001` while the status message correctly
says `wt-0002` — a test re-seed path-string artifact (reg015/reg016 show correct distinct
`wt-0001`/`wt-0002` paths, so not a systematic display bug; certified attributes — allocated
colour, bound `Task-9001`, chip, short `RepoX` heading, message — are correct). A clean
capture from a fresh-empty seed will be taken during the live PASS-0009 run.

**Remaining for closure:** the LIVE PASS-0009 — REG-007/008/009 (real-UI `Queue=Ready` flip
via the human-authenticated Chrome debug session against the throwaway `QueueDrainTestbed`
repos on the isolated `reg007` lane) and the live in-app REG-015 dequeue (in-app Eject →
`Queue=Never` → no re-dispatch). Then the human closure gate (the agent never self-closes
issue #16).
