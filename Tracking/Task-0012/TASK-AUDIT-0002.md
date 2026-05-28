# Task Audit 0002

Audit date: 2026-05-27

Audit path: TaskCreate clean-context auditor-worker

Reviewed draft:

- [TASK-DRAFT-0002-BEFORE-AUDIT.md](./TASK-DRAFT-0002-BEFORE-AUDIT.md)

Current draft path:

- [TASK.md](./TASK.md)

Audit standard:

- `C:\Users\gregs\.codex\Orchestration\Processes\TASK-AUDIT.md`

Auditor worker:

- `019e6c59-c33f-7ee3-92aa-6068a518534f`

## Intact Auditor Response

Blindness: no `non_blind_contaminated` issue detected. I used the supplied auditor docs, the draft, and durable files cited by the draft or required by the audit standard. I did not edit files.

**Verdict**

`revise`

**Primary Findings**

1. The draft is missing required burden-reduction sections. It clearly frames human burden in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L29), but it has no `## Human Relief If Successful` or `## Remaining Uncertainty` sections; the headings jump from [Falsifier](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L423) to [Not Solved Here](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L475). The shared task standard requires both for burden-reduction proposal tasks in [TASK-CREATE.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-CREATE.md#L276).

2. The closeout wording still leaves a proxy-closure path. The draft says only human acceptance closes the task in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L21), but later says "Task closeout records" `waiting_for_human` or `blocked` in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L400). That should be rewritten so only `accepted` is closure; `waiting_for_human` and `blocked` are nonterminal task states.

3. The proof plan does not pin backend proof to CodexDashboard's isolated validation lane. The draft calls for endpoint proof in [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L406), but repo rules require agent validation/regression to avoid the human lane and use isolated lane/data flow in [AGENTS.md](/c:/Agent/CodexDashboard/AGENTS.md#L71) and [TESTING.md](/c:/Agent/CodexDashboard/TESTING.md#L73).

**Blocking Clarifying Questions**

- Is [TASK.md](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L1) intended to remain a burden-reduction proposal task? If yes, add the missing required sections. If no, rewrite the burden framing so the selected task shape is honest under the ordinary concrete-implementation bar.
- Should real publish proof be authorized only for a specific GitHub repo/account and isolated backend lane? The current draft treats repo/auth selection as execution input, but the proof path should not imply the service lane or human lane is acceptable by default.

**Strengthening Clarifying Questions**

- Should `waiting_for_human` require a `TASK-STATE.json` or `HANDOFF.md` update, in addition to the review package?
- Should the pilot target default to publishing [Task-0012](/c:/Agent/CodexDashboard/Tracking/Task-0012/TASK.md#L1), or should the caller always supply a task id?

**Required Rewrites**

- Add `## Human Relief If Successful` and `## Remaining Uncertainty`.
- Replace "Task closeout records" with wording that separates terminal closure from nonterminal `waiting_for_human` and `blocked` states.
- Add proof-plan/acceptance wording requiring isolated validation-lane backend proof unless the human explicitly authorizes touching the service/human lane.

**Optional Strengthening**

- State where the nonterminal wait/block state is durably recorded.
- Require the review package to name the exact human approval question for the pilot issue shape.

**Relevant Rule Or Exemplar**

- Burden-reduction required sections: [TASK-CREATE.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-CREATE.md#L276)
- Audit rejection for missing rich sections: [TASK-AUDIT.md](/c:/Users/gregs/.codex/Orchestration/Processes/TASK-AUDIT.md#L520)
- Isolated proof lane requirement: [TESTING.md](/c:/Agent/CodexDashboard/TESTING.md#L75)

## Coordinator Disposition

Status: revision required before rerunning clean audit.

Rewrite instructions to writer:

- Keep the draft as a burden-reduction concrete implementation task and add the missing required `## Human Relief If Successful` and `## Remaining Uncertainty` sections.
- Rewrite closeout language so only explicit human acceptance of the pilot issue shape is terminal closure. `waiting_for_human` and `blocked` must be represented as nonterminal states, with the durable artifact or state location named.
- Pin backend proof to CodexDashboard's isolated validation lane by default. State that service/human lane publication or proof requires explicit human authorization.
- Add the optional approval-question requirement if it fits without expanding scope: the review package should ask the exact human approval question about whether the pilot issue shape is accepted for bulk publication.
