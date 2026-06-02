# WORKTREE card — mockup conformance review

Redesigned the WORKTREES-tab worktree panels to conform to the Stitch "Monolithic
Terminal" mockup (`C:\Users\gregs\Downloads\stitch_codex_token_velocity_overlay (4)\screen.png`
+ `code.html`) after the human's live feedback. Rendered through the real `DashboardApp`
surface (PrintWindow capture, no backend) with a fixed preview pool: one live allocated
slot, one parked allocated slot (needs review), one idle slot.

![Redesigned worktree cards](./worktree-cards.png)

## What changed (per the human's directives)

- **Single horizontal row, no bottom button drawer** — actions are right-justified on the
  row, exactly like the mockup.
- **Accent stripe**: 2× wider (6px) and ~66% height, vertically centered — a status bar,
  not a full-height hairline.
- **Repo name FIRST, UPPERCASE** (`OBSIDIAN`), then the status chip.
- **Mockup color scheme**: allocated = cyan, idle = gray, needs-review (a parked run gate)
  = red; the red also drives the **EJECT** button (Assign stays the cyan accent).
- **Status chip** = a solid 0px-radius pill: cyan `ALLOCATED` / red `REVIEW` / gray `IDLE`.
- **Full local dir** shown in monospace (Courier New), **white**, with the **copy control
  inline at the end of the path** (drawn as Tk Canvas vector art — the mockup's
  `content_copy` glyph; no Material font / emoji).
- **Bound task id is a clickable link** to its running GitHub issue (e.g. `Task-0016` →
  `https://github.com/Digital-Collective-Games/Obsidian/issues/16`), on the heading line.
  **No agent-model chip** (the mockup's `smart_toy` "Claude-3.5-Sonnet" is excluded — E4).
- **Right-justified actions**: Details (ⓘ) + DEQUEUE + EJECT for allocated; Details + a
  Destroy (trash) icon + ASSIGN for idle. Each icon is borderless, accent-on-hover, with a
  hover tooltip naming the action.

## Verification

- `python -m py_compile` clean; full unit suite green (184 tests), including the updated
  status-label/color tests + new `worktree_issue_url` and needs-review (red) coverage.
- Preview harness: [preview_card.py](../Runtime/preview_card.py) (task-owned, throwaway;
  `Testing/Runtime/` is gitignored so the harness itself is not committed).

## Remaining gate

This is a visual redesign of the surface REG-010 covers; before closure it still needs a
fresh clean-context INTERFACE-DESIGNER review + a REG-010 re-capture against the live tab.
