<!-- task-sync: repo=CodexDashboard; task_id=Task-0002; task_path=Tracking/Task-0002/TASK.md -->

# Task-0002: CodexDashboard Stitch-aligned overlay redesign.

## Source Of Truth

Local `Tracking/Task-0002/TASK.md` owns rich task truth: full scope, acceptance
criteria, rationale, proof plans, audits, pass history, and local review
artifacts.

This GitHub Issue owns the queryable accepted-task identity for Task-0002:
issue number, URL, open/closed state, title, and shallow task summary that can
be discovered with `gh`.

Codex owns the sync operation. Codex renders the desired issue from the local
task, updates the matching GitHub issue, reads it back through `gh`, and
writes local task metadata only after successful readback.

## Summary

This task reworks the existing desktop overlay in `app/` into a Stitch-inspired control-room UI without changing the product's core role as a hotkey-first private cockpit.

The Stitch mockup is the primary composition reference. `Design/GENERAL-DESIGN.md` remains the product-intent anchor where the two differ.

## Goals

- Redesign the current overlay shell around the Stitch composition: compact header, clear dismiss path, interval toggle row, metric strip, single main token-velocity chart, status area, and footer controls.
- Preserve the existing hotkey-overlay operator-console behavior: quick summon/dismiss, immediate readability, and no maximize-first workflow.
- Keep the visual language dark, dense, and operational while still honoring the repo-root general design for the weekly burn/redline presentation, budget editing, startup toggle, and advisory context.
- Keep the work inside the existing desktop app under `app/` rather than creating a new product surface.

## Acceptance Criteria

- The current dashboard surface is refactored to visually and semantically follow the Stitch overlay composition.
- The overlay remains hotkey-first and dismissible immediately, including via the existing close path and `Escape` if already supported.
- The UI keeps the dark operator-console feel and clearly separates title, interval controls, summary metrics, main chart, status, and footer actions.
- The main chart remains a single token-velocity chart, not a multi-pane browser or transcript view.
- The general-design requirements still appear where they are part of the product intent: weekly burn/redline context, budget editing, startup toggle, and advisory Codex context.
- The task stays bounded to interface implementation in `app/` and does not expand into ingest, persistence, or backend rewrites.

## Non-Goals

- Reworking token ingest, persistence, or aggregation logic.
- Turning the app into a transcript browser, session explorer, or multi-pane console.
- Rewriting the desktop app architecture from scratch.
- Adding decorative motion, light-theme variants, or consumer-product chrome.
- Changing the repo-root product intent in `Design/GENERAL-DESIGN.md`.

## Sync Metadata

- GitHub repo: `Digital-Collective-Games/Obsidian`
- Issue number: `2`
- Local task path: `Tracking/Task-0002/TASK.md`
- Source commit: `75177b6dee23399358ee66676791fb41dc01d51e`
- Local task SHA-256: `6A47EAC0B90EF1A16B21EB212EF6AC4D173CEB16FB6D3856603CB48554890D1F`
- Rendered at: `2026-05-28T21:14:45.5903226-04:00`