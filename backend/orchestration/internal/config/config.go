package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Config struct {
	BindAddress     string
	JobsRoot        string
	WorktreeRoot    string
	TrackingRoot    string
	Namespace       string
	TaskQueue       string
	TemporalAddress string
	CodexExecutable string
	RunsRoot        string

	// Run-alert surfacing (the @@JOB-ALERT@@ "I need an adult" convention). When a job run prints a
	// sentinel line or fails, the backend invokes AlertCommand with a JSON digest so the operator gets
	// ONE email per run. Empty AlertCommand (the default) disables alerting — it stays dormant until
	// configured, so this is safe to ship without breaking existing jobs.
	EnableRunAlerts bool   // CODEX_ORCHESTRATION_ENABLE_RUN_ALERTS (default true)
	AlertCommand    string // CODEX_ORCHESTRATION_ALERT_COMMAND  (path to a PowerShell sender script; empty = disabled)
	AlertRecipient  string // CODEX_ORCHESTRATION_ALERT_RECIPIENT (operator email passed to the sender)

	// QueueDrainRepo is retained ONLY for the legacy/manual single-task dispatch
	// path and the start-endpoint config shape; the registry-driven queue-drain
	// consumer no longer reads it to pick a provider repo (it enumerates the central
	// registry instead). Empty is the default.
	QueueDrainRepo string // CODEX_ORCHESTRATION_QUEUE_DRAIN_REPO

	// RegistryPath is the EXPLICIT path to the central REPO-MANIFEST.json registry
	// the registry-driven consumer enumerates each poll (single source of truth /
	// global awareness of all registered repos). It defaults to the backend repo
	// root's REPO-MANIFEST.json. New env vars use the OBSIDIAN_ prefix.
	RegistryPath string // OBSIDIAN_REGISTRY_PATH (default <WorktreeRoot>/REPO-MANIFEST.json)

	// LaunchQueueAgent gates the O5 wiring: when a queue dispatch provisions an owned
	// worktree, launch a top-level claude agent in it, bind its session, and start the
	// liveness watchdog supervisor. Default FALSE so legacy/manual dispatch paths are
	// unaffected — the launch fires ONLY when this is explicitly enabled.
	LaunchQueueAgent bool // CODEX_ORCHESTRATION_QUEUE_LAUNCH_AGENT (default false)
	// QueueAgentAllowedTools / QueueAgentPermissionMode are the configurable claude
	// launch knobs for a dispatched queue agent (a working task agent needs real tools
	// — Read/Edit/Bash/Agent — not just the trivial-proof "Agent"). Empty falls back to
	// the launcher's validated defaults.
	QueueAgentAllowedTools   string // CODEX_ORCHESTRATION_QUEUE_AGENT_ALLOWED_TOOLS
	QueueAgentPermissionMode string // CODEX_ORCHESTRATION_QUEUE_AGENT_PERMISSION_MODE

	// AutoCloseQueued is a TEST-ONLY toggle that lets the queue-drain consumer SIMULATE
	// a human approving a close: when an active dispatched task announces completion
	// (its worktree TASK-STATE.json current_gate == "closure") the consumer closes the
	// GitHub issue exactly as a human would, so a regression test can prove the full
	// dispatch -> done -> close -> reclaim -> slot-reuse lifecycle. Default FALSE so the
	// consumer stays read-only against GitHub (A4.6) and only a human closes.
	AutoCloseQueued bool // OBSIDIAN_AUTO_CLOSE_QUEUED (default false)
}

