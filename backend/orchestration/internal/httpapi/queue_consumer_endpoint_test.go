package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/config"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/controlplane"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/queue"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/taskrun"
)

// fakeQueueDrainController records start/stop calls so the endpoint test can prove
// the route starts/stops the consumer through the QueueDrainController seam (A3.4).
type fakeQueueDrainController struct {
	started []queue.QueueDrainConfig
	stops   int
}

func (c *fakeQueueDrainController) StartQueueDrainConsumer(_ context.Context, cfg queue.QueueDrainConfig) (string, error) {
	c.started = append(c.started, cfg)
	return queue.QueueDrainWorkflowID, nil
}

func (c *fakeQueueDrainController) StopQueueDrainConsumer(context.Context) error {
	c.stops++
	return nil
}

// A3.4: a new HTTP endpoint starts the consumer (and a sibling stop). We assert the
// POST /api/v1/queue/consumer/start route invokes StartQueueDrainConsumer with the
// configured provider repo, and /stop invokes StopQueueDrainConsumer.
func TestQueueConsumerStartStopEndpoint(t *testing.T) {
	worktreeRoot := writeTaskTrackingRoot(t)
	taskService := taskrun.NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), newFakeTaskRuntime())
	controller := &fakeQueueDrainController{}
	mux := NewMux(config.Config{
		BindAddress:     "127.0.0.1:4318",
		JobsRoot:        t.TempDir(),
		WorktreeRoot:    worktreeRoot,
		TrackingRoot:    filepath.Join(worktreeRoot, "Tracking"),
		Namespace:       "default",
		TaskQueue:       "codex-orchestration",
		TemporalAddress: "127.0.0.1:7233",
		QueueDrainRepo:  "Digital-Collective-Games/QueueDrainTestbed",
	}, controlplane.NewService(t.TempDir(), newFakeBackend()), taskService, controller)

	// Start.
	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/queue/consumer/start", nil)
	startResp := httptest.NewRecorder()
	mux.ServeHTTP(startResp, startReq)
	if startResp.Code != http.StatusAccepted {
		t.Fatalf("start status = %d, want %d (body %s)", startResp.Code, http.StatusAccepted, startResp.Body.String())
	}
	if len(controller.started) != 1 {
		t.Fatalf("StartQueueDrainConsumer called %d times, want 1", len(controller.started))
	}
	if got := controller.started[0].Repo; got != "Digital-Collective-Games/QueueDrainTestbed" {
		t.Fatalf("consumer started with repo %q, want the configured provider repo", got)
	}
	if !strings.Contains(startResp.Body.String(), queue.QueueDrainWorkflowID) {
		t.Fatalf("start response %q should report the consumer workflow id", startResp.Body.String())
	}

	// Start with a JSON body override (custom repo + poll interval).
	overrideReq := httptest.NewRequest(http.MethodPost, "/api/v1/queue/consumer/start",
		strings.NewReader(`{"repo":"Acme/Other","poll_interval_seconds":30}`))
	overrideResp := httptest.NewRecorder()
	mux.ServeHTTP(overrideResp, overrideReq)
	if overrideResp.Code != http.StatusAccepted {
		t.Fatalf("override start status = %d, want %d", overrideResp.Code, http.StatusAccepted)
	}
	if len(controller.started) != 2 {
		t.Fatalf("override start not recorded; started = %d", len(controller.started))
	}
	if got := controller.started[1]; got.Repo != "Acme/Other" || got.PollInterval.Seconds() != 30 {
		t.Fatalf("override config = %#v, want repo Acme/Other + 30s interval", got)
	}

	// Stop.
	stopReq := httptest.NewRequest(http.MethodPost, "/api/v1/queue/consumer/stop", nil)
	stopResp := httptest.NewRecorder()
	mux.ServeHTTP(stopResp, stopReq)
	if stopResp.Code != http.StatusAccepted {
		t.Fatalf("stop status = %d, want %d", stopResp.Code, http.StatusAccepted)
	}
	if controller.stops != 1 {
		t.Fatalf("StopQueueDrainConsumer called %d times, want 1", controller.stops)
	}
}

// A GET on the consumer route is rejected (it is a POST control surface), and an
// unknown action 404s.
func TestQueueConsumerEndpointRejectsBadMethodAndAction(t *testing.T) {
	worktreeRoot := writeTaskTrackingRoot(t)
	taskService := taskrun.NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), newFakeTaskRuntime())
	controller := &fakeQueueDrainController{}
	mux := NewMux(config.Config{
		BindAddress:  "127.0.0.1:4318",
		JobsRoot:     t.TempDir(),
		WorktreeRoot: worktreeRoot,
		TrackingRoot: filepath.Join(worktreeRoot, "Tracking"),
	}, controlplane.NewService(t.TempDir(), newFakeBackend()), taskService, controller)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/queue/consumer/start", nil)
	getResp := httptest.NewRecorder()
	mux.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want %d", getResp.Code, http.StatusMethodNotAllowed)
	}

	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/queue/consumer/bogus", nil)
	badResp := httptest.NewRecorder()
	mux.ServeHTTP(badResp, badReq)
	if badResp.Code != http.StatusNotFound {
		t.Fatalf("unknown action status = %d, want %d", badResp.Code, http.StatusNotFound)
	}
}
