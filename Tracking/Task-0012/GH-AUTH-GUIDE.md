# GitHub CLI Auth Guide

Use this to authenticate `gh` so Codex can create, edit, list, and inspect
GitHub Issues for the task-state pilot.

## What You Are Doing

You are signing the local GitHub CLI into GitHub from your Windows user account.
The login should persist across restarts unless the token is revoked, you log
out, Windows Credential Manager is cleared, or CodexDashboard later runs under a
different Windows account or service account.

Do not paste tokens into chat. The browser flow is preferred.

## Recommended Command

Open PowerShell and run:

```powershell
gh auth login --hostname github.com --web --git-protocol https --scopes repo,read:org
```

Why these options:

- `--hostname github.com`: authenticate the normal GitHub host.
- `--web`: use the browser login flow.
- `--git-protocol https`: avoid SSH key setup prompts for this task.
- `--scopes repo,read:org`: request access needed for private repo issue work
  and org/repo visibility.

## What To Click Or Choose

1. If a browser opens, sign into the GitHub account that should own/write the
   pilot issue.
2. If PowerShell shows a one-time code, copy it into the browser page GitHub
   opens.
3. Approve or authorize GitHub CLI when GitHub asks.
4. Return to PowerShell and wait until the command reports success.

If GitHub asks about SSO for an organization, authorize SSO for the organization
that owns the target repo.

## Verify It Worked

Run:

```powershell
gh auth status --hostname github.com
```

Expected result:

- the command exits successfully
- it shows an active account on `github.com`
- it does not say `not logged into any GitHub hosts`

Then optionally check repo access:

```powershell
gh issue list --repo Digital-Collective-Games/Obsidian --limit 1
```

If that command can list issues or says there are no issues, repo issue access is
working. If it says the repo cannot be found or access is denied, the active
GitHub account does not have the right repo access or SSO is not authorized.

## After You Finish

Tell Codex:

```text
gh auth done
```

Codex will rerun:

```powershell
gh auth status --hostname github.com
```

If it passes, Codex can continue `PASS-0002` and create or update exactly one
pilot issue after confirming the target repo.

## Troubleshooting

- Wrong account: run `gh auth status --hostname github.com`, then use
  `gh auth switch --hostname github.com` if multiple accounts are present.
- Need to start over: run `gh auth logout --hostname github.com`, then rerun the
  recommended login command.
- Do not run `gh auth status --show-token` unless you specifically need to debug
  credentials locally. It prints sensitive token material.

## Sources

- GitHub CLI manual, `gh auth login`: <https://cli.github.com/manual/gh_auth_login>
- GitHub CLI manual, `gh auth status`: <https://cli.github.com/manual/gh_auth_status>
