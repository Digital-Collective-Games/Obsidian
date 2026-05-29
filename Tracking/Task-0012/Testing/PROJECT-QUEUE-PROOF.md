# Project Queue Proof

Status: superseded. This file preserves the temporary GitHub Project proof from
the earlier iteration. The accepted surface is now org-level GitHub Issue Fields
on <https://github.com/Digital-Collective-Games/Obsidian/issues/1>, recorded in
[ISSUE-FIELDS-PROOF.md](./ISSUE-FIELDS-PROOF.md). After the repo transfer, issue
`#1` reports no `projectItems`.

## Project

- Owner: `gregsemple2003`
- Number: `1`
- Title: `Codex Task Queue`
- URL: <https://github.com/users/gregsemple2003/projects/1>
- Project id: `PVT_kwHOAOjEHM4BZFW5`
- Visibility: private user Project

## Fields

- `Queue`
  - field id: `PVTSSF_lAHOAOjEHM4BZFW5zhUGkhM`
  - options: `Never`, `Ready`, `Human Gate`
- `Priority`
  - field id: `PVTSSF_lAHOAOjEHM4BZFW5zhUGkhY`
  - options: `P0`, `P1`, `P2`, `P3`

## Issue Item

- Issue: <https://github.com/gregsemple2003/CodexDesktop/issues/1>
- Project item id: `PVTI_lAHOAOjEHM4BZFW5zguF8KU`

## Readback

`gh project item-list 1 --owner gregsemple2003 --format json --limit 100`
returned one item for issue `#1` with:

- `queue`: `Never`
- `priority`: `P2`
- `status`: `Todo`
- `repository`: `https://github.com/gregsemple2003/CodexDesktop`

`gh issue view 1 --repo gregsemple2003/CodexDesktop --json number,title,labels,projectItems,url`
returned:

- `labels`: `[]`
- `projectItems`: `Codex Task Queue`

## Scope Note

The repo manifest does not define queue or priority values. The global user
Project owns those fields; the issue is enrolled in that Project and carries
the selected values there.
