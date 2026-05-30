package jobexec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/controlplane"
)

// JobAlertSentinel is the cross-repo, language-agnostic contract: any job (any executor, any language)
// that prints a line containing this token to stdout/stderr is asking the operator for attention
// ("I need an adult"). The backend collects these per run and emits ONE digest via the configured alert
// command. Recoverable/transient issues should NOT print it. See bizdev/JOB-ALERT-CONVENTION.md.
const JobAlertSentinel = "@@JOB-ALERT@@"

const (
	alertMaxScanBytes   = 1 << 20 // scan at most the last 1 MiB of each output file
	alertStderrTailMax  = 4000    // bytes of stderr tail included in the digest
	alertCommandTimeout = 60 * time.Second
)

type runAlertPayload struct {
	JobID        string    `json:"job_id"`
	Label        string    `json:"label,omitempty"`
	WorkflowID   string    `json:"workflow_id"`
	RunID        string    `json:"run_id"`
	TriggerType  string    `json:"trigger_type,omitempty"`
	ExitCode     int       `json:"exit_code"`
	Failed       bool      `json:"failed"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	Recipient    string    `json:"recipient,omitempty"`
	AlertLines   []string  `json:"alert_lines"`
	StderrTail   string    `json:"stderr_tail,omitempty"`
	EventLogPath string    `json:"event_log_path,omitempty"`
	StderrPath   string    `json:"stderr_path,omitempty"`
}

// maybeSendRunAlert surfaces notify-worthy run outcomes — a @@JOB-ALERT@@ line in the run's output, or a
// failed/non-zero run — to the operator via the configured alert command, as ONE digest per run. It is
// fully defensive: any error or panic here is logged and swallowed, so alerting can NEVER change a job's
// result or fail a run. Dormant unless EnableRunAlerts && AlertCommand are configured.
func (a *codexExecActivity) maybeSendRunAlert(ctx context.Context, request controlplane.JobRunRequest, result controlplane.JobRunResult, failed bool) {
	logger := activity.GetLogger(ctx)
	defer func() {
		if r := recover(); r != nil {
			logger.Error("run-alert handler panicked; ignored", "panic", r)
		}
	}()

	if !a.cfg.EnableRunAlerts || strings.TrimSpace(a.cfg.AlertCommand) == "" {
		return
	}

	alertLines := scanAlertLines(result.EventLogPath)
	alertLines = append(alertLines, scanAlertLines(result.StderrPath)...)

	if len(alertLines) == 0 && !failed {
		return // nothing notify-worthy this run
	}

	payload := runAlertPayload{
		JobID:        result.JobID,
		Label:        request.Spec.Label,
		WorkflowID:   result.WorkflowID,
		RunID:        result.RunID,
		TriggerType:  result.TriggerType,
		ExitCode:     result.ExitCode,
		Failed:       failed,
		StartedAt:    result.StartedAt,
		CompletedAt:  result.CompletedAt,
		Recipient:    a.cfg.AlertRecipient,
		AlertLines:   alertLines,
		StderrTail:   tailFile(result.StderrPath, alertStderrTailMax),
		EventLogPath: result.EventLogPath,
		StderrPath:   result.StderrPath,
	}

	if err := invokeAlertCommand(ctx, a.cfg.AlertCommand, payload); err != nil {
		logger.Error("run-alert command failed; ignored", "error", err.Error(), "job_id", result.JobID)
		return
	}
	logger.Info("run-alert dispatched", "job_id", result.JobID, "alert_lines", len(alertLines), "failed", failed)
}

// scanAlertLines returns the lines of the file (scanning at most the trailing alertMaxScanBytes) that
// contain the sentinel. Missing/unreadable files yield no lines (never an error).
func scanAlertLines(path string) []string {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if len(data) > alertMaxScanBytes {
		data = data[len(data)-alertMaxScanBytes:]
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, JobAlertSentinel) {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}

// tailFile returns the trailing maxBytes of the file (trimmed), or "" if unreadable.
func tailFile(path string, maxBytes int) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if maxBytes > 0 && len(data) > maxBytes {
		data = data[len(data)-maxBytes:]
	}
	return strings.TrimSpace(string(data))
}

// invokeAlertCommand runs the configured PowerShell alert sender, passing the JSON payload on stdin.
// Bounded by alertCommandTimeout so a hung sender cannot block the activity.
func invokeAlertCommand(ctx context.Context, command string, payload runAlertPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, alertCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", command)
	cmd.Stdin = bytes.NewReader(body)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return err
	}
	return nil
}
