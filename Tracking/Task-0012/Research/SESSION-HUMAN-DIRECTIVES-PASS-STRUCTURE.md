# Session Human Directives - Pass Structure

Date written: 2026-05-28

## Search Method

- Used the `codex-session-search` skill workflow.
- Read the `.codex` session-safety guardrails before inspecting transcripts.
- Tried the helper script first:
  - `search_codex_sessions.py --recent 25`
  - `search_codex_sessions.py --query 'Task-0012' --files-only`
  - `search_codex_sessions.py --query 'TASK-META' --files-only`
  - `search_codex_sessions.py --query 'GitHub issue provider task proposal' --files-only`
  - `search_codex_sessions.py --query 'final pass' --files-only`
- The helper timed out on these searches, so I used the skill's direct-shell fallback:
  - `rg` over recent session folders for `Task-0012`, `TASK-META`, `GitHub issue`, `provider`, `task proposal`, `labels`, `final pass`, and `pass structure`.
  - JSONL parsing of the relevant transcript, filtered to human `user_message` records.
- Assistant content was used only where a later human message explicitly accepted it.

## Transcript Paths Used

- Primary parent session:
  - `C:\Users\gregs\.codex\sessions\2026\05\27\rollout-2026-05-27T23-39-47-019e6caa-b60a-7e00-ae31-a5b37126923f.jsonl`
  - Session id: `019e6caa-b60a-7e00-ae31-a5b37126923f`
  - Session timestamp from metadata: `2026-05-28T03:39:47`
  - CWD: `c:\Agent\CodexDashboard`
- Current blank worker session:
  - `C:\Users\gregs\.codex\sessions\2026\05\28\rollout-2026-05-28T14-00-13-019e6fbe-74f2-7752-bdc4-69dc077f34ec.jsonl`
  - Used only to identify the parent session and the current artifact request.

## Directive Snippets

- `2026-05-28T03:59:57`, parent lines 172-173: "Task definition is incosistent. This is the task scope. Conform. As I said human directives override subagents or coordinators."
- `2026-05-28T03:59:57`, parent lines 172-173: "Push a single `TASK.md` into a GitHub-backed representation. Inspect that single published representation. Agree that it meets the quality bar. Then push all `TASK.md` files across all configured repos."
- `2026-05-28T05:44:00`, parent lines 470-471: "Yeah change schope, new task shape is purely codex driven. Rescope plan. Make sure gh stays in sync, but I do want to maintain a local TASK.md file so we need some coownership semantics."
- `2026-05-28T15:45:28`, parent lines 1144-1145: "Create a GitHub issue first, then create Tracking/Task-<issue-number>/TASK.md."
- `2026-05-28T15:48:57`, parent lines 1164-1165: "as soon as we need to create a TASK.md, we first ask gh for an id. That commits the id forever."
- `2026-05-28T15:57:11`, parent lines 1210-1211: "How about using a supplemental repo for task proposals?"
- `2026-05-28T15:58:20`, parent lines 1220-1221: "Lets have a root level manifest for where to find all the pieces of the repo for binding purposes to thinks like \"proposal store\" and other things. In json"
- `2026-05-28T16:37:06`, parent lines 1322-1323: "keep just providers but put remote_task_store and proposal_store in there delete everything else or argue for its existence"
- `2026-05-28T16:43:38`, parent lines 1367-1368: "TaskCreate should have a subdocument describing how to interface with a github provider. So it can resolve the folder rule."
- `2026-05-28T16:55:39`, parent lines 1458-1459: "we need separate providers for task, task_proposal, source_control"
- `2026-05-28T17:05:48`, parent lines 1625-1626: "creating a single repo registry for codex dashboard that tells it - what repos should i care about? - how do i interact with each repo?"
- `2026-05-28T17:09:43`, parent lines 1679-1680: "I agree, make it the last pass in TASK.md so we won't forget to do this."
- `2026-05-28T17:45:46`, parent lines 1858-1859: "Let's call it TASK-META.json."
- `2026-05-28T17:48:47`, parent lines 1903-1904: "rename ... GITHUB-ISSUE.json ... to TASK-META.json" and "re-run the task export without the unnecessary tags like codex-task or codex- whatever. Those are implicit based on the gh repostiory url"
- `2026-05-28T17:59:51`, parent lines 2077-2078: "Pass structure has drifted from my directives. Redefine the passes according to my directives; use a blank subagent on a codex session search of this session to extract that, record locally in markdown"

