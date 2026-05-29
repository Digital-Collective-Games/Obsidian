# Chrome Issue Fields Inspection

## Source

- URL: <https://github.com/Digital-Collective-Games/Obsidian/issues/1>
- Capture time: `2026-05-28T16:42:39.4298559-04:00`
- Tool: `C:\Agent\Orchestrator\Scripts\Scrape-OpenChromeTabs.ps1`
- Format: page text from Chrome debug profile

## Result

The GitHub issue page loaded as:

```text
Task-0001: Codex token-velocity dashboard with a hotkey-first overlay. · Issue #1 · Digital-Collective-Games/Obsidian
```

The page text showed the right-sidebar metadata block:

```text
Fields

Priority

P2

Queue

Never

Human Needed

No
Projects
No projects
```

This verifies the human-facing issue page exposes the selected field values
without relying on the temporary GitHub Project surface.
