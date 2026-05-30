package queue

import (
	"fmt"
	"strings"
)

// O5 launch prompt for a dispatched queue agent. The consumer launches a TOP-LEVEL
// claude in the owned worktree (able to spawn its own subagents). The prompt tells
// the agent which Task-N it owns and binds it to the DONE-CONTRACT (O4 /
// HUMAN-DIRECTIVES): it NEVER self-closes the GitHub issue — on perceived completion
// or abandon it sets Human Needed=Yes via the obsidian-operator skill and STOPS,
// asking for an explicit human closure approval.
//
// The prompt is CONFIGURABLE (a template with safe defaults). BuildTaskAgentPrompt
// renders the default; callers may supply their own template via the {{TASK_ID}} and
// {{WORKTREE}} placeholders.

// DefaultTaskAgentPromptTemplate is the safe default launch prompt. {{TASK_ID}} and
// {{WORKTREE}} are substituted with the dispatched task id and its owned worktree.
const DefaultTaskAgentPromptTemplate = `You are the autonomous task agent for {{TASK_ID}}, working in the git worktree at {{WORKTREE}} (this is your cwd).

Do the work for this task:
1. Read Tracking/{{TASK_ID}}/TASK.md (and its PLAN/HANDOFF if present) to understand the scope and acceptance criteria.
2. Implement the task within this worktree. You are a top-level agent: you may spawn your own subagents via the Agent tool.

DONE-CONTRACT (mandatory — you are NOT permitted to close the GitHub issue):
- NEVER close the GitHub issue yourself, for ANY reason (not on perceived completion, not on abandon).
- When you believe the work is complete, do NOT self-close. ANNOUNCE completion by editing Tracking/{{TASK_ID}}/TASK-STATE.json: read the file, set ONLY the "current_gate" field to "closure" (leave every other field unchanged), and write it back. Then use the obsidian-operator skill to set Human Needed=Yes with a run/gate state of "awaiting closure approval", and STOP and ask the human for an explicit closure directive.
- Setting current_gate to "closure" is ONLY an announcement that you are done; it is NOT closing the GitHub issue. You still must never close the issue — closure remains a human/backend action.
- When you need a human mid-work (a research, plan, or regression gate) or judge the task should be abandoned / is a bad idea, likewise use the obsidian-operator skill to set Human Needed=Yes and STOP. Do NOT close the issue.
- Closing the issue is ALWAYS an explicit human action. A passed test, regression run, plan, or research approval is NOT closure approval.`

// BuildTaskAgentPrompt renders a launch prompt from a template by substituting
// {{TASK_ID}} and {{WORKTREE}}. An empty template uses
// DefaultTaskAgentPromptTemplate. It errors on an empty task id or worktree so a
// launch never goes out with an unbound prompt.
func BuildTaskAgentPrompt(template, taskID, worktreePath string) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("build task-agent prompt: task id is empty")
	}
	if worktreePath == "" {
		return "", fmt.Errorf("build task-agent prompt: worktree path is empty")
	}
	if template == "" {
		template = DefaultTaskAgentPromptTemplate
	}
	prompt := strings.ReplaceAll(template, "{{TASK_ID}}", taskID)
	prompt = strings.ReplaceAll(prompt, "{{WORKTREE}}", worktreePath)
	return prompt, nil
}