## Normalized Directives

- Human directives override subagent or coordinator interpretations for Task-0012.
- Task-0012 is a CodexDashboard task, not a shared-orchestration-only task.
- The task was rescoped to a Codex-driven workflow:
  - `gh` must stay in sync.
  - Local `TASK.md` remains useful and should keep task-owned execution context.
  - The design needs explicit coownership semantics between GitHub issue state and local task docs.
- Rollout sequence is staged:
  - publish one `TASK.md` into a GitHub-backed representation first;
  - inspect that one published representation;
  - get agreement that it meets the quality bar;
  - only then bulk-publish all `TASK.md` files across configured repos;
  - central "drain my tasks pls" worktree allocation is later follow-on work, not Task-0012 closure.
- Accepted task identity:
  - GitHub issue number is the accepted task ID source.
  - For new accepted tasks, create the GitHub issue first, then materialize `Tracking/Task-<issue-number>/TASK.md`.
  - Holes in local task numbering are acceptable.
  - Task creation should route through TaskCreate.
  - Current pilot mismatches are evidence, not the final identity convention.
- Proposal identity:
  - Task proposals should be searchable without polluting accepted task issue space.
  - Use a supplemental proposal provider/repo rather than accepted task issues.
  - For CodexDashboard, the accepted shape became repo-scoped proposal storage, e.g. `CodexDesktopProposals`.
  - Proposals are not dispatchable tasks.
  - Promotion should create a real GitHub issue in the task repo and then create the matching local task folder.
- Repo/provider registry:
  - Add a root-level JSON manifest/registry for CodexDashboard to know which repos it cares about and how to interact with each one.
  - Keep provider bindings, especially task, task proposal, and source control providers.
  - Do not introduce a configurable local task store abstraction; local task folder conventions remain fixed.
  - The source control provider should include the default agent user `gregsemple2003`.
  - A broad source-control abstraction/CLI can be considered later, but is too much complexity for this task.
- Labels:
  - Do not rely on extra labels like `codex-task` or `codex-*` for the pilot export.
  - Task/proposal role is implicit from the provider repository URL/store binding.
  - The GitHub issue export should be rerun without those unnecessary tags.
- Metadata naming:
  - Use `TASK-META.json` as the steady-state task/provider metadata artifact name.
  - Rename away from `GITHUB-ISSUE.json` / `TASK-PROVIDER-META.json`.
  - Keep `TASK-META.json` minimal: only fields immediately relevant to provider binding/readback should remain.
- Requested final pass:
  - Add a final pass covering the TaskCreate GitHub-provider interface and registry-based convention.
  - The pass should preserve the rule for creating accepted task issues, creating proposal issues, promoting proposals, and materializing `Tracking/Task-N/TASK.md` from the accepted issue number.

## Implications For PASS Structure

- PASS structure should follow the human decision sequence, not the older pilot-only plan.
- The single-issue pilot remains a required proof pass, but it is not the final identity model.
- A separate provider-registry pass is required because the manifest/registry became an explicit human directive.
- `TASK-META.json` and the no-extra-label export belong in the pilot/provider metadata pass, not as optional cleanup.
- The final pass should be the TaskCreate provider-interface follow-through:
  - document or queue the GitHub-provider subdocument;
  - encode the accepted issue-number-to-task-folder rule;
  - distinguish proposal provider behavior from accepted task behavior;
  - close the task against the registry-based convention rather than the superseded `GITHUB-ISSUE.json` pilot naming.
- Do not close Task-0012 as complete solely because issue creation/readback worked; closure also needs the final provider/TaskCreate follow-through requested by the human.
