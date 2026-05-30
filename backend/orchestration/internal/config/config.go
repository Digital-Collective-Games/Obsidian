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

	// QueueDrainRepo is the provider repo (owner/name) the O3 queue-drain consumer
	// polls for Queue==Ready issues. Empty (the default) leaves the consumer dormant
	// — the workflow is registered but the start endpoint dispatches against nothing
	// — so this is safe to ship without a configured provider repo.
	QueueDrainRepo string // CODEX_ORCHESTRATION_QUEUE_DRAIN_REPO
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
	}
	cfg.WorktreeRoot = envOrDefault("CODEX_ORCHESTRATION_WORKTREE_ROOT", resolveWorktreeRoot())
	cfg.TrackingRoot = envOrDefault("CODEX_ORCHESTRATION_TRACKING_ROOT", filepath.Join(cfg.WorktreeRoot, "Tracking"))

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
