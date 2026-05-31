package temporalbackend

import (
	"context"
	"os"
	"testing"
	"time"

	"go.temporal.io/sdk/worker"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/config"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/taskexec"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/taskrun"
)

// TestLanding2GateBindingRoundTripLiveTemporal proves the Landing 2 cutover against a REAL
// Temporal server: BindLaunchedSession and SetRunGateState SIGNAL the per-run
// TaskRunWorkflow and the query reads the live binding back (the JSON side-store is gone).
// It is GATED behind OBSIDIAN_LIVE_TEMPORAL so the normal `go test` suite never dials
// Temporal; run it explicitly against the isolated validation server:
//
//	OBSIDIAN_LIVE_TEMPORAL=1 CODEX_ORCHESTRATION_TEMPORAL_ADDRESS=127.0.0.1:17233 \
//	  CODEX_ORCHESTRATION_NAMESPACE=reg007r \
//	  go test ./internal/temporalbackend/ -run TestLanding2GateBindingRoundTripLiveTemporal -count=1 -v
func TestLanding2GateBindingRoundTripLiveTemporal(t *testing.T) {
	if os.Getenv("OBSIDIAN_LIVE_TEMPORAL") == "" {
		t.Skip("set OBSIDIAN_LIVE_TEMPORAL=1 to run the live-Temporal Landing 2 smoke")
	}

	cfg := config.Config{
		Namespace:       smokeEnvOr("CODEX_ORCHESTRATION_NAMESPACE", "default"),
		TaskQueue:       "landing2-smoke-" + time.Now().Format("20060102-150405.000000000"),
		TemporalAddress: smokeEnvOr("CODEX_ORCHESTRATION_TEMPORAL_ADDRESS", "127.0.0.1:17233"),
		RunsRoot:        t.TempDir(),
	}
	b, err := New(cfg)
	if err != nil {
		t.Fatalf("dial temporal %s ns=%s: %v", cfg.TemporalAddress, cfg.Namespace, err)
	}
	defer b.Close()

	// Minimal worker: only the per-run TaskRunWorkflow is needed (no jobexec/queue wiring).
	w := worker.New(b.client, cfg.TaskQueue, worker.Options{})
	taskexec.Register(w)
	if err := w.Start(); err != nil {
		t.Fatalf("start worker: %v", err)
	}
	defer w.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Unique run id (collision-free across reruns). Empty RepoLane -> RunArtifactRoot==""
	// -> runOwnedLaneExecution returns immediately and the workflow goes straight to the
	// signal select loop (no git/agent work needed to exercise the gate/binding signals).
	runID := "landing2-smoke--" + time.Now().Format("20060102-150405.000000000")
	if _, err := b.StartTaskRun(ctx, taskrun.StartTaskRunRequest{RunID: runID, TaskID: "Task-9001"}); err != nil {
		t.Fatalf("start task run: %v", err)
	}
	defer func() { _ = b.client.TerminateWorkflow(context.Background(), runID, "", "landing2 smoke cleanup") }()

	// Wait until the workflow's query handler is live.
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, qerr := b.GetTaskRun(ctx, runID); qerr == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("workflow %s not queryable within deadline", runID)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// BindLaunchedSession: signal -> workflow binding -> read back.
	bound, err := b.BindLaunchedSession(ctx, runID, "sess-xyz", `C:\tmp\t.jsonl`, 4242)
	if err != nil {
		t.Fatalf("bind launched session: %v", err)
	}
	if bound.RepoLane.Binding == nil || bound.RepoLane.Binding.AgentSessionID != "sess-xyz" || bound.RepoLane.Binding.LaunchedPID != 4242 {
		t.Fatalf("bind round-trip did not reflect on the workflow binding: %#v", bound.RepoLane.Binding)
	}

	// SetRunGateState: signal -> workflow gate -> read back. The earlier bind must survive.
	parked, err := b.SetRunGateState(ctx, runID, taskrun.RunGateStateParkedAwaitingClosure)
	if err != nil {
		t.Fatalf("set run/gate state: %v", err)
	}
	if parked.RepoLane.Binding == nil || parked.RepoLane.Binding.RunGateState != taskrun.RunGateStateParkedAwaitingClosure {
		t.Fatalf("gate round-trip did not park on the workflow: %#v", parked.RepoLane.Binding)
	}

	// A fresh query confirms BOTH signals are durably applied to the SAME workflow state.
	got, err := b.GetActiveTaskRun(ctx, runID)
	if err != nil {
		t.Fatalf("get active task run: %v", err)
	}
	bnd := got.RepoLane.Binding
	if bnd == nil || bnd.RunGateState != taskrun.RunGateStateParkedAwaitingClosure || bnd.AgentSessionID != "sess-xyz" || bnd.LaunchedPID != 4242 {
		t.Fatalf("durable workflow state lost a signal: %#v", bnd)
	}
	t.Logf("LIVE round-trip OK: run=%s gate=%s session=%s pid=%d", runID, bnd.RunGateState, bnd.AgentSessionID, bnd.LaunchedPID)
}

func smokeEnvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
