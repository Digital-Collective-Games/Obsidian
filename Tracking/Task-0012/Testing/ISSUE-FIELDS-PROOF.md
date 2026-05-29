# Issue Fields Proof

## Org

- Organization: <https://github.com/Digital-Collective-Games>
- Repo: <https://github.com/Digital-Collective-Games/Obsidian>
- Issue: <https://github.com/Digital-Collective-Games/Obsidian/issues/1>

## Org Issue Fields

Command:

```powershell
gh api /orgs/Digital-Collective-Games/issue-fields --jq 'map({id,name,data_type,visibility,options:(.options // [] | map({name,color,priority}))})'
```

Result:

```json
[
  {
    "data_type": "single_select",
    "id": 42656780,
    "name": "Priority",
    "options": [
      {"color": "red", "name": "P0", "priority": 1},
      {"color": "orange", "name": "P1", "priority": 2},
      {"color": "yellow", "name": "P2", "priority": 3},
      {"color": "green", "name": "P3", "priority": 4}
    ],
    "visibility": "all"
  },
  {
    "data_type": "single_select",
    "id": 42656828,
    "name": "Queue",
    "options": [
      {"color": "gray", "name": "Never", "priority": 1},
      {"color": "green", "name": "Ready", "priority": 2}
    ],
    "visibility": "all"
  },
  {
    "data_type": "single_select",
    "id": 42656829,
    "name": "Human Needed",
    "options": [
      {"color": "green", "name": "No", "priority": 1},
      {"color": "red", "name": "Yes", "priority": 2}
    ],
    "visibility": "all"
  }
]
```

## Issue Values

Command:

```powershell
gh api /repos/Digital-Collective-Games/Obsidian/issues/1/issue-field-values --jq 'map({issue_field_id,single_select_option})'
```

Result:

```json
[
  {
    "issue_field_id": 42656828,
    "single_select_option": {"color": "gray", "id": 74648225, "name": "Never"}
  },
  {
    "issue_field_id": 42656829,
    "single_select_option": {"color": "green", "id": 74648227, "name": "No"}
  },
  {
    "issue_field_id": 42656780,
    "single_select_option": {"color": "yellow", "id": 74648223, "name": "P2"}
  }
]
```

## Issue Project State

Command:

```powershell
gh issue view 1 --repo Digital-Collective-Games/Obsidian --json number,title,url,updatedAt,labels,projectItems
```

Result:

```json
{
  "labels": [],
  "number": 1,
  "projectItems": [],
  "title": "Task-0001: Codex token-velocity dashboard with a hotkey-first overlay.",
  "updatedAt": "2026-05-28T20:37:56Z",
  "url": "https://github.com/Digital-Collective-Games/Obsidian/issues/1"
}
```

The issue is detached from the temporary user Project. The old user Project was
deleted after broad `gh` auth refresh; see
[PROJECT-DELETION-PROOF.md](./PROJECT-DELETION-PROOF.md).