func Load() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, errors.New("resolve home directory")
	}

	cfg := Config{
		BindAddress:     envOrDefault("CODEX_ORCHESTRATION_BIND_ADDRESS", "127.0.0.1:4318"),
		JobsRoot:        envOrDefault("CODEX_ORCHESTRATION_JOBS_ROOT", filepath.Join(home, ".codex", "Orchestration", "Jobs")),
		Namespace:       envOrDefault("CODEX_ORCHESTRATION_NAMESPACE", "default"),
		TaskQueue:       envOrDefault("CODEX_ORCHESTRATION_TASK_QUEUE", "codex-orchestration"),
		TemporalAddress: envOrDefault("CODEX_ORCHESTRATION_TEMPORAL_ADDRESS", "127.0.0.1:7233"),
		CodexExecutable: resolveCodexExecutable(home),
		RunsRoot:        envOrDefault("CODEX_ORCHESTRATION_RUNS_ROOT", defaultRunsRoot(home)),
		EnableRunAlerts: envBoolOrDefault("CODEX_ORCHESTRATION_ENABLE_RUN_ALERTS", true),
		AlertCommand:    envOrDefault("CODEX_ORCHESTRATION_ALERT_COMMAND", ""),
		AlertRecipient:  envOrDefault("CODEX_ORCHESTRATION_ALERT_RECIPIENT", ""),
		QueueDrainRepo:  envOrDefault("CODEX_ORCHESTRATION_QUEUE_DRAIN_REPO", ""),

		LaunchQueueAgent:         envBoolOrDefault("CODEX_ORCHESTRATION_QUEUE_LAUNCH_AGENT", false),
		QueueAgentAllowedTools:   envOrDefault("CODEX_ORCHESTRATION_QUEUE_AGENT_ALLOWED_TOOLS", "Read,Edit,Bash,Agent"),
		QueueAgentPermissionMode: envOrDefault("CODEX_ORCHESTRATION_QUEUE_AGENT_PERMISSION_MODE", ""),

		// TEST-ONLY: simulates human closure approval; default false keeps the consumer
		// read-only against GitHub.
		AutoCloseQueued: envBoolOrDefault("OBSIDIAN_AUTO_CLOSE_QUEUED", false),
	}
	cfg.WorktreeRoot = envOrDefault("CODEX_ORCHESTRATION_WORKTREE_ROOT", resolveWorktreeRoot())
	cfg.TrackingRoot = envOrDefault("CODEX_ORCHESTRATION_TRACKING_ROOT", filepath.Join(cfg.WorktreeRoot, "Tracking"))
	cfg.RegistryPath = envOrDefault("OBSIDIAN_REGISTRY_PATH", filepath.Join(cfg.WorktreeRoot, "REPO-MANIFEST.json"))

	if cfg.JobsRoot == "" {
		return Config{}, errors.New("jobs root must not be empty")
	}
	if cfg.WorktreeRoot == "" {
		return Config{}, errors.New("worktree root must not be empty")
	}
	if cfg.TrackingRoot == "" {
		return Config{}, errors.New("tracking root must not be empty")
	}

	return cfg, nil
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBoolOrDefault(key string, fallback bool) bool {
	switch strings.TrimSpace(strings.ToLower(os.Getenv(key))) {
	case "":
		return fallback
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func defaultRunsRoot(home string) string {
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		return filepath.Join(localAppData, "CodexDashboard", "orchestration-runs")
	}
	return filepath.Join(home, "AppData", "Local", "CodexDashboard", "orchestration-runs")
}

func resolveCodexExecutable(home string) string {
	if configured := os.Getenv("CODEX_ORCHESTRATION_CODEX_EXECUTABLE"); configured != "" {
		return configured
	}

	if path, err := exec.LookPath("codex"); err == nil {
		return path
	}

	candidates := []string{}
	globs := []string{
		filepath.Join(home, ".vscode-oss", "extensions", "openai.chatgpt-*", "bin", "windows-x86_64", "codex.exe"),
		filepath.Join(home, ".vscode", "extensions", "openai.chatgpt-*", "bin", "windows-x86_64", "codex.exe"),
	}
	for _, pattern := range globs {
		matches, _ := filepath.Glob(pattern)
		candidates = append(candidates, matches...)
	}
	if len(candidates) == 0 {
		return "codex"
	}

	sort.Strings(candidates)
	return candidates[len(candidates)-1]
}

func resolveWorktreeRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}

	current := wd
	for {
		if pathExists(filepath.Join(current, "Tracking")) && pathExists(filepath.Join(current, "backend", "orchestration")) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return wd
		}
		current = parent
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
