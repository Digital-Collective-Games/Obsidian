# Project Deletion Proof

## Purpose

Remove the superseded user Project created during the rejected Project-field
iteration.

## Deleted Project

- Owner: `gregsemple2003`
- Number: `1`
- Title: `Codex Task Queue`
- Project id: `PVT_kwHOAOjEHM4BZFW5`

## Delete Command

```powershell
gh api graphql -f query='mutation($projectId:ID!){deleteProjectV2(input:{projectId:$projectId}){projectV2{id title}}}' -f projectId='PVT_kwHOAOjEHM4BZFW5'
```

Result:

```json
{"data":{"deleteProjectV2":{"projectV2":{"id":"PVT_kwHOAOjEHM4BZFW5","title":"Codex Task Queue"}}}}
```

## Readback

Command:

```powershell
gh project list --owner gregsemple2003 --format json --limit 100
```

Result:

```json
{"projects":[],"totalCount":0}
```

Issue `#1` still reports no labels and no Project items:

```json
{"labels":[],"number":1,"projectItems":[],"title":"Task-0001: Codex token-velocity dashboard with a hotkey-first overlay.","url":"https://github.com/Digital-Collective-Games/Obsidian/issues/1"}
```
