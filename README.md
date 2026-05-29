# Obsidian

Windows-first token velocity dashboard for local Codex and Claude Code session
telemetry. (The repository folder and Python package remain `CodexDashboard` /
`codex_dashboard` for tooling stability; the product name is "Obsidian".)

## Current Stack

- Python 3.13
- Tkinter for the desktop surface
- SQLite for cursor and event persistence
- Win32 integration through `ctypes`

## Current Status

The first implementation path is a Windows-only Python prototype because this host has Python and `tkinter` available but does not currently expose the Tauri toolchain on `PATH`.

## Quick Start

Create a local runtime folder if desired, then run a one-shot ingest summary:

```powershell
python -m app.codex_dashboard --scan-once --print-summary
```

Run the focused unit suite:

```powershell
python -m unittest discover -s tests -p "test_*.py" -v
```

The overlay UI and desktop integration land in later passes. The current pass establishes the ingest core, persistence, repo docs, and unit coverage.

> Temporary note for diff-color inspection.
