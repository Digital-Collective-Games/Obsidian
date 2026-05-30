package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/config"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/controlplane"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/taskrun"
)

// O6 PASS-0002 / A6.2: GET /api/v1/worktrees returns each active owned-lane
// worktree with its bound { repo, issue/Task, worktree path, agent session id,
// session transcript path, run/gate state }, and never emits a vscodium:// link.
func TestWorktreesEndpointReturnsBindingAfterDispatch(t *testing.T) {
	worktreeRoot := writeTaskTrackingRoot(t)
	t.Setenv("CODEX_SESSION_ID", "session-mux-1")
	t.Setenv("CODEX_TRANSCRIPT_PATH", filepath.Join(worktreeRoot, "mux-session.jsonl"))

	taskRuntime := newFakeTaskRuntime()
	taskService := taskrun.NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), taskRuntime)
	mux := NewMux(config.Config{
		BindAddress:     "127.0.0.1:4318",
		JobsRoot:        t.TempDir(),
		WorktreeRoot:    worktreeRoot,
		TrackingRoot:    filepath.Join(worktreeRoot, "Tracking"),
		Namespace:       "default",
		TaskQueue:       "codex-orchestration",
		TemporalAddress: "127.0.0.1:7233",
	}, controlplane.NewService(t.TempDir(), newFakeBackend()), taskService, nil)

	// Empty before any dispatch.
	emptyResponse := httptest.NewRecorder()
	mux.ServeHTTP(emptyResponse, httptest.NewRequest(http.MethodGet, "/api/v1/worktrees", nil))
	if emptyResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/worktrees (empty) status = %d, want 200", emptyResponse.Code)
	}
	var emptyPayload struct {
		Worktrees []taskrun.WorktreeBinding `json:"worktrees"`
	}
	if err := json.Unmarshal(emptyResponse.Body.Bytes(), &emptyPayload); err != nil {
		t.Fatalf("decode empty worktrees: %v", err)
	}
	if len(emptyPayload.Worktrees) != 0 {
		t.Fatalf("worktrees before dispatch = %d, want 0", len(emptyPayload.Worktrees))
	}

	dispatchResponse := httptest.NewRecorder()
	mux.ServeHTTP(dispatchResponse, httptest.NewRequest(http.MethodPost, "/api/v1/tasks/Task-0008/dispatch", nil))
	if dispatchResponse.Code != http.StatusAccepted {
		t.Fatalf("dispatch status = %d, want 202", dispatchResponse.Code)
	}

	listResponse := httptest.NewRecorder()
	mux.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/api/v1/worktrees", nil))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/worktrees status = %d, want 200", listResponse.Code)
	}
	var payload struct {
		Worktrees []taskrun.WorktreeBinding `json:"worktrees"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode worktrees: %v", err)
	}
	if len(payload.Worktrees) != 1 {
		t.Fatalf("worktrees after dispatch = %d, want 1: %s", len(payload.Worktrees), listResponse.Body.String())
	}
	wt := payload.Worktrees[0]
	if wt.TaskID != "Task-0008" {
		t.Fatalf("worktree task id = %q, want Task-0008", wt.TaskID)
	}
	if wt.WorktreePath == "" {
		t.Fatal("worktree path must be present (VSCodium link input)")
	}
	if wt.AgentSessionID != "session-mux-1" {
		t.Fatalf("worktree session id = %q, want session-mux-1", wt.AgentSessionID)
	}
	if wt.SessionTranscriptPath != filepath.Join(worktreeRoot, "mux-session.jsonl") {
		t.Fatalf("worktree transcript path = %q", wt.SessionTranscriptPath)
	}
	if wt.RunGateState != taskrun.RunGateStateRunning {
		t.Fatalf("worktree run/gate state = %q, want %q", wt.RunGateState, taskrun.RunGateStateRunning)
	}

	// O6 boundary: the endpoint supplies link-construction fields but never emits
	// a vscodium:// link of its own.
	if strings.Contains(strings.ToLower(listResponse.Body.String()), "vscodium://") {
		t.Fatalf("worktrees response must not emit a vscodium:// link: %s", listResponse.Body.String())
	}
}

// O4 PASS-0003 / A6.4 (re-proof at the endpoint): a PARKED needs-human worktree
// is still returned by GET /api/v1/worktrees with its parked run/gate state, so the
// operator can reach a parked agent's session. Parking retains the worktree (no
// deallocation), which is exactly why it stays listed. This drives the same
// endpoint handler the live lane serves, with no Temporal/Postgres dependency.
func TestWorktreesEndpointListsParkedWorktree(t *testing.T) {
	worktreeRoot := writeTaskTrackingRoot(t)
	t.Setenv("CODEX_SESSION_ID", "session-parked-1")
	t.Setenv("CODEX_TRANSCRIPT_PATH", filepath.Join(worktreeRoot, "parked-session.jsonl"))

	taskService := taskrun.NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), newFakeTaskRuntime())
	mux := NewMux(config.Config{
		BindAddress:     "127.0.0.1:4318",
		JobsRoot:        t.TempDir(),
		WorktreeRoot:    worktreeRoot,
		TrackingRoot:    filepath.Join(worktreeRoot, "Tracking"),
		Namespace:       "default",
		TaskQueue:       "codex-orchestration",
		TemporalAddress: "127.0.0.1:7233",
	}, controlplane.NewService(t.TempDir(), newFakeBackend()), taskService, nil)

	dispatchResponse := httptest.NewRecorder()
	mux.ServeHTTP(dispatchResponse, httptest.NewRequest(http.MethodPost, "/api/v1/tasks/Task-0008/dispatch", nil))
	if dispatchResponse.Code != http.StatusAccepted {
		t.Fatalf("dispatch status = %d, want 202", dispatchResponse.Code)
	}

	// Park the lane awaiting closure approval (Human Needed=Yes), as the O3 consumer
	// would when it observes the parked issue.
	if _, err := taskService.SetRunGateState("Task-0008", taskrun.RunGateStateParkedAwaitingClosure); err != nil {
		t.Fatalf("park lane: %v", err)
	}

	listResponse := httptest.NewRecorder()
	mux.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/api/v1/worktrees", nil))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/worktrees status = %d, want 200", listResponse.Code)
	}
	var payload struct {
		Worktrees []taskrun.WorktreeBinding `json:"worktrees"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode worktrees: %v", err)
	}
	if len(payload.Worktrees) != 1 {
		t.Fatalf("parked worktree should still be listed (retained), got %d: %s", len(payload.Worktrees), listResponse.Body.String())
	}
	wt := payload.Worktrees[0]
	if wt.RunGateState != taskrun.RunGateStateParkedAwaitingClosure {
		t.Fatalf("parked worktree run/gate state = %q, want %q", wt.RunGateState, taskrun.RunGateStateParkedAwaitingClosure)
	}
	if wt.TaskID != "Task-0008" || wt.WorktreePath == "" || wt.SessionTranscriptPath == "" {
		t.Fatalf("parked worktree must keep its binding for operator follow-up: %#v", wt)
	}
}

func TestWorktreesEndpointRejectsNonGet(t *testing.T) {
	worktreeRoot := writeTaskTrackingRoot(t)
	taskService := taskrun.NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), newFakeTaskRuntime())
	mux := NewMux(config.Config{
		BindAddress:     "127.0.0.1:4318",
		JobsRoot:        t.TempDir(),
		WorktreeRoot:    worktreeRoot,
		TrackingRoot:    filepath.Join(worktreeRoot, "Tracking"),
		Namespace:       "default",
		TaskQueue:       "codex-orchestration",
		TemporalAddress: "127.0.0.1:7233",
	}, controlplane.NewService(t.TempDir(), newFakeBackend()), taskService, nil)

	postResponse := httptest.NewRecorder()
	mux.ServeHTTP(postResponse, httptest.NewRequest(http.MethodPost, "/api/v1/worktrees", nil))
	if postResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/v1/worktrees status = %d, want 405", postResponse.Code)
	}
}
