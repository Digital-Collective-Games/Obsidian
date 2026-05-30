# Human Directives For Codex — Task-0014

Captured for Task-0014 from the current Codex session on 2026-05-29.

These directives are authoritative scope for Task-0014. Human directives override
subagent, coordinator, and agent-auditor preferences.

## Verbatim Human Directive

> Run TaskCreate to capture a task that resizes the Obsidian window based on tab
> hit. Move the window up so its closer to top of the monitor, and for the Jobs
> and Tasks tab make it take 95% of the screen height, while leaving like 50 px
> padding around the edges, and don't cover the windows task bar.

## Clarifying Answers (same session, 2026-05-29) — AUTHORITATIVE

The coordinator surfaced three load-bearing ambiguities (they change the
acceptance bar). The human answered:

1. **Tall-mode width** — "Keep current width (980px)."
   - In the tall Jobs/Tasks layout, only the HEIGHT and vertical POSITION change.
     The width stays at today's default (980px) and the window stays
     right-aligned. Do NOT widen the window.

2. **Height vs padding (reconciling "95% height" + "50px padding" + "don't cover
   taskbar")** — "Take area above taskbar as usable space. Pad by 5% screenheight
   (configurable) on top and bottom. That's the height. And implied y coordinate."
   - This SUPERSEDES the literal "95% of screen height" and the literal "50 px
     padding." The canonical rule is:
     - Usable space = the monitor work area, i.e. the area ABOVE/around the
       taskbar (taskbar excluded).
     - `pad = 5% of the full screen height`, and this 5% MUST be configurable.
     - Vertical padding `pad` is applied at the top AND the bottom of the usable
       space.
     - The tall window HEIGHT = `usable_height − 2 × pad`.
     - The window TOP (`y`) = `work_area_top + pad` (this is the "implied y
       coordinate"; it satisfies "move the window up, closer to the top").
     - Because the bottom of the window = `work_area_bottom − pad`, a `pad`-sized
       gap always remains above the taskbar → the taskbar is NEVER covered.

3. **Usage tab size** — "Current height and width but position is implied by
   previously specified padding / useable space constraint."
   - The Usage (default) tab keeps its CURRENT size (today's 980×660 clamp). It is
     NOT grown to the tall height. But its POSITION follows the same usable-space
     / padding rule as Jobs/Tasks — i.e. it is moved up so its top `y =
     work_area_top + pad`, inside the usable area. So "move the window up" is a
     GLOBAL reposition that applies to every tab; the tall ~full-usable-height
     sizing is specific to the Jobs and Tasks tabs.

## Normalized Spec (worker-safe synthesis of the above)

- The overlay window geometry becomes a function of the ACTIVE TAB and is
  recomputed when the tab changes.
- All tabs: window is repositioned into the usable space (above the taskbar) with
  top `y = work_area_top + pad`, `pad = round(pad_fraction × screen_height)`,
  `pad_fraction` configurable (default 0.05). Horizontal: keep current width
  (980px), right-aligned within the usable width (do not widen, do not bleed off
  the usable area).
- `usage` (and the default/initial tab): keep current height (660 clamp) and
  width (980 clamp); only the vertical position changes (moved up per the rule).
- `jobs` and `tasks`: height = `usable_height − 2 × pad`; width unchanged (980);
  top `y = work_area_top + pad`. The window fills the usable height minus the
  top/bottom padding and never overlaps the taskbar.
- "Usable space" must come from the real Windows work area (taskbar excluded), not
  from `winfo_screenheight()`/`winfo_screenwidth()` (which include the taskbar).

## Auditor / Scope Notes

- This is ONE concrete-implementation task: tab-aware geometry for the Obsidian
  desktop overlay. Do not split, broaden, or narrow it on preference.
- Do NOT change the Usage tab's SIZE, do NOT widen the window, and do NOT change
  the default hotkey or the show/hide toggle behavior shipped in Task-0013.
- Genuine correctness constraints that are NOT preference and MUST hold:
  - The taskbar must never be covered (hard constraint, with a falsifier).
  - The padding fraction must be configurable (the human said "configurable").
  - The geometry must use the real work area, not full screen dimensions.
- Multi-monitor: the human is a single operator on one machine. Targeting the
  primary monitor's work area (e.g. `SPI_GETWORKAREA`) is acceptable; deeper
  per-monitor follow-the-cursor behavior is out of scope unless the human asks.

## Deployment Reality (do not let proof lie)

The running overlay is a PINNED RELEASE; editing repo source does NOT change what
the human sees until a new release is published and the overlay is restarted
(`scripts/Publish-DashboardRelease.ps1` + `Start-CodexDashboard.ps1`,
`dashboard-current-release.json`). The publish + restart is a live-lane action
gated separately by the human; it is NOT part of task CREATION. The task draft's
proof plan should be honest that source edits + passing unit tests do not, by
themselves, change the live overlay.
