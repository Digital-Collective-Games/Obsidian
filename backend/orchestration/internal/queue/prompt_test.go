package queue

import (
	"strings"
	"testing"
)

// BuildTaskAgentPrompt renders the default launch prompt with the task id + worktree
// substituted, and the prompt MUST bind the agent to the done-contract: it names the
// task, tells the agent it works in the worktree, and forbids self-closing the issue.
func TestBuildTaskAgentPromptDefaultBindsDoneContract(t *testing.T) {
	prompt, err := BuildTaskAgentPrompt("", "Task-7001", `C:\Agent\QueueDrainTestbed`)
	if err != nil {
		t.Fatalf("BuildTaskAgentPrompt: %v", err)
	}
	for _, want := range []string{
		"Task-7001",
		`C:\Agent\QueueDrainTestbed`,
		"Tracking/Task-7001/TASK.md",
		"NEVER close the GitHub issue",
		"Human Needed=Yes",
		"awaiting closure approval",
		"obsidian-operator",
		"spawn your own subagents",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("default prompt missing %q:\n%s", want, prompt)
		}
	}
	// No unsubstituted placeholders remain.
	if strings.Contains(prompt, "{{TASK_ID}}") || strings.Contains(prompt, "{{WORKTREE}}") {
		t.Fatalf("prompt still has unsubstituted placeholders:\n%s", prompt)
	}
}

// A custom template is honored and its placeholders substituted (the prompt is
// configurable per HUMAN-DIRECTIVES O5).
func TestBuildTaskAgentPromptCustomTemplate(t *testing.T) {
	prompt, err := BuildTaskAgentPrompt("agent for {{TASK_ID}} in {{WORKTREE}}", "Task-0015", `C:\wt`)
	if err != nil {
		t.Fatalf("BuildTaskAgentPrompt: %v", err)
	}
	if prompt != `agent for Task-0015 in C:\wt` {
		t.Fatalf("custom prompt = %q", prompt)
	}
}

func TestBuildTaskAgentPromptValidatesInputs(t *testing.T) {
	if _, err := BuildTaskAgentPrompt("", "", `C:\wt`); err == nil {
		t.Fatal("expected error on empty task id")
	}
	if _, err := BuildTaskAgentPrompt("", "Task-1", ""); err == nil {
		t.Fatal("expected error on empty worktree")
	}
}

// The configurable launch knobs default to the validated invocation values when a
// LaunchSpec leaves them empty, so the existing trivial-proof command is unchanged.
func TestBuildAgentCommandConfigurableToolsAndModeWithDefaults(t *testing.T) {
	// Defaults applied when empty.
	_, args, err := buildAgentCommand(LaunchSpec{Executable: "claude", WorktreePath: `C:\wt`, SessionID: "s"})
	if err != nil {
		t.Fatalf("buildAgentCommand: %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--allowedTools Agent") || !strings.Contains(joined, "--permission-mode bypassPermissions") {
		t.Fatalf("defaults not applied: %v", args)
	}

	// Overrides honored (the real queue agent needs Read/Edit/Bash/Agent).
	_, args, err = buildAgentCommand(LaunchSpec{
		Executable: "claude", WorktreePath: `C:\wt`, SessionID: "s",
		AllowedTools: "Read,Edit,Bash,Agent", PermissionMode: "acceptEdits",
	})
	if err != nil {
		t.Fatalf("buildAgentCommand override: %v", err)
	}
	joined = strings.Join(args, " ")
	if !strings.Contains(joined, "--allowedTools Read,Edit,Bash,Agent") || !strings.Contains(joined, "--permission-mode acceptEdits") {
		t.Fatalf("overrides not honored: %v", args)
	}
}
